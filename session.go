package main

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

type Link struct {
	Index int
	URL   string
	Text  string
	Line  int // Line number where link appears
}

type HistoryEntry struct {
	URL          string
	ScrollOffset int
	SelectedLink int
}

type Session struct {
	conn            net.Conn
	currentURL      string
	content         []string // Content lines
	links           []Link
	headerLines     map[int]bool // Set of line numbers that are headers
	selectedLink    int
	scrollOffset    int      // Display line offset (accounts for wrapping)
	history         []HistoryEntry
	historyIndex    int // Current position in history (-1 means no history)
	terminalHeight  int
	terminalWidth   int
	inputMode       string // "", "goto"
	inputBuffer     string
}

func NewSession(conn net.Conn) *Session {
	return &Session{
		conn:           conn,
		terminalHeight: 24,
		terminalWidth:  80,
		selectedLink:   0,
		scrollOffset:   0,
		history:        make([]HistoryEntry, 0),
		historyIndex:   -1,
	}
}

func (s *Session) Run() error {
	// Initialize terminal
	s.write([]byte("\x1b[2J\x1b[H")) // Clear screen and move to home
	s.write([]byte("Welcome to gemnet - Gemini over Telnet\r\n"))
	s.write([]byte("\r\n"))

	// Load default page
	s.navigateTo("gemini://geminiprotocol.net/")

	// Main input loop
	buf := make([]byte, 1)
	for {
		n, err := s.conn.Read(buf)
		if err != nil {
			return err
		}

		if n > 0 {
			if err := s.handleInput(buf[0]); err != nil {
				return err
			}
		}
	}
}

func (s *Session) handleInput(b byte) error {
	// Check if we're in a special input mode
	if s.inputMode == "goto" {
		return s.handleGotoInput(b)
	}

	// Handle escape sequences
	if b == 0x1b { // ESC
		// Read next bytes for escape sequence
		seq := make([]byte, 2)
		s.conn.Read(seq)

		if seq[0] == '[' {
			switch seq[1] {
			case 'A': // Up arrow
				s.handleArrowKey(-1)
				s.render()
			case 'B': // Down arrow
				s.handleArrowKey(1)
				s.render()
			case 'C': // Right arrow - forward in history
				s.navigateForward()
				s.render()
			case 'D': // Left arrow - back in history
				s.navigateBack()
				s.render()
			case '5': // Page Up
				s.conn.Read(make([]byte, 1)) // Read trailing ~
				s.scrollPageWithDirection(-1)
				s.render()
			case '6': // Page Down
				s.conn.Read(make([]byte, 1)) // Read trailing ~
				s.scrollPageWithDirection(1)
				s.render()
			}
		}
		return nil
	}

	switch b {
	case 'g', 'G': // Go to URL
		s.inputMode = "goto"
		s.inputBuffer = ""
		s.write([]byte("\r\n\x1b[KEnter Gemini URL: "))
		return nil

	case '\r', '\n': // Enter - follow selected link
		if s.selectedLink >= 0 && s.selectedLink < len(s.links) {
			link := s.links[s.selectedLink]
			s.navigateTo(link.URL)
		}
		return nil

	case 0x7f, 0x08: // Backspace/Delete - go back
		s.navigateBack()
		s.render()
		return nil

	case 'q', 'Q': // Quit
		return fmt.Errorf("user quit")
	}

	return nil
}

func (s *Session) handleGotoInput(b byte) error {
	switch b {
	case '\r', '\n': // Submit
		s.inputMode = ""
		url := strings.TrimSpace(s.inputBuffer)
		if url != "" {
			// Add gemini:// prefix if not present
			if !strings.HasPrefix(url, "gemini://") {
				url = "gemini://" + url
			}
			s.navigateTo(url)
		} else {
			s.render()
		}
		return nil

	case 0x1b: // ESC - cancel
		s.inputMode = ""
		s.inputBuffer = ""
		s.render()
		return nil

	case 0x7f, 0x08: // Backspace
		if len(s.inputBuffer) > 0 {
			s.inputBuffer = s.inputBuffer[:len(s.inputBuffer)-1]
			s.write([]byte("\b \b")) // Erase character
		}
		return nil

	default:
		// Add printable characters to buffer
		if b >= 32 && b < 127 {
			s.inputBuffer += string(b)
			s.write([]byte{b})
		}
	}

	return nil
}

