package commands

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/data219/huectl/internal/app"
	"github.com/data219/huectl/internal/domain"
	"github.com/spf13/cobra"
)

func ParseActionInput(cmd *cobra.Command, domainName string, action string) (app.ActionInput, *domain.AppError) {
	input := app.ActionInput{}
	input.ID, _ = cmd.Flags().GetString("id")
	input.Name, _ = cmd.Flags().GetString("name")
	input.Address, _ = cmd.Flags().GetString("address")
	input.Path, _ = cmd.Flags().GetString("path")
	input.Method, _ = cmd.Flags().GetString("method")
	input.Duration, _ = cmd.Flags().GetDuration("duration")
	input.File, _ = cmd.Flags().GetString("file")

	switch domainName {
	case "api":
		return parseAPIRawInput(cmd, input)
	case "light":
		return parseLightInput(cmd, action, input)
	case "scene":
		return parseSceneInput(cmd, action, input)
	case "automation":
		return parseAutomationInput(cmd, action, input)
	case "sensor":
		return parseSensorInput(cmd, action, input)
	case "room", "zone":
		return parseGroupInput(cmd, action, input)
	default:
		return input, nil
	}
}

func parseAPIRawInput(cmd *cobra.Command, input app.ActionInput) (app.ActionInput, *domain.AppError) {
	bodyRaw, _ := cmd.Flags().GetString("body")
	bodyFile, _ := cmd.Flags().GetString("body-file")
	bodyRaw = strings.TrimSpace(bodyRaw)
	bodyFile = strings.TrimSpace(bodyFile)

	if bodyRaw != "" && bodyFile != "" {
		return input, &domain.AppError{
			Code:     "API_BODY_CONFLICT",
			Message:  "use either --body or --body-file, not both",
			ExitCode: domain.ExitUsage,
			Details:  map[string]any{"body_file": bodyFile},
		}
	}

	if bodyFile != "" {
		payload, readErr := os.ReadFile(bodyFile)
		if readErr != nil {
			return input, &domain.AppError{
				Code:     "BODY_FILE_READ",
				Message:  "failed to read --body-file",
				ExitCode: domain.ExitUsage,
				Details:  map[string]any{"body_file": bodyFile},
			}
		}
		bodyRaw = strings.TrimSpace(string(payload))
	}

	if bodyRaw == "" {
		return input, nil
	}
	input.RawBody = bodyRaw
	input.File = bodyFile

	var bodyMap map[string]any
	if err := json.Unmarshal([]byte(bodyRaw), &bodyMap); err == nil {
		input.Body = bodyMap
		input.RawAPI = &app.RawAPIInput{Method: input.Method, Path: input.Path, Body: bodyRaw}
		return input, nil
	}

	var scalar any
	if err := json.Unmarshal([]byte(bodyRaw), &scalar); err == nil {
		input.Body = map[string]any{"value": scalar}
		input.RawAPI = &app.RawAPIInput{Method: input.Method, Path: input.Path, Body: bodyRaw}
		return input, nil
	}

	return input, &domain.AppError{
		Code:     "INVALID_BODY",
		Message:  "--body must contain valid JSON",
		ExitCode: domain.ExitUsage,
		Details:  map[string]any{"body": bodyRaw},
	}
}

