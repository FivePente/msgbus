package msgbus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageBusLen(t *testing.T) {
	mb := NewMessageBus()
	assert.Equal(t, mb.Len(), 0)
}

func TestMessage(t *testing.T) {
	mb := NewMessageBus()
	assert.Equal(t, mb.Len(), 0)

	topic := "foo"
	expected := Message{Topic: topic, Payload: []byte("bar")}
	mb.Put(topic, expected)

	actual, ok := mb.Get(topic)
	assert.True(t, ok)
	assert.Equal(t, actual, expected)
}

func TestMessageGetEmpty(t *testing.T) {
	mb := NewMessageBus()
	assert.Equal(t, mb.Len(), 0)

	topic := "foo"
	msg, ok := mb.Get(topic)
	assert.False(t, ok)
	assert.Equal(t, msg, Message{})
}

func BenchmarkMessageBusPut(b *testing.B) {
	mb := NewMessageBus()
	topic := "foo"
	msg := Message{Payload: []byte("foo")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Put(topic, msg)
	}
}

func BenchmarkMessageBusGet(b *testing.B) {
	mb := NewMessageBus()
	topic := "foo"
	msg := Message{Payload: []byte("foo")}
	for i := 0; i < b.N; i++ {
		mb.Put(topic, msg)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Get(topic)
	}
}

func BenchmarkMessageBusGetEmpty(b *testing.B) {
	mb := NewMessageBus()
	topic := "foo"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Get(topic)
	}
}

func BenchmarkMessageBusPutGet(b *testing.B) {
	mb := NewMessageBus()
	topic := "foo"
	msg := Message{Payload: []byte("foo")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Put(topic, msg)
		mb.Get(topic)
	}
}
