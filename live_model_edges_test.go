package thunder

import (
	"encoding/json"
	"testing"
)

func TestReceiverBankMissingIDIsNotInvented(t *testing.T) {
	var bank SlipReceiverBank
	if err := json.Unmarshal([]byte(`{}`), &bank); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(bank)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "{}" {
		t.Fatal("absent bank id was synthesized")
	}
}
