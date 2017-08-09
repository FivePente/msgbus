package msgbus

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

// Message ...
type Message struct {
	ID      uint64    `json:"id"`
	Topic   string    `json:"topic"`
	Payload []byte    `json:"payload"`
	Created time.Time `json:"created"`
}

// Listeners ...
type Listeners struct {
	ids map[string]bool
	chs map[string]chan Message
}

// NewListeners ...
func NewListeners() *Listeners {
	return &Listeners{
		ids: make(map[string]bool),
		chs: make(map[string]chan Message),
	}
}

// Add ...
func (ls *Listeners) Add(id string) chan Message {
	ls.ids[id] = true
	ls.chs[id] = make(chan Message)
	return ls.chs[id]
}

// Remove ...
func (ls *Listeners) Remove(id string) {
	delete(ls.ids, id)

	close(ls.chs[id])
	delete(ls.chs, id)
}

// Exists ...
func (ls *Listeners) Exists(id string) bool {
	_, ok := ls.ids[id]
	return ok
}

// Get ...
func (ls *Listeners) Get(id string) (chan Message, bool) {
	ch, ok := ls.chs[id]
	if !ok {
		return nil, false
	}
	return ch, true
}

// NotifyAll ...
func (ls *Listeners) NotifyAll(message Message) {
	for _, ch := range ls.chs {
		ch <- message
	}
}

// MessageBus ...
type MessageBus struct {
	seqid     uint64
	topics    map[string]*Queue
	listeners map[string]*Listeners
}

// NewMessageBus ...
func NewMessageBus() *MessageBus {
	return &MessageBus{
		topics:    make(map[string]*Queue),
		listeners: make(map[string]*Listeners),
	}
}

// Len ...
func (mb *MessageBus) Len() int {
	return len(mb.topics)
}

// NewMessage ...
func (mb *MessageBus) NewMessage(payload []byte) Message {
	message := Message{
		ID:      mb.seqid,
		Payload: payload,
		Created: time.Now(),
	}

	mb.seqid++

	return message
}

// Put ...
func (mb *MessageBus) Put(topic string, message Message) {
	message.Topic = topic

	log.Printf(
		"[msgbus] PUT id=%d topic=%s payload=%s",
		message.ID, message.Topic, message.Payload,
	)
	q, ok := mb.topics[topic]
	if !ok {
		q = &Queue{}
		mb.topics[topic] = q
	}
	q.Push(message)

	mb.NotifyAll(topic, message)
}

// Get ...
func (mb *MessageBus) Get(topic string) (Message, bool) {
	log.Printf("[msgbus] GET topic=%s", topic)
	q, ok := mb.topics[topic]
	if !ok {
		return Message{}, false
	}

	m := q.Pop()
	if m == nil {
		return Message{}, false
	}
	return m.(Message), true
}

// NotifyAll ...
func (mb *MessageBus) NotifyAll(topic string, message Message) {
	log.Printf(
		"[msgbus] NotifyAll id=%d topic=%s payload=%s",
		message.ID, message.Topic, message.Payload,
	)
	ls, ok := mb.listeners[topic]
	if !ok {
		return
	}
	ls.NotifyAll(message)
}

// Subscribe ...
func (mb *MessageBus) Subscribe(id, topic string) chan Message {
	log.Printf("[msgbus] Subscribe id=%s topic=%s", id, topic)
	ls, ok := mb.listeners[topic]
	if !ok {
		ls = NewListeners()
		mb.listeners[topic] = ls
	}

	if ls.Exists(id) {
		// Already verified th listener exists
		ch, _ := ls.Get(id)
		return ch
	}
	return ls.Add(id)
}

// Unsubscribe ...
func (mb *MessageBus) Unsubscribe(id, topic string) {
	log.Printf("[msgbus] Unsubscribe id=%s topic=%s", id, topic)
	ls, ok := mb.listeners[topic]
	if !ok {
		return
	}

	if ls.Exists(id) {
		// Already verified th listener exists
		ls.Remove(id)
	}
}

func (mb *MessageBus) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" && (r.URL.Path == "/" || r.URL.Path == "") {
		for topic, _ := range mb.topics {
			w.Write([]byte(fmt.Sprintf("%s\n", topic)))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	topic := strings.TrimLeft(r.URL.Path, "/")
	topic = strings.TrimRight(topic, "/")

	switch r.Method {
	case "POST", "PUT":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		mb.Put(topic, mb.NewMessage(body))
	case "GET":
		if r.Header.Get("Upgrade") == "websocket" {
			NewClient(topic, mb).Handler().ServeHTTP(w, r)
			return
		}

		message, ok := mb.Get(topic)

		if !ok {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		out, err := json.Marshal(message)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write(out)
	case "DELETE":
		http.Error(w, "Not Implemented", http.StatusNotImplemented)
	}
}

type Client struct {
	topic string
	bus   *MessageBus
	id    string
	ch    chan Message
}

func NewClient(topic string, bus *MessageBus) *Client {
	return &Client{topic: topic, bus: bus}
}

func (c *Client) Handler() websocket.Handler {
	return func(conn *websocket.Conn) {
		c.id = conn.Request().RemoteAddr
		c.ch = c.bus.Subscribe(c.id, c.topic)
		defer func() {
			c.bus.Unsubscribe(c.id, c.topic)
		}()

		var err error

		for {
			msg := <-c.ch
			err = websocket.JSON.Send(conn, msg)
			if err != nil {
				log.Printf("Error sending msg to %s", c.id)
				continue
			}
		}
	}
}
