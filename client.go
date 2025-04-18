package sse

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// ClientConfig provides a means of passing configuration to NewClient.
type ClientConfig struct {

	// Request is used as the basis for requests to the SSE endpoint. At a
	// minimum, it should have the HTTP method and path set.
	Request *http.Request

	// Client is used for initiating the connection. If set to nil,
	// http.DefaultClient is used.
	Client *http.Client

	// Context is used for shutting down the client when Close is called. If
	// set to nil, context.Background() is used.
	Context context.Context
}

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
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
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

// NewClient creates a new SSE client instance. The client will immediately
// begin connecting to the server and continue sending events until an HTTP
// 204 is received or explicitly terminated with close or by cancelling the
// the provided context.
func NewClient(cfg *ClientConfig) *Client {
	var (
		client   = cfg.Client
		ctxParam = cfg.Context
	)
	if client == nil {
		client = http.DefaultClient
	}
	if ctxParam == nil {
		ctxParam = context.Background()
	}
	var (
		ctx, cancel = context.WithCancel(ctxParam)
		eventChan   = make(chan *Event)
		closedChan  = make(chan any)
		c           = &Client{
			Events:           eventChan,
			req:              cfg.Request,
			client:           client,
			reconnectionTime: defaultReconnectionTime,
			cancel:           cancel,
			closedChan:       closedChan,
		}
	)
	go c.lifecycleLoop(ctx, eventChan, closedChan)
	return c
}

// NewClientFromURL creates a new SSE client for the provided URL. By default,
// GET request is made by http.DefaultClient.
func NewClientFromURL(url string) (*Client, error) {
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return NewClient(&ClientConfig{
		Request: r,
	}), nil
}

// Close disconnects and shuts down the client.
func (c *Client) Close() {
	c.cancel()
	<-c.closedChan
}
