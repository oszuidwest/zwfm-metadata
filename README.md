# ZuidWest FM Metadata

Metadata routing middleware for radio stations. Routes metadata from inputs (playout software, APIs, static text) to outputs (Icecast, files, webhooks) with priority-based fallback and configurable delays. Originally designed for ZuidWest FM in the Netherlands.

<img width="1755" alt="Scherm­afbeelding 2025-07-05 om 00 20 05" src="https://github.com/user-attachments/assets/98fd12cb-d74e-4ca2-8224-c13d4f50b397" />


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

**Global Settings:**
- `webServerPort` (default: 9000) - Port for the web dashboard and API
- `debug` (default: false) - Enable debug logging
- `stationName` (default: "ZuidWest FM") - Station name displayed in dashboard and browser title
- `brandColor` (default: "#e6007e") - Brand color (hex) used throughout the dashboard UI

The dashboard automatically adapts to your brand colors, using them for headers, badges, and accent elements throughout the interface.

## Inputs

**Dynamic Input** - HTTP API for live updates
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
**Settings:**
- `secret` (optional) - Authentication secret for API calls
- `expiration.type` - `"dynamic"` (expires based on song duration), `"fixed"` (expires after set minutes), `"none"` (never expires)
- `expiration.minutes` (required if type=fixed) - Minutes until expiration

**API Usage:**
```bash
# Update with all fields (duration enables auto-expiration for type=dynamic)
curl "http://localhost:9000/input/dynamic?input=radio-live&songID=123&artist=Artist&title=Song&duration=3:45&secret=supersecret123"

# Minimal update (only title required)
curl "http://localhost:9000/input/dynamic?input=radio-live&title=Song&secret=supersecret123"
```

**Parameters:**
- `input` (required) - Input name from config
- `title` (required) - Song/track title  
- `songID` (optional) - Unique song identifier
- `artist` (optional) - Artist name
- `duration` (optional) - Song duration (MM:SS or HH:MM:SS format, leading zeros optional: `3:45` or `03:45`)
- `secret` (required if configured) - Authentication secret

**URL Input** - Poll external APIs
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
**Settings:**
- `url` (required) - URL to poll for metadata
- `pollingInterval` (required) - Seconds between HTTP requests
- `jsonParsing` (optional, default: false) - Parse response as JSON
- `jsonKey` (required if jsonParsing=true) - Dot notation path to extract value (e.g., `"data.song.title"`)

**Text Input** - Static fallback
```json
{
  "type": "text",
  "name": "default-text",
  "settings": {
    "text": "Welcome to ZuidWest FM!"
  }
}
```
**Settings:**
- `text` (required) - Static text to display

**Input Options (all types):**
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
| **DLS Plus** | DAB/DAB+ radio text | ✅ | ❌ | N/A |
| **WebSocket** | Real-time browser/app updates | ✅ | ✅ | N/A |

**Legend:**
- **Enhanced Metadata**: Receives full metadata details (title, artist, duration, etc.) not just formatted text
- **Custom Payload Mapping**: Supports transforming output to match any JSON structure
- **Authentication**: Security mechanism supported

### Output Configurations

**Icecast Output** - Update streaming server metadata
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
**Settings:**
- `delay` (required) - Seconds to delay metadata updates
- `server` (required) - Icecast server hostname/IP
- `port` (required) - Icecast server port
- `username` (required) - Icecast username (usually "source")
- `password` (required) - Icecast password
- `mountpoint` (required) - Stream mountpoint (e.g., "/stream")

**File Output** - Write to filesystem
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
**Settings:**
- `delay` (required) - Seconds to delay metadata updates
- `filename` (required) - Full path to output file

