package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"

	"strings"
	"time"
)

func (m *Manager) Layers() Layers {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := m.layers
	v.Points = slices.Clone(v.Points)
	return v
}
func (m *Manager) Read(ctx context.Context, id, kind, query string, pages ...int) (*Snapshot, error) {
	page := 1
	if len(pages) > 0 {
		page = pages[0]
	}
	if page < 1 || page > 10 {
		return nil, errors.New("Page must be 1 through 10")
	}
	cacheKey := fmt.Sprintf("%s/%s/%d", kind, query, page)
	m.mu.Lock()
	s := m.states[id]
	cap := m.cap(id)
	if m.closed || (m.opts.RequireHistory && m.history == nil) || s == nil || !s.config.Read || s.config.Paused || cap.Gate != "" || !slices.Contains(cap.Read, kind) {
		m.mu.Unlock()
		return nil, errors.New("Provider read consent/approval required")
	}
	if id == "edastro" && kind == "gec" && m.layers.FetchedAt != "" {
		expires, err := time.Parse(time.RFC3339, m.layers.ExpiresAt)
		if err == nil && m.opts.Now().Before(expires) {
			cached := &Snapshot{Source: "edastro / gec (rare subset)", FetchedAt: m.layers.FetchedAt, ExpiresAt: m.layers.ExpiresAt, Facts: []Fact{{Name: "Cached GEC points", Value: fmt.Sprint(len(m.layers.Points))}, {Name: "Truncated", Value: fmt.Sprint(m.layers.Truncated)}}}
			m.mu.Unlock()
			return cached, nil
		}
	}
	if ((id == "inara" && kind == "communitygoals" && s.config.CommunityGoals) || id == "spansh" || id == "elitebgs" || id == "canonn" || id == "raven" || id == "edsm-reference") && m.history != nil {
		raw, err := m.history.CachedReference(id, cacheKey, m.opts.Now())
		if err != nil {
			m.mu.Unlock()
			return nil, err
		}
		if len(raw) > 0 {
			var v Snapshot
			if json.Unmarshal(raw, &v) == nil {
				s.snapshot = &v
				m.mu.Unlock()
				return &v, nil
			}
		}
	}
	if id == "inara" && kind == "communitygoals" && !s.config.CommunityGoals {
		m.mu.Unlock()
		return nil, errors.New("Separate public community-goals read consent required")
	}
	if id == "inara" && m.history != nil && kind == "profile" && !m.history.NeedsInara(ctx, m.fid, "commander", []string{"name", "ranks", "squadron"}, m.opts.Now()) {
		m.mu.Unlock()
		return nil, errors.New("Local and Frontier facts are current; Inara gap read not needed")
	}
	if m.history != nil && kind == "profile" && (id == "frontier" || id == "inara") && (m.fid == "" || m.name == "") {
		m.mu.Unlock()
		return nil, errors.New("Verified local commander required before account reconciliation")
	}
	if s.busy || m.opts.Now().Before(s.next) {
		m.mu.Unlock()
		return nil, errors.New("Provider cooldown active")
	}
	if m.history != nil {
		rateID := id
		if id == "edsm-reference" {
			rateID = "edsm"
		}
		if err := m.history.AuthorizeRead(rateID, kind, m.opts.Now()); err != nil {
			m.mu.Unlock()
			return nil, err
		}
	}
	s.busy = true
	s.next = m.opts.Now().Add(time.Minute)
	generation := s.generation
	providerCtx := s.ctx
	cred := m.secrets[id]
	now := m.opts.Now()
	identity, commander, fid := m.identity, m.name, m.fid

	m.mu.Unlock()
	requestCtx, cancel := context.WithTimeout(providerCtx, 8*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	defer func() { m.mu.Lock(); s.busy = false; m.mu.Unlock() }()
	endpoint := cap.Endpoint
	method := "GET"
	ct := ""
	var body []byte
	bearer := ""
	switch id {
	case "edsm-reference":
		var err error
		endpoint, err = m.edsmReferenceEndpoint(requestCtx, kind, query)
		if err != nil {
			return nil, err
		}
	case "elitebgs", "raven", "canonn":
		var err error
		endpoint, err = communityEndpoint(id, kind, query, page)
		if err != nil {
			return nil, err
		}
	case "spansh":
		var err error
		endpoint, err = referenceEndpoint(id, kind, query)
		if err != nil {
			return nil, err
		}
	case "edsm":
		if cred.APIKey == "" || cred.Commander == "" {
			return nil, errors.New("Store EDSM account/key first")
		}
		endpoint = "https://www.edsm.net/api-logs-v1/get-logs"
		method = "POST"
		ct = "application/x-www-form-urlencoded"
		body = []byte(url.Values{"commanderName": {cred.Commander}, "apiKey": {cred.APIKey}, "startDateTime": {now.Add(-24 * time.Hour).UTC().Format("2006-01-02 15:04:05")}, "endDateTime": {now.UTC().Format("2006-01-02 15:04:05")}}.Encode())
	case "inara":
		if cred.APIKey == "" {
			return nil, errors.New("Store Inara key first")
		}
		method = "POST"
		ct = "application/json"
		app := m.opts.InaraAppName
		if app == "" {
			app = InaraApplicationName
		}
		eventName := "getCommanderProfile"
		var eventData any = map[string]any{"searchName": commander}
		if kind == "communitygoals" {
			eventName = "getCommunityGoalsRecent"
			eventData = []any{}
		} else if commander == "" {
			return nil, errors.New("Verified local commander name required for profile lookup")
		}
		body, _ = json.Marshal(map[string]any{"header": map[string]any{"appName": app, "appVersion": version, "isBeingDeveloped": true, "APIkey": cred.APIKey}, "events": []any{map[string]any{"eventName": eventName, "eventTimestamp": now.UTC().Format(time.RFC3339), "eventData": eventData}}})
	case "edastro":
		if kind == "gec" {
			endpoint = "https://edastro.com/gec/json/rare"
		} else {
			if len(query) < 1 || len(query) > 128 || strings.ContainsAny(query, "\r\n\x00,") {
				return nil, errors.New("Enter one bounded system name")
			}
			endpoint = "https://edastro.com/api/starsystem?q=" + url.QueryEscape(query)
		}
	case "frontier":
		if cred.AccessToken == "" || cred.RefreshToken == "" {
			return nil, errors.New("Connect an approved Frontier application first")
		}
		if now.After(cred.ExpiresAt.Add(-30 * time.Second)) {
			form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {cred.RefreshToken}, "client_id": {m.clientID()}}
			raw, status, err := m.request(requestCtx, "POST", "https://auth.frontierstore.net/token", "application/x-www-form-urlencoded", []byte(form.Encode()), "")
			if err != nil || status != 200 || m.storeTokens(raw, generation, cred.RefreshToken) != nil {
				return nil, errors.New("Frontier refresh failed; reconnect")
			}
			m.mu.Lock()
			cred = m.secrets[id]
			m.mu.Unlock()
		}
		endpoint += "/" + kind
		bearer = cred.AccessToken
	}
	raw, status, err := m.request(requestCtx, method, endpoint, ct, body, bearer)
	if err != nil || status != 200 {
		m.mu.Lock()
		if generation == s.generation {
			s.lastError = "Read failed; no server response or secret logged"
		}
		m.mu.Unlock()
		return nil, errors.New("Provider read failed")
	}
	if (id == "edsm" || id == "inara") && !accepted(id, raw) {
		return nil, errors.New("Provider did not authorize or return the requested data")
	}
	snapshot := &Snapshot{Source: id + " / " + kind, FetchedAt: now.UTC().Format(time.RFC3339), ExpiresAt: now.Add(5 * time.Minute).UTC().Format(time.RFC3339), Facts: []Fact{}}
	if id == "spansh" {
		parsed, err := normalizeSpansh(kind, query, raw)
		if err != nil {
			return nil, err
		}
		parsed.FetchedAt = snapshot.FetchedAt
		parsed.ExpiresAt = now.Add(6 * time.Hour).UTC().Format(time.RFC3339)
		snapshot = parsed
	}
	if id == "elitebgs" || id == "canonn" || id == "raven" {
		parsed, err := normalizeCommunity(id, kind, query, raw)
		if err != nil {
			return nil, err
		}
		parsed.FetchedAt = snapshot.FetchedAt
		parsed.ExpiresAt = now.Add(time.Hour).UTC().Format(time.RFC3339)
		snapshot = parsed
	}
	if id == "edsm-reference" {
		m.mu.Lock()
		if generation != s.generation || !s.config.Read || s.config.Paused || m.closed {
			m.mu.Unlock()
			return nil, errors.New("Read consent revoked")
		}
		parsed, err := m.normalizeEDSMReference(requestCtx, kind, query, raw, now, fid)
		m.mu.Unlock()
		if err != nil {
			return nil, err
		}
		parsed.FetchedAt = snapshot.FetchedAt
		parsed.ExpiresAt = now.Add(6 * time.Hour).UTC().Format(time.RFC3339)
		snapshot = parsed
	}
	add := func(name string, value any) {
		if value == nil || len(snapshot.Facts) >= 100 {
			return
		}
		s := fmt.Sprint(value)
		for _, secret := range []string{cred.APIKey, cred.AccessToken, cred.RefreshToken} {
			if secret != "" {
				s = strings.ReplaceAll(s, secret, "[redacted]")
			}
		}
		if len(s) > 160 {
			s = s[:160]
		}
		snapshot.Facts = append(snapshot.Facts, Fact{Name: name, Value: s})
	}
	data := object(raw)
	if data == nil && !(id == "edsm-reference" && kind == "nearby") && !(id == "edastro" && kind == "gec") && id != "canonn" && id != "raven" && !(id == "elitebgs" && kind == "ticks") {
		return nil, errors.New("Unsupported or oversized provider response")
	}
	if (id == "edsm" || id == "inara") && identity != "" && commander != "" && cred.Commander == commander {
		snapshot.commanderHash = identity
	}
	if id == "frontier" && kind == "profile" {
		if profile, ok := data["commander"].(map[string]any); ok && text(profile["name"]) == commander && commander != "" {
			snapshot.commanderHash = identity
		}
	}
	if id == "edsm" {
		if a, ok := data["logs"].([]any); ok {
			for _, x := range a {
				if v, ok := x.(map[string]any); ok {
					add(text(v["system"]), text(v["date"]))
				}
			}
		}
	}
	if id == "inara" && kind == "profile" {
		if a, ok := data["events"].([]any); ok && len(a) > 0 {
			if v, ok := a[0].(map[string]any); ok {
				if d, ok := v["eventData"].(map[string]any); ok {
					add("Commander", text(d["commanderName"]))
					add("Inara name", text(d["userName"]))
					if ranks, ok := d["commanderRanksPilot"].([]any); ok {
						for _, x := range ranks {
							if r, ok := x.(map[string]any); ok {
								add(text(r["rankName"]), r["rankValue"])
							}
						}
					}
				}
			}
		}
	}
	if id == "inara" && kind == "communitygoals" {
		events, _ := data["events"].([]any)
		if len(events) != 1 {
			return nil, errors.New("Expected one public goals response")
		}
		event, _ := events[0].(map[string]any)
		goals, ok := event["eventData"].([]any)
		if !ok || len(goals) > 32 {
			return nil, errors.New("Unsupported public goals response")
		}
		for _, item := range goals {
			g, _ := item.(map[string]any)
			for _, k := range []string{"communitygoalName", "starsystemName", "stationName", "goalExpiry", "tierReached", "tierMax", "contributorsNum", "contributionsTotal", "isCompleted", "lastUpdate"} {
				add(k, g[k])
			}
		}
	}
	var layers *Layers
	if id == "edastro" && kind == "gec" {
		var a []map[string]any
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.UseNumber()
		if dec.Decode(&a) != nil || dec.Decode(new(any)) != io.EOF {
			return nil, errors.New("Unsupported GEC response")
		}
		l := Layers{Source: "EDAstro GEC rare-POI subset — data provided for application use; no imagery copied", FetchedAt: snapshot.FetchedAt, ExpiresAt: now.Add(24 * time.Hour).UTC().Format(time.RFC3339)}
		for _, o := range a {
			if len(l.Points) >= 2048 {
				l.Truncated = true
				break
			}
			pos, ok := o["coordinates"].([]any)
			if !ok {
				continue
			}
			if !validPosition(pos) {
				continue
			}
			var p Point
			for i, x := range pos {
				p.Coordinates[i], _ = x.(json.Number).Float64()
			}
			p.SystemName = text(o["galMapSearch"])
			p.Name = text(o["name"])
			if p.Name == "" {
				p.Name = text(o["galMapSearch"])
			}
			if p.Name == "" {
				continue
			}
			p.Category = "poi"
			if strings.Contains(strings.ToLower(text(o["type"])+" "+text(o["type2"])), "nebula") {
				p.Category = "nebula"
			}
			p.Source = "EDAstro GEC rare subset"
			l.Points = append(l.Points, p)
		}
		snapshot.Source = "edastro / gec (rare subset)"
		snapshot.ExpiresAt = l.ExpiresAt
		layers = &l
		add("Cached GEC points", len(l.Points))
		add("Truncated", l.Truncated)
	} else if id == "edastro" {
		for _, k := range []string{"name", "id64", "bodyCount"} {
			add(k, data[k])
		}
	}
	if id == "frontier" {
		for _, key := range []string{"commander", "ship", "lastSystem", "lastStarport", "carrier"} {
			if o, ok := data[key].(map[string]any); ok {
				for _, field := range []string{"name", "id", "credits", "docked", "value", "cargo", "capacity", "health", "callsign", "currentStarSystem", "fuel", "balance"} {
					v := o[field]
					switch v.(type) {
					case string, json.Number, bool:
						add(key+" / "+field, v)
					}
				}
			}
		}
		for _, key := range []string{"ships", "commodities", "modules"} {
			switch a := data[key].(type) {
			case []any:
				add(key+" count", len(a))
			case map[string]any:
				add(key+" count", len(a))
			}
		}
		add("Snapshot note", "Supplemental CAPI data may lag; never overwrites journal or republishes market data")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if generation != s.generation || !s.config.Read || s.config.Paused || m.closed {
		return nil, errors.New("Read consent was revoked")
	}
	if id == "frontier" && kind == "communitygoals" {
		add("Community goals", "Documented read endpoint; response-field normalization awaits verified authorized fixture. No account/journal state inferred.")
	}
	if m.history != nil && kind == "profile" {
		if err := m.persistCommanderRead(requestCtx, id, kind, fid, commander, data, now); err != nil {
			return nil, err
		}
	}
	if m.history != nil && id == "elitebgs" && kind == "ticks" {
		if err := m.history.SaveFieldTicks(requestCtx, raw, now); err != nil {
			return nil, err
		}
	}
	if ((id == "inara" && kind == "communitygoals" && s.config.CommunityGoals) || id == "spansh" || id == "elitebgs" || id == "canonn" || id == "raven" || id == "edsm-reference") && m.history != nil {
		encoded, _ := json.Marshal(snapshot)
		expires, _ := time.Parse(time.RFC3339, snapshot.ExpiresAt)
		if err := m.history.SaveReference(requestCtx, id, cacheKey, encoded, now, expires); err != nil {
			return nil, err
		}
		if id == "spansh" && kind == "station" {
			station, err := spanshStation(raw, snapshot)
			if err != nil {
				return nil, err
			}
			if err = m.history.SaveStation(station); err != nil {
				return nil, err
			}
		}
	}
	s.snapshot = snapshot
	s.lastSuccess = now.UTC().Format(time.RFC3339)
	s.lastError = ""
	s.noNewLast = false
	if layers != nil {
		m.layers = *layers
	}
	copy := *snapshot
	copy.Facts = slices.Clone(snapshot.Facts)
	return &copy, nil
}
