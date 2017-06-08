 A Message Bus server and library written in Go

**WIP:** THis is still highly experimental and not fit for production use.

## Install

```#!bash
$ go install github.com/prologic/msgbus/...
```

## Usage

Run the message bus daemon/server:

```#!bash
$ msgbusd
```

Subscribe to a topic using the message bus client:

```#!bash
$ msgbus sub foo
2017/06/07 21:52:27 Listening for messages from ws://localhost:8000/push/foo
2017/06/07 21:52:36 Received: hello
2017/06/07 21:52:50 Received: hello
^C
```

Send a few messages with the message bus client:

```#!bash
$ msgbus pub foo hello
$ msgbus pub foo hello
```

## Design

Design decisions so far:

* In memory queues
* HTTP API
* Websockets for realtime push of events

Enjoy :)
