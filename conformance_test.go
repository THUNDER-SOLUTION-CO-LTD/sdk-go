package thunder

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	ID, API, Operation, Conformance string
	Call                            struct {
		Args          map[string]json.RawMessage
		Operation     string
		ClientOptions struct{ MaxRetries int }
	}
	Request struct {
		Method, Path, Auth, ContentType string
		Query                           map[string]string
		Body                            json.RawMessage
		Multipart                       *struct {
			Fields map[string]string
			Files  []struct{ Field, Filename, ContentType, SHA256 string }
		}
	}
	Response step
	Expect   struct {
		Model, Result, Error string
		ErrorValue           json.RawMessage `json:"-"`
		Requests             int
		MinDelaySeconds      float64
	}
	Steps []step
}
type step struct {
	Status         int
	Headers        map[string]string
	Body           json.RawMessage
	TransportError string
}

// Expect.error is either a string (behaviour) or an object (response).
func loadFixture(b []byte) (fixture, error) {
	var f fixture
	var root map[string]json.RawMessage
	if err := json.Unmarshal(b, &root); err != nil {
		return f, err
	}
	var expect map[string]json.RawMessage
	_ = json.Unmarshal(root["expect"], &expect)
	delete(root, "expect")
	clean, _ := json.Marshal(root)
	if err := json.Unmarshal(clean, &f); err != nil {
		return f, err
	}
	var errorName string
	_ = json.Unmarshal(expect["error"], &errorName)
	delete(expect, "error")
	x, _ := json.Marshal(expect)
	_ = json.Unmarshal(x, &f.Expect)
	var original map[string]json.RawMessage
	_ = json.Unmarshal(b, &original)
	var originalExpect map[string]json.RawMessage
	_ = json.Unmarshal(original["expect"], &originalExpect)
	f.Expect.ErrorValue = originalExpect["error"]
	f.Expect.Error = errorName
	return f, nil
}
func responseStep(s step) *http.Response {
	h := http.Header{}
	for k, v := range s.Headers {
		h.Set(k, v)
	}
	body := s.Body
	var non map[string]string
	if json.Unmarshal(body, &non) == nil && non["_nonJson"] != "" {
		body = []byte(non["_nonJson"])
	}
	return &http.Response{StatusCode: s.Status, Header: h, Body: io.NopCloser(bytes.NewReader(body))}
}
func fixtureDecode[T any](value json.RawMessage) *T {
	if len(value) == 0 {
		return nil
	}
	var out T
	if err := json.Unmarshal(value, &out); err != nil {
		panic(err)
	}
	return &out
}
func valueArg(args map[string]json.RawMessage, key string) string {
	var v string
	_ = json.Unmarshal(args[key], &v)
	return v
}
func fixtureInvoke(ctx context.Context, c *Client, operation string, args map[string]json.RawMessage) (any, error) {
	opts := args
	if raw, ok := args["options"]; ok {
		_ = json.Unmarshal(raw, &opts)
	}
	optJSON, _ := json.Marshal(opts)
	file := func() any {
		b, e := os.ReadFile(filepath.Join(fixturePath(""), valueArg(args, "file")))
		if e != nil {
			panic(e)
		}
		return b
	}
	id := 0
	_ = json.Unmarshal(args["id"], &id)
	switch operation {
	case "getInfo":
		return c.GetInfo(ctx)
	case "banks.list":
		return c.Banks.List(ctx)
	case "bankAccounts.list":
		return c.BankAccounts.List(ctx, fixtureDecode[ListBankAccountsParams](optJSON))
	case "bankAccounts.listAll":
		return c.BankAccounts.ListAll(ctx, fixtureDecode[ListBankAccountsParams](optJSON))
	case "bankAccounts.get":
		return c.BankAccounts.Get(ctx, id)
	case "bankAccounts.create":
		return c.BankAccounts.Create(ctx, *fixtureDecode[CreateBankAccountParams](optJSON))
	case "bankAccounts.update":
		return c.BankAccounts.Update(ctx, id, *fixtureDecode[UpdateBankAccountParams](optJSON))
	case "bankAccounts.link":
		return c.BankAccounts.Link(ctx, id)
	case "bankAccounts.unlink":
		return c.BankAccounts.Unlink(ctx, id)
	case "jobs.get":
		return c.Jobs.Get(ctx, valueArg(args, "jobId"))
	case "jobs.submitBank":
		return c.Jobs.SubmitBank(ctx, valueArg(args, "payload"), fixtureDecode[AsyncVerifyOptions](optJSON))
	case "jobs.submitBankBatch":
		return c.Jobs.SubmitBankBatch(ctx, *fixtureDecode[[]BatchSlip](args["slips"]), fixtureDecode[BatchOptions](optJSON))
	case "v1.getMe":
		return c.V1.GetMe(ctx)
	case "v1.generateQr":
		return c.V1.GenerateQR(ctx, *fixtureDecode[QRRequest](optJSON))
	case "verify.bankByPayload":
		return c.Verify.BankByPayload(ctx, valueArg(args, "payload"), fixtureDecode[VerifyOptions](optJSON))
	case "verify.bankByImage":
		return c.Verify.BankByImage(ctx, file(), fixtureDecode[VerifyOptions](optJSON))
	case "verify.bankByBase64":
		return c.Verify.BankByBase64(ctx, valueArg(args, "base64"), fixtureDecode[VerifyOptions](optJSON))
	case "verify.bankByUrl":
		return c.Verify.BankByURL(ctx, valueArg(args, "url"), fixtureDecode[VerifyOptions](optJSON))
	case "verify.truewalletByImage":
		return c.Verify.TruewalletByImage(ctx, file(), fixtureDecode[VerifyOptions](optJSON))
	case "verify.truewalletByBase64":
		return c.Verify.TruewalletByBase64(ctx, valueArg(args, "base64"), fixtureDecode[VerifyOptions](optJSON))
	case "verify.truewalletByUrl":
		return c.Verify.TruewalletByURL(ctx, valueArg(args, "url"), fixtureDecode[VerifyOptions](optJSON))
	case "v1.verifyBankByPayload":
		return c.V1.VerifyBankByPayload(ctx, valueArg(args, "payload"), fixtureDecode[V1VerifyOptions](optJSON))
	case "v1.verifyBankByImage":
		return c.V1.VerifyBankByImage(ctx, file(), fixtureDecode[V1VerifyOptions](optJSON))
	case "v1.verifyBankByBase64":
		return c.V1.VerifyBankByBase64(ctx, valueArg(args, "base64"), fixtureDecode[V1VerifyOptions](optJSON))
	case "v1.verifyBankByUrl":
		return c.V1.VerifyBankByURL(ctx, valueArg(args, "url"), fixtureDecode[V1VerifyOptions](optJSON))
	case "v1.verifyTruemoneyByImage":
		return c.V1.VerifyTruemoneyByImage(ctx, file(), fixtureDecode[V1VerifyOptions](optJSON))
	}
	return nil, fmt.Errorf("unknown fixture operation %s", operation)
}

