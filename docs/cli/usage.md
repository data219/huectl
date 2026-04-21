# huectl CLI Usage

## Output Contract

`huectl` has two renderers over one shared verbosity-scoped dataset:

- default human-readable output
- `--json` machine-readable output

Every `--json` response keeps the same top-level envelope:

```json
{
  "meta": {
    "schema": "huectl/v1",
    "command": "room list",
    "request_id": "",
    "timestamp": "",
    "bridge_scope": null,
    "duration_ms": 0,
    "partial_success": false
  },
  "data": {
    "resource_rows": [
      {
        "bridge": "Dachgeschoss",
        "name": "Wohnzimmer"
      }
    ]
  },
  "error": null
}
```

Verbosity defines which fields are populated in both renderers:

- default: core business fields only; routine human `summary:` / `meta:` lines stay hidden, but partial-success bridge errors still remain visible
- `-v`: add domain-specific extra fields such as resource `id`, `details`, and other non-diagnostic columns
- `-vv`: add diagnostics and execution metadata such as `summary`, `request_id`, `timestamp`, `bridge_scope`, `duration_ms`, and error `details`
- `-vvv`: allow low-level technical fields when a command exposes them

Error payload:

```json
{
  "meta": {
    "schema": "huectl/v1",
    "command": "light set",
    "request_id": "uuid",
    "timestamp": "RFC3339",
    "bridge_scope": ["bridge-a"],
    "duration_ms": 37,
    "partial_success": false
  },
  "data": null,
  "error": {
    "code": "TARGET_AMBIGUOUS",
    "message": "Target name 'Kitchen' matches multiple resources.",
    "hints": ["Use --bridge", "Use composite id bridge/resource"],
    "details": {}
  }
}
```

`error.details` is diagnostic scope and is expected at `-vv` and above.

## Multi-Bridge Write Safety

- Reads aggregate across all scoped bridges.
- Writes resolve target uniqueness first.
- On ambiguity, writes fail with `TARGET_AMBIGUOUS`.
- Use `--broadcast` to explicitly write to all matches.

## Examples

```bash
# human output
huectl bridge list

# human output with extra domain fields
huectl -v room list

# machine output with the same default field scope
huectl --json bridge list

# machine output with diagnostics
huectl --json -vv room list

# scope to one bridge
huectl --bridge living-room light list

# explicit multi-write
huectl --broadcast light off --name Kitchen

# raw API
huectl --bridge living-room api get --path /resource/light
```
