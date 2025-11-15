package gemini

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net/url"
	"strings"
)

type Response struct {
	StatusCode int
	Meta       string
	Body       string
}

// Fetch fetches a Gemini URL and returns the response
func Fetch(urlStr string) (*Response, error) {
	// Parse URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "gemini" {
		return nil, fmt.Errorf("only gemini:// URLs are supported")
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		host = host + ":1965"
	}

	// Connect with TLS (no certificate verification for simplicity)
	config := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	// Send request (URL + CRLF)
	request := urlStr + "\r\n"
	if _, err := conn.Write([]byte(request)); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	reader := bufio.NewReader(conn)

	// Read header line
	header, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	header = strings.TrimSpace(header)
	parts := strings.SplitN(header, " ", 2)
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid response header")
	}

	var statusCode int
	fmt.Sscanf(parts[0], "%d", &statusCode)

	meta := ""
	if len(parts) > 1 {
		meta = parts[1]
	}

	response := &Response{
		StatusCode: statusCode,
		Meta:       meta,
	}

	// For success responses (2x), read the body
	if statusCode >= 20 && statusCode < 30 {
		bodyBytes, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
		response.Body = string(bodyBytes)
	}

	return response, nil
}
