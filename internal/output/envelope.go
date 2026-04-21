package output

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const SchemaVersion = "huectl/v1"

type Meta struct {
	Schema         string   `json:"schema"`
	Command        string   `json:"command"`
	RequestID      string   `json:"request_id"`
	Timestamp      string   `json:"timestamp"`
	BridgeScope    []string `json:"bridge_scope"`
	DurationMS     int64    `json:"duration_ms"`
	PartialSuccess bool     `json:"partial_success"`
}

type ErrorData struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Hints   []string       `json:"hints,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

type Envelope struct {
	Meta  Meta       `json:"meta"`
	Data  any        `json:"data"`
	Error *ErrorData `json:"error"`
}

func (m Meta) filtered(verbosity int) Meta {
	filtered := Meta{
		Schema:         m.Schema,
		Command:        m.Command,
		PartialSuccess: m.PartialSuccess,
	}
	if verbosity >= 2 {
		filtered.RequestID = m.RequestID
		filtered.Timestamp = m.Timestamp
		filtered.BridgeScope = append([]string(nil), m.BridgeScope...)
		filtered.DurationMS = m.DurationMS
	}
	return filtered
}

func (e ErrorData) filtered(verbosity int) ErrorData {
	filtered := ErrorData{
		Code:    e.Code,
		Message: e.Message,
	}
	if len(e.Hints) > 0 {
		filtered.Hints = append([]string(nil), e.Hints...)
	}
	if verbosity >= 2 && len(e.Details) > 0 {
		filtered.Details = cloneMap(e.Details)
	}
	return filtered
}

func BuildSuccess(command string, bridgeScope []string, started time.Time, data any, partial bool) Envelope {
	return Envelope{
		Meta: buildMeta(command, bridgeScope, started, partial),
		Data: data,
	}
}

func BuildError(command string, bridgeScope []string, started time.Time, err ErrorData, partial bool) Envelope {
	return Envelope{
		Meta:  buildMeta(command, bridgeScope, started, partial),
		Data:  nil,
		Error: &err,
	}
}

func (e Envelope) MarshalIndented() ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}

func buildMeta(command string, bridgeScope []string, started time.Time, partial bool) Meta {
	now := time.Now().UTC()
	if started.IsZero() {
		started = now
	}
	return Meta{
		Schema:         SchemaVersion,
		Command:        command,
		RequestID:      uuid.NewString(),
		Timestamp:      now.Format(time.RFC3339),
		BridgeScope:    bridgeScope,
		DurationMS:     now.Sub(started).Milliseconds(),
		PartialSuccess: partial,
	}
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
