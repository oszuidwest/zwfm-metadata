# Extending ZWFM Metadata

This guide covers how to add new inputs, outputs, and formatters to the ZWFM metadata system. The system uses a clean interface-based architecture that makes extending functionality straightforward.

## Architecture Overview

The ZWFM metadata system consists of three main extension points:

- **Inputs** - Source metadata from various systems (APIs, files, static text)
- **Outputs** - Send formatted metadata to destinations (streaming servers, files, webhooks)  
- **Formatters** - Transform metadata text (uppercase, lowercase, RDS compliance, etc.)

All components communicate through the central `Manager` which handles priority fallback, scheduling, and change detection.

## Adding a New Input

Inputs implement the `core.Input` interface and typically embed `core.BaseInput` for common functionality.

### 1. Create Input Structure

```go
package inputs

import (
    "context"
    "zwfm-metadata/config"
    "zwfm-metadata/core"
)

// MyCustomInput handles custom input source
type MyCustomInput struct {
    *core.BaseInput
    core.WaitForShutdown  // For passive inputs
    settings config.MyCustomInputSettings
}

// NewMyCustomInput creates a new custom input
func NewMyCustomInput(name string, settings config.MyCustomInputSettings) *MyCustomInput {
    return &MyCustomInput{
        BaseInput: core.NewBaseInput(name),
        settings:  settings,
    }
}
```

### 2. Implement Required Methods

For **passive inputs** (like Dynamic/Text that wait for external updates):

```go
// Start implements the Input interface (WaitForShutdown provides this)
// No additional implementation needed - just waits for shutdown

// UpdateMetadata updates metadata from external source
func (m *MyCustomInput) UpdateMetadata(title, artist string) error {
    metadata := &core.Metadata{
        Name:      m.GetName(),
        Title:     title,
        Artist:    artist,
        UpdatedAt: time.Now(),
        // Set ExpiresAt if needed
    }
    
    m.SetMetadata(metadata)
    return nil
}
```

For **active inputs** (like URL input that polls external sources):

```go
// Start implements the Input interface  
func (m *MyCustomInput) Start(ctx context.Context) error {
    ticker := time.NewTicker(time.Duration(m.settings.PollingInterval) * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            if err := m.fetchAndUpdate(); err != nil {
                utils.LogError("Failed to fetch data: %v", err)
            }
        }
    }
}

func (m *MyCustomInput) fetchAndUpdate() error {
    // Fetch data from external source
    title, artist := m.fetchFromSource()
    
    metadata := &core.Metadata{
        Name:      m.GetName(),
        Title:     title,
        Artist:    artist,
        UpdatedAt: time.Now(),
    }
    
    m.SetMetadata(metadata)
    return nil
}
```

### 3. Add Configuration Support

Add settings struct to `config/config.go`:

```go
// MyCustomInputSettings represents settings for custom input
type MyCustomInputSettings struct {
    APIKey          string `json:"apiKey"`
    PollingInterval int    `json:"pollingInterval"`
    CustomParam     string `json:"customParam"`
}
```

### 4. Register Input in Main Application

In `main.go`, add a case for your new input type in the `createInput` function:

```go
// createInput creates an input based on configuration
func createInput(cfg config.InputConfig) (core.Input, error) {
    switch cfg.Type {
    case "mycustom":
        settings, err := utils.ParseJSONSettings[config.MyCustomInputSettings](cfg.Settings)
        if err != nil {
            return nil, err
        }
        return inputs.NewMyCustomInput(cfg.Name, *settings), nil
    
    default:
        return nil, &unknownTypeError{Type: cfg.Type}
    }
}
```

The main application will automatically handle:
- Adding the input to the manager
- Setting the input type for status display
- Configuring prefix/suffix if specified
- Starting the input with proper context

### 5. Usage Example

```json
{
  "type": "mycustom",
  "name": "my-source",
  "prefix": "Custom: ",
  "suffix": " 🎵",
  "settings": {
    "apiKey": "secret123",
    "pollingInterval": 30,
    "customParam": "value"
  }
}
```

