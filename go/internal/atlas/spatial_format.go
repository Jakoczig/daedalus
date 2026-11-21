package atlas

// Frozen G3A v1 contract, GPL-3.0 reference spatial.py at
// 5d1cbf1aee7a707259000e57ea963175f7cd4f3b. No source dataset is bundled.
import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"regexp"
	"sort"
)

const spatialFormat = "eridion-spatial-atlas-v1"
const spatialProtocol = "eridion-spatial-distribution-v1"
const frozenSpatial = "5d1cbf1aee7a707259000e57ea963175f7cd4f3b"
const spatialDomain = 32000000

var spatialMember = regexp.MustCompile(`^(spatial\.json|indexes/[0-9a-f]{64}\.json|blocks/[0-9a-f]{64}\.z)$`)
var errSpatial = errors.New("Invalid spatial data; density fallback required")
var errSpatialLimit = errors.New("Spatial volume exceeds safety budget; reduce inspection volume or use density overview")

type spatialRoot struct {
	Cell   [3]int64 `json:"cell"`
	SHA256 string   `json:"sha256"`
	Bytes  int64    `json:"bytes"`
	Count  int64    `json:"count"`
}
type spatialBlock struct {
	Lo     [3]int32 `json:"lo"`
	Hi     [3]int32 `json:"hi"`
	Count  int      `json:"count"`
	Bytes  int64    `json:"bytes"`
	SHA256 string   `json:"sha256"`
}
type spatialManifest struct {
	Format  string `json:"format"`
	RootLY  int64  `json:"root_ly"`
	Systems int64  `json:"systems"`
	Source  struct {
		SHA256 string `json:"sha256"`
		Label  string `json:"label"`
	} `json:"source"`
	Roots    []spatialRoot `json:"roots"`
	RadiusLY int64         `json:"radius_ly,omitempty"`
}

