package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"
)

const Version = 1

const (
	MinChunkSize = 1024
	MaxChunkSize = 16 * 1024 * 1024
)

type AddressType string

const (
	AddressIPv4   AddressType = "ipv4"
	AddressIPv6   AddressType = "ipv6"
	AddressDomain AddressType = "domain"
)

type Target struct {
	Type AddressType `json:"type"`
	Host string      `json:"host"`
	Port uint16      `json:"port"`
}

func (t Target) Address() string {
	return netJoinHostPort(t.Host, t.Port)
}

type OpenRequest struct {
	Version             int       `json:"version"`
	SessionID           string    `json:"session_id"`
	Target              Target    `json:"target"`
	MaxReceiveChunkSize int       `json:"max_receive_chunk_size"`
	CreatedAt           time.Time `json:"created_at"`
}

type OpenResult struct {
	Version             int       `json:"version"`
	SessionID           string    `json:"session_id"`
	Accepted            bool      `json:"accepted"`
	Error               string    `json:"error,omitempty"`
	MaxReceiveChunkSize int       `json:"max_receive_chunk_size"`
	CreatedAt           time.Time `json:"created_at"`
}

type Ack struct {
	Version   int       `json:"version"`
	SessionID string    `json:"session_id"`
	Direction string    `json:"direction"`
	NextSeq   uint64    `json:"next_seq"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Close struct {
	Version   int       `json:"version"`
	SessionID string    `json:"session_id"`
	Side      string    `json:"side"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Heartbeat struct {
	Version   int       `json:"version"`
	SessionID string    `json:"session_id"`
	Side      string    `json:"side"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func NormalizePrefix(prefix string) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return "s3s5"
	}
	return prefix
}

func OpenKey(prefix, sessionID string) string {
	return key(prefix, "v1", "open", sessionID+".json")
}

func OpenResultKey(prefix, sessionID string) string {
	return key(prefix, "v1", "open-result", sessionID+".json")
}

func DataKey(prefix, direction, sessionID string, seq uint64) string {
	return key(prefix, "v1", "data", direction, sessionID, FormatSeq(seq)+".bin")
}

func AckKey(prefix, direction, sessionID string) string {
	return key(prefix, "v1", "ack", direction, sessionID+".json")
}

func CloseKey(prefix, side, sessionID string) string {
	return key(prefix, "v1", "close", side, sessionID+".json")
}

func HeartbeatKey(prefix, side, sessionID string) string {
	return key(prefix, "v1", "heartbeat", side, sessionID+".json")
}

func OpenPrefix(prefix string) string {
	return key(prefix, "v1", "open") + "/"
}

func FormatSeq(seq uint64) string {
	return fmt.Sprintf("%020d", seq)
}

func ParseSeq(s string) (uint64, error) {
	s = strings.TrimSuffix(path.Base(s), ".bin")
	return strconv.ParseUint(s, 10, 64)
}

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func ValidateChunkSize(n int) error {
	if n < MinChunkSize {
		return fmt.Errorf("chunk size %d below minimum %d", n, MinChunkSize)
	}
	if n > MaxChunkSize {
		return fmt.Errorf("chunk size %d exceeds maximum %d", n, MaxChunkSize)
	}
	return nil
}

func EffectiveSendChunkSize(local, peerReceive int) (int, error) {
	if err := ValidateChunkSize(local); err != nil {
		return 0, err
	}
	if err := ValidateChunkSize(peerReceive); err != nil {
		return 0, err
	}
	if local < peerReceive {
		return local, nil
	}
	return peerReceive, nil
}

func key(prefix string, parts ...string) string {
	all := append([]string{NormalizePrefix(prefix)}, parts...)
	return path.Join(all...)
}

func netJoinHostPort(host string, port uint16) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]:" + strconv.Itoa(int(port))
	}
	return host + ":" + strconv.Itoa(int(port))
}
