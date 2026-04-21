# Hardware-in-the-Loop (HIL) Smoke Track

These checks are intentionally not part of CI and require a reachable Hue bridge on local network.

## Preconditions

1. Bridge reachable from host.
2. Bridge configured in `huectl` config.
3. Bridge linked with valid app key.

## Smoke Commands

```bash
huectl --bridge <id> bridge health
huectl --bridge <id> light list
huectl --bridge <id> scene list
huectl --bridge <id> light set --name <light-name> --body '{"on":{"on":true}}'
huectl --bridge <id> device search --body '{}'
```

## Expected

- Every command returns exit code `0` or `2` (partial for expected bridge subsets).
- `--json` output validates against `meta.schema=huectl/v1`.