func parseSpatial(b []byte) (*spatialManifest, error) {
	var m spatialManifest
	if decodeStrict(b, &m, 16<<20) != nil || m.Format != spatialFormat || (m.RootLY != 100 && m.RootLY != 200 && m.RootLY != 400 && m.RootLY != 800) || m.Systems < 0 || m.Systems > 1<<53-1 || !hashPattern.MatchString(m.Source.SHA256) || len(m.Source.Label) > 4096 || m.Roots == nil || len(m.Roots) > 110000 || m.RadiusLY < 0 || m.RadiusLY > 1000 {
		return nil, errSpatial
	}
	// Fixed-size arrays in encoding/json otherwise silently accept missing elements.
	v, e := parseJSON(b, 16<<20)
	if e != nil {
		return nil, e
	}
	obj := v.(map[string]any)
	if source, ok := obj["source"].(map[string]any); !ok || len(source) != 2 {
		return nil, errSpatial
	}
	if len(obj) != 5 && !(len(obj) == 6 && m.RadiusLY > 0) {
		return nil, errSpatial
	}
	seen := map[[3]int64]bool{}
	var count int64
	for i, r := range m.Roots {
		raw := obj["roots"].([]any)[i].(map[string]any)
		cell, ok := raw["cell"].([]any)
		if !ok || len(cell) != 3 || len(raw) != 4 {
			return nil, errSpatial
		}
		if seen[r.Cell] || !hashPattern.MatchString(r.SHA256) || r.Bytes < 1 || r.Bytes > 2<<20 || r.Count < 1 || r.Count > 1<<53-1 {
			return nil, errSpatial
		}
		seen[r.Cell] = true
		for _, c := range r.Cell {
			if c < -10000 || c > 9999 || (c+1)*m.RootLY*32 <= -spatialDomain || c*m.RootLY*32 >= spatialDomain {
				return nil, errSpatial
			}
		}
		if count > (1<<53-1)-r.Count {
			return nil, errSpatial
		}
		count += r.Count
	}
	if m.Systems > count || (m.RadiusLY == 0 && m.Systems != count) {
		return nil, errSpatial
	}
	return &m, nil
}
func overlap(a, b, c, d [3]int32) bool {
	for i := range 3 {
		if a[i] >= d[i] || c[i] >= b[i] {
			return false
		}
	}
	return true
}
func parseSpatialIndex(b []byte, r spatialRoot, width int64) ([]spatialBlock, error) {
	if int64(len(b)) != r.Bytes || digest(b) != r.SHA256 {
		return nil, errSpatial
	}
	var blocks []spatialBlock
	if decodeStrict(b, &blocks, 2<<20) != nil || len(blocks) == 0 || len(blocks) > 15000 {
		return nil, errSpatial
	}
	raw, e := parseJSON(b, 2<<20)
	if e != nil {
		return nil, e
	}
	var total int64
	seen := map[string]bool{}
	for i, d := range blocks {
		obj, ok := raw.([]any)[i].(map[string]any)
		if !ok || len(obj) != 5 {
			return nil, errSpatial
		}
		for _, k := range []string{"lo", "hi"} {
			v, ok := obj[k].([]any)
			if !ok || len(v) != 3 {
				return nil, errSpatial
			}
		}
		if !hashPattern.MatchString(d.SHA256) || seen[d.SHA256] || d.Count < 1 || d.Count > 16384 || d.Bytes < 1 || d.Bytes > 200000 {
			return nil, errSpatial
		}
		seen[d.SHA256] = true
		for axis := range 3 {
			lo, hi := int64(d.Lo[axis]), int64(d.Hi[axis])
			if lo < -spatialDomain || hi > spatialDomain || lo >= hi || lo < r.Cell[axis]*width*32 || hi > (r.Cell[axis]+1)*width*32 {
				return nil, errSpatial
			}
		}
		total += int64(d.Count)
	}
	if total != r.Count {
		return nil, errSpatial
	}
	sorted := append([]spatialBlock(nil), blocks...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Lo[0] < sorted[j].Lo[0] })
	// Sweep X. Bound comparisons as well as bytes to avoid adversarial quadratic work.
	checks := 0
	for i, a := range sorted {
		for _, b := range sorted[i+1:] {
			if b.Lo[0] >= a.Hi[0] {
				break
			}
			checks++
			if checks > 2000000 {
				return nil, errSpatialLimit
			}
			if overlap(a.Lo, a.Hi, b.Lo, b.Hi) {
				return nil, errSpatial
			}
		}
	}
	return blocks, nil
}
func decodeESP(b []byte, d spatialBlock) ([][3]int32, error) {
	if d.Count < 1 || d.Count > 16384 || int64(len(b)) != d.Bytes || digest(b) != d.SHA256 {
		return nil, errSpatial
	}
	input := bytes.NewReader(b)
	z, e := zlib.NewReader(input)
	if e != nil {
		return nil, errSpatial
	}
	n := 8 + 12*d.Count
	raw, e := io.ReadAll(io.LimitReader(z, int64(n+1)))
	closeErr := z.Close()
	if e != nil || closeErr != nil || input.Len() != 0 || len(raw) != n || string(raw[:4]) != "ESP1" || binary.LittleEndian.Uint32(raw[4:8]) != uint32(d.Count) {
		return nil, errSpatial
	}
	points := make([][3]int32, d.Count)
	lo, hi := [3]int32{spatialDomain, spatialDomain, spatialDomain}, [3]int32{-spatialDomain, -spatialDomain, -spatialDomain}
	for i := range points {
		for a := range 3 {
			p := int32(binary.LittleEndian.Uint32(raw[8+i*12+a*4:]))
			if p < d.Lo[a] || p >= d.Hi[a] {
				return nil, errSpatial
			}
			points[i][a] = p
			if p < lo[a] {
				lo[a] = p
			}
			if p+1 > hi[a] {
				hi[a] = p + 1
			}
		}
		if i > 0 {
			a, b := points[i-1], points[i]
			if !(a[0] < b[0] || a[0] == b[0] && (a[1] < b[1] || a[1] == b[1] && a[2] < b[2])) {
				return nil, errSpatial
			}
		}
	}
	if lo != d.Lo || hi != d.Hi {
		return nil, errSpatial
	}
	return points, nil
}

