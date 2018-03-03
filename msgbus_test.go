package msgbus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageBusLen(t *testing.T) {
	mb := NewMessageBus(nil)
	assert.Equal(t, mb.Len(), 0)
}

func TestMessage(t *testing.T) {
	mb := NewMessageBus(nil)
	assert.Equal(t, mb.Len(), 0)

	topic := mb.NewTopic("foo")
	expected := Message{Topic: topic, Payload: []byte("bar")}
	mb.Put(expected)

	actual, ok := mb.Get(topic)
	assert.True(t, ok)
	assert.Equal(t, actual, expected)
}

func TestMessageGetEmpty(t *testing.T) {
	mb := NewMessageBus(nil)
	assert.Equal(t, mb.Len(), 0)

	topic := mb.NewTopic("foo")
	msg, ok := mb.Get(topic)
	assert.False(t, ok)
	assert.Equal(t, msg, Message{})
}

func BenchmarkMessageBusPut(b *testing.B) {
	mb := NewMessageBus(nil)
	topic := mb.NewTopic("foo")
	msg := Message{Topic: topic, Payload: []byte("foo")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Put(msg)
	}
}

func BenchmarkMessageBusGet(b *testing.B) {
	mb := NewMessageBus(nil)
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
	mb := NewMessageBus(nil)
	topic := mb.NewTopic("foo")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Get(topic)
	}
}

func BenchmarkMessageBusPutGet(b *testing.B) {
	mb := NewMessageBus(nil)
	topic := mb.NewTopic("foo")
	msg := Message{Topic: topic, Payload: []byte("foo")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Put(msg)
		mb.Get(topic)
	}
}
