# ZuidWest FM Metadata

Metadata routing middleware for radio stations. Routes metadata from inputs (playout software, APIs, static text) to outputs (Icecast, files, webhooks) with priority-based fallback and configurable delays. Originally designed for ZuidWest FM in the Netherlands.

<img width="1755" alt="Scherm­afbeelding 2025-07-05 om 00 20 05" src="https://github.com/user-attachments/assets/98fd12cb-d74e-4ca2-8224-c13d4f50b397" />

## Table of Contents

- [Quick Start](#quick-start)
- [Configuration](#configuration)
  - [Global Settings](#global-settings)
- [Inputs](#inputs)
  - [Dynamic Input](#dynamic-input)
  - [URL Input](#url-input)
  - [Text Input](#text-input)
  - [Input Options](#input-options)
- [Outputs](#outputs)
  - [Output Feature Comparison](#output-feature-comparison)
  - [Output Configurations](#output-configurations)
    - [Icecast Output](#icecast-output)
    - [File Output](#file-output)
    - [POST Output](#post-output)
    - [HTTP Output](#http-output)
    - [WebSocket Output](#websocket-output)
    - [DLS Plus Output](#dls-plus-output)
    - [StereoTool Output](#stereotool-output)
  - [Custom Payload Mapping](#custom-payload-mapping)
- [Formatters](#formatters)
  - [Available Formatters](#available-formatters)
- [Features](#features)
- [API](#api)
- [Development](#development)
- [License](#license)

## Quick Start

```bash
go build
cp config-example.json config.json
# Edit config.json
./zwfm-metadata
```

Dashboard: http://localhost:9000

## Configuration

The application supports basic styling customization through the configuration file:

```json
{
  "webServerPort": 9000,
  "debug": false,
  "stationName": "Your Radio Station",
  "brandColor": "#e6007e",
  "inputs": [...],
  "outputs": [...]
}
```

### Global Settings

- `webServerPort` (default: 9000) - Port for the web dashboard and API
- `debug` (default: false) - Enable debug logging
- `stationName` (default: "ZuidWest FM") - Station name displayed in dashboard and browser title
- `brandColor` (default: "#e6007e") - Brand color (hex) used throughout the dashboard UI

The dashboard automatically adapts to your brand colors, using them for headers, badges, and accent elements throughout the interface.

## Inputs

Configure metadata sources in priority order.

### Input Feature Comparison

| Input Type | Purpose | Update Method | Authentication | Expiration Support | Polling |
|------------|---------|---------------|----------------|--------------------|---------|
| **Dynamic** | Live playout updates | HTTP API | Secret | ✅ (Dynamic/Fixed/None) | ❌ |
| **URL** | External API integration | HTTP polling | N/A | ✅ (via JSON field) | ✅ |
| **Text** | Static fallback | Config file | N/A | ❌ | ❌ |

**Legend:**
- **Expiration Support**: Whether metadata can automatically expire (Dynamic: based on song duration, Fixed: after set time, None: never expires)
- **Polling**: Whether the input actively polls for updates vs receiving them via API

### Dynamic Input

HTTP API for live updates

```json
{
  "type": "dynamic",
  "name": "radio-live",
  "prefix": "🎵 Now Playing: ",
  "suffix": " 🎵",
  "settings": {
    "secret": "supersecret123",
    "expiration": {
      "type": "dynamic"
    }
  }
}
```

#### Settings
- `secret` (optional) - Authentication secret for API calls
- `expiration.type` - `"dynamic"` (expires based on song duration), `"fixed"` (expires after set minutes), `"none"` (never expires)
- `expiration.minutes` (required if type=fixed) - Minutes until expiration

#### API Usage
```bash
# Update with all fields (duration enables auto-expiration for type=dynamic)
curl "http://localhost:9000/input/dynamic?input=radio-live&songID=123&artist=Artist&title=Song&duration=3:45&secret=supersecret123"

# Minimal update (only title required)
curl "http://localhost:9000/input/dynamic?input=radio-live&title=Song&secret=supersecret123"
```

#### Parameters
- `input` (required) - Input name from config
- `title` (required) - Song/track title
- `songID` (optional) - Unique song identifier
- `artist` (optional) - Artist name
- `duration` (optional) - Song duration (MM:SS or HH:MM:SS format, leading zeros optional: `3:45` or `03:45`)
- `secret` (required if configured) - Authentication secret

### URL Input

Poll external APIs

```json
{
  "type": "url",
  "name": "nowplaying-api",
  "prefix": "From API: ",
  "suffix": " [Live]",
  "settings": {
    "url": "https://api.example.com/nowplaying",
    "jsonParsing": true,
    "jsonKey": "data.current.title",
    "pollingInterval": 30
  }
}
```

#### Settings
- `url` (required) - URL to poll for metadata
- `pollingInterval` (required) - Seconds between HTTP requests
- `jsonParsing` (optional, default: false) - Parse response as JSON
- `jsonKey` (required if jsonParsing=true) - Dot notation path to extract value (e.g., `"data.song.title"`)
- `expiryKey`: (optional) Dot-separated path to the expiry value in the JSON response. If set, the expiry will be parsed and used for metadata expiration. When the expiry is reached, polling will occur immediately, in addition to regular interval polling.
- `expiryFormat`: (optional) Format string for parsing the expiry value (e.g., RFC3339). Defaults to RFC3339 if not specified.

### Text Input

Static fallback

```json
{
  "type": "text",
  "name": "default-text",
  "settings": {
    "text": "Welcome to ZuidWest FM!"
  }
}
```

#### Settings
- `text` (required) - Static text to display

### Input Options

All input types support:
- `prefix` (optional) - Text added before metadata
- `suffix` (optional) - Text added after metadata

## Outputs

Control where formatted metadata is sent.

### Output Feature Comparison

| Output Type | Purpose | Enhanced Metadata | Custom Payload Mapping | Authentication |
|-------------|---------|-------------------|------------------------|----------------|
| **Icecast** | Update streaming server metadata | ❌ | ❌ | Basic Auth |
| **File** | Write to local filesystem | ❌ | ❌ | N/A |
| **POST** | Send via HTTP webhooks | ✅ | ✅ | Bearer Token |
| **HTTP** | Serve metadata via GET endpoints | ✅ | ✅ | N/A |
| **DLS Plus** | DAB/DAB+ radio text | ✅ | ❌ | N/A |
| **WebSocket** | Real-time browser/app updates | ✅ | ✅ | N/A |
| **StereoTool** | Update RDS RadioText | ❌ | ❌ | N/A |

**Legend:**
- **Enhanced Metadata**: Receives full metadata details (title, artist, duration, etc.) not just formatted text
- **Custom Payload Mapping**: Supports transforming output to match any JSON structure
- **Authentication**: Security mechanism supported

### Output Configurations

All output types support:
- `inputs` (required) - Array of input names in priority order
- `formatters` (optional) - Array of formatter names to apply

#### Icecast Output

Update streaming server metadata

```json
{
  "type": "icecast",
  "name": "main-stream",
  "inputs": ["radio-live", "nowplaying-api", "default-text"],
  "formatters": ["ucwords"],
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

##### Settings
- `delay` (required) - Seconds to delay metadata updates
- `server` (required) - Icecast server hostname/IP
- `port` (required) - Icecast server port
- `username` (required) - Icecast username (usually "source")
- `password` (required) - Icecast password
- `mountpoint` (required) - Stream mountpoint (e.g., "/stream")

#### File Output

Write to filesystem

```json
{
  "type": "file",
  "name": "nowplaying-file",
  "inputs": ["radio-live", "default-text"],
  "formatters": ["uppercase"],
  "settings": {
    "delay": 0,
    "filename": "/tmp/nowplaying.txt"
  }
}
```

##### Settings
- `delay` (required) - Seconds to delay metadata updates
- `filename` (required) - Full path to output file

#### POST Output

Send metadata via HTTP webhooks

```json
{
  "type": "post",
  "name": "webhook",
  "inputs": ["radio-live", "nowplaying-api", "default-text"],
  "formatters": ["ucwords"],
  "settings": {
    "delay": 1,
    "url": "https://api.example.com/metadata",
    "bearerToken": "your-bearer-token-here",
    "payloadMapping": {...}  // See Custom Payload Mapping section
  }
}
```

##### Settings
- `delay` (required) - Seconds to delay metadata updates
- `url` (required) - Webhook endpoint URL
- `bearerToken` (optional) - Authorization bearer token
- `payloadMapping` (optional) - Custom JSON payload structure (see [Custom Payload Mapping](#custom-payload-mapping))

#### HTTP Output

Serve metadata via GET endpoints with multiple response formats

```json
{
  "type": "http",
  "name": "metadata-api",
  "inputs": ["radio-live", "nowplaying-api", "default-text"],
  "formatters": ["ucwords"],
  "settings": {
    "delay": 0,
    "endpoints": [
      {
        "path": "/api/nowplaying.json",
        "responseType": "json"
      },
      {
        "path": "/api/current.txt",
        "responseType": "plaintext"
      },
      {
        "path": "/api/custom.json",
        "responseType": "json",
        "payloadMapping": {
          "station": "My Radio",
          "track": "{{.title}}",
          "artist": "{{.artist}}"
        }
      }
    ]
  }
}
```

##### Settings
- `delay` (required) - Seconds to delay metadata updates
- `endpoints` (required) - Array of HTTP endpoints to serve

##### Endpoint Configuration
- `path` (required) - URL path for the endpoint
- `responseType` (optional) - Response format: `json` (default), `xml`, `yaml`, or `plaintext`
- `payloadMapping` (optional) - Custom response structure (see [Custom Payload Mapping](#custom-payload-mapping))

##### Response Types
- **JSON**: Standard metadata object with all fields
- **XML**: XML with escaped content
- **YAML**: YAML format for configuration files
- **Plaintext**: Just the formatted metadata text

#### WebSocket Output

Broadcast metadata to connected clients

```json
{
  "type": "websocket",
  "name": "websocket-server",
  "inputs": ["radio-live", "nowplaying-api", "default-text"],
  "formatters": ["ucwords"],
  "settings": {
    "delay": 0,
    "path": "/metadata",
    "payloadMapping": {...}  // See Custom Payload Mapping section
  }
}
```

##### Settings
- `delay` (required) - Seconds to delay metadata updates
- `path` (required) - URL path for WebSocket connections (e.g., "/metadata", "/ws")
- `payloadMapping` (optional) - Custom JSON message structure (see [Custom Payload Mapping](#custom-payload-mapping))

**Note**: WebSocket endpoints are served on the main web server port (default: 9000), not on a separate port.

##### Client Connection Example

JavaScript:
```javascript
const ws = new WebSocket('ws://localhost:9000/metadata');

ws.onmessage = (event) => {
  const metadata = JSON.parse(event.data);
  console.log('Metadata update:', metadata);
  // Update your UI with the new metadata
};

ws.onopen = () => {
  console.log('Connected to metadata WebSocket');
  // You'll immediately receive the current metadata
};
```

#### DLS Plus Output

Generate DLS Plus format for DAB/DAB+ transmission

```json
{
  "type": "dlsplus",
  "name": "dlsplus-output",
  "inputs": ["radio-live", "nowplaying-api", "default-text"],
  "formatters": [],
  "settings": {
    "delay": 0,
    "filename": "/tmp/dlsplus.txt"
  }
}
```

##### Settings
- `delay` (required) - Seconds to delay metadata updates
- `filename` (required) - Full path to output file

##### Output Format
Generates ODR-PadEnc compatible DLS Plus files with parameter blocks:
```
##### parameters { #####
DL_PLUS=1
DL_PLUS_TAG=4 0 5
DL_PLUS_TAG=1 9 9
##### parameters } #####
Artist - Song Title
```

The output automatically:
- Detects artist and title positions in the formatted text
- Generates correct DL_PLUS_TAG entries (type 4 for ARTIST, type 1 for TITLE)
- Handles prefixes and suffixes correctly
- Works with any formatters applied to the text

Note: ODR-PadEnc automatically re-reads DLS files before each transmission.

#### StereoTool Output

Update StereoTool's RDS RadioText

```json
{
  "type": "stereotool",
  "name": "stereotool-rds",
  "inputs": ["radio-live", "nowplaying-api", "default-text"],
  "formatters": ["rds"],
  "settings": {
    "delay": 2,
    "hostname": "localhost",
    "port": 8080
  }
}
```

##### Settings
- `delay` (required) - Seconds to delay metadata updates
- `hostname` (required) - StereoTool server hostname/IP
- `port` (required) - StereoTool HTTP server port (typically 8080)

##### Notes
- Uses StereoTool's undocumented JSON API to update RadioText (ID: 9985)
- Recommended to use with RDS formatter for 64-character compliance

### Custom Payload Mapping

Both **POST** and **WebSocket** outputs support custom payload mapping to transform the output format to match any JSON structure your API expects.

#### How it works
- **Static values**: Any string without `{{}}` is used as-is
- **Field references**: Use `{{.fieldname}}` to reference metadata fields
- **Nested objects**: Create complex JSON structures with nested mappings
- **Mixed templates**: Combine static text with fields, e.g., `"Now playing: {{.title}}"`

#### Available fields
- `{{.formatted_metadata}}` - The formatted text after applying formatters
- `{{.songID}}` - Song identifier
- `{{.title}}` - Song title
- `{{.artist}}` - Artist name
- `{{.duration}}` - Song duration
- `{{.updated_at}}` - When the metadata was updated (RFC3339 format)
- `{{.expires_at}}` - When the metadata expires (RFC3339 format, empty if no expiration)
- `{{.type}}` - Message type (WebSocket only: "metadata_update")
- `{{.source}}` - Name of the input that provided this metadata
- `{{.source_type}}` - Type of the input (e.g., "dynamic", "url", "text")

#### Default Payloads

When `payloadMapping` is not specified:

POST Output:
```json
{
  "formatted_metadata": "Artist - Title",
  "songID": "12345",
  "title": "Title",
  "artist": "Artist",
  "duration": "3:45",
  "updated_at": "2023-12-01T15:30:00Z",
  "expires_at": "2023-12-01T15:33:45Z",
  "source": "radio-live",
  "source_type": "dynamic"
}
```

WebSocket Output:
```json
{
  "type": "metadata_update",
  "formatted_metadata": "Artist - Title",
  "songID": "12345",
  "title": "Title",
  "artist": "Artist",
  "duration": "3:45",
  "updated_at": "2023-12-01T15:30:00Z",
  "expires_at": "2023-12-01T15:33:45Z",
  "source": "radio-live",
  "source_type": "dynamic"
}
```

#### Examples

##### Custom API Format
```json
{
  "payloadMapping": {
    "event": "{{.type}}",
    "station": "My Radio Station",
    "now_playing": {
      "song": "{{.title}}",
      "artist": "{{.artist}}",
      "full_text": "{{.formatted_metadata}}"
    },
    "metadata": {
      "song_id": "{{.songID}}",
      "duration": "{{.duration}}",
      "timestamp": "{{.updated_at}}",
      "expires": "{{.expires_at}}"
    }
  }
}
```

Output:
```json
{
  "event": "metadata_update",
  "station": "My Radio Station",
  "now_playing": {
    "song": "Imagine",
    "artist": "John Lennon",
    "full_text": "John Lennon - Imagine"
  },
  "metadata": {
    "song_id": "12345",
    "duration": "3:04",
    "timestamp": "2023-12-01T15:30:00Z",
    "expires": "2023-12-01T15:33:04Z"
  }
}
```

##### Simple Format
```json
{
  "payloadMapping": {
    "title": "{{.title}}",
    "artist": "{{.artist}}",
    "timestamp": "{{.updated_at}}"
  }
}
```

Output:
```json
{
  "title": "Imagine",
  "artist": "John Lennon",
  "timestamp": "2023-12-01T15:30:00Z"
}
```

##### Mixed Static and Dynamic Content
```json
{
  "payloadMapping": {
    "message": "Now playing: {{.title}} by {{.artist}}",
    "station_info": {
      "name": "My Radio Station",
      "frequency": "101.5 FM",
      "region": "Amsterdam"
    },
    "expires_at": "{{.expires_at}}"
  }
}
```

##### Source Tracking
```json
{
  "payloadMapping": {
    "title": "{{.title}}",
    "artist": "{{.artist}}",
    "metadata_source": {
      "input_name": "{{.source}}",
      "input_type": "{{.source_type}}",
      "reliability": "{{if eq .source_type \"dynamic\"}}live{{else if eq .source_type \"url\"}}automated{{else}}static{{end}}"
    }
  }
}
```

## Formatters

Apply text transformations to metadata before sending to outputs.

### Available Formatters

#### uppercase
Convert to uppercase
```
"Artist - Song Title" → "ARTIST - SONG TITLE"
```

#### lowercase
Convert to lowercase
```
"Artist - Song Title" → "artist - song title"
```

#### ucwords
Convert to title case
```
"artist - song title" → "Artist - Song Title"
```

#### rds
Radio Data System (64 character limit)
```
"<b>Very&shy;Long Artist Name</b> feat. Someone - Very Long Song Title (Extended Remix Version)"
→ "VeryLong Artist Name - Very Long Song Title"
```

Smart processing for RDS compliance:
- **HTML cleaning**: Strips all HTML tags (`<b>`, `<i>`, `<span>`, `<script>`) and decodes entities (`&amp;` → `&`, `&lt;` → `<`, `&quot;` → `"`, `&shy;` → soft hyphen Unicode, `&nbsp;` → non-breaking space)
- **Invisible characters**: Removes soft hyphens (`\u00AD`), zero-width spaces (`\u200B`, `\u200C`, `\u200D`), and other invisible Unicode characters
- **Single-line output**: Converts newlines (`\n`, `\r`) to spaces for RDS displays
- **Smart truncation** (applied in order until under 64 chars):
  1. Removes content in parentheses: `(Radio Edit)`, `(2024 Remaster)`
  2. Removes content in brackets: `[Live]`, `[Explicit]`
  3. Removes featured artists: `feat.`, `ft.`, `featuring`, `with`, `&`
  4. Removes remix indicators after second hyphen
  5. Removes common suffixes: `Remix`, `Mix`, `Edit`, `Version`, `Instrumental`, `Acoustic`, `Live`, `Remaster`
  6. Truncates at word boundaries with `...` if still too long

### Usage
```json
{
  "type": "icecast",
  "name": "main-stream",
  "formatters": ["ucwords", "rds"],
  "settings": {...}
}
```
Formatters are applied in order: `ucwords` first, then `rds`.

## Features

- **Priority fallback** - Outputs use first available input in priority list
- **Configurable delays** - Sync timing across different outputs
- **Input expiration** - Dynamic inputs expire automatically
- **Prefix/suffix** - Add station branding to inputs
- **Web dashboard** - Real-time status at http://localhost:9000

## API

### Update metadata
```bash
curl "http://localhost:9000/input/dynamic?input=radio-live&title=Song&artist=Artist&duration=3:45"
```

### Status
```bash
curl http://localhost:9000/status
```

## Development

```bash
go fmt ./...
go vet ./...
go build
```

Set `"debug": true` in config.json for detailed logging.

### Extending

See `EXTENDING.md` for detailed guidance on adding new inputs, outputs, and formatters. Key patterns:

- Use `utils.ConvertMetadata()` for consistent metadata handling across outputs
- Embed base types (`core.InputBase`, `core.OutputBase`) for common functionality
- Use `core.PassiveComponent` for components without background tasks

## License

MIT
