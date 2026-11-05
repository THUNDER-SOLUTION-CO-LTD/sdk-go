package thunder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSafeReadRetriesAndMetadata(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer hidden-key" {
			t.Error("missing auth")
		}
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		w.Header().Set("X-Request-ID", "req-1")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c, err := newTransport("hidden-key", srv.URL, WithHTTPClient(srv.Client()), WithRetry(1))
	if err != nil {
		t.Fatal(err)
	}
	body, meta, err := c.request(context.Background(), "GET", "/info", nil, true)
	if err != nil || string(body) != `{"ok":true}` || meta.RequestID != "req-1" || calls != 2 {
		t.Fatalf("unexpected result calls=%d err=%v", calls, err)
	}
}

func TestQuotaIsNotRetriedAndSecretRedacted(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(503)
		w.Write([]byte(`{"code":"BUSY","message":"hidden-key"}`))
	}))
	defer srv.Close()
	c, _ := newTransport("hidden-key", srv.URL, WithRetry(3))
	_, _, err := c.request(context.Background(), "POST", "/verify", strings.NewReader(`{}`), false)
	var api *APIError
	if calls != 1 || !errors.As(err, &api) || optionalString(api.Code) != "BUSY" || strings.Contains(err.Error(), "hidden-key") || strings.Contains(api.RawBody, "hidden-key") {
		t.Fatalf("unexpected safe failure %v", err)
	}
}

func TestCancellation(t *testing.T) {
	c, _ := newTransport("hidden-key", "https://invalid.example", WithTimeout(time.Second))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := c.request(ctx, "GET", "/info", nil, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
}

func TestTransportFormattingHidesCredential(t *testing.T) {
	c, _ := newTransport("hidden-key", "https://fixture.invalid")
	if strings.Contains(fmt.Sprintf("%+v %#v", c, c), "hidden-key") {
		t.Fatal("credential exposed in formatted transport")
	}
}
func TestReadFailureAfterHeadersIsNetworkError(t *testing.T) {
	c, _ := newTransport("hidden-key", "https://fixture.invalid", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(failingReader{})}, nil
	})}))
	_, _, err := c.request(context.Background(), "POST", "/verify", nil, false)
	var network *NetworkError
	if !errors.As(err, &network) {
		t.Fatalf("expected sanitized network error: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) { return 0, errors.New("hidden-key") }

func TestCancellationDuringRetryWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "30"); w.WriteHeader(429) }))
	defer srv.Close()
	c, _ := newTransport("hidden-key", srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := c.request(ctx, "GET", "/info", nil, true)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("retry wait did not honor context: %v", err)
	}
}
func TestTimeoutAndRedirect(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		<-r.Context().Done()
	}))
	defer srv.Close()
	c, _ := newTransport("hidden-key", srv.URL, WithTimeout(20*time.Millisecond), WithRetry(0))
	_, _, err := c.request(context.Background(), "GET", "/redirect", nil, true)
	var api *APIError
	if !errors.As(err, &api) || api.Status != 302 || calls != 1 {
		t.Fatalf("redirect followed: %v", err)
	}
	_, _, err = c.request(context.Background(), "POST", "/slow", nil, false)
	var network *NetworkError
	if !errors.As(err, &network) || !network.Timeout {
		t.Fatalf("expected timeout: %v", err)
	}
}

func TestEscapedEchoCannotRestoreKey(t *testing.T) {
	escaped := ""
	for _, r := range "hidden-key" {
		escaped += fmt.Sprintf("\\u%04x", r)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "hidden-key")
		w.WriteHeader(400)
		w.Write([]byte(`{"code":"BAD","message":"` + escaped + `"}`))
	}))
	defer srv.Close()
	c, _ := newTransport("hidden-key", srv.URL)
	_, _, err := c.request(context.Background(), "POST", "/verify", nil, false)
	var api *APIError
	if !errors.As(err, &api) || strings.Contains(api.Message+optionalString(api.Code)+optionalString(api.RequestID), "hidden-key") {
		t.Fatalf("decoded response leaked credential")
	}
}

