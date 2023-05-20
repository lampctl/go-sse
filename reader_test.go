package sse

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// String returns a string representation of the event.
func (e *Event) String() string {
	return fmt.Sprintf("{Type:%#v, Data:%#v, ID:%#v}", e.Type, e.Data, e.ID)
}

func TestScanLines(t *testing.T) {
	for _, v := range []struct {
		Name    string
		Data    []byte
		AtEOF   bool
		Advance int
		Token   []byte
		Err     error
	}{
		{
			Name:    "Empty body with more data",
			Data:    []byte(""),
			AtEOF:   false,
			Advance: 0,
			Token:   nil,
			Err:     nil,
		},
		{
			Name:    "Carriage return with no more data",
			Data:    []byte("\r"),
			AtEOF:   true,
			Advance: 1,
			Token:   nil,
			Err:     nil,
		},
		{
			Name:    "Carriage return with line feed",
			Data:    []byte("\r\n"),
			AtEOF:   false,
			Advance: 2,
			Token:   nil,
			Err:     nil,
		},
		{
			Name:    "No CRLF before EOF",
			Data:    []byte(""),
			AtEOF:   false,
			Advance: 0,
			Token:   nil,
			Err:     nil,
		},
		{
			Name:    "No CRLF at EOF",
			Data:    []byte(""),
			AtEOF:   true,
			Advance: 0,
			Token:   nil,
			Err:     nil,
		},
	} {
		advance, token, err := scanLines(v.Data, v.AtEOF)
		if advance != v.Advance {
			t.Fatalf("%s (advance): %#v != %#v", v.Name, advance, v.Advance)
		}
		if !bytes.Equal(token, v.Token) {
			t.Fatalf("%s (token): %#v != %#v", v.Name, token, v.Token)
		}
		if err != v.Err {
			t.Fatalf("%s (err): %#v != %#v", v.Name, err, v.Err)
		}
	}
}

func TestReader(t *testing.T) {
	for _, v := range []struct {
		Name             string
		Input            string
		Events           []*Event
		Err              error
		LastEventID      string
		ReconnectionTime int
	}{
		{
			Name:             "Empty",
			Input:            "",
			Events:           nil,
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 0,
		},
		{
			Name:             "Comment",
			Input:            ":test\n\n",
			Events:           nil,
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 0,
		},
		{
			Name:             "Field without CRLF",
			Input:            "event",
			Events:           nil,
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 0,
		},
		{
			Name:             "Field with no value",
			Input:            "event\n\n",
			Events:           nil,
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 0,
		},
		{
			Name:             "Event with no type",
			Input:            "data\n\n",
			Events:           []*Event{{Type: defaultMessageType}},
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 0,
		},
		{
			Name:             "Event with type",
			Input:            "event:1\ndata:\n\n",
			Events:           []*Event{{Type: "1"}},
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 0,
		},
		{
			Name:             "Event with ID",
			Input:            "id:1\ndata:\n\n",
			Events:           []*Event{{Type: defaultMessageType, ID: "1"}},
			Err:              nil,
			LastEventID:      "1",
			ReconnectionTime: 0,
		},
		{
			Name:  "Event retaining previous ID",
			Input: "id:1\ndata:\n\ndata:\n\n",
			Events: []*Event{
				{Type: defaultMessageType, ID: "1"},
				{Type: defaultMessageType, ID: "1"},
			},
			Err:              nil,
			LastEventID:      "1",
			ReconnectionTime: 0,
		},
		{
			Name:             "Ignore NULL character in ID",
			Input:            "id:\x00\ndata:\n\n",
			Events:           []*Event{{Type: defaultMessageType}},
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 0,
		},
		{
			Name:             "Retry",
			Input:            "data\nretry:10\n\n",
			Events:           []*Event{{Type: defaultMessageType}},
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 10,
		},
		{
			Name:             "Bad retry",
			Input:            "data\nretry:$\n\n",
			Events:           []*Event{{Type: defaultMessageType}},
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 0,
		},
		{
			Name:             "Event with multiline data",
			Input:            "data:\ndata:\n\n",
			Events:           []*Event{{Type: defaultMessageType, Data: "\n"}},
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 0,
		},
		{
			Name:  "WHATWG example #1",
			Input: ": test stream\n\ndata: first event\nid: 1\n\ndata:second event\nid\n\ndata:  third event\n\n",
			Events: []*Event{
				{Type: defaultMessageType, Data: "first event", ID: "1"},
				{Type: defaultMessageType, Data: "second event"},
				{Type: defaultMessageType, Data: " third event"},
			},
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 0,
		},
		{
			Name:  "WHATWG example #2",
			Input: "data\n\ndata\ndata\n\ndata:",
			Events: []*Event{
				{Type: defaultMessageType},
				{Type: defaultMessageType, Data: "\n"},
			},
			Err:              nil,
			LastEventID:      "",
			ReconnectionTime: 0,
		},
	} {
		var (
			r            = NewReader(strings.NewReader(v.Input))
			events       []*Event
			nextEventErr error
		)
		for {
			e, err := r.NextEvent()
			if err != nil {
				nextEventErr = err
				break
			}
			if e == nil {
				break
			}
			events = append(events, e)
		}
		if !reflect.DeepEqual(events, v.Events) {
			t.Fatalf("%s (events): %+v != %+v", v.Name, events, v.Events)
		}
		if nextEventErr != v.Err {
			t.Fatalf("%s (err): %#v != %#v", v.Name, nextEventErr, v.Err)
		}
		if r.LastEventID != v.LastEventID {
			t.Fatalf("%s (err): %#v != %#v", v.Name, r.LastEventID, v.LastEventID)
		}
		if r.ReconnectionTime != v.ReconnectionTime {
			t.Fatalf("%s (err): %#v != %#v", v.Name, r.ReconnectionTime, v.ReconnectionTime)
		}
	}
}
