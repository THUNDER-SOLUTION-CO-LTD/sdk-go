package thunder

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

func TestSignedMalformedWebhookReturnsInputError(t *testing.T) {
	for _, raw := range []string{"not JSON", "{}", "[]", "null", `{"jobId":null,"status":"success","timestamp":"now"}`} {
		t.Run(raw, func(t *testing.T) {
			secret := "example-webhook-secret"
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write([]byte(raw))
			_, err := ConstructWebhookEvent([]byte(raw), "sha256="+hex.EncodeToString(mac.Sum(nil)), secret)
			var input *InputError
			if !errors.As(err, &input) || input.Code != "INVALID_ARGUMENT" {
				t.Fatalf("expected public input error, got %v", err)
			}
		})
	}
}
