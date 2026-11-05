package thunder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ApiError describes an HTTP failure without retaining request credentials.
type ApiError struct {
	sensitiveBody func() string
	Status        int
	Code          *string
	Message       string
	RequestID     *string
	RawBody       string
	Headers       map[string]string
	Details       json.RawMessage
	RetryAfter    *float64
	RateLimit     *RateLimit
	Slip          Slip
}

// SensitiveRawBody returns the original response. Never log this value.
func (e *ApiError) SensitiveRawBody() string {
	if e.sensitiveBody == nil {
		return e.RawBody
	}
	return e.sensitiveBody()
}

var jsonEscape = regexp.MustCompile(`(?:\\u[0-9a-fA-F]{4})+|\\[/\\"]|\\[bfnrt]`)

func decodedText(value string) string {
	// PathUnescape rejects a whole string on one bad escape; decode valid octets.
	value = percentEscape.ReplaceAllStringFunc(value, func(s string) string {
		decoded, _ := url.PathUnescape(s)
		return decoded
	})
	return jsonEscape.ReplaceAllStringFunc(value, func(s string) string {
		var decoded string
		if json.Unmarshal([]byte("\""+s+"\""), &decoded) == nil {
			return decoded
		}
		return s
	})
}

var percentEscape = regexp.MustCompile(`%[0-9a-fA-F]{2}`)

func redact(value, key string) string {
	if len(key) < 4 {
		return value
	}
	original := value
	for {
		if strings.Contains(value, key) {
			parts := strings.Split(value, key)
			for i, part := range parts {
				parts[i] = redact(part, key)
			}
			return strings.Join(parts, "[REDACTED]")
		}
		decoded := decodedText(value)
		if decoded == value {
			return original
		}
		value = decoded
	}
}
func redactHeader(name, value, key string) string {
	if strings.EqualFold(name, "Authorization") || strings.EqualFold(name, "Proxy-Authorization") || redact(value, key) != value {
		return "[REDACTED]"
	}
	return value
}

// APIError is a compatibility alias for ApiError.
type APIError = ApiError

// Error returns a credential-free error message.
func (e *APIError) Error() string {
	return fmt.Sprintf("[%d %s] %s", e.Status, optionalString(e.Code), e.Message)
}

// AuthenticationError is a typed APIError.
type AuthenticationError struct{ APIError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *AuthenticationError) Unwrap() error { return &e.APIError }

// PermissionDeniedError is a typed APIError.
type PermissionDeniedError struct{ APIError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *PermissionDeniedError) Unwrap() error { return &e.APIError }

// QuotaExceededError is a typed PermissionDeniedError.
type QuotaExceededError struct{ PermissionDeniedError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *QuotaExceededError) Unwrap() error { return &e.PermissionDeniedError }

// InvalidRequestError is a typed APIError.
type InvalidRequestError struct{ APIError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *InvalidRequestError) Unwrap() error { return &e.APIError }

// InvalidImageError is a typed InvalidRequestError.
type InvalidImageError struct{ InvalidRequestError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *InvalidImageError) Unwrap() error { return &e.InvalidRequestError }

// InvalidUrlError is a typed InvalidRequestError.
type InvalidUrlError struct{ InvalidRequestError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *InvalidUrlError) Unwrap() error { return &e.InvalidRequestError }

// DuplicateSlipError is a typed InvalidRequestError.
type DuplicateSlipError struct{ InvalidRequestError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *DuplicateSlipError) Unwrap() error { return &e.InvalidRequestError }

// NotFoundError is a typed APIError.
type NotFoundError struct{ APIError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *NotFoundError) Unwrap() error { return &e.APIError }

// SlipNotFoundError is a typed NotFoundError.
type SlipNotFoundError struct{ NotFoundError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *SlipNotFoundError) Unwrap() error { return &e.NotFoundError }

// SlipPendingError is a typed NotFoundError.
type SlipPendingError struct{ NotFoundError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *SlipPendingError) Unwrap() error { return &e.NotFoundError }

// ConflictError is a typed APIError.
type ConflictError struct{ APIError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *ConflictError) Unwrap() error { return &e.APIError }

// RateLimitError is a typed APIError.
type RateLimitError struct{ APIError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *RateLimitError) Unwrap() error { return &e.APIError }

// ServerError is a typed APIError.
type ServerError struct{ APIError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *ServerError) Unwrap() error { return &e.APIError }

// ServiceUnavailableError is a typed ServerError.
type ServiceUnavailableError struct{ ServerError }

// Unwrap exposes the parent for errors.As and errors.Is.
func (e *ServiceUnavailableError) Unwrap() error { return &e.ServerError }

// RequestTimeoutError identifies a client-side attempt timeout.
type RequestTimeoutError struct{ NetworkError }

