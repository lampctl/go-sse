package sse

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const CLIENT_DELAY = time.Millisecond * 100

func TestClient(t *testing.T) {
	for _, v := range []struct {
		Name      string
		Data      []byte
		DataDelay time.Duration
		Fn        func(c *Client) error
	}{
		{
			Name:      "Cancel immediately",
			Data:      []byte(""),
			DataDelay: CLIENT_DELAY * 2,
			Fn: func(c *Client) error {
				return nil
			},
		},
		{
			Name:      "Cancel after initial connection",
			Data:      []byte(""),
			DataDelay: CLIENT_DELAY * 2,
			Fn: func(c *Client) error {
				time.Sleep(CLIENT_DELAY)
				return nil
			},
		},
		{
			Name: "Send one event",
			Data: []byte("data\n\n"),
			Fn: func(c *Client) error {
				select {
				case <-c.Events:
				case <-time.After(CLIENT_DELAY):
					return errors.New("event expected")
				}
				return nil
			},
		},
	} {
		func() {
			s := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.Write(v.Data)
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					time.Sleep(v.DataDelay)
				},
			))
			defer s.Close()
			c, err := NewClientFromURL(s.URL)
			if err != nil {
				t.Fatalf("%s: %s", v.Name, err)
			}
			defer c.Close()
			if err := v.Fn(c); err != nil {
				t.Fatalf("%s: %s", v.Name, err)
			}
		}()
	}
}
