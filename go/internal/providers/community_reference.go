package providers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const ravenHost = "ravencolonial100-awcbdvabgze4c5cq.canadacentral-01.azurewebsites.net"

var canonnCollections = map[string]string{"guardian": "grsites", "thargoid": "tssites", "biology": "bmsites", "generation": "gensites"}

func communityEndpoint(id, kind, query string, page int) (string, error) {
	if page < 1 || page > 10 {
		return "", errors.New("Page must be between 1 and 10")
	}
	switch id {
	case "elitebgs":
		if kind == "ticks" {
			return "https://elitebgs.app/api/ebgs/v5/ticks", nil
		}
		if kind != "systems" && kind != "factions" && kind != "stations" {
			break
		}
		if query == "" || len(query) > 128 || strings.ContainsAny(query, "\x00\r\n,;") {
			break
		}
		return "https://elitebgs.app/api/ebgs/v5/" + kind + "?" + url.Values{"name": {query}, "page": {strconv.Itoa(page)}, "count": {"20"}}.Encode(), nil
	case "canonn":
		collection := canonnCollections[kind]
		if collection == "" || !canonicalID(query) {
			break
		}
		return "https://api.canonn.tech/" + collection + "?" + url.Values{"system.id64": {query}, "_limit": {"32"}, "_start": {strconv.Itoa((page - 1) * 32)}}.Encode(), nil
	case "raven":
		if kind == "system" && canonicalID(query) {
			return "https://" + ravenHost + "/api/system/" + query, nil
		}
		if kind == "project" || kind == "last" || kind == "stats" {
			if len(query) != 36 {
				break
			}
			for i, c := range query {
				if i == 8 || i == 13 || i == 18 || i == 23 {
					if c != '-' {
						return "", errors.New("Project UUID required")
					}
				} else if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
					return "", errors.New("Project UUID required")
				}
			}
			path := "https://" + ravenHost + "/api/project/" + query
			if kind != "project" {
				path += "/" + kind
			}
			return path, nil
		}
	}
	return "", errors.New("Choose a documented bounded reference query")
}
func communityPath(u *url.URL) bool {
	switch u.Host {
	case "elitebgs.app":
		return u.Path == "/api/ebgs/v5/systems" || u.Path == "/api/ebgs/v5/factions" || u.Path == "/api/ebgs/v5/stations" || u.Path == "/api/ebgs/v5/ticks"
	case "api.canonn.tech":
		for _, p := range canonnCollections {
			if u.Path == "/"+p {
				return true
			}
		}
	case ravenHost:
		parts := strings.Split(u.Path, "/")
		if len(parts) == 4 && parts[1] == "api" && parts[2] == "system" && canonicalID(parts[3]) {
			return true
		}
		if (len(parts) == 4 || len(parts) == 5) && parts[1] == "api" && parts[2] == "project" {
			kind := "project"
			if len(parts) == 5 {
				kind = parts[4]
			}
			_, err := communityEndpoint("raven", kind, parts[3], 1)
			return err == nil
		}
	}
	return false
}
func normalizeCommunity(id, kind, query string, raw []byte) (*Snapshot, error) {
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	var root any
	if d.Decode(&root) != nil {
		return nil, errors.New("Invalid reference JSON")
	}
	snapshot := &Snapshot{Source: id + " / " + kind, Facts: []Fact{}}
	add := func(name string, value any) {
		if len(snapshot.Facts) >= 100 {
			return
		}
		var s string
		switch x := value.(type) {
		case string:
			s = text(x)
		case json.Number:
			s = string(x)
		case bool:
			s = strconv.FormatBool(x)
		default:
			return
		}
		if s != "" {
			snapshot.Facts = append(snapshot.Facts, Fact{Name: name, Value: s})
		}
	}
	switch id {
	case "elitebgs":
		var docs []any
		if kind == "ticks" {
			docs, _ = root.([]any)
		} else {
			o, _ := root.(map[string]any)
			docs, _ = o["docs"].([]any)
			add("page", o["page"])
			add("pages", o["pages"])
			add("total", o["total"])
		}
		if docs == nil {
			return nil, errors.New("Unsupported Elite BGS page envelope")
		}
		if len(docs) > 32 {
			return nil, errors.New("Reference page exceeds limit")
		}
		for i, item := range docs {
			o, _ := item.(map[string]any)
			if kind != "ticks" && !strings.EqualFold(text(o["name"]), query) {
				return nil, errors.New("Elite BGS returned a different named entity")
			}
			for _, k := range []string{"name", "system", "system_address", "market_id", "government", "allegiance", "state", "controlling_minor_faction_cased", "updated_at", "time"} {
				add(fmt.Sprintf("%d / %s", i+1, k), o[k])
			}
			if a, ok := o["faction_presence"].([]any); ok && len(a) <= 32 {
				for _, x := range a {
					f, _ := x.(map[string]any)
					add("system", f["system_name"])
					add("influence", f["influence"])
					add("presence updated", f["updated_at"])
				}
			}
		}
	case "canonn":
		docs, ok := root.([]any)
		if !ok || len(docs) > 32 {
			return nil, errors.New("Unsupported Canonn collection")
		}
		for _, item := range docs {
			o, _ := item.(map[string]any)
			system, _ := o["system"].(map[string]any)
			if text(system["id64"]) != query {
				return nil, errors.New("Canonn site lacks exact requested system association")
			}
			add("site", o["siteID"])
			add("system", system["systemName"])
			add("verified", o["verified"])
			add("updated", o["updatedAt"])
			for _, k := range []string{"edsmCoordX", "edsmCoordY", "edsmCoordZ"} {
				add(k, system[k])
			}
			add("latitude", o["latitude"])
			add("longitude", o["longitude"])
		}
	case "raven":
		// Raven project schemas expose commander-linked data too. Return only these
		// project progress fields, never contributor/commander lists or arbitrary JSON.
		objects := []any{}
		switch x := root.(type) {
		case []any:
			objects = x
		case map[string]any:
			objects = []any{x}
		case string:
			add("last change", x)
		}
		if len(objects) > 32 {
			return nil, errors.New("Project response exceeds limit")
		}
		for _, item := range objects {
			o, _ := item.(map[string]any)
			if kind != "system" && text(o["buildId"]) != query {
				return nil, errors.New("Raven project identity mismatch")
			}
			for _, k := range []string{"buildId", "systemAddress", "marketId", "systemName", "buildName", "complete", "sumNeed", "maxNeed", "timestamp", "totalCargo", "totalDeliveries", "start", "end"} {
				add(k, o[k])
			}
		}
	}
	add("Evidence", "Contract-fixture adapter; live acceptance has not been established")
	return snapshot, nil
}