type spatialBubble struct {
	file     *os.File
	reader   *zip.Reader
	members  map[string]*zip.File
	manifest *spatialManifest
}

func (z *spatialBubble) Close() error { return z.file.Close() }
func (z *spatialBubble) read(name string, limit int64) ([]byte, error) {
	f := z.members[name]
	if f == nil || f.UncompressedSize64 > uint64(limit) {
		return nil, errSpatial
	}
	r, e := f.Open()
	if e != nil {
		return nil, e
	}
	defer r.Close()
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, errSpatial
	}
	return b, nil
}
func openSpatialBubble(path, hash string) (out *spatialBubble, err error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() < 22 || st.Size() > 1<<30 {
		return nil, errSpatial
	}
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil || hex.EncodeToString(h.Sum(nil)) != hash {
		return nil, errSpatial
	}
	end := make([]byte, 22)
	if _, e = f.ReadAt(end, st.Size()-22); e != nil {
		return nil, e
	}
	u16 := func(i int) uint16 { return binary.LittleEndian.Uint16(end[i:]) }
	u32 := func(i int) uint32 { return binary.LittleEndian.Uint32(end[i:]) }
	if string(end[:4]) != "PK\x05\x06" || u16(4) != 0 || u16(6) != 0 || u16(8) != u16(10) || u16(10) == 0 || u16(10) > 32768 || u16(20) != 0 || u32(12) > 4<<20 || int64(u32(12))+int64(u32(16)) != st.Size()-22 {
		return nil, errSpatial
	}
	zr, e := zip.NewReader(f, st.Size())
	if e != nil {
		return nil, e
	}
	z := &spatialBubble{file: f, reader: zr, members: map[string]*zip.File{}}
	for _, m := range zr.File {
		if z.members[m.Name] != nil || !spatialMember.MatchString(m.Name) || m.Method != zip.Store || m.Flags&1 != 0 || len(m.Extra) != 0 || m.Comment != "" || m.CompressedSize64 != m.UncompressedSize64 || m.UncompressedSize64 > 16<<20 {
			return nil, errSpatial
		}
		z.members[m.Name] = m
		// Local header must agree with central directory, including no extra fields.
		off, e := m.DataOffset()
		if e != nil {
			return nil, e
		}
		head := make([]byte, 30)
		local := off - int64(len(m.Name)) - 30
		if local < 0 {
			return nil, errSpatial
		}
		if _, e = f.ReadAt(head, local); e != nil || string(head[:4]) != "PK\x03\x04" || binary.LittleEndian.Uint16(head[26:]) != uint16(len(m.Name)) || binary.LittleEndian.Uint16(head[28:]) != 0 || binary.LittleEndian.Uint16(head[6:]) != m.Flags || binary.LittleEndian.Uint16(head[8:]) != zip.Store {
			return nil, errSpatial
		}
		name := make([]byte, len(m.Name))
		if _, e = f.ReadAt(name, local+30); e != nil || string(name) != m.Name || off+int64(m.CompressedSize64) > int64(u32(16)) {
			return nil, errSpatial
		}
	}
	b, e := z.read("spatial.json", 16<<20)
	if e != nil {
		return nil, e
	}
	z.manifest, e = parseSpatial(b)
	if e != nil || z.manifest.RadiusLY == 0 {
		return nil, errSpatial
	}
	used := map[string]bool{"spatial.json": true}
	for _, r := range z.manifest.Roots {
		name := "indexes/" + r.SHA256 + ".json"
		b, e = z.read(name, r.Bytes)
		if e != nil {
			return nil, e
		}
		ds, e := parseSpatialIndex(b, r, z.manifest.RootLY)
		if e != nil {
			return nil, e
		}
		used[name] = true
		for _, d := range ds {
			name = "blocks/" + d.SHA256 + ".z"
			m := z.members[name]
			if m == nil || m.UncompressedSize64 != uint64(d.Bytes) {
				return nil, errSpatial
			}
			used[name] = true
		}
	}
	if len(used) != len(z.members) {
		return nil, errSpatial
	}
	return z, nil
}
