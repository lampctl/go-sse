package sse

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

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
			Name:    "Carriage return with more data",
			Data:    []byte("\r"),
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
		Name   string
		Input  string
		Events []*Event
		Err    error
	}{
		{
			Name:   "Empty",
			Input:  "",
			Events: nil,
			Err:    nil,
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
			t.Fatalf("%s (events): %#v != %#v", v.Name, events, v.Events)
		}
		if nextEventErr != v.Err {
			t.Fatalf("%s (err): %#v != %#v", v.Name, nextEventErr, v.Err)
		}
	}
}
