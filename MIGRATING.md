# Migrating from v2.6.5 to v3

Version 3 changes dynamic expiration, output timing, dashboard data, and several Go APIs used by custom components. This guide assumes an upgrade from v2.6.5. For older installations, also read the intervening [release notes](https://github.com/oszuidwest/zwfm-metadata/releases).

If you only run the binary with JSON configuration, the sections before [Go API changes](#go-api-changes) are sufficient. If you maintain custom Go code, complete that section too.

## Dynamic expiration

Remove `roundUpMinutes` from dynamic inputs:

```json
{
  "expiration": {
    "type": "dynamic"
  }
}
```

In v2, a missing or enabled `roundUpMinutes` rounded a track's duration up to the next full minute. In v3, a track expires at its reported duration. Inputs that already set `roundUpMinutes` to `false` keep the same behavior.

The old setting may remain in the JSON file and is ignored. `expiration.minutes` still provides a fallback when the duration is missing, zero, or invalid. With no positive fallback, the update expires immediately.

## Output delays

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

If this output is showing `radio-live` when that input expires, `default-text` is sent after 20 seconds. A new `radio-live` update during that wait cancels the fallback. If `default-text` changes while it is waiting, the full timer starts again. The first update after startup and moves to a higher-priority input use only `delay`.

`fallbackDelay` defaults to `0`. Both delay values are whole seconds and cannot be negative. Outputs with little or no normal delay generally need 15 to 30 seconds to hide short gaps. Outputs that already have about 10 seconds of delay often need no extra fallback delay.

## Configuration validation

V3 centralizes more validation at startup and rejects invalid or ambiguous settings instead of deferring failures until an update is processed. Check for:

- unknown dynamic `expiration.type` values;
- an output `delay` or `fallbackDelay` below zero;
- missing, duplicate, or unknown inputs on an output;
- bearer tokens configured for non-HTTPS URLs;
- unknown HTTP `responseType` values; and
- invalid payload templates.

An HTTP endpoint that used `"responseType": "custom"` with a payload mapping must use `"responseType": "json"` in v3. The payload mapping itself is unchanged.

The existing validation for URL schemes, polling intervals, HTTP methods, and URL templates still applies.

These checks do not replace validation of every external service or application-specific value. Start the new binary with the production configuration before switching traffic.

## Stereo Tool

V3 targets Stereo Tool 11. It writes Streaming Output Song to field `6751` and FM RDS RadioText to field `9985`.

Upgrade Stereo Tool before deploying v3 and add the `rds` formatter to every Stereo Tool output. The formatter keeps RadioText within 64 characters and transliterates characters that Stereo Tool's RDS encoder does not handle correctly.

V3 fixes request encoding for metadata containing `/`, `&`, `+`, `%`, or `?`. Check both Song and Current RadioText after the upgrade.

## Dashboard WebSocket

The dashboard WebSocket payload no longer includes the redundant top-level `activeFlows` field or the `available` property on each input.

- Count outputs with a non-empty `currentInput` to replace `activeFlows`.
- Test `status == "available"` to replace an input's `available` property.

Each output now includes `fallbackDelay` next to `delay`. In Go, `core.OutputStatus` replaces the former `web.OutputStatus` and embeds `core.OutputTiming`.

## Go API changes

### Raw configuration settings

`config.InputConfig.Settings` and `config.OutputConfig.Settings` changed from `map[string]any` to `json.RawMessage`. JSON configuration files do not need a change, but Go code that constructs config values directly does:

```go
cfg := config.OutputConfig{
    Type:   "file",
    Name:   "archive",
    Inputs: []string{"radio-live"},
    Settings: json.RawMessage(`{
        "delay": 2,
        "filename": "/tmp/metadata.txt"
    }`),
}
```

`utils.ParseJSONSettings` now decodes that raw JSON directly and returns a value. Remove the old pointer dereference:

```go
// v2
settings, err := utils.ParseJSONSettings[config.FileOutputConfig](cfg.Settings)
output := outputs.NewFileOutput(cfg.Name, *settings)

// v3
settings, err := utils.ParseJSONSettings[config.FileOutputConfig](cfg.Settings)
output := outputs.NewFileOutput(cfg.Name, settings)
```

An absent or empty `Settings` value now decodes to the zero value of the target type.

### Router registration

`AddInput` and `AddOutput` now receive complete, immutable specs. Register every input before any output that refers to it:

```go
router := core.NewMetadataRouter()

if err := router.AddInput(input, core.InputSpec{
    Type:        "custom",
    Prefix:      "Now playing: ",
    Suffix:      " on ZuidWest FM",
    Filters:     inputFilters,
    FilterNames: []string{"pattern"},
}); err != nil {
    return err
}

if err := router.AddOutput(output, core.OutputSpec{
    Type:           "custom",
    Inputs:         []string{input.GetName()},
    Formatters:     outputFormatters,
    FormatterNames: []string{"rds"},
    Timing: core.OutputTiming{
        Delay:         2,
        FallbackDelay: 20,
    },
}); err != nil {
    return err
}
```

The following v2 configuration methods no longer exist; move their values into the corresponding spec:

| Removed method            | V3 field                               |
| ------------------------- | -------------------------------------- |
| `SetInputType`            | `InputSpec.Type`                       |
| `SetInputPrefixSuffix`    | `InputSpec.Prefix`, `InputSpec.Suffix` |
| `SetInputFilters`         | `InputSpec.Filters`                    |
| `SetInputFilterNames`     | `InputSpec.FilterNames`                |
| `SetOutputType`           | `OutputSpec.Type`                      |
| `SetOutputInputs`         | `OutputSpec.Inputs`                    |
| `SetOutputFormatters`     | `OutputSpec.Formatters`                |
| `SetOutputFormatterNames` | `OutputSpec.FormatterNames`            |

`OutputSpec.Timing` replaces the delay previously stored on each output through `OutputBase.SetDelay`.

The old router getters for those separate maps were also removed. Use `GetInputStatus` or `GetOutputStatus` when you need a complete status snapshot. The exported `InputPrefixSuffix`, `ScheduledUpdate`, and `Timeline` types were router implementation details and have also been removed.

With a zero-value `OutputTiming`, both delays are zero. `AddOutput` rejects negative timing and requires at least one unique, registered input. Inputs and outputs cannot be added after `Start`.

### Custom outputs

The v3 `core.Output` interface is:

```go
type Output interface {
    Start(ctx context.Context) error
    GetName() string
    Send(st *StructuredText) error
}
```

Change every custom `Send` method to return its delivery error. The router logs that error and does not update its deduplication state. It does not automatically retry failed deliveries.

`OutputBase` now stores only the name. Remove calls to `SetDelay`; remove custom uses of `GetDelay` where they are no longer needed. Shared timing belongs in `OutputSpec.Timing`, not in an output-specific config struct.

The built-in output config structs no longer contain `Delay`. Code that creates values such as `config.FileOutputConfig` directly must configure timing on the router.

### Component factories

The formatter and filter registries were replaced by explicit factory switches:

| V2                             | V3                             |
| ------------------------------ | ------------------------------ |
| `formatters.RegisterFormatter` | Add a case to `formatters.New` |
| `formatters.GetFormatter`      | `formatters.New`               |
| `filters.RegisterFilter`       | Add a case to `filters.New`    |
| `filters.GetFilter`            | `filters.New`                  |

Custom component types are therefore source-level extensions compiled into the binary. See [EXTENDING.md](EXTENDING.md) for the current pattern.

### Constructor changes

Most constructors still accept their config by value. These signatures changed:

| Constructor | V3 change |
| --- | --- |
| `inputs.NewDynamicInput` | Now returns `(*DynamicInput, error)` |
| `outputs.NewHTTPOutput` | Now returns `(*HTTPOutput, error)` |
| `outputs.NewWebSocketOutput` | Now returns `(*WebSocketOutput, error)` |
| `outputs.NewIcecastOutput` | Accepts `IcecastOutputConfig` by value instead of pointer |
| `outputs.NewPayloadMapper` | Now returns `(*PayloadMapper, error)` |

Handle these errors during startup. They report invalid expiration types, response types, or templates before the router starts.

### Payload and HTTP helpers

`outputs.ConvertStructuredTextWithType` was removed. Set the type explicitly:

```go
metadata := outputs.ConvertStructuredText(st)
metadata.Type = "custom"
```

`NewPayloadMapper(nil)` now returns a nil mapper and nil error. Calling `mapper.Apply(metadata)` is valid even when `mapper` is nil; it passes the metadata through unchanged. Invalid templates are returned as constructor errors instead of being logged and retained as raw strings.

`UniversalMetadata.ToTemplateData` now always includes every supported key. Unavailable optional values render as empty strings instead of `<no value>`. Programmatically constructed mappings should use JSON-shaped `[]any` arrays; only objects inside arrays are recursively templated.

`outputs.TemplateFuncs` is no longer exported. Payload mapping supports `lower`, `upper`, and `trim`; custom templates should define their own `template.FuncMap`.

`utils.Do` was removed. Use `utils.DoOK` when only a successful response matters; it closes the body and returns non-2xx responses as errors. Use `utils.Get` when the caller needs to inspect a GET response, and close its body.

### Shared metadata

`InputBase.GetMetadata` now returns the stored pointer instead of a clone. `Metadata.Clone` and `StructuredText.Clone` were removed. Treat values passed to `SetMetadata`, returned by `GetMetadata`, received through subscriptions, and exposed as `StructuredText.Original` as read-only.

If custom code needs a mutable value, copy it first. Copy `ExpiresAt` separately when a fully independent `Metadata` value is required. Call `core.NewStructuredText` only with a non-nil `*core.Metadata`.

`PassiveComponent.Start` now returns immediately instead of waiting for context cancellation. Components with background work must implement their own `Start` method and stop that work when the context is cancelled.

## Test the upgrade

Keep a copy of the v2 configuration and binary until v3 has run successfully. Then:

1. Run `go test ./...`, `go vet ./...`, and `go build` for deployments with custom code.
2. Start v3 with the production configuration and resolve every validation error before switching traffic.
3. Confirm the dashboard shows the expected inputs, outputs, delays, and current input selection.
4. Check that a normal update arrives after `delay` and a fallback after `delay + fallbackDelay`. The expiration check runs once per second, so a fallback can be almost one second later.
5. While a fallback is waiting, send another update from the current input and confirm that the fallback is cancelled.
6. For Stereo Tool, send `BLØF / Test & More + 100%?` and inspect both Current RadioText and the stored Song value.
