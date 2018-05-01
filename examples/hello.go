package main

import (
	"log"

	"github.com/prologic/msgbus"
)

func main() {
	m := msgbus.NewMessageBus(nil)
	t := m.NewTopic("foo")
	m.Put(m.NewMessage(t, []byte("Hello World!")))

	msg, ok := m.Get(t)
	if !ok {
		log.Printf("No more messages in queue: foo")
		return
	}

	log.Printf(
		"Received message: id=%d topic=%s payload=%s",
		msg.ID, msg.Topic.Name, msg.Payload,
	)
}
