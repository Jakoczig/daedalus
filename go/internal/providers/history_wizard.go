package providers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"slices"
	"strings"
	"time"
)

type HistorySelection struct {
	FID         string `json:"fid"`
	Provider    string `json:"provider"`
	Category    string `json:"category"`
	From        string `json:"from"`
	To          string `json:"to"`
	Token       string `json:"token"`
	Acknowledge bool   `json:"acknowledge"`
}
type HistoryPreviewRow struct {
	ID        string          `json:"id"`
	Candidate string          `json:"candidate"`
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	System    string          `json:"system"`
	Status    string          `json:"status"`
	Reason    string          `json:"reason"`
	Fields    json.RawMessage `json:"fields,omitempty"`
	Bytes     int             `json:"bytes"`
}
type HistoryPreview struct {
	Rows   []HistoryPreviewRow `json:"rows"`
	Token  string              `json:"token"`
	Notice string              `json:"notice"`
	Queued int                 `json:"queued"`
}

// A preview is never a network operation. Confirmation re-evaluates the exact
// bounded selection, current policy, provider consent and account under one lock.
func (m *Manager) Historical(ctx context.Context, selection HistorySelection, confirm bool) (HistoryPreview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := HistoryPreview{Rows: []HistoryPreviewRow{}, Notice: "At most 32 committed records per review. Provider sees your IP, account and selected location/ship fields. No automatic retry of uncertain delivery. Only Inara has a verified <=30-day historical contract; other recipients stay private."}
	if m.closed || m.history == nil {
		return out, errors.New("Database not ready")
	}
	state := m.states[selection.Provider]
	if state == nil || !slices.Contains(m.cap(selection.Provider).Upload, selection.Category) {
		return out, errors.New("Unsupported recipient/category")
	}
	from, e1 := time.Parse(time.RFC3339, selection.From)
	to, e2 := time.Parse(time.RFC3339, selection.To)
	if e1 != nil || e2 != nil {
		return out, errors.New("Choose a UTC time window")
	}
	records, err := m.history.PublicationRecords(ctx, selection.FID, from, to, 32)
	if err != nil {
		return out, err
	}
	cred := m.secrets[selection.Provider]
	jobs := []queued{}
	now := m.opts.Now()
	for _, r := range records {
		if category(selection.Provider, r.Event.Type) != selection.Category {
			continue
		}
		candidate, err := m.history.Candidate(selection.FID, selection.Provider, selection.Category, r.Event, journal.Replay, true, now)
		if err != nil {
			return out, err
		}
		row := HistoryPreviewRow{ID: r.ID, Candidate: candidate.ID, Type: r.Event.Type, Timestamp: r.Event.Timestamp.UTC().Format(time.RFC3339), System: r.Event.SystemName, Status: candidate.Status, Reason: candidate.Reason}
		permitted := state.config.Upload && !state.config.Paused && state.config.CommanderHash == hash(selection.FID) && slices.Contains(state.config.Categories, selection.Category) && m.cap(selection.Provider).Gate == "" && cred.APIKey != "" && cred.Commander == r.Name
		if !permitted {
			row.Status = "held"
			row.Reason = "Independent provider/category approval, consent and exact account match required"
		}
		if !strings.HasPrefix(r.Version, "4.") || strings.ContainsAny(strings.ToLower(r.Version), "abcdefghijklmnopqrstuvwxyz") {
			row.Status = "held"
			row.Reason = "Source game version is not verified Live galaxy"
		}
		if row.Status == "eligible" {
			encoder := &Manager{opts: m.opts, gameVersion: r.Version, name: r.Name, identity: hash(selection.FID), uploader: m.uploader}
			body, endpoint, ct, err := encoder.encode(selection.Provider, r.Event, object(r.Event.Raw), cred)
			if err != nil {
				row.Status = "held"
				row.Reason = "Unsupported or incomplete event fields"
			} else {
				// Only the event array is returned, never credential-bearing request headers.
				root := object(body)
				fields, _ := json.Marshal(root["events"])
				row.Fields = fields
				row.Bytes = len(body)
				jobs = append(jobs, queued{candidateID: candidate.ID, key: hash(selection.Provider + "/" + hash(selection.FID) + "/" + string(r.Event.Raw)), category: selection.Category, body: body, endpoint: endpoint, contentType: ct, generation: state.generation})
			}
		}
		out.Rows = append(out.Rows, row)
	}
	b, _ := json.Marshal(struct {
		FID, Provider, Category, From, To string
		Rows                              []HistoryPreviewRow
		Generation                        uint64
	}{selection.FID, selection.Provider, selection.Category, selection.From, selection.To, out.Rows, state.generation})
	out.Token = hash(string(b))
	if !confirm {
		return out, nil
	}
	if !selection.Acknowledge || selection.Token == "" || selection.Token != out.Token {
		return out, errors.New("Review changed or disclosure not acknowledged; preview again")
	}
	if len(state.queue)+len(jobs) > 64 {
		return out, errors.New("Queue full; no records authorized")
	}
	total := 0
	for _, q := range append(slices.Clone(state.queue), jobs...) {
		total += len(q.body)
	}
	if total > 2<<20 {
		return out, errors.New("Queue byte limit; narrow selection")
	}
	for _, q := range jobs {
		if _, seen := m.ledger[q.key]; seen {
			continue
		}
		if slices.ContainsFunc(state.queue, func(v queued) bool { return v.candidateID == q.candidateID }) {
			continue
		}
		if err = m.history.CandidateOutcome(q.candidateID, "queued", "Owner confirmed exact historical preview"); err != nil {
			return out, err
		}
		state.queue = append(state.queue, q)
		out.Queued++
	}
	if out.Queued > 0 {
		select {
		case state.wake <- struct{}{}:
		default:
		}
	}
	return out, nil
}
