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
- Tracks current URL, content lines, links, selected link index
- scrollOffset: Display line offset (not content line) - accounts for wrapped lines
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
3. Automatically loads gemini://geminiprotocol.net/ as the default start page
4. FetchGemini() retrieves content over TLS
5. parseContent() extracts links (lines starting with "=>") and converts UTF-8 to ASCII
6. Links are numbered and displayed as "[N] link text"
7. User navigates with arrow keys (changes selectedLink)
8. Enter key follows the selected link (resolves relative URLs against current base)
9. Backspace returns to previous page via history stack
10. User can press 'g' at any time to enter a new Gemini URL

### Terminal Control

Uses VT100/ANSI escape sequences:
- `\x1b[2J\x1b[H` - clear screen and home cursor
- `\x1b[7m` - reverse video (highlight selected link)
- `\x1b[0m` - reset formatting
- `\x1b[K` - clear to end of line

Long lines are wrapped to the terminal width (default 80 columns). The wrapLine() function breaks lines into multiple display lines that fit within the terminal width. When a link line is wrapped, all wrapped segments are highlighted together.

**Scrolling with Wrapped Lines:**
- scrollOffset is measured in display lines, not content lines
- Helper functions convert between content lines and display lines:
  - getDisplayLineCount() - returns how many display lines a content line occupies
  - contentLineToDisplayLine() - converts content line index to display line index
  - getTotalDisplayLines() - returns total display lines for all content
- When scrolling (page up/down) or navigating links (arrows), the system automatically accounts for wrapped lines
- The render() function iterates through content lines, wraps each one, and skips display lines until reaching scrollOffset

Keyboard sequences:
- ESC [ A/B - up/down arrows (smart scrolling - see below)
- ESC [ 5~ - page up
- ESC [ 6~ - page down
- 0x1b alone - escape (cancel input)
- 0x7f/0x08 - backspace/delete

**Smart Arrow Key Scrolling:**
The handleArrowKey() function provides intelligent navigation:
- If the next link in the direction is visible on screen, jump to it
- If there's no next link (at first/last link), treat arrow as page up/down
- If the next link exists but is off-screen, page scroll toward it and select it if now visible
- This provides quick link-to-link jumping when possible, and page-based scrolling otherwise

**Direction-Aware Link Selection:**
When scrolling via arrows or Page Up/Down:
- scrollPageWithDirection(delta) uses updateLinkSelectionWithDirection(delta)
- If delta < 0 (scrolling up), selects the LAST visible link on the new page
- If delta >= 0 (scrolling down), selects the FIRST visible link on the new page
- This provides intuitive behavior: scrolling up highlights the bottom link, scrolling down highlights the top link

**Link Selection State Management:**
To prevent issues with selectedLink being out of bounds across page navigations:
- parseContent() resets selectedLink to 0 when parsing new content
- updateLinkSelectionWithDirection() performs bounds checking and has a fallback to select link 0
- This ensures selectedLink is always valid, even when navigating between pages with different numbers of links

### Gemini Protocol Details

- Requests are URL + CRLF
- Responses have header: STATUS_CODE SPACE META CRLF
- Status codes: 2x (success with body), 3x (redirect), others (errors/input)
- Only 2x responses include a body
- Default port is 1965
- TLS required (InsecureSkipVerify for simplicity)

## Common Modifications

When adding features or fixing bugs:

- **Default start page**: Change the URL in navigateTo() call in Run() in session.go
- **Keyboard shortcuts**: Modify handleInput() in session.go
- **Link rendering**: Update parseContent() to change how Gemini links are displayed
- **UTF-8 mappings**: Add entries to unicodeToASCII() in utils.go
- **Terminal size**: Adjust terminalHeight/terminalWidth in NewSession()
- **Gemini features**: Extend parseContent() to handle headings, lists, preformatted text, quotes
- **Server port**: Change the port constant in main()
