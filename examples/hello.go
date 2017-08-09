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
		log.Printf(
			"Received message: id=%s topic=%s payload=%s",
			msg.ID, msg.Topic, msg.Payload,
		)
	}
}
