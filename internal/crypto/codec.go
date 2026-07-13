package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
)

type Codec interface {
	Seal(objectType, sessionID, direction string, seq uint64, plaintext []byte) ([]byte, error)
	Open(objectType, sessionID, direction string, seq uint64, data []byte) ([]byte, error)
	SealData(sessionID, direction string, seq uint64, plaintext []byte) ([]byte, error)
	OpenData(sessionID, direction string, seq uint64, data []byte) ([]byte, error)
	Enabled() bool
}

type NoopCodec struct{}

func (NoopCodec) Seal(_, _, _ string, _ uint64, plaintext []byte) ([]byte, error) {
	return append([]byte(nil), plaintext...), nil
}

func (NoopCodec) Open(_, _, _ string, _ uint64, data []byte) ([]byte, error) {
	return append([]byte(nil), data...), nil
}

func (NoopCodec) SealData(_, _ string, _ uint64, plaintext []byte) ([]byte, error) {
	return append([]byte(nil), plaintext...), nil
}

func (NoopCodec) OpenData(_, _ string, _ uint64, data []byte) ([]byte, error) {
	return append([]byte(nil), data...), nil
}

func (NoopCodec) Enabled() bool { return false }

type PSKCodec struct {
	psk []byte
}

type envelope struct {
	Version    int    `json:"v"`
	Algorithm  string `json:"alg"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

var dataMagic = [4]byte{'S', '5', 'D', '1'}

const (
	dataEnvelopeVersion = 1
	dataAlgorithmAESGCM = 1
	dataHeaderLen       = 12
)

func NewPSKCodec(psk string) (*PSKCodec, error) {
	if len(psk) < 16 {
		return nil, errors.New("S3S5_PSK must be at least 16 characters")
	}
	return &PSKCodec{psk: []byte(psk)}, nil
}

func (c *PSKCodec) Enabled() bool { return true }

func (c *PSKCodec) Seal(objectType, sessionID, direction string, seq uint64, plaintext []byte) ([]byte, error) {
	return c.sealJSON(objectType, sessionID, direction, seq, plaintext)
}

func (c *PSKCodec) Open(objectType, sessionID, direction string, seq uint64, data []byte) ([]byte, error) {
	return c.openJSON(objectType, sessionID, direction, seq, data)
}

func (c *PSKCodec) SealData(sessionID, direction string, seq uint64, plaintext []byte) ([]byte, error) {
	nonce, ciphertext, err := c.sealRaw("data", sessionID, direction, seq, plaintext)
	if err != nil {
		return nil, err
	}
	out := make([]byte, dataHeaderLen+len(nonce)+len(ciphertext))
	copy(out[:4], dataMagic[:])
	out[4] = dataEnvelopeVersion
	out[5] = dataAlgorithmAESGCM
	out[6] = byte(len(nonce))
	out[7] = 0
	binary.BigEndian.PutUint32(out[8:12], uint32(len(ciphertext)))
	copy(out[dataHeaderLen:], nonce)
	copy(out[dataHeaderLen+len(nonce):], ciphertext)
	return out, nil
}

func (c *PSKCodec) OpenData(sessionID, direction string, seq uint64, data []byte) ([]byte, error) {
	if len(data) < dataHeaderLen {
		return nil, errors.New("truncated data crypto envelope")
	}
	if !bytes.Equal(data[:4], dataMagic[:]) {
		return nil, errors.New("invalid data crypto envelope magic")
	}
	if data[4] != dataEnvelopeVersion {
		return nil, errors.New("unsupported data crypto envelope version")
	}
	if data[5] != dataAlgorithmAESGCM {
		return nil, errors.New("unsupported data crypto envelope algorithm")
	}
	nonceLen := int(data[6])
	if nonceLen != 12 {
		return nil, errors.New("invalid data crypto nonce size")
	}
	ctLen := int(binary.BigEndian.Uint32(data[8:12]))
	if ctLen < 16 {
		return nil, errors.New("invalid data crypto ciphertext size")
	}
	if len(data) != dataHeaderLen+nonceLen+ctLen {
		return nil, errors.New("invalid data crypto envelope length")
	}
	nonce := data[dataHeaderLen : dataHeaderLen+nonceLen]
	ciphertext := data[dataHeaderLen+nonceLen:]
	return c.openRaw("data", sessionID, direction, seq, nonce, ciphertext)
}

func (c *PSKCodec) sealJSON(objectType, sessionID, direction string, seq uint64, plaintext []byte) ([]byte, error) {
	nonce, ct, err := c.sealRaw(objectType, sessionID, direction, seq, plaintext)
	if err != nil {
		return nil, err
	}
	env := envelope{
		Version:    1,
		Algorithm:  "AES-256-GCM",
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ct),
	}
	return json.Marshal(env)
}

func (c *PSKCodec) openJSON(objectType, sessionID, direction string, seq uint64, data []byte) ([]byte, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	if env.Version != 1 || env.Algorithm != "AES-256-GCM" {
		return nil, errors.New("unsupported crypto envelope")
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, err
	}
	ct, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return nil, err
	}
	return c.openRaw(objectType, sessionID, direction, seq, nonce, ct)
}

func (c *PSKCodec) sealRaw(objectType, sessionID, direction string, seq uint64, plaintext []byte) ([]byte, []byte, error) {
	key := c.deriveKey(sessionID, direction)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	aad := associatedData(objectType, sessionID, direction, seq)
	ct := gcm.Seal(nil, nonce, plaintext, aad)
	return nonce, ct, nil
}

func (c *PSKCodec) openRaw(objectType, sessionID, direction string, seq uint64, nonce, ct []byte) ([]byte, error) {
	key := c.deriveKey(sessionID, direction)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid crypto nonce size")
	}
	aad := associatedData(objectType, sessionID, direction, seq)
	return gcm.Open(nil, nonce, ct, aad)
}

func (c *PSKCodec) deriveKey(sessionID, direction string) []byte {
	salt := []byte("s3s5/v1/" + sessionID)
	info := []byte("payload/" + direction)
	return hkdfSHA256(c.psk, salt, info, 32)
}

func associatedData(objectType, sessionID, direction string, seq uint64) []byte {
	return []byte(fmt.Sprintf("s3s5/v1|%s|%s|%s|%020d", objectType, sessionID, direction, seq))
}

func hkdfSHA256(secret, salt, info []byte, n int) []byte {
	prk := hmacHash(sha256.New, salt, secret)
	var out bytes.Buffer
	var prev []byte
	counter := byte(1)
	for out.Len() < n {
		h := hmac.New(sha256.New, prk)
		h.Write(prev)
		h.Write(info)
		h.Write([]byte{counter})
		prev = h.Sum(nil)
		out.Write(prev)
		counter++
	}
	return out.Bytes()[:n]
}

func hmacHash(h func() hash.Hash, key, data []byte) []byte {
	m := hmac.New(h, key)
	m.Write(data)
	return m.Sum(nil)
}
