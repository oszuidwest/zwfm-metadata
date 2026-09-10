# Migrating from v2.6.5 to v3

Version 3 changes how dynamic inputs expire, how output delays are stored, and which Stereo Tool version is supported. This guide assumes you are upgrading from v2.6.5. For older installations, read the [release notes](https://github.com/oszuidwest/zwfm-metadata/releases) for the versions in between as well.

## Configuration

### Dynamic expiration

Remove `roundUpMinutes` from dynamic inputs:

```json
{
  "expiration": {
    "type": "dynamic"
  }
}
```

In v2, a missing or enabled `roundUpMinutes` rounded a track's duration up to the next full minute. In v3, a track expires at its reported duration. Nothing changes for inputs that already had `roundUpMinutes` set to `false`.

The old setting may remain in the file and is ignored. `expiration.minutes` still provides a fallback when the duration is missing or invalid. Without that fallback, the update expires immediately.

### Output delays

`delay` still applies to every update. The new `fallbackDelay` is added when an output moves to an input lower in its priority list:

```json
{
  "type": "stereotool",
  "name": "rds",
  "inputs": ["radio-live", "default-text"],
  "formatters": ["rds"],
  "settings": {
    "delay": 0,
    "fallbackDelay": 20,
    "hostname": "localhost",
    "port": 8080
  }
}
```

For example, if this output is showing `radio-live` and that input expires, `default-text` is sent after 20 seconds. A new `radio-live` update during those 20 seconds cancels the fallback. If `default-text` changes while it is waiting, the timer starts again. The first update after startup uses only `delay`, as does a move to a higher-priority input.

`fallbackDelay` defaults to `0`. Values are whole seconds and cannot be negative. For outputs with little or no normal delay, 15 to 30 seconds is usually enough to hide short gaps. Outputs that already have about 10 seconds of delay often need no extra fallback delay.

### Startup validation

V3 rejects invalid configuration at startup instead of continuing with partial or ambiguous behavior. Check custom configurations for unknown dynamic `expiration.type` values, unknown HTTP `responseType` values, and invalid payload templates.

### Stereo Tool

V3 targets Stereo Tool 11. It writes Streaming Output Song to field `6751` and FM RDS RadioText to field `9985`.

Upgrade Stereo Tool before deploying v3 and add the `rds` formatter to every Stereo Tool output. The formatter keeps RadioText within 64 characters and transliterates characters that Stereo Tool's RDS encoder does not handle correctly.

V3 fixes request encoding for metadata containing `/`, `&`, `+`, `%`, or `?`. Check both Song and Current RadioText after the upgrade.

## Dashboard WebSocket

The dashboard WebSocket payload no longer includes the redundant top-level `activeFlows` field or `available` on each input. Consumers can count outputs with a non-empty `currentInput` and use `status == "available"`, respectively.

## Custom outputs

Timing has moved out of output implementations and into `MetadataRouter`. The JSON location for `delay` is unchanged, and the new `fallbackDelay` belongs alongside it in the output's `settings` object.

The v3 `core.Output` interface is:

```go
type Output interface {
    Start(ctx context.Context) error
    GetName() string
    Send(st *StructuredText) error
}
```

`Send` errors are logged by the router and do not update its deduplication state. The router does not automatically retry failed deliveries.

Remove calls to `SetDelay`; that method no longer exists on `OutputBase`. `GetDelay` is no longer part of the `Output` interface either. If nothing else calls it, it can be removed too.

`OutputBase` now only stores the name:

```go
type MyOutputConfig struct {
    URL string `json:"url"`
}

func NewMyOutput(name string, settings MyOutputConfig) *MyOutput {
    return &MyOutput{
        OutputBase: core.NewOutputBase(name),
        settings:   settings,
    }
}
```

Remove `Delay` from an output-specific config struct, and do not add `FallbackDelay` there. `setupOutput` reads both values separately through `core.OutputTiming`, so a type added to the `createOutput` switch needs no timing code of its own.

Code that builds a router directly, without `setupOutput`, supplies the inputs and timing when it registers the output:

```go
router := core.NewMetadataRouter()
output := NewMyOutput("custom", settings)

if err := router.AddOutput(output, &core.OutputSpec{
    Type:   "custom",
    Inputs: []string{"radio-live", "fallback"},
    Timing: core.OutputTiming{
        Delay:         2,
        FallbackDelay: 20,
    },
}); err != nil {
    return err
}
```

With a zero-value `OutputTiming`, both delays are `0`. `AddOutput` requires at least one unique, registered input and rejects negative timing. Inputs and outputs cannot be added after `Start`.

The built-in output config structs no longer contain `Delay`. Code that creates values such as `config.FileOutputConfig` directly must configure the router with `core.OutputTiming`.

Compared with v2.6.5, the dashboard JSON adds `fallbackDelay` as a top-level property next to `delay` for each output. The Go type `core.OutputStatus` embeds `core.OutputTiming`, which matters only to code that constructs that struct directly.

## Test the upgrade

Keep a copy of the old config and binary until the new version has run successfully. On startup, check the logged delay values and the output cards in the dashboard. A normal update should arrive after `delay`; a fallback should arrive after `delay + fallbackDelay`. The expiration check runs once per second, so a fallback can be almost one second later.

While a fallback is waiting, send another update from the current input. The fallback should be cancelled. For Stereo Tool, use a test value such as `BLØF / Test & More + 100%?` and inspect Current RadioText as well as the stored Song value.