func (s *Session) navigateTo(urlStr string) {
	// Resolve relative URLs
	if s.currentURL != "" {
		base, err := url.Parse(s.currentURL)
		if err == nil {
			u, err := url.Parse(urlStr)
			if err == nil {
				urlStr = base.ResolveReference(u).String()
			}
		}
	}

	s.write([]byte(fmt.Sprintf("\r\n\x1b[KFetching %s...\r\n", urlStr)))

	resp, err := FetchGemini(urlStr)
	if err != nil {
		s.write([]byte(fmt.Sprintf("Error: %v\r\n", err)))
		s.write([]byte("Press any key to continue..."))
		buf := make([]byte, 1)
		s.conn.Read(buf)
		s.render()
		return
	}

	if resp.StatusCode >= 30 && resp.StatusCode < 40 {
		// Redirect
		s.navigateTo(resp.Meta)
		return
	}

	if resp.StatusCode < 20 || resp.StatusCode >= 30 {
		s.write([]byte(fmt.Sprintf("Error: Status %d - %s\r\n", resp.StatusCode, resp.Meta)))
		s.write([]byte("Press any key to continue..."))
		buf := make([]byte, 1)
		s.conn.Read(buf)
		s.render()
		return
	}

	// Success - save current page state to history before navigating
	if s.currentURL != "" {
		// Save current state
		currentEntry := HistoryEntry{
			URL:          s.currentURL,
			ScrollOffset: s.scrollOffset,
			SelectedLink: s.selectedLink,
		}

		// If we're in the middle of history, truncate forward history
		if s.historyIndex >= 0 && s.historyIndex < len(s.history)-1 {
			s.history = s.history[:s.historyIndex+1]
		}

		// Update the current history entry if it exists
		if s.historyIndex >= 0 && s.historyIndex < len(s.history) {
			s.history[s.historyIndex] = currentEntry
		} else if len(s.history) == 0 {
			s.history = append(s.history, currentEntry)
			s.historyIndex = 0
		}
	}

	// Parse new content
	s.currentURL = urlStr
	s.parseContent(resp.Body)
	s.scrollOffset = 0
	s.selectedLink = 0

	// Add new page to history
	newEntry := HistoryEntry{
		URL:          urlStr,
		ScrollOffset: 0,
		SelectedLink: 0,
	}
	s.history = append(s.history, newEntry)
	s.historyIndex = len(s.history) - 1

	s.render()
}

func (s *Session) parseContent(body string) {
	// Convert UTF-8 to ASCII
	asciiBody := UTF8ToASCII(body)

	// Split into lines
	lines := strings.Split(asciiBody, "\n")
	s.content = make([]string, 0, len(lines))
	s.links = make([]Link, 0)
	s.headerLines = make(map[int]bool)
	s.selectedLink = 0 // Reset selected link when parsing new content

	linkIndex := 0
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		// Check if this is a header line
		if strings.HasPrefix(line, "#") {
			s.headerLines[len(s.content)] = true
		} else if strings.HasPrefix(line, "=>") {
			// Check if this is a link line
			// Parse link
			linkText := strings.TrimSpace(line[2:])
			parts := strings.Fields(linkText)
			if len(parts) > 0 {
				linkURL := parts[0]
				linkLabel := linkURL
				if len(parts) > 1 {
					linkLabel = strings.Join(parts[1:], " ")
				}

				link := Link{
					Index: linkIndex,
					URL:   linkURL,
					Text:  linkLabel,
					Line:  len(s.content),
				}
				s.links = append(s.links, link)

				// Display link with index
				line = fmt.Sprintf("[%d] %s", linkIndex, linkLabel)
				linkIndex++
			}
		}

		s.content = append(s.content, line)
	}
}

func (s *Session) navigateBack() {
	if s.historyIndex <= 0 {
		return // Can't go back further
	}

	// Save current state
	if s.historyIndex < len(s.history) {
		s.history[s.historyIndex] = HistoryEntry{
			URL:          s.currentURL,
			ScrollOffset: s.scrollOffset,
			SelectedLink: s.selectedLink,
		}
	}

	// Move back in history
	s.historyIndex--
	s.loadFromHistory()
}

func (s *Session) navigateForward() {
	if s.historyIndex >= len(s.history)-1 {
		return // Can't go forward further
	}

	// Save current state
	if s.historyIndex >= 0 && s.historyIndex < len(s.history) {
		s.history[s.historyIndex] = HistoryEntry{
			URL:          s.currentURL,
			ScrollOffset: s.scrollOffset,
			SelectedLink: s.selectedLink,
		}
	}

	// Move forward in history
	s.historyIndex++
	s.loadFromHistory()
}

