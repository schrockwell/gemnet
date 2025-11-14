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
- Maintains page content, link list, header lines, scroll position, and history stack
- Renders the terminal UI using VT100/ANSI escape codes with partial update optimization
- Input modes: normal navigation and "goto" mode for URL entry
- Detects and highlights Gemini headers (lines starting with #) in bold text
- Uses smart rendering: only redraws changed links when navigating without scrolling
- CRLF handling: Tracks last byte to detect CRLF sequences and ignore the LF (prevents dual navigation)

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
- headerLines: Map of content line numbers that are headers (for bold rendering)
- scrollOffset: Display line offset (not content line) - accounts for wrapped lines
- prevScrollOffset, prevSelectedLink: Previous state for detecting when partial redraw is possible
- lastByte: Last byte received (for CRLF handling - prevents dual Enter processing)
- history: Array of HistoryEntry structs storing URL, scroll position, and selected link for each visited page
- historyIndex: Current position in history array (-1 means no history)
- Terminal dimensions (default 80x24)
- Input state (mode and buffer for URL entry)

**HistoryEntry**
- URL: The page URL
- ScrollOffset: Saved scroll position for this page
- SelectedLink: Saved selected link index for this page

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
5. parseContent() processes content:
   - Extracts links (lines starting with "=>") and numbers them as "[N] link text"
   - Detects headers (lines starting with "#") and marks them for bold rendering (# symbols kept)
   - Converts UTF-8 to ASCII for vintage terminal compatibility
6. User navigates with:
   - Up/Down arrows to change selected link
   - Left arrow to go back in history
   - Right arrow to go forward in history
   - Enter to follow the selected link (resolves relative URLs against current base)
   - Backspace also goes back in history (same as left arrow)
   - 'g' to enter a new Gemini URL at any time
7. History preserves scroll position and selected link for each page
8. When navigating to a new page, forward history is truncated

### Terminal Control

Uses VT100/ANSI escape sequences:
- `\x1b[2J\x1b[H` - clear screen and home cursor
- `\x1b[7m` - reverse video (highlight selected link)
- `\x1b[1m` - bold/bright text (for headers)
- `\x1b[0m` - reset formatting
- `\x1b[K` - clear to end of line
- `\x1b[row;colH` - move cursor to specific position (for partial updates)

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
- ESC [ C - right arrow (forward in history)
- ESC [ D - left arrow (back in history)
- ESC [ 5~ - page up
- ESC [ 6~ - page down
- 0x1b alone - escape (cancel input)
- 0x7f/0x08 - backspace/delete (back in history)

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

**History Navigation:**
Left/Right arrows and Backspace provide full browser-like history navigation:
- navigateTo() saves current page state (URL, scrollOffset, selectedLink) before navigating to a new page
- If navigating from the middle of history, forward history is truncated
- navigateBack() and navigateForward() move through the history array and call loadFromHistory()
- loadFromHistory() fetches the page and restores the saved scroll position and selected link
- State validation ensures restored positions are valid (bounds checking for links and scroll offset)

**Rendering Optimization:**
To improve responsiveness on vintage terminals with slow baud rates:
- prevScrollOffset and prevSelectedLink track the previous state
- When up/down arrows change the selected link WITHOUT scrolling, only those two links are redrawn
- render() does a full screen redraw and updates prev state for next comparison
- renderPartialLinkUpdate() only redraws the old and new selected links
- renderContentLine() positions the cursor and redraws a single content line at its screen location
- This dramatically reduces output for link navigation, making the UI much snappier on slow connections

**CRLF Handling:**
Many telnet clients send CRLF (\\r\\n) for the Enter key, which would trigger dual navigation:
- Session.lastByte tracks the last byte received
- When CR (\\r) is received, it processes the input and sets lastByte = '\\r'
- When LF (\\n) is received, it checks if lastByte == '\\r'
  - If yes: this is part of CRLF, so LF is ignored (but lastByte is updated to '\\n')
  - If no: this is a standalone LF, so it processes the input
- All other input updates lastByte to prevent stale state
- This prevents the "double enter" bug where following a link would navigate twice

### Gemini Protocol Details

- Requests are URL + CRLF
- Responses have header: STATUS_CODE SPACE META CRLF
- Status codes: 2x (success with body), 3x (redirect), others (errors/input)
- Only 2x responses include a body
- Default port is 1965
- TLS required (InsecureSkipVerify for simplicity)

**Gemini Content Formatting:**
- Lines starting with `=>` are links - parsed, numbered, and made selectable
- Lines starting with `#`, `##`, or `###` are headers - displayed in bold with # symbols preserved
- All other content is displayed as plain text
- Content is converted from UTF-8 to ASCII for vintage terminal compatibility

## Common Modifications

When adding features or fixing bugs:

- **Default start page**: Change the URL in navigateTo() call in Run() in session.go
- **Keyboard shortcuts**: Modify handleInput() in session.go
- **Link rendering**: Update parseContent() to change how Gemini links are displayed; also update renderContentLine() if display format changes
- **Header rendering**: Modify the header detection logic in parseContent() and formatting in render() and renderContentLine()
- **Rendering optimization**: Modify renderPartialLinkUpdate() and renderContentLine() for different update strategies
- **History behavior**: Modify navigateBack(), navigateForward(), and loadFromHistory() in session.go
- **State preservation**: Add fields to HistoryEntry struct to save additional state per page
- **UTF-8 mappings**: Add entries to unicodeToASCII() in utils.go
- **Terminal size**: Adjust terminalHeight/terminalWidth in NewSession()
- **Gemini features**: Extend parseContent() to handle lists, preformatted text, quotes (currently handles links and headers)
- **Server port**: Change the port constant in main()
