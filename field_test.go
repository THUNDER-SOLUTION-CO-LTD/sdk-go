package thunder

import (
	"encoding/json"
	"testing"
)

func TestFieldPreservesAbsentNullAndValue(t *testing.T) {
	type nested struct {
		Name string `json:"name"`
	}
	type fixture struct {
		ID     int           `json:"id"`
		Nested Field[nested] `json:"nested,omitempty"`
	}
	var absent, null, present fixture
	for raw, target := range map[string]*fixture{`{"id":1}`: &absent, `{"id":1,"nested":null}`: &null, `{"id":1,"nested":{"name":"test"}}`: &present} {
		if err := json.Unmarshal([]byte(raw), target); err != nil {
			t.Fatal(err)
		}
	}
	if absent.Nested.IsSet() || !null.Nested.IsNull() {
		t.Fatal("absent and null conflated")
	}
	value, ok := present.Nested.Value()
	if !ok || value.Name != "test" {
		t.Fatal("nested type lost")
	}
	raw, err := marshalFields(absent)
	if err != nil || string(raw) != `{"id":1}` {
		t.Fatalf("absent field emitted: %s %v", raw, err)
	}
	raw, err = marshalFields(null)
	if err != nil || string(raw) != `{"id":1,"nested":null}` {
		t.Fatalf("null field lost: %s %v", raw, err)
	}
}
func TestFieldConstructorsAndReuse(t *testing.T) {
	value := Some(3)
	v, ok := value.Value()
	if !ok || v != 3 {
		t.Fatal("value missing")
	}
	value = Null[int]()
	if !value.IsSet() || !value.IsNull() {
		t.Fatal("null missing")
	}
	if err := json.Unmarshal([]byte(`4`), &value); err != nil {
		t.Fatal(err)
	}
	v, ok = value.Value()
	if !ok || v != 4 || value.IsNull() {
		t.Fatal("stale null retained")
	}
}
