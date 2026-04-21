package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/data219/huectl/internal/app"
	cliCommands "github.com/data219/huectl/internal/cli/commands"
	"github.com/data219/huectl/internal/domain"
	"github.com/data219/huectl/internal/output"
	"github.com/data219/huectl/internal/store/config"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	jsonMode   bool
	bridge     string
	allBridges bool
	broadcast  bool
	timeout    time.Duration
	verbose    int
	noColor    bool
	configPath string
}

type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("cli exited with code %d", e.code)
}

func Execute() int {
	rootCmd, opts := buildRootCommand()
	err := rootCmd.Execute()
	code := exitCodeFromExecuteError(err)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "huectl: %v\n", err)
		_ = opts
	}
	return code
}

func exitCodeFromExecuteError(err error) int {
	if err == nil {
		return domain.ExitSuccess
	}
	var exitErr *cliError
	if errors.As(err, &exitErr) {
		return exitErr.code
	}
	return domain.ExitUsage
}

func buildRootCommand() (*cobra.Command, *rootOptions) {
	defaultConfigPath, err := config.DefaultPath()
	if err != nil {
		defaultConfigPath = ""
	}

	opts := &rootOptions{
		timeout:    10 * time.Second,
		configPath: defaultConfigPath,
	}

	cmd := &cobra.Command{
		Use:   "huectl",
		Short: "Manage Philips Hue bridges and resources from the command line",
		Example: strings.Join([]string{
			"huectl light list",
			"huectl light set --name Kitchen --brightness 40 --on",
			"huectl api post --path /resource/light --body-file payload.json --json",
		}, "\n"),
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.PersistentFlags().BoolVar(&opts.jsonMode, "json", false, "output machine-readable JSON envelope")
	cmd.PersistentFlags().StringVar(&opts.bridge, "bridge", "", "bridge id or name scope")
	cmd.PersistentFlags().BoolVar(&opts.allBridges, "all-bridges", false, "target all configured bridges")
	cmd.PersistentFlags().BoolVar(&opts.broadcast, "broadcast", false, "explicitly write to all matching targets across scope")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 10*time.Second, "command timeout (e.g. 10s, 1m)")
	cmd.PersistentFlags().CountVarP(&opts.verbose, "verbose", "v", "increase output verbosity; repeat for more detail")
	cmd.PersistentFlags().BoolVar(&opts.noColor, "no-color", false, "disable ANSI colors in human output")
	cmd.PersistentFlags().StringVar(&opts.configPath, "config", defaultConfigPath, "path to huectl config file")

	for _, domainCmd := range buildDomainCommands(opts) {
		cmd.AddCommand(domainCmd)
	}
	cmd.AddCommand(buildEntertainmentCommand(opts))

	return cmd, opts
}

func buildDomainCommands(opts *rootOptions) []*cobra.Command {
	specs := cliCommands.DomainSpecs()
	commands := make([]*cobra.Command, 0, len(specs))
	for _, spec := range specs {
		domainName := spec.Name
		domainCmd := &cobra.Command{
			Use:   domainName,
			Short: fmt.Sprintf("%s commands", domainName),
		}
		for _, action := range spec.Actions {
			actionCopy := action
			actionCmd := &cobra.Command{
				Use:   actionCopy,
				Short: fmt.Sprintf("%s %s", domainName, actionCopy),
				RunE: func(cmd *cobra.Command, _ []string) error {
					return runAction(cmd, opts, domainName, actionCopy)
				},
			}
			if example := commandExample(domainName, actionCopy); example != "" {
				actionCmd.Example = example
			}
			registerActionFlags(actionCmd, domainName, actionCopy)
			domainCmd.AddCommand(actionCmd)
		}
		commands = append(commands, domainCmd)
	}
	return commands
}

