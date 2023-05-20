## go-sse

[![Build Status](https://github.com/lampctl/go-sse/actions/workflows/test.yml/badge.svg)](https://github.com/lampctl/go-sse/actions/workflows/test.yml)
[![Coverage Status](https://coveralls.io/repos/github/lampctl/go-sse/badge.svg?branch=main)](https://coveralls.io/github/lampctl/go-sse?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/lampctl/go-sse)](https://goreportcard.com/report/github.com/lampctl/go-sse)
[![Go Reference](https://pkg.go.dev/badge/github.com/lampctl/go-sse.svg)](https://pkg.go.dev/github.com/lampctl/go-sse)
[![MIT License](https://img.shields.io/badge/license-MIT-9370d8.svg?style=flat)](https://opensource.org/licenses/MIT)

This package attempts to provide a robust and reliable implementation of [server-sent events](https://html.spec.whatwg.org/multipage/server-sent-events.html#concept-event-stream-reconnection-time). One might use this package if they were writing an application that needed to connect to an SSE server endpoint and read events in a continuous stream or provide events to a front-end service.

### Features

Here's what you get with this package:

- Complete documentation for every last type and method in the package
- Compliancy with section 9.2 of the WHATWG HTML specification
- Extensive test suite to ensure conformance

go-sse requires a minimum of **Go 1.18**.

### Basic Usage

To use the package in your application, begin by importing it:

```golang
import "github.com/lampctl/go-sse"
```

### Use as a Client

Create a client using:

```golang
c, err := sse.NewClientFromURL("http://example.com/sse")
if err != nil {
    // ...
}
```

You can now read events directly from the `c.Events` channel as they are received:

```golang
for e := range c.Events {
    fmt.Println("Event received!")
    fmt.Println(e.Data)
}
```

> Note that if the connection is closed or interrupted, the client will attempt to reconnect as per the spec and continue returning events from where it was interrupted. This is all handled behind the scenes and won't affect the status of the event channel.

When you are done receiving events, close the client:

```golang
c.Close()
```

### Use as a Server

The server component is provided via `Handler`, which implements `http.Handler`:

```golang
h := sse.NewHandler(nil)

// ...or if you want to customize initialization:
h := sse.NewHandler(&sse.HandlerConfig{
    NumEventsToKeep:   10,
    ChannelBufferSize: 4,
})
```

To send an event, simply use the `Send()` method:

```golang
h.Send(&sse.Event{
    Type: "alert",
    Data: "The aliens are invading!",
    ID:   "12345",
})
```

When you are done, use the `Close()` method:

```golang
h.Close()
```