func (s *Session) loadFromHistory() {
	if s.historyIndex < 0 || s.historyIndex >= len(s.history) {
		return
	}

	entry := s.history[s.historyIndex]

	s.write([]byte(fmt.Sprintf("\r\n\x1b[KLoading %s...\r\n", entry.URL)))

	resp, err := FetchGemini(entry.URL)
	if err != nil {
		s.write([]byte(fmt.Sprintf("Error: %v\r\n", err)))
		s.write([]byte("Press any key to continue..."))
		buf := make([]byte, 1)
		s.conn.Read(buf)
		s.render()
		return
	}

	if resp.StatusCode < 20 || resp.StatusCode >= 30 {
		s.write([]byte(fmt.Sprintf("Error: Status %d - %s\r\n", resp.StatusCode, resp.Meta)))
		s.write([]byte("Press any key to continue..."))
		buf := make([]byte, 1)
		s.conn.Read(buf)
		s.render()
		return
	}

	// Load content and restore state
	s.currentURL = entry.URL
	s.parseContent(resp.Body)
	s.scrollOffset = entry.ScrollOffset
	s.selectedLink = entry.SelectedLink

	// Validate restored state
	if s.selectedLink >= len(s.links) {
		s.selectedLink = 0
	}
	totalDisplayLines := s.getTotalDisplayLines()
	if s.scrollOffset >= totalDisplayLines {
		s.scrollOffset = 0
	}
}

func (s *Session) handleArrowKey(delta int) {
	visibleLines := s.terminalHeight - 3

	// If no links, treat as page up/down
	if len(s.links) == 0 {
		s.scrollPageWithDirection(delta)
		return
	}

	// Check if there's a next/previous link
	nextLinkIdx := s.selectedLink + delta
	if nextLinkIdx < 0 || nextLinkIdx >= len(s.links) {
		// No next link in this direction, treat as page up/down
		s.scrollPageWithDirection(delta)
		return
	}

	// Check if next link is visible on current screen
	nextLinkContentLine := s.links[nextLinkIdx].Line
	nextLinkDisplayLine := s.contentLineToDisplayLine(nextLinkContentLine)

	isVisible := nextLinkDisplayLine >= s.scrollOffset &&
	             nextLinkDisplayLine < s.scrollOffset+visibleLines

	if isVisible {
		// Link is visible, jump to it
		s.moveLinkSelection(delta)
	} else {
		// Link is off-screen, page scroll and then select it
		s.scrollPageWithDirection(delta)
		// After page scroll, try to select the next link if it's now visible
		nextLinkDisplayLine = s.contentLineToDisplayLine(nextLinkContentLine)
		isNowVisible := nextLinkDisplayLine >= s.scrollOffset &&
		                nextLinkDisplayLine < s.scrollOffset+visibleLines
		if isNowVisible {
			s.selectedLink = nextLinkIdx
		}
	}
}

// updateLinkSelection ensures a visible link is selected after scrolling
func (s *Session) updateLinkSelection() {
	s.updateLinkSelectionWithDirection(1) // Default to forward direction
}

// updateLinkSelectionWithDirection ensures a visible link is selected after scrolling
// If delta < 0 (scrolling up), selects the last visible link
// If delta >= 0 (scrolling down), selects the first visible link
func (s *Session) updateLinkSelectionWithDirection(delta int) {
	if len(s.links) == 0 {
		s.selectedLink = 0
		return
	}

	visibleLines := s.terminalHeight - 3

	// Bounds check selectedLink
	if s.selectedLink < 0 {
		s.selectedLink = 0
	}
	if s.selectedLink >= len(s.links) {
		s.selectedLink = len(s.links) - 1
	}

	// Check if current selection is visible
	if s.selectedLink >= 0 && s.selectedLink < len(s.links) {
		currentLinkContentLine := s.links[s.selectedLink].Line
		currentLinkDisplayLine := s.contentLineToDisplayLine(currentLinkContentLine)

		isVisible := currentLinkDisplayLine >= s.scrollOffset &&
		             currentLinkDisplayLine < s.scrollOffset+visibleLines

		if isVisible {
			return // Current selection is fine
		}
	}

	if delta < 0 {
		// Scrolling up - find LAST visible link
		for i := len(s.links) - 1; i >= 0; i-- {
			link := s.links[i]
			linkDisplayLine := s.contentLineToDisplayLine(link.Line)
			if linkDisplayLine >= s.scrollOffset && linkDisplayLine < s.scrollOffset+visibleLines {
				s.selectedLink = i
				return
			}
		}
	} else {
		// Scrolling down - find FIRST visible link
		for i, link := range s.links {
			linkDisplayLine := s.contentLineToDisplayLine(link.Line)
			if linkDisplayLine >= s.scrollOffset && linkDisplayLine < s.scrollOffset+visibleLines {
				s.selectedLink = i
				return
			}
		}
	}

	// No visible links found - shouldn't happen, but select first link as fallback
	s.selectedLink = 0
}

