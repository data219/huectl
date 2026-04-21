package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRegisterActionFlagsLightSetDoesNotExposeGenericBodyFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "set"}
	RegisterActionFlags(cmd, "light", "set")

	if cmd.Flag("body") != nil {
		t.Fatal("light set must not expose --body")
	}
	if cmd.Flag("type") != nil {
		t.Fatal("light set must not expose --type")
	}
	if cmd.Flag("brightness") == nil {
		t.Fatal("light set must expose --brightness")
	}
	if cmd.Flag("kelvin") == nil {
		t.Fatal("light set must expose --kelvin")
	}
}

func TestRegisterActionFlagsAPIRawKeepsBody(t *testing.T) {
	cmd := &cobra.Command{Use: "post"}
	RegisterActionFlags(cmd, "api", "post")

	if cmd.Flag("body") == nil {
		t.Fatal("api post must expose --body")
	}
	if cmd.Flag("path") == nil {
		t.Fatal("api post must expose --path")
	}
	if cmd.Flag("body-file") == nil {
		t.Fatal("api post must expose --body-file")
	}
}

func TestParseActionInputLightSetParsesExplicitFields(t *testing.T) {
	cmd := &cobra.Command{Use: "set"}
	RegisterActionFlags(cmd, "light", "set")

	err := cmd.Flags().Set("name", "Kitchen")
	if err != nil {
		t.Fatalf("set name flag: %v", err)
	}
	err = cmd.Flags().Set("on", "true")
	if err != nil {
		t.Fatalf("set on flag: %v", err)
	}
	err = cmd.Flags().Set("brightness", "55")
	if err != nil {
		t.Fatalf("set brightness flag: %v", err)
	}
	err = cmd.Flags().Set("kelvin", "3000")
	if err != nil {
		t.Fatalf("set kelvin flag: %v", err)
	}

	input, appErr := ParseActionInput(cmd, "light", "set")
	if appErr != nil {
		t.Fatalf("parse light set input: %v", appErr)
	}
	if input.LightSet == nil {
		t.Fatal("expected LightSet options to be set")
	}
	if input.LightSet.Brightness == nil || *input.LightSet.Brightness != 55 {
		t.Fatalf("unexpected brightness: %#v", input.LightSet.Brightness)
	}
	if input.LightSet.Kelvin == nil || *input.LightSet.Kelvin != 3000 {
		t.Fatalf("unexpected kelvin: %#v", input.LightSet.Kelvin)
	}
}

func TestParseActionInputLightEffectMapsEffectFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "effect"}
	RegisterActionFlags(cmd, "light", "effect")

	if err := cmd.Flags().Set("id", "light-1"); err != nil {
		t.Fatalf("set id flag: %v", err)
	}
	if err := cmd.Flags().Set("effect", "candle"); err != nil {
		t.Fatalf("set effect flag: %v", err)
	}

	input, appErr := ParseActionInput(cmd, "light", "effect")
	if appErr != nil {
		t.Fatalf("parse light effect input: %v", appErr)
	}
	if input.LightSet == nil {
		t.Fatal("expected LightSet options to be set")
	}
	if strings.TrimSpace(input.LightSet.Effect) != "candle" {
		t.Fatalf("unexpected effect: %q", input.LightSet.Effect)
	}
	rawEffect, ok := input.Body["effect"].(string)
	if !ok {
		t.Fatalf("expected body effect string, got %#v", input.Body["effect"])
	}
	if got := strings.TrimSpace(rawEffect); got != "candle" {
		t.Fatalf("unexpected body effect: %q", got)
	}
}

func TestParseActionInputLightFlashMapsAlertActionFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "flash"}
	RegisterActionFlags(cmd, "light", "flash")

	if err := cmd.Flags().Set("id", "light-1"); err != nil {
		t.Fatalf("set id flag: %v", err)
	}
	if err := cmd.Flags().Set("alert-action", "breathe"); err != nil {
		t.Fatalf("set alert-action flag: %v", err)
	}

	input, appErr := ParseActionInput(cmd, "light", "flash")
	if appErr != nil {
		t.Fatalf("parse light flash input: %v", appErr)
	}
	if input.LightSet == nil {
		t.Fatal("expected LightSet options to be set")
	}
	if strings.TrimSpace(input.LightSet.AlertAction) != "breathe" {
		t.Fatalf("unexpected alert-action: %q", input.LightSet.AlertAction)
	}
	rawAction, ok := input.Body["action"].(string)
	if !ok {
		t.Fatalf("expected body action string, got %#v", input.Body["action"])
	}
	if got := strings.TrimSpace(rawAction); got != "breathe" {
		t.Fatalf("unexpected body action: %q", got)
	}
}

