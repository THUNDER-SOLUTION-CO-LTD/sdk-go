package thunder

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestImageSniffAndBound(t *testing.T) {
	raw := []byte("\x89PNG\r\n\x1a\nabc")
	image, err := ReadImage(bytes.NewReader(raw), "x.png", int64(len(raw)))
	if err != nil || image.ContentType != "image/png" {
		t.Fatalf("invalid image %v", err)
	}
	if _, err := ReadImage(bytes.NewReader(raw), "x", 3); err == nil {
		t.Fatal("oversize accepted")
	}
	if _, err := ReadImage(bytes.NewReader([]byte("bad")), "x", 10); err == nil {
		t.Fatal("unsupported accepted")
	}
}
func TestWebhookThaiRawBytes(t *testing.T) {
	raw := []byte(`{"message":"ทดสอบ"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(raw)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !VerifySignature(raw, signature, "secret") || VerifySignature(append(raw, ' '), signature, "secret") || VerifySignature(raw, signature[7:], "secret") {
		t.Fatal("invalid signature handling")
	}
}

func TestWebhookMatchesOpenSSLUTF8Signature(t *testing.T) {
	// openssl dgst -sha256 -hmac fixture over the exact UTF-8 body below.
	raw := []byte(`{"message":"ทดสอบ"}`)
	signature := "sha256=e33a4d2685803c3df04f24d5db1648d210b67cf2c627aa4a26b1990988218249"
	if !VerifySignature(raw, signature, "fixture") {
		t.Fatal("OpenSSL signature mismatch")
	}
}

func TestDefaultImageFilenameAndLowercaseSignature(t *testing.T) {
	image, err := ImageBytes([]byte("\x89PNG\r\n\x1a\nfixture"), 1024)
	if err != nil || image.Filename != "slip.png" {
		t.Fatalf("invalid default filename: %s %v", image.Filename, err)
	}
	signature := "sha256=E33A4D2685803C3DF04F24D5DB1648D210B67CF2C627AA4A26B1990988218249"
	if VerifySignature([]byte(`{"message":"ทดสอบ"}`), signature, "fixture") {
		t.Fatal("uppercase signature accepted")
	}
}

func TestImageFilenameAlwaysUsesSniffedType(t *testing.T) {
	image, err := ReadImage(bytes.NewReader([]byte("\x89PNG\r\n\x1a\ncontents")), "private-account.jpeg", 4194304)
	if err != nil || image.Filename != "slip.png" || image.ContentType != "image/png" {
		t.Fatal("image filename must use sniffed extension")
	}
}
