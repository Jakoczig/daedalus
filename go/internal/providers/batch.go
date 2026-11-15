package providers

import (
	"encoding/json"
	"net/url"
)

// Authenticated providers document event batching. EDDN/EDAstro accept one
// observation per envelope and use their own independent queue/rate budget.
func batchRequest(id string, queue []queued) ([]queued, []byte) {
	first := queue[0]
	batch := []queued{first}
	body := first.body
	if id != "edsm" && id != "inara" {
		return batch, body
	}
	for n := 2; n <= len(queue) && n <= 16; n++ {
		if queue[n-1].generation != first.generation || queue[n-1].attempts != first.attempts {
			break
		}
		candidate := queue[:n]
		var b []byte
		if id == "edsm" {
			form, err := url.ParseQuery(string(first.body))
			if err != nil {
				break
			}
			events := []json.RawMessage{}
			for _, q := range candidate {
				f, _ := url.ParseQuery(string(q.body))
				var a []json.RawMessage
				if json.Unmarshal([]byte(f.Get("message")), &a) != nil {
					return batch, body
				}
				events = append(events, a...)
			}
			raw, _ := json.Marshal(events)
			form.Set("message", string(raw))
			b = []byte(form.Encode())
		} else {
			v := object(first.body)
			events := []any{}
			for _, q := range candidate {
				payload := object(q.body)
				a, ok := payload["events"].([]any)
				if !ok {
					return batch, body
				}
				events = append(events, a...)
			}
			v["events"] = events
			b, _ = json.Marshal(v)
		}
		if len(b) > 256<<10 {
			break
		}
		batch = append([]queued(nil), candidate...)
		body = b
	}
	return batch, body
}
