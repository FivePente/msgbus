package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
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
	url string

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
func NewClient(url string, options *Options) *Client {
	var (
		reconnectInterval = DefaultReconnectInterval
		retryInterval     = DefaultRetryInterval
	)

	url = strings.TrimSuffix(url, "/")

	client := &Client{url: url}

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
func (c *Client) Handle(msg *msgbus.Message) error {
	out, err := json.Marshal(msg)
	if err != nil {
		log.Errorf("error marshalling message: %s", err)
		return err
	}

	os.Stdout.Write(out)
	return nil
}

// Pull ...
func (c *Client) Pull(topic string) {
	var msg *msgbus.Message

	url := fmt.Sprintf("%s/%s", c.url, topic)
	client := &http.Client{}

	for {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf(
				"error constructing pull request to %s: %s",
				url, err,
			)
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

	url := fmt.Sprintf("%s/%s", c.url, topic)

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
	return &Subscriber{
		client: c,
		topic:  topic,
		errch:  make(chan error),
		stopch: make(chan bool),
	}
}

// Subscriber ...
type Subscriber struct {
	conn   *websocket.Conn
	client *Client

	topic string

	errch  chan error
	stopch chan bool
}

// Stop ...
func (s *Subscriber) Stop() {
	close(s.errch)
	s.stopch <- true
}

// Run ...
func (s *Subscriber) Run() {
	var err error

	origin := "http://localhost/"

	u, err := url.Parse(s.client.url)
	if err != nil {
		log.Fatal("invalid url: %s", s.client.url)
	}

	u.Scheme = "ws"
	u.Path += fmt.Sprintf("/%s", s.topic)

	url := u.String()

	for {
		s.conn, err = websocket.Dial(url, "", origin)
		if err != nil {
			log.Warnf("error connecting to %s: %s", url, err)
			time.Sleep(s.client.reconnect)
			continue
		}

		go s.Reader()

		select {
		case err = <-s.errch:
			if err != nil {
				log.Warnf("lost connection to %s: %s", url, err)
				time.Sleep(s.client.reconnect)
			}
		case <-s.stopch:
			log.Infof("shutting down ...")
			s.conn.Close()
			break
		}
	}
}

// Reader ...
func (s *Subscriber) Reader() {
	var msg *msgbus.Message

	for {
		err := websocket.JSON.Receive(s.conn, &msg)
		if err != nil {
			s.errch <- err
			break
		}
		s.client.Handle(msg)
	}
}
