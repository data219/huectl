package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/data219/huectl/internal/domain"
)

func Write(w io.Writer, jsonMode bool, envelope Envelope) error {
	return WriteWithVerbosity(w, jsonMode, 0, envelope)
}

func WriteWithVerbosity(w io.Writer, jsonMode bool, verbosity int, envelope Envelope) error {
	view := BuildView(envelope, RenderOptions{JSON: jsonMode, Verbosity: verbosity})

	if jsonMode {
		payload, err := json.MarshalIndent(view, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal view envelope: %w", err)
		}
		_, err = fmt.Fprintln(w, string(payload))
		return err
	}

	return writeHumanView(w, view)
}

func writeHumanView(w io.Writer, view ViewEnvelope) error {
	if view.Error != nil {
		if _, err := fmt.Fprintf(w, "ERROR [%s] %s\n", view.Error.Code, view.Error.Message); err != nil {
			return err
		}
		for _, hint := range view.Error.Hints {
			if _, err := fmt.Fprintf(w, "HINT: %s\n", hint); err != nil {
				return err
			}
		}
		if len(view.Error.Details) > 0 {
			if _, err := fmt.Fprintln(w, "DETAILS:"); err != nil {
				return err
			}
			if err := writeMap(w, view.Error.Details); err != nil {
				return err
			}
		}
	}

	if view.Data != nil {
		if err := writeHumanData(w, view.Data); err != nil {
			return err
		}
	}

	if !shouldWriteHumanMeta(view.Meta) {
		return nil
	}

	_, err := fmt.Fprintf(w, "\nmeta: %s\n", formatHumanMeta(view.Meta))
	return err
}

func writeHumanData(w io.Writer, data any) error {
	switch typed := data.(type) {
	case AggregateView:
		return writeAggregateResult(w, typed)
	case *AggregateView:
		if typed == nil {
			_, err := fmt.Fprintln(w, "No data.")
			return err
		}
		return writeAggregateResult(w, *typed)
	case map[string]any:
		return writeMap(w, typed)
	case []map[string]any:
		return writeTableFromMaps(w, typed)
	default:
		_, err := fmt.Fprintf(w, "%v\n", data)
		return err
	}
}

func writeMap(w io.Writer, data map[string]any) error {
	if len(data) == 0 {
		_, err := fmt.Fprintln(w, "(empty)")
		return err
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := data[key]
		rendered := renderScalar(value)
		if _, err := fmt.Fprintf(w, "%-20s %s\n", key+":", rendered); err != nil {
			return err
		}
	}
	return nil
}

