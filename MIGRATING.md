# Migrating from v2 to v3

Version 3 replaces rounded dynamic expiration with an explicit fallback delay on each output. It also extends the `core.Output` interface for custom output implementations.

## Update dynamic expiration

Remove `roundUpMinutes` from every dynamic input:

```json
{
  "expiration": {
    "type": "dynamic"
  }
}
```

Track metadata now expires at its exact reported duration. The removed `roundUpMinutes` setting is ignored if it remains in the configuration, but it no longer changes expiration.

## Configure output fallback timing

Add `fallbackDelay` to outputs that should bridge short gaps between tracks:

```json
{
  "type": "stereotool",
  "name": "rds",
  "inputs": ["radio-live", "default-text"],
  "settings": {
    "delay": 0,
    "fallbackDelay": 20,
    "hostname": "localhost",
    "port": 8080
  }
}
```

`fallbackDelay` is an extra number of seconds added to the normal `delay` when an output switches to a lower-priority input. It defaults to `0`. Returning to the current or a higher-priority input uses only the normal delay.

Outputs with little or no regular delay, such as RDS RadioText, typically need 15 to 30 seconds. Outputs already delayed by 10 seconds or more often need no additional fallback delay.

## Update custom outputs

The `core.Output` interface now requires:

```go
GetFallbackDelay() int
```

Custom outputs that embed `core.OutputBase` receive the implementation automatically. Add `FallbackDelay` to the output settings and pass it to the base during construction:

```go
output.SetDelay(settings.Delay)
output.SetFallbackDelay(settings.FallbackDelay)
```

Custom outputs that do not embed `core.OutputBase` must implement `GetFallbackDelay` directly.

## Verify the upgrade

1. Back up the production configuration.
2. Remove every `roundUpMinutes` setting.
3. Add `fallbackDelay` to outputs that should suppress short fallback flashes.
4. Start v3 with the updated configuration and check the logged delay values.
5. Let a dynamic input expire and confirm that each output switches at `delay + fallbackDelay` seconds.