func (s *Session) moveLinkSelection(delta int) {
	if len(s.links) == 0 {
		return
	}

	s.selectedLink += delta
	if s.selectedLink < 0 {
		s.selectedLink = 0
	}
	if s.selectedLink >= len(s.links) {
		s.selectedLink = len(s.links) - 1
	}

	// Auto-scroll to keep selected link visible (in display lines)
	linkContentLine := s.links[s.selectedLink].Line
	linkDisplayLine := s.contentLineToDisplayLine(linkContentLine)
	visibleLines := s.terminalHeight - 3

	if linkDisplayLine < s.scrollOffset {
		s.scrollOffset = linkDisplayLine
	} else if linkDisplayLine >= s.scrollOffset+visibleLines {
		s.scrollOffset = linkDisplayLine - visibleLines + 1
	}
}

func (s *Session) scrollPage(delta int) {
	linesPerPage := s.terminalHeight - 3
	s.scrollOffset += delta * linesPerPage

	totalDisplayLines := s.getTotalDisplayLines()

	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
	if s.scrollOffset >= totalDisplayLines {
		s.scrollOffset = totalDisplayLines - 1
	}
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}

	// Update link selection to ensure a visible link is selected
	s.updateLinkSelection()
}

func (s *Session) scrollPageWithDirection(delta int) {
	linesPerPage := s.terminalHeight - 3
	s.scrollOffset += delta * linesPerPage

	totalDisplayLines := s.getTotalDisplayLines()

	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
	if s.scrollOffset >= totalDisplayLines {
		s.scrollOffset = totalDisplayLines - 1
	}
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}

	// Update link selection based on scroll direction
	s.updateLinkSelectionWithDirection(delta)
}

func (s *Session) wrapLine(line string) []string {
	if len(line) <= s.terminalWidth-1 {
		return []string{line}
	}

	var wrapped []string
	for len(line) > 0 {
		if len(line) <= s.terminalWidth-1 {
			wrapped = append(wrapped, line)
			break
		}
		wrapped = append(wrapped, line[:s.terminalWidth-1])
		line = line[s.terminalWidth-1:]
	}
	return wrapped
}

// getDisplayLineCount returns how many display lines a content line takes
func (s *Session) getDisplayLineCount(contentLineIdx int) int {
	if contentLineIdx < 0 || contentLineIdx >= len(s.content) {
		return 0
	}
	return len(s.wrapLine(s.content[contentLineIdx]))
}

// contentLineToDisplayLine converts a content line index to its first display line index
func (s *Session) contentLineToDisplayLine(contentLineIdx int) int {
	displayLine := 0
	for i := 0; i < contentLineIdx && i < len(s.content); i++ {
		displayLine += s.getDisplayLineCount(i)
	}
	return displayLine
}

// getTotalDisplayLines returns the total number of display lines
func (s *Session) getTotalDisplayLines() int {
	total := 0
	for i := 0; i < len(s.content); i++ {
		total += s.getDisplayLineCount(i)
	}
	return total
}

func (s *Session) render() {
	// Clear screen
	s.write([]byte("\x1b[2J\x1b[H"))

	// Status line
	status := "gemnet"
	if s.currentURL != "" {
		status = s.currentURL
		if len(status) > s.terminalWidth-1 {
			status = status[:s.terminalWidth-1]
		}
	}
	s.write([]byte(status))
	s.write([]byte("\r\n"))
	s.write([]byte(strings.Repeat("-", s.terminalWidth)))
	s.write([]byte("\r\n"))

	// Content area
	if s.content == nil {
		s.write([]byte("No page loaded. Press 'g' to enter a URL.\r\n"))
		return
	}

	visibleLines := s.terminalHeight - 3

	// Find which link is currently selected
	selectedContentLine := -1
	if s.selectedLink >= 0 && s.selectedLink < len(s.links) {
		selectedContentLine = s.links[s.selectedLink].Line
	}

	// Render content with wrapping, accounting for scroll offset in display lines
	currentDisplayLine := 0
	linesDisplayed := 0

	for contentLineIdx := 0; contentLineIdx < len(s.content) && linesDisplayed < visibleLines; contentLineIdx++ {
		line := s.content[contentLineIdx]
		wrappedLines := s.wrapLine(line)
		isSelected := contentLineIdx == selectedContentLine
		isHeader := s.headerLines[contentLineIdx]

		for _, wrappedLine := range wrappedLines {
			// Skip lines before scroll offset
			if currentDisplayLine < s.scrollOffset {
				currentDisplayLine++
				continue
			}

			// Stop if we've filled the screen
			if linesDisplayed >= visibleLines {
				break
			}

			if isSelected {
				s.write([]byte("\x1b[7m")) // Reverse video
			} else if isHeader {
				s.write([]byte("\x1b[1m")) // Bold for headers
			}

			s.write([]byte(wrappedLine))

			if isSelected || isHeader {
				s.write([]byte("\x1b[0m")) // Reset
			}

			s.write([]byte("\r\n"))
			linesDisplayed++
			currentDisplayLine++
		}
	}
}

func (s *Session) write(data []byte) {
	s.conn.Write(data)
}
