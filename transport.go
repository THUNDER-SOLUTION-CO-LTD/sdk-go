// Package thunder verifies Thai bank and TrueMoney slips and checks receiving accounts, amounts and duplicates.
package thunder

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Metadata contains response tracing and quota headers.
type Metadata struct {
	Status    int
	RequestID string
	Headers   http.Header
}

// SDKError is implemented by API and network failures.
type SDKError interface {
	error
	sdkError()
}

// NetworkError represents transport failure without exposing request credentials.
type NetworkError struct {
	Timeout  bool
	Cause    error
	canceled bool
	deadline bool
}

// Error returns a credential-free error message.
func (e *NetworkError) Error() string {
	if e.Timeout {
		return "HTTP request timed out"
	}
	return "HTTP network request failed"
}

// Unwrap exposes a sanitized transport cause.
func (e *NetworkError) Unwrap() error { return e.Cause }

// Is checks the error category.
func (e *NetworkError) Is(target error) bool {
	_, category := target.(*NetworkError)
	return category || e.canceled && target == context.Canceled || e.deadline && target == context.DeadlineExceeded
}

// Option configures a client transport.
type Option func(*transport) error

// WithHTTPClient injects an HTTP client without transferring ownership.
func WithHTTPClient(c *http.Client) Option {
	return func(t *transport) error {
		if c == nil {
			return errors.New("HTTP client is required")
		}
		copy := *c
		t.http = &copy
		t.owned = false
		return nil
	}
}

// WithRetry sets retries; billed calls only retry definite connect failures and 429.
func WithRetry(n int) Option {
	return func(t *transport) error {
		if n < 0 || n > 20 {
			return errors.New("retries must be between 0 and 20")
		}
		t.retries = n
		return nil
	}
}

// WithTimeout configures the overall duration of each HTTP attempt.
func WithTimeout(d time.Duration) Option {
	return func(t *transport) error {
		if d <= 0 {
			return errors.New("timeout must be positive")
		}
		t.timeout = d
		return nil
	}
}

type transport struct {
	overrideV1, overrideV2 string
	sleep                  func(context.Context, time.Duration) error
	owned                  bool
	key                    string
	base                   string
	http                   *http.Client
	timeout                time.Duration
	retries                int
	agent                  string
}

