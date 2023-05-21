package sse

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testHandlerServerAndClient struct {
	Handler *Handler
	Server  *httptest.Server
	Client  *Client
}

func (h *testHandlerServerAndClient) CreateHandlerAndServer(cfg *HandlerConfig) {
	h.Handler = NewHandler(cfg)
	h.Server = httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if h.Handler != nil {
					h.Handler.ServeHTTP(w, r)
				}
			},
		),
	)
}

func (h *testHandlerServerAndClient) CloseHandlerAndServer() {
	if h.Handler != nil {
		h.Handler.Close()
		h.Handler = nil
	}
	if h.Server != nil {
		h.Server.Close()
		h.Server = nil
	}
}

func (h *testHandlerServerAndClient) CreateClient() error {
	c, err := NewClientFromURL(h.Server.URL)
	if err != nil {
		return err
	}
	h.Client = c
	return nil
}

func (h *testHandlerServerAndClient) CloseClient() {
	if h.Client != nil {
		h.Client.Close()
		h.Client = nil
	}
}

func TestHandler(t *testing.T) {
	for _, v := range []struct {
		Name   string
		Config *HandlerConfig
		Fn     func(*testHandlerServerAndClient) error
	}{
		{
			Name: "basic event propagation",
			Fn: func(h *testHandlerServerAndClient) error {
				h.Handler.Send(&Event{ID: "1"})
				if err := receiveAtLeastNEvents(1, h.Client, CLIENT_DELAY); err != nil {
					return err
				}
				return nil
			},
		},
		{
			Name: "handle client disconnect",
			Fn: func(h *testHandlerServerAndClient) error {
				h.CloseClient()
				time.Sleep(CLIENT_DELAY)
				return func() error {
					defer h.Handler.mutex.Unlock()
					h.Handler.mutex.Lock()
					if len(h.Handler.eventChans) != 0 {
						return errors.New("client event channel still present")
					}
					return nil
				}()
			},
		},
		{
			Name: "handle server disconnect",
			Fn: func(h *testHandlerServerAndClient) error {
				h.CloseHandlerAndServer()
				return nil
			},
		},
		{
			Name: "send last events",
			Fn: func(h *testHandlerServerAndClient) error {

				// Send the first event and receive it
				h.Handler.Send(&Event{ID: "1", Retry: 2 * CLIENT_DELAY})
				if err := receiveAtLeastNEvents(1, h.Client, CLIENT_DELAY); err != nil {
					return err
				}

				// Disconnect the client and send a message before the client
				// reconnects; it should request and receive the last message
				func() {
					defer h.Handler.mutex.Unlock()
					h.Handler.mutex.Lock()
					for c := range h.Handler.eventChans {
						close(c)
						delete(h.Handler.eventChans, c)
					}
				}()
				time.Sleep(CLIENT_DELAY)
				h.Handler.Send(&Event{})

				// Wait for the client to reconnect and fetch the missed event
				return receiveAtLeastNEvents(1, h.Client, 2*CLIENT_DELAY)
			},
		},
		{
			Name: "use callback functions",
			Config: &HandlerConfig{
				ConnectedFn: func(r *http.Request) any {
					return "1"
				},
				InitFn: func(v any) []*Event {
					return []*Event{{}}
				},
				FilterFn: func(v any) bool {
					return v.(string) == "1"
				},
			},
			Fn: func(h *testHandlerServerAndClient) error {
				h.Handler.Send(&Event{})
				return receiveAtLeastNEvents(2, h.Client, CLIENT_DELAY)
			},
		},
	} {
		func() {

			// Create the handler, server, and client
			h := &testHandlerServerAndClient{}
			h.CreateHandlerAndServer(v.Config)
			defer h.CloseHandlerAndServer()
			if err := h.CreateClient(); err != nil {
				t.Fatalf("%s: %s", v.Name, err)
			}
			defer h.CloseClient()

			// Wait for the client to connect
			time.Sleep(CLIENT_DELAY)

			// Run the test
			if err := v.Fn(h); err != nil {
				t.Fatalf("%s: %s", v.Name, err)
			}
		}()
	}
}
