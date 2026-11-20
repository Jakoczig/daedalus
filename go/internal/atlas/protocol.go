// Package atlas consumes static signed-v2 Galaxy Painter releases. It never
// reads journals, generates rasters, accepts credentials or uploads data.
package atlas

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const Origin = "https://eridion.fitoserver.org"
const protocol = "eridion-atlas-signed-v2"
const sourceURL = "https://www.edsm.net/dump/systemsWithCoordinates.json.gz"

// Retrieved over verified HTTPS, SHA256(raw) prefix independently matched to
// operator-specified d679ccea38ac5d02. Website PEM is never a runtime trust source.
const publicKeyHex = "02003dd1d8c65f28c103eb0d7e3513bb36f6c82cb35208da7606005d1c4f4cdb"

var idPattern = regexp.MustCompile(`^[0-9a-f]{24}$`)
var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var sigPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{86}$`)

func digest(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }

// Token decoding, rather than unmarshalling to a map, detects duplicate keys at
// EVERY nesting level. Numeric lexemes retain integer/float identity for Python.
func parseJSON(b []byte, limit int) (any, error) {
	if len(b) > limit || !utf8.Valid(b) || !validSurrogates(b) {
		return nil, errors.New("invalid/oversized JSON")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		if depth > 32 {
			return nil, errors.New("JSON nesting limit")
		}
		t, e := d.Token()
		if e != nil {
			return nil, e
		}
		switch v := t.(type) {
		case json.Delim:
			switch v {
			case '{':
				m := map[string]any{}
				for d.More() {
					k, e := d.Token()
					if e != nil {
						return nil, e
					}
					s, ok := k.(string)
					if !ok {
						return nil, errors.New("invalid key")
					}
					if _, ok = m[s]; ok {
						return nil, errors.New("duplicate JSON key")
					}
					x, e := read(depth + 1)
					if e != nil {
						return nil, e
					}
					m[s] = x
				}
				_, e = d.Token()
				return m, e
			case '[':
				a := []any{}
				for d.More() {
					x, e := read(depth + 1)
					if e != nil {
						return nil, e
					}
					a = append(a, x)
				}
				_, e = d.Token()
				return a, e
			}
			return nil, errors.New("unexpected delimiter")
		case json.Number:
			if len(v) > 128 {
				return nil, errors.New("numeric limit")
			}
			if strings.ContainsAny(string(v), ".eE") {
				f, e := v.Float64()
				if e != nil || math.IsNaN(f) || math.IsInf(f, 0) {
					return nil, errors.New("invalid numeric value")
				}
			}
			return v, nil
		default:
			return v, nil
		}
	}
	v, e := read(0)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	return v, nil
}
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 32 || r > 126 {
				if r > 0xffff {
					a, c := utf16.EncodeRune(r)
					fmt.Fprintf(&b, `\u%04x\u%04x`, a, c)
				} else {
					fmt.Fprintf(&b, `\u%04x`, r)
				}
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
func pythonNumber(n json.Number) (string, error) {
	s := string(n)
	if !strings.ContainsAny(s, ".eE") {
		i, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return "", errors.New("invalid integer")
		}
		return i.String(), nil
	}
	f, e := n.Float64()
	if e != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return "", errors.New("invalid float")
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0", nil
		}
		return "0.0", nil
	}
	sci := strconv.FormatFloat(f, 'e', -1, 64)
	parts := strings.Split(sci, "e")
	exp, _ := strconv.Atoi(parts[1])
	if exp >= -4 && exp < 16 {
		s = strconv.FormatFloat(f, 'f', -1, 64)
		if !strings.Contains(s, ".") {
			s += ".0"
		}
		return s, nil
	}
	return fmt.Sprintf("%se%+03d", parts[0], exp), nil
}
func canonical(v any) ([]byte, error) {
	var b strings.Builder
	var put func(any) error
	put = func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			b.WriteByte('{')
			for i, k := range keys {
				if i > 0 {
					b.WriteByte(',')
				}
				b.WriteString(quote(k))
				b.WriteByte(':')
				if e := put(x[k]); e != nil {
					return e
				}
			}
			b.WriteByte('}')
		case []any:
			b.WriteByte('[')
			for i, v := range x {
				if i > 0 {
					b.WriteByte(',')
				}
				if e := put(v); e != nil {
					return e
				}
			}
			b.WriteByte(']')
		case string:
			b.WriteString(quote(x))
		case json.Number:
			s, e := pythonNumber(x)
			if e != nil {
				return e
			}
			b.WriteString(s)
		case bool:
			if x {
				b.WriteString("true")
			} else {
				b.WriteString("false")
			}
		case nil:
			b.WriteString("null")
		default:
			return errors.New("unsupported canonical value")
		}
		return nil
	}
	if e := put(v); e != nil {
		return nil, e
	}
	b.WriteByte('\n')
	return []byte(b.String()), nil
}
func verifyEnvelope(b, key []byte) (map[string]any, error) {
	v, e := parseJSON(b, 4<<20)
	if e != nil {
		return nil, e
	}
	m, ok := v.(map[string]any)
	if !ok || len(m) != 2 {
		return nil, errors.New("invalid envelope")
	}
	s, ok := m["signature"].(map[string]any)
	if !ok || len(s) != 3 || len(key) != ed25519.PublicKeySize || s["algorithm"] != "Ed25519" || s["key_id"] != digest(key)[:16] {
		return nil, errors.New("unknown publisher key/algorithm")
	}
	value, ok := s["value"].(string)
	if !ok || !sigPattern.MatchString(value) {
		return nil, errors.New("invalid signature encoding")
	}
	sig, e := base64.RawURLEncoding.Strict().DecodeString(value)
	if e != nil {
		return nil, e
	}
	p, ok := m["payload"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid payload")
	}
	raw, e := canonical(p)
	if e != nil || !ed25519.Verify(key, raw, sig) {
		return nil, errors.New("invalid publisher signature")
	}
	return p, nil
}
func decodeStrict(b []byte, out any, limit int) error {
	if _, e := parseJSON(b, limit); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
func decodePayload(p map[string]any, out any) error {
	b, e := canonical(p)
	if e != nil {
		return e
	}
	return decodeStrict(b, out, 4<<20)
}

// encoding/json substitutes U+FFFD for unpaired surrogate escapes. Reject them
// before tokenization so distinct malformed strings cannot collapse to one value.
func validSurrogates(b []byte) bool {
	in := false
	for i := 0; i < len(b); i++ {
		if b[i] == '"' {
			in = !in
			continue
		}
		if in && b[i] == '\\' {
			i++
			if i >= len(b) {
				return false
			}
			if b[i] != 'u' {
				continue
			}
			if i+4 >= len(b) {
				return false
			}
			n, e := strconv.ParseUint(string(b[i+1:i+5]), 16, 16)
			if e != nil {
				return false
			}
			i += 4
			if n >= 0xdc00 && n <= 0xdfff {
				return false
			}
			if n >= 0xd800 && n <= 0xdbff {
				if i+6 >= len(b) || b[i+1] != '\\' || b[i+2] != 'u' {
					return false
				}
				low, e := strconv.ParseUint(string(b[i+3:i+7]), 16, 16)
				if e != nil || low < 0xdc00 || low > 0xdfff {
					return false
				}
				i += 6
			}
		}
	}
	return true
}
