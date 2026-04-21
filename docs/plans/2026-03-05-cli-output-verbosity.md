# CLI Output Verbosity Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a shared verbosity-gated output model so human-readable output and `--json` expose the same data scope at `default`, `-v`, `-vv`, and `-vvv`, starting with the `room list`-style resource outputs and extending the contract repo-wide.

**Architecture:** Introduce a renderer-neutral output view model in `internal/output` that applies command-family field profiles and verbosity filtering before any final rendering happens. Keep the JSON envelope shape (`meta/data/error`) but make its populated fields and the human-readable fields come from the same filtered view, so the two output modes cannot drift.

**Tech Stack:** Go 1.25, Cobra, `internal/output`, Go tests, golden files under `internal/output/testdata/`, integration tests under `test/integration`, Taskfile, Docker.

---

### Task 1: Replace Boolean Verbose With Counted Global Verbosity

**Files:**
- Modify: `internal/cli/root.go:19-27`
- Modify: `internal/cli/root.go:64-101`
- Modify: `internal/cli/root.go:172-200`
- Modify: `internal/domain/types.go:14-23`
- Test: `internal/cli/root_test.go`

**Step 1: Write the failing tests**

Add tests that lock the new flag contract:

```go
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
```

**Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/cli -run 'Test(AllActionCommandsAcceptGlobalVerboseFlag|RootCommandCountsVerboseLevels)' -v
```

Expected:
- FAIL because `verbose` is still a `bool`
- FAIL because `-v` shorthand / counted parsing is not implemented

**Step 3: Write minimal implementation**

Update the root options and command context to use an integer verbosity level and wire the root flag with repeatable `-v` support.

```go
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

type CommandContext struct {
	JSON       bool
	Bridge     string
	AllBridges bool
	Broadcast  bool
	TimeoutSec int
	Verbose    int
	NoColor    bool
	Command    string
}

cmd.PersistentFlags().CountVarP(&opts.verbose, "verbose", "v", "increase output verbosity; repeat for more detail")
```

**Step 4: Run test to verify it passes**

Run:

```bash
go test ./internal/cli -run 'Test(AllActionCommandsAcceptGlobalVerboseFlag|RootCommandCountsVerboseLevels)' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/cli/root.go internal/domain/types.go internal/cli/root_test.go
git commit -m "feat: add counted global verbosity flag"
```

### Task 2: Introduce a Shared Verbosity-Gated Output View Model

**Files:**
- Create: `internal/output/view.go`
- Create: `internal/output/view_test.go`
- Modify: `internal/output/envelope.go:12-67`
- Test: `internal/output/envelope_test.go`

**Step 1: Write the failing tests**

Create a test suite that defines the filtering contract before touching the renderer:

```go
func TestBuildViewRoomListDefaultOmitsVerboseFields(t *testing.T) {
	view := BuildView(
		BuildSuccess("room list", []string{"bridge-a"}, time.Now(), sampleRoomAggregate(), false),
		RenderOptions{Verbosity: 0, JSON: false},
	)

	row := view.Data.(TableView).Rows[0]
	if _, ok := row["id"]; ok {
		t.Fatal("default room list must omit id")
	}
	if _, ok := row["details"]; ok {
		t.Fatal("default room list must omit details")
	}
	if view.Meta.RequestID != "" {
		t.Fatal("default view must omit request_id")
	}
}

func TestBuildViewRoomListVerboseIncludesIDAndDetails(t *testing.T) {
	view := BuildView(
		BuildSuccess("room list", []string{"bridge-a"}, time.Now(), sampleRoomAggregate(), false),
		RenderOptions{Verbosity: 1, JSON: false},
	)

	row := view.Data.(TableView).Rows[0]
	if row["id"] == "" || row["details"] == "" {
		t.Fatal("verbose room list must expose id and details")
	}
}

func TestBuildViewPartialSuccessKeepsBridgeErrorsAtDefault(t *testing.T) {
	view := BuildView(
		BuildSuccess("bridge health", []string{"*"}, time.Now(), samplePartialAggregate(), true),
		RenderOptions{Verbosity: 0, JSON: false},
	)

	if !view.Meta.PartialSuccess {
		t.Fatal("partial success must remain visible at default verbosity")
	}
	if len(view.BridgeErrors) == 0 {
		t.Fatal("bridge errors must remain visible at default verbosity")
	}
}
```

**Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/output -run 'TestBuildView(RoomListDefaultOmitsVerboseFields|RoomListVerboseIncludesIDAndDetails|PartialSuccessKeepsBridgeErrorsAtDefault)' -v
```

Expected:
- FAIL because `BuildView`, `RenderOptions`, `TableView`, and sample fixtures do not exist yet

**Step 3: Write minimal implementation**

Create a renderer-neutral view builder and move field selection into it.

```go
type RenderOptions struct {
	JSON      bool
	Verbosity int
}

type ViewEnvelope struct {
	Meta         Meta
	Data         any
	Error        *ErrorData
	BridgeErrors []domain.BridgeResult
}

func BuildView(envelope Envelope, opts RenderOptions) ViewEnvelope {
	// 1. detect command family from envelope.Meta.Command
	// 2. build command-specific table/map view
	// 3. apply verbosity filtering
	// 4. preserve partial-success bridge errors at default
}
```

