// Package community is a read-only, fixed-host EDSM cache. It never receives raw
// journals, commander identities, credentials or surface positions.
package community

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/daedalus/go/internal/explorestate"
	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const ttl = 6 * time.Hour
const maxPayload = 2 << 20

type Station struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	DistanceLS *float64 `json:"distanceLS"`
	UpdatedAt  string   `json:"updatedAt"`
}
type View struct {
	Enabled           bool                    `json:"enabled"`
	Status            string                  `json:"status"`
	Notice            string                  `json:"notice"`
	SystemName        string                  `json:"systemName"`
	SystemAddress     string                  `json:"systemAddress"`
	FetchedAt         string                  `json:"fetchedAt"`
	ExpiresAt         string                  `json:"expiresAt"`
	StationsFetchedAt string                  `json:"stationsFetchedAt"`
	Bodies            []explorestate.BodyView `json:"bodies"`
	Stations          []Station               `json:"stations"`
	Truncated         bool                    `json:"truncated"`
}
type entry struct {
	view    View
	expires time.Time
}
type Manager struct {
	mu         sync.Mutex
	profile    string
	client     *http.Client
	enabled    bool
	notice     string
	cache      map[string]entry
	order      []string
	next       time.Time
	busy       bool
	generation uint64
	cancel     context.CancelFunc
	now        func() time.Time
}

