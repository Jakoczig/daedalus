package atlas

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPythonPublisherGolden(t *testing.T) {
	b, e := os.ReadFile("testdata/python-golden.json")
	if e != nil {
		t.Fatal(e)
	}
	var g struct {
		Public, Canonical string
		Envelope          json.RawMessage
	}
	if e = json.Unmarshal(b, &g); e != nil {
		t.Fatal(e)
	}
	key, _ := hex.DecodeString(g.Public)
	payload, e := verifyEnvelope(g.Envelope, key)
	if e != nil {
		t.Fatal(e)
	}
	got, e := canonical(payload)
	if e != nil || string(got) != g.Canonical {
		t.Fatalf("canonical mismatch %q %v", got, e)
	}
	key[0] ^= 1
	if _, e = verifyEnvelope(g.Envelope, key); e == nil {
		t.Fatal("accepted wrong key")
	}
	for _, bad := range []string{strings.Replace(string(g.Envelope), "Ed25519", "RSA", 1), strings.Replace(string(g.Envelope), "unicode", "wrong", 1)} {
		if _, e = verifyEnvelope([]byte(bad), key); e == nil {
			t.Fatal("accepted tampering")
		}
	}
}
func TestStrictJSON(t *testing.T) {
	for _, s := range []string{`{"a":1,"a":2}`, `{"x":{"q":1,"q":2}}`, `{"x":NaN}`, `{"x":1e999}`, `{"x":"\ud800"}`, `{"x":"\udc00"}`, `{} {}`, strings.Repeat("[", 35) + "0" + strings.Repeat("]", 35)} {
		if _, e := parseJSON([]byte(s), 4096); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}