func parseLightInput(cmd *cobra.Command, action string, input app.ActionInput) (app.ActionInput, *domain.AppError) {
	if action != "set" && action != "effect" && action != "flash" {
		return input, nil
	}

	light := &app.LightSetInput{}
	hasPayload := false

	if cmd.Flags().Changed("on") {
		v, _ := cmd.Flags().GetBool("on")
		light.On = &v
		hasPayload = true
	}
	if cmd.Flags().Changed("brightness") {
		v, _ := cmd.Flags().GetInt("brightness")
		if v < 0 || v > 100 {
			return input, &domain.AppError{Code: "INVALID_BRIGHTNESS", Message: "brightness must be between 0 and 100", ExitCode: domain.ExitUsage}
		}
		light.Brightness = &v
		hasPayload = true
	}
	if cmd.Flags().Changed("kelvin") {
		v, _ := cmd.Flags().GetInt("kelvin")
		if v <= 0 {
			return input, &domain.AppError{Code: "INVALID_KELVIN", Message: "kelvin must be greater than 0", ExitCode: domain.ExitUsage}
		}
		light.Kelvin = &v
		hasPayload = true
	}
	if cmd.Flags().Changed("xy-x") {
		v, _ := cmd.Flags().GetFloat64("xy-x")
		light.XYX = &v
		hasPayload = true
	}
	if cmd.Flags().Changed("xy-y") {
		v, _ := cmd.Flags().GetFloat64("xy-y")
		light.XYY = &v
		hasPayload = true
	}
	if cmd.Flags().Changed("effect") {
		v, _ := cmd.Flags().GetString("effect")
		light.Effect = strings.TrimSpace(v)
		hasPayload = true
	}
	if cmd.Flags().Changed("transition-ms") {
		v, _ := cmd.Flags().GetInt("transition-ms")
		if v < 0 {
			return input, &domain.AppError{Code: "INVALID_TRANSITION", Message: "transition-ms must be >= 0", ExitCode: domain.ExitUsage}
		}
		light.TransitionMS = &v
		hasPayload = true
	}
	if cmd.Flags().Changed("alert-action") {
		v, _ := cmd.Flags().GetString("alert-action")
		light.AlertAction = strings.TrimSpace(v)
		hasPayload = true
	}
	if (light.XYX != nil && light.XYY == nil) || (light.XYX == nil && light.XYY != nil) {
		return input, &domain.AppError{Code: "INVALID_XY", Message: "xy-x and xy-y must be provided together", ExitCode: domain.ExitUsage}
	}

	if action == "set" && !hasPayload {
		return input, &domain.AppError{Code: "LIGHT_SET_EMPTY", Message: "light set requires explicit flags (e.g. --brightness, --on, --kelvin)", ExitCode: domain.ExitUsage}
	}

	if action == "effect" && strings.TrimSpace(light.Effect) != "" {
		if input.Body == nil {
			input.Body = map[string]any{}
		}
		input.Body["effect"] = light.Effect
	}
	if action == "flash" && strings.TrimSpace(light.AlertAction) != "" {
		if input.Body == nil {
			input.Body = map[string]any{}
		}
		input.Body["action"] = light.AlertAction
	}

	if hasPayload {
		input.LightSet = light
	}
	return input, nil
}

func parseSceneInput(cmd *cobra.Command, action string, input app.ActionInput) (app.ActionInput, *domain.AppError) {
	scene := &app.SceneInput{}
	if action == "activate" {
		if cmd.Flags().Changed("dynamic") {
			v, _ := cmd.Flags().GetBool("dynamic")
			scene.Dynamic = &v
		}
		if cmd.Flags().Changed("transition-ms") {
			v, _ := cmd.Flags().GetInt("transition-ms")
			if v < 0 {
				return input, &domain.AppError{
					Code:     "INVALID_TRANSITION",
					Message:  "transition-ms must be >= 0. Example: huectl scene activate --id <scene-id> --transition-ms 500",
					ExitCode: domain.ExitUsage,
				}
			}
			scene.DurationMS = &v
		}
		input.Scene = scene
		return input, nil
	}

	if action == "create" || action == "update" {
		roomID, _ := cmd.Flags().GetString("room-id")
		zoneID, _ := cmd.Flags().GetString("zone-id")
		roomID = strings.TrimSpace(roomID)
		zoneID = strings.TrimSpace(zoneID)
		if roomID != "" && zoneID != "" {
			return input, &domain.AppError{
				Code:     "SCENE_TARGET_CONFLICT",
				Message:  "provide either --room-id or --zone-id, not both. Example: huectl scene create --name Evening --room-id <room-id>",
				ExitCode: domain.ExitUsage,
			}
		}
		if action == "create" && roomID == "" && zoneID == "" {
			return input, &domain.AppError{
				Code:     "SCENE_TARGET_REQUIRED",
				Message:  "scene create requires --room-id or --zone-id. Example: huectl scene create --name Evening --room-id <room-id>",
				ExitCode: domain.ExitUsage,
			}
		}
		if input.Name != "" {
			if input.Body == nil {
				input.Body = map[string]any{}
			}
			input.Body["metadata"] = map[string]any{"name": input.Name}
		}
		targetID := roomID
		if targetID == "" {
			targetID = zoneID
		}
		if targetID != "" {
			if input.Body == nil {
				input.Body = map[string]any{}
			}
			input.Body["group"] = map[string]any{"rid": targetID}
		}
	}

	return input, nil
}

