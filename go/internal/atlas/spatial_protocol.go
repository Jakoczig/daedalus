package atlas

import (
	"errors"
	"time"
)

type spatialPointer struct {
	Protocol string    `json:"protocol"`
	ID       string    `json:"release_id"`
	Path     string    `json:"distribution"`
	Hash     string    `json:"distribution_sha256"`
	Issued   time.Time `json:"issued_at"`
	Expires  time.Time `json:"expires_at"`
}
type spatialBubbleAsset struct {
	Path    string `json:"path"`
	Bytes   int64  `json:"bytes"`
	SHA256  string `json:"sha256"`
	Radius  int64  `json:"radius_ly"`
	Systems int64  `json:"systems"`
}
type spatialDistribution struct {
	Protocol string `json:"protocol"`
	ID       string `json:"release_id"`
	Format   string `json:"format"`
	Frozen   string `json:"frozen_contract_commit"`
	Source   struct {
		URL            string  `json:"url"`
		SHA256         string  `json:"sha256"`
		Bytes          int64   `json:"bytes"`
		ETag           *string `json:"etag"`
		Modified       *string `json:"last_modified"`
		Downloaded     *string `json:"downloaded_at"`
		Fresh          bool    `json:"freshness_verified"`
		Checked        *string `json:"freshness_checked_at"`
		Method         *string `json:"freshness_method"`
		RemoteChecksum bool    `json:"remote_checksum_verified"`
	} `json:"source"`
	Spatial Asset              `json:"spatial"`
	Bubble  spatialBubbleAsset `json:"bubble"`
	RootLY  int64              `json:"root_ly"`
	Systems int64              `json:"systems"`
	Roots   int                `json:"roots"`
	Indexes int                `json:"indexes"`
	Blocks  int                `json:"blocks"`
	Prefix  string             `json:"assets_prefix"`
	Total   int64              `json:"assets_total_bytes"`
}

func (m *SpatialManager) pointer(b []byte, online bool) (spatialPointer, error) {
	var p spatialPointer
	v, e := verifyEnvelope(b, m.key)
	if e == nil {
		e = decodePayload(v, &p)
	}
	if e != nil {
		return p, e
	}
	n := m.now()
	if len(v) != 6 || p.Protocol != spatialProtocol || !idPattern.MatchString(p.ID) || p.Path != "releases/"+p.ID+"/distribution.json" || !hashPattern.MatchString(p.Hash) || p.Issued.IsZero() || p.Issued.After(n.Add(5*time.Minute)) || !p.Expires.After(p.Issued) || p.Expires.Sub(p.Issued) > 63*24*time.Hour || (online && !p.Expires.After(n)) {
		return p, errors.New("Invalid or expired spatial pointer")
	}
	return p, nil
}
func (m *SpatialManager) distribution(b []byte, p spatialPointer) (spatialDistribution, error) {
	var d spatialDistribution
	if digest(b) != p.Hash {
		return d, errSpatial
	}
	v, e := verifyEnvelope(b, m.key)
	if e == nil {
		e = decodePayload(v, &d)
	}
	if e != nil {
		return d, e
	}
	for name, count := range map[string]int{"source": 10, "spatial": 3, "bubble": 5} {
		o, ok := v[name].(map[string]any)
		if !ok || len(o) != count {
			return d, errSpatial
		}
	}
	if len(v) != 14 || d.Protocol != spatialProtocol || d.ID != p.ID || d.Format != spatialFormat || d.Frozen != frozenSpatial || d.Prefix != "assets/" || d.RootLY != 800 || d.Systems < 0 || d.Systems > 1<<53-1 || d.Roots < 0 || d.Roots > 110000 || d.Indexes < 0 || d.Indexes > d.Roots || d.Blocks < 0 || d.Blocks > 110000*15000 || d.Total < 0 || d.Total > 1<<40 || d.Source.URL != sourceURL || !hashPattern.MatchString(d.Source.SHA256) || d.Source.Bytes < 1 || d.Source.Bytes > 8<<30 || !d.Source.Fresh || d.Spatial.Path != "releases/"+d.ID+"/spatial.json" || !validAsset(d.Spatial, 16<<20) || d.Spatial.Count != 0 || d.Bubble.Path != "assets/bubbles/"+d.Bubble.SHA256+".zip" || !hashPattern.MatchString(d.Bubble.SHA256) || d.Bubble.Bytes < 22 || d.Bubble.Bytes > 1<<30 || d.Bubble.Radius != 350 || d.Bubble.Systems < 0 || d.Bubble.Systems > d.Systems {
		return d, errSpatial
	}
	return d, nil
}
func spatialNetworkPath(p string) bool {
	if p == "latest.json" {
		return true
	}
	// Constructed only from exact validated digest/release names.
	if spatialAssetPath.MatchString(p) || spatialReleasePath.MatchString(p) {
		return true
	}
	return false
}
