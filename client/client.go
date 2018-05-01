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

var (
	// PublishedRegexp ...
	PublishedRegexp = regexp.MustCompile(
		"message successfully published to \\w+ with sequence \\d",
	)
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
	os.Stdout.Write([]byte{'\r', '\n'})
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
			log.Printf("error constructing request to %s: %s", url, err)
			time.Sleep(c.retry)
			continue
		}

		res, err := client.Do(req)
		if err != nil {
			log.Printf("error sending request to %s: %s", url, err)
			time.Sleep(c.retry)
			continue
		}

		if res.StatusCode == http.StatusNotFound {
			// Empty queue
			break
		}

		defer res.Body.Close()
		err = json.NewDecoder(res.Body).Decode(&msg)
		if err != nil {
			log.Errorf(
				"error decoding response from %s for %s: %s",
				url, topic, err,
			)
			break
		} else {
			err := c.Handle(msg)
			if err != nil {
				log.Errorf(
					"error handling message from %s for %s: %s",
					url, topic, err,
				)
			}
			break
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
	conn   *websocket.Conn
	client *Client

	topic   string
	handler msgbus.HandlerFunc

	errch  chan error
	stopch chan bool
}

// NewSubscriber ...
func NewSubscriber(client *Client, topic string, handler msgbus.HandlerFunc) *Subscriber {
	if handler == nil {
		handler = client.Handle
	}
	return &Subscriber{
		client:  client,
		topic:   topic,
		handler: handler,
		errch:   make(chan error),
		stopch:  make(chan bool),
	}
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

	if strings.HasPrefix(s.client.url, "https") {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}

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
		err = s.handler(msg)
		if err != nil {
			log.Errorf("error handling message: %s", err)
		}
	}
}
