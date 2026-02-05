package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ReadMessage reads an LSP message (header + body) from the reader.
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	// 1. Read Headers
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			// End of headers
			break
		}

		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 && parts[0] == "Content-Length" {
			contentLength, err = strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %v", err)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing or zero Content-Length")
	}

	// 2. Read Body
	body := make([]byte, contentLength)
	_, err := io.ReadFull(r, body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %v", err)
	}

	return body, nil
}

// WriteMessage writes an LSP message to the writer.
func WriteMessage(w io.Writer, msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := w.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	return nil
}
