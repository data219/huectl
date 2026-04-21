# AGENTS.md (Repo Override for `huectl`)

This file adds repo-specific rules on top of the global AGENTS instructions.

## Go Toolchain In Containers

- Always verify Go availability inside container before running checks: `go version`.
- If `go` is not found in `PATH`, use `/usr/local/go/bin/go` explicitly.
- For Docker-based verification commands, prefer:
  - `/usr/local/go/bin/go test ./...`
  - `/usr/local/go/bin/go build ./cmd/huectl`
- Keep Docker build stages robust by setting:
  - `ENV PATH="/usr/local/go/bin:${PATH}"`

## Verification Requirement

Before claiming completion for code changes:

1. Run local checks:
   - `go test ./...`
   - `go vet ./...`
2. Run container checks (Go 1.25 image) using explicit Go path if needed.
3. Build Docker image at least once for regression safety:
   - `docker build -t huectl:local .`

## Simulator E2E Policy (diyHue)

- Use the built-in simulator workflow for E2E:
  - `go-task sim-up`
  - `go-task test-e2e-sim`
  - `go-task sim-down`
- In CI or release-like validation, enforce hard-fail mode:
  - `HUECTL_SIM_REQUIRED=1`
- Simulator user provisioning must be performed inside the container via:
  - `docker compose -f test/simulator/compose.yml exec -T diyhue ...`
  This is required to set the link button timestamp and create a Hue API user reliably.
- Keep cleanup guaranteed for simulator runs (trap/finally style), always bringing Compose down with volumes:
  - `docker compose -f test/simulator/compose.yml down -v --remove-orphans`
- Govern unsupported simulator behavior through:
  - `test/e2e/simulator/unsupported_allowlist.yaml`
  Rules:
  - Fail when a command newly becomes unsupported and is not allowlisted.
  - Fail when an allowlisted command starts passing (remove it from allowlist).

## CLI UX & Output Contract

- Default CLI output must be human-readable (tables/summary), never JSON-like envelopes unless `--json` is explicitly set.
- `--json` is a global automation mode and must keep stable envelope schema (`meta/data/error` with `meta.schema=huectl/v1`).
- Human-readable output and `--json` are two render forms over the same verbosity-defined data scope; `--json` must not expose a richer payload than human output at the same verbosity.
- Verbosity contract:
  - default: core business fields only; keep routine summary/meta hidden unless needed for correctness
  - `-v`: add domain-specific extra fields (for example resource `id`, `details`, or similar fachliche Zusatzfelder)
  - `-vv`: add diagnostics/execution metadata (`summary`, `request_id`, `timestamp`, `bridge_scope`, `duration_ms`, error `details`)
  - `-vvv`: may add low-level technical data when commands expose it
- Partial-success visibility is a correctness rule, not a verbosity bonus: keep bridge errors and `partial_success` visible at default output when they matter.
- Domain commands (`light`, `scene`, `automation`, `sensor`, `room`, `zone`, etc.) must use explicit action flags; do not reintroduce generic payload flags such as `--body` or `--type`.
- Raw JSON input is allowed only in expert `api` commands; prefer `--body-file` for larger payloads and enforce `--body` XOR `--body-file`.
- Multi-bridge execution must be explicit. Do not silently fan out operations across all configured bridges; require explicit scope flags for cross-bridge actions.
- Action-specific CLI flags must map deterministically to outbound API payload fields. Avoid hidden defaults when the user supplied explicit values.

## Linking & Fingerprint Security

- `bridge link` is hard-fail if certificate fingerprint cannot be persisted (`FINGERPRINT_FAILED` remains blocking).
- Fingerprint capture uses a dedicated TOFU-style TLS handshake for certificate extraction (self-signed bridges are expected in local networks).
- Restrict insecure TLS behavior to fingerprint capture only; normal API request security model must stay unchanged.
- Enforce fail-closed fingerprint verification for authenticated bridge operations (except bootstrap flows such as `discover`/`link`).
- Never print secrets in default output (`client_key`, tokens, raw credentialed URLs, raw bridge error bodies). Redact by default; allow verbose diagnostics only with explicit opt-in.

## Test Policy Additions

- For human renderer changes, update/add golden tests under `internal/output/testdata/` and assert deterministic key ordering.
- For shared output/view-model refactors, preserve the semantic difference between an empty successful resource payload and missing bridge results; successful empty list reads must keep the human empty state (`No resources.`), not fall back to bridge-status empty states.
- Keep integration coverage for output-mode contract:
  - default command output is human-readable
  - `--json` output returns valid envelope with expected schema/version.
- For list-output contract changes, add at least one regression test for successful empty resource collections in renderer/unit coverage and keep simulator or integration coverage for the default human-readable empty state.
- Maintain explicit v2→v1 fallback contract tests for both read and write paths.
- For every new or changed action flag, add tests that verify parser behavior and final outbound payload mapping for the corresponding command action.
- Add fallback contract coverage for `id` vs `id_v1` mismatch to ensure v1 fallback write paths use v1-compatible identifiers.
- Add regression tests that assert sensitive fields are redacted/omitted in both human and `--json` outputs.