## Adding a New Output

Outputs implement the `core.Output` interface and typically embed `core.BaseOutput` for common functionality.

### 1. Create Output Structure

```go
package outputs

import (
    "context"
    "zwfm-metadata/config"
    "zwfm-metadata/core"
)

// MyCustomOutput handles custom output destination
type MyCustomOutput struct {
    *core.BaseOutput
    core.WaitForShutdown
    settings config.MyCustomOutputSettings
}

// NewMyCustomOutput creates a new custom output
func NewMyCustomOutput(name string, settings config.MyCustomOutputSettings) *MyCustomOutput {
    return &MyCustomOutput{
        BaseOutput: core.NewBaseOutput(name),
        settings:   settings,
    }
}
```

### 2. Implement Required Methods

```go
// GetDelay implements the Output interface
func (m *MyCustomOutput) GetDelay() int {
    return m.settings.Delay
}

// ProcessFormattedMetadata implements the Output interface
func (m *MyCustomOutput) ProcessFormattedMetadata(formattedText string) {
    // Check if value changed to avoid unnecessary operations
    if !m.HasChanged(formattedText) {
        return
    }
    
    // Send to your custom destination
    if err := m.sendToDestination(formattedText); err != nil {
        utils.LogError("Failed to send to custom output %s: %v", m.GetName(), err)
    }
}

func (m *MyCustomOutput) sendToDestination(metadata string) error {
    // Implement your custom sending logic
    utils.LogDebug("Sent to custom output %s: %s", m.GetName(), metadata)
    return nil
}
```

### 3. Enhanced Output (Optional)

If your output needs access to full metadata details (not just formatted text), implement `core.EnhancedOutput`:

```go
// ProcessEnhancedMetadata implements the EnhancedOutput interface
func (m *MyCustomOutput) ProcessEnhancedMetadata(formattedText string, metadata *core.Metadata) {
    if !m.HasChanged(formattedText) {
        return
    }
    
    // Send with additional metadata fields
    payload := CustomPayload{
        FormattedText: formattedText,
        Title:         metadata.Title,
        Artist:        metadata.Artist,
        UpdatedAt:     metadata.UpdatedAt,
        ExpiresAt:     metadata.ExpiresAt,
    }
    
    if err := m.sendCustomPayload(payload); err != nil {
        utils.LogError("Failed to send enhanced payload: %v", err)
    }
}
```

### 4. Add Configuration Support

Add settings struct to `config/config.go`:

```go
// MyCustomOutputSettings represents settings for custom output
type MyCustomOutputSettings struct {
    Delay          int                    `json:"delay"`
    Endpoint       string                 `json:"endpoint"`
    APIKey         string                 `json:"apiKey"`
    PayloadMapping map[string]interface{} `json:"payloadMapping,omitempty"`
}
```

### 5. Register Output in Main Application

In `main.go`, add a case for your new output type in the `createOutput` function:

```go
// createOutput creates an output based on configuration
func createOutput(cfg config.OutputConfig) (core.Output, error) {
    switch cfg.Type {
    case "mycustom":
        settings, err := utils.ParseJSONSettings[config.MyCustomOutputSettings](cfg.Settings)
        if err != nil {
            return nil, err
        }
        return outputs.NewMyCustomOutput(cfg.Name, *settings), nil
    
    default:
        return nil, &unknownTypeError{Type: cfg.Type}
    }
}
```

The main application will automatically handle:
- Setting inputs for the output
- Registering input mappings with the timeline manager
- Registering formatters with the timeline manager
- Adding the output to the manager
- Setting the output type for status display

### 6. Usage Example

```json
{
  "type": "mycustom",
  "name": "my-destination",
  "inputs": ["radio-live", "fallback"],
  "formatters": ["ucwords"],
  "settings": {
    "delay": 2,
    "endpoint": "https://api.example.com/metadata",
    "apiKey": "secret123"
  }
}
```

#### Custom Payload Mapping (POST Output)

The POST output supports custom payload mapping to transform the internal metadata format to match any API structure:

