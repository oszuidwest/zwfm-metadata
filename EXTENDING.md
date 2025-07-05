# Extending ZuidWest FM Metadata

This guide covers how to add new inputs, outputs, and formatters to the ZuidWest FM metadata system. The system uses a clean interface-based architecture that makes extending functionality straightforward.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Before You Begin](#before-you-begin)
  - [Available Utilities](#available-utilities)
  - [Common Gotchas](#common-gotchas)
- [Adding a New Input](#adding-a-new-input)
  - [Input Types](#input-types)
  - [Step 1: Create Input Structure](#step-1-create-input-structure)
  - [Step 2: Implement Required Methods](#step-2-implement-required-methods)
  - [Step 3: Add Configuration Support](#step-3-add-configuration-support)
  - [Step 4: Register Input](#step-4-register-input)
  - [Step 5: Test Your Input](#step-5-test-your-input)
- [Adding a New Output](#adding-a-new-output)
  - [Output Types](#output-types)
  - [Step 1: Create Output Structure](#step-1-create-output-structure)
  - [Step 2: Implement Required Methods](#step-2-implement-required-methods-1)
  - [Step 3: Enhanced Output (Optional)](#step-3-enhanced-output-optional)
  - [Step 4: Add Configuration Support](#step-4-add-configuration-support)
  - [Step 5: Register Output](#step-5-register-output)
  - [Step 6: Test Your Output](#step-6-test-your-output)
- [Adding a New Formatter](#adding-a-new-formatter)
  - [Step 1: Create Formatter Structure](#step-1-create-formatter-structure)
  - [Step 2: Register Formatter](#step-2-register-formatter)
  - [Step 3: Test Your Formatter](#step-3-test-your-formatter)
- [Complete Examples](#complete-examples)
  - [Example: Redis Input](#example-redis-input)
  - [Example: Discord Output](#example-discord-output)
  - [Example: Sanitize Formatter](#example-sanitize-formatter)
- [Interface Reference](#interface-reference)
  - [core.Input Interface](#coreinput-interface)
  - [core.Output Interface](#coreoutput-interface)
  - [core.EnhancedOutput Interface](#coreenhancedoutput-interface)
  - [formatters.Formatter Interface](#formattersformatter-interface)
- [Design Patterns](#design-patterns)
  - [Base Class Embedding](#base-class-embedding)
  - [PassiveComponent](#passivecomponent)
  - [Change Detection](#change-detection)
  - [Universal Metadata Converter](#universal-metadata-converter)
  - [Error Handling](#error-handling)
  - [Thread Safety](#thread-safety)
- [Testing](#testing)
  - [Creating Test Configuration](#creating-test-configuration)
  - [Running Tests](#running-tests)
  - [Debugging Tips](#debugging-tips)
- [Best Practices](#best-practices)

## Architecture Overview

The ZuidWest FM metadata system consists of three main extension points:

- **Inputs** - Source metadata from various systems (APIs, files, static text)
- **Outputs** - Send formatted metadata to destinations (streaming servers, files, webhooks)  
- **Formatters** - Transform metadata text (uppercase, lowercase, RDS compliance, etc.)

All components communicate through the central `MetadataRouter` which handles:
- Priority-based fallback between inputs
- Scheduling updates with configurable delays
- Change detection to avoid duplicate updates
- Thread-safe subscription management

## Before You Begin

### Available Utilities

The codebase provides several utilities you can use:

- **Logging**: Use `log/slog` package (NOT utils.LogError/LogDebug)
  ```go
  import "log/slog"
  
  slog.Debug("Debug message", "key", "value")
  slog.Info("Info message", "key", "value")
  slog.Error("Error message", "error", err)
  ```

- **JSON Parsing**: `utils.ParseJSONSettings` for configuration parsing
  ```go
  settings, err := utils.ParseJSONSettings[YourConfigType](cfg.Settings)
  ```

- **Universal Metadata Converter**: `utils.ConvertMetadata` for consistent metadata handling
  ```go
  import "zwfm-metadata/utils"
  
  // Convert core.Metadata to universal format
  universal := utils.ConvertMetadata(formattedText, metadata)
  
  // Convert with a specific type field
  universal := utils.ConvertMetadataWithType(formattedText, metadata, "webhook")
  
  // Convert to template data for payload mapping
  templateData := universal.ToTemplateData()
  ```

### Common Gotchas

1. **Logging**: Use `slog` package directly, not `utils.LogError()` or `utils.LogDebug()`
2. **Error Handling**: Outputs should log errors but never return them from Send methods
3. **Formatter Registration**: Must use `init()` function to register formatters
4. **Imports**: Use full import paths like `zwfm-metadata/config`, not just `config`
5. **Build and Test**: Remember to `go build` before testing your extensions

## Adding a New Input

Inputs implement the `core.Input` interface and typically embed `core.InputBase` for common functionality.

### Input Types

- **Passive Inputs**: Wait for external updates (e.g., Dynamic, Text inputs)
- **Active Inputs**: Poll external sources periodically (e.g., URL input)

### Step 1: Create Input Structure

Create a new file in the `inputs/` directory:

```go
// inputs/myinput.go
package inputs

import (
    "context"
    "log/slog"
    "time"
    "zwfm-metadata/config"
    "zwfm-metadata/core"
)

// MyCustomInput handles custom input source
type MyCustomInput struct {
    *core.InputBase
    core.PassiveComponent  // For passive inputs only
    settings config.MyCustomInputConfig
}

// NewMyCustomInput creates a new custom input
func NewMyCustomInput(name string, settings config.MyCustomInputConfig) *MyCustomInput {
    return &MyCustomInput{
        InputBase: core.NewInputBase(name),
        settings:  settings,
    }
}
```

### Step 2: Implement Required Methods

#### For Passive Inputs

```go
// Start implements the Input interface (PassiveComponent provides empty implementation)
// No additional implementation needed for passive inputs

// UpdateMetadata updates metadata from external source (called by your API endpoint)
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

#### For Active Inputs

```go
// Start implements the Input interface  
func (m *MyCustomInput) Start(ctx context.Context) error {
    // Initial fetch
    if err := m.fetchAndUpdate(); err != nil {
        slog.Error("Initial fetch failed", "error", err)
    }

    ticker := time.NewTicker(time.Duration(m.settings.PollingInterval) * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            if err := m.fetchAndUpdate(); err != nil {
                slog.Error("Failed to fetch data", "input", m.GetName(), "error", err)
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

### Step 3: Add Configuration Support

Add your configuration struct to `config/config.go`:

```go
// MyCustomInputConfig represents settings for custom input
type MyCustomInputConfig struct {
    APIKey          string `json:"apiKey"`
    PollingInterval int    `json:"pollingInterval"`
    CustomParam     string `json:"customParam"`
}
```

### Step 4: Register Input

In `main.go`, add a case for your new input type in the `createInput` function:

```go
case "mycustom":
    settings, err := utils.ParseJSONSettings[config.MyCustomInputConfig](cfg.Settings)
    if err != nil {
        return nil, err
    }
    return inputs.NewMyCustomInput(cfg.Name, *settings), nil
```

### Step 5: Test Your Input

Create a test configuration:

```json
{
  "inputs": [
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
  ]
}
```

Build and run:
```bash
go build
./zwfm-metadata -config test-config.json
```

## Adding a New Output

Outputs implement the `core.Output` interface and typically embed `core.OutputBase` for common functionality.

### Output Types

- **Basic Outputs**: Receive only formatted text
- **Enhanced Outputs**: Receive full metadata details (implement `core.EnhancedOutput`)

### Step 1: Create Output Structure

Create a new file in the `outputs/` directory:

```go
// outputs/myoutput.go
package outputs

import (
    "context"
    "log/slog"
    "zwfm-metadata/config"
    "zwfm-metadata/core"
)

// MyCustomOutput handles custom output destination
type MyCustomOutput struct {
    *core.OutputBase
    core.PassiveComponent  // Most outputs are passive
    settings config.MyCustomOutputConfig
}

// NewMyCustomOutput creates a new custom output
func NewMyCustomOutput(name string, settings config.MyCustomOutputConfig) *MyCustomOutput {
    return &MyCustomOutput{
        OutputBase: core.NewOutputBase(name),
        settings:   settings,
    }
}
```

### Step 2: Implement Required Methods

```go
// GetDelay implements the Output interface
func (m *MyCustomOutput) GetDelay() int {
    return m.settings.Delay
}

// SendFormattedMetadata implements the Output interface
func (m *MyCustomOutput) SendFormattedMetadata(formattedText string) {
    // IMPORTANT: Check if value changed to avoid unnecessary operations
    if !m.HasChanged(formattedText) {
        return
    }
    
    // Send to your custom destination
    if err := m.sendToDestination(formattedText); err != nil {
        // IMPORTANT: Log error but don't return it
        slog.Error("Failed to send to custom output", "output", m.GetName(), "error", err)
    }
}

func (m *MyCustomOutput) sendToDestination(metadata string) error {
    // Implement your custom sending logic
    slog.Debug("Sent to custom output", "output", m.GetName(), "metadata", metadata)
    return nil
}
```

### Step 3: Enhanced Output (Optional)

If your output needs access to full metadata details:

```go
// SendEnhancedMetadata implements the EnhancedOutput interface
func (m *MyCustomOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata) {
    if !m.HasChanged(formattedText) {
        return
    }
    
    // Use universal metadata converter for consistent field mapping
    universal := utils.ConvertMetadataWithType(formattedText, metadata, "custom")
    
    if err := m.sendUniversalPayload(*universal); err != nil {
        slog.Error("Failed to send enhanced payload", "output", m.GetName(), "error", err)
    }
}

func (m *MyCustomOutput) sendUniversalPayload(metadata utils.UniversalMetadata) error {
    // Access all metadata fields without manual mapping
    payload := map[string]interface{}{
        "type":         metadata.Type,
        "title":        metadata.Title,
        "artist":       metadata.Artist,
        "duration":     metadata.Duration,
        "updated_at":   metadata.UpdatedAt,
        "expires_at":   metadata.ExpiresAt,
        "formatted":    metadata.FormattedMetadata,
    }
    
    // Send payload...
    return nil
}
```

### Step 4: Add Configuration Support

Add your configuration struct to `config/config.go`:

```go
// MyCustomOutputConfig represents settings for custom output
type MyCustomOutputConfig struct {
    Delay          int                    `json:"delay"`
    Endpoint       string                 `json:"endpoint"`
    APIKey         string                 `json:"apiKey"`
    PayloadMapping map[string]interface{} `json:"payloadMapping,omitempty"`
}
```

### Step 5: Register Output

In `main.go`, add a case for your new output type in the `createOutput` function:

```go
case "mycustom":
    settings, err := utils.ParseJSONSettings[config.MyCustomOutputConfig](cfg.Settings)
    if err != nil {
        return nil, err
    }
    return outputs.NewMyCustomOutput(cfg.Name, *settings), nil
```

### Step 6: Test Your Output

Create a test configuration:

```json
{
  "outputs": [
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
  ]
}
```

## Adding a New Formatter

Formatters implement the simple `formatters.Formatter` interface.

### Step 1: Create Formatter Structure

Create a new file in the `formatters/` directory:

```go
// formatters/myformatter.go
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
```

### Step 2: Register Formatter

Add an `init()` function to register your formatter:

```go
func init() {
    RegisterFormatter("mycustom", func() Formatter { 
        return &MyCustomFormatter{} 
    })
}
```

### Step 3: Test Your Formatter

Use in configuration:

```json
{
  "outputs": [
    {
      "type": "file",
      "name": "formatted-output",
      "inputs": ["radio-live"],
      "formatters": ["mycustom", "ucwords"],
      "settings": {
        "delay": 0,
        "filename": "/tmp/formatted.txt"
      }
    }
  ]
}
```

## Complete Examples

### Example: Redis Input

A complete Redis input that polls a Redis key for metadata:

```go
// inputs/redis.go
package inputs

import (
    "context"
    "encoding/json"
    "log/slog"
    "time"
    
    "github.com/go-redis/redis/v8"
    "zwfm-metadata/config"
    "zwfm-metadata/core"
)

type RedisInput struct {
    *core.InputBase
    settings config.RedisInputConfig
    client   *redis.Client
}

func NewRedisInput(name string, settings config.RedisInputConfig) *RedisInput {
    client := redis.NewClient(&redis.Options{
        Addr:     settings.Address,
        Password: settings.Password,
        DB:       settings.Database,
    })
    
    return &RedisInput{
        InputBase: core.NewInputBase(name),
        settings:  settings,
        client:    client,
    }
}

func (r *RedisInput) Start(ctx context.Context) error {
    // Initial fetch
    if err := r.fetchFromRedis(); err != nil {
        slog.Error("Initial Redis fetch failed", "error", err)
    }

    ticker := time.NewTicker(time.Duration(r.settings.PollingInterval) * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            r.client.Close()
            return nil
        case <-ticker.C:
            if err := r.fetchFromRedis(); err != nil {
                slog.Error("Failed to fetch from Redis", "error", err)
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
        var data map[string]string
        if err := json.Unmarshal([]byte(result), &data); err != nil {
            return err
        }
        title = data["title"]
        artist = data["artist"]
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
    slog.Debug("Updated from Redis", "key", r.settings.Key, "title", title)
    return nil
}
```

Configuration (`config/config.go`):
```go
type RedisInputConfig struct {
    Address         string `json:"address"`
    Password        string `json:"password,omitempty"`
    Database        int    `json:"database"`
    Key             string `json:"key"`
    PollingInterval int    `json:"pollingInterval"`
    JSONParsing     bool   `json:"jsonParsing"`
}
```

Registration (`main.go`):
```go
case "redis":
    settings, err := utils.ParseJSONSettings[config.RedisInputConfig](cfg.Settings)
    if err != nil {
        return nil, err
    }
    return inputs.NewRedisInput(cfg.Name, *settings), nil
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

### Example: Discord Output

A Discord webhook output with enhanced metadata support:

```go
// outputs/discord.go
package outputs

import (
    "bytes"
    "encoding/json"
    "fmt"
    "log/slog"
    "net/http"
    "time"
    
    "zwfm-metadata/config"
    "zwfm-metadata/core"
)

type DiscordOutput struct {
    *core.OutputBase
    core.PassiveComponent
    settings   config.DiscordOutputConfig
    httpClient *http.Client
}

func NewDiscordOutput(name string, settings config.DiscordOutputConfig) *DiscordOutput {
    return &DiscordOutput{
        OutputBase: core.NewOutputBase(name),
        settings:   settings,
        httpClient: &http.Client{Timeout: 10 * time.Second},
    }
}

func (d *DiscordOutput) GetDelay() int {
    return d.settings.Delay
}

func (d *DiscordOutput) SendFormattedMetadata(formattedText string) {
    if !d.HasChanged(formattedText) {
        return
    }
    
    embed := map[string]interface{}{
        "title":       "Now Playing",
        "description": formattedText,
        "color":       0x00ff00,
        "timestamp":   time.Now().Format(time.RFC3339),
    }
    
    if err := d.sendWebhook(embed); err != nil {
        slog.Error("Failed to send to Discord", "error", err)
    }
}

func (d *DiscordOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata) {
    if !d.HasChanged(formattedText) {
        return
    }
    
    // Use universal converter for consistent metadata handling
    universal := utils.ConvertMetadataWithType(formattedText, metadata, "discord")
    
    fields := []map[string]interface{}{}
    
    if universal.Artist != "" {
        fields = append(fields, map[string]interface{}{
            "name":   "Artist",
            "value":  universal.Artist,
            "inline": true,
        })
    }
    
    if universal.Title != "" {
        fields = append(fields, map[string]interface{}{
            "name":   "Title",
            "value":  universal.Title,
            "inline": true,
        })
    }
    
    if universal.Duration != "" {
        fields = append(fields, map[string]interface{}{
            "name":   "Duration",
            "value":  universal.Duration,
            "inline": true,
        })
    }
    
    embed := map[string]interface{}{
        "title":       "🎵 Now Playing",
        "description": universal.FormattedMetadata,
        "color":       0x00ff00,
        "fields":      fields,
        "timestamp":   universal.UpdatedAt.Format(time.RFC3339),
    }
    
    if err := d.sendWebhook(embed); err != nil {
        slog.Error("Failed to send to Discord", "error", err)
    }
}

func (d *DiscordOutput) sendWebhook(embed map[string]interface{}) error {
    payload := map[string]interface{}{
        "embeds": []map[string]interface{}{embed},
    }
    
    jsonData, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    
    req, err := http.NewRequest("POST", d.settings.WebhookURL, bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }
    
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("User-Agent", utils.UserAgent())
    
    resp, err := d.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 400 {
        return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
    }
    
    return nil
}
```

### Example: Sanitize Formatter

A formatter that removes profanity and inappropriate content:

```go
// formatters/sanitize.go
package formatters

import (
    "regexp"
    "strings"
)

type SanitizeFormatter struct {
    badWords []string
    regex    *regexp.Regexp
}

func NewSanitizeFormatter() *SanitizeFormatter {
    badWords := []string{
        "explicit1", "explicit2", // Add actual words to filter
    }
    
    // Create regex pattern
    pattern := "(?i)\\b(" + strings.Join(badWords, "|") + ")\\b"
    regex := regexp.MustCompile(pattern)
    
    return &SanitizeFormatter{
        badWords: badWords,
        regex:    regex,
    }
}

func (s *SanitizeFormatter) Format(text string) string {
    // Replace bad words with asterisks
    return s.regex.ReplaceAllStringFunc(text, func(match string) string {
        return strings.Repeat("*", len(match))
    })
}

func init() {
    RegisterFormatter("sanitize", func() Formatter {
        return NewSanitizeFormatter()
    })
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
    SendFormattedMetadata(formattedText string)         // Process metadata
}
```

### core.EnhancedOutput Interface

```go
type EnhancedOutput interface {
    Output
    SendEnhancedMetadata(formattedText string, metadata *Metadata)
}
```

### formatters.Formatter Interface

```go
type Formatter interface {
    Format(text string) string    // Transform text
}
```

## Design Patterns

### Base Class Embedding

Always embed `core.InputBase` or `core.OutputBase` to get common functionality:

```go
type MyInput struct {
    *core.InputBase  // Provides subscription management, metadata storage
    // your fields...
}
```

### PassiveComponent

Use `core.PassiveComponent` for components that don't need background tasks:

```go
type MyOutput struct {
    *core.OutputBase
    core.PassiveComponent  // Provides empty Start() implementation
}
```

This is typically used for:
- Outputs that only react to metadata updates
- Inputs that wait for external triggers (like API calls)

### Change Detection

Outputs should always use `HasChanged()` to avoid unnecessary operations:

```go
func (o *MyOutput) SendFormattedMetadata(formattedText string) {
    if !o.HasChanged(formattedText) {
        return  // Skip if metadata hasn't changed
    }
    // Process the update...
}
```

### Universal Metadata Converter

Use `utils.ConvertMetadata` instead of manually mapping fields from `core.Metadata`. This ensures consistency across all outputs and makes maintenance easier:

```go
import "zwfm-metadata/utils"

// Instead of manual field mapping:
// payload := CustomPayload{
//     FormattedMetadata: formattedText,
//     SongID:            metadata.SongID,
//     Title:             metadata.Title,
//     Artist:            metadata.Artist,
//     Duration:          metadata.Duration,
//     UpdatedAt:         metadata.UpdatedAt,
//     ExpiresAt:         metadata.ExpiresAt,
// }

// Use the universal converter:
func (o *MyOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata) {
    if !o.HasChanged(formattedText) {
        return
    }
    
    // Convert to universal format
    universal := utils.ConvertMetadata(formattedText, metadata)
    
    // Or with a type field:
    universal := utils.ConvertMetadataWithType(formattedText, metadata, "myoutput")
    
    // Send the universal metadata
    o.sendMetadata(*universal)
}

// For payload mapping (templates):
func (o *MyOutput) processWithMapping(universal utils.UniversalMetadata) {
    templateData := universal.ToTemplateData()
    result := o.payloadMapper.MapPayload(templateData)
    // Use result...
}
```

**Benefits:**
- **Consistency**: All outputs use the same metadata structure
- **Maintainability**: Adding new metadata fields only requires changes in one place
- **DRY Principle**: No duplicate field mapping code
- **Template Compatibility**: Built-in `ToTemplateData()` method for payload mapping

### Error Handling

1. **Inputs**: Can return errors from Start(), should log errors during operation
2. **Outputs**: Should NEVER return errors from Send methods, only log them
3. **Formatters**: Should handle errors gracefully and return valid text
4. **Metadata Conversion**: Use `utils.ConvertMetadata` instead of manual field mapping

```go
// Good - Output error handling
func (o *MyOutput) SendFormattedMetadata(text string) {
    if err := o.send(text); err != nil {
        slog.Error("Send failed", "error", err)  // Log but don't return
    }
}

// Bad - Don't do this in outputs
func (o *MyOutput) SendFormattedMetadata(text string) error {
    return o.send(text)  // DON'T return errors!
}
```

### HTTP Requests and User-Agent

When making HTTP requests in inputs or outputs, always set a proper User-Agent header:

```go
import "zwfm-metadata/utils"

// In your HTTP request code:
req, err := http.NewRequest("POST", url, body)
if err != nil {
    return err
}

// Set headers
req.Header.Set("Content-Type", "application/json")
req.Header.Set("User-Agent", utils.UserAgent())  // Returns "zwfm-metadata/{version}"
```

This ensures:
- Proper identification of requests in server logs
- Compliance with API best practices
- Version tracking for debugging

### Thread Safety

The base classes handle thread safety for:
- Metadata storage and retrieval
- Subscription management
- Change detection

Your code should:
- Use the provided SetMetadata/GetMetadata methods
- Use `utils.ConvertMetadata` for consistent metadata handling
- Not directly access shared state
- Use mutexes for any additional shared state you add

## Testing

### Creating Test Configuration

Create a minimal test configuration:

```json
{
  "webServerPort": 9000,
  "debug": true,
  "stationName": "Test Station",
  "inputs": [
    {
      "type": "yourcustominput",
      "name": "test-input",
      "settings": {
        "yourSetting": "value"
      }
    },
    {
      "type": "text",
      "name": "fallback",
      "settings": {
        "text": "No data"
      }
    }
  ],
  "outputs": [
    {
      "type": "yourcustomoutput",
      "name": "test-output",
      "inputs": ["test-input", "fallback"],
      "formatters": ["yourcustomformatter"],
      "settings": {
        "delay": 0,
        "yourSetting": "value"
      }
    }
  ]
}
```

### Running Tests

```bash
# Build the project
go build

# Run with test configuration
./zwfm-metadata -config test-config.json

# Check the dashboard
open http://localhost:9000

# Test dynamic input via API
curl "http://localhost:9000/input/dynamic?input=test-input&title=Test&artist=Artist"
```

### Debugging Tips

1. **Enable Debug Logging**: Set `"debug": true` in config
2. **Check Dashboard**: View real-time status at http://localhost:9000
3. **Add Debug Logs**: Use `slog.Debug()` liberally during development
4. **Test Incrementally**: Test each component separately before combining

## Best Practices

1. **Naming**: Use descriptive names that indicate the component's purpose
2. **Configuration**: Make settings configurable rather than hardcoded
3. **Logging**: Use appropriate log levels:
   - `slog.Debug()` - Detailed information for debugging
   - `slog.Info()` - Important events (startup, shutdown)
   - `slog.Error()` - Errors that don't stop operation
4. **Resource Management**: Always clean up in Start() method:
   ```go
   defer client.Close()
   defer ticker.Stop()
   ```
5. **Graceful Degradation**: Handle failures without crashing
6. **Documentation**: Comment your configuration struct fields
7. **Validation**: Validate configuration in constructors:
   ```go
   if settings.URL == "" {
       return nil, fmt.Errorf("URL is required")
   }
   ```
8. **Universal Metadata**: Use `utils.ConvertMetadata` instead of manual field mapping
9. **Payload Mapping**: Use `universal.ToTemplateData()` for template processing

This guide should help you create robust extensions for the ZuidWest FM metadata system. Happy coding!