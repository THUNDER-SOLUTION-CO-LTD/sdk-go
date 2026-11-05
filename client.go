package thunder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Client owns resource services; close it after use.
type Client struct {
	Verify       *VerifyService
	Banks        *BanksService
	BankAccounts *BankAccountsService
	Jobs         *JobsService
	V1           *V1Service
	v2, v1       *transport
}

// WithBaseURLs overrides v2 and v1 hosts independently.
func WithBaseURLs(v2, v1 string) Option {
	return func(t *transport) error { t.overrideV2 = v2; t.overrideV1 = v1; return nil }
}

// WithMaxRetries sets the retry budget; billed requests only retry connect failures and 429.
func WithMaxRetries(n int) Option { return WithRetry(n) }

// NewClient accepts an explicit key or THUNDER environment variables.
func NewClient(apiKey string, options ...Option) (*Client, error) {
	if apiKey == "" {
		apiKey = os.Getenv("THUNDER_API_KEY")
	}
	if apiKey == "" {
		if path := os.Getenv("THUNDER_API_KEY_FILE"); path != "" {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, inputError("INVALID_ARGUMENT", "Cannot read API key file")
			}
			apiKey = strings.TrimSpace(string(b))
		}
	}
	v2 := "https://api.thunder.in.th/v2"
	v1 := "https://api.thunder.in.th/v1"
	if v := os.Getenv("THUNDER_V2_BASE_URL"); v != "" {
		v2 = v
	}
	if v := os.Getenv("THUNDER_V1_BASE_URL"); v != "" {
		v1 = v
	}
	// Read URL overrides before constructing the transports.
	probe := &transport{}
	for _, opt := range options {
		if err := opt(probe); err != nil {
			return nil, inputError("INVALID_ARGUMENT", "Invalid client configuration")
		}
	}
	if probe.overrideV2 != "" {
		v2 = probe.overrideV2
	}
	if probe.overrideV1 != "" {
		v1 = probe.overrideV1
	}
	t2, err := newTransport(apiKey, v2, options...)
	if err != nil {
		return nil, inputError("INVALID_ARGUMENT", "Invalid client configuration")
	}
	t1, err := newTransport(apiKey, v1, options...)
	if err != nil {
		t2.close()
		return nil, inputError("INVALID_ARGUMENT", "Invalid client configuration")
	}
	agent := "thunder-slip-go"
	t1.agent = agent + "/1.0.0 go/" + strings.TrimPrefix(runtime.Version(), "go") // x-release-please-version
	t2.agent = t1.agent
	c := &Client{v2: t2, v1: t1}
	c.Verify = &VerifyService{t2}
	c.Banks = &BanksService{t2}
	c.BankAccounts = &BankAccountsService{t2}
	c.Jobs = &JobsService{t2}
	c.V1 = &V1Service{t1}
	return c, nil
}

// Close releases owned idle connections, leaving injected HTTP clients open.
func (c *Client) Close() { c.v1.close(); c.v2.close() }

// String returns a credential-free description.
func (*Client) String() string { return "Client{credentials:[REDACTED]}" }

