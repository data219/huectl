# huectl

`huectl` is a Linux-first CLI to manage Philips Hue bridges, lights, rooms, zones, scenes, sensors, automations, updates, diagnostics, and raw API access.

## Design Goals

- One CLI command with subcommands (`huectl ...`)
- Multi-bridge aware by default
- Human-readable output by default
- Stable, machine-readable JSON envelope via global `--json`
- Non-interactive, one-shot commands only

## Quick Start

```bash
# Build
GOOS=linux GOARCH=amd64 go build -o bin/huectl ./cmd/huectl

# Discover bridges
./bin/huectl bridge discover

# Add a bridge
./bin/huectl bridge add --name living-room --address 192.168.1.10

# Link (press physical bridge button first)
./bin/huectl --bridge living-room bridge link

# List lights
./bin/huectl light list

# Show extra business columns for rooms
./bin/huectl -v room list

# Set light state
./bin/huectl light set --name Kitchen --on --brightness 55

# Activate scene with transition
./bin/huectl scene activate --name Evening --transition-ms 500

# Create automation with explicit schedule flags
./bin/huectl automation create --name Morning --trigger time --at 07:30:00 --every Mon,Tue,Wed --enable

# JSON output for automation
./bin/huectl --json light list

# JSON output with diagnostic metadata
./bin/huectl --json -vv room list
```

## Global Flags

- `--json`: stable JSON envelope output (`huectl/v1`)
- `--bridge <name|id>`: scope commands to one bridge
- `--all-bridges`: scope to all configured bridges
- `--broadcast`: explicit write fanout for ambiguous targets
- `--timeout <duration>`: command timeout (e.g., `10s`)
- `-v, --verbose`: increase output verbosity; repeat for more detail (`-v`, `-vv`, `-vvv`)
- `--no-color`: disable ANSI colors in human output
- `--config <path>`: override config location

Default config file: `~/.config/huectl/config.yaml` (must be `0600`).

## Output Modes

- Default mode renders human-readable output.
- `--json` renders the same data scope in the stable `meta` / `data` / `error` envelope.
- Verbosity controls how much data both renderers expose:
  - default: core business fields only; routine summary/meta is hidden, but partial-success bridge errors still show
  - `-v`: add domain-specific extra fields such as resource `id` and `details`
  - `-vv`: add diagnostics such as `summary`, `request_id`, `timestamp`, `bridge_scope`, `duration_ms`, and error `details`
  - `-vvv`: allow low-level technical fields when a command exposes them

Examples:

```bash
# Human-readable output (default)
./bin/huectl light list

# Human-readable output with extra domain fields
./bin/huectl -v room list

# Machine-readable output with the same default field scope
./bin/huectl --json light list

# Machine-readable output with diagnostics enabled
./bin/huectl --json -vv room list
```

## Command Domains

- `bridge`: `discover`, `add`, `link`, `list`, `show`, `rename`, `remove`, `health`, `capabilities`
- `device`: `search`, `list`, `show`, `identify`, `rename`, `delete`
- `light`: `list`, `show`, `on`, `off`, `toggle`, `set`, `effect`, `flash`
- `room`: `list`, `create`, `update`, `delete`, `assign`, `unassign`
- `zone`: `list`, `create`, `update`, `delete`, `assign`, `unassign`
- `scene`: `list`, `show`, `create`, `update`, `delete`, `activate`, `clone`
- `automation`: `list`, `show`, `create`, `update`, `delete`, `enable`, `disable`, `run`
- `sensor`: `list`, `show`, `rename`, `sensitivity`, `enable`, `disable`
- `entertainment`: `area list|create|update|delete`, `session start|stop`
- `update`: `list`, `check`, `install`, `status`
- `backup`: `export`, `import`, `diff`
- `diagnose`: `ping`, `latency`, `events`, `logs`
- `api`: `get`, `post`, `put`, `delete` (raw expert mode)

## API Expert Mode

`huectl api ...` keeps raw JSON input as an expert escape hatch.

```bash
# Inline JSON
./bin/huectl api post --path /resource/light --body '{"on":{"on":true}}'

# JSON from file (recommended for larger payloads)
./bin/huectl api post --path /resource/light --body-file payload.json
```

Notes:
- Use either `--body` or `--body-file` (not both).
- Domain commands outside `api` are intentionally human-flag driven (no generic JSON body).

## Scene & Automation UX

- Scene flags:
  - `scene activate`: `--dynamic`, `--transition-ms`
  - `scene create/update`: `--room-id` or `--zone-id` (exactly one for create)
- Automation flags:
  - `automation create/update`: `--script`, `--trigger`, `--at`, `--every`, `--enable`, `--disable`
- Validation errors include concrete CLI examples to speed up correction.

## Exit Codes

- `0` success
- `1` CLI usage/validation error
- `2` partial success (multi-bridge, partial failures)
- `3` auth/linking/permission error
- `4` not found / ambiguous target
- `5` bridge unreachable / timeout
- `6` rate-limit / retry exhausted
- `7` internal error

## Testing

```bash
go test ./...
```

The test suite includes:

- output envelope schema tests
- resolver ambiguity/broadcast tests
- config permission safety tests
- fallback request behavior tests
- CLI global `--json` coverage over all leaf commands