// Unwrap exposes the sanitized parent error.
func (e *RequestTimeoutError) Unwrap() error { return &e.NetworkError }

// InputError rejects an argument before HTTP.
type InputError struct {
	Code    string
	Message string
}

// Error returns a credential-free error message.
func (e *InputError) Error() string { return e.Code + ": " + e.Message }
func (*InputError) sdkError()       {}

// WebhookSignatureError indicates invalid HMAC authentication.
type WebhookSignatureError struct{}

// Error returns a credential-free error message.
func (*WebhookSignatureError) Error() string { return "Invalid webhook signature" }
func (*WebhookSignatureError) sdkError()     {}

// JobWaitTimeoutError retains the final polled state.
type JobWaitTimeoutError struct{ LastJob *Job }

// Error returns a credential-free error message.
func (*JobWaitTimeoutError) Error() string { return "Job polling timed out" }
func (*JobWaitTimeoutError) sdkError()     {}

// BatchSubmitError retains accepted batches when a later chunk fails.
type BatchSubmitError struct {
	Submitted   []BatchSubmission
	FailedChunk int
	Cause       error
}

// Error returns a credential-free error message.
func (e *BatchSubmitError) Error() string { return fmt.Sprintf("Batch chunk %d failed", e.FailedChunk) }

// Unwrap exposes the sanitized parent error.
func (e *BatchSubmitError) Unwrap() error   { return e.Cause }
func (*BatchSubmitError) sdkError()         {}
func inputError(code, message string) error { return &InputError{Code: code, Message: message} }
func parseRateLimit(h http.Header) *RateLimit {
	values := []int{}
	for _, k := range []string{"Limit", "Remaining", "Reset"} {
		n, err := strconv.Atoi(h.Get("X-RateLimit-" + k))
		if err != nil {
			return nil
		}
		values = append(values, n)
	}
	return &RateLimit{Limit: values[0], Remaining: values[1], Reset: values[2]}
}
func decodeAPIError(status int, data []byte, h http.Header, key string) error {
	raw := string(data)
	var body map[string]json.RawMessage
	_ = json.Unmarshal([]byte(raw), &body)
	// Normalize only when JSON escaping concealed a credential.
	var normalized any
	if json.Unmarshal([]byte(raw), &normalized) == nil {
		var clean func(any) any
		clean = func(v any) any {
			switch x := v.(type) {
			case string:
				return redact(x, key)
			case []any:
				for i := range x {
					x[i] = clean(x[i])
				}
				return x
			case map[string]any:
				result := map[string]any{}
				for k, v := range x {
					result[redact(k, key)] = clean(v)
				}
				return result
			}
			return v
		}
		before, _ := json.Marshal(normalized)
		after, _ := json.Marshal(clean(normalized))
		if string(after) != string(before) {
			b, _ := json.Marshal(clean(normalized))
			raw = string(b)
		}
		_ = json.Unmarshal([]byte(raw), &body)
	} else {
		raw = redact(raw, key)
		if raw != string(data) && decodedText(string(data)) != string(data) {
			raw = "[REDACTED]"
		}
	}
	value := body
	if len(body["error"]) > 0 {
		var nested map[string]json.RawMessage
		if json.Unmarshal(body["error"], &nested) == nil && nested != nil {
			value = nested
		}
	}
	var code, message string
	_ = json.Unmarshal(value["code"], &code)
	_ = json.Unmarshal(value["message"], &message)
	if code == "" {
		code = message
	}
	if message == "" {
		message = fmt.Sprintf("HTTP %d", status)
	}
	headers := map[string]string{}
	for k, vs := range h {
		headers[strings.ToLower(k)] = redactHeader(k, strings.Join(vs, ", "), key)
	}
	id := headers["x-request-id"]
	if id == "" {
		id = headers["cf-ray"]
	}
	var retry *float64
	if v := h.Get("Retry-After"); v != "" {
		if seconds, err := strconv.ParseFloat(v, 64); err == nil {
			if seconds < 0 {
				seconds = 0
			}
			retry = &seconds
		} else if date, err := http.ParseTime(v); err == nil {
			seconds := time.Until(date).Seconds()
			if seconds < 0 {
				seconds = 0
			}
			retry = &seconds
		}
	}
	base := APIError{sensitiveBody: func() string { return string(data) }, Status: status, Code: nullableString(redact(code, key)), Message: redact(message, key), RequestID: nullableString(id), RawBody: raw, Headers: headers, Details: body["details"], RetryAfter: retry, RateLimit: parseRateLimit(h)}
	if code == "duplicate_slip" {
		var probe map[string]any
		_ = json.Unmarshal(body["data"], &probe)
		if _, ok := probe["transactionId"]; ok {
			var slip TrueWalletSlip
			if json.Unmarshal(body["data"], &slip) == nil {
				base.Slip = &slip
			}
		} else {
			var slip BankSlip
			if json.Unmarshal(body["data"], &slip) == nil {
				base.Slip = &slip
			}
		}
	}
	class := errorCodes[code]
	if class == "" {
		class = errorStatus[status]
	}
	if class == "" && status >= 500 {
		class = "ServerError"
	}
	switch class {
	case "AuthenticationError":
		return &AuthenticationError{APIError: base}
	case "PermissionDeniedError":
		return &PermissionDeniedError{APIError: base}
	case "QuotaExceededError":
		return &QuotaExceededError{PermissionDeniedError: PermissionDeniedError{APIError: base}}
	case "InvalidRequestError":
		return &InvalidRequestError{APIError: base}
	case "InvalidImageError":
		return &InvalidImageError{InvalidRequestError: InvalidRequestError{APIError: base}}
	case "InvalidUrlError":
		return &InvalidUrlError{InvalidRequestError: InvalidRequestError{APIError: base}}
	case "DuplicateSlipError":
		return &DuplicateSlipError{InvalidRequestError: InvalidRequestError{APIError: base}}
	case "NotFoundError":
		return &NotFoundError{APIError: base}
	case "SlipNotFoundError":
		return &SlipNotFoundError{NotFoundError: NotFoundError{APIError: base}}
	case "SlipPendingError":
		return &SlipPendingError{NotFoundError: NotFoundError{APIError: base}}
	case "ConflictError":
		return &ConflictError{APIError: base}
	case "RateLimitError":
		return &RateLimitError{APIError: base}
	case "ServerError":
		return &ServerError{APIError: base}
	case "ServiceUnavailableError":
		return &ServiceUnavailableError{ServerError: ServerError{APIError: base}}
	default:
		return &base
	}
}

