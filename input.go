package main

import (
	"fmt"
	"strings"
)

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
				oldScroll := s.scrollOffset
				s.handleArrowKey(-1)
				if s.scrollOffset == oldScroll {
					s.renderPartialLinkUpdate()
				} else {
					s.render()
				}
			case 'B': // Down arrow
				oldScroll := s.scrollOffset
				s.handleArrowKey(1)
				if s.scrollOffset == oldScroll {
					s.renderPartialLinkUpdate()
				} else {
					s.render()
				}
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
		s.lastByte = b
		s.inputMode = "goto"
		s.inputBuffer = ""
		s.write([]byte("\r\n\x1b[KEnter Gemini URL: "))
		return nil

	case '\r': // Enter - follow selected link
		s.lastByte = '\r'
		if s.selectedLink >= 0 && s.selectedLink < len(s.links) {
			link := s.links[s.selectedLink]
			s.navigateTo(link.URL)
		}
		return nil

	case '\n': // LF - ignore if it immediately follows CR (CRLF handling)
		if s.lastByte == '\r' {
			// This is part of CRLF, ignore it
			s.lastByte = '\n'
			return nil
		}
		// LF alone (some clients send just LF)
		s.lastByte = '\n'
		if s.selectedLink >= 0 && s.selectedLink < len(s.links) {
			link := s.links[s.selectedLink]
			s.navigateTo(link.URL)
		}
		return nil

	case 0x7f, 0x08: // Backspace/Delete - go back
		s.lastByte = b
		s.navigateBack()
		s.render()
		return nil

	case 'q', 'Q': // Quit
		return fmt.Errorf("user quit")

	default:
		s.lastByte = b
	}

	return nil
}

func (s *Session) handleGotoInput(b byte) error {
	switch b {
	case '\r': // Submit
		s.lastByte = '\r'
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

	case '\n': // LF - ignore if it immediately follows CR (CRLF handling)
		if s.lastByte == '\r' {
			// This is part of CRLF, ignore it
			s.lastByte = '\n'
			return nil
		}
		// LF alone (some clients send just LF)
		s.lastByte = '\n'
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
		s.lastByte = b
		s.inputMode = ""
		s.inputBuffer = ""
		s.render()
		return nil

	case 0x7f, 0x08: // Backspace
		s.lastByte = b
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
		s.lastByte = b
	}

	return nil
}
