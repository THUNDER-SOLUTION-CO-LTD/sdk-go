package thunder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactionEncodedHeadersAndBody(t *testing.T) {
	key := "test-SYNTHETIC/+=secret"
	forms := []string{key, url.PathEscape(key)}
	full, mixed, percent := "", "", ""
	for i, c := range key {
		full += fmt.Sprintf("\\u%04x", c)
		if i%2 == 0 {
			mixed += string(c)
			percent += string(c)
		} else {
			mixed += fmt.Sprintf("\\u%04X", c)
			percent += fmt.Sprintf("%%%02x", c)
		}
	}
	forms = append(forms, full, mixed, percent)
	for i, form := range forms {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if got := redactHeader("X-Echo", form, key); got != "[REDACTED]" {
				t.Errorf("header not removed: %q", got)
			}
			data, _ := json.Marshal(map[string]any{"message": form, "details": map[string]string{form: form}})
			err := decodeAPIError(400, data, http.Header{}, key)
			var body map[string]any
			base := err.(*InvalidRequestError)
			if json.Unmarshal([]byte(base.RawBody), &body) != nil {
				t.Fatal("invalid public JSON")
			}
			encoded, _ := json.Marshal(body)
			if strings.Contains(string(encoded), form) || strings.Contains(string(encoded), key) {
				t.Error("body contains encoded secret")
			}
			if base.SensitiveRawBody() != string(data) {
				t.Error("sensitive body lost")
			}
		})
	}
}

func TestRedactionEveryBilledOperationSendsOnce(t *testing.T) {
	calls := map[string]fixture{}
	err := filepath.WalkDir(fixturePath(""), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		b, _ := os.ReadFile(path)
		f, e := loadFixture(b)
		if e == nil && f.Conformance == "call" && (strings.HasPrefix(f.Operation, "verify.") || strings.HasPrefix(f.Operation, "v1.verify") || strings.HasPrefix(f.Operation, "jobs.submit")) {
			if _, ok := calls[f.Operation]; !ok {
				calls[f.Operation] = f
			}
		}
		return nil
	})
	if err != nil {
		t.Skip("fixtures not included in package")
	}
	if len(calls) < 14 {
		t.Fatalf("incomplete operation inventory: %d", len(calls))
	}
	calls["jobs.submitBankMany"] = calls["jobs.submitBankBatch"]
	for op, f := range calls {
		for _, failure := range []string{"read", "500"} {
			t.Run(op+"/"+failure, func(t *testing.T) {
				sends := 0
				c, e := NewClient("synthetic", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					sends++
					if failure == "read" {
						return nil, context.DeadlineExceeded
					}
					return &http.Response{StatusCode: 500, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}"))}, nil
				})}))
				if e != nil {
					t.Fatal(e)
				}
				defer c.Close()
				if op == "jobs.submitBankMany" {
					_, e = c.Jobs.SubmitBankMany(context.Background(), []BatchSlip{{Payload: "synthetic"}}, nil)
				} else {
					_, e = fixtureInvoke(context.Background(), c, op, f.Call.Args)
				}
				if e == nil || sends != 1 {
					t.Fatalf("error=%v sends=%d", e, sends)
				}
			})
		}
	}
}

type redactionSecretTransport struct {
	Message, Code string
	Headers       map[string]string
}

func (e *redactionSecretTransport) Error() string { return e.Message }

func TestRedactionTransportAndSuccessMetadata(t *testing.T) {
	key := "test-SYNTHETIC/+=secret"
	escaped := ""
	for _, c := range key {
		escaped += fmt.Sprintf("\\u%04x", c)
	}
	for _, form := range []string{key, url.PathEscape(key), escaped} {
		original := &redactionSecretTransport{Message: form, Code: form, Headers: map[string]string{"authorization": key}}
		c, e := NewClient(key, WithMaxRetries(0), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, original })}))
		if e != nil {
			t.Fatal(e)
		}
		_, e = c.Verify.BankByPayload(context.Background(), "synthetic", nil)
		c.Close()
		var source *redactionSecretTransport
		if errors.As(e, &source) {
			t.Fatal("original reachable")
		}
		for cursor := e; cursor != nil; cursor = errors.Unwrap(cursor) {
			wire, _ := json.Marshal(cursor)
			views := []string{fmt.Sprintf("%v", cursor), fmt.Sprintf("%+v", cursor), fmt.Sprintf("%#v", cursor), string(wire)}
			for _, view := range views {
				if strings.Contains(decodedText(view), key) {
					t.Fatal("transport secret recoverable")
				}
			}
		}
		b, err := os.ReadFile(fixturePath("v2/info/success.json"))
		if err != nil {
			continue
		}
		f, err := loadFixture(b)
		if err != nil {
			t.Fatal(err)
		}
		c, e = NewClient(key, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			response := responseStep(f.Response)
			response.Header.Set("X-Echo", form)
			return response, nil
		})}))
		if e != nil {
			t.Fatal(e)
		}
		result, e := c.GetInfo(context.Background())
		c.Close()
		if e != nil {
			t.Fatal(e)
		}
		if result.Meta.Headers["x-echo"] != "[REDACTED]" {
			t.Fatal("success header leak")
		}
	}
}

func TestRedactionAllBodyAndHeaderSurfaces(t *testing.T) {
	key := "test-SYNTHETIC/+=secret"
	full, upper, percentLower, percentUpper, mixed := "", "", "", "", ""
	for i, c := range key {
		full += fmt.Sprintf("\\u%04x", c)
		upper += fmt.Sprintf("\\u%04X", c)
		percentLower += fmt.Sprintf("%%%02x", c)
		percentUpper += fmt.Sprintf("%%%02X", c)
		if i%2 == 0 {
			mixed += string(c)
		} else {
			mixed += fmt.Sprintf("\\u%04x", c)
		}
	}
	for i, form := range []string{key, full, upper, percentLower, percentUpper, mixed, strings.ReplaceAll(key, "/", "\\/")} {
		for _, v2 := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/v2=%v", i, v2), func(t *testing.T) {
				value := map[string]any{"message": form, "details": map[string]string{form: form}}
				body := value
				if v2 {
					body = map[string]any{"error": value, "details": value["details"]}
				}
				data, _ := json.Marshal(body)
				err := decodeAPIError(400, data, http.Header{"X-Echo": {form}, "Authorization": {"anything"}}, key)
				var base *ApiError
				if !errors.As(err, &base) {
					t.Fatal("missing base")
				}
				wire, _ := json.Marshal(base)
				for _, text := range []string{base.RawBody, string(base.Details), base.Headers["x-echo"], fmt.Sprintf("%+v", base), fmt.Sprintf("%#v", base), string(wire)} {
					if strings.Contains(decodedText(decodedText(text)), key) {
						t.Fatal("recoverable key")
					}
				}
				if base.Headers["x-echo"] != "[REDACTED]" || base.Headers["authorization"] != "[REDACTED]" {
					t.Fatal("unsafe header")
				}
				if base.SensitiveRawBody() != string(data) {
					t.Fatal("lost original")
				}
				plain := "upstream says " + form
				err = decodeAPIError(500, []byte(plain), http.Header{}, key)
				if !errors.As(err, &base) {
					t.Fatal("missing base")
				}
				if strings.Contains(decodedText(base.RawBody), key) {
					t.Fatal("non-JSON leak")
				}
			})
		}
	}
}
