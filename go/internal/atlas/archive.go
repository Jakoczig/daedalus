package atlas

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var catalogPattern = regexp.MustCompile(`^catalogs/(xz|xy|yz)-L[5-7]\.json$`)
var overviewPattern = regexp.MustCompile(`^(xz|xy|yz)/([0-4])/(0|[1-9][0-9]{0,2})-(0|[1-9][0-9]{0,2})\.png$`)
var detailPattern = regexp.MustCompile(`^tiles/(xz|xy|yz)/([5-7])/(0|[1-9][0-9]{0,2})-(0|[1-9][0-9]{0,2})\.png$`)

func localPath(p string) bool {
	return p != "" && len(p) <= 180 && !strings.ContainsAny(p, "\\:\x00") && !strings.HasPrefix(p, "/") && !strings.Contains(p, "..") && !strings.Contains(p, "//")
}
func networkPath(p string) bool {
	if p == "latest.json" {
		return true
	}
	s := strings.SplitN(p, "/", 3)
	return len(s) == 3 && s[0] == "releases" && idPattern.MatchString(s[1]) && (s[2] == "index.json" || s[2] == "overview.zip" || catalogPattern.MatchString(s[2]) || detailPattern.MatchString(s[2]))
}

type Level struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Columns   int    `json:"columns"`
	Rows      int    `json:"rows"`
	Count     int    `json:"nonempty_tiles"`
	Occupancy string `json:"occupancy"`
	bits      []byte
}
type Metadata struct {
	Schema     int                   `json:"schema"`
	Status     string                `json:"status"`
	Label      string                `json:"label"`
	Extents    map[string][2]float64 `json:"extents"`
	TileSize   int                   `json:"tile_size"`
	MaxLevel   int                   `json:"max_level"`
	BaseLevel  int                   `json:"base_level"`
	Source     string                `json:"source_url"`
	Generation json.RawMessage       `json:"generation"`
	Views      map[string]struct {
		Levels map[string]*Level `json:"levels"`
	} `json:"views"`
}

func (m Metadata) Generated() string {
	if len(m.Generation) > 0 {
		var note string
		if json.Unmarshal(m.Generation, &note) == nil {
			return note
		}
		return "Generation timestamp not supplied"
	}
	return "Generation date not supplied"
}

type release struct {
	catalogsMu sync.Mutex
	catalogs   map[string]map[string]Asset
	index      Index
	meta       Metadata
	overview   map[string]Asset
	dir        string
}

