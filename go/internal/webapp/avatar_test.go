package webapp

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"
)

func TestAvatarIsBoundedReencodedPNGNotSVGOrPath(t *testing.T) {
	for _, raw := range []string{"C:\\private.png", base64.StdEncoding.EncodeToString([]byte(`<svg onload="bad()"/>`))} {
		if _, e := avatarPNG(raw); e == nil {
			t.Fatal("unsafe portrait accepted")
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 1024, 512)))
	out, e := avatarPNG(base64.StdEncoding.EncodeToString(b.Bytes()))
	if e != nil {
		t.Fatal(e)
	}
	cfg, kind, e := image.DecodeConfig(bytes.NewReader(out))
	if e != nil || kind != "png" || cfg.Width != 256 || cfg.Height != 128 {
		t.Fatal("portrait not normalized", cfg, e)
	}
}
