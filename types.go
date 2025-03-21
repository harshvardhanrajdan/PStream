package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Message represents a message in the system
type Message struct {
	ID        string    `json:"id"`
	Data      []byte    `json:"data"`
	Topic     string    `json:"topic"`
	Timestamp time.Time `json:"timestamp"`
	Headers   Headers   `json:"headers"`
}

// Headers represents message metadata
type Headers map[string]string

// FromHTTPHeader converts http.Header to Headers
func FromHTTPHeader(h http.Header) Headers {
	headers := make(Headers)
	for k, v := range h {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return headers
}

// Custom error types
type ErrTopicNotFound struct {
	Topic string
}

func (e *ErrTopicNotFound) Error() string {
	return fmt.Sprintf("topic not found: %s", e.Topic)
}

type ErrInvalidMessage struct {
	Reason string
}

func (e *ErrInvalidMessage) Error() string {
	return fmt.Sprintf("invalid message: %s", e.Reason)
}

// MessageValidator is a function type that validates messages
type MessageValidator func(Message) error

// MessageHandler defines how messages should be processed
type MessageHandler interface {
	Handle(context.Context, Message) error
}