**Step 4: Run test to verify it passes**

Run:

```bash
go test ./internal/output -run 'TestBuildView(RoomListDefaultOmitsVerboseFields|RoomListVerboseIncludesIDAndDetails|PartialSuccessKeepsBridgeErrorsAtDefault)' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/output/view.go internal/output/view_test.go internal/output/envelope.go internal/output/envelope_test.go
git commit -m "feat: add shared verbosity-gated output view"
```

### Task 3: Refactor Human Rendering to Use the Shared View and Golden Contracts

**Files:**
- Modify: `internal/output/renderer.go:14-404`
- Modify: `internal/output/renderer_test.go`
- Create: `internal/output/testdata/human_room_list_default.golden`
- Create: `internal/output/testdata/human_room_list_v.golden`
- Create: `internal/output/testdata/human_room_list_vv.golden`
- Create: `internal/output/testdata/human_bridge_health_partial_default.golden`
- Modify: `internal/output/testdata/human_success.golden`
- Modify: `internal/output/testdata/human_error.golden`
- Modify: `internal/output/testdata/human_light_list.golden`
- Modify: `internal/output/testdata/human_bridge_health.golden`

**Step 1: Write the failing tests**

Add golden-backed renderer tests for the new human contract:

```go
func TestWriteHumanRoomListDefaultGolden(t *testing.T) {
	env := sampleRoomListEnvelope()

	var buf bytes.Buffer
	if err := Write(&buf, false, 0, env); err != nil {
		t.Fatalf("write default room list: %v", err)
	}

	if diff := cmp.Diff(readGolden(t, "human_room_list_default.golden"), buf.String()); diff != "" {
		t.Fatalf("golden mismatch (-want +got):\n%s", diff)
	}
}

func TestWriteHumanRoomListVerboseGolden(t *testing.T) {
	env := sampleRoomListEnvelope()

	var buf bytes.Buffer
	if err := Write(&buf, false, 1, env); err != nil {
		t.Fatalf("write verbose room list: %v", err)
	}

	if diff := cmp.Diff(readGolden(t, "human_room_list_v.golden"), buf.String()); diff != "" {
		t.Fatalf("golden mismatch (-want +got):\n%s", diff)
	}
}

func TestWriteHumanPartialSuccessShowsBridgeErrorsByDefault(t *testing.T) {
	env := samplePartialBridgeHealthEnvelope()

	var buf bytes.Buffer
	if err := Write(&buf, false, 0, env); err != nil {
		t.Fatalf("write partial bridge health: %v", err)
	}
	if !strings.Contains(buf.String(), "bridge errors:") {
		t.Fatalf("expected bridge errors in default output:\n%s", buf.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/output -run 'TestWriteHuman(RoomListDefaultGolden|RoomListVerboseGolden|PartialSuccessShowsBridgeErrorsByDefault)' -v
```

Expected:
- FAIL because `Write` does not yet accept verbosity / shared view usage
- FAIL because the new goldens describe the new contract, not the current one

**Step 3: Write minimal implementation**

Refactor the renderer to render from `BuildView(...)` rather than from raw envelope data.

```go
func Write(w io.Writer, jsonMode bool, verbosity int, envelope Envelope) error {
	view := BuildView(envelope, RenderOptions{
		JSON:      jsonMode,
		Verbosity: verbosity,
	})
	if jsonMode {
		return writeJSONView(w, view)
	}
	return writeHumanView(w, view)
}
```

Keep these rules in place:

- default room/resource tables hide redundant `type`, `id`, `details`
- `-v` reintroduces fachliche Zusatzspalten
- `-vv` reintroduces summary/meta diagnostics
- partial-success bridge errors stay visible at default
- human error output always keeps `ERROR` and `HINT`

**Step 4: Run test to verify it passes**

Run:

```bash
go test ./internal/output -run 'TestWriteHuman(RoomListDefaultGolden|RoomListVerboseGolden|PartialSuccessShowsBridgeErrorsByDefault|SuccessGolden|ErrorGolden|LightListGolden|BridgeHealthGolden)' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/output/renderer.go internal/output/renderer_test.go internal/output/testdata/
git commit -m "feat: apply verbosity profiles to human output"
```

### Task 4: Apply the Same Scope Rules to JSON Output and Integration Tests

**Files:**
- Modify: `internal/output/envelope_test.go`
- Modify: `test/integration/light_list_output_test.go`
- Create: `test/integration/room_list_output_test.go`
- Modify: `internal/cli/root.go:199-234`

**Step 1: Write the failing tests**

Add JSON-facing tests that prove scope parity by verbosity level:

