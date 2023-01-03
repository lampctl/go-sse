package sse

import (
	"context"
	"net/http"
	"time"
)

// Client connects to a server providing SSE. The client will continue to
// maintain the connection, reconnecting if it is lost.
type Client struct {

	// Events provides a stream of events from the server.
	Events <-chan *Event

	req        *http.Request
	client     *http.Client
	cancel     context.CancelFunc
	closedChan <-chan any
}

func (c *Client) connectionLoop(
	ctx context.Context,
	eventChan chan<- *Event,
) error {
	r, err := c.client.Do(c.req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	reader := NewReader(r.Body)
	for {
		e, err := reader.NextEvent()
		if err != nil {
			return err
		}
		if e == nil {
			return nil
		}
		select {
		case eventChan <- e:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// TODO: use the retry value for time.After

func (c *Client) lifecycleLoop(
	ctx context.Context,
	eventChan chan<- *Event,
	closedChan chan<- any,
) {
	defer close(closedChan)
	defer close(eventChan)
	for {
		err := c.connectionLoop(ctx, eventChan)
		if err != nil {
			select {
			case <-time.After(time.Second * 3):
			case <-ctx.Done():
				return
			}
		}
	}
}

// NewClient creates a new SSE client from the provided parameters. If client
// is set to nil, http.DefaultClient will be used.
func NewClient(req *http.Request, client *http.Client) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	var (
		ctx, cancel = context.WithCancel(context.Background())
		eventChan   = make(chan *Event)
		closedChan  = make(chan any)
		c           = &Client{
			Events:     eventChan,
			req:        req,
			client:     client,
			cancel:     cancel,
			closedChan: closedChan,
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
