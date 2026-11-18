package webapp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"image"
	_ "image/jpeg"
	"image/png"
)

func avatarPNG(data string) ([]byte, error) {
	if len(data) > 700000 {
		return nil, errors.New("Portrait exceeds 512 KiB")
	}
	raw, e := base64.StdEncoding.DecodeString(data)
	if e != nil || len(raw) > 512<<10 {
		return nil, errors.New("Invalid portrait")
	}
	cfg, kind, e := image.DecodeConfig(bytes.NewReader(raw))
	if e != nil || (kind != "png" && kind != "jpeg") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 {
		return nil, errors.New("Choose a PNG/JPEG no larger than 2048 pixels")
	}
	src, _, e := image.Decode(bytes.NewReader(raw))
	if e != nil {
		return nil, errors.New("Cannot decode portrait")
	}
	w, h := cfg.Width, cfg.Height
	if w > 256 || h > 256 {
		if w >= h {
			h = h * 256 / w
			w = 256
		} else {
			w = w * 256 / h
			h = 256
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(x*cfg.Width/w, y*cfg.Height/h))
		}
	}
	var out bytes.Buffer
	if e = png.Encode(&out, dst); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
func (s *Server) avatarName() string { return "avatar-" + tokenHash(s.game.FieldCommander()) + ".png" }
func (s *Server) avatarURIFor(fid string) string {
	if fid == "" {
		return ""
	}
	b, e := privatefs.Read(s.profile, "avatar-"+tokenHash(fid)+".png")
	if e != nil || len(b) > 300000 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
}
func (s *Server) saveAvatar(raw json.RawMessage) error {
	if s.game == nil || s.game.FieldCommander() == "" {
		return errors.New("Verified commander required")
	}
	fid := s.game.FieldCommander()
	var q struct {
		Data string `json:"data"`
	}
	if strictDecode(raw, &q) != nil {
		return errors.New("Invalid portrait")
	}
	b, e := avatarPNG(q.Data)
	if e != nil {
		return e
	}
	if s.game.FieldCommander() != fid {
		return errors.New("Commander changed; choose the portrait again")
	}
	return privatefs.Write(s.profile, "avatar-"+tokenHash(fid)+".png", b)
}

// Small account-card portraits keep the bounded owner response under a megabyte.
func (s *Server) avatarThumbnail(fid string) string {
	b, e := privatefs.Read(s.profile, "avatar-"+tokenHash(fid)+".png")
	if e != nil || len(b) > 300000 {
		return ""
	}
	cfg, kind, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || kind != "png" || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 256 || cfg.Height > 256 {
		return ""
	}
	src, e := png.Decode(bytes.NewReader(b))
	if e != nil {
		return ""
	}
	dst := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			dst.Set(x, y, src.At(x*cfg.Width/32, y*cfg.Height/32))
		}
	}
	var out bytes.Buffer
	if png.Encode(&out, dst) != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
}