var errorCodes = map[string]string{"MISSING_API_KEY": "AuthenticationError", "INVALID_API_KEY": "AuthenticationError", "BRANCH_INACTIVE": "PermissionDeniedError", "SERVICE_BANNED": "PermissionDeniedError", "SERVICE_DELETED": "PermissionDeniedError", "USER_BANNED": "PermissionDeniedError", "IP_NOT_ALLOWED": "PermissionDeniedError", "SERVICE_EXPIRED": "PermissionDeniedError", "QUOTA_EXCEEDED": "QuotaExceededError", "VALIDATION_ERROR": "InvalidRequestError", "INVALID_BANK_CODE": "InvalidRequestError", "INVALID_EXTRA_VERIFY": "InvalidRequestError", "INVALID_MATCH_MODE": "InvalidRequestError", "IMAGE_SIZE_TOO_LARGE": "InvalidImageError", "INVALID_IMAGE_FORMAT": "InvalidImageError", "INVALID_IMAGE_TYPE": "InvalidImageError", "INVALID_IMAGE": "InvalidImageError", "INVALID_WALLET_QR": "InvalidImageError", "URL_PROTOCOL_NOT_ALLOWED": "InvalidUrlError", "URL_INVALID_IP_RANGE": "InvalidUrlError", "IMAGE_URL_UNREACHABLE": "InvalidUrlError", "INVALID_CALLBACK_URL": "InvalidUrlError", "SLIP_NOT_FOUND": "SlipNotFoundError", "SLIP_PENDING": "SlipPendingError", "BANK_ACCOUNT_NOT_FOUND": "NotFoundError", "JOB_NOT_FOUND": "NotFoundError", "NOT_FOUND": "NotFoundError", "BANK_ACCOUNT_DUPLICATE": "ConflictError", "RATE_LIMIT_EXCEEDED": "RateLimitError", "API_SERVER_ERROR": "ServerError", "INTERNAL_SERVER_ERROR": "ServerError", "RENEWAL_TEMPORARILY_UNAVAILABLE": "ServiceUnavailableError", "CALLBACK_UNAVAILABLE": "ServiceUnavailableError", "unauthorized": "AuthenticationError", "access_denied": "PermissionDeniedError", "account_not_verified": "PermissionDeniedError", "application_expired": "PermissionDeniedError", "application_deactivated": "PermissionDeniedError", "quota_exceeded": "QuotaExceededError", "invalid_payload": "InvalidRequestError", "invalid_check_duplicate": "InvalidRequestError", "invalid_request": "InvalidRequestError", "invalid_type": "InvalidRequestError", "invalid_transaction_id": "InvalidRequestError", "invalid_sender_name": "InvalidRequestError", "invalid_receiver_name": "InvalidRequestError", "invalid_receiver_phone": "InvalidRequestError", "invalid_date": "InvalidRequestError", "invalid_amount": "InvalidRequestError", "invalid_image": "InvalidImageError", "invalid_wallet_qr": "InvalidImageError", "invalid_base64": "InvalidImageError", "image_size_too_large": "InvalidImageError", "invalid_url": "InvalidUrlError", "url_unreachable": "InvalidUrlError", "duplicate_slip": "DuplicateSlipError", "slip_not_found": "SlipNotFoundError", "qrcode_not_found": "SlipNotFoundError", "slip_pending": "SlipPendingError", "not_found": "NotFoundError", "rate_limit_exceeded": "RateLimitError", "server_error": "ServerError", "api_server_error": "ServerError", "renewal_temporarily_unavailable": "ServiceUnavailableError"}
var errorStatus = map[int]string{400: "InvalidRequestError", 401: "AuthenticationError", 403: "PermissionDeniedError", 404: "NotFoundError", 409: "ConflictError", 429: "RateLimitError", 500: "ServerError", 502: "ServerError", 503: "ServiceUnavailableError", 504: "ServerError"}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// Slip is the typed bank-or-wallet value carried by a duplicate-slip error.
type Slip interface{ isSlip() }

