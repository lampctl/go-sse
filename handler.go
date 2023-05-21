package sse

import (
	"net/http"
	"sync"
)

// HandlerConfig provides a means of passing configuration to NewHandler.
type HandlerConfig struct {

	// NumEventsToKeep indicates the number of events that should be kept for
	// clients reconnecting.
	NumEventsToKeep int

	// ChannelBufferSize indicates how many events should be buffered before
	// the connection is assumed to be dead.
	ChannelBufferSize int

	// ConnectedFn, if provided, is invoked when a client connects. The return
	// value of this function is associated with the client and is passed to
	// InitFn and FilterFn.
	ConnectedFn func(*http.Request) any

	// InitFn, if provided, is invoked right before a client enters the event
	// loop and sends any events that it returns to the client. This is useful,
	// for example, if you are synchronizing application state. The single
	// parameter is equal to the value returned by ConnectedFn.
	InitFn func(any) []*Event

	// FilterFn, if provided, is invoked when an event is being sent to a
	// client to determine if it should actually be sent. The first parameter
	// is equal to the value returned by ConnectedFn and the return value
	// should be set to true to send the event.
	FilterFn func(any, *Event) bool
}

// DefaultHandlerConfig provides a set of defaults.
var DefaultHandlerConfig = &HandlerConfig{
	NumEventsToKeep:   10,
	ChannelBufferSize: 4,
}

// Handler provides an http.Handler that can be used for sending events to any
// number of connected clients.
type Handler struct {
	mutex      sync.Mutex
	waitGroup  sync.WaitGroup
	cfg        *HandlerConfig
	eventQueue []*Event
	eventChans map[chan *Event]any
	isClosed   bool
}

// NewHandler creates a new Handler instance.
func NewHandler(cfg *HandlerConfig) *Handler {
	if cfg == nil {
		cfg = DefaultHandlerConfig
	}
	return &Handler{
		cfg:        cfg,
		eventChans: make(map[chan *Event]any),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Determine if there is a value to associate with this client
	var v any = nil
	if h.cfg.ConnectedFn != nil {
		v = h.cfg.ConnectedFn(r)
	}

	// We need to be able to flush the writer after each chunk
	f, ok := w.(http.Flusher)
	if !ok {
		panic("http.ResponseWriter does not implement http.Flusher")
	}

	// Register the channel
	h.mutex.Lock()
	if h.isClosed {
		h.mutex.Unlock()
		http.Error(
			w,
			http.StatusText(http.StatusServiceUnavailable),
			http.StatusServiceUnavailable,
		)
		return
	}
	h.waitGroup.Add(1)
	defer h.waitGroup.Done()
	eventChan := make(chan *Event, h.cfg.ChannelBufferSize)
	h.eventChans[eventChan] = nil
	h.mutex.Unlock()

	// Write the response headers
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)

	// Make a list of events to send on intialization if requested
	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID != "" {
		events := []*Event{}
		func() {
			defer h.mutex.Unlock()
			h.mutex.Lock()
			lastEventIdx := -1
			for i, e := range h.eventQueue {
				if lastEventID == e.ID {
					lastEventIdx = i
				}
			}
			events = append(events, h.eventQueue[lastEventIdx+1:]...)
		}()
		for _, e := range events {
			if h.cfg.FilterFn == nil || h.cfg.FilterFn(v, e) {
				w.Write(e.Bytes())
			}
		}
		f.Flush()
	}

	// Send messages received from InitFn (if provided)
	if h.cfg.InitFn != nil {
		for _, e := range h.cfg.InitFn(v) {
			w.Write(e.Bytes())
		}
		f.Flush()
	}

	// Write events as they come in
	for {
		select {
		case e, ok := <-eventChan:
			if !ok {
				// The server is shutting down the connection; no need to
				// remove ourselves from the map
				return
			}
			if h.cfg.FilterFn == nil || h.cfg.FilterFn(v, e) {
				w.Write(e.Bytes())
				f.Flush()
			}
		case <-r.Context().Done():
			// Client disconnected, remove this channel from the map
			func() {
				defer h.mutex.Unlock()
				h.mutex.Lock()
				delete(h.eventChans, eventChan)
			}()
			return
		}
	}
}

// Send sends the provided event to all connected clients. Any clients that
// block are forcibly disconnected.
func (h *Handler) Send(e *Event) {
	defer h.mutex.Unlock()
	h.mutex.Lock()
	for c := range h.eventChans {
		select {
		case c <- e:
		default:
			close(c)
			delete(h.eventChans, c)
		}
	}
	h.eventQueue = append(h.eventQueue, e)
	if len(h.eventQueue) > h.cfg.NumEventsToKeep {
		h.eventQueue = h.eventQueue[1:]
	}
}

// Close shuts down all of the event channels and waits for them to complete.
func (h *Handler) Close() {
	h.mutex.Lock()
	for c := range h.eventChans {
		close(c)
	}
	h.isClosed = true
	h.mutex.Unlock()
	h.waitGroup.Wait()
}