var fixtureModels = map[string]reflect.Type{
	"Info":                    reflect.TypeOf(Info{}),
	"InfoApplication":         reflect.TypeOf(InfoApplication{}),
	"AutoRenew":               reflect.TypeOf(AutoRenew{}),
	"ApplicationQuota":        reflect.TypeOf(ApplicationQuota{}),
	"InfoBranch":              reflect.TypeOf(InfoBranch{}),
	"BranchQuota":             reflect.TypeOf(BranchQuota{}),
	"InfoAccount":             reflect.TypeOf(InfoAccount{}),
	"InfoProduct":             reflect.TypeOf(InfoProduct{}),
	"BankSlipResult":          reflect.TypeOf(BankSlipResult{}),
	"TrueWalletResult":        reflect.TypeOf(TrueWalletResult{}),
	"BankSlip":                reflect.TypeOf(BankSlip{}),
	"SlipAmount":              reflect.TypeOf(SlipAmount{}),
	"LocalAmount":             reflect.TypeOf(LocalAmount{}),
	"SlipReceiver":            reflect.TypeOf(SlipReceiver{}),
	"SlipReceiverBank":        reflect.TypeOf(SlipReceiverBank{}),
	"SlipReceiverAccount":     reflect.TypeOf(SlipReceiverAccount{}),
	"SlipReceiverAccountName": reflect.TypeOf(SlipReceiverAccountName{}),
	"SlipParty":               reflect.TypeOf(SlipParty{}),
	"SlipBank":                reflect.TypeOf(SlipBank{}),
	"SlipAccount":             reflect.TypeOf(SlipAccount{}),
	"AccountName":             reflect.TypeOf(AccountName{}),
	"AccountNumber":           reflect.TypeOf(AccountNumber{}),
	"MatchedAccount":          reflect.TypeOf(MatchedAccount{}),
	"MatchedBank":             reflect.TypeOf(MatchedBank{}),
	"TrueWalletSlip":          reflect.TypeOf(TrueWalletSlip{}),
	"TrueWalletSender":        reflect.TypeOf(TrueWalletSender{}),
	"TrueWalletReceiver":      reflect.TypeOf(TrueWalletReceiver{}),
	"Bank":                    reflect.TypeOf(Bank{}),
	"ExtraVerifyOption":       reflect.TypeOf(ExtraVerifyOption{}),
	"BankAccount":             reflect.TypeOf(BankAccount{}),
	"BankAccountPage":         reflect.TypeOf(BankAccountPage{}),
	"LinkResult":              reflect.TypeOf(LinkResult{}),
	"UnlinkResult":            reflect.TypeOf(UnlinkResult{}),
	"JobSubmission":           reflect.TypeOf(JobSubmission{}),
	"BatchSubmission":         reflect.TypeOf(BatchSubmission{}),
	"BatchJob":                reflect.TypeOf(BatchJob{}),
	"Job":                     reflect.TypeOf(Job{}),
	"JobResult":               reflect.TypeOf(JobResult{}),
	"JobError":                reflect.TypeOf(JobError{}),
	"CallbackDelivery":        reflect.TypeOf(CallbackDelivery{}),
	"WebhookEvent":            reflect.TypeOf(WebhookEvent{}),
	"WebhookError":            reflect.TypeOf(WebhookError{}),
	"V1Me":                    reflect.TypeOf(V1Me{}),
	"QrCode":                  reflect.TypeOf(QRCode{}),
	"VerifyOptions":           reflect.TypeOf(VerifyOptions{}),
	"CreateBankAccountParams": reflect.TypeOf(CreateBankAccountParams{}),
	"UpdateBankAccountParams": reflect.TypeOf(UpdateBankAccountParams{}),
	"BatchSlip":               reflect.TypeOf(BatchSlip{}),
	"AsyncVerifyOptions":      reflect.TypeOf(AsyncVerifyOptions{}),
	"BatchOptions":            reflect.TypeOf(BatchOptions{}),
	"V1VerifyOptions":         reflect.TypeOf(V1VerifyOptions{}),
	"ListBankAccountsParams":  reflect.TypeOf(ListBankAccountsParams{}),
	"QrRequest":               reflect.TypeOf(QRRequest{}),
	"RateLimit":               reflect.TypeOf(RateLimit{}),
	"ResponseMeta":            reflect.TypeOf(ResponseMeta{}),
	"BankList":                reflect.TypeOf(BankList{}),
}

