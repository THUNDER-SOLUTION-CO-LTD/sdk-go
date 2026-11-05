# Thunder Go SDK

[![CI](https://github.com/THUNDER-SOLUTION-CO-LTD/sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/THUNDER-SOLUTION-CO-LTD/sdk-go/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

ใช้ Go ตรวจสลิปโอนเงินธนาคารไทยและใบเสร็จ TrueMoney Wallet พร้อมตรวจบัญชีรับเงิน ยอดเงิน และสลิปซ้ำก่อนเติมเงินหรือยืนยันคำสั่งซื้อ

[English](README.md)

## ความสามารถ

- ตรวจสลิปธนาคารจากรูป ข้อความ QR, URL หรือ base64
- ตรวจว่ารายการโอนเข้าบัญชีที่ผูกไว้และตรงกับยอดที่ต้องชำระ
- ตรวจสลิปซ้ำก่อนรับชำระเงิน
- ตรวจใบเสร็จ TrueMoney Wallet
- ดูโควต้าคงเหลือและจัดการบัญชีรับเงิน
- ส่งสลิปเป็นชุด ติดตาม job เบื้องหลัง และตรวจลายเซ็น webhook
- สร้าง QR สำหรับพร้อมเพย์และบิลเพย์เมนต์

## สิ่งที่ต้องเตรียม

Go 1.26 หรือ 1.27 โดย SDK ใช้เฉพาะ standard library สร้างบริการใน [Thunder developer portal](https://developer.thunder.in.th) แล้วนำ API key ของบริการนั้นมาเก็บใน environment ของเซิร์ฟเวอร์ชื่อ `THUNDER_API_KEY` หรือใช้ `THUNDER_API_KEY_FILE` ชี้ไปยังไฟล์ส่วนตัวที่เก็บ key

## ติดตั้ง

```sh
go get github.com/THUNDER-SOLUTION-CO-LTD/sdk-go
```

## เริ่มตรวจสลิป

บันทึกโปรแกรมด้านล่างเป็น `main.go` เตรียมสลิปชื่อ `slip.png` และตั้ง API key ให้เรียบร้อย เพิ่มบัญชีรับเงินใน portal แล้วผูกกับบริการ/สาขาที่ใช้ key นี้ เปลี่ยน `150.0` เป็นยอดคำสั่งซื้อจากฐานข้อมูลของคุณเอง อย่าใช้ยอดที่ลูกค้าส่งมา การตรวจแต่ละครั้งอาจใช้โควต้า

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

รัน `go run .` เมื่อตรวจผ่าน ผลลัพธ์จะมีลักษณะดังนี้ (ข้อมูลตัวอย่าง):

    Amount: 150
    Date: 2026-10-29T14:32:00+07:00
    Sender: ผู้โอนตัวอย่าง
    Receiver: ร้านตัวอย่าง
    Transaction: EXAMPLE-TRANSFER-001
    Payment checks passed. Record this transaction once for the order.

เติมเงินหรือยืนยันคำสั่งซื้อเมื่อผ่านทุกเงื่อนไขแล้วเท่านั้น เก็บเลขอ้างอิงรายการโอนแบบไม่ให้ซ้ำในฐานข้อมูล และอัปเดตคำสั่งซื้อใน transaction เดียวกัน เพื่อกันคำขอที่เข้ามาพร้อมกันเติมเงินซ้ำ SDK ไม่ได้อัปเดตคำสั่งซื้อให้

อ่านต่อ: [เงื่อนไขรับชำระ](#เงื่อนไขรับชำระ), [บัญชีรับเงิน](#บัญชีรับเงิน), [งานเบื้องหลัง](#งานเบื้องหลัง), [จัดการข้อผิดพลาด](#จัดการข้อผิดพลาด)

## วิธีใช้งาน

### ตรวจสลิปธนาคาร

| สิ่งที่มี | เมธอด |
| --- | --- |
| ไฟล์รูปหรือ bytes จากการอัปโหลด | `client.Verify.BankByImage` |
| ข้อความ payload ของ QR | `client.Verify.BankByPayload` |
| URL รูปที่เข้าถึงได้จากภายนอก | `client.Verify.BankByURL` |
| รูปแบบ base64 | `client.Verify.BankByBase64` |

payload คือข้อความที่อ่านจาก QR บนสลิป ไม่ใช่เลขอ้างอิงรายการโอน รูปส่งเป็น `Image`, `[]byte`, path string หรือ `io.Reader` ได้ ถ้าส่ง string SDK จะอ่านไฟล์ตาม path นั้นบนเซิร์ฟเวอร์ จึงห้ามนำ string จากผู้ใช้มาส่งตรงๆ

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

### เงื่อนไขรับชำระ

API v2 ส่งผลการตรวจมากับ response ที่สำเร็จ: `IsDuplicate=true`, `IsAmountMatched=false` หรือ `MatchedAccount` เป็น null หมายถึงต้องไม่รับชำระรายการนี้ ค่าที่ไม่ตรงไม่ได้ทำให้เกิด API error ตัวอย่างเริ่มต้นจึงตรวจผลเหล่านี้และปฏิเสธเมื่อไม่มีผลตรวจด้วย `matchAccount` ตรวจบัญชีที่ผูกกับบริการ/สาขานี้ `matchAmount` เทียบยอดโอนกับยอดคำสั่งซื้อ และ `checkDuplicate` ตรวจว่าเคยใช้สลิปนี้แล้วหรือไม่

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

### ใบเสร็จ TrueMoney Wallet

ใช้รูปใบเสร็จ Wallet และตรวจผลบัญชี ยอดเงิน และสลิปซ้ำก่อนยืนยันคำสั่งซื้อเช่นเดียวกับสลิปธนาคาร เลขอ้างอิงอยู่ที่ `RawSlip.TransactionID`

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

### ข้อมูลบริการและโควต้า

ดูโควต้าก่อนวางแผนส่งสลิปเป็นชุด ถ้าโควต้าคงเหลือเป็น null แปลว่า API ไม่ได้ส่งขีดจำกัดเป็นตัวเลข ไม่ใช่โควต้าเป็นศูนย์

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

### บัญชีรับเงิน

ตัวอย่างนี้ไล่อ่านบัญชีที่ผูกไว้ทุกหน้า สร้างบัญชีรับเงิน แก้ชื่อ แล้วผูกกับสาขาที่ใช้ key ตั้ง `RECEIVING_ACCOUNT_NUMBER` เป็นเลขบัญชีของคุณ สร้างครั้งเดียวแล้วเก็บ id ไว้ใช้ต่อ `List` และ `ListAll` คืนทีละหน้า ส่วน `IterateAll` อ่านทุกบัญชีของบริการ

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

### งานเบื้องหลัง

ส่ง payload ของ QR เป็นชุดและเก็บ job id ของแต่ละรายการ การติดตาม job เดิมไม่ได้ส่งตรวจสลิปใหม่ `WaitForJob` รอได้สิบนาทีโดยค่าเริ่มต้น รวมเวลาส่ง HTTP และรอก่อนลองใหม่ ต้องตรวจสถานะสุดท้ายและผลเงื่อนไขรับชำระก่อนใช้ผล

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

### สร้าง QR รับชำระ

ตั้ง `PROMPTPAY_PHONE` เป็นเบอร์พร้อมเพย์ของคุณ สำหรับบิลเพย์เมนต์ใช้ `Type: thunder.QRTypePromptPay` พร้อม `BillerID` และ `Ref1` โดย `Ref2` ไม่บังคับ ส่วน `QRTypeBillPayment` ยังใช้ได้สำหรับระบบที่เชื่อมต่อไว้แล้ว ยอดเงินต้องอยู่ในช่วง 0.01 ถึง 9,999,999,999.99 และมีทศนิยมไม่เกินสองตำแหน่ง ถ้าไม่กำหนดยอดให้เว้น amount ไว้ บันทึกรูปที่ API ส่งกลับได้ดังนี้:

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

### API v1 สำหรับระบบเดิม

ระบบใหม่ควรใช้ v2 ระบบเดิมที่ยังเรียก v1 ใช้ผ่าน `client.V1` ได้ โดยคืนข้อมูลสลิปโดยตรง และการตรวจพบสลิปซ้ำจะเป็น `DuplicateSlipError`

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

### Context และคำขอพร้อมกัน

ใช้ client ร่วมกันระหว่าง goroutine ได้ ทุกเมธอดรับ context จึงตั้ง deadline เพื่อจำกัดเวลารวมของงาน และเรียก cancel เมื่อใช้งานเสร็จเพื่อคืนทรัพยากร

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

## ข้อมูลผลตรวจ

ผลธนาคารและ Wallet มีฟิลด์ยอดเงินและผลเงื่อนไขรับชำระเหมือนกัน รายละเอียดธนาคารอยู่ใน `RawSlip` ส่วน Wallet ใช้โมเดลใบเสร็จของตัวเอง ฟิลด์ที่ไม่บังคับจะมีเมื่อ API ส่งมาเท่านั้น: `Field[T].Value()` คืน `(value, present)` และใช้ `IsNull()` แยก JSON null ออกจากฟิลด์ที่ไม่มีได้

| ฟิลด์ | ชนิด | ความหมาย | ตัวอย่าง |
| --- | --- | --- | --- |
| `AmountInSlip` | float | ยอดโอนที่ตรวจได้ หน่วยบาท | 150.0 |
| `IsDuplicate` | optional bool | เคยใช้สลิปนี้แล้วหรือไม่ | false |
| `IsAmountMatched` | optional bool | ยอดโอนตรงกับคำสั่งซื้อหรือไม่ | true |
| `AmountInOrder` | optional float | ยอดคำสั่งซื้อที่ส่งไปเทียบ | 150.0 |
| `MatchedAccount` | optional account or null | บัญชีรับเงินที่ผูกไว้และตรงกับรายการ | account object |
| `RawSlip.TransRef` | string | เลขอ้างอิงรายการโอนธนาคาร | EXAMPLE-TRANSFER-001 |
| `RawSlip.Date` | string | วันเวลาโอน รวม offset หาก API ส่งมา | 2026-10-29T14:32:00+07:00 |
| `RawSlip.Sender.Account.Name.Th` | optional string | ชื่อผู้โอนภาษาไทย | ผู้โอนตัวอย่าง |
| `RawSlip.Receiver.Account.Name.Th` | optional string | ชื่อผู้รับภาษาไทย | ร้านตัวอย่าง |
| `RawSlip.Sender.Account.Bank` | `Field[AccountNumber]` | เลขบัญชีผู้โอนตามที่ API ส่งมา อาจปิดบังบางส่วน | xxx-x-x1234-x |
| `RawSlip.Receiver.Account.Bank` | `Field[AccountNumber]` | เลขบัญชีผู้รับตามที่ API ส่งมา อาจปิดบังบางส่วน | xxx-x-x5678-x |
| `RawSlip.TransactionID` | string (Wallet) | เลขอ้างอิงรายการ Wallet | EXAMPLE-WALLET-001 |
| `RawSlip.Sender.Name` | string (Wallet) | ชื่อผู้ส่ง Wallet | ผู้ส่งตัวอย่าง |
| `RawSlip.Receiver.Name` | string (Wallet) | ชื่อผู้รับ Wallet | ผู้รับตัวอย่าง |
| `RawSlip.Receiver.Phone` | string (Wallet) | เบอร์ผู้รับ Wallet ตามที่ API ส่งมา | 0xx-xxx-1234 |

อ่านเลขบัญชีจาก `AccountNumber.Account` หลังตรวจฟิลด์ด้วย `Value()`

ดู [โมเดล Go ฉบับเต็ม](https://document.thunder.in.th/th/sdk/go)

## ตั้งค่า

| ตัวเลือก | Environment variable | ค่าเริ่มต้น | หน้าที่ |
| --- | --- | --- | --- |
| `NewClient(apiKey)` | THUNDER_API_KEY; THUNDER_API_KEY_FILE | environment/file | key ที่ส่งตรงมีลำดับก่อน environment และใช้ไฟล์เป็นทางเลือกสุดท้าย |
| `WithTimeout` | none | 30 seconds | เวลาสูงสุดของ HTTP แต่ละครั้ง |
| `WithMaxRetries` | none | 2 | ลองเพิ่มได้ไม่เกินสองครั้งเมื่อเข้าเงื่อนไข retry |
| `WithBaseURLs` | THUNDER_V2_BASE_URL; THUNDER_V1_BASE_URL | https://api.thunder.in.th/v2; https://api.thunder.in.th/v1 | กำหนด URL ของ API |
| `WithHTTPClient` | none | new HTTP client | ใช้ HTTP client ที่เตรียมเองและปิดเองเมื่อเสร็จ |

ตัวอย่างปิดทั้ง HTTP client ที่สร้างเองและ SDK client `client.Close()` คืน idle connection เมื่อ SDK เป็นผู้สร้าง HTTP client

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

## จัดการข้อผิดพลาด

`ThunderError` เป็น interface หลัก ข้อผิดพลาด HTTP unwrap ไปที่ `ApiError` และ `RequestTimeoutError` unwrap ผ่าน `NetworkError` ใช้ `errors.As` ตรวจชนิด และ `errors.Is` ตรวจ `context.Canceled` หรือ `context.DeadlineExceeded`

SDK เลือกคลาสจากรหัสข้อผิดพลาดที่รู้จักก่อน ถ้าไม่รู้จักจึงเลือกตาม HTTP status ส่วน 4xx ที่ไม่ได้ระบุใช้ `ApiError`

| คลาส | HTTP status หรือจุดที่เกิด | สาเหตุ | วิธีจัดการ |
| --- | --- | --- | --- |
| `ThunderError (interface)` | ขึ้นกับข้อผิดพลาด | คลาสหลักของข้อผิดพลาด SDK | ดักคลาสที่เฉพาะเจาะจงก่อน |
| `ApiError` | HTTP | คลาสหลักของคำขอที่ API ปฏิเสธ | ดู status, code และ request id |
| `AuthenticationError` | 401 | API key ไม่ถูกต้อง | ตรวจ key ของบริการนี้ |
| `PermissionDeniedError` | 403 | บริการ สาขา หรือ IP ไม่มีสิทธิ์เรียก API | ตรวจสิทธิ์บริการและข้อจำกัด IP |
| `QuotaExceededError` | 403 | โควต้าตรวจสลิปหมด | เพิ่มโควต้าก่อนรับสลิปเพิ่ม |
| `InvalidRequestError` | 400 | API ไม่รับค่าที่ส่งไป | แก้ข้อมูลก่อนส่งอีกครั้ง |
| `InvalidImageError` | 400 | รูปอ่านไม่ได้ ใหญ่เกิน หรือไม่รองรับ | ส่งรูปสลิปที่ชัดและเป็นชนิดที่รองรับ |
| `InvalidUrlError` | 400 | API ไม่รับ URL รูปหรือ callback | ใช้ HTTPS URL ที่เข้าถึงได้จากภายนอก |
| `DuplicateSlipError` | 400 (v1) | ส่งสลิปที่เคยตรวจแล้วโดยเปิดตรวจสลิปซ้ำ | อย่าเติมเงินซ้ำ ตรวจผลเดิมจาก slip |
| `NotFoundError` | 404 | ไม่พบข้อมูลหรือข้อมูลอยู่นอกสาขานี้ | ตรวจ id บัญชีหรือ job และสาขา |
| `SlipNotFoundError` | 404 | ไม่พบรายการโอนหรือ QR สลิป | ขอสลิปที่ชัดจากรายการโอนสำเร็จ |
| `SlipPendingError` | 404 | ธนาคารยังไม่พร้อมให้ตรวจสลิปที่เพิ่งโอน | รอสักครู่แล้วตรวจใหม่ |
| `ConflictError` | 409 | มีบัญชีรับเงินนี้แล้ว | ใช้บัญชีเดิมแทนการสร้างซ้ำ |
| `RateLimitError` | 429 | ส่งคำขอถี่เกินไป | รอตาม RetryAfter หากมี |
| `ServerError` | 500, 502, 504; 5xx อื่นยกเว้น 503 | API หรือธนาคารประมวลผลไม่สำเร็จ | ตรวจผลรายการที่อาจใช้โควต้าแล้วก่อนส่งซ้ำ |
| `ServiceUnavailableError` | 503 | บริการที่เกี่ยวข้องไม่พร้อม | รอและตรวจสถานะบริการ |
| `InputError` | ก่อนส่ง | SDK ไม่รับข้อมูลก่อนส่ง | แก้ค่าที่ระบุใน code |
| `NetworkError` | เครือข่าย | เชื่อมต่อไม่สำเร็จ | ตรวจเครือข่ายและผลรายการก่อนส่งซ้ำ |
| `RequestTimeoutError` | เครือข่าย | คำขอ HTTP เกินเวลาที่ตั้งไว้ | ตรวจผลการจ่ายก่อนส่งซ้ำ |
| `JobWaitTimeoutError` | หมดเวลารอ job | รอผล job เกินเวลารวม | เก็บ LastJob แล้วติดตาม job เดิมภายหลัง |
| `BatchSubmitError` | ส่ง batch บางส่วน | บางชุดส่งไม่สำเร็จหลัง API รับชุดก่อนหน้าแล้ว | ส่งต่อเฉพาะชุดที่ยังไม่ได้รับ |
| `WebhookSignatureError` | ก่อนประมวลผล | ลายเซ็น webhook ไม่ถูกต้องหรือไม่มี | ตอบ 400 และอย่าประมวลผล event |

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

| กรณี | การลองซ้ำอัตโนมัติ (ค่าเริ่มต้น) |
| --- | --- |
| เชื่อมต่อไม่ได้ก่อนส่ง | ลองเพิ่มไม่เกิน 2 ครั้ง รวมคำขอตรวจสลิป |
| HTTP 429 | ลองเพิ่มไม่เกิน 2 ครั้งเมื่อเวลารอไม่เกิน 60 วินาที |
| การอ่านและผูกบัญชีที่ทำซ้ำได้ | ลองเพิ่มไม่เกิน 2 ครั้งเมื่อเครือข่ายผิดพลาดหรือ HTTP 500, 502, 503, 504 |
| ตรวจสลิปหรือส่ง job หลังส่งคำขอแล้ว | ไม่ลองซ้ำเมื่อ read timeout หรือ 5xx เพราะส่งใหม่อาจใช้โควต้าซ้ำ |
| 4xx อื่นๆ | ไม่ลองซ้ำอัตโนมัติ แก้ข้อมูลหรือสิทธิ์ก่อน |

SDK เพิ่มเวลารอก่อนลองซ้ำแบบ exponential backoff พร้อม jitter และใช้ `Retry-After` ไม่เกิน 60 วินาที เครือข่ายที่ขัดข้องไม่ได้ยืนยันว่ารายการที่ใช้โควต้ายังไม่ถูกประมวลผล

## Webhook

ตั้ง `THUNDER_WEBHOOK_SECRET` บันทึกเป็น `main.go` แล้วรัน `go run .` handler อ่าน body เดิมและ `X-Thunder-Signature` ตอบ 400 เมื่อลายเซ็นไม่ถูกต้อง และตอบ 204 เมื่อยืนยัน event ได้ ก่อนใช้ยืนยันการจ่ายจริง ให้เปลี่ยนขั้นตอน log เป็นการเก็บลงฐานข้อมูลหรือคิวอย่างถาวร และประมวลผลแต่ละ job id เพียงครั้งเดียว ใช้ HTTPS ผ่าน ingress ของระบบคุณ

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

## ความปลอดภัยและข้อมูลส่วนบุคคล

เก็บ API key ไว้ฝั่งเซิร์ฟเวอร์ ห้ามใส่ใน browser หรือแอปมือถือ ข้อผิดพลาดและ response header ที่ SDK เปิดให้อ่านจะปิดบัง key ที่ตั้งไว้ รวมรูปแบบ percent-encoded และ JSON-escaped และปิดบัง authorization header เสมอ สาเหตุจาก transport เก็บเฉพาะข้อมูลวินิจฉัยที่ล้างข้อมูลลับแล้ว `SensitiveRawBody()` คืน body ข้อผิดพลาดต้นฉบับซึ่งอาจมีข้อมูลลับ จึงไม่ควรนำไป log ผลตรวจมีข้อมูลส่วนบุคคล เช่น ชื่อและเลขบัญชี ควรจำกัดสิทธิ์ ระยะเวลาเก็บ และข้อมูลที่เขียนลง log ของแอป

## เวอร์ชันและระยะเวลารองรับ

SDK ใช้ semantic versioning การเปลี่ยน public API ที่ทำให้โค้ดเดิมใช้ไม่ได้จะเพิ่ม major version รองรับ runtime ที่ยังได้รับการดูแลจากผู้พัฒนา หากเปลี่ยนช่วงเวอร์ชันที่รองรับจะแจ้งใน [changelog](CHANGELOG.md)

## ตัวอย่างและเอกสาร

[ตัวอย่างที่รันได้](examples/README.md) แยกโปรแกรมตามแต่ละเมธอด ดูตัวเลือกทั้งหมดได้ที่ [คู่มือ Go SDK](https://document.thunder.in.th/th/sdk/go) และ [API reference](https://document.thunder.in.th)

## ช่วยเหลือ

เรื่องบัญชี โควต้า หรือการชำระเงิน ติดต่อผ่าน[หน้าติดต่อเรา](https://developer.thunder.in.th/contact) พร้อม request id ถ้ามี แจ้งบั๊กหรือขอฟีเจอร์ได้ใน [issues](https://github.com/THUNDER-SOLUTION-CO-LTD/sdk-go/issues) ห้ามแนบ API key หรือรูปสลิปจริง

## ร่วมพัฒนา

อ่านวิธีติดตั้งเครื่องมือ รันทดสอบ และส่ง pull request ใน [CONTRIBUTING.md](CONTRIBUTING.md) ผู้ดูแลการออกเวอร์ชันอ่าน [RELEASING.md](RELEASING.md)

## แจ้งปัญหาความปลอดภัย

แจ้งแบบส่วนตัวตาม [SECURITY.md](SECURITY.md) หรือติดต่อผ่าน[หน้าติดต่อ](https://developer.thunder.in.th/contact) ห้ามแจ้งช่องโหว่ใน issue สาธารณะ

## สัญญาอนุญาต

[MIT](LICENSE). Copyright (c) 2026 Thunder Solution Co., Ltd.
