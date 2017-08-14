package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/net/websocket"

	"github.com/prologic/msgbus"
)

const (
	// DefaultReconnectInterval ...
	DefaultReconnectInterval = 5

	// DefaultRetryInterval ...
	DefaultRetryInterval = 5
)

// Client ...
type Client struct {
	host string
	port int

	retry     time.Duration
	reconnect time.Duration

	ws *websocket.Conn
}

// Options ...
type Options struct {
	ReconnectInterval int
	RetryInterval     int
}

// NewClient ...
func NewClient(host string, port int, options *Options) *Client {
	var (
		reconnectInterval = DefaultReconnectInterval
		retryInterval     = DefaultRetryInterval
	)

	client := &Client{
		host: host,
		port: port,
	}

	if options != nil {
		if options.ReconnectInterval != 0 {
			reconnectInterval = options.ReconnectInterval
		}

		if options.RetryInterval != 0 {
			retryInterval = options.RetryInterval
		}
	}

	client.reconnect = time.Duration(reconnectInterval) * time.Second
	client.retry = time.Duration(retryInterval) * time.Second

	return client
}

// Handle ...
func (c *Client) Handle(msg *msgbus.Message) {
	log.Printf(
		"[msgbus] received message: id=%d topic=%s payload=%s",
		msg.ID, msg.Topic.Name, msg.Payload,
	)
}

// Pull ...
func (c *Client) Pull(topic string) {
	var msg *msgbus.Message

	url := fmt.Sprintf("http://%s:%d/%s", c.host, c.port, topic)
	client := &http.Client{}

	for {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("error constructing pull request to %s: %s", url, err)
			time.Sleep(c.retry)
			continue
		}

		res, err := client.Do(req)
		if err != nil {
			log.Printf("error sending pull request to %s: %s", url, err)
			time.Sleep(c.retry)
			continue
		}

		if res.StatusCode == http.StatusNotFound {
			break
		}

		defer res.Body.Close()
		err = json.NewDecoder(res.Body).Decode(&msg)
		if err != nil {
			log.Printf(
				"error decoding response from %s for %s: %s",
				url, topic, err,
			)
			time.Sleep(c.retry)
			break
		} else {
			c.Handle(msg)
		}
	}
}

// Publish ...
func (c *Client) Publish(topic, message string) error {
	var payload bytes.Buffer

	payload.Write([]byte(message))

	url := fmt.Sprintf("http://%s:%d/%s", c.host, c.port, topic)

	client := &http.Client{}

	req, err := http.NewRequest("PUT", url, &payload)
	if err != nil {
		return err
	}

	_, err = client.Do(req)
	if err != nil {
		return err
	}

	return nil
}

// Subscribe ...
func (c *Client) Subscribe(topic string) *Subscriber {
	return &Subscriber{client: c, topic: topic}
}

// Subscriber ...
type Subscriber struct {
	client *Client
	topic  string

	conn *websocket.Conn

	stopchan chan bool
}

// Stop ...
func (s *Subscriber) Stop() {
	s.stopchan <- true
}

// Run ...
func (s *Subscriber) Run() {
	var (
		err error
		msg *msgbus.Message
	)

	origin := "http://localhost/"

	url := fmt.Sprintf(
		"ws://%s:%d/%s",
		s.client.host, s.client.port, s.topic,
	)

	for {
		s.conn, err = websocket.Dial(url, "", origin)
		if err != nil {
			log.Printf("error connecting to %s: %s", url, err)
			time.Sleep(s.client.reconnect)
			continue
		}

		for {
			err = websocket.JSON.Receive(s.conn, &msg)
			if err != nil {
				log.Printf("lost connection to %s: %s", url, err)
				time.Sleep(s.client.reconnect)
				break
			} else {
				s.client.Handle(msg)
			}
		}
	}
}
