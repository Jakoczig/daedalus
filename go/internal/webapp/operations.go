package webapp

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/history"
	"strings"
)

func (s *Server) operationsRPC(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if s.game == nil {
		return nil, errors.New("Journal service unavailable")
	}
	db, _ := s.game.History()
	if db == nil {
		return nil, errors.New("History database not ready")
	}
	var q history.OperationQuery
	if strictDecode(raw, &q) != nil {
		return nil, errors.New("Invalid operations request")
	}
	fid := s.game.FieldCommander()
	if q.FID != "" {
		fid = q.FID
	}
	if name == "backfillGoOperationsSource" {
		if !q.Acknowledge {
			return nil, errors.New("Acknowledge verified original-source recovery; no uploads will be created")
		}
		n, e := db.RecoverOperations(ctx, q.ID)
		if e != nil {
			return nil, e
		}
		return map[string]any{"recovered": n, "notice": "Only hash-verified indexed payloads restored; checkpoints and publication permissions unchanged"}, nil
	}
	if name == "exportGoOperations" && !q.Acknowledge {
		return nil, errors.New("Preview this bounded private page and acknowledge names, locations and financial exposure before CSV export")
	}
	page, e := db.Operations(ctx, fid, q)
	if e != nil {
		return nil, e
	}
	encoded, _ := json.Marshal(struct {
		FID, Kind string
		Rows      []history.OperationRow
	}{fid, q.Kind, page.Rows})
	sum := sha256.Sum256(encoded)
	page.PreviewHash = fmt.Sprintf("%x", sum)
	if name == "exportGoOperations" && q.PreviewHash != page.PreviewHash {
		return nil, errors.New("History changed or preview missing; refresh and preview the selected page before export")
	}
	if name != "exportGoOperations" {
		return page, nil
	}
	var out strings.Builder
	w := csv.NewWriter(&out)
	_ = w.Write([]string{"record", "entity", "event", "observed_utc", "state_or_basis", "title", "observed_details"})
	for _, r := range page.Rows {
		row := []string{r.ID, r.Entity, r.Kind, r.At, r.State, r.Title, string(r.Details)}
		for i, v := range row {
			trim := strings.TrimLeft(v, " \t\r\n")
			if safeExportCell(trim) != trim {
				row[i] = "'" + v
			}
		}
		if e = w.Write(row); e != nil {
			return nil, e
		}
	}
	w.Flush()
	if w.Error() != nil {
		return nil, w.Error()
	}
	return map[string]string{"csv": out.String(), "notice": "Selected bounded page only; private owner export. No upload."}, nil
}
