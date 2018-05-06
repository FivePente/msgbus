package msgbus

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/websocket"
)

const (
	// DefaultTTL is the default TTL (time to live) for newly created topics
	DefaultTTL = 60 * time.Second

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 2048
)

// TODO: Make this configurable?
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
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

// Length ...
func (ls *Listeners) Length() int {
	return len(ls.ids)
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
func (ls *Listeners) NotifyAll(message Message) int {
	i := 0
	for id, ch := range ls.chs {
		select {
		case ch <- message:
			log.Debugf("successfully published message to %s: %#v", id, message)
			i++
		default:
			// TODO: Drop this client?
			// TODO: Retry later?
			log.Warnf("cannot publish message to %s: %#v", id, message)
		}
	}

	return i
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
			"server", "requests",
			"Number of total requests processed",
		)

		// client latency summary
		metrics.NewSummary(
			"client", "latency_seconds",
			"Client latency in seconds",
		)

		// client errors counter
		metrics.NewCounter(
			"client", "errors",
			"Number of errors publishing messages to clients",
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

	n := ls.NotifyAll(message)
	if n != ls.Length() && mb.metrics != nil {
		log.Warnf("%d/%d subscribers notified", n, ls.Length())
		mb.metrics.Counter("bus", "dropped").Inc()
	}
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
			mb.metrics.Counter("server", "requests").Inc()
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

func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	c.conn.SetPongHandler(func(message string) error {
		log.Debugf("recieved pong from %s: %s", c.id, message)
		t, err := strconv.ParseInt(message, 10, 64)
		d := time.Duration(time.Now().UnixNano() - t)
		if err != nil {
			log.Warnf("garbage pong reply from %s: %s", c.id, err)
		} else {
			log.Debugf("pong latency of %s: %s", c.id, d)
		}
		c.conn.SetReadDeadline(time.Now().Add(pongWait))

		if c.bus.metrics != nil {
			v := c.bus.metrics.Summary("client", "latency_seconds")
			v.Observe(d.Seconds())
		}

		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
				log.Errorf("unexpected close error from %s: %s", c.id, err)
				c.bus.Unsubscribe(c.id, c.topic.Name)
			}
			log.Errorf("error reading from %s: %s", c.id, err)
			break
		}
		log.Debugf("recieved message from %s: %s", c.id, message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	var err error

	for {
		select {
		case msg, ok := <-c.ch:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The bus closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err = c.conn.WriteJSON(msg)
			if err != nil {
				// TODO: Retry? Put the message back in the queue?
				log.Errorf("Error sending msg to %s: %s", c.id, err)
				if c.bus.metrics != nil {
					c.bus.metrics.Counter("client", "errors").Inc()
				}
			} else {
				if c.bus.metrics != nil {
					c.bus.metrics.Counter("bus", "delivered").Inc()
				}
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			t := time.Now()
			message := []byte(fmt.Sprintf("%d", t.UnixNano()))
			if err := c.conn.WriteMessage(websocket.PingMessage, message); err != nil {
				log.Errorf("error sending ping to %s: %s", c.id, err)
				return
			}
		}
	}
}

// Start ...
func (c *Client) Start() {
	c.id = c.conn.RemoteAddr().String()
	c.ch = c.bus.Subscribe(c.id, c.topic.Name)

	c.conn.SetCloseHandler(func(code int, text string) error {
		log.Debugf("recieved close from client %s", c.id)
		c.bus.Unsubscribe(c.id, c.topic.Name)
		message := websocket.FormatCloseMessage(code, "")
		c.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second*1))
		return nil
	})

	go c.writePump()
	go c.readPump()
}
