package msgbus

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"golang.org/x/net/websocket"
)

const (
	// DefaultTTL is the default TTL (time to live) for newly created topics
	DefaultTTL = 60 * time.Second
)

// Topic ...
type Topic struct {
	Name     string        `json:"name"`
	TTL      time.Duration `json:"ttl"`
	Sequence uint64        `json:"seq"`
	Created  time.Time     `json:"created"`
}

// Message ...
type Message struct {
	ID      uint64    `json:"id"`
	Topic   *Topic    `json:"topic"`
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

// Options ...
type Options struct {
	DefaultTTL time.Duration
}

// MessageBus ...
type MessageBus struct {
	sync.Mutex

	ttl       time.Duration
	topics    map[string]*Topic
	queues    map[*Topic]*Queue
	listeners map[*Topic]*Listeners
}

// NewMessageBus ...
func NewMessageBus(options *Options) *MessageBus {
	var ttl time.Duration

	if options != nil {
		ttl = options.DefaultTTL
	} else {
		ttl = DefaultTTL
	}

	return &MessageBus{
		ttl:       ttl,
		topics:    make(map[string]*Topic),
		queues:    make(map[*Topic]*Queue),
		listeners: make(map[*Topic]*Listeners),
	}
}

// Len ...
func (mb *MessageBus) Len() int {
	return len(mb.topics)
}

// NewTopic ...
func (mb *MessageBus) NewTopic(topic string) *Topic {
	mb.Lock()
	defer mb.Unlock()

	t, ok := mb.topics[topic]
	if !ok {
		t = &Topic{Name: topic, TTL: mb.ttl, Created: time.Now()}
		mb.topics[topic] = t
	}
	return t
}

// NewMessage ...
func (mb *MessageBus) NewMessage(topic *Topic, payload []byte) Message {
	defer func() {
		topic.Sequence++
	}()

	return Message{
		ID:      topic.Sequence,
		Topic:   topic,
		Payload: payload,
		Created: time.Now(),
	}
}

// Put ...
func (mb *MessageBus) Put(message Message) {
	log.Debugf(
		"[msgbus] PUT id=%d topic=%s payload=%s",
		message.ID, message.Topic.Name, message.Payload,
	)

	q, ok := mb.queues[message.Topic]
	if !ok {
		q = &Queue{}
		mb.queues[message.Topic] = q
	}
	q.Push(message)

	mb.NotifyAll(message)
}

// Get ...
func (mb *MessageBus) Get(topic *Topic) (Message, bool) {
	log.Debugf("[msgbus] GET topic=%s", topic)

	q, ok := mb.queues[topic]
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
func (mb *MessageBus) NotifyAll(message Message) {
	log.Debugf(
		"[msgbus] NotifyAll id=%d topic=%s payload=%s",
		message.ID, message.Topic.Name, message.Payload,
	)
	ls, ok := mb.listeners[message.Topic]
	if !ok {
		return
	}
	ls.NotifyAll(message)
}

// Subscribe ...
func (mb *MessageBus) Subscribe(id, topic string) chan Message {
	log.Debugf("[msgbus] Subscribe id=%s topic=%s", id, topic)
	t, ok := mb.topics[topic]
	if !ok {
		t = &Topic{Name: topic, TTL: mb.ttl, Created: time.Now()}
		mb.topics[topic] = t
	}

	ls, ok := mb.listeners[t]
	if !ok {
		ls = NewListeners()
		mb.listeners[t] = ls
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
	log.Debugf("[msgbus] Unsubscribe id=%s topic=%s", id, topic)
	t, ok := mb.topics[topic]
	if !ok {
		return
	}

	ls, ok := mb.listeners[t]
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
		// XXX: guard with a mutex?
		out, err := json.Marshal(mb.topics)
		if err != nil {
			msg := fmt.Sprintf("error serializing topics: %s", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
		return
	}

	topic := strings.TrimLeft(r.URL.Path, "/")
	topic = strings.TrimRight(topic, "/")

	t := mb.NewTopic(topic)

	switch r.Method {
	case "POST", "PUT":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := fmt.Sprintf("error reading payload: %s", err)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		message := mb.NewMessage(t, body)
		mb.Put(message)

		msg := fmt.Sprintf(
			"message successfully published to %s with sequence %d",
			t.Name, t.Sequence,
		)
		w.Write([]byte(msg))
	case "GET":
		if r.Header.Get("Upgrade") == "websocket" {
			NewClient(t, mb).Handler().ServeHTTP(w, r)
			return
		}

		message, ok := mb.Get(t)

		if !ok {
			msg := fmt.Sprintf("no messages enqueued for topic: %s", topic)
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		out, err := json.Marshal(message)
		if err != nil {
			msg := fmt.Sprintf("error serializing message: %s", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	case "DELETE":
		http.Error(w, "Not Implemented", http.StatusNotImplemented)
	}
}

// Client ...
type Client struct {
	topic *Topic
	bus   *MessageBus
	id    string
	ch    chan Message
}

// NewClient ...
func NewClient(topic *Topic, bus *MessageBus) *Client {
	return &Client{topic: topic, bus: bus}
}

// Handler ...
func (c *Client) Handler() websocket.Handler {
	return func(conn *websocket.Conn) {
		c.id = conn.Request().RemoteAddr
		c.ch = c.bus.Subscribe(c.id, c.topic.Name)
		defer func() {
			c.bus.Unsubscribe(c.id, c.topic.Name)
		}()

		var err error

		for {
			msg := <-c.ch
			err = websocket.JSON.Send(conn, msg)
			if err != nil {
				// TODO: Retry? Put the message back in the queue?
				log.Errorf("Error sending msg to %s", c.id)
				continue
			}
		}
	}
}