func parseAutomationInput(cmd *cobra.Command, action string, input app.ActionInput) (app.ActionInput, *domain.AppError) {
	if action != "create" && action != "update" {
		return input, nil
	}
	automation := &app.AutomationInput{}
	body := map[string]any{}
	hasActionableField := false

	if input.Name != "" {
		body["metadata"] = map[string]any{"name": input.Name}
	}
	enableChanged := cmd.Flags().Changed("enable")
	disableChanged := cmd.Flags().Changed("disable")
	if enableChanged && disableChanged {
		return input, &domain.AppError{
			Code:     "AUTOMATION_ENABLE_CONFLICT",
			Message:  "use either --enable or --disable, not both. Example: huectl automation update --id <id> --enable",
			ExitCode: domain.ExitUsage,
		}
	}
	if enableChanged {
		v, _ := cmd.Flags().GetBool("enable")
		automation.Enabled = &v
		body["enabled"] = v
		hasActionableField = true
	}
	if disableChanged {
		disabled := false
		automation.Enabled = &disabled
		body["enabled"] = false
		hasActionableField = true
	}
	if cmd.Flags().Changed("script") {
		v, _ := cmd.Flags().GetString("script")
		automation.ScriptID = strings.TrimSpace(v)
		if automation.ScriptID != "" {
			body["script_id"] = automation.ScriptID
			hasActionableField = true
		}
	}
	if cmd.Flags().Changed("trigger") {
		v, _ := cmd.Flags().GetString("trigger")
		automation.TriggerType = strings.TrimSpace(v)
		if automation.TriggerType != "" {
			body["trigger_type"] = automation.TriggerType
			hasActionableField = true
		}
	}
	if cmd.Flags().Changed("at") {
		v, _ := cmd.Flags().GetString("at")
		automation.Time = strings.TrimSpace(v)
		if automation.Time != "" {
			body["time"] = automation.Time
			hasActionableField = true
		}
	}
	if cmd.Flags().Changed("every") {
		v, _ := cmd.Flags().GetString("every")
		automation.Recurrence = strings.TrimSpace(v)
		if automation.Recurrence != "" {
			body["recurrence"] = automation.Recurrence
			hasActionableField = true
		}
	}

	if !hasActionableField {
		return input, &domain.AppError{
			Code:     "AUTOMATION_INVALID_INPUT",
			Message:  "automation create/update needs at least one control flag (--script, --trigger, --at, --every, --enable, --disable). Example: huectl automation create --name Morning --trigger time --at 07:30:00",
			ExitCode: domain.ExitUsage,
		}
	}

	input.Automation = automation
	input.Body = body
	return input, nil
}

func parseSensorInput(cmd *cobra.Command, action string, input app.ActionInput) (app.ActionInput, *domain.AppError) {
	if action != "sensitivity" {
		return input, nil
	}
	sensor := &app.SensorInput{}
	if !cmd.Flags().Changed("sensitivity") {
		return input, &domain.AppError{Code: "SENSITIVITY_REQUIRED", Message: "sensor sensitivity requires --sensitivity", ExitCode: domain.ExitUsage}
	}
	v, _ := cmd.Flags().GetInt("sensitivity")
	if v < 0 || v > 2 {
		return input, &domain.AppError{Code: "INVALID_SENSITIVITY", Message: "sensitivity must be between 0 and 2", ExitCode: domain.ExitUsage}
	}
	sensor.Sensitivity = &v

	if cmd.Flags().Changed("enabled") {
		enabled, _ := cmd.Flags().GetBool("enabled")
		sensor.Enabled = &enabled
	}

	input.Sensor = sensor
	return input, nil
}

func parseGroupInput(cmd *cobra.Command, action string, input app.ActionInput) (app.ActionInput, *domain.AppError) {
	if action == "assign" || action == "unassign" {
		childIDs, _ := cmd.Flags().GetStringSlice("child-id")
		if len(childIDs) == 0 {
			return input, &domain.AppError{Code: "CHILD_ID_REQUIRED", Message: "assign/unassign requires --child-id", ExitCode: domain.ExitUsage}
		}
		input.Assignment = &app.GroupAssignmentInput{ChildIDs: childIDs}
		input.Body = map[string]any{"child_ids": childIDs}
		return input, nil
	}

	if action == "create" || action == "update" {
		if strings.TrimSpace(input.Name) == "" {
			return input, &domain.AppError{Code: "NAME_REQUIRED", Message: action + " requires --name", ExitCode: domain.ExitUsage}
		}
		input.Body = map[string]any{"metadata": map[string]any{"name": input.Name}}
	}
	return input, nil
}
