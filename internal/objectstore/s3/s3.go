package s3

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"s3s5/internal/objectstore"
)

type Config struct {
	Provider        string
	Bucket          string
	Region          string
	Endpoint        string
	ForcePathStyle  bool
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

type Store struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) (*Store, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("bucket is required")
	}
	applyProviderDefaults(&cfg)
	if cfg.AccessKeyID == "" {
		cfg.AccessKeyID = firstEnv("S3S5_ACCESS_KEY_ID", "AWS_ACCESS_KEY_ID", "YC_ACCESS_KEY_ID")
	}
	if cfg.SecretAccessKey == "" {
		cfg.SecretAccessKey = firstEnv("S3S5_SECRET_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY", "YC_SECRET_ACCESS_KEY")
	}
	if cfg.SessionToken == "" {
		cfg.SessionToken = firstEnv("S3S5_SESSION_TOKEN", "AWS_SESSION_TOKEN", "YC_SESSION_TOKEN")
	}
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, errors.New("S3 credentials are required: set S3S5_ACCESS_KEY_ID/S3S5_SECRET_ACCESS_KEY or AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY")
	}
	return &Store{cfg: cfg, client: &http.Client{Timeout: 60 * time.Second}}, nil
}

func applyProviderDefaults(cfg *Config) {
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	if cfg.Provider == "" {
		cfg.Provider = "aws"
	}
	switch cfg.Provider {
	case "aws":
		if cfg.Region == "" {
			cfg.Region = "us-east-1"
		}
	case "yandex", "yc", "yandex-cloud", "yandexcloud":
		cfg.Provider = "yandex"
		if cfg.Endpoint == "" {
			cfg.Endpoint = "https://storage.yandexcloud.net"
		}
		if cfg.Region == "" || cfg.Region == "us-east-1" || strings.HasPrefix(cfg.Region, "ru-central1-") {
			cfg.Region = "ru-central1"
		}
		cfg.ForcePathStyle = true
	case "minio":
		if cfg.Endpoint == "" {
			cfg.Endpoint = "http://127.0.0.1:9000"
		}
		if cfg.Region == "" {
			cfg.Region = "us-east-1"
		}
		cfg.ForcePathStyle = true
	case "custom":
		if cfg.Region == "" {
			cfg.Region = "us-east-1"
		}
	default:
		if cfg.Region == "" {
			cfg.Region = "us-east-1"
		}
	}
	if strings.Contains(cfg.Bucket, ".") && cfg.Endpoint != "" {
		cfg.ForcePathStyle = true
	}
}

func (s *Store) PutObject(ctx context.Context, key string, data []byte, opts objectstore.PutOptions) error {
	req, err := s.newRequest(ctx, http.MethodPut, key, nil, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if opts.ContentType != "" {
		req.Header.Set("Content-Type", opts.ContentType)
	}
	for k, v := range opts.Metadata {
		req.Header.Set("x-amz-meta-"+strings.ToLower(k), v)
	}
	req.ContentLength = int64(len(data))
	return s.doNoBody(req, http.StatusOK)
}

func (s *Store) GetObject(ctx context.Context, key string) ([]byte, error) {
	req, err := s.newRequest(ctx, http.MethodGet, key, nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, objectstore.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp)
	}
	return io.ReadAll(resp.Body)
}

func (s *Store) HeadObject(ctx context.Context, key string) (objectstore.ObjectInfo, error) {
	req, err := s.newRequest(ctx, http.MethodHead, key, nil, nil)
	if err != nil {
		return objectstore.ObjectInfo{}, err
	}
	resp, err := s.do(req)
	if err != nil {
		return objectstore.ObjectInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return objectstore.ObjectInfo{}, objectstore.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return objectstore.ObjectInfo{}, statusError(resp)
	}
	return objectstore.ObjectInfo{Key: key, Size: resp.ContentLength, Metadata: metadataFromHeader(resp.Header)}, nil
}

func (s *Store) ListPrefix(ctx context.Context, prefix string, opts objectstore.ListOptions) ([]string, error) {
	q := url.Values{}
	q.Set("list-type", "2")
	q.Set("prefix", prefix)
	if opts.MaxKeys > 0 {
		q.Set("max-keys", fmt.Sprintf("%d", opts.MaxKeys))
	}
	req, err := s.newRequest(ctx, http.MethodGet, "", q, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp)
	}
	var out listBucketResult
	if err := xml.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(out.Contents))
	for _, c := range out.Contents {
		keys = append(keys, c.Key)
	}
	sort.Strings(keys)
	return keys, nil
}

