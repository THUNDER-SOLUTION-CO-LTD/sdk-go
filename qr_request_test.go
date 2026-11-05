package thunder

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// QR request shapes of the API v1 rules of 2026-10-30 are sent exactly as given.
func TestQRRequestShapesSentUnchanged(t *testing.T) {
	cases := []struct {
		request QRRequest
		wire    string
	}{
		{QRRequest{Type: "PROMPTPAY", BillerID: Some("010555123456701"), Amount: Some(125.5)}, `{"type":"PROMPTPAY","billerId":"010555123456701","amount":125.5}`},
		{QRRequest{Type: "BILL_PAYMENT", BillerID: Some("010555123456701")}, `{"type":"BILL_PAYMENT","billerId":"010555123456701"}`},
		{QRRequest{Type: "KSHOP", Ref1: Some("KB000001234567"), Amount: Some(250.0)}, `{"type":"KSHOP","ref1":"KB000001234567","amount":250}`},
		{QRRequest{Type: "KSHOP", Ref1: Some("kps004kb000001234567")}, `{"type":"KSHOP","ref1":"kps004kb000001234567"}`},
		{QRRequest{Type: "MAE_MANEE", Ref1: Some("014000123456789")}, `{"type":"MAE_MANEE","ref1":"014000123456789"}`},
		{QRRequest{Type: "TUNGNGERN", Ref1: Some("ORDER12345"), MerchantName: Some("MY STORE"), Amount: Some(750.0)}, `{"type":"TUNGNGERN","ref1":"ORDER12345","merchantName":"MY STORE","amount":750}`},
	}
	var bodies []string
	c, e := NewClient("fixture-secret", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		if !strings.HasSuffix(r.URL.Path, "/qr/generate") {
			t.Errorf("path %s", r.URL.Path)
		}
		bodies = append(bodies, string(b))
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"status":200,"data":{"image":"iVBORw0KGgo=","mime":"image/png","payload":"000201"}}`))}, nil
	})}))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	for i, tc := range cases {
		qr, err := c.V1.GenerateQR(context.Background(), tc.request)
		if err != nil || qr.Payload != "000201" {
			t.Fatalf("case %d: %v", i, err)
		}
		var got, want any
		_ = json.Unmarshal([]byte(bodies[i]), &got)
		_ = json.Unmarshal([]byte(tc.wire), &want)
		gb, _ := json.Marshal(got)
		wb, _ := json.Marshal(want)
		if string(gb) != string(wb) {
			t.Errorf("case %d: sent %s, want %s", i, bodies[i], tc.wire)
		}
	}
}
