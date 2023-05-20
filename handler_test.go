package sse

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

func runOnce(fn func()) func() {
	hasRun := false
	return func() {
		if !hasRun {
			fn()
			hasRun = true
		}
	}
}

func TestHandler(t *testing.T) {
	for _, v := range []struct {
		Name              string
		ChannelBufferSize int
		Fn                func(
			h *Handler,
			hClose func(),
			c *Client,
			cClose func(),
		) error
	}{
		{
			Name: "basic event propagation",
			Fn: func(h *Handler, hClose func(), c *Client, cClose func()) error {
				h.Send(&Event{ID: "1"})
				if err := receiveAtLeastNEvents(1, c, CLIENT_DELAY); err != nil {
					return err
				}
				return nil
			},
		},
		{
			Name: "handle client disconnect",
			Fn: func(h *Handler, hClose func(), c *Client, cClose func()) error {
				cClose()
				time.Sleep(CLIENT_DELAY)
				return func() error {
					defer h.mutex.Unlock()
					h.mutex.Lock()
					if len(h.eventChans) != 0 {
						return errors.New("client event channel still present")
					}
					return nil
				}()
			},
		},
		{
			Name: "handle server disconnect",
			Fn: func(h *Handler, hClose func(), c *Client, cClose func()) error {
				hClose()
				return nil
			},
		},
	} {
		func() {

			// Create the handler
			var (
				h = NewHandler(&HandlerConfig{
					ChannelBufferSize: v.ChannelBufferSize,
				})
				hClose = runOnce(func() { h.Close() })
			)
			defer hClose()

			// Create the server
			s := httptest.NewServer(h)
			defer s.Close()

			// Create the client
			c, err := NewClientFromURL(s.URL)
			if err != nil {
				t.Fatalf("%s: %s", v.Name, err)
			}
			cClose := runOnce(func() { c.Close() })
			defer cClose()

			// Wait for the client to connect
			time.Sleep(CLIENT_DELAY)

			// Run the test
			if err := v.Fn(h, hClose, c, cClose); err != nil {
				t.Fatalf("%s: %s", v.Name, err)
			}
		}()
	}
}
