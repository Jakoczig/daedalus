package webapp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"testing"
)

func TestG2PNativePortraitRejectsBrowserBytesAndUnknownProfile(t *testing.T) {
	s, _, _, _, _ := g2eFixture(t)
	owner, ctx := dialOwner(t, s)
	if len(rpcMessage(t, ctx, owner, "setGoAvatar", map[string]any{"data": "PRIVATE"})["error"]) == 0 {
		t.Fatal("dashboard/browser upload accepted")
	}
	calls := 0
	s.chooseAvatar = func() ([]byte, error) { calls++; return nil, nil }
	if len(rpcMessage(t, ctx, owner, "chooseGoAvatar", map[string]any{"fid": "UNVERIFIED"})["error"]) == 0 || calls != 0 {
		t.Fatal("unverified profile reached native dialog")
	}
	cards := s.game.AccountCards()
	if len(cards) == 0 {
		t.Fatal("fixture identity")
	}
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 4, 4)))
	s.chooseAvatar = func() ([]byte, error) { return b.Bytes(), nil }
	before := s.game.Accounts()
	raw, _ := json.Marshal(map[string]string{"fid": cards[0].FID})
	if _, err := s.accountRPC(ctx, "chooseGoAvatar", raw); err != nil {
		t.Fatal(err)
	}
	if s.avatarURIFor(cards[0].FID) == "" {
		t.Fatal("portrait not persisted")
	}
	if s.avatarURIFor("UNVERIFIED") != "" {
		t.Fatal("cross-account portrait")
	}
	if after := s.game.Accounts(); after != before {
		t.Fatal("portrait changed selected account")
	}
}

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
