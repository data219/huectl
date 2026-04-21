# CLI Output Verbosity Design

## Context

`huectl` currently mixes three concerns in its output layer:

- human-readable presentation
- JSON presentation
- unconditional diagnostic metadata (`summary`, `meta`, wide resource tables)

The approved direction is to separate presentation from data scope:

- `--json` and default human-readable output are only two renderers
- the data scope is controlled globally by repeatable verbosity flags
- `-v` enables fachliche Zusatzdaten
- `-vv` enables Diagnose-/Ausfuehrungsdaten
- `-vvv` enables low-level technical data

## Reviewed Inputs

- user requirements from the current `room list` output
- current CLI/output implementation in:
  - `internal/cli/root.go`
  - `internal/domain/types.go`
  - `internal/output/envelope.go`
  - `internal/output/renderer.go`
- current tests and goldens in:
  - `internal/output/renderer_test.go`
  - `internal/output/testdata/`
  - `test/integration/light_list_output_test.go`
- review findings from:
  - `create-cli`
  - `code-review-excellence`
  - `code-review-master`
  - `skeptic-router`
  - `skeptic-original`

## Requirements

### Global contract

- Replace boolean `--verbose` with repeatable `-v/-vv/-vvv`.
- The same verbosity level must expose the same data scope in human-readable and `--json` output.
- `--json` remains a renderer, not a "show everything" escape hatch.
- The JSON envelope shape may stay `meta/data/error`, but verbosity controls which fields are populated and emitted inside it.

### Verbosity levels

- `Default`
  - show task-relevant primary data only
  - hide routine `summary`
  - hide routine `meta`
  - hide redundant columns such as `TYPE` for context-clear commands like `room list`
- `-v`
  - show fachliche Zusatzdaten
  - examples: `id`, `details`, non-redundant follow-up fields
- `-vv`
  - show Diagnose-/Ausfuehrungsdaten
  - examples: `summary`, `request_id`, `duration_ms`, bridge scope, fallback hints
- `-vvv`
  - show low-level technical data
  - examples: deeper redacted transport/fallback context and redacted raw details

### Failure and partial-success rules

- If at least one bridge fails, the normal output must still show a clear partial-success signal without requiring verbosity.
- `bridge errors` stay visible when needed for correctness/recovery.
- `ERROR` and `HINT` remain visible in normal error output.
- sensitive values must stay redacted on every verbosity level unless a future explicit diagnostics mode is designed

## Architecture

Use a shared verbosity-gated view model between the service results and the final renderer.

### Layering

1. Raw result:
   - current `Envelope` and domain result types from app/service execution
2. View-model builder:
   - receives command name, JSON/human mode, verbosity level, raw result
   - applies command-family field profiles and verbosity filtering exactly once
3. Renderer:
   - renders the filtered result either as human text or JSON

This prevents drift between human and JSON output and keeps the visibility rules in one place.

### Command-family field profiles

The view-model builder must support command-specific field profiles, starting with resource list commands.

Examples:

- `room list`
  - default: `bridge`, `name`, optional `status`
  - `-v`: add `id`, `details`
  - `-vv`: add summary/meta-style diagnostics
- `light list`
  - default can still keep `status` because it is user-relevant
  - `type` remains redundant in context-clear single-domain lists
- bridge/health-style aggregate outputs
  - must retain failure signals even at default verbosity

## Testing Strategy

- Add parser/help tests for repeatable global verbosity flags.
- Add view-model tests for default, `-v`, `-vv`, `-vvv`.
- Add golden tests for human output by verbosity level.
- Add JSON contract tests for the same verbosity levels.
- Add regression tests for:
  - `room list` default vs `-v` vs `-vv`
  - partial success visibility without verbosity
  - redaction of sensitive fields
  - consistent field scope between human-readable and `--json`

## Documentation and Policy Updates

Update the user-facing contract in:

- `README.md`
- `docs/cli/usage.md`
- `docs/cli/manpage.md`

Update repo policy in `AGENTS.md` to reflect:

- repeatable verbosity levels
- verbosity-gated JSON payload scope
- continued stable envelope schema

## Risks and Guardrails

- This is a deliberate breaking change for human output.
- Scripts that scrape the default table output may break; docs must point them to `--json` and/or `-v`.
- Output changes must not hide failure signals required for recovery.
- No secret leakage is allowed while expanding technical diagnostics at `-vvv`.
