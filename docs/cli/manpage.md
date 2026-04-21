# huectl Manpage (Summary)

## NAME

`huectl` - manage Philips Hue bridges and locally reachable Hue resources.

## SYNOPSIS

```text
huectl [global options] <domain> <action> [action options]
```

## GLOBAL OPTIONS

- `--json`
- `--bridge <id|name>`
- `--all-bridges`
- `--broadcast`
- `--timeout <duration>`
- `-v, --verbose` (repeat for more detail: `-v`, `-vv`, `-vvv`)
- `--no-color`
- `--config <path>`

## OUTPUT SCOPE

- Human-readable output and `--json` share the same verbosity-defined data scope.
- Default output shows core business fields only.
- `-v` adds domain-specific extra fields.
- `-vv` adds diagnostics such as summary and execution metadata.
- `-vvv` may add low-level technical fields.

## DOMAINS

- `bridge`
- `device`
- `light`
- `room`
- `zone`
- `scene`
- `automation`
- `sensor`
- `entertainment`
- `update`
- `backup`
- `diagnose`
- `api`

Run `huectl <domain> --help` or `huectl <domain> <action> --help` for details.
