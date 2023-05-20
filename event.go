package sse

import (
	"bytes"
	"fmt"
	"time"
)

const (
	defaultMessageType = "message"

	fieldNameEvent = "event"
	fieldNameData  = "data"
	fieldNameID    = "id"
	fieldNameRetry = "retry"
)

// Event represents an individual event from the event stream.
type Event struct {
	Type string
	Data string
	ID   string

	// Retry is only used when sending events, not receiving them.
	Retry time.Duration
}

// Bytes returns the byte representation of the event. Note that the result is
// only valid if Type, Data, and ID do NOT contain a CR or LF.
func (e *Event) Bytes() []byte {
	b := &bytes.Buffer{}
	if e.Type != "" {
		fmt.Fprintf(b, "event:%s\r", e.Type)
	}
	if e.ID != "" {
		fmt.Fprintf(b, "id:%s\r", e.ID)
	}
	if e.Retry != 0 {
		fmt.Fprintf(b, "retry:%d\r", e.Retry.Milliseconds())
	}
	fmt.Fprintf(b, "data:%s\r\r", e.Data)
	return b.Bytes()
}
