# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gemnet is a telnet-to-Gemini proxy server that enables vintage computing systems to browse the modern Gemini protocol. It bridges the gap between old technology (lacking TLS and UTF-8 support) and the Gemini ecosystem by providing a plain-text, ASCII-only interface over telnet.

## Building and Running

```bash
# Build the project
go build

# Run the server
./gemnet
```

The server listens on port 2323 by default. Connect using any telnet client:
```bash
telnet localhost 2323
```

## Architecture

### Core Components

**main.go**
- Entry point and telnet server
- Accepts incoming TCP connections on port 2323
- Spawns a new goroutine (session) for each client connection

**session.go**
- Manages individual client sessions and terminal state
- Handles all keyboard input and navigation logic
- Maintains page content, link list, scroll position, and history stack
- Renders the terminal UI using VT100/ANSI escape codes
- Input modes: normal navigation and "goto" mode for URL entry

**gemini.go**
- Implements Gemini protocol client
- Establishes TLS connections to Gemini servers (port 1965)
- Sends requests and parses responses
- Returns structured GeminiResponse with status code, meta, and body

**utils.go**
- UTF-8 to ASCII conversion logic
- Maps Unicode characters to ASCII equivalents (accented letters, special quotes, etc.)
- Replaces unmappable characters with "?" to ensure 8-bit ASCII output

### Key Data Structures

**Session**
- Tracks current URL, content lines, links, selected link index, scroll offset
- Maintains navigation history for back functionality
- Terminal dimensions (default 80x24)
- Input state (mode and buffer for URL entry)

**Link**
- Index number for display
- URL (can be relative or absolute)
- Display text
- Line number where the link appears in content

### Navigation Flow

1. Client connects via telnet
2. Session initialized with welcome message
3. User presses 'g' to enter a Gemini URL
4. FetchGemini() retrieves content over TLS
5. parseContent() extracts links (lines starting with "=>") and converts UTF-8 to ASCII
6. Links are numbered and displayed as "[N] link text"
7. User navigates with arrow keys (changes selectedLink)
8. Enter key follows the selected link (resolves relative URLs against current base)
9. Backspace returns to previous page via history stack

### Terminal Control

Uses VT100/ANSI escape sequences:
- `\x1b[2J\x1b[H` - clear screen and home cursor
- `\x1b[7m` - reverse video (highlight selected link)
- `\x1b[0m` - reset formatting
- `\x1b[K` - clear to end of line

Keyboard sequences:
- ESC [ A/B - up/down arrows
- ESC [ 5~ - page up
- ESC [ 6~ - page down
- 0x1b alone - escape (cancel input)
- 0x7f/0x08 - backspace/delete

### Gemini Protocol Details

- Requests are URL + CRLF
- Responses have header: STATUS_CODE SPACE META CRLF
- Status codes: 2x (success with body), 3x (redirect), others (errors/input)
- Only 2x responses include a body
- Default port is 1965
- TLS required (InsecureSkipVerify for simplicity)

## Common Modifications

When adding features or fixing bugs:

- **Keyboard shortcuts**: Modify handleInput() in session.go
- **Link rendering**: Update parseContent() to change how Gemini links are displayed
- **UTF-8 mappings**: Add entries to unicodeToASCII() in utils.go
- **Terminal size**: Adjust terminalHeight/terminalWidth in NewSession()
- **Gemini features**: Extend parseContent() to handle headings, lists, preformatted text, quotes
- **Server port**: Change the port constant in main()