**POST Output** - Send metadata via HTTP webhooks
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
**Settings:**
- `delay` (required) - Seconds to delay metadata updates
- `url` (required) - Webhook endpoint URL
- `bearerToken` (optional) - Authorization bearer token
- `payloadMapping` (optional) - Custom JSON payload structure (see [Custom Payload Mapping](#custom-payload-mapping))
- `omitEmpty` (optional, default: false) - Omit empty fields from custom payload

**WebSocket Output** - Broadcast metadata to connected clients
```json
{
  "type": "websocket",
  "name": "websocket-server",
  "inputs": ["radio-live", "nowplaying-api", "default-text"],
  "formatters": ["ucwords"],
  "settings": {
    "delay": 0,
    "address": ":8080",
    "path": "/metadata",
    "payloadMapping": {...}  // See Custom Payload Mapping section
  }
}
```
**Settings:**
- `delay` (required) - Seconds to delay metadata updates
- `address` (required) - Address to bind the WebSocket server (e.g., ":8080", "localhost:8080")
- `path` (required) - URL path for WebSocket connections (e.g., "/metadata", "/ws")
- `payloadMapping` (optional) - Custom JSON message structure (see [Custom Payload Mapping](#custom-payload-mapping))

**Client Connection Example (JavaScript):**
```javascript
const ws = new WebSocket('ws://localhost:8080/metadata');

ws.onmessage = (event) => {
  const metadata = JSON.parse(event.data);
  console.log('Metadata update:', metadata);
  // Update your UI with the new metadata
};

ws.onopen = () => {
  console.log('Connected to metadata WebSocket');
  // You'll immediately receive the current metadata as a "welcome" message
};
```

**DLS Plus Output** - Generate DLS Plus format for DAB/DAB+ transmission
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
**Settings:**
- `delay` (required) - Seconds to delay metadata updates
- `filename` (required) - Full path to output file

**Output Format:**
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

**Output Options (all types):**
- `inputs` (required) - Array of input names in priority order
- `formatters` (optional) - Array of formatter names to apply

### Custom Payload Mapping

Both **POST** and **WebSocket** outputs support custom payload mapping to transform the output format to match any JSON structure your API expects.

**How it works:**
- **Static values**: Any string without `{{}}` is used as-is
- **Field references**: Use `{{.fieldname}}` to reference metadata fields
- **Nested objects**: Create complex JSON structures with nested mappings
- **Mixed templates**: Combine static text with fields, e.g., `"Now playing: {{.title}}"`

**Available fields for mapping:**
- `{{.formatted_metadata}}` - The formatted text after applying formatters
- `{{.songID}}` - Song identifier
- `{{.title}}` - Song title
- `{{.artist}}` - Artist name
- `{{.duration}}` - Song duration
- `{{.updated_at}}` - When the metadata was updated (RFC3339 format)
- `{{.expires_at}}` - When the metadata expires (RFC3339 format, empty if no expiration)
- `{{.type}}` - Message type (WebSocket only: "metadata_update" or "welcome")

**Default Payloads (when payloadMapping is not specified):**

POST Output:
```json
{
  "formatted_metadata": "Artist - Title",
  "songID": "12345",
  "title": "Title",
  "artist": "Artist",
  "duration": "3:45",
  "updated_at": "2023-12-01T15:30:00Z",
  "expires_at": "2023-12-01T15:33:45Z"
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
  "expires_at": "2023-12-01T15:33:45Z"
}
```

**Example: Custom API Format**
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

**Example: Simple Format**
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

**Example: Mixed Static and Dynamic Content**
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

**POST Output Specific:** The `omitEmpty` option (default: false) removes empty fields from the output:
```json
{
  "settings": {
    "omitEmpty": true,
    "payloadMapping": {
      "title": "{{.title}}",
      "artist": "{{.artist}}",
      "album": "{{.album}}"  // This field doesn't exist
    }
  }
}
```

With `omitEmpty: true`, if artist is empty, the output would be:
```json
{
  "title": "Imagine"
}
```

## Formatters

Apply text transformations to metadata before sending to outputs.

**Available Formatters:**

**`uppercase`** - Convert to uppercase
```
"Artist - Song Title" → "ARTIST - SONG TITLE"
```

**`lowercase`** - Convert to lowercase  
```
"Artist - Song Title" → "artist - song title"
```

**`ucwords`** - Convert to title case
```
"artist - song title" → "Artist - Song Title"
```

**`rds`** - Radio Data System (64 character limit)
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


**Usage:**
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

**Update metadata:**
```bash
curl "http://localhost:9000/input/dynamic?input=radio-live&title=Song&artist=Artist&duration=3:45"
```

**Status:**
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

## License
MIT
