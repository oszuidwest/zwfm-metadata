# Extending ZuidWest FM Metadata

This guide explains how to add an input, output, formatter, or filter to the application. Extensions are compiled into the binary; there is no runtime plugin system.

For configuration of existing components, see [README.md](README.md). When upgrading extension code from v2, see [MIGRATING.md](MIGRATING.md).

## How metadata moves through the application

The router processes metadata in this order:

1. An input publishes `core.Metadata` through `core.InputBase`.
2. The router adds the input prefix and suffix and applies its filters.
3. For each output, the router selects the highest-priority available input.
4. The router applies the output's formatter chain.
5. After the configured delay, the router calls `Output.Send`.

The router also handles expiration, fallback delays, and output deduplication. All inputs, outputs, filters, and formatters must be registered before `MetadataRouter.Start` is called.

## Shared rules

- Embed `*core.InputBase` in inputs and `*core.OutputBase` in outputs.
- Embed `core.PassiveComponent` when a component has no background work.
- Treat metadata as immutable after calling `InputBase.SetMetadata`. `GetMetadata`, subscribers, and `StructuredText.Original` share pointers.
- Return constructor errors for invalid configuration.
- Return delivery errors from `Output.Send`. The router logs the error and does not update its deduplication state. It does not automatically retry.
- Use `log/slog` and include the component name as an `input` or `output` field.
- Use `utils.DoOK` for requests where only success matters. It sets the standard User-Agent, applies the shared timeout, closes the response body, and returns non-2xx responses as errors.
- Run `go test ./...`, `go vet ./...`, and `go build` before committing.

## Add an input

An input implements `core.Input`:

```go
type Input interface {
    Start(ctx context.Context) error
    GetName() string
    GetMetadata() *Metadata
    Subscribe(ch chan<- *Metadata)
}
```

`InputBase` supplies `GetName`, `GetMetadata`, `Subscribe`, and `SetMetadata`. For a passive input, `PassiveComponent` supplies `Start`.

### 1. Implement the input

This complete passive input can be added as `inputs/manual.go`:

```go
package inputs

import (
    "time"

    "zwfm-metadata/core"
)

// ManualInput accepts metadata pushed by another part of the application.
type ManualInput struct {
    *core.InputBase
    core.PassiveComponent
}

// NewManualInput creates an empty manual input.
func NewManualInput(name string) *ManualInput {
    return &ManualInput{InputBase: core.NewInputBase(name)}
}

// Update publishes a new title and artist.
func (i *ManualInput) Update(title, artist string) {
    i.SetMetadata(&core.Metadata{
        Title:     title,
        Artist:    artist,
        UpdatedAt: time.Now(),
    })
}
```

Wire `Update` to the system that supplies the metadata. Implementing an HTTP handler is separate from implementing an input; the built-in dynamic input is a useful example.

For an active input, do not embed `PassiveComponent`. Implement `Start`, stop when `ctx.Done()` is closed, and release tickers, clients, and other resources before returning. Pass the supplied context into operations that support cancellation. See [`inputs/url.go`](inputs/url.go) for a polling input.

`SetMetadata(nil)` clears an input. A non-nil value is considered available only when it has a title and has not expired. `SetMetadata` notifies subscribers when the title, artist, song ID, or duration changes; changing timestamps alone does not publish an update.

### 2. Add configuration when needed

Put type-specific settings in `config/config.go`:

```go
type MyInputConfig struct {
    URL             string `json:"url"`
    PollingInterval int    `json:"pollingInterval"`
}
```

Parse settings in the `createInput` switch in `main.go`. The generic helper returns a value, not a pointer:

```go
case "myinput":
    settings, err := utils.ParseJSONSettings[config.MyInputConfig](cfg.Settings)
    if err != nil {
        return nil, err
    }
    return inputs.NewMyInput(cfg.Name, settings)
```

If a constructor cannot fail, return the component and `nil` from the switch. For the `ManualInput` above:

```go
case "manual":
    return inputs.NewManualInput(cfg.Name), nil
```

`setupInput` creates the `core.InputSpec` from the input's type, prefix, suffix, and filters. A new input type therefore needs no router-specific setup.

### 3. Test the input

Test metadata publication and any parsing, validation, expiration, or polling behavior. A minimal test for the passive example is:

```go
func TestManualInputUpdate(t *testing.T) {
    input := NewManualInput("manual")
    input.Update("Song", "Artist")

    metadata := input.GetMetadata()
    if metadata == nil || metadata.Title != "Song" || metadata.Artist != "Artist" {
        t.Fatalf("GetMetadata() = %#v", metadata)
    }
}
```