```json
{
  "type": "post",
  "name": "custom-api",
  "inputs": ["radio-live", "fallback"],
  "formatters": ["ucwords"],
  "settings": {
    "delay": 0,
    "url": "http://localhost:8080/track",
    "bearerToken": "your_secret_api_key_here",
    "payloadMapping": {
      "item": {
        "title": "title",
        "artist": "artist"
      },
      "expires_at": "expires_at"
    }
  }
}
```

This transforms the default payload structure:
```json
{
  "formatted_metadata": "Artist - Title",
  "songID": "12345",
  "title": "Title",
  "artist": "Artist",
  "duration": "3:45",
  "updated_at": "2023-12-01T15:30:00Z",
  "expires_at": "2023-12-01T15:33:00Z"
}
```

Into your custom structure:
```json
{
  "item": {
    "title": "Title",
    "artist": "Artist"
  },
  "expires_at": "2023-12-01T15:33:00Z"
}
```

Available fields for mapping:
- `formatted_metadata` - The formatted text after applying formatters
- `songID` - Song identifier
- `title` - Song title
- `artist` - Artist name
- `duration` - Song duration
- `updated_at` - When the metadata was updated
- `expires_at` - When the metadata expires (null if no expiration)

## Adding a New Formatter

Formatters implement the simple `formatters.Formatter` interface.

### 1. Create Formatter Structure

```go
package formatters

import "strings"

// MyCustomFormatter applies custom text transformation
type MyCustomFormatter struct{}

// Format implements the Formatter interface
func (m *MyCustomFormatter) Format(text string) string {
    // Apply your custom transformation
    return m.customTransform(text)
}

func (m *MyCustomFormatter) customTransform(text string) string {
    // Example: Replace special characters
    text = strings.ReplaceAll(text, "&", "and")
    text = strings.ReplaceAll(text, "@", "at")
    return text
}

func init() {
    RegisterFormatter("mycustom", func() Formatter { return &MyCustomFormatter{} })
}
```

### 2. Register Formatter

Add an `init()` function to your formatter file to register it:

```go
func init() {
    RegisterFormatter("mycustom", func() Formatter { return &MyCustomFormatter{} })
}
```

This will automatically register your formatter when the package is imported.

### 3. Usage Example

```json
{
  "type": "icecast",
  "name": "main-stream",
  "inputs": ["radio-live"],
  "formatters": ["mycustom", "ucwords"],
  "settings": {
    "delay": 2,
    "server": "localhost",
    "port": 8000,
    "username": "source",
    "password": "hackme",
    "mountpoint": "/stream"
  }
}
```

## Interface Reference

### core.Input Interface

```go
type Input interface {
    Start(ctx context.Context) error          // Start processing
    GetName() string                          // Return input name
    GetMetadata() *Metadata                   // Get current metadata
    Subscribe(ch chan<- *Metadata)            // Subscribe to updates
    Unsubscribe(ch chan<- *Metadata)          // Unsubscribe from updates
}
```

### core.Output Interface

```go
type Output interface {
    Start(ctx context.Context) error                    // Start processing
    GetName() string                                    // Return output name
    GetDelay() int                                      // Return delay in seconds
    SetInputs(inputs []Input)                           // Set input list
    ProcessFormattedMetadata(formattedText string)      // Process metadata
}
```

### core.EnhancedOutput Interface

```go
type EnhancedOutput interface {
    Output
    ProcessEnhancedMetadata(formattedText string, metadata *Metadata)
}
```

### formatters.Formatter Interface

```go
type Formatter interface {
    Format(text string) string    // Transform text
}
```

## Key Design Patterns

### 1. Embedding BaseInput/BaseOutput
Always embed `core.BaseInput` or `core.BaseOutput` to get common functionality like subscription management and change detection.

### 2. WaitForShutdown for Passive Components
Use `core.WaitForShutdown` for components that don't need background tasks (most outputs, dynamic/text inputs).

### 3. Change Detection
Outputs should call `HasChanged()` to avoid unnecessary operations when metadata hasn't changed.

