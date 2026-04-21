package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/data219/huectl/internal/domain"
	"github.com/spf13/cobra"
)

func TestAllActionCommandsAcceptGlobalJSONFlag(t *testing.T) {
	rootCmd, _ := buildRootCommand()

	var visit func(cmd *cobra.Command)
	visit = func(cmd *cobra.Command) {
		children := cmd.Commands()
		if len(children) == 0 {
			if cmd.Flag("json") == nil {
				t.Fatalf("command %q does not accept --json", cmd.CommandPath())
			}
			return
		}
		for _, child := range children {
			if child.Name() == "help" || child.Name() == "completion" {
				continue
			}
			visit(child)
		}
	}

	visit(rootCmd)
}

func TestAllActionCommandsAcceptGlobalVerboseFlag(t *testing.T) {
	rootCmd, _ := buildRootCommand()

	var visit func(cmd *cobra.Command)
	visit = func(cmd *cobra.Command) {
		children := cmd.Commands()
		if len(children) == 0 {
			flag := cmd.Flag("verbose")
			if flag == nil {
				t.Fatalf("command %q does not accept --verbose", cmd.CommandPath())
			}
			if flag.Shorthand != "v" {
				t.Fatalf("command %q verbose shorthand = %q, want v", cmd.CommandPath(), flag.Shorthand)
			}
			return
		}
		for _, child := range children {
			if child.Name() == "help" || child.Name() == "completion" {
				continue
			}
			visit(child)
		}
	}

	visit(rootCmd)
}

func TestRootCommandCountsVerboseLevels(t *testing.T) {
	rootCmd, opts := buildRootCommand()

	if err := rootCmd.ParseFlags([]string{"-vvv"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if opts.verbose != 3 {
		t.Fatalf("verbose level = %d, want 3", opts.verbose)
	}
}

func TestRootCommandCountsLongVerboseFlag(t *testing.T) {
	rootCmd, opts := buildRootCommand()

	if err := rootCmd.ParseFlags([]string{"--verbose", "--verbose"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if opts.verbose != 2 {
		t.Fatalf("verbose level = %d, want 2", opts.verbose)
	}
}

func TestRootHelpIncludesVerbosityLevels(t *testing.T) {
	rootCmd, _ := buildRootCommand()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute help: %v", err)
	}

	help := buf.String()
	if !strings.Contains(help, "-v, --verbose") {
		t.Fatalf("help is missing -v/--verbose flag:\n%s", help)
	}
	if !strings.Contains(help, "repeat for more detail") {
		t.Fatalf("help is missing repeatable verbosity description:\n%s", help)
	}
}

func TestExitCodeFromExecuteError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if code := exitCodeFromExecuteError(nil); code != domain.ExitSuccess {
			t.Fatalf("unexpected code: %d", code)
		}
	})

	t.Run("cliError", func(t *testing.T) {
		err := &cliError{code: domain.ExitTarget, err: errors.New("target failed")}
		if code := exitCodeFromExecuteError(err); code != domain.ExitTarget {
			t.Fatalf("unexpected code: %d", code)
		}
	})

	t.Run("cobraUsageError", func(t *testing.T) {
		if code := exitCodeFromExecuteError(errors.New("unknown flag")); code != domain.ExitUsage {
			t.Fatalf("unexpected code: %d", code)
		}
	})
}