func newTransport(key, base string, options ...Option) (*transport, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("base URL must be HTTP(S) without credentials, query or fragment")
	}
	if key == "" {
		return nil, errors.New("API key is required")
	}
	defaultTransport := &http.Transport{ForceAttemptHTTP2: true}
	if configured, ok := http.DefaultTransport.(*http.Transport); ok {
		defaultTransport = configured.Clone()
	}
	defaultTransport.Proxy = nil
	t := &transport{sleep: waitRetry, owned: true, key: key, base: strings.TrimRight(base, "/"), http: &http.Client{Transport: defaultTransport}, timeout: 30 * time.Second, retries: 2, agent: "sdk/1.0.0 go/" + strings.TrimPrefix(runtime.Version(), "go")} // x-release-please-version
	// Do not inherit environment proxy configuration implicitly.
	t.http.Transport.(*http.Transport).Proxy = nil
	for _, opt := range options {
		if err := opt(t); err != nil {
			return nil, err
		}
	}
	t.http.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return t, nil
}
func retryDelay(h http.Header, attempt int) time.Duration {
	if v := h.Get("Retry-After"); v != "" {
		if sec, err := strconv.ParseInt(v, 10, 32); err == nil && sec >= 0 {
			return time.Duration(sec) * time.Second
		}
		if date, err := http.ParseTime(v); err == nil {
			d := time.Until(date)
			if d < 0 {
				return 0
			}
			return d
		}
	}
	shift := attempt
	if shift > 4 {
		shift = 4
	}
	return time.Duration(float64(500*time.Millisecond) * float64(int64(1)<<shift) * (.5 + .5*rand.Float64()))
}
func (t *transport) request(ctx context.Context, method, path string, body io.Reader, safe bool, requestHeaders ...http.Header) ([]byte, Metadata, error) {
	var empty Metadata
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, empty, errors.New("path must be relative")
	}
	var payload []byte
	if body != nil {
		var err error
		payload, err = io.ReadAll(body)
		if err != nil {
			return nil, empty, errors.New("cannot read request body")
		}
	}
	for attempt := 0; attempt <= t.retries; attempt++ {
		if ctx.Err() != nil {
			return nil, empty, &NetworkError{Cause: ctx.Err(), canceled: errors.Is(ctx.Err(), context.Canceled), deadline: errors.Is(ctx.Err(), context.DeadlineExceeded)}
		}
		attemptCtx, cancel := context.WithTimeout(ctx, t.timeout)
		req, err := http.NewRequestWithContext(attemptCtx, method, t.base+path, bytes.NewReader(payload))
		if err != nil {
			cancel()
			return nil, empty, errors.New("invalid HTTP request")
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for _, extra := range requestHeaders {
			for key, values := range extra {
				req.Header[http.CanonicalHeaderKey(key)] = append([]string(nil), values...)
			}
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+t.key)
		req.Header.Set("User-Agent", t.agent)
		response, err := t.http.Do(req)
		var data []byte
		var headers http.Header
		status := 0
		if response != nil {
			headers = response.Header.Clone()
			status = response.StatusCode
			data, err = io.ReadAll(io.LimitReader(response.Body, 16<<20))
			response.Body.Close()
		}
		var timeoutError net.Error
		expired := errors.Is(attemptCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeoutError) && timeoutError.Timeout())
		cancel()
		delay := retryDelay(headers, attempt)
		eligible := status == 429 || (safe && (status == 500 || status == 502 || status == 503 || status == 504))
		if err != nil {
			eligible = safe || (response == nil && connectFailure(err))
		}
		retry := attempt < t.retries && eligible && delay <= 60*time.Second
		if retry {
			if sleepErr := t.sleep(ctx, delay); sleepErr != nil {
				return nil, empty, &NetworkError{Cause: sleepErr, canceled: errors.Is(sleepErr, context.Canceled), deadline: errors.Is(sleepErr, context.DeadlineExceeded)}
			}
			continue
		}
		if err != nil {
			failure := NetworkError{Cause: sanitizeTransport(err, t.key), Timeout: expired, canceled: errors.Is(ctx.Err(), context.Canceled), deadline: errors.Is(ctx.Err(), context.DeadlineExceeded)}
			if expired {
				return nil, empty, &RequestTimeoutError{NetworkError: failure}
			}
			return nil, empty, &failure
		}
		for name, values := range headers {
			for i, value := range values {
				headers[name][i] = redactHeader(name, value, t.key)
			}
		}
		meta := Metadata{Status: status, RequestID: redact(headers.Get("X-Request-ID"), t.key), Headers: headers}
		if status >= 300 {
			return nil, meta, decodeAPIError(status, data, headers, t.key)
		}
		var envelope map[string]json.RawMessage
		if json.Unmarshal(data, &envelope) == nil {
			var message string
			if json.Unmarshal(envelope["message"], &message) == nil && redact(message, t.key) != message {
				envelope["message"], _ = json.Marshal(redact(message, t.key))
				data, _ = json.Marshal(envelope)
			}
		}
		return data, meta, nil
	}
	return nil, empty, errors.New("retry state exhausted")
}

// String returns a credential-free description.
func (t *transport) String() string { return "transport{credentials:[REDACTED]}" }

// GoString returns a credential-free debug description.
func (t *transport) GoString() string { return t.String() }

func (*APIError) sdkError()     {}
func (*NetworkError) sdkError() {}

func (t *transport) close() {
	if t.owned {
		t.http.CloseIdleConnections()
	}
}

func connectFailure(err error) bool {
	var operation *net.OpError
	if errors.As(err, &operation) && operation.Op == "dial" {
		return true
	}
	var dns *net.DNSError
	var certificate *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var invalidCertificate x509.CertificateInvalidError
	var hostname x509.HostnameError
	var record tls.RecordHeaderError
	return errors.As(err, &dns) || errors.As(err, &certificate) || errors.As(err, &unknownAuthority) || errors.As(err, &invalidCertificate) || errors.As(err, &hostname) || errors.As(err, &record) || errors.Is(err, syscall.ECONNREFUSED)
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
