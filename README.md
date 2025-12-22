# ZuidWest FM Metadata

<p align="center">
  <img width="120" height="120" alt="apple-touch-icon" src="https://github.com/user-attachments/assets/04d9491c-b18e-4741-890d-847937109013" />
</p>
     
Metadata routing middleware for radio stations that routes metadata from inputs (playout software, APIs, and static text) to outputs (Icecast, files, and webhooks) with priority-based fallback and configurable delays. Originally designed for ZuidWest FM in the Netherlands.

<img width="1561" height="1406" alt="Scherm­afbeelding 2025-11-01 om 22 51 39" src="https://github.com/user-attachments/assets/b4ccc560-f509-47a1-9f3e-526819b01fec" />

## Table of Contents

- [Quick Start](#quick-start)
- [Configuration](#configuration)
  - [Global Settings](#global-settings)
- [Inputs](#inputs)
  - [Input Feature Comparison](#input-feature-comparison)
  - [Dynamic Input](#dynamic-input)
  - [URL Input](#url-input)
  - [Text Input](#text-input)
  - [Input Options](#input-options)
- [Outputs](#outputs)
  - [Output Feature Comparison](#output-feature-comparison)
  - [Output Configurations](#output-configurations)
    - [Icecast Output](#icecast-output)
    - [File Output](#file-output)
    - [URL Output](#url-output)
    - [HTTP Output](#http-output)
    - [WebSocket Output](#websocket-output)
    - [DL Plus Output](#dl-plus-output)
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

The application supports styling customization through the configuration file:

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
- `debug` (default: false) - Enables debug logging
- `stationName` (default: "ZuidWest FM") - Station name displayed in the dashboard and browser title
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

- **Expiration Support**: Whether metadata expires automatically (Dynamic: based on song duration, Fixed: after a set time, None: never expires)
- **Polling**: Whether the input polls for updates versus receives them via API

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
- `expiration.type` - `"dynamic"` (expires based on song duration), `"fixed"` (expires after a set number of minutes), or `"none"` (never expires)
- `expiration.minutes` (required if type=fixed, optional for type=dynamic) - Number of minutes until expiration. When `type` is `"dynamic"`, this serves as a fallback when the duration parameter is missing or invalid

#### API Usage
```bash
# Update with all fields (duration enables auto-expiration for type=dynamic)
curl "http://localhost:9000/input/dynamic?input=radio-live&songID=123&artist=Artist&title=Song&duration=03:45&secret=supersecret123"

# Minimal update (only title required)
curl "http://localhost:9000/input/dynamic?input=radio-live&title=Song&secret=supersecret123"

# PlayIt Live example (uses {{macros}} and seconds,microseconds format)
http://localhost:9000/input/dynamic?input=playit-live&title={{html-text}}&songID={{guid}}&duration={{duration}}&secret=supersecret123

# Aeron Studio example (uses <#Placeholders>)
http://localhost:9000/input/dynamic?input=aeron-studio&title=<#Title>&artist=<#Artist>&songID=<#titleid>&duration=<#Duration>&secret=supersecret123
```

#### Parameters
- `input` (required) - Input name from config
- `title` (required) - Song/track title
- `songID` (optional) - Unique song identifier
- `artist` (optional) - Artist name
- `duration` (optional) - Song duration in multiple formats. Leading zeros are optional. Used for auto-expiration when `expiration.type` is `"dynamic"`. Supported formats:
  - **Seconds**: `272` or `272.5` or `272,5` (whole or decimal seconds, comma or period separator)
  - **MM:SS**: `3:45` or `03:45` (minutes and seconds)
  - **HH:MM:SS**: `1:30:00` or `01:30:00` (hours, minutes, and seconds)
  - Invalid formats cause immediate expiration, or fallback to `expiration.minutes` if configured
- `secret` (required if configured) - Authentication secret

### URL Input

Polls external APIs

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
- `pollingInterval` (required) - Number of seconds between HTTP requests
- `jsonParsing` (optional, default: false) - Parses the response as JSON
- `jsonKey` (required if jsonParsing=true) - Dot-notation path to extract the value (e.g., `"data.song.title"`)
- `expiryKey` (optional) - Dot-notation path to the expiry value in the JSON response. When set, the expiry is parsed and used for metadata expiration. When the expiry is reached, polling occurs immediately in addition to regular interval polling.
- `expiryFormat` (optional) - Format string for parsing the expiry value (e.g., RFC3339). Defaults to RFC3339 if not specified.

### Text Input

A static fallback

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
- `prefix` (optional) - Text prepended to metadata
- `suffix` (optional) - Text appended to metadata

## Outputs

Control where formatted metadata is sent.

### Output Feature Comparison

| Output Type | Purpose | Template Support | Authentication |
|-------------|---------|------------------|----------------|
| **Icecast** | Update streaming server metadata | ❌ | Basic Auth |
| **File** | Write to local filesystem | ❌ | N/A |
| **URL** | Send via HTTP GET/POST | ✅ (GET in URL, POST via payloadMapping) | Bearer Token |
| **HTTP** | Serve metadata via GET endpoints | ✅ (via payloadMapping) | N/A |
| **DL Plus** | DAB/DAB+ radio text | ❌ | N/A |
| **WebSocket** | Real-time browser/app updates | ✅ (via payloadMapping) | N/A |
| **StereoTool** | Update RDS RadioText | ❌ | N/A |

All outputs receive full metadata (artist, title, duration, etc.) via StructuredText. The table shows additional capabilities:

- **Template Support**: Can use Go template functions to transform values (e.g., `{{.title | upper}}`, `{{printf "%.20s" .title}}`)
- **Authentication**: Security mechanisms supported

### Output Configurations

All output types support:
- `inputs` (required) - Array of input names in priority order
- `formatters` (optional) - Array of formatter names to apply

**Note on Templates**: Template functions (like `{{.title | upper}}`) are only available in outputs with template support (URL, HTTP, WebSocket). Other outputs use the formatted text directly and cannot use template syntax in their configuration.

#### Icecast Output

Updates streaming server metadata

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
- `delay` (required) - Number of seconds to delay metadata updates
- `server` (required) - Icecast server hostname/IP
- `port` (required) - Icecast server port
- `username` (required) - Icecast username (usually "source")
- `password` (required) - Icecast password
- `mountpoint` (required) - Stream mountpoint (e.g., "/stream")

#### File Output

Writes metadata to the filesystem.

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
- `delay` (required) - Number of seconds to delay metadata updates
- `filename` (required) - Full path to output file

**Note**: File output writes the formatted text as-is. To transform text, use formatters like `uppercase`, `lowercase`, `ucwords`, or `rds`. Template functions are not available for file outputs.

#### URL Output

Sends metadata via HTTP GET or POST requests. Supports both GET requests with URL templates and POST requests with JSON payloads.

##### POST Request Example
```json
{
  "type": "url",
  "name": "webhook",
  "inputs": ["radio-live", "nowplaying-api", "default-text"],
  "formatters": ["ucwords"],
  "settings": {
    "delay": 1,
    "url": "https://api.example.com/metadata",
    "method": "POST",
    "bearerToken": "your-bearer-token-here",
    "payloadMapping": {
      "station": "My Radio Station",
      "now_playing": {
        "title": "{{.title | upper}}",
        "artist": "{{.artist | lower}}",
        "text": "{{print .artist \" - \" .title}}",
        "duration_seconds": "{{.duration}}"
      },
      "metadata": {
        "has_artist": "{{if .artist}}true{{else}}false{{end}}",
        "truncated_title": "{{printf \"%.30s\" .title}}",
        "source": "{{.source}}"
      }
    }
  }
}
```

##### GET Request Example (TuneIn)
```json
{
  "type": "url",
  "name": "tunein",
  "inputs": ["radio-live"],
  "settings": {
    "method": "GET",
    "url": "http://air.radiotime.com/Playing.ashx?partnerId=YOUR_PARTNER_ID&partnerKey=YOUR_PARTNER_KEY&id=YOUR_STATION_ID&title={{.title}}&artist={{.artist}}",
    "delay": 0
  }
}
```

##### Settings
- `delay` (required) - Number of seconds to delay metadata updates
- `url` (required) - Target URL (supports Go templates for GET requests)
- `method` (required) - HTTP method: "GET" or "POST"
- `bearerToken` (optional) - Authorization bearer token
- `payloadMapping` (optional) - Custom JSON payload structure for POST requests (see [Custom Payload Mapping](#custom-payload-mapping))

##### HTTP Methods

**GET Requests:**
- Use Go template syntax in the URL (see [URL Templates](#url-templates-get-requests) below)
- Supports template functions for value transformation
- Ideal for services like TuneIn that expect metadata in the URL

**POST Requests:**
- Send JSON payload in the request body
- Support custom payload mapping with template syntax (see [Custom Payload Mapping](#custom-payload-mapping))
- Include bearer token authentication if configured
- No automatic URL encoding (values are in JSON body, not URL)

##### URL Templates (GET Requests)

When using GET requests, the URL output supports Go template syntax for dynamic URL construction. **All template values are automatically URL-encoded** for safety with special characters.

**Available Template Fields:**
- `{{.title}}` - Song/track title
- `{{.artist}}` - Artist name
- `{{.songID}}` - Unique song identifier
- `{{.duration}}` - Song duration
- `{{.formatted_metadata}}` - Complete formatted text after formatters
- `{{.source}}` - Name of the input providing this metadata
- `{{.source_type}}` - Type of input (e.g., "dynamic", "url", "text")
- `{{.updated_at}}` - Update timestamp (RFC3339)
- `{{.expires_at}}` - Expiration timestamp (RFC3339, empty if none)

**Template Functions:**

Custom formatting functions:
- `{{.title | upper}}` - Convert to uppercase
- `{{.title | lower}}` - Convert to lowercase
- `{{.title | trim}}` - Remove leading/trailing whitespace

Built-in Go template functions for formatting:
- `{{.title | urlquery}}` - URL-encode a value (automatic for GET requests)
- `{{.title | html}}` - HTML-escape special characters
- `{{.title | js}}` - JavaScript-escape for safe inclusion in JS strings
- `{{printf "%.20s" .title}}` - Format with printf (truncate to 20 chars)
- `{{print .artist " - " .title}}` - Concatenate multiple values
- `{{len .title}}` - Get length of a string

Conditional logic (built-in):
- `{{if .artist}}{{.artist}} - {{end}}{{.title}}` - Conditional inclusion
- `{{or .empty .title}}` - Use first non-empty value
- `{{and .artist .title}}` - Check if both values exist

**Automatic URL Encoding for GET Requests:**

For GET requests, all template values are automatically URL-encoded. This means special characters like `&`, `?`, `=`, `#` are handled safely without manual intervention:

- With `title: "Rock & Roll"` and `artist: "AC/DC"`
- Template: `?title={{.title}}&artist={{.artist}}`
- Result: `?title=Rock+%26+Roll&artist=AC%2FDC`

**Examples:**

Basic template:
```json
{
  "type": "url",
  "name": "metadata-api",
  "settings": {
    "method": "GET",
    "url": "http://api.example.com/update?title={{.title}}&artist={{.artist}}"
  }
}
```

Using template functions:
```json
{
  "url": "http://api.example.com/update?title={{.title | upper}}&artist={{.artist | lower}}"
}
```

Advanced template examples:
```json
{
  "url": "http://api.example.com/update?text={{print .artist \" - \" .title}}"
}
```

```json
{
  "url": "http://api.example.com/update?title={{printf \"%.30s\" .title}}"
}
```

```json
{
  "url": "http://api.example.com/update?{{if .artist}}artist={{.artist}}&{{end}}title={{.title}}"
}
```

Complex URL with path and query parameters:
```json
{
  "url": "http://api.example.com/station/{{.source}}/song/{{.songID}}?title={{.title}}&artist={{.artist}}&duration={{.duration}}"
}
```

Note: The `urlquery` function is available but typically not needed for GET requests due to automatic encoding. It may be useful in special cases like building encoded values for other purposes.

#### HTTP Output

Serves metadata via GET endpoints with multiple response formats

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
- `delay` (required) - Number of seconds to delay metadata updates
- `endpoints` (required) - Array of HTTP endpoints to serve

##### Endpoint Configuration
- `path` (required) - URL path for the endpoint
- `responseType` (optional) - Response format: `json` (default), `xml`, or `plaintext`
- `payloadMapping` (optional) - Custom response structure (see [Custom Payload Mapping](#custom-payload-mapping))

##### Response Types
- **JSON**: Standard metadata object with all fields
- **XML**: XML with escaped content
- **Plaintext**: Just the formatted metadata text

#### WebSocket Output

Broadcasts metadata to connected clients with real-time updates.

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
- `delay` (required) - Number of seconds to delay metadata updates
- `path` (required) - URL path for WebSocket connections (e.g., "/metadata", "/ws")
- `payloadMapping` (optional) - Custom JSON message structure (see [Custom Payload Mapping](#custom-payload-mapping))

**Note**: WebSocket endpoints are served on the main web server port (default: 9000), not on a separate port. All WebSocket operations use write timeouts to prevent hanging connections.

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

#### DL Plus Output

Generates DL Plus format for DAB/DAB+ transmission

```json
{
  "type": "dlplus",
  "name": "dlplus-output",
  "inputs": ["radio-live", "nowplaying-api", "default-text"],
  "formatters": ["ucwords"],
  "settings": {
    "delay": 0,
    "filename": "/tmp/dlplus.txt"
  }
}
```

##### Settings
- `delay` (required) - Number of seconds to delay metadata updates
- `filename` (required) - Full path to output file

##### Output Format
Generates ODR-PadEnc compatible DL Plus files with parameter blocks:
```
##### parameters { #####
DL_PLUS=1
DL_PLUS_TAG=4 0 5
DL_PLUS_TAG=1 9 9
DL_PLUS_ITEM_RUNNING=1
DL_PLUS_ITEM_TOGGLE=0
##### parameters } #####
Artist - Song Title
```

The output automatically:
- Calculates accurate artist and title positions from StructuredText field boundaries
- Generates correct DL_PLUS_TAG entries (type 4 for ARTIST, type 1 for TITLE)
- Sets DL_PLUS_ITEM_RUNNING=1 for tracks (with artist+title), 0 for station/program info
- Alternates DL_PLUS_ITEM_TOGGLE between 0 and 1 on each update to indicate content changes
- Preserves accurate positions even after formatters modify the text

Note: ODR-PadEnc automatically re-reads DL files before each transmission.

#### StereoTool Output

Updates StereoTool's RDS RadioText and Streaming Output Metadata

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
- `delay` (required) - Number of seconds to delay metadata updates
- `hostname` (required) - StereoTool server hostname/IP
- `port` (required) - StereoTool HTTP server port (typically 8080)

##### Notes
- Updates both FM RDS RadioText and Streaming Output Song
- Uses StereoTool's undocumented JSON API. Field ID's might be different in other versions
- Currently validated with StereoTool version 10.71
- **REQUIRED:** Must be used with the RDS formatter for proper character encoding
- **StereoTool Bug Workaround:** The RDS formatter converts all extended Latin characters (é, ø, ß, etc.) to pure ASCII as a temporary workaround for a bug in StereoTool's RDS implementation. While the EBU Latin character set (0x80-0xFF) should be valid for RDS, StereoTool doesn't handle these characters correctly. The formatter transliterates them (é→e, ø→o, ß→ss) to ensure compatibility until this bug is fixed

### Custom Payload Mapping

**URL** (POST method), **HTTP**, and **WebSocket** outputs support custom payload mapping to transform the output format to match any JSON structure that your API expects.

#### How It Works
- **Static values**: Any string without `{{}}` is used as-is
- **Field references**: Use `{{.fieldname}}` to reference metadata fields
- **Template functions**: Use pipe syntax for transformations, e.g., `{{.title | upper}}`
- **Nested objects**: Create complex JSON structures with nested mappings
- **Mixed templates**: Combine static text with fields, e.g., `"Now playing: {{.title}}"`

#### Available Fields
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

#### Available Template Functions

Custom functions (added for payload mapping):
- `{{.field | upper}}` - Convert to uppercase
- `{{.field | lower}}` - Convert to lowercase
- `{{.field | trim}}` - Remove leading/trailing whitespace
- `{{.field | formatTime}}` - Format time.Time to RFC3339 (rarely needed as times are pre-formatted)
- `{{.field | formatTimePtr}}` - Format *time.Time to RFC3339, returns empty string if nil

Built-in Go template functions (also available):
- `{{.field | urlquery}}` - URL-encode a value (useful if building URLs in JSON)
- `{{.field | html}}` - HTML-escape special characters
- `{{.field | js}}` - JavaScript-escape for safe inclusion in JS strings
- `{{printf "%.20s" .field}}` - Format with printf (e.g., truncate to 20 chars)
- `{{print .artist " - " .title}}` - Concatenate multiple values
- `{{len .field}}` - Get length of a string
- `{{if .artist}}{{.artist}} - {{end}}{{.title}}` - Conditional inclusion
- `{{or .empty .default}}` - Use first non-empty value

#### Default Payloads

When `payloadMapping` is not specified:

URL Output (POST method):
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

##### Using Template Functions
```json
{
  "payloadMapping": {
    "title_upper": "{{.title | upper}}",
    "artist_lower": "{{.artist | lower}}",
    "clean_text": "{{.formatted_metadata | trim}}",
    "display": "NOW PLAYING: {{.title | upper}} BY {{.artist | upper}}"
  }
}
```

Output:
```json
{
  "title_upper": "IMAGINE",
  "artist_lower": "john lennon",
  "clean_text": "John Lennon - Imagine",
  "display": "NOW PLAYING: IMAGINE BY JOHN LENNON"
}
```

## Formatters

Transform metadata fields (artist, title) before sending to outputs. Formatters operate on individual fields within StructuredText, preserving field boundaries for accurate position tracking.

**Formatters vs Templates**:
- **Formatters** apply to ALL output types and transform fields before output processing
- **Templates** (e.g., `{{.title | upper}}`) are only available in URL, HTTP, and WebSocket outputs
- Use formatters when you want the same transformation for all outputs, use templates for output-specific formatting

### Available Formatters

#### uppercase
Converts to uppercase
```
"Artist - Song Title" → "ARTIST - SONG TITLE"
```

#### lowercase
Converts to lowercase
```
"Artist - Song Title" → "artist - song title"
```

#### ucwords
Converts to title case
```
"artist - song title" → "Artist - Song Title"
```

#### rds
Radio Data System formatter (64-character limit)
```
"<b>Very Long Artist Name</b> feat. Someone - Very Long Song Title (Extended Remix Version)"
→ "Very Long Artist Name - Very Long Song Title"
```

Smart processing for RDS compliance:
- **HTML cleaning**: Strips all HTML tags (`<b>`, `<i>`, `<span>`, `<script>`) and decodes entities (`&amp;` → `&`, `&lt;` → `<`, `&quot;` → `"`, `&shy;` → soft hyphen, `&nbsp;` → non-breaking space)
- **ASCII transliteration**: Converts all extended Latin characters to pure ASCII (0-127 range) as a workaround for a StereoTool RDS bug. This is temporary until StereoTool properly supports the EBU Latin character set. Examples:
  - `BLØF` → `BLOF`
  - `Café` → `Cafe`
  - `Straße` → `Strasse`
  - And 150+ other European characters

- **Single-line output**: Converts newlines (`\n`, `\r`) and tabs (`\t`) to spaces for RDS displays
- **Smart truncation** (applied in order until under 64 chars):
  1. Progressively removes content in parentheses from right to left: `Artist - Song (Important Info) (Extended Mix)` → `Artist - Song (Important Info)`
  2. Progressively removes content in brackets from right to left: `Artist - Song [Live] [Remastered]` → `Artist - Song [Live]`
  3. Removes featured artists: `feat.`, `ft.`, `featuring`, `with`, `&`
  4. Removes remix indicators after second hyphen
  5. Removes common suffixes: `Remix`, `Mix`, `Edit`, `Version`, `Instrumental`, `Acoustic`, `Live`, `Remaster`
  6. Truncates at word boundaries with `...` if still too long

### Usage

Formatters can be applied individually or chained together in sequence. When multiple formatters are specified, each formatter modifies the StructuredText fields in sequence.

#### Single Formatter
```json
{
  "type": "icecast",
  "name": "main-stream",
  "formatters": ["rds"],
  "settings": {...}
}
```

#### Chained Formatters
```json
{
  "type": "icecast",
  "name": "main-stream",
  "formatters": ["ucwords", "rds"],
  "settings": {...}
}
```
Formatters are applied in order: `ucwords` first (converts to title case), then `rds` (applies 64-character limit and RDS compliance).

#### Chaining Examples

**Example 1: Title Case + RDS Compliance**
```
Input:    "john lennon - imagine (remastered)"
ucwords:  "John Lennon - Imagine (Remastered)"
rds:      "John Lennon - Imagine"
```

**Example 2: RDS Compliance + Uppercase**
```json
{
  "formatters": ["rds", "uppercase"]
}
```
```
Input:      "very long artist name feat. someone - very long song title (extended remix)"
rds:        "Very Long Artist Name - Very Long Song Title"
uppercase:  "VERY LONG ARTIST NAME - VERY LONG SONG TITLE"
```

**Example 3: Lowercase Only**
```json
{
  "formatters": ["lowercase"]
}
```
```
Input:     "Artist Name - Song Title"
lowercase: "artist name - song title"
```

**Note:** The order of formatters matters. For example, `["rds", "uppercase"]` first truncates to 64 characters, then converts to uppercase, while `["uppercase", "rds"]` first converts to uppercase, then truncates.

## Features

- **Priority fallback**: Outputs use the first available input in the priority list
- **Configurable delays**: Synchronizes timing across different outputs
- **Input expiration**: Dynamic inputs expire automatically
- **Prefix/suffix support**: Adds station branding to inputs
- **Web dashboard**: Real-time status at http://localhost:9000 with WebSocket updates and a connection status indicator

## API

### Update Metadata
```bash
curl "http://localhost:9000/input/dynamic?input=radio-live&title=Song&artist=Artist&duration=03:45"
```

## Development

```bash
go fmt ./...
go vet ./...
go build
```

Set `"debug": true` in `config.json` for detailed logging.

### Extending

See `EXTENDING.md` for detailed guidance on adding new inputs, outputs, and formatters. Key patterns include:

- All outputs receive `*core.StructuredText` with full metadata access
- Using `utils.ConvertStructuredText()` for JSON/webhook payloads
- Embedding base types (`core.InputBase`, `core.OutputBase`) for common functionality
- Using `core.PassiveComponent` for components without background tasks

## License

MIT
