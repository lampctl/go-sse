package sse

import (
	"testing"
	"time"
)

func TestBytes(t *testing.T) {
	for _, v := range []struct {
		Name   string
		Event  *Event
		Output string
	}{
		{
			Name:   "empty event",
			Event:  &Event{},
			Output: "data:\r\r",
		},
		{
			Name:   "event with ID and type",
			Event:  &Event{Type: "test", ID: "1"},
			Output: "event:test\rid:1\rdata:\r\r",
		},
		{
			Name:   "event with retry",
			Event:  &Event{Retry: 1 * time.Second},
			Output: "retry:1000\rdata:\r\r",
		},
	} {
		b := v.Event.Bytes()
		if string(b) != v.Output {
			t.Fatalf("%s: %#v != %#v", v.Name, string(b), v.Output)
		}
	}
}
