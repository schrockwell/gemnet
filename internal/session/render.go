package session

import (
	"fmt"
	"strings"
)

func (s *Session) render() {
	// Clear screen
	s.write([]byte("\x1b[2J\x1b[H"))

	// Status line
	statusLine := "gemnet"

	if s.currentURL != "" {
		// Calculate progress information based on LAST visible line
		visibleLines := s.terminalHeight - 3
		totalDisplayLines := s.getTotalDisplayLines()

		// Current line is the last line visible on screen
		currentLine := s.scrollOffset + visibleLines
		if currentLine > totalDisplayLines {
			currentLine = totalDisplayLines
		}
		if currentLine < 1 {
			currentLine = 1
		}

		percentage := 0
		if totalDisplayLines > 0 {
			percentage = (currentLine * 100) / totalDisplayLines
			if percentage > 100 {
				percentage = 100
			}
		}

		// Format progress with padded percentage (always 3 digits + %)
		progress := fmt.Sprintf("[%d/%d] %3d%%", currentLine, totalDisplayLines, percentage)

		// Calculate how much space we have for the URL
		maxURLLen := s.terminalWidth - len(progress) - 1 // -1 for space between URL and progress
		if maxURLLen < 10 {
			maxURLLen = 10
		}

		url := s.currentURL
		if len(url) > maxURLLen {
			url = url[:maxURLLen]
		}

		// Right-align progress by padding with spaces
		padding := s.terminalWidth - len(url) - len(progress)
		if padding < 1 {
			padding = 1
		}

		statusLine = url + strings.Repeat(" ", padding) + progress
	}

	s.write([]byte(statusLine))
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

	// Update state for next render
	s.prevScrollOffset = s.scrollOffset
	s.prevSelectedLink = s.selectedLink
}

// renderPartialLinkUpdate redraws only the changed links when scrolling hasn't occurred
func (s *Session) renderPartialLinkUpdate() {
	if s.content == nil {
		return
	}

	visibleLines := s.terminalHeight - 3

	// Redraw old selected link (remove highlight)
	if s.prevSelectedLink >= 0 && s.prevSelectedLink < len(s.links) {
		oldLinkContentLine := s.links[s.prevSelectedLink].Line
		s.renderContentLine(oldLinkContentLine, s.prevSelectedLink, visibleLines)
	}

	// Redraw new selected link (add highlight)
	if s.selectedLink >= 0 && s.selectedLink < len(s.links) {
		newLinkContentLine := s.links[s.selectedLink].Line
		s.renderContentLine(newLinkContentLine, s.selectedLink, visibleLines)
	}

	// Update state for next render
	s.prevSelectedLink = s.selectedLink
}

// renderContentLine redraws a specific content line at its screen position
func (s *Session) renderContentLine(contentLineIdx int, selectedLinkIdx int, visibleLines int) {
	if contentLineIdx < 0 || contentLineIdx >= len(s.content) {
		return
	}

	// Calculate which screen line this content line starts on
	displayLine := s.contentLineToDisplayLine(contentLineIdx)

	// Check if this line is visible
	if displayLine < s.scrollOffset || displayLine >= s.scrollOffset+visibleLines {
		return
	}

	// Calculate screen row (0-indexed from content area start)
	screenRow := displayLine - s.scrollOffset
	// +3 for status line, separator, and 0-indexing -> 1-indexing
	absoluteRow := screenRow + 3

	line := s.content[contentLineIdx]
	wrappedLines := s.wrapLine(line)
	isSelected := s.selectedLink >= 0 && s.selectedLink < len(s.links) &&
	              s.links[s.selectedLink].Line == contentLineIdx
	isHeader := s.headerLines[contentLineIdx]

	// Render each wrapped segment
	for i, wrappedLine := range wrappedLines {
		currentRow := absoluteRow + i

		// Make sure we don't go past the visible area
		if screenRow+i >= visibleLines {
			break
		}

		// Move cursor to the line position
		s.write([]byte(fmt.Sprintf("\x1b[%d;1H", currentRow)))

		// Clear the line
		s.write([]byte("\x1b[K"))

		// Apply formatting
		if isSelected {
			s.write([]byte("\x1b[7m")) // Reverse video
		} else if isHeader {
			s.write([]byte("\x1b[1m")) // Bold for headers
		}

		s.write([]byte(wrappedLine))

		if isSelected || isHeader {
			s.write([]byte("\x1b[0m")) // Reset
		}
	}
}