func writeTableFromMaps(w io.Writer, rows []map[string]any) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No rows.")
		return err
	}
	keysMap := map[string]struct{}{}
	for _, row := range rows {
		for key := range row {
			keysMap[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(keysMap))
	for key := range keysMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	headers := make([]string, 0, len(keys))
	for _, key := range keys {
		headers = append(headers, strings.ToUpper(key))
	}
	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		values := make([]string, 0, len(keys))
		for _, key := range keys {
			values = append(values, renderScalar(row[key]))
		}
		if _, err := fmt.Fprintln(tw, strings.Join(values, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeAggregateResult(w io.Writer, result AggregateView) error {
	if result.HasResourcePayload {
		if len(result.ResourceRows) == 0 {
			if _, err := fmt.Fprintln(w, "No resources."); err != nil {
				return err
			}
		} else {
			if err := writeResourceTable(w, result.ResourceRows); err != nil {
				return err
			}
		}
		if len(result.BridgeErrors) > 0 {
			if _, err := fmt.Fprintln(w, "\nbridge errors:"); err != nil {
				return err
			}
			if err := writeBridgeStatusTable(w, result.BridgeErrors); err != nil {
				return err
			}
		}
	} else {
		if len(result.BridgeRows) == 0 {
			_, err := fmt.Fprintln(w, "No bridge results.")
			return err
		}
		if err := writeBridgeStatusTable(w, result.BridgeRows); err != nil {
			return err
		}
	}

	summary := formatSummary(result.Summary)
	if summary == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, "\nsummary: %s\n", summary)
	return err
}

func writeBridgeStatusTable(w io.Writer, items []domain.BridgeResult) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "BRIDGE\tSTATUS\tHTTP\tDETAILS"); err != nil {
		return err
	}
	for _, item := range items {
		status := "OK"
		if !item.Success {
			status = "ERR"
		}
		httpStatus := "-"
		if item.StatusCode > 0 {
			httpStatus = fmt.Sprintf("%d", item.StatusCode)
		}
		details := summarizeBridgeResult(item)
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\n",
			firstNonEmpty(item.BridgeName, item.BridgeID),
			status,
			httpStatus,
			details,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func extractResourceRows(items []domain.BridgeResult) ([]map[string]any, bool) {
	rows := make([]map[string]any, 0)
	hasResourcePayload := false
	for _, item := range items {
		if !item.Success || item.Data == nil {
			continue
		}
		resources, ok := item.Data["resources"]
		if !ok {
			continue
		}
		hasResourcePayload = true
		for _, resource := range asResourceSlice(resources) {
			rows = append(rows, map[string]any{
				"bridge":  firstNonEmpty(item.BridgeName, item.BridgeID),
				"name":    extractResourceName(resource),
				"type":    renderScalar(resource["type"]),
				"id":      firstNonEmpty(toString(resource["id"]), toString(resource["id_v1"]), "-"),
				"status":  extractResourceStatus(resource),
				"details": extractResourceDetails(resource),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		left := strings.ToLower(rows[i]["bridge"].(string) + "|" + rows[i]["name"].(string) + "|" + rows[i]["id"].(string))
		right := strings.ToLower(rows[j]["bridge"].(string) + "|" + rows[j]["name"].(string) + "|" + rows[j]["id"].(string))
		return left < right
	})
	return rows, hasResourcePayload
}

func writeResourceTable(w io.Writer, rows []map[string]any) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No resources.")
		return err
	}

	columnSet := map[string]struct{}{}
	for _, row := range rows {
		for key := range row {
			columnSet[key] = struct{}{}
		}
	}
	columnOrder := []string{"bridge", "name", "type", "id", "status", "details"}
	columns := make([]string, 0, len(columnOrder))
	for _, column := range columnOrder {
		if _, ok := columnSet[column]; ok {
			columns = append(columns, column)
		}
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	headers := make([]string, 0, len(columns))
	for _, column := range columns {
		headers = append(headers, strings.ToUpper(column))
	}
	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		values := make([]string, 0, len(columns))
		for _, column := range columns {
			values = append(values, renderScalar(row[column]))
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\n",
			strings.Join(values, "\t"),
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func shouldWriteHumanMeta(meta Meta) bool {
	return meta.PartialSuccess || meta.RequestID != "" || meta.DurationMS > 0 || meta.Timestamp != "" || len(meta.BridgeScope) > 0
}

func formatHumanMeta(meta Meta) string {
	parts := []string{
		fmt.Sprintf("schema=%s", meta.Schema),
		fmt.Sprintf("command=%s", meta.Command),
	}
	if meta.RequestID != "" {
		parts = append(parts, fmt.Sprintf("request_id=%s", meta.RequestID))
	}
	if meta.Timestamp != "" {
		parts = append(parts, fmt.Sprintf("timestamp=%s", meta.Timestamp))
	}
	if len(meta.BridgeScope) > 0 {
		parts = append(parts, fmt.Sprintf("bridge_scope=%s", strings.Join(meta.BridgeScope, ",")))
	}
	parts = append(parts, fmt.Sprintf("partial_success=%t", meta.PartialSuccess))
	parts = append(parts, fmt.Sprintf("duration_ms=%d", meta.DurationMS))
	return strings.Join(parts, " ")
}

func failedBridgeResults(items []domain.BridgeResult) []domain.BridgeResult {
	failed := make([]domain.BridgeResult, 0)
	for _, item := range items {
		if !item.Success {
			failed = append(failed, item)
		}
	}
	return failed
}

func asResourceSlice(raw any) []map[string]any {
	switch typed := raw.(type) {
	case []map[string]any:
		return typed
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if asMap, ok := item.(map[string]any); ok {
				rows = append(rows, asMap)
			}
		}
		return rows
	default:
		return nil
	}
}

func extractResourceName(resource map[string]any) string {
	if metadata, ok := resource["metadata"].(map[string]any); ok {
		if name, ok := metadata["name"].(string); ok && strings.TrimSpace(name) != "" {
			return name
		}
	}
	if name, ok := resource["name"].(string); ok && strings.TrimSpace(name) != "" {
		return name
	}
	return "-"
}

func extractResourceStatus(resource map[string]any) string {
	if onMap, ok := resource["on"].(map[string]any); ok {
		if on, ok := onMap["on"].(bool); ok {
			if on {
				return "on"
			}
			return "off"
		}
	}
	if enabled, ok := resource["enabled"].(bool); ok {
		if enabled {
			return "enabled"
		}
		return "disabled"
	}
	if status, ok := resource["status"].(string); ok && strings.TrimSpace(status) != "" {
		return status
	}
	return "-"
}

func extractResourceDetails(resource map[string]any) string {
	parts := make([]string, 0, 3)
	if children, ok := resource["children"].([]any); ok && len(children) > 0 {
		parts = append(parts, fmt.Sprintf("children=%d", len(children)))
	}
	if actions, ok := resource["actions"].([]any); ok && len(actions) > 0 {
		parts = append(parts, fmt.Sprintf("actions=%d", len(actions)))
	}
	if metadata, ok := resource["metadata"].(map[string]any); ok {
		if archetype, ok := metadata["archetype"].(string); ok && strings.TrimSpace(archetype) != "" {
			parts = append(parts, fmt.Sprintf("archetype=%s", archetype))
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func toString(v any) string {
	if asString, ok := v.(string); ok {
		return strings.TrimSpace(asString)
	}
	return ""
}

func summarizeBridgeResult(item domain.BridgeResult) string {
	if !item.Success {
		return firstNonEmpty(item.Error, "request failed")
	}
	if item.Data == nil {
		return "ok"
	}
	asMap := item.Data

	if resources, ok := asMap["resources"]; ok {
		switch typed := resources.(type) {
		case []map[string]any:
			return fmt.Sprintf("resources=%d", len(typed))
		case []any:
			return fmt.Sprintf("resources=%d", len(typed))
		}
	}
	if source, ok := asMap["source"]; ok {
		return fmt.Sprintf("source=%s", renderScalar(source))
	}
	if response, ok := asMap["response"]; ok && response != nil {
		return "response=ok"
	}
	return "ok"
}

func formatSummary(summary map[string]any) string {
	if len(summary) == 0 {
		return ""
	}
	preferredKeys := []string{"bridges_total", "bridges_success", "bridges_failed"}
	parts := make([]string, 0, len(summary))
	used := map[string]struct{}{}
	for _, key := range preferredKeys {
		if value, ok := summary[key]; ok {
			parts = append(parts, fmt.Sprintf("%s=%v", key, value))
			used[key] = struct{}{}
		}
	}
	extras := make([]string, 0)
	for key := range summary {
		if _, ok := used[key]; ok {
			continue
		}
		extras = append(extras, key)
	}
	sort.Strings(extras)
	for _, key := range extras {
		parts = append(parts, fmt.Sprintf("%s=%v", key, summary[key]))
	}
	return strings.Join(parts, " ")
}

func renderScalar(v any) string {
	if v == nil {
		return "-"
	}
	switch typed := v.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return "-"
		}
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case []domain.Bridge:
		return fmt.Sprintf("%d items", len(typed))
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", key, renderScalar(typed[key])))
		}
		return strings.Join(parts, ", ")
	case []any:
		if len(typed) == 0 {
			return "-"
		}
		scalarOnly := true
		for _, item := range typed {
			if !isScalar(item) {
				scalarOnly = false
				break
			}
		}
		if !scalarOnly {
			return fmt.Sprintf("%d items", len(typed))
		}
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ", ")
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			return fmt.Sprintf("%d items", rv.Len())
		}
		return fmt.Sprintf("%v", v)
	}
}

func isScalar(v any) bool {
	switch v.(type) {
	case nil, string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