// GoString returns a credential-free debug description.
func (c *Client) GoString() string { return c.String() }
func responseMeta(meta Metadata, message *string) ResponseMeta {
	h := map[string]string{}
	for k, v := range meta.Headers {
		h[strings.ToLower(k)] = strings.Join(v, ", ")
	}
	var id *string
	if v := h["x-request-id"]; v != "" {
		id = &v
	} else if v := h["cf-ray"]; v != "" {
		id = &v
	}
	return ResponseMeta{Status: meta.Status, RequestID: id, Message: message, RateLimit: parseRateLimit(meta.Headers), Headers: h}
}
func call[T any](ctx context.Context, t *transport, method, path string, body any, safe bool) (*T, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, inputError("INVALID_ARGUMENT", "Invalid JSON body")
		}
		reader = bytes.NewReader(b)
	}
	raw, meta, err := t.request(ctx, method, path, reader, safe)
	if err != nil {
		return nil, err
	}
	return decodeResponse[T](raw, meta)
}
func decodeResponse[T any](raw []byte, meta Metadata) (*T, error) {
	var envelope struct {
		Data    json.RawMessage `json:"data"`
		Message *string         `json:"message"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return nil, inputError("INVALID_ARGUMENT", "Invalid JSON response")
	}
	var result T
	if json.Unmarshal(envelope.Data, &result) != nil {
		return nil, inputError("INVALID_ARGUMENT", "Invalid response model")
	}
	attachMeta(&result, responseMeta(meta, envelope.Message))
	return &result, nil
}
func attachMeta(value any, meta ResponseMeta) {
	switch v := value.(type) {
	case *Info:
		v.Meta = meta
	case *BankSlipResult:
		v.Meta = meta
	case *TrueWalletResult:
		v.Meta = meta
	case *BankSlip:
		v.Meta = meta
	case *TrueWalletSlip:
		v.Meta = meta
	case *BankList:
		v.Meta = meta
	case *BankAccountPage:
		v.Meta = meta
	case *BankAccount:
		v.Meta = meta
	case *LinkResult:
		v.Meta = meta
	case *UnlinkResult:
		v.Meta = meta
	case *JobSubmission:
		v.Meta = meta
	case *BatchSubmission:
		v.Meta = meta
	case *Job:
		v.Meta = meta
	case *V1Me:
		v.Meta = meta
	case *QRCode:
		v.Meta = meta
	}
}
func checkText(s string, max int) error {
	if len([]rune(s)) < 1 || len([]rune(s)) > max {
		return inputError("INVALID_ARGUMENT", "String argument has invalid length")
	}
	return nil
}
func accountPath(id int) (string, error) {
	if id < 1 {
		return "", inputError("INVALID_ARGUMENT", "Account id must be positive")
	}
	return fmt.Sprintf("/bank-accounts/%d", id), nil
}
func wireMap(value any) (map[string]any, error) {
	b, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	m := map[string]any{}
	e = json.Unmarshal(b, &m)
	if m == nil {
		m = map[string]any{}
	}
	return m, e
}
func formValue(v any) string {
	switch x := v.(type) {
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return fmt.Sprintf("%v", x)
	default:
		return fmt.Sprint(x)
	}
}
func fileInput(value any) (Image, error) {
	switch v := value.(type) {
	case Image:
		return ReadImage(bytes.NewReader(v.Data), v.Filename, 4194304)
	case []byte:
		return ImageBytes(v, 4194304)
	case string:
		return OpenImage(v, 4194304)
	case io.Reader:
		return ReadImage(v, "", 4194304)
	default:
		return Image{}, inputError("INVALID_ARGUMENT", "Expected image path, bytes or reader")
	}
}
func upload[T any](ctx context.Context, t *transport, path, field string, file any, options any) (*T, error) {
	image, err := fileInput(file)
	if err != nil {
		return nil, err
	}
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	values, err := wireMap(options)
	if err != nil {
		return nil, inputError("INVALID_ARGUMENT", "Invalid options")
	}
	for key, v := range values {
		if err = writer.WriteField(key, formValue(v)); err != nil {
			return nil, err
		}
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filepath.Base(image.Filename)))
	h.Set("Content-Type", image.ContentType)
	part, err := writer.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err = part.Write(image.Data); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	raw, meta, err := t.request(ctx, "POST", path, buffer, false, http.Header{"Content-Type": []string{writer.FormDataContentType()}})
	if err != nil {
		return nil, err
	}
	return decodeResponse[T](raw, meta)
}

// GetInfo reads quota, credit, application and branch information.
func (c *Client) GetInfo(ctx context.Context) (*Info, error) {
	return call[Info](ctx, c.v2, "GET", "/info", nil, true)
}

// VerifyService provides billed bank and wallet verification.
type VerifyService struct{ t *transport }

// BankByPayload verifies one slip; successful verification may spend quota.
func (s *VerifyService) BankByPayload(ctx context.Context, value string, options *VerifyOptions) (*BankSlipResult, error) {
	if err := checkText(value, 128); err != nil {
		return nil, err
	}
	b, err := wireMap(options)
	if err != nil {
		return nil, inputError("INVALID_ARGUMENT", "Invalid options")
	}
	b["payload"] = value
	return call[BankSlipResult](ctx, s.t, "POST", "/verify/bank", b, false)
}

// BankByImage verifies one slip; successful verification may spend quota.
func (s *VerifyService) BankByImage(ctx context.Context, value any, options *VerifyOptions) (*BankSlipResult, error) {
	return upload[BankSlipResult](ctx, s.t, "/verify/bank", "image", value, options)
}

// BankByBase64 verifies one slip; successful verification may spend quota.
func (s *VerifyService) BankByBase64(ctx context.Context, value string, options *VerifyOptions) (*BankSlipResult, error) {
	b, err := wireMap(options)
	if err != nil {
		return nil, inputError("INVALID_ARGUMENT", "Invalid options")
	}
	b["base64"] = value
	return call[BankSlipResult](ctx, s.t, "POST", "/verify/bank", b, false)
}

// BankByURL verifies one slip; successful verification may spend quota.
func (s *VerifyService) BankByURL(ctx context.Context, value string, options *VerifyOptions) (*BankSlipResult, error) {
	if err := checkText(value, 2048); err != nil {
		return nil, err
	}
	b, err := wireMap(options)
	if err != nil {
		return nil, inputError("INVALID_ARGUMENT", "Invalid options")
	}
	b["url"] = value
	return call[BankSlipResult](ctx, s.t, "POST", "/verify/bank", b, false)
}

// TruewalletByImage verifies one slip; successful verification may spend quota.
func (s *VerifyService) TruewalletByImage(ctx context.Context, value any, options *VerifyOptions) (*TrueWalletResult, error) {
	return upload[TrueWalletResult](ctx, s.t, "/verify/truewallet", "image", value, options)
}

// TruewalletByBase64 verifies one slip; successful verification may spend quota.
func (s *VerifyService) TruewalletByBase64(ctx context.Context, value string, options *VerifyOptions) (*TrueWalletResult, error) {
	b, err := wireMap(options)
	if err != nil {
		return nil, inputError("INVALID_ARGUMENT", "Invalid options")
	}
	b["base64"] = value
	return call[TrueWalletResult](ctx, s.t, "POST", "/verify/truewallet", b, false)
}

// TruewalletByURL verifies one slip; successful verification may spend quota.
func (s *VerifyService) TruewalletByURL(ctx context.Context, value string, options *VerifyOptions) (*TrueWalletResult, error) {
	if err := checkText(value, 255); err != nil {
		return nil, err
	}
	b, err := wireMap(options)
	if err != nil {
		return nil, inputError("INVALID_ARGUMENT", "Invalid options")
	}
	b["url"] = value
	return call[TrueWalletResult](ctx, s.t, "POST", "/verify/truewallet", b, false)
}

// BanksService reads the public bank catalog.
type BanksService struct{ t *transport }

// List returns banks and extra verification options.
func (s *BanksService) List(ctx context.Context) (*BankList, error) {
	return call[BankList](ctx, s.t, "GET", "/banks", nil, true)
}

// BankAccountsService manages receiving accounts.
type BankAccountsService struct{ t *transport }

func (s *BankAccountsService) list(ctx context.Context, params *ListBankAccountsParams, all bool) (*BankAccountPage, error) {
	values, err := wireMap(params)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	for k, v := range values {
		query.Set(k, formValue(v))
	}
	if v, ok := values["page"].(float64); ok && v < 1 {
		return nil, inputError("INVALID_ARGUMENT", "page must be positive")
	}
	if v, ok := values["limit"].(float64); ok && (v < 1 || v > 100) {
		return nil, inputError("INVALID_ARGUMENT", "limit must be 1-100")
	}
	path := "/bank-accounts"
	if all {
		path += "/all"
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	return call[BankAccountPage](ctx, s.t, "GET", path, nil, true)
}

// List returns accounts linked to this branch.
func (s *BankAccountsService) List(ctx context.Context, params *ListBankAccountsParams) (*BankAccountPage, error) {
	return s.list(ctx, params, false)
}

// ListAll returns accounts across the service.
func (s *BankAccountsService) ListAll(ctx context.Context, params *ListBankAccountsParams) (*BankAccountPage, error) {
	return s.list(ctx, params, true)
}

// Get reads an account by id.
func (s *BankAccountsService) Get(ctx context.Context, id int) (*BankAccount, error) {
	path, e := accountPath(id)
	if e != nil {
		return nil, e
	}
	return call[BankAccount](ctx, s.t, "GET", path, nil, true)
}

// Create creates a receiving account.
func (s *BankAccountsService) Create(ctx context.Context, params CreateBankAccountParams) (*BankAccount, error) {
	return call[BankAccount](ctx, s.t, "POST", "/bank-accounts", params, false)
}

// Update changes supplied fields; Null[string]() explicitly clears ExtraVerify.
func (s *BankAccountsService) Update(ctx context.Context, id int, params UpdateBankAccountParams) (*BankAccount, error) {
	path, e := accountPath(id)
	if e != nil {
		return nil, e
	}
	return call[BankAccount](ctx, s.t, "PATCH", path, params, false)
}

// Link idempotently links an account to this branch.
func (s *BankAccountsService) Link(ctx context.Context, id int) (*LinkResult, error) {
	path, e := accountPath(id)
	if e != nil {
		return nil, e
	}
	return call[LinkResult](ctx, s.t, "POST", path+"/link", nil, true)
}

// Unlink removes the branch association.
func (s *BankAccountsService) Unlink(ctx context.Context, id int) (*UnlinkResult, error) {
	path, e := accountPath(id)
	if e != nil {
		return nil, e
	}
	return call[UnlinkResult](ctx, s.t, "DELETE", path+"/link", nil, false)
}
func (s *BankAccountsService) iterate(ctx context.Context, params *ListBankAccountsParams, all bool) iter.Seq2[BankAccount, error] {
	return func(yield func(BankAccount, error) bool) {
		p := ListBankAccountsParams{}
		if params != nil {
			p = *params
		}
		page := 1
		limit := 100
		if v, ok := p.Page.Value(); ok {
			page = v
		}
		if v, ok := p.Limit.Value(); ok {
			limit = v
		}
		for {
			p.Page = Some(page)
			p.Limit = Some(limit)
			result, err := s.list(ctx, &p, all)
			if err != nil {
				yield(BankAccount{}, err)
				return
			}
			for _, item := range result.Items {
				if !yield(item, nil) {
					return
				}
			}
			if len(result.Items) == 0 || page*limit >= result.Total {
				return
			}
			page++
		}
	}
}

// Iterate lazily walks linked accounts, fetching only the next needed page.
func (s *BankAccountsService) Iterate(ctx context.Context, params *ListBankAccountsParams) iter.Seq2[BankAccount, error] {
	return s.iterate(ctx, params, false)
}

// IterateAll lazily walks service accounts.
func (s *BankAccountsService) IterateAll(ctx context.Context, params *ListBankAccountsParams) iter.Seq2[BankAccount, error] {
	return s.iterate(ctx, params, true)
}

// JobsService submits and polls asynchronous verification.
type JobsService struct{ t *transport }

// SubmitBank enqueues one payload. Pass a callback unless the branch has a default.
func (s *JobsService) SubmitBank(ctx context.Context, payload string, options *AsyncVerifyOptions) (*JobSubmission, error) {
	if e := checkText(payload, 128); e != nil {
		return nil, e
	}
	b, e := wireMap(options)
	if e != nil {
		return nil, e
	}
	b["payload"] = payload
	return call[JobSubmission](ctx, s.t, "POST", "/verify/bank/async", b, false)
}

// SubmitBankBatch enqueues at most 100 slips, each independently billable.
func (s *JobsService) SubmitBankBatch(ctx context.Context, slips []BatchSlip, options *BatchOptions) (*BatchSubmission, error) {
	if len(slips) == 0 {
		return nil, inputError("EMPTY_BATCH", "At least one slip is required")
	}
	if len(slips) > 100 {
		return nil, inputError("BATCH_TOO_LARGE", "At most 100 slips per batch")
	}
	b, e := wireMap(options)
	if e != nil {
		return nil, e
	}
	b["slips"] = slips
	return call[BatchSubmission](ctx, s.t, "POST", "/verify/bank/batch", b, false)
}

// SubmitBankMany submits chunks in order, retaining accepted receipts on failure.
func (s *JobsService) SubmitBankMany(ctx context.Context, slips []BatchSlip, options *BatchOptions) ([]BatchSubmission, error) {
	if len(slips) == 0 {
		return nil, inputError("EMPTY_BATCH", "At least one slip is required")
	}
	submitted := []BatchSubmission{}
	for i := 0; i < len(slips); i += 100 {
		end := i + 100
		if end > len(slips) {
			end = len(slips)
		}
		batch, e := s.SubmitBankBatch(ctx, slips[i:end], options)
		if e != nil {
			return nil, &BatchSubmitError{Submitted: submitted, FailedChunk: i / 100, Cause: e}
		}
		submitted = append(submitted, *batch)
	}
	return submitted, nil
}

// Get returns the current job, including failed terminal states.
func (s *JobsService) Get(ctx context.Context, id string) (*Job, error) {
	if e := checkText(id, 255); e != nil {
		return nil, e
	}
	return call[Job](ctx, s.t, "GET", "/verify/bank/jobs/"+url.PathEscape(id), nil, true)
}

// WaitOptions configures bounded polling. Zero values select 2 seconds and 10 minutes.
type WaitOptions struct{ Interval, Timeout time.Duration }

// WaitForJob returns done or failed; timeout retains the last observed job.
func (s *JobsService) WaitForJob(ctx context.Context, id string, options *WaitOptions) (*Job, error) {
	interval := 2 * time.Second
	timeout := 600 * time.Second
	if options != nil {
		if options.Interval != 0 {
			interval = options.Interval
		}
		if options.Timeout != 0 {
			timeout = options.Timeout
		}
	}
	if interval < 500*time.Millisecond || timeout <= 0 {
		return nil, inputError("INVALID_ARGUMENT", "Invalid polling interval or timeout")
	}
	child, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var last *Job
	for {
		type pollResult struct {
			job *Job
			err error
		}
		replies := make(chan pollResult, 1)
		go func() { job, err := s.Get(child, id); replies <- pollResult{job, err} }()
		var result pollResult
		select {
		case <-child.Done():
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, &JobWaitTimeoutError{LastJob: last}
		case result = <-replies:
		}
		job, e := result.job, result.err
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if child.Err() != nil {
				return nil, &JobWaitTimeoutError{LastJob: last}
			}
			return nil, e
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if child.Err() != nil {
			return nil, &JobWaitTimeoutError{LastJob: last}
		}
		last = job
		if job.Status == "done" || job.Status == "failed" {
			return job, nil
		}
		if e = waitRetry(child, interval); e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, &JobWaitTimeoutError{LastJob: last}
		}
	}
}

// V1Service provides legacy operations on the independent v1 host.
type V1Service struct{ t *transport }

// GetMe reads legacy application quota and credit.
func (s *V1Service) GetMe(ctx context.Context) (*V1Me, error) {
	return call[V1Me](ctx, s.t, "GET", "/me", nil, true)
}

// VerifyBankByPayload is a billed GET and does not retry ambiguous responses.
func (s *V1Service) VerifyBankByPayload(ctx context.Context, payload string, options *V1VerifyOptions) (*BankSlip, error) {
	if e := checkText(payload, 255); e != nil {
		return nil, e
	}
	b, e := wireMap(options)
	if e != nil {
		return nil, e
	}
	b["payload"] = payload
	q := url.Values{}
	for k, v := range b {
		q.Set(k, formValue(v))
	}
	return call[BankSlip](ctx, s.t, "GET", "/verify?"+q.Encode(), nil, false)
}

// VerifyBankByImage uploads an image under the legacy field file.
func (s *V1Service) VerifyBankByImage(ctx context.Context, file any, options *V1VerifyOptions) (*BankSlip, error) {
	return upload[BankSlip](ctx, s.t, "/verify", "file", file, options)
}

// VerifyBankByBase64 uses the legacy field image.
func (s *V1Service) VerifyBankByBase64(ctx context.Context, image string, options *V1VerifyOptions) (*BankSlip, error) {
	b, e := wireMap(options)
	if e != nil {
		return nil, e
	}
	b["image"] = image
	return call[BankSlip](ctx, s.t, "POST", "/verify", b, false)
}

// VerifyBankByURL verifies a remote image.
func (s *V1Service) VerifyBankByURL(ctx context.Context, imageURL string, options *V1VerifyOptions) (*BankSlip, error) {
	if e := checkText(imageURL, 2048); e != nil {
		return nil, e
	}
	b, e := wireMap(options)
	if e != nil {
		return nil, e
	}
	b["url"] = imageURL
	return call[BankSlip](ctx, s.t, "POST", "/verify", b, false)
}

// VerifyTruemoneyByImage uploads one wallet slip.
func (s *V1Service) VerifyTruemoneyByImage(ctx context.Context, file any, options *V1VerifyOptions) (*TrueWalletSlip, error) {
	return upload[TrueWalletSlip](ctx, s.t, "/verify/truewallet", "file", file, options)
}

// GenerateQR generates a payment QR without verification quota consumption.
func (s *V1Service) GenerateQR(ctx context.Context, request QRRequest) (*QRCode, error) {
	return call[QRCode](ctx, s.t, "POST", "/qr/generate", request, true)
}
