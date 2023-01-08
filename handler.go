package sse

import (
	"net/http"
	"sync"
)

// Handler provides an http.Handler that can be used for sending events.
type Handler struct {
	mutex      sync.Mutex
	waitGroup  sync.WaitGroup
	eventChans []chan<- *Event
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer h.waitGroup.Done()
	h.waitGroup.Add(1)
	eventChan := make(chan *Event)
	func() {
		defer h.mutex.Unlock()
		h.mutex.Lock()
		h.eventChans = append(h.eventChans, eventChan)
	}()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/event-stream")
	for e := range eventChan {

		// TODO: process event
		_ = e
	}
}

// Close shuts down all of the event channels and waits for them to complete.
func (h *Handler) Close() {
	defer h.mutex.Unlock()
	h.mutex.Lock()
	if h.eventChans != nil {
		for _, c := range h.eventChans {
			close(c)
		}
		h.waitGroup.Wait()
		h.eventChans = nil
	}
}