func (s *Store) DeleteObject(ctx context.Context, key string) error {
	req, err := s.newRequest(ctx, http.MethodDelete, key, nil, nil)
	if err != nil {
		return err
	}
	return s.doNoBody(req, http.StatusNoContent, http.StatusOK)
}

func (s *Store) newRequest(ctx context.Context, method, key string, query url.Values, body io.Reader) (*http.Request, error) {
	u, err := s.objectURL(key)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	if body == nil {
		body = http.NoBody
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (s *Store) objectURL(key string) (*url.URL, error) {
	escapedKey := escapePath(key)
	if s.cfg.Endpoint != "" {
		base, err := url.Parse(s.cfg.Endpoint)
		if err != nil {
			return nil, err
		}
		if s.cfg.ForcePathStyle {
			base.Path = joinURLPath(base.Path, s.cfg.Bucket, escapedKey)
		} else {
			base.Host = s.cfg.Bucket + "." + base.Host
			base.Path = joinURLPath(base.Path, escapedKey)
		}
		return base, nil
	}
	host := fmt.Sprintf("%s.s3.%s.amazonaws.com", s.cfg.Bucket, s.cfg.Region)
	return &url.URL{Scheme: "https", Host: host, Path: "/" + escapedKey}, nil
}

func (s *Store) doNoBody(req *http.Request, success ...int) error {
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	for _, code := range success {
		if resp.StatusCode == code {
			return nil
		}
	}
	if resp.StatusCode == http.StatusNotFound {
		return objectstore.ErrNotFound
	}
	return statusError(resp)
}

func (s *Store) do(req *http.Request) (*http.Response, error) {
	if err := s.sign(req); err != nil {
		return nil, err
	}
	return s.client.Do(req)
}

func (s *Store) sign(req *http.Request) error {
	var body []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	sum := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", amzDate)
	if s.cfg.SessionToken != "" {
		req.Header.Set("x-amz-security-token", s.cfg.SessionToken)
	}
	signedHeaders, canonicalHeaders := canonicalHeaders(req.Header, req.URL.Host)
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL.EscapedPath()),
		canonicalQuery(req.URL.Query()),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
	scope := strings.Join([]string{shortDate, s.cfg.Region, "s3", "aws4_request"}, "/")
	reqHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hex.EncodeToString(reqHash[:]),
	}, "\n")
	signingKey := sigKey(s.cfg.SecretAccessKey, shortDate, s.cfg.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
	auth := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", s.cfg.AccessKeyID, scope, signedHeaders, signature)
	req.Header.Set("Authorization", auth)
	if len(body) > 0 {
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	return nil
}

type listBucketResult struct {
	Contents []struct {
		Key string `xml:"Key"`
	} `xml:"Contents"`
}

func metadataFromHeader(h http.Header) map[string]string {
	out := map[string]string{}
	for k, v := range h {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "x-amz-meta-") && len(v) > 0 {
			out[strings.TrimPrefix(lk, "x-amz-meta-")] = v[0]
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func statusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("s3 %s %s: %s", resp.Request.Method, resp.Request.URL.Redacted(), msg)
}

func canonicalHeaders(h http.Header, host string) (string, string) {
	values := map[string]string{"host": strings.TrimSpace(host)}
	for k, vs := range h {
		lk := strings.ToLower(k)
		trimmed := make([]string, 0, len(vs))
		for _, v := range vs {
			trimmed = append(trimmed, strings.Join(strings.Fields(v), " "))
		}
		values[lk] = strings.Join(trimmed, ",")
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(values[k])
		b.WriteByte('\n')
	}
	return strings.Join(keys, ";"), b.String()
}

func canonicalQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vals := append([]string(nil), q[k]...)
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, awsEscape(k)+"="+awsEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

func canonicalURI(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = awsEscape(part)
	}
	return strings.Join(parts, "/")
}

func awsEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func joinURLPath(parts ...string) string {
	var cleaned []string
	for _, p := range parts {
		p = strings.Trim(p, "/")
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) == 0 {
		return "/"
	}
	return "/" + strings.Join(cleaned, "/")
}

func sigKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
