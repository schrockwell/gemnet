package session

import (
	"fmt"
	"net/url"
	"strings"

	"gemnet/internal/gemini"
	"gemnet/internal/util"
)

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

	resp, err := gemini.Fetch(urlStr)
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
	asciiBody := util.UTF8ToASCII(body)

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

	resp, err := gemini.Fetch(entry.URL)
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