```go
func TestWriteJSONRoomListDefaultOmitsVerboseFields(t *testing.T) {
	env := sampleRoomListEnvelope()

	var buf bytes.Buffer
	if err := Write(&buf, true, 0, env); err != nil {
		t.Fatalf("write json default room list: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	meta := payload["meta"].(map[string]any)
	data := payload["data"].(map[string]any)
	rows := data["rows"].([]any)
	row := rows[0].(map[string]any)

	if _, ok := row["id"]; ok {
		t.Fatal("default json room list must omit id")
	}
	if _, ok := row["details"]; ok {
		t.Fatal("default json room list must omit details")
	}
	if _, ok := meta["request_id"]; ok {
		t.Fatal("default json room list must omit request_id")
	}
}

func TestWriteJSONRoomListVerboseIncludesIDAndDetails(t *testing.T) {
	// same fixture, verbosity 1; assert id and details are present
}

func TestRoomListDefaultOutputOmitsIDAndDetails(t *testing.T) {
	// build binary, serve /clip/v2/resource/room, assert stdout default omits ID/details columns
}
```

**Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/output -run 'TestWriteJSONRoomList(DefaultOmitsVerboseFields|VerboseIncludesIDAndDetails)' -v
go test ./test/integration -run 'Test(RoomListDefaultOutputOmitsIDAndDetails|LightListJSONOutputEnvelope)' -v
```

Expected:
- FAIL because JSON still renders the full raw envelope data
- FAIL because the integration output still uses the old table/meta contract

**Step 3: Write minimal implementation**

Update the CLI call sites to pass verbosity into `output.Write(...)`, then make JSON rendering use the same filtered view that human output uses.

```go
if err := output.Write(cmd.OutOrStdout(), opts.jsonMode, opts.verbose, envelope); err != nil {
	return &cliError{code: domain.ExitInternal, err: err}
}
```

Ensure the JSON contract stays structurally stable:

- top-level `meta`, `data`, `error` remain present
- `meta.schema` remains `huectl/v1`
- verbosity only controls the populated payload fields, not the existence of the envelope

**Step 4: Run test to verify it passes**

Run:

```bash
go test ./internal/output -run 'TestWriteJSONRoomList(DefaultOmitsVerboseFields|VerboseIncludesIDAndDetails)' -v
go test ./test/integration -run 'Test(RoomListDefaultOutputOmitsIDAndDetails|LightListDefaultOutputIsHumanReadable|LightListJSONOutputEnvelope)' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/cli/root.go internal/output/envelope_test.go test/integration/light_list_output_test.go test/integration/room_list_output_test.go
git commit -m "feat: align json output scope with verbosity levels"
```

### Task 5: Update Help Text, Docs, and Repo Policy

**Files:**
- Modify: `internal/cli/root_test.go`
- Modify: `README.md:44-73`
- Modify: `docs/cli/usage.md:3-60`
- Modify: `docs/cli/manpage.md:7-23`
- Modify: `AGENTS.md:45-71`

**Step 1: Write the failing tests**

Add a help-text test so the user-facing flag contract is enforced in code:

```go
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
```

**Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/cli -run TestRootHelpIncludesVerbosityLevels -v
```

Expected:
- FAIL because help text still describes a boolean `--verbose`

**Step 3: Write minimal implementation**

Update CLI help text and documentation together:

- README:
  - explain `default`, `-v`, `-vv`, `-vvv`
  - explain that `--json` uses the same verbosity-gated payload scope
- `docs/cli/usage.md`:
  - document the stable envelope shape and verbosity-gated field population
- `docs/cli/manpage.md`:
  - replace boolean `--verbose` wording with repeatable `-v`
- `AGENTS.md`:
  - replace the old "stable JSON means always full payload" assumption
  - codify the new rule that human and JSON share the same verbosity-defined data scope

**Step 4: Run test to verify it passes**

Run:

```bash
go test ./internal/cli -run TestRootHelpIncludesVerbosityLevels -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/cli/root_test.go README.md docs/cli/usage.md docs/cli/manpage.md AGENTS.md
git commit -m "docs: describe verbosity-gated output contract"
```

### Task 6: Run Full Verification Before Completion

**Files:**
- Modify: none unless a verification failure requires a follow-up fix

**Step 1: Run the required local checks**

Run:

```bash
go test ./...
go vet ./...
```

Expected:
- PASS

**Step 2: Run the required container checks with explicit Go path**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25 sh -lc 'go version && /usr/local/go/bin/go test ./... && /usr/local/go/bin/go vet ./... && /usr/local/go/bin/go build -o /tmp/huectl ./cmd/huectl'
```

Expected:
- PASS
- container prints a Go 1.25 version line before the checks

**Step 3: Build the Docker image**

Run:

```bash
docker build -t huectl:local .
```

Expected:
- PASS

**Step 4: Run simulator E2E coverage for output regressions**

Run:

```bash
set -e
trap 'docker compose -f test/simulator/compose.yml down -v --remove-orphans' EXIT
go-task sim-up
go-task test-e2e-sim
docker compose -f test/simulator/compose.yml down -v --remove-orphans
trap - EXIT
```

Expected:
- PASS
- simulator stack is cleaned up even if a test fails

**Step 5: Record verification results**

Capture the exact commands and whether they passed so the final report can cite them explicitly.
