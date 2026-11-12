package journal

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxRecordBytes = 1 << 20

type SourcePosition struct {
	Path, FileID string
	Offset       int64
}
type Event struct {
	Type                                                             string
	Timestamp                                                        time.Time
	GameVersion, GameBuild, FID, SystemName, SystemAddress, MarketID string
	Raw                                                              json.RawMessage
	Position                                                         SourcePosition
}

func ParseLine(line []byte, source SourcePosition) (Event, error) {
	var event Event
	if len(line) > MaxRecordBytes || !utf8.Valid(line) {
		return event, errors.New("journal record exceeds limit or has invalid encoding")
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	var value map[string]any
	if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF {
		return event, errors.New("invalid complete journal record")
	}
	kind, ok := value["event"].(string)
	if !ok || kind == "" || len(kind) > 128 {
		return event, errors.New("invalid journal event category")
	}
	if strings.EqualFold(kind, "fileheader") {
		kind = "Fileheader"
	}
	event.Type = kind
	event.Position = source
	text := func(key string) string { v, _ := value[key].(string); return v }
	if raw, ok := value["timestamp"]; ok {
		stamp, ok := raw.(string)
		if !ok {
			return Event{}, errors.New("invalid journal event time")
		}
		var err error
		event.Timestamp, err = time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			return Event{}, errors.New("invalid journal event time")
		}
	}
	event.GameVersion = text("gameversion")
	event.GameBuild = text("build")
	event.FID = text("FID")
	event.SystemName = text("StarSystem")
	for key, out := range map[string]*string{"SystemAddress": &event.SystemAddress, "MarketID": &event.MarketID} {
		if raw, ok := value[key]; ok {
			var digits string
			switch v := raw.(type) {
			case json.Number:
				digits = string(v)
			case string:
				digits = v
			default:
				return Event{}, errors.New("invalid journal identifier")
			}
			id, err := strconv.ParseUint(digits, 10, 64)
			if err != nil || strings.ContainsAny(digits, "+-.eE") {
				return Event{}, errors.New("invalid journal identifier")
			}
			*out = strconv.FormatUint(id, 10)
		}
	}
	event.Raw = append(json.RawMessage(nil), bytes.TrimSpace(line)...)
	return event, nil
}
