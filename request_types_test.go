package thunder

import (
	"encoding/json"
	"testing"
)

func TestRequestTypeConstantsEncodeAcceptedValues(t *testing.T) {
	for _, typ := range []AccountType{AccountTypeNatural, AccountTypeJuristic} {
		for _, request := range []any{CreateBankAccountParams{Type: typ}, UpdateBankAccountParams{Type: Some(typ)}, ListBankAccountsParams{Type: Some(typ)}} {
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded["type"] != string(typ) {
				t.Fatalf("wrong account type: %s", raw)
			}
		}
	}
	for _, typ := range []QRType{QRTypePromptPay, QRTypeBillPayment, QRTypeKShop, QRTypeMaeManee, QRTypeTungngern} {
		raw, err := json.Marshal(QRRequest{Type: typ})
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded["type"] != string(typ) {
			t.Fatalf("wrong QR type: %s", raw)
		}
	}
	var response BankAccount
	if err := json.Unmarshal([]byte(`{"type":"FUTURE"}`), &response); err != nil {
		t.Fatal(err)
	}
	if response.Type != "FUTURE" {
		t.Fatal("unknown response value was lost")
	}
}
