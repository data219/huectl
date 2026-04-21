package output

import (
	"sort"
	"strings"

	"github.com/data219/huectl/internal/domain"
)

type RenderOptions struct {
	JSON      bool
	Verbosity int
}

type AggregateView struct {
	HasResourcePayload bool                  `json:"-"`
	ResourceRows       []map[string]any      `json:"resource_rows,omitempty"`
	BridgeRows         []domain.BridgeResult `json:"bridge_rows,omitempty"`
	BridgeErrors       []domain.BridgeResult `json:"bridge_errors,omitempty"`
	Summary            map[string]any        `json:"summary,omitempty"`
}

type ViewEnvelope struct {
	Meta  Meta       `json:"meta"`
	Data  any        `json:"data"`
	Error *ErrorData `json:"error"`
}

func BuildView(envelope Envelope, opts RenderOptions) ViewEnvelope {
	view := ViewEnvelope{
		Meta: envelope.Meta.filtered(opts.Verbosity),
	}
	if envelope.Error != nil {
		filteredErr := envelope.Error.filtered(opts.Verbosity)
		view.Error = &filteredErr
	}
	if envelope.Data != nil {
		view.Data = buildViewData(envelope.Meta.Command, envelope.Data, opts.Verbosity)
	}
	return view
}

func buildViewData(command string, data any, verbosity int) any {
	switch typed := data.(type) {
	case domain.AggregateResult:
		return buildAggregateView(command, typed, verbosity)
	case *domain.AggregateResult:
		if typed == nil {
			return nil
		}
		return buildAggregateView(command, *typed, verbosity)
	default:
		return data
	}
}

func buildAggregateView(command string, result domain.AggregateResult, verbosity int) AggregateView {
	items := sortedBridgeResults(result.Items)
	resourceRows, hasResourcePayload := extractResourceRows(items)

	view := AggregateView{}
	if hasResourcePayload {
		view.HasResourcePayload = true
		view.ResourceRows = filterResourceRows(command, resourceRows, verbosity)
		view.BridgeErrors = failedBridgeResults(items)
	} else {
		view.BridgeRows = items
		view.BridgeErrors = failedBridgeResults(items)
	}
	if verbosity >= 2 {
		view.Summary = cloneMap(result.Summary)
	}
	return view
}

func sortedBridgeResults(items []domain.BridgeResult) []domain.BridgeResult {
	sorted := append([]domain.BridgeResult(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		left := strings.ToLower(firstNonEmpty(sorted[i].BridgeName, sorted[i].BridgeID))
		right := strings.ToLower(firstNonEmpty(sorted[j].BridgeName, sorted[j].BridgeID))
		if left == right {
			return sorted[i].BridgeID < sorted[j].BridgeID
		}
		return left < right
	})
	return sorted
}

func filterResourceRows(command string, rows []map[string]any, verbosity int) []map[string]any {
	includeStatus := false
	for _, row := range rows {
		if renderScalar(row["status"]) != "-" {
			includeStatus = true
			break
		}
	}

	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		viewRow := map[string]any{
			"bridge": row["bridge"],
			"name":   row["name"],
		}
		if includeStatus && renderScalar(row["status"]) != "-" {
			viewRow["status"] = row["status"]
		}
		if verbosity >= 1 {
			if rendered := renderScalar(row["id"]); rendered != "-" {
				viewRow["id"] = row["id"]
			}
			if rendered := renderScalar(row["details"]); rendered != "-" {
				viewRow["details"] = row["details"]
			}
			if shouldIncludeResourceType(command) && renderScalar(row["type"]) != "-" {
				viewRow["type"] = row["type"]
			}
		}
		filtered = append(filtered, viewRow)
	}
	return filtered
}

func shouldIncludeResourceType(command string) bool {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) >= 2 && parts[1] == "list" {
		return false
	}
	return true
}
