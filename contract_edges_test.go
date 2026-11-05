package thunder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestErrorParentIdentityAndSafeCause(t *testing.T) {
	err := decodeAPIError(404, []byte(`{"error":{"code":"SLIP_NOT_FOUND","message":"missing"}}`), http.Header{}, "private-key")
	if !errors.Is(err, &ApiError{}) || !errors.Is(err, &NotFoundError{}) || !errors.Is(err, &SlipNotFoundError{}) {
		t.Fatal("typed parent errors.Is failed")
	}
	c, e := NewClient("private-key", WithMaxRetries(0), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("transport echoed private-key") })}))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	_, e = c.GetInfo(context.Background())
	var network *NetworkError
	if !errors.As(e, &network) || errors.Unwrap(network) == nil {
		t.Fatal("NetworkError needs a safe cause")
	}
	if strings.Contains(fmt.Sprintf("%+v %#v %v", network, network, errors.Unwrap(network)), "private-key") {
		t.Fatal("cause leaked key")
	}
}

func TestSuccessMetadataDoesNotEchoKey(t *testing.T) {
	c, e := NewClient("private-key", WithMaxRetries(0), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"X-Request-Id": []string{"private-key"}, "X-Echo": []string{"private-key"}}, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}, nil
	})}))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	result, e := c.Banks.List(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(fmt.Sprintf("%+v", result.Meta), "private-key") {
		t.Fatal("metadata echoed key")
	}
}

func TestInvalidPublicOptionsUseInputError(t *testing.T) {
	for _, option := range []Option{WithRetry(-1), WithTimeout(0), WithHTTPClient(nil)} {
		_, err := NewClient("fixture-key", option)
		var input *InputError
		if !errors.As(err, &input) || input.Code != "INVALID_ARGUMENT" {
			t.Errorf("Invalid public option returned %T", err)
		}
	}
}

func TestUtilityErrorsSupportCategoryIs(t *testing.T) {
	for _, pair := range [][2]error{
		{&NetworkError{}, &NetworkError{}},
		{&RequestTimeoutError{}, &RequestTimeoutError{}},
		{&InputError{Code: "EMPTY_FILE"}, &InputError{}},
		{&JobWaitTimeoutError{}, &JobWaitTimeoutError{}},
		{&WebhookSignatureError{}, &WebhookSignatureError{}},
		{&BatchSubmitError{}, &BatchSubmitError{}},
	} {
		if !errors.Is(pair[0], pair[1]) {
			t.Errorf("Category errors.Is failed for %T", pair[0])
		}
	}
}