## Add an output

An output implements `core.Output`:

```go
type Output interface {
    Start(ctx context.Context) error
    GetName() string
    Send(st *StructuredText) error
}
```

`OutputBase` supplies `GetName`; `PassiveComponent` supplies `Start` for outputs without background work. The router serializes `Send` calls for each output. HTTP handlers and other goroutines may still run concurrently with `Send`, so protect any state they share.

### 1. Add the output settings

Add only settings owned by the output to `config/config.go`:

```go
type WebhookOutputConfig struct {
    URL string `json:"url"`
}
```

Do not add `Delay` or `FallbackDelay` to this struct. `setupOutput` parses those shared fields separately into `core.OutputTiming`.

### 2. Implement the output

This complete output can be added as `outputs/webhook.go`:

```go
package outputs

import (
    "context"
    "fmt"
    "net/http"
    "strings"
    "time"

    "zwfm-metadata/config"
    "zwfm-metadata/core"
    "zwfm-metadata/utils"
)

// WebhookOutput sends the formatted text to an HTTP endpoint.
type WebhookOutput struct {
    *core.OutputBase
    core.PassiveComponent
    url string
}

// NewWebhookOutput validates settings and creates a webhook output.
func NewWebhookOutput(name string, settings config.WebhookOutputConfig) (*WebhookOutput, error) {
    if err := utils.ValidateHTTPURL(settings.URL); err != nil {
        return nil, err
    }
    return &WebhookOutput{
        OutputBase: core.NewOutputBase(name),
        url:        settings.URL,
    }, nil
}

// Send delivers one formatted metadata update.
func (o *WebhookOutput) Send(st *core.StructuredText) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(
        ctx,
        http.MethodPost,
        o.url,
        strings.NewReader(st.String()),
    )
    if err != nil {
        return fmt.Errorf("create webhook request: %w", err)
    }
    req.Header.Set("Content-Type", "text/plain; charset=utf-8")

    if err := utils.DoOK(req); err != nil {
        return fmt.Errorf("send webhook request: %w", err)
    }
    return nil
}
```

`utils.DoOK` owns the response body. Do not close or read it again. Use `utils.Get` instead when an input needs the response body, and always close the returned body.

### 3. Register the output

Add a case to `createOutput` in `main.go`:

```go
case "webhook":
    settings, err := utils.ParseJSONSettings[config.WebhookOutputConfig](cfg.Settings)
    if err != nil {
        return nil, err
    }
    return outputs.NewWebhookOutput(cfg.Name, settings)
```

The configuration can then use the output:

```json
{
  "type": "webhook",
  "name": "website",
  "inputs": ["radio-live", "fallback"],
  "formatters": ["ucwords"],
  "settings": {
    "delay": 2,
    "fallbackDelay": 20,
    "url": "https://example.com/metadata"
  }
}
```

### Optional HTTP routes

An output that serves HTTP or WebSocket clients can also implement:

```go
type RouteRegistrar interface {
    RegisterRoutes(mux *http.ServeMux)
}
```

The web server calls `RegisterRoutes` during startup. Keep configurable patterns valid and unique: `http.ServeMux` panics on invalid or duplicate patterns. See [`outputs/http.go`](outputs/http.go) and [`outputs/websocket.go`](outputs/websocket.go).

### Optional universal payload and mapping

Outputs that send structured payloads can start with `outputs.ConvertStructuredText`. It preserves the formatted text, individual fields, original metadata, and input identity:

```go
metadata := ConvertStructuredText(st)
metadata.Type = "webhook"
```

For user-configurable JSON shapes, compile a payload mapper in the constructor:

```go
mapper, err := NewPayloadMapper(settings.PayloadMapping)
if err != nil {
    return nil, fmt.Errorf("create payload mapper: %w", err)
}
```

Store `mapper` on the output and call `mapper.Apply(metadata)` before encoding. A nil mapping produces a nil mapper; calling `Apply` on it is supported and returns the original metadata. Templates have the fields returned by `UniversalMetadata.ToTemplateData` and the `lower`, `upper`, and `trim` functions. See [`outputs/url.go`](outputs/url.go) for an end-to-end example.

### 4. Test the output

Use `httptest.Server` for HTTP delivery and assert the method, headers, body, and error behavior. Constructor validation and non-2xx responses need tests too. See [`outputs/url_test.go`](outputs/url_test.go) and [`outputs/http_test.go`](outputs/http_test.go).

## Add a formatter