func dimensions(axes string, level int) (int, int) {
	size := 256 << level
	switch axes {
	case "xz":
		return size, size
	case "xy":
		return size, size / 4
	case "yz":
		return size / 4, size
	}
	return 0, 0
}
func (l *Level) occupied(x, y int) bool {
	if x < 0 || y < 0 || x >= l.Columns || y >= l.Rows {
		return false
	}
	bit := y*l.Columns + x
	return l.bits[bit/8]&(1<<uint(bit%8)) != 0
}
func validateMeta(m *Metadata) error {
	if m.Schema != 2 || m.Status != "ready" || m.TileSize != 256 || m.MaxLevel != 7 || m.BaseLevel != 4 || m.Source != sourceURL || len(m.Extents) != 3 || len(m.Views) != 3 {
		return errors.New("unsupported atlas metadata")
	}
	for _, a := range []string{"x", "y", "z"} {
		b, ok := m.Extents[a]
		if !ok || math.IsNaN(b[0]) || math.IsNaN(b[1]) || math.IsInf(b[0], 0) || math.IsInf(b[1], 0) || b[1] <= b[0] || math.Abs(b[0]) > 1e6 || math.Abs(b[1]) > 1e6 {
			return errors.New("invalid atlas extents")
		}
	}
	for _, axes := range []string{"xz", "xy", "yz"} {
		v, ok := m.Views[axes]
		if !ok || len(v.Levels) != 8 {
			return errors.New("missing atlas projection/LOD")
		}
		for level := 0; level <= 7; level++ {
			l := v.Levels[strconv.Itoa(level)]
			w, h := dimensions(axes, level)
			if l == nil || l.Width != w || l.Height != h || l.Columns != (w+255)/256 || l.Rows != (h+255)/256 || l.Count < 0 || l.Count > l.Columns*l.Rows {
				return errors.New("invalid level dimensions")
			}
			bits, e := base64.StdEncoding.Strict().DecodeString(l.Occupancy)
			if e != nil || len(bits) != (l.Columns*l.Rows+7)/8 {
				return errors.New("invalid occupancy bitmap")
			}
			l.bits = bits
			count := 0
			for bit := 0; bit < len(bits)*8; bit++ {
				if bits[bit/8]&(1<<uint(bit%8)) != 0 {
					if bit >= l.Columns*l.Rows {
						return errors.New("occupancy padding")
					}
					count++
				}
			}
			if count != l.Count {
				return errors.New("occupancy count mismatch")
			}
		}
	}
	return nil
}
func validPNG(b []byte, w, h int) error {
	if len(b) > 4<<20 {
		return errors.New("oversized PNG")
	}
	cfg, e := png.DecodeConfig(bytes.NewReader(b))
	if e != nil || cfg.Width != w || cfg.Height != h {
		return errors.New("invalid PNG dimensions")
	}
	_, e = png.Decode(bytes.NewReader(b))
	return e
}
func openOverview(b []byte, idx Index, write func(string, []byte) error) (*release, error) {
	z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		return nil, e
	}
	if len(z.File) < 1 || len(z.File) > 2000 {
		return nil, errors.New("overview entry limit")
	}
	seen := map[string]bool{}
	files := map[string]Asset{}
	var meta Metadata
	var total uint64
	observed := map[string]int{}
	for _, f := range z.File {
		name := f.Name
		match := overviewPattern.FindStringSubmatch(name)
		if seen[name] || !f.Mode().IsRegular() || f.FileInfo().IsDir() || (name != "atlas.json" && match == nil) || f.Flags&1 != 0 {
			return nil, errors.New("unsafe or duplicate ZIP entry")
		}
		seen[name] = true
		total += f.UncompressedSize64
		if f.UncompressedSize64 > 4<<20 || total > 128<<20 {
			return nil, errors.New("ZIP extraction limit")
		}
		rd, e := f.Open()
		if e != nil {
			return nil, e
		}
		data, e := io.ReadAll(io.LimitReader(rd, int64(f.UncompressedSize64)+1))
		rd.Close()
		if e != nil || len(data) != int(f.UncompressedSize64) {
			return nil, errors.New("truncated/CRC-invalid ZIP")
		}
		if name == "atlas.json" {
			if digest(data) != idx.AtlasHash {
				return nil, errors.New("atlas metadata hash mismatch")
			}
			if _, e = parseJSON(data, 4<<20); e != nil {
				return nil, e
			}
			if e = json.Unmarshal(data, &meta); e != nil {
				return nil, e
			}
		} else {
			level, _ := strconv.Atoi(match[2])
			x, _ := strconv.Atoi(match[3])
			y, _ := strconv.Atoi(match[4])
			w, h := dimensions(match[1], level)
			w = min(256, w-x*256)
			h = min(256, h-y*256)
			if w <= 0 || h <= 0 {
				return nil, errors.New("tile outside projection")
			}
			if e = validPNG(data, w, h); e != nil {
				return nil, e
			}
			files[name] = Asset{Path: name, Bytes: int64(len(data)), SHA256: digest(data)}
			observed[match[1]+"/"+match[2]]++
		}
		if write != nil {
			if e = write(name, data); e != nil {
				return nil, e
			}
		}
	}
	if !seen["atlas.json"] {
		return nil, errors.New("missing atlas metadata")
	}
	if e = validateMeta(&meta); e != nil {
		return nil, e
	}
	detailCount := 0
	for _, axes := range []string{"xz", "xy", "yz"} {
		for n := 0; n <= 7; n++ {
			l := meta.Views[axes].Levels[strconv.Itoa(n)]
			if n <= 4 {
				if observed[fmt.Sprintf("%s/%d", axes, n)] != l.Count {
					return nil, errors.New("overview count mismatch")
				}
				for y := 0; y < l.Rows; y++ {
					for x := 0; x < l.Columns; x++ {
						_, ok := files[fmt.Sprintf("%s/%d/%d-%d.png", axes, n, x, y)]
						if ok != l.occupied(x, y) {
							return nil, errors.New("overview occupancy mismatch")
						}
					}
				}
			} else {
				detailCount += l.Count
			}
		}
	}
	if len(files)+detailCount != idx.TileCount || detailCount != idx.DetailCount {
		return nil, errors.New("tile summary mismatch")
	}
	return &release{index: idx, meta: meta, overview: files}, nil
}