func TestNestedV2ErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"success":false,"error":{"code":"VALIDATION_ERROR","message":"Invalid input"}}`))
	}))
	defer srv.Close()
	c, _ := newTransport("hidden-key", srv.URL)
	_, _, err := c.request(context.Background(), "POST", "/verify", nil, false)
	var api *APIError
	if !errors.As(err, &api) || optionalString(api.Code) != "VALIDATION_ERROR" || api.Message != "Invalid input" {
		t.Fatalf("invalid envelope decoding %v", err)
	}
}

func TestErrorsShareTypedRoot(t *testing.T) {
	var root SDKError
	if !errors.As(error(&APIError{Status: 400, Code: nullableString("BAD"), Message: "Invalid"}), &root) {
		t.Fatal("API error lacks root")
	}
	if !errors.As(error(&NetworkError{}), &root) {
		t.Fatal("network error lacks root")
	}
}

func TestChangedGlobalDefaultTransportDoesNotPanicOrLeakProxy(t *testing.T) {
	saved := http.DefaultTransport
	defer func() { http.DefaultTransport = saved }()
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("unexpected global transport") })
	c, err := newTransport("hidden-key", "https://fixture.invalid")
	if err != nil || c == nil {
		t.Fatalf("cannot construct transport: %v", err)
	}
	if c.http.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("environment proxy unexpectedly enabled")
	}
}

func TestQuotaGETIsNotImplicitlySafe(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(503) }))
	defer srv.Close()
	c, _ := newTransport("hidden-key", srv.URL, WithRetry(3))
	_, _, err := c.request(context.Background(), "GET", "/verify", nil, false)
	if err == nil || calls != 1 {
		t.Fatalf("quota GET retried: %d %v", calls, err)
	}
}

func TestCustomContentTypeCannotReplaceAuthOrUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "multipart/form-data; boundary=fixture" || r.Header.Get("Authorization") != "Bearer hidden-key" || !(strings.HasPrefix(r.Header.Get("User-Agent"), "sdk/") && strings.Contains(r.Header.Get("User-Agent"), " go/1.")) {
			t.Error("invalid protected headers")
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c, _ := newTransport("hidden-key", srv.URL)
	_, _, err := c.request(context.Background(), "POST", "/upload", strings.NewReader("fixture"), false, http.Header{"Content-Type": {"multipart/form-data; boundary=fixture"}, "Authorization": {"bad"}, "User-Agent": {"bad"}})
	if err != nil {
		t.Fatal(err)
	}
}

type closableRoundTripper struct{ closed int }

func (c *closableRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("fixture")
}
func (c *closableRoundTripper) CloseIdleConnections() { c.closed++ }
func TestCloseLeavesInjectedClientOwnedByCaller(t *testing.T) {
	httpTransport := &closableRoundTripper{}
	client, _ := newTransport("hidden-key", "https://fixture.invalid", WithHTTPClient(&http.Client{Transport: httpTransport}))
	client.close()
	if httpTransport.closed != 0 {
		t.Fatal("SDK closed caller-owned connections")
	}
}

func TestRetryClassesAndServerDelayCap(t *testing.T) {
	for _, item := range []struct {
		status int
		safe   bool
		after  string
		count  int
	}{{429, false, "0", 3}, {429, false, "61", 1}, {501, true, "", 1}, {503, true, "61", 1}, {502, true, "0", 3}} {
		t.Run(fmt.Sprint(item.status, item.safe, item.after), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if item.after != "" {
					w.Header().Set("Retry-After", item.after)
				}
				w.WriteHeader(item.status)
			}))
			defer srv.Close()
			client, _ := newTransport("hidden-key", srv.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_, _, err := client.request(ctx, "POST", "/fixture", nil, item.safe)
			if err == nil || calls != item.count {
				t.Fatalf("wrong retry count: %d %v", calls, err)
			}
		})
	}
}

func TestDefiniteDialFailureCanRetryBeforeQuotaRequestIsSent(t *testing.T) {
	calls := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls < 3 {
			return nil, &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	client, _ := newTransport("hidden-key", "https://fixture.invalid", WithHTTPClient(&http.Client{Transport: transport}))
	delays := []time.Duration{}
	client.sleep = func(_ context.Context, delay time.Duration) error { delays = append(delays, delay); return nil }
	_, _, err := client.request(context.Background(), "POST", "/verify", strings.NewReader(`{}`), false)
	if err != nil || calls != 3 || len(delays) != 2 || delays[0] < 250*time.Millisecond || delays[0] > 500*time.Millisecond {
		t.Fatalf("invalid connect retries: %d %v", calls, err)
	}
}
