package sse

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// TODO: spec allows for a BOM at the beginning of the stream

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
	scanner     *bufio.Scanner
	lastEventID string
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
	var (
		eventType = defaultMessageType
		eventData []string
		eventID   = r.lastEventID
	)
	for len(eventData) == 0 {
		for {
			if !r.scanner.Scan() {
				return nil, r.scanner.Err()
			}
			line := r.scanner.Bytes()
			if len(line) == 0 {
				break
			}
			if line[0] == ':' {
				continue
			}
			var (
				field []byte = line
				value []byte
			)
			if i := bytes.IndexRune(line, ':'); i != -1 {
				field = line[:i]
				value = line[i+1:]
				if len(value) != 0 && value[0] == ' ' {
					value = value[1:]
				}
			}
			switch string(field) {
			case fieldNameEvent:
				eventType = string(value)
			case fieldNameData:
				eventData = append(eventData, string(value))
			case fieldNameID:
				if !bytes.Contains(value, []byte{'\x00'}) {
					eventID = string(value)
					r.lastEventID = eventID
				}
			}
		}
	}
	return &Event{
		Type: eventType,
		Data: strings.Join(eventData, "\n"),
		ID:   eventID,
	}, nil
}
