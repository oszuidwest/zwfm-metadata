# ZWFM Metadata

Metadata routing middleware designed for ZuidWest FM and Radio Rucphen in the Netherlands. Routes metadata from inputs (playout software, APIs, static text) to outputs (Icecast, files, webhooks) with priority-based fallback and configurable delays.

## Quick Start

```bash
go build
cp config-example.json config.json
# Edit config.json
./zwfm-metadata
```

Dashboard: http://localhost:9000

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


**POST Output** - Complete metadata POST with bearer token
```json
{
  "type": "post",
  "name": "full-webhook",
  "inputs": ["radio-live", "nowplaying-api", "default-text"],
  "formatters": ["ucwords"],
  "settings": {
    "delay": 1,
    "url": "https://api.example.com/metadata",
    "bearerToken": "your-bearer-token-here"
  }
}
```
**Settings:**
- `delay` (required) - Seconds to delay metadata updates
- `url` (required) - Webhook endpoint URL
- `bearerToken` (optional) - Authorization bearer token
- `payloadMapping` (optional) - Custom JSON payload structure mapping
- `payloadMappingOmitEmpty` (optional, default: false) - Omit empty fields from custom payload (TODO: Remove when padenc-api supports empty fields)

**Default JSON Payload (when payloadMapping is not specified):**
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

**Custom Payload Mapping:**
Define any JSON structure by mapping internal fields to your API format. The `payloadMapping` supports both static values and dynamic field references using Go template syntax:

- **Static values**: Any string without `{{}}` is used as-is
- **Field references**: Use `{{.fieldname}}` to reference metadata fields
- **Mixed templates**: Combine static text with fields, e.g., `"Now playing: {{.title}}"`

```json
{
  "type": "post",
  "name": "custom-api",
  "inputs": ["radio-live", "nowplaying-api", "default-text"],
  "formatters": ["ucwords"],
  "settings": {
    "delay": 0,
    "url": "http://localhost:8080/track",
    "bearerToken": "your_secret_api_key_here",
    "payloadMapping": {
      "item": {
        "title": "{{.title}}",
        "artist": "{{.artist}}"
      },
      "expires_at": "{{.expires_at}}",
      "station": "ZWFM Radio"
    },
    "payloadMappingOmitEmpty": true
  }
}
```

This configuration produces:
```json
{
  "item": {
    "title": "Viva la Vida",
    "artist": "Coldplay"
  },
  "expires_at": "2023-12-31T23:59:59Z",
  "station": "ZWFM Radio"
}
```

**Static expires_at example:**
To set a fixed expiration date (e.g., for shows that don't expire):
```json
{
  "payloadMapping": {
    "item": {
      "name": "{{.title}}"
    },
    "expires_at": "2099-12-31T23:59:59Z"
  }
}
```

With `payloadMappingOmitEmpty: true`, empty fields are excluded. For example, if there's no artist:
```json
{
  "item": {
    "title": "Viva la Vida"
  },
  "expires_at": "2023-12-31T23:59:59Z"
}
```

**Available fields for mapping:**
- `{{.formatted_metadata}}` - The formatted text after applying formatters
- `{{.songID}}` - Song identifier
- `{{.title}}` - Song title
- `{{.artist}}` - Artist name
- `{{.duration}}` - Song duration
- `{{.updated_at}}` - When the metadata was updated
- `{{.expires_at}}` - When the metadata expires (null if no expiration)

**Template examples:**
```json
{
  "payloadMapping": {
    "description": "Now playing: {{.title}} by {{.artist}}",
    "category": "music",
    "timestamp": "{{.updated_at}}",
    "static_field": "This is a static value"
  }
}
```

**Output Options (all types):**
- `inputs` (required) - Array of input names in priority order
- `formatters` (optional) - Array of formatter names to apply

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