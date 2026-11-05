package thunder

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSharedWebhookContract(t *testing.T) {
	raw, e := os.ReadFile(fixturePath("webhook/signatures.json"))
	if os.IsNotExist(e) {
		t.Skip("fixtures unavailable in prepared module")
	}
	if e != nil {
		t.Fatal(e)
	}
	var signatures struct {
		Secret string
		Cases  []struct {
			ID, Body  string
			Signature *string
			Valid     bool
		}
	}
	if json.Unmarshal(raw, &signatures) != nil {
		t.Fatal("signature fixture")
	}
	for _, c := range signatures.Cases {
		body, e := os.ReadFile(filepath.Join(fixturePath(""), c.Body))
		if e != nil {
			t.Fatal(e)
		}
		sig := ""
		if c.Signature != nil {
			sig = *c.Signature
		}
		if VerifyWebhookSignature(body, sig, signatures.Secret) != c.Valid {
			t.Fatal(c.ID)
		}
		if !c.Valid {
			_, err := ConstructWebhookEvent(body, sig, signatures.Secret)
			var bad *WebhookSignatureError
			if !errors.As(err, &bad) {
				t.Fatal("missing signature error")
			}
		}
	}
	raw, e = os.ReadFile(fixturePath("webhook/events.json"))
	if e != nil {
		t.Fatal(e)
	}
	var events struct {
		Cases []struct {
			Body   string
			Expect struct{ JSON json.RawMessage }
		}
	}
	_ = json.Unmarshal(raw, &events)
	for _, c := range events.Cases {
		body, _ := os.ReadFile(filepath.Join(fixturePath(""), c.Body))
		event, e := ParseWebhookEvent(body)
		if e != nil {
			t.Fatal(e)
		}
		got, _ := json.Marshal(event)
		jsonEqual(t, got, c.Expect.JSON)
	}
}
func TestBatchPartialAndBounds(t *testing.T) {
	calls := []int{}
	c, e := NewClient("fixture-secret", WithMaxRetries(0), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct{ Slips []BatchSlip }
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls = append(calls, len(body.Slips))
		if len(calls) == 2 {
			return &http.Response{StatusCode: 400, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":"VALIDATION_ERROR","message":"no"}}`))}, nil
		}
		return &http.Response{StatusCode: 202, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":{"batchId":"one","jobs":[]}}`))}, nil
	})}))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	slips := make([]BatchSlip, 201)
	for i := range slips {
		slips[i] = BatchSlip{Payload: "a"}
	}
	_, e = c.Jobs.SubmitBankMany(context.Background(), slips, nil)
	var partial *BatchSubmitError
	if !errors.As(e, &partial) || partial.FailedChunk != 1 || len(partial.Submitted) != 1 || partial.Submitted[0].BatchID != "one" || len(calls) != 2 || calls[0] != 100 || calls[1] != 100 {
		t.Fatal("partial receipts lost")
	}
	for _, input := range [][]BatchSlip{nil, slips} {
		_, err := c.Jobs.SubmitBankBatch(context.Background(), input, nil)
		var bad *InputError
		if !errors.As(err, &bad) {
			t.Fatal("batch bounds not checked")
		}
	}
}
func TestPaginationStartsRequestedPage(t *testing.T) {
	calls := 0
	c, e := NewClient("fixture-secret", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Query().Get("page") != "3" || r.URL.Query().Get("limit") != "2" {
			t.Fatal("wrong page")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":{"items":[],"total":100,"page":3,"limit":2}}`))}, nil
	})}))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	for _, err := range c.BankAccounts.Iterate(context.Background(), &ListBankAccountsParams{Page: Some(3), Limit: Some(2)}) {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("unexpected row")
	}
	if calls != 1 {
		t.Fatal("did not stop")
	}
}
func TestWaitDeadlineIncludesRetries(t *testing.T) {
	c, e := NewClient("fixture-secret", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Header: http.Header{"Retry-After": []string{"20"}}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	start := time.Now()
	_, e = c.Jobs.WaitForJob(context.Background(), "job", &WaitOptions{Timeout: 50 * time.Millisecond})
	var timeout *JobWaitTimeoutError
	if !errors.As(e, &timeout) || time.Since(start) > 300*time.Millisecond {
		t.Fatal("poll timeout not bounded")
	}
}
