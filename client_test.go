package sse

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const CLIENT_DELAY = time.Millisecond * 100

func receiveAtLeastNEvents(
	n int,
	c *Client,
	timeout time.Duration,
) error {
	tCh := time.After(timeout)
	for i := 0; i < n; i++ {
		select {
		case _, ok := <-c.Events:
			if !ok {
				return errors.New("unexpected end of events")
			}
		case <-tCh:
			return fmt.Errorf("expected %d events, received %d", n, i)
		}
	}
	return nil
}

func flushAndWait(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	time.Sleep(CLIENT_DELAY * 2)
}

func TestClient(t *testing.T) {
	for _, v := range []struct {
		Name   string
		Client func(c *Client) error
		Server func(w http.ResponseWriter, r *http.Request, i int) error
	}{
		{
			Name: "Cancel immediately",
			Client: func(c *Client) error {
				return nil
			},
			Server: func(w http.ResponseWriter, r *http.Request, i int) error {
				return nil
			},
		},
		{
			Name: "Cancel while waiting for next event",
			Client: func(c *Client) error {
				time.Sleep(CLIENT_DELAY)
				return nil
			},
			Server: func(w http.ResponseWriter, r *http.Request, i int) error {
				w.Write([]byte(""))
				flushAndWait(w)
				return nil
			},
		},
		{
			Name: "Cancel while sending on channel",
			Client: func(c *Client) error {
				time.Sleep(CLIENT_DELAY)
				c.Close()
				if _, ok := <-c.Events; ok {
					return errors.New("unexpected event received")
				}
				return nil
			},
			Server: func(w http.ResponseWriter, r *http.Request, i int) error {
				w.Write([]byte("data\n\n"))
				flushAndWait(w)
				return nil
			},
		},
		{
			Name: "Send two events",
			Client: func(c *Client) error {
				return receiveAtLeastNEvents(2, c, CLIENT_DELAY)
			},
			Server: func(w http.ResponseWriter, r *http.Request, i int) error {
				w.Write([]byte("data\n\ndata\n\n"))
				return nil
			},
		},
		{
			Name: "Use last ID",
			Client: func(c *Client) error {
				return receiveAtLeastNEvents(2, c, CLIENT_DELAY)
			},
			Server: func(w http.ResponseWriter, r *http.Request, i int) error {
				const eventID = "1"
				if i > 0 {
					lastEventID := r.Header.Get("Last-Event-ID")
					if lastEventID != eventID {
						w.WriteHeader(http.StatusNoContent)
						return fmt.Errorf("%#v != %#v", lastEventID, eventID)
					}
				}
				w.Write([]byte("id:1\nretry:50\ndata\n\n"))
				return nil
			},
		},
		{
			Name: "Reply with 204",
			Client: func(c *Client) error {
				<-c.Events
				_, ok := <-c.Events
				if ok {
					return errors.New("expected channel close")
				}
				return nil
			},
			Server: func(w http.ResponseWriter, r *http.Request, i int) error {
				if i > 0 {
					w.WriteHeader(http.StatusNoContent)
				} else {
					w.Write([]byte("retry:50\ndata\n\n"))
				}
				return nil
			},
		},
	} {
		func() {
			var (
				i         = 0
				serverErr error
			)
			s := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					if err := v.Server(w, r, i); err != nil {
						serverErr = err
					}
					i += 1
				},
			))
			defer func() {
				s.Close()
				if serverErr != nil {
					t.Fatalf("%s: %s", v.Name, serverErr)
				}
			}()
			cfg, err := NewClientConfigFromURL(s.URL)
			if err != nil {
				t.Fatalf("%s: %s", v.Name, err)
			}
			c := NewClient(cfg)
			defer c.Close()
			if err := v.Client(c); err != nil {
				t.Fatalf("%s: %s", v.Name, err)
			}
		}()
	}
}