func jsonEqual(t *testing.T, a, b []byte) {
	t.Helper()
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		t.Fatal("invalid JSON comparison")
	}
	if !reflect.DeepEqual(x, y) {
		t.Fatalf("JSON differs:\n%s\n%s", a, b)
	}
}
func checkFixtureRequest(t *testing.T, r *http.Request, f fixture) {
	t.Helper()
	exp := f.Request
	if r.Method != exp.Method || r.URL.Path != "/"+f.API+exp.Path {
		t.Fatalf("request path/method %s %s", r.Method, r.URL.Path)
	}
	query := map[string]string{}
	for k, v := range r.URL.Query() {
		query[k] = v[0]
	}
	if len(query) != len(exp.Query) {
		t.Fatal("query count differs")
	}
	for k, v := range exp.Query {
		if query[k] != v {
			t.Fatalf("query %s", k)
		}
	}
	if r.Header.Get("Authorization") != "Bearer fixture-secret" {
		t.Fatal("bearer missing")
	}
	if exp.ContentType != "" && strings.Split(r.Header.Get("Content-Type"), ";")[0] != exp.ContentType {
		t.Fatal("content type differs")
	}
	if len(exp.Body) > 0 {
		body, _ := io.ReadAll(r.Body)
		jsonEqual(t, body, exp.Body)
	}
	if exp.Multipart != nil {
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		fields := map[string]string{}
		i := 0
		for {
			part, e := reader.NextPart()
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(e)
			}
			data, _ := io.ReadAll(part)
			if part.FileName() == "" {
				fields[part.FormName()] = string(data)
				continue
			}
			if i >= len(exp.Multipart.Files) {
				t.Fatal("extra file")
			}
			file := exp.Multipart.Files[i]
			i++
			hash := sha256.Sum256(data)
			if part.FormName() != file.Field || part.FileName() != file.Filename || part.Header.Get("Content-Type") != file.ContentType || hex.EncodeToString(hash[:]) != file.SHA256 {
				t.Fatal("multipart file differs")
			}
		}
		if i != len(exp.Multipart.Files) || !reflect.DeepEqual(fields, exp.Multipart.Fields) {
			t.Fatal("multipart fields differ")
		}
	}
}
func TestSharedConformance(t *testing.T) {
	paths := []string{}
	_ = filepath.WalkDir(fixturePath(""), func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".json") {
			paths = append(paths, path)
		}
		return err
	})
	if len(paths) == 0 {
		t.Skip("fixture directory unavailable in prepared package")
	}
	for _, path := range paths {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		f, e := loadFixture(b)
		if e != nil {
			t.Fatal(e)
		}
		if f.Response.Status == 0 && f.Conformance != "behaviour" {
			continue
		}
		t.Run(f.ID, func(t *testing.T) {
			if f.Conformance == "behaviour" {
				testBehaviourFixture(t, f)
				return
			}
			raw := responseStep(f.Response)
			data, _ := io.ReadAll(raw.Body)
			if len(f.Expect.ErrorValue) > 0 {
				var want struct {
					Class, Code, Message, RequestID string
					Status                          int
					RetryAfter                      *float64
				}
				if e := json.Unmarshal(f.Expect.ErrorValue, &want); e != nil {
					t.Fatal(e)
				}
				err := decodeAPIError(f.Response.Status, data, raw.Header, "fixture-secret")
				if reflect.TypeOf(err).Elem().Name() != want.Class {
					t.Fatalf("class %T want %s", err, want.Class)
				}
				var base *APIError
				if !errors.As(err, &base) {
					t.Fatal("errors.As base failed")
				}
				if optionalString(base.Code) != want.Code || base.Status != want.Status || base.Message != want.Message || optionalString(base.RequestID) != want.RequestID || base.RawBody != string(data) {
					t.Fatalf("error metadata differs: %+v", base)
				}
				if want.RetryAfter != nil && (base.RetryAfter == nil || *base.RetryAfter != *want.RetryAfter) {
					t.Fatal("retry after differs")
				}
			} else if f.Expect.Model != "" {
				typ, ok := fixtureModels[f.Expect.Model]
				if !ok {
					t.Fatalf("unknown model %s", f.Expect.Model)
				}
				value := reflect.New(typ).Interface()
				var envelope struct{ Data json.RawMessage }
				_ = json.Unmarshal(data, &envelope)
				if e := json.Unmarshal(envelope.Data, value); e != nil {
					t.Fatal(e)
				}
				got, e := json.Marshal(value)
				if e != nil {
					t.Fatal(e)
				}
				jsonEqual(t, got, envelope.Data)
			}
			if f.Conformance == "call" && f.Request.Auth != "none" {
				requests := 0
				c, e := NewClient("fixture-secret", WithBaseURLs("https://api.test/v2", "https://api.test/v1"), WithRetry(0), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					requests++
					checkFixtureRequest(t, r, f)
					return responseStep(f.Response), nil
				})}))
				if e != nil {
					t.Fatal(e)
				}
				defer c.Close()
				value, err := fixtureInvoke(context.Background(), c, f.Operation, f.Call.Args)
				if requests != 1 {
					t.Fatalf("requests %d", requests)
				}
				if f.Response.Status < 300 {
					if err != nil {
						t.Fatal(err)
					}
					got, _ := json.Marshal(value)
					var env struct{ Data json.RawMessage }
					_ = json.Unmarshal(data, &env)
					jsonEqual(t, got, env.Data)
				} else if err == nil {
					t.Fatal("expected error")
				}
			}
		})
	}
}
func testBehaviourFixture(t *testing.T, f fixture) {
	n := 0
	delays := []time.Duration{}
	client, e := NewClient("fixture-secret", WithBaseURLs("https://api.test/v2", "https://api.test/v1"), WithRetry(f.Call.ClientOptions.MaxRetries), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if n >= len(f.Steps) {
			t.Fatal("too many requests")
		}
		step := f.Steps[n]
		n++
		switch step.TransportError {
		case "connect":
			return nil, &net.OpError{Op: "dial", Err: errors.New("refused")}
		case "timeout":
			return nil, context.DeadlineExceeded
		case "reset":
			return nil, io.ErrUnexpectedEOF
		}
		return responseStep(step), nil
	})}))
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	for _, transport := range []*transport{client.v1, client.v2} {
		transport.sleep = func(_ context.Context, d time.Duration) error { delays = append(delays, d); return nil }
	}
	_, err := fixtureInvoke(context.Background(), client, f.Call.Operation, f.Call.Args)
	if n != f.Expect.Requests {
		t.Fatalf("requests %d want %d", n, f.Expect.Requests)
	}
	if f.Expect.Error != "" {
		if err == nil || reflect.TypeOf(err).Elem().Name() != f.Expect.Error {
			t.Fatalf("got %T want %s", err, f.Expect.Error)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	if f.Expect.MinDelaySeconds > 0 && (len(delays) == 0 || delays[0].Seconds() < f.Expect.MinDelaySeconds) {
		t.Fatal("retry delay too short")
	}
}
