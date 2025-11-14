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

type Session struct {
	conn            net.Conn
	currentURL      string
	content         []string // Content lines
	links           []Link
	selectedLink    int
	scrollOffset    int
	history         []string
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
		history:        make([]string, 0),
	}
}

func (s *Session) Run() error {
	// Initialize terminal
	s.write([]byte("\x1b[2J\x1b[H")) // Clear screen and move to home
	s.write([]byte("Welcome to gemnet - Gemini over Telnet\r\n"))
	s.write([]byte("Press 'g' to enter a Gemini URL\r\n"))
	s.write([]byte("\r\n"))

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
				s.moveLinkSelection(-1)
				s.render()
			case 'B': // Down arrow
				s.moveLinkSelection(1)
				s.render()
			case '5': // Page Up
				s.conn.Read(make([]byte, 1)) // Read trailing ~
				s.scrollPage(-1)
				s.render()
			case '6': // Page Down
				s.conn.Read(make([]byte, 1)) // Read trailing ~
				s.scrollPage(1)
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
		if len(s.history) > 0 {
			s.history = s.history[:len(s.history)-1]
			if len(s.history) > 0 {
				lastURL := s.history[len(s.history)-1]
				s.history = s.history[:len(s.history)-1] // Remove it since navigateTo will add it back
				s.navigateTo(lastURL)
			} else {
				s.currentURL = ""
				s.content = nil
				s.links = nil
				s.render()
			}
		}
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

	// Success - parse content
	s.history = append(s.history, urlStr)
	s.currentURL = urlStr
	s.parseContent(resp.Body)
	s.scrollOffset = 0
	s.selectedLink = 0
	s.render()
}

func (s *Session) parseContent(body string) {
	// Convert UTF-8 to ASCII
	asciiBody := UTF8ToASCII(body)

	// Split into lines
	lines := strings.Split(asciiBody, "\n")
	s.content = make([]string, 0, len(lines))
	s.links = make([]Link, 0)

	linkIndex := 0
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		// Check if this is a link line
		if strings.HasPrefix(line, "=>") {
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

	// Auto-scroll to keep selected link visible
	linkLine := s.links[s.selectedLink].Line
	visibleLines := s.terminalHeight - 3

	if linkLine < s.scrollOffset {
		s.scrollOffset = linkLine
	} else if linkLine >= s.scrollOffset+visibleLines {
		s.scrollOffset = linkLine - visibleLines + 1
	}
}

func (s *Session) scrollPage(delta int) {
	linesPerPage := s.terminalHeight - 3
	s.scrollOffset += delta * linesPerPage

	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
	if s.scrollOffset >= len(s.content) {
		s.scrollOffset = len(s.content) - 1
	}
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
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
	endLine := s.scrollOffset + visibleLines
	if endLine > len(s.content) {
		endLine = len(s.content)
	}

	// Find which link is currently selected
	selectedLine := -1
	if s.selectedLink >= 0 && s.selectedLink < len(s.links) {
		selectedLine = s.links[s.selectedLink].Line
	}

	for i := s.scrollOffset; i < endLine; i++ {
		line := s.content[i]

		// Highlight selected link
		if i == selectedLine {
			s.write([]byte("\x1b[7m")) // Reverse video
		}

		// Truncate line if too long
		if len(line) > s.terminalWidth-1 {
			line = line[:s.terminalWidth-1]
		}

		s.write([]byte(line))

		if i == selectedLine {
			s.write([]byte("\x1b[0m")) // Reset
		}

		s.write([]byte("\r\n"))
	}
}

func (s *Session) write(data []byte) {
	s.conn.Write(data)
}