Formatters modify `StructuredText` in place. They may change its text fields but must not mutate `st.Original`.

Add `formatters/collapse_space.go`:

```go
package formatters

import (
    "strings"

    "zwfm-metadata/core"
)

// CollapseSpaceFormatter replaces runs of whitespace with one space.
type CollapseSpaceFormatter struct{}

// Format normalizes whitespace in the artist and title.
func (f *CollapseSpaceFormatter) Format(st *core.StructuredText) {
    st.Artist = strings.Join(strings.Fields(st.Artist), " ")
    st.Title = strings.Join(strings.Fields(st.Title), " ")
}
```

Add a case to `formatters.New` in `formatters/registry.go`:

```go
case "collapse-space":
    return &CollapseSpaceFormatter{}, nil
```

Test artist and title separately, plus empty strings and Unicode input. Formatter order is significant: the router applies them in configuration order.

## Add a filter

Filters inspect `StructuredText` before formatters run and return one of these actions:

| Action | Effect |
| --- | --- |
| `core.FilterPass` | Keep both fields |
| `core.FilterClearArtist` | Clear only the artist |
| `core.FilterClearTitle` | Clear only the title |
| `core.FilterReject` | Clear both fields and keep the output's previous content |

For a configurable filter, first add its setting to `config.FilterConfig`:

```go
MinTitleRunes int `json:"minTitleRunes,omitempty"`
```

Then add `filters/title_length.go`:

```go
package filters

import (
    "errors"
    "unicode/utf8"

    "zwfm-metadata/core"
)

// TitleLengthFilter rejects titles shorter than a configured rune count.
type TitleLengthFilter struct {
    minimum int
}

// NewTitleLengthFilter validates the minimum and creates the filter.
func NewTitleLengthFilter(minimum int) (*TitleLengthFilter, error) {
    if minimum < 1 {
        return nil, errors.New("minimum title length must be positive")
    }
    return &TitleLengthFilter{minimum: minimum}, nil
}

// Decide rejects metadata with a title shorter than the minimum.
func (f *TitleLengthFilter) Decide(st *core.StructuredText) core.FilterAction {
    if utf8.RuneCountInString(st.Title) < f.minimum {
        return core.FilterReject
    }
    return core.FilterPass
}
```

Add a case to `filters.New` in `filters/registry.go`:

```go
case "title-length":
    return NewTitleLengthFilter(cfg.MinTitleRunes)
```

Test every action the filter can return. When multiple filters clear fields, their effects are cumulative.

## Build a router directly

Most extensions only need a constructor switch entry because `setupInput` and `setupOutput` build the specs. Code that constructs a router directly must register inputs before outputs and supply complete specs:

```go
router := core.NewMetadataRouter()

if err := router.AddInput(input, &core.InputSpec{
    Type:   "manual",
    Prefix: "Now playing: ",
}); err != nil {
    return err
}

if err := router.AddOutput(output, &core.OutputSpec{
    Type:           "webhook",
    Inputs:         []string{input.GetName()},
    Formatters:     []core.Formatter{&formatters.UcwordsFormatter{}},
    FormatterNames: []string{"ucwords"},
    Timing: core.OutputTiming{
        Delay:         2,
        FallbackDelay: 20,
    },
}); err != nil {
    return err
}
```

`AddOutput` rejects missing, duplicate, or unknown inputs and negative delays. The router copies its specs, so later slice changes do not reconfigure it. Calling `AddInput` or `AddOutput` after `Start` panics.

## Reference implementations

Prefer a nearby production implementation over inventing a new pattern:

| Need | Implementation |
| --- | --- |
| Passive input and expiration | [`inputs/dynamic.go`](inputs/dynamic.go) |
| Polling input and response parsing | [`inputs/url.go`](inputs/url.go) |
| Simple file output | [`outputs/file.go`](outputs/file.go) |
| Outbound HTTP output | [`outputs/url.go`](outputs/url.go) |
| HTTP routes and atomic state | [`outputs/http.go`](outputs/http.go) |
| WebSocket output | [`outputs/websocket.go`](outputs/websocket.go) |
| Field-aware truncation | [`formatters/rds.go`](formatters/rds.go) |
| Configurable filters | [`filters/registry.go`](filters/registry.go) |

## Verification

Run the repository checks after adding an extension:

```bash
go test ./...
go vet ./...
go build
golangci-lint run --timeout=5m
```

For a manual smoke test, copy `config-example.json`, add the component, start the binary with `./zwfm-metadata -config test-config.json`, and inspect the dashboard at <http://localhost:9000>.
