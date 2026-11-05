package thunder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWaitRejectsLateTerminal(t *testing.T) {
	raw, err := os.ReadFile(fixturePath("v2/jobs/get/done.json"))
	if os.IsNotExist(err) {
		t.Skip("fixtures unavailable in prepared module")
	}
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Response struct{ Body json.RawMessage }
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	c, err := NewClient("synthetic-secret", WithMaxRetries(0), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		time.Sleep(80 * time.Millisecond) // Deliberately uncooperative RoundTripper.
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(fixture.Response.Body)))}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err = c.Jobs.WaitForJob(context.Background(), "j", &WaitOptions{Timeout: 20 * time.Millisecond})
	if time.Since(started) > 65*time.Millisecond {
		t.Fatal("late transport blocked caller")
	}
	defer time.Sleep(90 * time.Millisecond)
	var timeout *JobWaitTimeoutError
	if !errors.As(err, &timeout) {
		t.Fatalf("expected wait timeout, got %T", err)
	}
}

func TestWaitStalledRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer srv.Close()
	c, err := NewClient("synthetic-secret", WithBaseURLs(srv.URL, srv.URL), WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = c.Jobs.WaitForJob(context.Background(), "j", &WaitOptions{Timeout: 30 * time.Millisecond})
	var timeout *JobWaitTimeoutError
	if !errors.As(err, &timeout) || time.Since(start) > 300*time.Millisecond {
		t.Fatalf("deadline not enforced: %T", err)
	}
}

func TestReflectedAuthorizationAllErrorRepresentations(t *testing.T) {
	const secret = "synthetic-credential-98"
	c, err := NewClient(secret, WithMaxRetries(0), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		auth := r.Header.Get("Authorization")
		raw, _ := json.Marshal(map[string]any{"error": map[string]string{"code": auth, "message": auth}, "details": map[string]string{"authorization": auth}})
		return &http.Response{StatusCode: 401, Header: http.Header{"X-Request-Id": {auth}, "X-Echo": {auth}}, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetInfo(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	raw, _ := json.Marshal(err)
	for _, view := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err), string(raw)} {
		if strings.Contains(view, secret) {
			t.Fatal("credential retained")
		}
	}
}

func TestSensitiveOriginalAndEncodedEchoes(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, encoded := range []bool{false, true} {
			const key = "synthetic+credential/98"
			echo := key
			if encoded {
				echo = url.QueryEscape(key)
			}
			body := map[string]any{"message": echo, "details": map[string]string{echo: echo}}
			if !legacy {
				body = map[string]any{"error": map[string]string{"code": "INVALID_API_KEY", "message": echo}, "details": map[string]string{echo: echo}}
			}
			raw, _ := json.Marshal(body)
			err := decodeAPIError(401, raw, http.Header{"Authorization": {"different"}, "Proxy-Authorization": {"different"}, "X-Echo": {echo}}, key)
			var api *ApiError
			if !errors.As(err, &api) {
				t.Fatal("expected API error")
			}
			if api.SensitiveRawBody() != string(raw) {
				t.Fatal("original response lost")
			}
			if api.Headers["authorization"] != "[REDACTED]" || api.Headers["proxy-authorization"] != "[REDACTED]" {
				t.Fatal("unsafe auth header")
			}
			serialized, _ := json.Marshal(err)
			for _, view := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err), string(serialized)} {
				if strings.Contains(view, key) || strings.Contains(view, echo) {
					t.Fatal("credential retained")
				}
			}
		}
	}
}

func TestHTTPServerReflectsAuthorization(t *testing.T) {
	const key = "synthetic-reflection-98"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		w.Header().Set("Authorization", auth)
		w.Header().Set("X-Echo", auth)
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "INVALID_API_KEY", "message": auth}})
	}))
	defer srv.Close()
	client, err := NewClient(key, WithBaseURLs(srv.URL, srv.URL), WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_, err = client.GetInfo(context.Background())
	var api *ApiError
	if !errors.As(err, &api) {
		t.Fatal("expected API error")
	}
	if !strings.Contains(api.SensitiveRawBody(), key) {
		t.Fatal("original body lost")
	}
	if api.Headers["authorization"] != "[REDACTED]" {
		t.Fatal("auth header retained")
	}
	raw, _ := json.Marshal(err)
	for _, view := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err), string(raw)} {
		if strings.Contains(view, key) {
			t.Fatal("credential retained")
		}
	}
}
