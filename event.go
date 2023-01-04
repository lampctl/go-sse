package sse

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
}
