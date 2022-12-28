package sse

import (
	"bufio"
	"bytes"
	"io"
)

func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	var (
		eol  = bytes.IndexAny(data, "\r\n")
		crlf = 1
	)
	if eol == -1 {
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
	if data[eol] == '\r' {
		if eol+1 < len(data) {
			if data[eol+1] == '\n' {
				crlf = 2
			}
		} else if !atEOF {
			return 0, nil, nil
		}
	}
	return eol + crlf, data[0:eol], nil
}

// Reader reads events from an io.Reader.
type Reader struct {
	scanner *bufio.Scanner
}

// NewReader creates a new Reader instance for the provided io.Reader.
func NewReader(r io.Reader) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Split(scanLines)
	return &Reader{
		scanner: scanner,
	}
}

// NextEvent blocks until the next event is received, there are no more events,
// or an error occurs.
func (r *Reader) NextEvent() (*Event, error) {
	if !r.scanner.Scan() {
		return nil, r.scanner.Err()
	}

	// TODO: parse line

	return &Event{}, nil
}
