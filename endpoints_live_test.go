package thunder

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestLiveEndpoints is explicitly enabled with caller-provided API URLs.
func TestLiveEndpoints(t *testing.T) {
	if os.Getenv("THUNDER_ENDPOINT_INTEGRATION") != "1" {
		t.Skip("set THUNDER_INTEGRATION=1 for authorized integration endpoint checks")
	}
	roots := struct{ URLs map[string]string }{map[string]string{"v2": os.Getenv("THUNDER_V2_BASE_URL"), "v1": os.Getenv("THUNDER_V1_BASE_URL")}}
	for _, base := range roots.URLs {
		if !strings.HasPrefix(base, "https://") {
			t.Fatal("Explicit HTTPS base URLs required")
		}
	}
	key, e := os.ReadFile(os.Getenv("THUNDER_API_KEY_FILE"))
	if e != nil {
		t.Fatal("Missing key file")
	}
	c, e := NewClient(strings.TrimSpace(string(key)), WithBaseURLs(roots.URLs["v2"], roots.URLs["v1"]), WithMaxRetries(0), WithTimeout(20*time.Second))
	if e != nil {
		t.Fatal("Client configuration failed")
	}
	defer c.Close()
	fixtures := map[string]fixture{}
	_ = filepath.WalkDir(fixturePath(""), func(path string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() && strings.HasSuffix(path, ".json") {
			b, _ := os.ReadFile(path)
			f, e := loadFixture(b)
			if e == nil && f.Conformance == "call" && f.Response.Status >= 200 && f.Response.Status < 300 {
				if _, ok := fixtures[f.Operation]; !ok {
					fixtures[f.Operation] = f
				}
			}
		}
		return e
	})
	type receipt struct {
		Operation   string  `json:"operation"`
		Outcome     string  `json:"outcome"`
		Status      int     `json:"status,omitempty"`
		Code        *string `json:"code,omitempty"`
		LatencyMS   int64   `json:"latency_ms"`
		Note        string  `json:"note,omitempty"`
		IsDuplicate *bool   `json:"is_duplicate,omitempty"`
	}
	results := []receipt{}
	ops := []string{"getInfo", "banks.list", "bankAccounts.list", "bankAccounts.listAll", "verify.bankByUrl", "verify.truewalletByImage", "verify.truewalletByBase64", "verify.truewalletByUrl", "jobs.submitBank", "jobs.submitBankBatch", "jobs.get", "v1.getMe", "v1.verifyBankByUrl", "v1.verifyTruemoneyByImage", "v1.generateQr"}
	run := func(op string, fn func(context.Context) (any, error), note string) any {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		start := time.Now()
		result, err := fn(ctx)
		row := receipt{Operation: op, Outcome: "success", LatencyMS: time.Since(start).Milliseconds(), Note: note}
		if slip, ok := result.(*BankSlipResult); ok && err == nil {
			if duplicate, present := slip.IsDuplicate.Value(); present {
				row.IsDuplicate = &duplicate
			}
		}
		if err == nil && result != nil {
			value := reflect.Indirect(reflect.ValueOf(result))
			if value.Kind() == reflect.Struct {
				meta := value.FieldByName("Meta")
				if meta.IsValid() {
					row.Status = meta.Interface().(ResponseMeta).Status
				}
			}
		}

		if err != nil {
			var api *ApiError
			if errors.As(err, &api) {
				row.Outcome = "error-path"
				row.Status = api.Status
				row.Code = api.Code
				if api.Status >= 500 {
					row.Outcome = "dependency-failure"
				}
			} else {
				row.Outcome = fmt.Sprintf("failure:%T", err)
			}
		}
		results = append(results, row)
		t.Log(op, row.Outcome, row.Status, row.LatencyMS)
		return result
	}
	sampleDir := os.Getenv("THUNDER_SAMPLES_DIR")
	payloadFile, err := os.ReadFile(filepath.Join(sampleDir, "payloads.txt"))
	if err != nil {
		t.Fatal("Cached sample file required")
	}
	payloads := strings.Split(strings.TrimSpace(string(payloadFile)), "\n")
	if len(payloads) != 4 {
		t.Fatal("Expected four cached samples")
	}
	for i, payload := range payloads {
		image, err := os.ReadFile(filepath.Join(sampleDir, fmt.Sprintf("qr-%d.png", i)))
		if err != nil {
			t.Fatal("Cached sample image required")
		}
		encoded := base64.StdEncoding.EncodeToString(image)
		calls := []struct {
			name string
			call func(context.Context) (any, error)
		}{
			{"verify.bankByPayload", func(ctx context.Context) (any, error) {
				return c.Verify.BankByPayload(ctx, payload, &VerifyOptions{CheckDuplicate: Some(true)})
			}},
			{"verify.bankByImage", func(ctx context.Context) (any, error) {
				return c.Verify.BankByImage(ctx, image, &VerifyOptions{CheckDuplicate: Some(true)})
			}},
			{"verify.bankByBase64", func(ctx context.Context) (any, error) {
				return c.Verify.BankByBase64(ctx, encoded, &VerifyOptions{CheckDuplicate: Some(true)})
			}},
			{"v1.verifyBankByPayload", func(ctx context.Context) (any, error) { return c.V1.VerifyBankByPayload(ctx, payload, nil) }},
			{"v1.verifyBankByImage", func(ctx context.Context) (any, error) { return c.V1.VerifyBankByImage(ctx, image, nil) }},
			{"v1.verifyBankByBase64", func(ctx context.Context) (any, error) { return c.V1.VerifyBankByBase64(ctx, encoded, nil) }},
		}
		for _, call := range calls {
			run(fmt.Sprintf("%s.sample-%d", call.name, i), call.call, "Cached dev sample read from file")
		}
	}
	ctx := context.Background()
	var account *BankAccount
	for candidate, err := range c.BankAccounts.IterateAll(ctx, nil) {
		if err != nil {
			t.Fatal("Cannot list smoke account")
		}
		if candidate.NameTh == "SDK smoke" || candidate.NameEn == "SDK smoke" {
			item := candidate
			account = &item
			break
		}
	}
	if account == nil {
		t.Fatal("Existing SDK smoke account required; creation disabled")
	} else {
		results = append(results, receipt{Operation: "bankAccounts.create", Outcome: "reused", Note: "SDK smoke already exists; no second account created as ordered"})
	}
	if account == nil {
		t.Fatal("Cannot provision smoke account")
	}
	run("bankAccounts.get", func(ctx context.Context) (any, error) { return c.BankAccounts.Get(ctx, account.ID) }, "")
	run("bankAccounts.update", func(ctx context.Context) (any, error) {
		return c.BankAccounts.Update(ctx, account.ID, UpdateBankAccountParams{NameTh: Some(account.NameTh), NameEn: Some(account.NameEn)})
	}, "Same name values written back")
	linked := func() bool {
		found := false
		for candidate, err := range c.BankAccounts.Iterate(ctx, nil) {
			if err != nil {
				t.Fatal("Cannot read branch association")
			}
			if candidate.ID == account.ID {
				found = true
			}
		}
		return found
	}
	before := linked()
	// Restore even when a test assertion fails after a link operation.
	restore := func() {
		if before {
			if _, err := c.BankAccounts.Link(ctx, account.ID); err != nil {
				t.Error("Cannot restore link")
			}
		} else if linked() {
			if _, err := c.BankAccounts.Unlink(ctx, account.ID); err != nil {
				t.Error("Cannot restore unlink")
			}
		}
	}
	func() {
		defer restore()
		run("bankAccounts.link", func(ctx context.Context) (any, error) { return c.BankAccounts.Link(ctx, account.ID) }, "")
		run("bankAccounts.unlink", func(ctx context.Context) (any, error) { return c.BankAccounts.Unlink(ctx, account.ID) }, "")
	}()
	if linked() != before {
		t.Fatal("Branch association changed")
	}
	results = append(results, receipt{Operation: "bankAccounts.restore", Outcome: "success", Note: "Initial branch link state restored"})

	for _, op := range ops {
		f, ok := fixtures[op]
		if !ok {
			t.Fatalf("Missing fixture for %s", op)
		}
		args := f.Call.Args
		note := "Shared synthetic dev fixture; endpoint response and typed decode checked"
		if strings.HasPrefix(op, "jobs.submit") {
			args["callbackUrl"], _ = json.Marshal(roots.URLs["v2"] + "/health")
			delete(args, "options")
			args["payload"], _ = json.Marshal(payloads[0])
			args["checkDuplicate"] = json.RawMessage(`false`)
			if op == "jobs.submitBankBatch" {
				args["slips"], _ = json.Marshal([]BatchSlip{{Payload: payloads[0]}})
			}
		}
		run(op, func(ctx context.Context) (any, error) { return fixtureInvoke(ctx, c, op, args) }, note)
	}
	run("bankAccounts.iterate", func(ctx context.Context) (any, error) {
		n := 0
		for _, err := range c.BankAccounts.Iterate(ctx, &ListBankAccountsParams{Limit: Some(2)}) {
			if err != nil {
				return nil, err
			}
			n++
		}
		return n, nil
	}, "")
	run("bankAccounts.iterateAll", func(ctx context.Context) (any, error) {
		n := 0
		for _, err := range c.BankAccounts.IterateAll(ctx, &ListBankAccountsParams{Limit: Some(2)}) {
			if err != nil {
				return nil, err
			}
			n++
		}
		return n, nil
	}, "")
	f := fixtures["jobs.submitBankBatch"]
	f.Call.Args["slips"], _ = json.Marshal([]BatchSlip{{Payload: payloads[0]}})
	f.Call.Args["callbackUrl"], _ = json.Marshal(roots.URLs["v2"] + "/health")
	b, _ := json.Marshal(f.Call.Args)
	var options BatchOptions
	_ = json.Unmarshal(b, &options)
	var slips []BatchSlip
	_ = json.Unmarshal(f.Call.Args["slips"], &slips)
	run("jobs.submitBankMany", func(ctx context.Context) (any, error) { return c.Jobs.SubmitBankMany(ctx, slips, &options) }, "One small live chunk; >100 chunking tested with mocks")
	jobFixture, err := os.ReadFile(fixturePath("v2/jobs/get/done.json"))
	if err != nil {
		t.Fatal(err)
	}
	jf, _ := loadFixture(jobFixture)
	run("jobs.waitForJob", func(ctx context.Context) (any, error) {
		return c.Jobs.WaitForJob(ctx, valueArg(jf.Call.Args, "jobId"), &WaitOptions{Interval: time.Second, Timeout: 5 * time.Second})
	}, "Existing completed fixture job avoids relying on worker timing")
	for _, path := range []string{"promptpay-biller.json", "promptpay-biller-no-ref1.json", "kshop.json", "kshop-kps004.json", "mae-manee.json", "tungngern.json", "bill-payment.json", "bill-payment-no-ref1.json"} {
		b, e := os.ReadFile(filepath.Join(fixturePath("v1/qr"), path))
		if e != nil {
			t.Fatal(e)
		}
		f, e := loadFixture(b)
		if e != nil {
			t.Fatal(e)
		}
		run("v1.generateQr."+path, func(ctx context.Context) (any, error) { return fixtureInvoke(ctx, c, f.Operation, f.Call.Args) }, "")
	}
	b, _ = json.MarshalIndent(results, "", "  ")
	path := os.Getenv("THUNDER_INTEGRATION_RECEIPT")
	if path == "" {
		t.Fatal("THUNDER_INTEGRATION_RECEIPT required")
	}
	if os.WriteFile(path, append(b, '\n'), 0600) != nil {
		t.Fatal("Cannot write receipt")
	}
	for _, r := range results {
		if r.Outcome != "success" && r.Outcome != "reused" && !strings.Contains(r.Operation, "truewallet") && !strings.Contains(r.Operation, "Truemoney") && !strings.Contains(r.Operation, "ByUrl") {
			t.Errorf("%s %s", r.Operation, r.Outcome)
		}
	}
}
