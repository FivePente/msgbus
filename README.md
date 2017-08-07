# msgbus

A distirbuted, scalable Message Bus server and library written in Go

**WIP:** THis is still highly experimental and not fit for production use.

**Update:** (*2017-08-07*) This is now being used by [autodock](https://github.com/prologic/autodock) and is undergoing heavy development to deliver what is laid out here.

## Install

```#!bash
$ go install github.com/prologic/msgbus/...
```

## Usage

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

## Design

Design decisions so far:

* In memory queues (*may extend this with interfaces and persistence*)
* HTTP API
* Websockets for realtime push of events
* Sequence ID Message tracking
* Pull and Push model

## License

msgbus is licensed under the [MIT License](https://github.com/prologic/msgbus/blob/master/LICENSE)
