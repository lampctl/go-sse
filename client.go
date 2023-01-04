package sse

import (
	"context"
	"errors"
	"net/http"
	"time"
)

const defaultReconnectionTime = time.Second * 3

var errConnectionClosed = errors.New("connection was closed by the server")

// Client connects to a server providing SSE. The client will continue to
// maintain the connection, resuming from the last event ID when disconnected.
type Client struct {

	// Events provides a stream of events from the server.
	Events <-chan *Event

	req              *http.Request
	client           *http.Client
	lastEventID      string
	reconnectionTime time.Duration
	cancel           context.CancelFunc
	closedChan       <-chan any
}

func (c *Client) connectionLoop(
	ctx context.Context,
	eventChan chan<- *Event,
) error {
	req := c.req.Clone(ctx)
	if len(c.lastEventID) != 0 {
		req.Header.Set("Last-Event-ID", c.lastEventID)
	}
	r, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode == http.StatusNoContent {
		return nil
	}
	reader := NewReader(r.Body)
	reader.LastEventID = c.lastEventID
	defer func() {
		c.lastEventID = reader.LastEventID
		if reader.ReconnectionTime != 0 {
			c.reconnectionTime = time.Millisecond *
				time.Duration(reader.ReconnectionTime)
		}
	}()
	for {
		e, err := reader.NextEvent()
		if err != nil {
			return err
		}
		if e == nil {
			return errConnectionClosed
		}
		select {
		case eventChan <- e:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Client) lifecycleLoop(
	ctx context.Context,
	eventChan chan<- *Event,
	closedChan chan<- any,
) {
	defer close(closedChan)
	defer close(eventChan)
	for {
		if err := c.connectionLoop(ctx, eventChan); err == nil {
			return
		}
		select {
		case <-time.After(c.reconnectionTime):
		case <-ctx.Done():
			return
		}
	}
}

// NewClient creates a new SSE client from the provided parameters. If client
// is set to nil, http.DefaultClient will be used. The client will begin
// connecting to the server and continue sending events until an HTTP 204 is
// received or explicitly terminated with Close().
func NewClient(req *http.Request, client *http.Client) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	var (
		ctx, cancel = context.WithCancel(context.Background())
		eventChan   = make(chan *Event)
		closedChan  = make(chan any)
		c           = &Client{
			Events:           eventChan,
			req:              req,
			client:           client,
			reconnectionTime: defaultReconnectionTime,
			cancel:           cancel,
			closedChan:       closedChan,
		}
	)
	go c.lifecycleLoop(ctx, eventChan, closedChan)
	return c
}

// NewClientFromURL creates a new SSE client for the provided URL and uses
// http.DefaultClient to send the requests.
func NewClientFromURL(url string) (*Client, error) {
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return NewClient(r, nil), nil
}

// Close disconnects and shuts down the client.
func (c *Client) Close() {
	c.cancel()
	<-c.closedChan
}
