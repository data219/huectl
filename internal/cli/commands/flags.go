package commands

import "github.com/spf13/cobra"

func RegisterActionFlags(cmd *cobra.Command, domainName string, action string) {
	registerCommonTargetFlags(cmd)

	switch domainName {
	case "bridge":
		registerBridgeFlags(cmd, action)
	case "api":
		registerAPIRawFlags(cmd)
	case "backup":
		cmd.Flags().String("file", "", "file path for import/export/diff")
	case "diagnose":
		if action == "events" {
			cmd.Flags().Duration("duration", 0, "duration for one-shot operations")
		}
	case "light":
		registerLightFlags(cmd, action)
	case "scene":
		registerSceneFlags(cmd, action)
	case "automation":
		registerAutomationFlags(cmd, action)
	case "sensor":
		registerSensorFlags(cmd, action)
	case "room", "zone":
		registerGroupingFlags(cmd, action)
	case "entertainment":
		if action == "start" || action == "stop" {
			cmd.Flags().Duration("duration", 0, "duration for one-shot operations")
		}
	}
}

func registerCommonTargetFlags(cmd *cobra.Command) {
	cmd.Flags().String("id", "", "target resource id (for show/write actions)")
	cmd.Flags().String("name", "", "target resource name")
}

func registerBridgeFlags(cmd *cobra.Command, action string) {
	if action == "add" {
		cmd.Flags().String("address", "", "bridge address (ip, host, or URL)")
	}
}

func registerAPIRawFlags(cmd *cobra.Command) {
	cmd.Flags().String("path", "", "raw API path (for example /resource/light)")
	cmd.Flags().String("method", "", "HTTP method override (defaults to action verb)")
	cmd.Flags().String("body", "", "inline JSON body payload")
	cmd.Flags().String("body-file", "", "path to JSON body file (alternative to --body)")
}

func registerLightFlags(cmd *cobra.Command, action string) {
	switch action {
	case "set":
		cmd.Flags().Bool("on", false, "switch light on/off")
		cmd.Flags().Int("brightness", -1, "brightness percentage [0..100]")
		cmd.Flags().Int("kelvin", -1, "color temperature in kelvin")
		cmd.Flags().Float64("xy-x", -1, "CIE x coordinate")
		cmd.Flags().Float64("xy-y", -1, "CIE y coordinate")
		cmd.Flags().String("effect", "", "light effect")
		cmd.Flags().Int("transition-ms", -1, "transition duration in milliseconds")
	case "effect":
		cmd.Flags().String("effect", "", "light effect")
	case "flash":
		cmd.Flags().String("alert-action", "", "flash alert action")
	}
}

func registerSceneFlags(cmd *cobra.Command, action string) {
	switch action {
	case "activate":
		cmd.Flags().Bool("dynamic", false, "activate dynamic scene playback")
		cmd.Flags().Int("transition-ms", -1, "scene transition duration in milliseconds")
	case "create", "update":
		cmd.Flags().String("room-id", "", "target room id for scene")
		cmd.Flags().String("zone-id", "", "target zone id for scene")
	}
}

func registerAutomationFlags(cmd *cobra.Command, action string) {
	if action == "create" || action == "update" {
		cmd.Flags().Bool("enable", false, "enable automation")
		cmd.Flags().Bool("disable", false, "disable automation")
		cmd.Flags().String("script", "", "behavior script identifier")
		cmd.Flags().String("trigger", "", "trigger type (for example time)")
		cmd.Flags().String("at", "", "time trigger value")
		cmd.Flags().String("every", "", "recurrence expression")
	}
}

func registerSensorFlags(cmd *cobra.Command, action string) {
	if action == "sensitivity" {
		cmd.Flags().Int("sensitivity", -1, "sensor sensitivity [0..2]")
		cmd.Flags().Bool("enabled", false, "enable/disable sensor")
	}
}

func registerGroupingFlags(cmd *cobra.Command, action string) {
	switch action {
	case "assign", "unassign":
		cmd.Flags().StringSlice("child-id", []string{}, "child resource id(s) to assign or unassign")
	}
}