func (*BankSlip) isSlip()       {}
func (*TrueWalletSlip) isSlip() {}

// Is matches the typed error category.
func (e *AuthenticationError) Is(target error) bool {
	_, ok := target.(*AuthenticationError)
	return ok
}

// Is matches the typed error category.
func (e *PermissionDeniedError) Is(target error) bool {
	_, ok := target.(*PermissionDeniedError)
	return ok
}

// Is matches the typed error category.
func (e *QuotaExceededError) Is(target error) bool { _, ok := target.(*QuotaExceededError); return ok }

// Is matches the typed error category.
func (e *InvalidRequestError) Is(target error) bool {
	_, ok := target.(*InvalidRequestError)
	return ok
}

// Is matches the typed error category.
func (e *InvalidImageError) Is(target error) bool { _, ok := target.(*InvalidImageError); return ok }

// Is matches the typed error category.
func (e *InvalidUrlError) Is(target error) bool { _, ok := target.(*InvalidUrlError); return ok }

// Is matches the typed error category.
func (e *DuplicateSlipError) Is(target error) bool { _, ok := target.(*DuplicateSlipError); return ok }

// Is matches the typed error category.
func (e *NotFoundError) Is(target error) bool { _, ok := target.(*NotFoundError); return ok }

// Is matches the typed error category.
func (e *SlipNotFoundError) Is(target error) bool { _, ok := target.(*SlipNotFoundError); return ok }

// Is matches the typed error category.
func (e *SlipPendingError) Is(target error) bool { _, ok := target.(*SlipPendingError); return ok }

// Is matches the typed error category.
func (e *ConflictError) Is(target error) bool { _, ok := target.(*ConflictError); return ok }

// Is matches the typed error category.
func (e *RateLimitError) Is(target error) bool { _, ok := target.(*RateLimitError); return ok }

// Is matches the typed error category.
func (e *ServerError) Is(target error) bool { _, ok := target.(*ServerError); return ok }

// Is matches the typed error category.
func (e *ServiceUnavailableError) Is(target error) bool {
	_, ok := target.(*ServiceUnavailableError)
	return ok
}

// Is matches the API error base through any typed child.
func (e *ApiError) Is(target error) bool { _, ok := target.(*ApiError); return ok }

// Is matches the RequestTimeoutError category.
func (e *RequestTimeoutError) Is(target error) bool {
	_, ok := target.(*RequestTimeoutError)
	return ok
}

// Is matches the InputError category.
func (e *InputError) Is(target error) bool { _, ok := target.(*InputError); return ok }

// Is matches the JobWaitTimeoutError category.
func (e *JobWaitTimeoutError) Is(target error) bool {
	_, ok := target.(*JobWaitTimeoutError)
	return ok
}

// Is matches the WebhookSignatureError category.
func (e *WebhookSignatureError) Is(target error) bool {
	_, ok := target.(*WebhookSignatureError)
	return ok
}

// Is matches the BatchSubmitError category.
func (e *BatchSubmitError) Is(target error) bool { _, ok := target.(*BatchSubmitError); return ok }

// transportDiagnostic contains only copied text, never an original transport object.
type transportDiagnostic struct {
	Type    string
	Message string
}

func (e *transportDiagnostic) Error() string { return e.Type + ": " + e.Message }
func sanitizeTransport(err error, key string) error {
	message := redact(err.Error(), key)
	if message != err.Error() && decodedText(err.Error()) != err.Error() {
		message = "[REDACTED]"
	}
	return &transportDiagnostic{Type: fmt.Sprintf("%T", err), Message: message}
}