func buildEntertainmentCommand(opts *rootOptions) *cobra.Command {
	root := &cobra.Command{Use: "entertainment", Short: "Entertainment area and session commands"}

	areaCmd := &cobra.Command{Use: "area", Short: "Manage entertainment areas"}
	for _, action := range cliCommands.EntertainmentAreaActions() {
		actionCopy := action
		cmd := &cobra.Command{
			Use:   actionCopy,
			Short: fmt.Sprintf("entertainment area %s", actionCopy),
			RunE: func(command *cobra.Command, _ []string) error {
				return runAction(command, opts, "entertainment", actionCopy)
			},
		}
		registerActionFlags(cmd, "entertainment", actionCopy)
		areaCmd.AddCommand(cmd)
	}

	sessionCmd := &cobra.Command{Use: "session", Short: "Control entertainment sessions"}
	for _, action := range cliCommands.EntertainmentSessionActions() {
		actionCopy := action
		cmd := &cobra.Command{
			Use:   actionCopy,
			Short: fmt.Sprintf("entertainment session %s", actionCopy),
			RunE: func(command *cobra.Command, _ []string) error {
				return runAction(command, opts, "entertainment", actionCopy)
			},
		}
		registerActionFlags(cmd, "entertainment", actionCopy)
		sessionCmd.AddCommand(cmd)
	}

	root.AddCommand(areaCmd, sessionCmd)
	return root
}

func registerActionFlags(cmd *cobra.Command, domainName string, action string) {
	cliCommands.RegisterActionFlags(cmd, domainName, action)
}

func runAction(cmd *cobra.Command, opts *rootOptions, domainName string, action string) error {
	started := time.Now()
	service := app.NewService(opts.configPath, opts.timeout)
	input, appErr := parseActionInput(cmd, domainName, action)
	if appErr != nil {
		return emitError(cmd, opts, started, domainName, action, appErr, false)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
	defer cancel()

	cmdCtx := domain.CommandContext{
		JSON:       opts.jsonMode,
		Bridge:     opts.bridge,
		AllBridges: opts.allBridges,
		Broadcast:  opts.broadcast,
		TimeoutSec: int(opts.timeout.Seconds()),
		Verbose:    opts.verbose,
		NoColor:    opts.noColor,
		Command:    domainName + " " + action,
	}

	data, partial, executionErr := service.Execute(ctx, cmdCtx, domainName, action, input)
	if executionErr != nil {
		return emitError(cmd, opts, started, domainName, action, executionErr, partial)
	}

	envelope := output.BuildSuccess(domainName+" "+action, buildBridgeScope(opts), started, data, partial)
	if err := output.WriteWithVerbosity(cmd.OutOrStdout(), opts.jsonMode, opts.verbose, envelope); err != nil {
		return &cliError{code: domain.ExitInternal, err: err}
	}
	if partial {
		return &cliError{code: domain.ExitPartial}
	}
	return nil
}

func parseActionInput(cmd *cobra.Command, domainName string, action string) (app.ActionInput, *domain.AppError) {
	return cliCommands.ParseActionInput(cmd, domainName, action)
}

func emitError(
	cmd *cobra.Command,
	opts *rootOptions,
	started time.Time,
	domainName string,
	action string,
	appErr *domain.AppError,
	partial bool,
) error {
	errEnvelope := output.BuildError(
		domainName+" "+action,
		buildBridgeScope(opts),
		started,
		output.ErrorData{
			Code:    appErr.Code,
			Message: appErr.Message,
			Hints:   appErr.Hints,
			Details: appErr.Details,
		},
		partial,
	)
	if writeErr := output.WriteWithVerbosity(cmd.OutOrStdout(), opts.jsonMode, opts.verbose, errEnvelope); writeErr != nil {
		return &cliError{code: domain.ExitInternal, err: writeErr}
	}
	exitCode := appErr.ExitCode
	if exitCode == 0 {
		exitCode = domain.ExitInternal
	}
	return &cliError{code: exitCode, err: appErr}
}

func buildBridgeScope(opts *rootOptions) []string {
	if strings.TrimSpace(opts.bridge) != "" {
		return []string{strings.TrimSpace(opts.bridge)}
	}
	if opts.allBridges {
		return []string{"*"}
	}
	return []string{"*"}
}

func commandExample(domainName string, action string) string {
	switch domainName + " " + action {
	case "light list":
		return "huectl light list"
	case "light set":
		return "huectl light set --name Kitchen --brightness 40 --on"
	case "scene activate":
		return "huectl scene activate --name Evening --transition-ms 500"
	case "scene create":
		return "huectl scene create --name Evening --room-id <room-id>"
	case "automation create":
		return "huectl automation create --name Morning --trigger time --at 07:30:00 --enable"
	case "api post":
		return "huectl api post --path /resource/light --body-file payload.json --json"
	default:
		return ""
	}
}
