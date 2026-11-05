package thunder

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// VerifySignature checks the signature over untouched request bytes in constant time.
// Read the signature from X-Thunder-Signature.
func VerifySignature(rawBody []byte, signature, secret string) bool {
	if secret == "" || !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	digest := strings.TrimPrefix(signature, "sha256=")
	if strings.ToLower(digest) != digest {
		return false
	}
	supplied, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil || len(supplied) != sha256.Size {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	return hmac.Equal(mac.Sum(nil), supplied)
}

// WebhookSignatureHeader names the signature header for the selected build.
var WebhookSignatureHeader = "X-Thunder-Signature"

// VerifyWebhookSignature authenticates exact raw bytes with HMAC-SHA256.
func VerifyWebhookSignature(rawBody []byte, signature, secret string) bool {
	return VerifySignature(rawBody, signature, secret)
}

// ParseWebhookEvent parses a body without authentication.
func ParseWebhookEvent(rawBody []byte) (*WebhookEvent, error) {
	var shape map[string]json.RawMessage
	if json.Unmarshal(rawBody, &shape) != nil || shape == nil {
		return nil, inputError("INVALID_ARGUMENT", "Webhook body must be an event JSON object")
	}
	for _, field := range []string{"jobId", "status", "timestamp"} {
		var value *string
		if json.Unmarshal(shape[field], &value) != nil || value == nil {
			return nil, inputError("INVALID_ARGUMENT", "Webhook body must contain jobId, status and timestamp strings")
		}
	}
	var event WebhookEvent
	if json.Unmarshal(rawBody, &event) != nil {
		return nil, inputError("INVALID_ARGUMENT", "Invalid webhook JSON")
	}
	return &event, nil
}

// ConstructWebhookEvent authenticates before parsing.
func ConstructWebhookEvent(rawBody []byte, signature, secret string) (*WebhookEvent, error) {
	if !VerifyWebhookSignature(rawBody, signature, secret) {
		return nil, &WebhookSignatureError{}
	}
	return ParseWebhookEvent(rawBody)
}
