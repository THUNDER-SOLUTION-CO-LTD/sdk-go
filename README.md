# Thunder Go SDK

[![CI](https://github.com/THUNDER-SOLUTION-CO-LTD/sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/THUNDER-SOLUTION-CO-LTD/sdk-go/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Verify Thai bank transfer slips and TrueMoney Wallet receipts from Go. Check the receiving account, amount and duplicate status before crediting an order.

[ภาษาไทย](README.th.md)

## Features

- Verify a bank slip from an image, QR payload, URL or base64.
- Check whether a transfer pays a linked receiving account for the expected amount.
- Detect previously used slips before accepting payment.
- Verify TrueMoney Wallet receipts.
- Read remaining quota and manage receiving accounts.
- Submit batches, track background jobs and authenticate webhooks.
- Generate PromptPay and bill payment QR codes.

## Requirements

Go 1.26 or 1.27. The SDK uses only the standard library. Create a service in [the Thunder developer portal](https://developer.thunder.in.th), then copy its API key into your server environment as `THUNDER_API_KEY`. You can instead set `THUNDER_API_KEY_FILE` to a private file containing the key.

## Installation

```sh
go get github.com/THUNDER-SOLUTION-CO-LTD/sdk-go
```

## Quick start

Save the program below as `main.go`. Put a transfer slip at `slip.png` and set the API key. In the portal, add your receiving account and link it to the service/branch associated with the key. Change `150.0` to the order amount from your own database, not a value sent by the customer. Each verification can consume quota.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
)

func main() {
	client, err := thunder.NewClient("")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	result, err := client.Verify.BankByImage(ctx, "slip.png", &thunder.VerifyOptions{
		MatchAccount: thunder.Some(true), MatchAmount: thunder.Some(150.0),
		CheckDuplicate: thunder.Some(true),
	})
	var pending *thunder.SlipPendingError
	var missing *thunder.SlipNotFoundError
	if errors.As(err, &pending) {
		log.Print("The bank has not made this slip available yet")
		return
	}
	if errors.As(err, &missing) {
		log.Print("No transfer was found; check the slip")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	duplicate, checked := result.IsDuplicate.Value()
	if !checked || duplicate {
		log.Print("Slip used before, or duplicate check unavailable")
		return
	}
	matched, checked := result.IsAmountMatched.Value()
	if !checked || !matched {
		log.Print("Transfer does not match the order amount")
		return
	}
	account, checked := result.MatchedAccount.Value()
	if !checked || account == nil {
		log.Print("Transfer did not match a linked account")
		return
	}
	slip := result.RawSlip
	sender, _ := slip.Sender.Account.Name.Th.Value()
	receiver, _ := slip.Receiver.Account.Name.Th.Value()
	fmt.Println("Amount:", result.AmountInSlip)
	fmt.Println("Date:", slip.Date)
	fmt.Println("Sender:", sender)
	fmt.Println("Receiver:", receiver)
	fmt.Println("Transaction:", slip.TransRef)
	fmt.Println("Payment checks passed. Record this transaction once for the order.")
}
```

Run `go run .`. A successful check prints output like this (illustrative values):

    Amount: 150
    Date: 2026-10-29T14:32:00+07:00
    Sender: ผู้โอนตัวอย่าง
    Receiver: ร้านตัวอย่าง
    Transaction: EXAMPLE-TRANSFER-001
    Payment checks passed. Record this transaction once for the order.

Credit the order only after all checks pass. Store the transaction reference with a database uniqueness constraint and update the order in the same transaction, so concurrent requests cannot credit it twice. The SDK does not update your orders.

Next steps: [payment checks](#payment-checks), [receiving accounts](#receiving-accounts), [background jobs](#background-jobs), [error handling](#error-handling).

## Usage

### Verify a bank slip

| What you have | Method |
| --- | --- |
| Image file or uploaded bytes | `client.Verify.BankByImage` |
| QR payload text | `client.Verify.BankByPayload` |
| Public image URL | `client.Verify.BankByURL` |
| Base64 image | `client.Verify.BankByBase64` |

Payload means the text decoded from the slip QR, not the transaction reference. Image input accepts `Image`, `[]byte`, a path string or `io.Reader`. A string is a local file path: never pass an untrusted user string, because the SDK will read that file on your server.

```go
package main

import (
	"context"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
	"os"
	"strings"
)

func main() {
	client, err := thunder.NewClient("")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	payload, err := os.ReadFile("payload.txt")
	if err != nil {
		log.Fatal(err)
	}
	result, err := client.Verify.BankByPayload(ctx, strings.TrimSpace(string(payload)), nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Print(result.AmountInSlip, result.RawSlip.TransRef)
}
```

### Payment checks

API v2 returns check results in a successful response: `IsDuplicate=true`, `IsAmountMatched=false`, or a null `MatchedAccount` means you must reject the payment. A mismatch does not raise an API error. The quick start also rejects missing check results. `matchAccount` checks accounts linked to this service/branch; `matchAmount` compares the transfer against your order total; `checkDuplicate` asks whether this slip was used before.

```go
package main

import (
	"context"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
)

func main() {
	client, err := thunder.NewClient("")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	result, err := client.Verify.BankByImage(ctx, "slip.png", &thunder.VerifyOptions{
		MatchAccount: thunder.Some(true), MatchAmount: thunder.Some(150.0),
		CheckDuplicate: thunder.Some(true),
	})
	if err != nil {
		log.Fatal(err)
	}
	duplicate, checked := result.IsDuplicate.Value()
	log.Print("Duplicate check:", checked, " duplicate:", duplicate)
	matched, checked := result.IsAmountMatched.Value()
	log.Print("Amount check:", checked, " matched:", matched)
	account, checked := result.MatchedAccount.Value()
	log.Print("Account check:", checked, " matched:", account != nil)
}
```

### TrueMoney Wallet

Use a Wallet receipt image. Check the same payment flags before crediting the order; the receipt reference is `RawSlip.TransactionID`.

```go
package main

import (
	"context"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
)

func main() {
	client, err := thunder.NewClient("")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	result, err := client.Verify.TruewalletByImage(ctx, "wallet-slip.png", &thunder.VerifyOptions{
		MatchAccount: thunder.Some(true), MatchAmount: thunder.Some(150.0),
		CheckDuplicate: thunder.Some(true),
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Print(result.AmountInSlip, result.RawSlip.TransactionID)
}
```

### Account info and quota

Read quota before planning a batch. A null remaining quota means the API did not return a numeric limit; it is not zero.

```go
package main

import (
	"context"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
)

func main() {
	client, err := thunder.NewClient("")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	info, err := client.GetInfo(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if info.Application.Quota.Remaining == nil {
		log.Print("No quota limit returned")
		return
	}
	log.Print("Remaining quota:", *info.Application.Quota.Remaining)
}
```

### Receiving accounts

This example lists linked accounts with automatic pagination, creates a receiving account, updates its name and links it to the key's branch. Set `RECEIVING_ACCOUNT_NUMBER` to your own account; run the create step once, then reuse the returned id. `List` and `ListAll` return one page; `IterateAll` walks all service accounts.

```go
package main

import (
	"context"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
	"os"
)

func main() {
	client, err := thunder.NewClient("")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	for account, err := range client.BankAccounts.Iterate(ctx, &thunder.ListBankAccountsParams{Limit: thunder.Some(50)}) {
		if err != nil {
			log.Fatal(err)
		}
		log.Print(account.ID, account.NameTh)
	}
	account, err := client.BankAccounts.Create(ctx, thunder.CreateBankAccountParams{
		BankCode: "004", BankNumber: os.Getenv("RECEIVING_ACCOUNT_NUMBER"),
		NameTh: "ร้านตัวอย่าง", NameEn: "Example Shop", Type: thunder.AccountTypeJuristic,
	})
	if err != nil {
		log.Fatal(err)
	}
	updated, err := client.BankAccounts.Update(ctx, account.ID, thunder.UpdateBankAccountParams{NameEn: thunder.Some("Example Shop")})
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Updated:", updated.ID)
	linked, err := client.BankAccounts.Link(ctx, account.ID)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Linked:", linked.Linked)
}
```

### Background jobs

Submit QR payloads as a batch and keep each job id. Polling the same job does not submit another verification. `WaitForJob` waits up to ten minutes by default, including HTTP requests and retry delays; inspect the terminal status and payment checks before accepting a result.

```go
package main

import (
	"context"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
	"os"
	"strings"
)

func main() {
	client, err := thunder.NewClient("")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	payload, err := os.ReadFile("payload.txt")
	if err != nil {
		log.Fatal(err)
	}
	batch, err := client.Jobs.SubmitBankBatch(ctx, []thunder.BatchSlip{{
		Payload: strings.TrimSpace(string(payload)), CheckDuplicate: thunder.Some(true),
	}}, nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, submitted := range batch.Jobs {
		job, err := client.Jobs.WaitForJob(ctx, submitted.JobID, nil)
		if err != nil {
			log.Fatal(err)
		}
		log.Print(job.JobID, job.Status, job.Result)
	}
}
```

### Generate a payment QR

Set `PROMPTPAY_PHONE` to your PromptPay phone number. For bill payment, use `Type: thunder.QRTypePromptPay` with `BillerID` and `Ref1`; `Ref2` is optional. The older `QRTypeBillPayment` type still works for existing integrations. Amounts must be from 0.01 to 9,999,999,999.99 with at most two decimal places; omit the amount for an open-amount QR. Save the image returned by the API:

```go
package main

import (
	"context"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
	"os"
)

func main() {
	client, err := thunder.NewClient("")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	qr, err := client.V1.GenerateQR(ctx, thunder.QRRequest{
		Type: thunder.QRTypePromptPay, Msisdn: thunder.Some(os.Getenv("PROMPTPAY_PHONE")), Amount: thunder.Some(150.0),
	})
	if err != nil {
		log.Fatal(err)
	}
	image, err := qr.ImageBytes()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("payment-qr.png", image, 0600); err != nil {
		log.Fatal(err)
	}
	log.Print("Saved payment-qr.png")
}
```

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    thunder "github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
)

func main() {
    client, err := thunder.NewClient("")
    if err != nil { log.Fatal(err) }
    defer client.Close()
    qr, err := client.V1.GenerateQR(context.Background(), thunder.QRRequest{
        Type: thunder.QRTypePromptPay,
        BillerID: thunder.Some(os.Getenv("BILLER_ID")),
        Ref1: thunder.Some(os.Getenv("BILL_REFERENCE")),
        Amount: thunder.Some(150.0),
    })
    if err != nil { log.Fatal(err) }
    fmt.Println(qr.Image)
}
```

### Legacy API v1

Use v2 for new integrations. Existing v1 code can use `client.V1`; it returns the slip directly, and duplicate detection raises `DuplicateSlipError`.

```go
package main

import (
	"context"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
)

func main() {
	client, err := thunder.NewClient("")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	slip, err := client.V1.VerifyBankByImage(ctx, "slip.png", &thunder.V1VerifyOptions{CheckDuplicate: thunder.Some(true)})
	if err != nil {
		log.Fatal(err)
	}
	log.Print(slip.Amount.Amount, slip.TransRef)
}
```

### Contexts and concurrency

Reuse a client across goroutines. Each operation accepts a context; set a deadline to bound the whole operation and defer cancellation to release its resources.

```go
package main

import (
	"context"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
	"time"
)

func main() {
	client, err := thunder.NewClient("")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := client.Verify.BankByImage(ctx, "slip.png", &thunder.VerifyOptions{
		MatchAccount: thunder.Some(true), MatchAmount: thunder.Some(150.0), CheckDuplicate: thunder.Some(true),
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Print(result.AmountInSlip)
}
```

## The result object

The bank and Wallet result share amount and payment-check fields. Bank details are under `RawSlip`; Wallet uses a different receipt model. Optional fields appear only when the API supplies them: `Field[T].Value()` returns `(value, present)`; check `IsNull()` to distinguish JSON null from absence.

| Field | Type | Meaning | Example |
| --- | --- | --- | --- |
| `AmountInSlip` | float | Verified transfer amount in baht | 150.0 |
| `IsDuplicate` | optional bool | Whether the slip was used before | false |
| `IsAmountMatched` | optional bool | Whether the transfer matches the order amount | true |
| `AmountInOrder` | optional float | Order amount sent for comparison | 150.0 |
| `MatchedAccount` | optional account or null | Linked receiving account that matched | account object |
| `RawSlip.TransRef` | string | Bank transaction reference | EXAMPLE-TRANSFER-001 |
| `RawSlip.Date` | string | Transfer date and time, including offset when returned | 2026-10-29T14:32:00+07:00 |
| `RawSlip.Sender.Account.Name.Th` | optional string | Sender name in Thai | ผู้โอนตัวอย่าง |
| `RawSlip.Receiver.Account.Name.Th` | optional string | Receiver name in Thai | ร้านตัวอย่าง |
| `RawSlip.Sender.Account.Bank` | `Field[AccountNumber]` | Sender account number as returned, possibly masked | xxx-x-x1234-x |
| `RawSlip.Receiver.Account.Bank` | `Field[AccountNumber]` | Receiver account number as returned, possibly masked | xxx-x-x5678-x |
| `RawSlip.TransactionID` | string (Wallet) | Wallet transaction reference | EXAMPLE-WALLET-001 |
| `RawSlip.Sender.Name` | string (Wallet) | Wallet sender name | ผู้ส่งตัวอย่าง |
| `RawSlip.Receiver.Name` | string (Wallet) | Wallet receiver name | ผู้รับตัวอย่าง |
| `RawSlip.Receiver.Phone` | string (Wallet) | Wallet receiver phone as returned | 0xx-xxx-1234 |

Read the account number from `AccountNumber.Account` after checking the field with `Value()`.

See the [full Go model reference](https://document.thunder.in.th/en/sdk/go).

## Configuration

| Option | Environment variable | Default | What it does |
| --- | --- | --- | --- |
| `NewClient(apiKey)` | THUNDER_API_KEY; THUNDER_API_KEY_FILE | environment/file | Explicit key overrides environment; file is the fallback. |
| `WithTimeout` | none | 30 seconds | Timeout for each HTTP attempt. |
| `WithMaxRetries` | none | 2 | At most two additional attempts when retry rules allow. |
| `WithBaseURLs` | THUNDER_V2_BASE_URL; THUNDER_V1_BASE_URL | https://api.thunder.in.th/v2; https://api.thunder.in.th/v1 | Override the API endpoints. |
| `WithHTTPClient` | none | new HTTP client | Supply a custom client; close it yourself. |

The example closes the custom HTTP client as well as the SDK client. `client.Close()` releases idle connections when the SDK created the HTTP client.

```go
package main

import (
	"context"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
	"net/http"
	"time"
)

func main() {
	httpClient := &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}
	defer httpClient.CloseIdleConnections()
	client, err := thunder.NewClient("", thunder.WithTimeout(15*time.Second), thunder.WithMaxRetries(1), thunder.WithHTTPClient(httpClient))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	info, err := client.GetInfo(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Print(info.Application.Name)
}
```

## Error handling

`ThunderError` is the root interface; HTTP errors unwrap to `ApiError`, and `RequestTimeoutError` unwraps through `NetworkError`. Use `errors.As` for error types and `errors.Is` for `context.Canceled` or `context.DeadlineExceeded`.

A recognized API error code selects the error class first; otherwise the SDK uses the HTTP status. Unmapped 4xx responses use `ApiError`.

| Error class | HTTP status or stage | When it happens | What to do |
| --- | --- | --- | --- |
| `ThunderError (interface)` | varies | Base for all SDK failures. | Catch a specific error first. |
| `ApiError` | HTTP | Base for rejected API requests. | Use status, code and request id for diagnosis. |
| `AuthenticationError` | 401 | API key is invalid. | Check the key for this service. |
| `PermissionDeniedError` | 403 | Service, branch or IP cannot call the API. | Check service access and IP restrictions. |
| `QuotaExceededError` | 403 | Verification quota is exhausted. | Add quota before accepting more slips. |
| `InvalidRequestError` | 400 | The API rejects an argument. | Correct the request before sending again. |
| `InvalidImageError` | 400 | Image is unreadable, oversized or unsupported. | Send a readable supported image. |
| `InvalidUrlError` | 400 | Image or callback URL is refused. | Use a reachable public HTTPS URL. |
| `DuplicateSlipError` | 400 (v1) | A previously verified slip is submitted with duplicate checking. | Do not credit the same transfer again; inspect slip. |
| `NotFoundError` | 404 | Resource is missing or outside this branch. | Check the account or job id and branch. |
| `SlipNotFoundError` | 404 | No transfer or slip QR was found. | Ask for a readable slip from a completed transfer. |
| `SlipPendingError` | 404 | A recent slip is not available from the bank yet. | Wait before checking again. |
| `ConflictError` | 409 | Receiving account already exists. | Reuse it instead of creating another. |
| `RateLimitError` | 429 | Request rate is too high. | Wait for RetryAfter when present. |
| `ServerError` | 500, 502, 504; other 5xx except 503 | The API or bank could not finish the request. | Reconcile billed calls before resending. |
| `ServiceUnavailableError` | 503 | A dependency is unavailable. | Wait and check service status. |
| `InputError` | before sending | Local validation rejects input. | Fix the argument identified by code. |
| `NetworkError` | network | Connection fails. | Check connectivity and reconcile the outcome. |
| `RequestTimeoutError` | network | HTTP request exceeds its timeout. | Check the payment outcome before resubmitting. |
| `JobWaitTimeoutError` | polling deadline | Job polling exceeds the overall deadline. | Keep LastJob and poll the same job later. |
| `BatchSubmitError` | partial batch | A chunk fails after earlier chunks were accepted. | Resume only the unsent chunks. |
| `WebhookSignatureError` | before processing | Webhook signature is invalid or missing. | Return 400; do not process the event. |

```go
package main

import (
	"context"
	"errors"
	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"log"
)

func main() {
	client, err := thunder.NewClient("")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	result, err := client.Verify.BankByImage(ctx, "slip.png", nil)
	var missing *thunder.SlipNotFoundError
	var quota *thunder.QuotaExceededError
	var api *thunder.ApiError
	switch {
	case errors.As(err, &missing):
		log.Print("Ask the customer for a readable slip from a completed transfer")
	case errors.As(err, &quota):
		log.Print("Add verification quota before processing more payments")
	case errors.As(err, &api):
		log.Print("Verification failed:", api.Status, api.Message)
	case err != nil:
		log.Print("Outcome unknown; reconcile the payment before resubmitting")
	default:
		log.Print(result.AmountInSlip)
	}
}
```

| Situation | Automatic retry (default) |
| --- | --- |
| Connection fails before sending | Up to 2 extra attempts, including verification. |
| HTTP 429 | Up to 2 extra attempts when the delay is at most 60 seconds. |
| Safe reads and idempotent account linking | Up to 2 extra attempts for transport failures and HTTP 500, 502, 503, 504. |
| Verification or job submission after sending | Never retry read timeouts or 5xx: another send could consume quota twice. |
| Other 4xx | No automatic retry; correct the input or permissions. |

Retries use exponential backoff with jitter and respect `Retry-After` up to 60 seconds. A failed network call does not prove that a billed operation was never processed.

## Webhooks

Set `THUNDER_WEBHOOK_SECRET`, save this program as `main.go`, then run `go run .`. The handler reads the original body and `X-Thunder-Signature`, returns 400 for an invalid signature, and acknowledges a verified event with 204. Replace the logging step with durable storage or a queue before using it to confirm payments; process each job id once. Deploy behind HTTPS with your usual ingress.

```go
package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
)

func main() {
	secret := os.Getenv("THUNDER_WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("Set THUNDER_WEBHOOK_SECRET")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhooks/thunder", func(w http.ResponseWriter, r *http.Request) {
		rawBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
		if err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}
		event, err := thunder.ConstructWebhookEvent(rawBody, r.Header.Get("X-Thunder-Signature"), secret)
		if err != nil {
			http.Error(w, "Invalid webhook", http.StatusBadRequest)
			return
		}
		// Save or enqueue durably, using JobID to prevent duplicate processing.
		log.Printf("Verified job %s: %s", event.JobID, event.Status)
		w.WriteHeader(http.StatusNoContent)
	})
	server := &http.Server{Addr: ":8000", Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second}
	log.Fatal(server.ListenAndServe())
}
```

## Security and privacy

Keep the API key on your server, never in a browser or mobile application. Public errors and response headers redact the configured key, including percent-encoded and JSON-escaped forms, and always hide authorization headers. Transport causes contain sanitized diagnostics. `SensitiveRawBody()` returns the original error body and can expose secrets; keep it out of logs. Transfer results include personal data such as names and account numbers. Limit access, retention and what you print in application logs.

## Versioning and support policy

This SDK follows semantic versioning. Breaking public API changes require a major version. The runtime support policy follows versions still supported upstream; changes to the supported range are announced in the [changelog](CHANGELOG.md).

## Examples and documentation

[Runnable examples](examples/README.md) include one program per operation. Read the [Go SDK guide](https://document.thunder.in.th/en/sdk/go) and [API reference](https://document.thunder.in.th) for all options.

## Support

For account, quota or billing questions, use the [contact page](https://developer.thunder.in.th/contact) and include the request id if available. Report bugs and suggest features in [issues](https://github.com/THUNDER-SOLUTION-CO-LTD/sdk-go/issues). Never post API keys or real slip images.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, checks and pull requests. Maintainers follow [RELEASING.md](RELEASING.md).

## Security reports

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md), or use the [contact page](https://developer.thunder.in.th/contact). Never report vulnerabilities in public issues.

## License

[MIT](LICENSE). Copyright (c) 2026 Thunder Solution Co., Ltd.