func TestParseActionInputRejectsInvalidBrightnessRange(t *testing.T) {
	cmd := &cobra.Command{Use: "set"}
	RegisterActionFlags(cmd, "light", "set")

	err := cmd.Flags().Set("id", "abc")
	if err != nil {
		t.Fatalf("set id flag: %v", err)
	}
	err = cmd.Flags().Set("brightness", "120")
	if err != nil {
		t.Fatalf("set brightness flag: %v", err)
	}

	_, appErr := ParseActionInput(cmd, "light", "set")
	if appErr == nil {
		t.Fatal("expected validation error for invalid brightness")
	}
	if appErr.Code != "INVALID_BRIGHTNESS" {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
}

func TestParseActionInputSensorSensitivity(t *testing.T) {
	cmd := &cobra.Command{Use: "sensitivity"}
	RegisterActionFlags(cmd, "sensor", "sensitivity")

	err := cmd.Flags().Set("name", "Motion Sensor")
	if err != nil {
		t.Fatalf("set name flag: %v", err)
	}
	err = cmd.Flags().Set("sensitivity", "2")
	if err != nil {
		t.Fatalf("set sensitivity flag: %v", err)
	}
	err = cmd.Flags().Set("enabled", "true")
	if err != nil {
		t.Fatalf("set enabled flag: %v", err)
	}

	input, appErr := ParseActionInput(cmd, "sensor", "sensitivity")
	if appErr != nil {
		t.Fatalf("parse sensor sensitivity input: %v", appErr)
	}
	if input.Sensor == nil {
		t.Fatal("expected sensor options")
	}
	if input.Sensor.Sensitivity == nil || *input.Sensor.Sensitivity != 2 {
		t.Fatalf("unexpected sensitivity: %#v", input.Sensor.Sensitivity)
	}
	if input.Sensor.Enabled == nil || !*input.Sensor.Enabled {
		t.Fatalf("unexpected enabled: %#v", input.Sensor.Enabled)
	}
}

func TestParseActionInputRejectsHalfXYPair(t *testing.T) {
	cmd := &cobra.Command{Use: "set"}
	RegisterActionFlags(cmd, "light", "set")

	err := cmd.Flags().Set("id", "abc")
	if err != nil {
		t.Fatalf("set id flag: %v", err)
	}
	err = cmd.Flags().Set("xy-x", "0.31")
	if err != nil {
		t.Fatalf("set xy-x flag: %v", err)
	}

	_, appErr := ParseActionInput(cmd, "light", "set")
	if appErr == nil {
		t.Fatal("expected validation error for incomplete xy pair")
	}
	if appErr.Code != "INVALID_XY" {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
}

func TestRegisterActionFlagsSceneActivateUsesTransitionMS(t *testing.T) {
	cmd := &cobra.Command{Use: "activate"}
	RegisterActionFlags(cmd, "scene", "activate")

	if cmd.Flag("transition-ms") == nil {
		t.Fatal("scene activate must expose --transition-ms")
	}
	if cmd.Flag("duration-ms") != nil {
		t.Fatal("scene activate must not expose legacy --duration-ms")
	}
}

func TestRegisterActionFlagsSceneCreateUsesRoomZoneFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "create"}
	RegisterActionFlags(cmd, "scene", "create")

	if cmd.Flag("room-id") == nil {
		t.Fatal("scene create must expose --room-id")
	}
	if cmd.Flag("zone-id") == nil {
		t.Fatal("scene create must expose --zone-id")
	}
	if cmd.Flag("group-id") != nil {
		t.Fatal("scene create must not expose legacy --group-id")
	}
}

func TestParseActionInputSceneActivate(t *testing.T) {
	cmd := &cobra.Command{Use: "activate"}
	RegisterActionFlags(cmd, "scene", "activate")

	if err := cmd.Flags().Set("id", "scene-1"); err != nil {
		t.Fatalf("set id flag: %v", err)
	}
	if err := cmd.Flags().Set("dynamic", "true"); err != nil {
		t.Fatalf("set dynamic flag: %v", err)
	}
	if err := cmd.Flags().Set("transition-ms", "1200"); err != nil {
		t.Fatalf("set transition-ms flag: %v", err)
	}

	input, appErr := ParseActionInput(cmd, "scene", "activate")
	if appErr != nil {
		t.Fatalf("parse scene activate input: %v", appErr)
	}
	if input.Scene == nil {
		t.Fatal("expected scene options")
	}
	if input.Scene.Dynamic == nil || !*input.Scene.Dynamic {
		t.Fatalf("unexpected dynamic value: %#v", input.Scene.Dynamic)
	}
	if input.Scene.DurationMS == nil || *input.Scene.DurationMS != 1200 {
		t.Fatalf("unexpected transition-ms value: %#v", input.Scene.DurationMS)
	}
}

