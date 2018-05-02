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

	"github.com/gorilla/websocket"
)

const (
	// DefaultTTL is the default TTL (time to live) for newly created topics
	DefaultTTL = 60 * time.Second
)

// TODO: Make this configurable?
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// HandlerFunc ...
type HandlerFunc func(msg *Message) error

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
	DefaultTTL  time.Duration
	WithMetrics bool
}

// MessageBus ...
type MessageBus struct {
	sync.Mutex

	metrics *Metrics

	ttl       time.Duration
	topics    map[string]*Topic
	queues    map[*Topic]*Queue
	listeners map[*Topic]*Listeners
}

// NewMessageBus ...
func NewMessageBus(options *Options) *MessageBus {
	var (
		ttl         time.Duration
		withMetrics bool
	)

	if options != nil {
		ttl = options.DefaultTTL
		withMetrics = options.WithMetrics
	} else {
		ttl = DefaultTTL
		withMetrics = false
	}

	var metrics *Metrics

	if withMetrics {
		metrics = NewMetrics("msgbus")

		ctime := time.Now()

		// server uptime counter
		metrics.NewCounterFunc(
			"server", "uptime",
			"Number of nanoseconds the server has been running",
			func() float64 {
				return float64(time.Since(ctime).Nanoseconds())
			},
		)

		// server requests counter
		metrics.NewCounter(
			"http", "requests",
			"Number of total HTTP requests processed",
		)

		// bus messages counter
		metrics.NewCounter(
			"bus", "messages",
			"Number of total messages exchanged",
		)

		// bus dropped counter
		metrics.NewCounter(
			"bus", "dropped",
			"Number of messages dropped to subscribers",
		)

		// bus delivered counter
		metrics.NewCounter(
			"bus", "delivered",
			"Number of messages delivered to subscribers",
		)

		// bus fetched counter
		metrics.NewCounter(
			"bus", "fetched",
			"Number of messages fetched from clients",
		)

		// bus topics gauge
		metrics.NewCounter(
			"bus", "topics",
			"Number of active topics registered",
		)

		// bus subscribers gauge
		metrics.NewGauge(
			"bus", "subscribers",
			"Number of active subscribers",
		)
	}

	return &MessageBus{
		metrics: metrics,

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

// Metrics ...
func (mb *MessageBus) Metrics() *Metrics {
	return mb.metrics
}

// NewTopic ...
func (mb *MessageBus) NewTopic(topic string) *Topic {
	mb.Lock()
	defer mb.Unlock()

	t, ok := mb.topics[topic]
	if !ok {
		t = &Topic{Name: topic, TTL: mb.ttl, Created: time.Now()}
		mb.topics[topic] = t
		if mb.metrics != nil {
			mb.metrics.Counter("bus", "topics").Inc()
		}
	}
	return t
}

// NewMessage ...
func (mb *MessageBus) NewMessage(topic *Topic, payload []byte) Message {
	defer func() {
		topic.Sequence++
		if mb.metrics != nil {
			mb.metrics.Counter("bus", "messages").Inc()
		}
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

	if mb.metrics != nil {
		mb.metrics.Counter("bus", "fetched").Inc()
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
	defer func() {
		if mb.metrics != nil {
			mb.metrics.Gauge("bus", "subscribers").Inc()
		}
	}()

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
	defer func() {
		if mb.metrics != nil {
			mb.metrics.Gauge("bus", "subscribers").Dec()
		}
	}()

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
	defer func() {
		if mb.metrics != nil {
			mb.metrics.Counter("http", "requests").Inc()
		}
	}()

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
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Errorf("error creating websocket client: %s", err)
				return
			}

			NewClient(conn, t, mb).Start()
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
		// TODO: Implement deleting topics
	}
}

// Client ...
type Client struct {
	conn  *websocket.Conn
	topic *Topic
	bus   *MessageBus

	id string
	ch chan Message
}

// NewClient ...
func NewClient(conn *websocket.Conn, topic *Topic, bus *MessageBus) *Client {
	return &Client{conn: conn, topic: topic, bus: bus}
}

// Start ...
func (c *Client) Start() {
	c.id = c.conn.RemoteAddr().String()
	c.ch = c.bus.Subscribe(c.id, c.topic.Name)
	defer func() {
		c.bus.Unsubscribe(c.id, c.topic.Name)
	}()

	var err error

	for {
		msg := <-c.ch
		c.conn.SetWriteDeadline(time.Now().Add(time.Second * 1))
		err = c.conn.WriteJSON(msg)
		if err != nil {
			// TODO: Retry? Put the message back in the queue?
			log.Errorf("Error sending msg to %s", c.id)
			if c.bus.metrics != nil {
				c.bus.metrics.Counter("bus", "dropped").Inc()
			}
		} else {
			if c.bus.metrics != nil {
				c.bus.metrics.Counter("bus", "delivered").Inc()
			}
		}
	}
}