func New(profile string, client *http.Client) *Manager {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, MaxIdleConns: 2, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect rejected") }}
	}
	m := &Manager{profile: profile, client: client, cache: map[string]entry{}, now: time.Now}
	if profile != "" {
		raw, err := privatefs.Read(profile, "community.json")
		if err == nil {
			var c struct {
				Enabled bool `json:"enabled"`
			}
			if json.Unmarshal(raw, &c) == nil {
				m.enabled = c.Enabled
			} else {
				m.notice = "EDSM disabled: community.json is damaged. Disable and save to recover; original file is preserved until you save."
			}
		} else if !os.IsNotExist(err) {
			m.notice = "EDSM disabled: community settings cannot be read safely. Check profile permissions."
		}
	}
	return m
}
func (m *Manager) SetEnabled(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.profile != "" {
		raw, _ := json.Marshal(struct {
			Enabled bool `json:"enabled"`
		}{enabled})
		if err := privatefs.Write(m.profile, "community.json", raw); err != nil {
			return errors.New("EDSM preference could not be saved; check private profile permissions")
		}
	}
	m.generation++
	if m.cancel != nil {
		m.cancel()
	}
	m.enabled = enabled
	m.notice = ""
	m.cache = map[string]entry{}
	m.order = nil
	m.next = time.Time{}
	return nil
}
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generation++
	m.enabled = false
	if m.cancel != nil {
		m.cancel()
	}
	m.client.CloseIdleConnections()
}
func clone(v View) View {
	b, _ := json.Marshal(v)
	var out View
	_ = json.Unmarshal(b, &out)
	return out
}
func key(name, address string) string { return strings.ToLower(name) + "/" + address }
func (m *Manager) viewLocked(name, address string) View {
	v := View{Enabled: m.enabled, Status: "disabled", Notice: m.notice, SystemName: name, SystemAddress: address}
	if !m.enabled {
		return v
	}
	v.Status = "not_cached"
	if name == "" || exactID(address) == "" {
		v.Status = "waiting_for_journal"
		return v
	}
	if e, ok := m.cache[key(name, address)]; ok {
		v = clone(e.view)
		v.Enabled = true
		if !m.now().Before(e.expires) {
			v.Status = "stale"
		}
		return v
	}
	return v
}
func (m *Manager) View(name, address string) View {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.viewLocked(name, address)
}
func (m *Manager) Refresh(ctx context.Context, name, address string) View {
	m.mu.Lock()
	v := m.viewLocked(name, address)
	if !m.enabled || v.Status == "waiting_for_journal" || v.Status == "cached" {
		m.mu.Unlock()
		return v
	}
	if m.busy || m.now().Before(m.next) {
		v.Notice = "Lookup cooling down; cached/offline data remains available."
		m.mu.Unlock()
		return v
	}
	if clean(name) != name || len(name) > 128 {
		v.Status = "unavailable"
		m.mu.Unlock()
		return v
	}
	m.busy = true
	m.next = m.now().Add(time.Minute)
	gen := m.generation
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	m.cancel = cancel
	m.mu.Unlock()
	loaded, err := m.load(ctx, name, address)
	cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.busy = false
	m.cancel = nil
	if !m.enabled || m.generation != gen {
		return m.viewLocked(name, address)
	}
	if err != nil {
		v = m.viewLocked(name, address)
		v.Status = "unavailable"
		v.Notice = "EDSM unavailable or identity not verified. Local observations remain usable; retry after one minute."
		return v
	}
	if len(m.order) >= 16 {
		delete(m.cache, m.order[0])
		m.order = m.order[1:]
	}
	k := key(name, address)
	for i, x := range m.order {
		if x == k {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.order = append(m.order, k)
	m.cache[k] = entry{loaded, m.now().Add(ttl)}
	return clone(loaded)
}
func (m *Manager) get(ctx context.Context, path, name string, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.edsm.net/"+path+"?"+url.Values{"systemName": {name}}.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "DAEDALUS-Go-Preview/1 (read-only system lookup)")
	req.Header.Set("Accept", "application/json")
	res, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("provider unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxPayload+1))
	if err != nil || len(raw) > maxPayload {
		return errors.New("payload budget")
	}
	return json.Unmarshal(raw, out)
}
func clean(s string) string {
	if len(s) > 128 {
		return ""
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return strings.TrimSpace(s)
}
func exactID(s string) string {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(n, 10)
}
func number(p *float64, min, max float64) *float64 {
	if p == nil || math.IsNaN(*p) || math.IsInf(*p, 0) || *p < min || *p > max {
		return nil
	}
	v := *p
	return &v
}
func scale(p *float64, f float64) *float64 {
	p = number(p, 0, 1e20)
	if p != nil {
		*p *= f
	}
	return p
}

type providerBody struct {
	BodyID              json.Number `json:"bodyId"`
	Name                string
	Type                string
	SubType             string
	IsLandable          *bool
	DistanceToArrival   *float64
	Gravity             *float64
	Radius              *float64
	SolarRadius         *float64
	SurfaceTemperature  *float64
	SurfacePressure     *float64
	EarthMasses         *float64
	SolarMasses         *float64
	OrbitalPeriod       *float64
	RotationalPeriod    *float64
	SemiMajorAxis       *float64
	OrbitalEccentricity *float64
	AxialTilt           *float64
	AtmosphereType      string
	VolcanismType       string
	TerraformingState   string
	Parents             []map[string]json.Number
	Rings               []struct {
		Name        string
		Type        string
		InnerRadius *float64
		OuterRadius *float64
	}
}

func (m *Manager) load(ctx context.Context, name, address string) (View, error) {
	var raw struct {
		Name      string
		ID64      json.Number
		Bodies    []providerBody
		BodyCount *int
	}
	if err := m.get(ctx, "api-system-v1/bodies", name, &raw); err != nil {
		return View{}, err
	}
	if !strings.EqualFold(raw.Name, name) || exactID(raw.ID64.String()) != address {
		return View{}, errors.New("system identity mismatch")
	}
	at := m.now().UTC()
	v := View{Enabled: true, Status: "cached", SystemName: name, SystemAddress: address, FetchedAt: at.Format(time.RFC3339), ExpiresAt: at.Add(ttl).Format(time.RFC3339)}
	if len(raw.Bodies) > 256 {
		raw.Bodies = raw.Bodies[:256]
		v.Truncated = true
	}
	ids := map[string]bool{}
	for _, r := range raw.Bodies {
		id := exactID(r.BodyID.String())
		if id == "" || ids[id] || clean(r.Name) == "" {
			continue
		}
		ids[id] = true
		d := &explorestate.BodyDetails{IsStar: r.Type == "Star", RadiusKM: number(r.Radius, 0, 1e12), TemperatureK: number(r.SurfaceTemperature, 0, 1e12), PressureATM: number(r.SurfacePressure, 0, 1e12), MassEarths: number(r.EarthMasses, 0, 1e12), MassSolar: number(r.SolarMasses, 0, 1e12), OrbitalDays: number(r.OrbitalPeriod, 0, 1e12), RotationHours: scale(r.RotationalPeriod, 24), SemiMajorAU: number(r.SemiMajorAxis, 0, 1e12), Eccentricity: number(r.OrbitalEccentricity, 0, 1), AxialTiltDegrees: number(r.AxialTilt, -360, 360)}
		if d.IsStar && d.RadiusKM == nil {
			d.RadiusKM = scale(r.SolarRadius, 695700)
		}
		for i, p := range r.Parents {
			if i >= 8 {
				break
			}
			for k, n := range p {
				if k != "Star" && k != "Planet" && k != "Null" {
					continue
				}
				if parent := exactID(n.String()); parent != "" {
					d.Parents = append(d.Parents, explorestate.ParentView{Kind: k, BodyID: parent})
				}
			}
		}
		for i, x := range r.Rings {
			if i >= 16 {
				break
			}
			d.Rings = append(d.Rings, explorestate.RingView{Name: clean(x.Name), Class: clean(x.Type), InnerKM: number(x.InnerRadius, 0, 1e12), OuterKM: number(x.OuterRadius, 0, 1e12)})
		}
		v.Bodies = append(v.Bodies, explorestate.BodyView{BodyID: id, Name: clean(r.Name), Kind: clean(r.SubType), Landable: r.IsLandable, DistanceLS: number(r.DistanceToArrival, 0, 1e12), GravityG: number(r.Gravity, 0, 1e9), Atmosphere: clean(r.AtmosphereType), Volcanism: clean(r.VolcanismType), Details: d})
	}
	// Stations are independently timestamped and may fail without discarding bodies.
	var stations struct {
		Name     string
		ID64     json.Number
		Stations []struct {
			Name              string
			Type              string
			DistanceToArrival *float64
			UpdateTime        struct{ Information string }
		}
	}
	if err := m.get(ctx, "api-system-v1/stations", name, &stations); err == nil && strings.EqualFold(stations.Name, name) && exactID(stations.ID64.String()) == address {
		v.StationsFetchedAt = m.now().UTC().Format(time.RFC3339)
		if len(stations.Stations) > 64 {
			stations.Stations = stations.Stations[:64]
			v.Truncated = true
		}
		for _, s := range stations.Stations {
			if clean(s.Name) != "" {
				v.Stations = append(v.Stations, Station{Name: clean(s.Name), Type: clean(s.Type), DistanceLS: number(s.DistanceToArrival, 0, 1e12), UpdatedAt: clean(s.UpdateTime.Information)})
			}
		}
	} else {
		v.Notice = "Community stations unavailable; body lookup succeeded."
	}
	return v, nil
}
