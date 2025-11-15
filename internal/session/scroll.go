package session

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
	totalDisplayLines := s.getTotalDisplayLines()
	oldScrollOffset := s.scrollOffset

	// Calculate new scroll position
	newScrollOffset := s.scrollOffset + delta*linesPerPage

	// Clamp to valid range
	if newScrollOffset < 0 {
		newScrollOffset = 0
	}
	maxScroll := totalDisplayLines - linesPerPage
	if maxScroll < 0 {
		maxScroll = 0
	}
	if newScrollOffset > maxScroll {
		newScrollOffset = maxScroll
	}

	// If scroll position didn't change, we're at a boundary - send beep
	if newScrollOffset == oldScrollOffset {
		s.write([]byte("\x07")) // BEL - beep
		return
	}

	s.scrollOffset = newScrollOffset

	// Update link selection to ensure a visible link is selected
	s.updateLinkSelection()
}

func (s *Session) scrollPageWithDirection(delta int) {
	linesPerPage := s.terminalHeight - 3
	totalDisplayLines := s.getTotalDisplayLines()
	oldScrollOffset := s.scrollOffset

	// Calculate new scroll position
	newScrollOffset := s.scrollOffset + delta*linesPerPage

	// Clamp to valid range
	if newScrollOffset < 0 {
		newScrollOffset = 0
	}
	maxScroll := totalDisplayLines - linesPerPage
	if maxScroll < 0 {
		maxScroll = 0
	}
	if newScrollOffset > maxScroll {
		newScrollOffset = maxScroll
	}

	// If scroll position didn't change, we're at a boundary - send beep
	if newScrollOffset == oldScrollOffset {
		s.write([]byte("\x07")) // BEL - beep
		return
	}

	s.scrollOffset = newScrollOffset

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