func TestParseActionInputSceneActivateRejectsNegativeTransition(t *testing.T) {
	cmd := &cobra.Command{Use: "activate"}
	RegisterActionFlags(cmd, "scene", "activate")

	if err := cmd.Flags().Set("id", "scene-1"); err != nil {
		t.Fatalf("set id flag: %v", err)
	}
	if err := cmd.Flags().Set("transition-ms", "-1"); err != nil {
		t.Fatalf("set transition-ms flag: %v", err)
	}

	_, appErr := ParseActionInput(cmd, "scene", "activate")
	if appErr == nil {
		t.Fatal("expected validation error for negative transition")
	}
	if appErr.Code != "INVALID_TRANSITION" {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
	if !strings.Contains(appErr.Message, "transition-ms") {
		t.Fatalf("error message must reference transition-ms: %q", appErr.Message)
	}
}

func TestParseActionInputSceneCreateRequiresTarget(t *testing.T) {
	cmd := &cobra.Command{Use: "create"}
	RegisterActionFlags(cmd, "scene", "create")

	if err := cmd.Flags().Set("name", "Evening"); err != nil {
		t.Fatalf("set name flag: %v", err)
	}

	_, appErr := ParseActionInput(cmd, "scene", "create")
	if appErr == nil {
		t.Fatal("expected validation error for missing scene target")
	}
	if appErr.Code != "SCENE_TARGET_REQUIRED" {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
	if !strings.Contains(appErr.Message, "huectl scene create") {
		t.Fatalf("expected concrete example in error message, got: %q", appErr.Message)
	}
}

func TestParseActionInputSceneCreateRejectsTargetConflict(t *testing.T) {
	cmd := &cobra.Command{Use: "create"}
	RegisterActionFlags(cmd, "scene", "create")

	if err := cmd.Flags().Set("name", "Evening"); err != nil {
		t.Fatalf("set name flag: %v", err)
	}
	if err := cmd.Flags().Set("room-id", "room-1"); err != nil {
		t.Fatalf("set room-id flag: %v", err)
	}
	if err := cmd.Flags().Set("zone-id", "zone-1"); err != nil {
		t.Fatalf("set zone-id flag: %v", err)
	}

	_, appErr := ParseActionInput(cmd, "scene", "create")
	if appErr == nil {
		t.Fatal("expected validation error for conflicting scene targets")
	}
	if appErr.Code != "SCENE_TARGET_CONFLICT" {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
}

func TestParseActionInputSceneCreateWithRoomTarget(t *testing.T) {
	cmd := &cobra.Command{Use: "create"}
	RegisterActionFlags(cmd, "scene", "create")

	if err := cmd.Flags().Set("name", "Evening"); err != nil {
		t.Fatalf("set name flag: %v", err)
	}
	if err := cmd.Flags().Set("room-id", "room-1"); err != nil {
		t.Fatalf("set room-id flag: %v", err)
	}

	input, appErr := ParseActionInput(cmd, "scene", "create")
	if appErr != nil {
		t.Fatalf("parse scene create input: %v", appErr)
	}
	groupRaw, ok := input.Body["group"].(map[string]any)
	if !ok {
		t.Fatalf("expected group payload, got: %#v", input.Body)
	}
	if groupRaw["rid"] != "room-1" {
		t.Fatalf("unexpected group rid: %#v", groupRaw["rid"])
	}
}

func TestRegisterActionFlagsAutomationUsesNewNames(t *testing.T) {
	cmd := &cobra.Command{Use: "create"}
	RegisterActionFlags(cmd, "automation", "create")

	required := []string{"enable", "disable", "script", "trigger", "at", "every"}
	for _, name := range required {
		if cmd.Flag(name) == nil {
			t.Fatalf("automation create must expose --%s", name)
		}
	}
	legacy := []string{"enabled", "script-id", "trigger-type", "time", "recurrence"}
	for _, name := range legacy {
		if cmd.Flag(name) != nil {
			t.Fatalf("automation create must not expose legacy --%s", name)
		}
	}
}

func TestParseActionInputAutomationCreateRequiresActionableFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "create"}
	RegisterActionFlags(cmd, "automation", "create")
	if err := cmd.Flags().Set("name", "Morning"); err != nil {
		t.Fatalf("set name flag: %v", err)
	}

	_, appErr := ParseActionInput(cmd, "automation", "create")
	if appErr == nil {
		t.Fatal("expected validation error for empty automation payload")
	}
	if appErr.Code != "AUTOMATION_INVALID_INPUT" {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
	if !strings.Contains(appErr.Message, "huectl automation create") {
		t.Fatalf("expected concrete example in error message, got: %q", appErr.Message)
	}
}

func TestParseActionInputAutomationEnableDisableConflict(t *testing.T) {
	cmd := &cobra.Command{Use: "create"}
	RegisterActionFlags(cmd, "automation", "create")
	if err := cmd.Flags().Set("name", "Morning"); err != nil {
		t.Fatalf("set name flag: %v", err)
	}
	if err := cmd.Flags().Set("enable", "true"); err != nil {
		t.Fatalf("set enable flag: %v", err)
	}
	if err := cmd.Flags().Set("disable", "true"); err != nil {
		t.Fatalf("set disable flag: %v", err)
	}

	_, appErr := ParseActionInput(cmd, "automation", "create")
	if appErr == nil {
		t.Fatal("expected enable/disable conflict")
	}
	if appErr.Code != "AUTOMATION_ENABLE_CONFLICT" {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
}

func TestParseActionInputAutomationCreateParsesNewFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "create"}
	RegisterActionFlags(cmd, "automation", "create")

	flagValues := map[string]string{
		"name":    "Morning",
		"enable":  "true",
		"script":  "script-123",
		"trigger": "time",
		"at":      "07:30:00",
		"every":   "mon,tue,wed",
	}
	for key, value := range flagValues {
		if err := cmd.Flags().Set(key, value); err != nil {
			t.Fatalf("set %s flag: %v", key, err)
		}
	}

	input, appErr := ParseActionInput(cmd, "automation", "create")
	if appErr != nil {
		t.Fatalf("parse automation create input: %v", appErr)
	}
	if input.Automation == nil {
		t.Fatal("expected automation payload")
	}
	if input.Automation.Enabled == nil || !*input.Automation.Enabled {
		t.Fatalf("unexpected enabled value: %#v", input.Automation.Enabled)
	}
	if input.Automation.ScriptID != "script-123" {
		t.Fatalf("unexpected script id: %q", input.Automation.ScriptID)
	}
	if input.Automation.TriggerType != "time" {
		t.Fatalf("unexpected trigger: %q", input.Automation.TriggerType)
	}
	if input.Automation.Time != "07:30:00" {
		t.Fatalf("unexpected time: %q", input.Automation.Time)
	}
	if input.Automation.Recurrence != "mon,tue,wed" {
		t.Fatalf("unexpected recurrence: %q", input.Automation.Recurrence)
	}

	expectedBody := map[string]any{
		"enabled":      true,
		"script_id":    "script-123",
		"trigger_type": "time",
		"time":         "07:30:00",
		"recurrence":   "mon,tue,wed",
	}
	for key, expected := range expectedBody {
		if input.Body[key] != expected {
			t.Fatalf("unexpected body[%s]: got=%v want=%v", key, input.Body[key], expected)
		}
	}
}

func TestParseActionInputAPIBodyFile(t *testing.T) {
	cmd := &cobra.Command{Use: "post"}
	RegisterActionFlags(cmd, "api", "post")

	bodyFile := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(bodyFile, []byte(`{"on":{"on":true}}`), 0o600); err != nil {
		t.Fatalf("write body file: %v", err)
	}

	if err := cmd.Flags().Set("body-file", bodyFile); err != nil {
		t.Fatalf("set body-file: %v", err)
	}

	input, appErr := ParseActionInput(cmd, "api", "post")
	if appErr != nil {
		t.Fatalf("parse api input: %v", appErr)
	}
	if input.RawBody == "" {
		t.Fatal("expected raw body to be populated")
	}
	if input.Body == nil {
		t.Fatal("expected parsed body map")
	}
}

func TestParseActionInputAPIBodyFileInvalidJSON(t *testing.T) {
	cmd := &cobra.Command{Use: "post"}
	RegisterActionFlags(cmd, "api", "post")

	bodyFile := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(bodyFile, []byte(`{`), 0o600); err != nil {
		t.Fatalf("write body file: %v", err)
	}

	if err := cmd.Flags().Set("body-file", bodyFile); err != nil {
		t.Fatalf("set body-file: %v", err)
	}

	_, appErr := ParseActionInput(cmd, "api", "post")
	if appErr == nil {
		t.Fatal("expected invalid json error")
	}
	if appErr.Code != "INVALID_BODY" {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
}

func TestParseActionInputAPIBodyAndBodyFileConflict(t *testing.T) {
	cmd := &cobra.Command{Use: "post"}
	RegisterActionFlags(cmd, "api", "post")

	bodyFile := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(bodyFile, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("write body file: %v", err)
	}

	if err := cmd.Flags().Set("body", `{"x":1}`); err != nil {
		t.Fatalf("set body flag: %v", err)
	}
	if err := cmd.Flags().Set("body-file", bodyFile); err != nil {
		t.Fatalf("set body-file flag: %v", err)
	}

	_, appErr := ParseActionInput(cmd, "api", "post")
	if appErr == nil {
		t.Fatal("expected body conflict error")
	}
	if appErr.Code != "API_BODY_CONFLICT" {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
}
