# msgbus

[![Build Status](https://travis-ci.org/prologic/msgbus.svg)](https://travis-ci.org/prologic/msgbus)
[![CodeCov](https://codecov.io/gh/prologic/msgbus/branch/master/graph/badge.svg)](https://codecov.io/gh/prologic/msgbus)
[![Go Report Card](https://goreportcard.com/badge/prologic/msgbus)](https://goreportcard.com/report/prologic/msgbus)
[![GoDoc](https://godoc.org/github.com/prologic/msgbus?status.svg)](https://godoc.org/github.com/prologic/msgbus) 

A real-time message bus server and library written in Go with strong
consistency and reliability guarantees.

(*eventual goals of distributed high-availability and sharding*)

## Install

```#!bash
$ go install github.com/prologic/msgbus/...
```

## Usage (library)

Install the package into your project:

```#!bash
$ go get github.com/prologic/msgbus
```

Use the `MessageBus` type either directly:

```#!go
package main

import (
    "log"

    "github.com/prologic/msgbus"
)

func main() {
    m := msgbus.NewMessageBus()
    m.Put("foo", m.NewMessage([]byte("Hello World!")))

    msg, ok := m.Get("foo")
    if !ok {
        log.Printf("No more messages in queue: foo")
    } else {
        log.Printf
	    "Received message: id=%s topic=%s payload=%s",
	    msg.ID, msg.Topic, msg.Payload,
	)
    }
}
```

Running this example should yield something like this:

```#!bash
$ go run examples/hello.go
2017/08/09 03:01:54 [msgbus] PUT id=0 topic=foo payload=Hello World!
2017/08/09 03:01:54 [msgbus] NotifyAll id=0 topic=foo payload=Hello World!
2017/08/09 03:01:54 [msgbus] GET topic=foo
2017/08/09 03:01:54 Received message: id=%!s(uint64=0) topic=foo payload=Hello World!
``` 

See the [godoc](https://godoc.org/github.com/prologic/msgbus) for further
documentation and other examples.

## Usage (tool)

Run the message bus daemon/server:

```#!bash
$ msgbusd
2017/08/07 01:11:16 [msgbus] Subscribe id=[::1]:55341 topic=foo
2017/08/07 01:11:22 [msgbus] PUT id=0 topic=foo payload=hi
2017/08/07 01:11:22 [msgbus] NotifyAll id=0 topic=foo payload=hi
2017/08/07 01:11:26 [msgbus] PUT id=1 topic=foo payload=bye
2017/08/07 01:11:26 [msgbus] NotifyAll id=1 topic=foo payload=bye
2017/08/07 01:11:33 [msgbus] GET topic=foo
2017/08/07 01:11:33 [msgbus] GET topic=foo
2017/08/07 01:11:33 [msgbus] GET topic=foo
```

Subscribe to a topic using the message bus client:

```#!bash
$ msgbus sub foo
2017/08/07 01:11:22 [msgbus] received message: id=0 topic=foo payload=hi
2017/08/07 01:11:26 [msgbus] received message: id=1 topic=foo payload=bye
```

Send a few messages with the message bus client:

```#!bash
$ msgbus pub foo hi
$ msgbus pub foo bye
```

You can also manually pull messages using the client:

```#!bash
$ msgbus pull foo
2017/08/07 01:11:33 [msgbus] received message: id=0 topic=foo payload=hi
2017/08/07 01:11:33 [msgbus] received message: id=1 topic=foo payload=bye
```

> This is slightly different from a listening subscriber (*using websockets*) where messages are pulled directly.

## Usage (HTTP)

Run the message bus daemon/server:

```#!bash
$ msgbusd
2018/03/25 13:21:18 msgbusd listening on :8000
```

Send a message with using `curl`:

```#!bash
$ curl -q -o - -X PUT -d '{"message": "hello"}' http://localhost:8000/hello
```

Pull the messages off the "hello" queue using `curl`:

```#!bash
$ curl -q -o - http://localhost:8000/hello
{"id":0,"topic":{"name":"hello","ttl":60000000000,"seq":1,"created":"2018-03-25T13:18:38.732437-07:00"},"payload":"eyJtZXNzYWdlIjogImhlbGxvIn0=","created":"2018-03-25T13:18:38.732465-07:00"}
```

Decode the payload:

```#!bash
$ echo 'eyJtZXNzYWdlIjogImhlbGxvIn0=' | base64 -d
{"message": "hello"}
```

## API

### GET /

List all known topics/queues.

Example:

```#!bash
$ curl -q -o - http://localhost:8000/
hello
```

## POST|PUT /<topic>

Post a new message to the queue named by `<topic>`.

**NB:** Either `POST` or `PUT` methods can be used here.

Example:

```#!bash
$ curl -q -o - -X PUT -d '{"message": "hello"}' http://localhost:8000/hello
```

## GET /<topic>

Get the next message of the queue named by `<topic>`.

- If the topic is not found. Returns: `404 Not Found`
- If the Websockets `Upgrade` header is found, upgrades to a websocket channel
  and subscribes to the topic `<topic>`. Each new message published to the
  topic `<topic>` are instantly published to all subscribers.

Example:

```#!bash
$ curl -q -o - http://localhost:8000/hello
{"id":0,"topic":{"name":"hello","ttl":60000000000,"seq":1,"created":"2018-03-25T13:18:38.732437-07:00"},"payload":"eyJtZXNzYWdlIjogImhlbGxvIn0=","created":"2018-03-25T13:18:38.732465-07:00"}
```

## DELETE /<topic>

Deletes a queue named by `<topic>`.

*Not implemented*.

## Design

Design decisions so far:

* In memory queues (*may extend this with interfaces and persistence*)
* HTTP API
* Websockets for real-time push of events
* Sequence ID Message tracking
* Pull and Push model

## License

msgbus is licensed under the [MIT License](https://github.com/prologic/msgbus/blob/master/LICENSE)