### 4. Error Handling
Use `utils.LogError()` and `utils.LogDebug()` for consistent logging across the system.

### 5. Thread Safety
The base classes handle thread safety. Avoid direct field access in custom implementations.

### 6. Configuration Validation
Always validate configuration in constructors and return meaningful errors.

## Testing Your Extensions

1. **Unit Tests**: Test your components in isolation
2. **Integration Tests**: Test with the full Manager
3. **Configuration Tests**: Verify JSON configuration parsing
4. **Error Handling**: Test failure scenarios

Example test structure:

```go
func TestMyCustomInput(t *testing.T) {
    settings := config.MyCustomInputSettings{
        APIKey: "test123",
        PollingInterval: 30,
    }
    
    input := NewMyCustomInput("test-input", settings)
    
    // Test metadata update
    err := input.UpdateMetadata("Test Title", "Test Artist")
    assert.NoError(t, err)
    
    // Verify metadata
    metadata := input.GetMetadata()
    assert.Equal(t, "Test Title", metadata.Title)
    assert.Equal(t, "Test Artist", metadata.Artist)
}
```

## Best Practices

1. **Naming**: Use descriptive names that indicate the component's purpose
2. **Configuration**: Make settings configurable rather than hardcoded
3. **Logging**: Use appropriate log levels (Debug for verbose, Error for failures)
4. **Resource Management**: Clean up HTTP clients, file handles, etc.
5. **Graceful Degradation**: Handle failures gracefully without crashing the system
6. **Documentation**: Document your extension's configuration options and behavior

## Complete Example: Redis Input

Here's a complete example implementing a Redis input that polls a Redis key:

```go
// inputs/redis.go
package inputs

import (
    "context"
    "time"
    "github.com/go-redis/redis/v8"
    "zwfm-metadata/config"
    "zwfm-metadata/core"
    "zwfm-metadata/utils"
)

type RedisInput struct {
    *core.BaseInput
    settings config.RedisInputSettings
    client   *redis.Client
}

func NewRedisInput(name string, settings config.RedisInputSettings) *RedisInput {
    client := redis.NewClient(&redis.Options{
        Addr: settings.Address,
        DB:   settings.Database,
    })
    
    return &RedisInput{
        BaseInput: core.NewBaseInput(name),
        settings:  settings,
        client:    client,
    }
}

func (r *RedisInput) Start(ctx context.Context) error {
    ticker := time.NewTicker(time.Duration(r.settings.PollingInterval) * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            r.client.Close()
            return nil
        case <-ticker.C:
            if err := r.fetchFromRedis(); err != nil {
                utils.LogError("Failed to fetch from Redis: %v", err)
            }
        }
    }
}

func (r *RedisInput) fetchFromRedis() error {
    ctx := context.Background()
    
    // Get metadata from Redis key
    result, err := r.client.Get(ctx, r.settings.Key).Result()
    if err != nil {
        if err == redis.Nil {
            // Key doesn't exist - clear metadata
            r.SetMetadata(nil)
            return nil
        }
        return err
    }
    
    // Parse JSON or use as title
    var title, artist string
    if r.settings.JSONParsing {
        // Parse JSON to extract fields
        title, artist = r.parseJSON(result)
    } else {
        title = result
    }
    
    metadata := &core.Metadata{
        Name:      r.GetName(),
        Title:     title,
        Artist:    artist,
        UpdatedAt: time.Now(),
    }
    
    r.SetMetadata(metadata)
    return nil
}
```

Configuration:

```go
// config/config.go
type RedisInputSettings struct {
    Address         string `json:"address"`
    Database        int    `json:"database"`
    Key             string `json:"key"`
    PollingInterval int    `json:"pollingInterval"`
    JSONParsing     bool   `json:"jsonParsing"`
}
```

Usage:

```json
{
  "type": "redis",
  "name": "redis-nowplaying",
  "settings": {
    "address": "localhost:6379",
    "database": 0,
    "key": "nowplaying",
    "pollingInterval": 5,
    "jsonParsing": true
  }
}
```

This example demonstrates all the key concepts for creating a robust, production-ready input extension.