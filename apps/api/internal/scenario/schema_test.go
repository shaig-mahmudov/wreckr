package scenario

import "testing"

func TestLookupJSONPath(t *testing.T) {
	raw := []byte(`{"count":2,"orders":[{"id":"a"},{"id":"b"}]}`)

	tests := map[string]any{
		"$.count":         float64(2),
		"$.orders.length": float64(2),
		"$.orders[1].id":  "b",
	}

	for path, want := range tests {
		got, ok, err := LookupJSONPath(raw, path)
		if err != nil {
			t.Fatalf("LookupJSONPath(%s): %v", path, err)
		}
		if !ok {
			t.Fatalf("LookupJSONPath(%s) did not find value", path)
		}
		if got != want {
			t.Fatalf("LookupJSONPath(%s) = %#v, want %#v", path, got, want)
		}
	}
}
