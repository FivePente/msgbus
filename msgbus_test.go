package msgbus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestMessageBusLen(t *testing.T) {
	mb := New(nil)
	assert.Equal(t, mb.Len(), 0)
}

func TestMessage(t *testing.T) {
	mb := New(nil)
	assert.Equal(t, mb.Len(), 0)

	topic := mb.NewTopic("foo")
	expected := Message{Topic: topic, Payload: []byte("bar")}
	mb.Put(expected)

	actual, ok := mb.Get(topic)
	assert.True(t, ok)
	assert.Equal(t, actual, expected)
}

func TestMessageGetEmpty(t *testing.T) {
	mb := New(nil)
	assert.Equal(t, mb.Len(), 0)

	topic := mb.NewTopic("foo")
	msg, ok := mb.Get(topic)
	assert.False(t, ok)
	assert.Equal(t, msg, Message{})
}

func TestMessageBusPutGet(t *testing.T) {
	mb := New(nil)
	topic := mb.NewTopic("foo")
	expected := Message{Topic: topic, Payload: []byte("foo")}
	mb.Put(expected)

	actual, ok := mb.Get(topic)
	assert.True(t, ok)
	assert.Equal(t, actual, expected)
}

func TestServeHTTPGETEmpty(t *testing.T) {
	assert := assert.New(t)

	mb := New(nil)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/", nil)

	mb.ServeHTTP(w, r)
	assert.Equal(w.Code, http.StatusOK)
	assert.Equal(w.Body.String(), "{}")
}

func TestServeHTTPPOST(t *testing.T) {
	assert := assert.New(t)

	mb := New(nil)
	w := httptest.NewRecorder()
	b := bytes.NewBufferString("hello world")
	r, _ := http.NewRequest("POST", "/hello", b)

	mb.ServeHTTP(w, r)
	assert.Equal(w.Code, http.StatusOK)
	assert.Regexp(`message successfully published to hello with sequence \d+`, w.Body.String())
}

func TestServeHTTPMaxPayloadSize(t *testing.T) {
	assert := assert.New(t)

	mb := New(nil)
	w := httptest.NewRecorder()
	b := bytes.NewBuffer(bytes.Repeat([]byte{'X'}, (DefaultMaxPayloadSize * 2)))
	r, _ := http.NewRequest("POST", "/hello", b)

	mb.ServeHTTP(w, r)
	assert.Equal(http.StatusRequestEntityTooLarge, w.Code)
	assert.Regexp(`payload exceeds max-payload-size`, w.Body.String())
}

func TestServeHTTPSimple(t *testing.T) {
	assert := assert.New(t)

	mb := New(nil)

	w := httptest.NewRecorder()
	b := bytes.NewBufferString("hello world")
	r, _ := http.NewRequest("POST", "/hello", b)

	mb.ServeHTTP(w, r)
	assert.Equal(w.Code, http.StatusOK)
	assert.Regexp(`message successfully published to hello with sequence \d+`, w.Body.String())

	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "/hello", nil)

	mb.ServeHTTP(w, r)
	assert.Equal(w.Code, http.StatusOK)

	var msg *Message
	json.Unmarshal(w.Body.Bytes(), &msg)
	assert.Equal(msg.ID, uint64(0))
	assert.Equal(msg.Topic.Name, "hello")
	assert.Equal(msg.Payload, []byte("hello world"))
}

func BenchmarkServeHTTPPOST(b *testing.B) {
	mb := New(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		b := bytes.NewBufferString("hello world")
		r, _ := http.NewRequest("POST", "/hello", b)

		mb.ServeHTTP(w, r)
	}
}

func TestServeHTTPSubscriber(t *testing.T) {
	assert := assert.New(t)

	mb := New(nil)

	s := httptest.NewServer(mb)
	defer s.Close()

	msgs := make(chan *Message)
	ready := make(chan bool, 1)

	consumer := func() {
		var msg *Message

		u := fmt.Sprintf("ws%s/hello", strings.TrimPrefix(s.URL, "http"))

		ws, _, err := websocket.DefaultDialer.Dial(u, nil)
		assert.NoError(err)
		defer ws.Close()

		ready <- true

		err = ws.ReadJSON(&msg)
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

		msgs <- msg
	}

	go consumer()

	<-ready

	c := s.Client()
	b := bytes.NewBufferString("hello world")
	r, err := c.Post(s.URL+"/hello", "text/plain", b)
	assert.NoError(err)
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	assert.NoError(err)
	assert.Regexp(`message successfully published to hello with sequence \d+`, string(body))

	msg := <-msgs
	assert.Equal(msg.ID, uint64(0))
	assert.Equal(msg.Topic.Name, "hello")
	assert.Equal(msg.Payload, []byte("hello world"))
}

func TestMsgBusMetrics(t *testing.T) {
	assert := assert.New(t)

	opts := Options{
		WithMetrics: true,
	}
	mb := New(&opts)

	assert.IsType(&Metrics{}, mb.Metrics())
}

func BenchmarkMessageBusPut(b *testing.B) {
	mb := New(nil)
	topic := mb.NewTopic("foo")
	msg := Message{Topic: topic, Payload: []byte("foo")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Put(msg)
	}
}

func BenchmarkMessageBusGet(b *testing.B) {
	mb := New(nil)
	topic := mb.NewTopic("foo")
	msg := Message{Topic: topic, Payload: []byte("foo")}
	for i := 0; i < b.N; i++ {
		mb.Put(msg)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Get(topic)
	}
}

func BenchmarkMessageBusGetEmpty(b *testing.B) {
	mb := New(nil)
	topic := mb.NewTopic("foo")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Get(topic)
	}
}

func BenchmarkMessageBusPutGet(b *testing.B) {
	mb := New(nil)
	topic := mb.NewTopic("foo")
	msg := Message{Topic: topic, Payload: []byte("foo")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Put(msg)
		mb.Get(topic)
	}
}
