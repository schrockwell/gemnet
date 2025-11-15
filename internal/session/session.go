package session

import (
	"net"
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
	conn             net.Conn
	currentURL       string
	content          []string // Content lines
	links            []Link
	headerLines      map[int]bool // Set of line numbers that are headers
	selectedLink     int
	scrollOffset     int  // Display line offset (accounts for wrapping)
	prevSelectedLink int  // Previous selected link for partial redraw
	prevScrollOffset int  // Previous scroll offset for partial redraw
	lastByte         byte // Last byte received (for CRLF handling)
	history          []HistoryEntry
	historyIndex     int // Current position in history (-1 means no history)
	terminalHeight   int
	terminalWidth    int
	inputMode        string // "", "goto"
	inputBuffer      string
}

func New(conn net.Conn) *Session {
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

func (s *Session) write(data []byte) {
	s.conn.Write(data)
}
