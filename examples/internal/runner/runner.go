// Package runner shares input handling between runnable operation examples.
package runner

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	sdk "github.com/THUNDER-SOLUTION-CO-LTD/sdk-go"
	"os"
	"strings"
	"time"
)

// Run reads a key from a file and invokes exactly one named operation.
func Run(operation string) {
	input := flag.String("input", "", "JSON argument file; fields follow the operation options")
	consent := flag.Bool("run", false, "Consent to requests, quota use and writes for this operation")
	flag.Parse()
	if !*consent {
		flag.Usage()
		return
	}
	keyPath := os.Getenv("THUNDER_API_KEY_FILE")
	key, err := os.ReadFile(keyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot read THUNDER_API_KEY_FILE")
		os.Exit(1)
	}
	args := map[string]json.RawMessage{}
	if *input != "" {
		b, e := os.ReadFile(*input)
		if e != nil || json.Unmarshal(b, &args) != nil {
			fmt.Fprintln(os.Stderr, "Invalid argument file")
			os.Exit(1)
		}
	}
	c, err := sdk.NewClient(strings.TrimSpace(string(key)), sdk.WithBaseURLs(os.Getenv("THUNDER_V2_BASE_URL"), os.Getenv("THUNDER_V1_BASE_URL")))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Invalid client configuration")
		os.Exit(1)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result, err := invoke(ctx, c, operation, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Operation failed (%T)\n", err)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
}
func decode[T any](value json.RawMessage) *T {
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
func invoke(ctx context.Context, c *sdk.Client, operation string, args map[string]json.RawMessage) (any, error) {
	opts := args
	if raw, ok := args["options"]; ok {
		_ = json.Unmarshal(raw, &opts)
	}
	optJSON, _ := json.Marshal(opts)
	file := func() any {
		v := valueArg(args, "file")
		path := v
		if strings.HasSuffix(v, "slip-qr.png") {
			b, e := os.ReadFile(path)
			if e != nil {
				panic(e)
			}
			return b
		}
		return path
	}
	id := 0
	_ = json.Unmarshal(args["id"], &id)
	switch operation {
	case "getInfo":
		return c.GetInfo(ctx)
	case "banks.list":
		return c.Banks.List(ctx)
	case "bankAccounts.list":
		return c.BankAccounts.List(ctx, decode[sdk.ListBankAccountsParams](optJSON))
	case "bankAccounts.listAll":
		return c.BankAccounts.ListAll(ctx, decode[sdk.ListBankAccountsParams](optJSON))
	case "bankAccounts.get":
		return c.BankAccounts.Get(ctx, id)
	case "bankAccounts.create":
		return c.BankAccounts.Create(ctx, *decode[sdk.CreateBankAccountParams](optJSON))
	case "bankAccounts.update":
		return c.BankAccounts.Update(ctx, id, *decode[sdk.UpdateBankAccountParams](optJSON))
	case "bankAccounts.link":
		return c.BankAccounts.Link(ctx, id)
	case "bankAccounts.unlink":
		return c.BankAccounts.Unlink(ctx, id)
	case "jobs.get":
		return c.Jobs.Get(ctx, valueArg(args, "jobId"))
	case "jobs.submitBank":
		return c.Jobs.SubmitBank(ctx, valueArg(args, "payload"), decode[sdk.AsyncVerifyOptions](optJSON))
	case "jobs.submitBankMany":
		return c.Jobs.SubmitBankMany(ctx, *decode[[]sdk.BatchSlip](args["slips"]), decode[sdk.BatchOptions](optJSON))
	case "jobs.waitForJob":
		return c.Jobs.WaitForJob(ctx, valueArg(args, "jobId"), nil)
	case "bankAccounts.iterate", "bankAccounts.iterateAll":
		result := []sdk.BankAccount{}
		iterator := c.BankAccounts.Iterate(ctx, decode[sdk.ListBankAccountsParams](optJSON))
		if operation == "bankAccounts.iterateAll" {
			iterator = c.BankAccounts.IterateAll(ctx, decode[sdk.ListBankAccountsParams](optJSON))
		}
		for item, err := range iterator {
			if err != nil {
				return nil, err
			}
			result = append(result, item)
		}
		return result, nil
	case "jobs.submitBankBatch":
		return c.Jobs.SubmitBankBatch(ctx, *decode[[]sdk.BatchSlip](args["slips"]), decode[sdk.BatchOptions](optJSON))
	case "v1.getMe":
		return c.V1.GetMe(ctx)
	case "v1.generateQr":
		return c.V1.GenerateQR(ctx, *decode[sdk.QRRequest](optJSON))
	case "verify.bankByPayload":
		return c.Verify.BankByPayload(ctx, valueArg(args, "payload"), decode[sdk.VerifyOptions](optJSON))
	case "verify.bankByImage":
		return c.Verify.BankByImage(ctx, file(), decode[sdk.VerifyOptions](optJSON))
	case "verify.bankByBase64":
		return c.Verify.BankByBase64(ctx, valueArg(args, "base64"), decode[sdk.VerifyOptions](optJSON))
	case "verify.bankByUrl":
		return c.Verify.BankByURL(ctx, valueArg(args, "url"), decode[sdk.VerifyOptions](optJSON))
	case "verify.truewalletByImage":
		return c.Verify.TruewalletByImage(ctx, file(), decode[sdk.VerifyOptions](optJSON))
	case "verify.truewalletByBase64":
		return c.Verify.TruewalletByBase64(ctx, valueArg(args, "base64"), decode[sdk.VerifyOptions](optJSON))
	case "verify.truewalletByUrl":
		return c.Verify.TruewalletByURL(ctx, valueArg(args, "url"), decode[sdk.VerifyOptions](optJSON))
	case "v1.verifyBankByPayload":
		return c.V1.VerifyBankByPayload(ctx, valueArg(args, "payload"), decode[sdk.V1VerifyOptions](optJSON))
	case "v1.verifyBankByImage":
		return c.V1.VerifyBankByImage(ctx, file(), decode[sdk.V1VerifyOptions](optJSON))
	case "v1.verifyBankByBase64":
		return c.V1.VerifyBankByBase64(ctx, valueArg(args, "base64"), decode[sdk.V1VerifyOptions](optJSON))
	case "v1.verifyBankByUrl":
		return c.V1.VerifyBankByURL(ctx, valueArg(args, "url"), decode[sdk.V1VerifyOptions](optJSON))
	case "v1.verifyTruemoneyByImage":
		return c.V1.VerifyTruemoneyByImage(ctx, file(), decode[sdk.V1VerifyOptions](optJSON))
	}
	return nil, fmt.Errorf("unknown fixture operation %s", operation)
}
