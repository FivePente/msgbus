package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jpillora/backoff"
	log "github.com/sirupsen/logrus"

	"github.com/prologic/msgbus"
)

const (
	// DefaultReconnectInterval ...
	DefaultReconnectInterval = 2

	// DefaultMaxReconnectInterval ...
	DefaultMaxReconnectInterval = 64

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 2048
)

var (
	// PublishedRegexp ...
	PublishedRegexp = regexp.MustCompile(
		"message successfully published to \\w+ with sequence \\d",
	)
)

// Client ...
type Client struct {
	url string

	reconnectInterval    time.Duration
	maxReconnectInterval time.Duration
}

// Options ...
type Options struct {
	ReconnectInterval    int
	MaxReconnectInterval int
}

// NewClient ...
func NewClient(url string, options *Options) *Client {
	var (
		reconnectInterval    = DefaultReconnectInterval
		maxReconnectInterval = DefaultMaxReconnectInterval
	)

	url = strings.TrimSuffix(url, "/")

	client := &Client{url: url}

	if options != nil {
		if options.ReconnectInterval != 0 {
			reconnectInterval = options.ReconnectInterval
		}

		if options.MaxReconnectInterval != 0 {
			maxReconnectInterval = options.MaxReconnectInterval
		}
	}

	client.reconnectInterval = time.Duration(reconnectInterval) * time.Second
	client.maxReconnectInterval = time.Duration(maxReconnectInterval) * time.Second

	return client
}

// Handle ...
func (c *Client) Handle(msg *msgbus.Message) error {
	out, err := json.Marshal(msg)
	if err != nil {
		log.Errorf("error marshalling message: %s", err)
		return err
	}

	os.Stdout.Write(out)
	os.Stdout.Write([]byte{'\r', '\n'})
	return nil
}

// Pull ...
func (c *Client) Pull(topic string) (msg *msgbus.Message, err error) {
	url := fmt.Sprintf("%s/%s", c.url, topic)
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("error constructing request to %s: %s", url, err)
		return
	}

	res, err := client.Do(req)
	if err != nil {
		log.Errorf("error sending request to %s: %s", url, err)
		return
	}

	if res.StatusCode == http.StatusNotFound {
		// Empty queue
		return
	}

	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&msg)
	if err != nil {
		log.Errorf(
			"error decoding response from %s for %s: %s",
			url, topic, err,
		)
		return
	}
	err = c.Handle(msg)
	if err != nil {
		log.Errorf(
			"error handling message from %s for %s: %s",
			url, topic, err,
		)
		return
	}

	return
}

// Publish ...
func (c *Client) Publish(topic, message string) error {
	var payload bytes.Buffer

	payload.Write([]byte(message))

	url := fmt.Sprintf("%s/%s", c.url, topic)

	client := &http.Client{}

	req, err := http.NewRequest("PUT", url, &payload)
	if err != nil {
		return fmt.Errorf("error constructing request: %s", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error publishing message: %s", err)
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("unexpected non-200 response: %s", res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %s", err)
	}

	if !PublishedRegexp.Match(body) {
		return fmt.Errorf("unexpected non-matching response: %s", body)
	}

	return nil
}

// Subscribe ...
func (c *Client) Subscribe(topic string, handler msgbus.HandlerFunc) *Subscriber {
	return NewSubscriber(c, topic, handler)
}

// Subscriber ...
type Subscriber struct {
	sync.RWMutex

	conn *websocket.Conn

	client *Client

	topic   string
	handler msgbus.HandlerFunc

	url                  string
	reconnectInterval    time.Duration
	maxReconnectInterval time.Duration
}

// NewSubscriber ...
func NewSubscriber(client *Client, topic string, handler msgbus.HandlerFunc) *Subscriber {
	if handler == nil {
		handler = client.Handle
	}

	u, err := url.Parse(client.url)
	if err != nil {
		log.Fatal("invalid url: %s", client.url)
	}

	if strings.HasPrefix(client.url, "https") {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}

	u.Path += fmt.Sprintf("/%s", topic)

	url := u.String()

	return &Subscriber{
		client:  client,
		topic:   topic,
		handler: handler,

		url:                  url,
		reconnectInterval:    client.reconnectInterval,
		maxReconnectInterval: client.maxReconnectInterval,
	}
}

func (s *Subscriber) closeAndReconnect() {
	s.conn.Close()
	go s.connect()
}

func (s *Subscriber) connect() {
	b := &backoff.Backoff{
		Min:    s.reconnectInterval,
		Max:    s.maxReconnectInterval,
		Factor: 2,
		Jitter: false,
	}

	for {
		d := b.Duration()

		conn, _, err := websocket.DefaultDialer.Dial(s.url, nil)

		if err != nil {
			log.Warnf("error connecting to %s: %s", s.url, err)
			log.Infof("reconnecting in %s", d)
			time.Sleep(d)
			continue
		}

		log.Infof("successfully connected to %s", s.url)

		s.Lock()
		s.conn = conn
		s.Unlock()

		go s.readLoop()
		go s.writeLoop()

		break
	}
}

func (s *Subscriber) readLoop() {
	var msg *msgbus.Message

	s.conn.SetReadLimit(maxMessageSize)
	s.conn.SetReadDeadline(time.Now().Add(pongWait))

	s.conn.SetPongHandler(func(message string) error {
		log.Debugf("recieved pong from %s: %s", s.url, message)
		t, err := strconv.ParseInt(message, 10, 64)
		d := time.Duration(time.Now().UnixNano() - t)
		if err != nil {
			log.Warnf("garbage pong reply from %s: %s", s.url, err)
		} else {
			log.Debugf("pong latency of %s: %s", s.url, d)
		}
		s.conn.SetReadDeadline(time.Now().Add(pongWait))

		return nil
	})

	for {
		err := s.conn.ReadJSON(&msg)
		if err != nil {
			log.Errorf("error reading from %s: %s", s.url, err)
			s.closeAndReconnect()
			return
		}

		err = s.handler(msg)
		if err != nil {
			log.Warnf("error handling message: %s", err)
		}
	}
}

func (s *Subscriber) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		if s.conn != nil {
			s.conn.Close()
		}
	}()

	for {
		<-ticker.C
		s.conn.SetWriteDeadline(time.Now().Add(writeWait))
		t := time.Now()
		message := []byte(fmt.Sprintf("%d", t.UnixNano()))
		if err := s.conn.WriteMessage(websocket.PingMessage, message); err != nil {
			log.Errorf("error sending ping to %s: %s", s.url, err)
			s.closeAndReconnect()
			return
		}
	}
}

// Start ...
func (s *Subscriber) Start() {
	go s.connect()
}

// Stop ...
func (s *Subscriber) Stop() {
	log.Infof("shutting down ...")

	err := s.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Warnf("error sending close message: %s", err)
	}

	err = s.conn.Close()
	if err != nil {
		log.Warnf("error closing connection: %s", err)
	}

	s.conn = nil
}
