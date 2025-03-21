package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID        string            `json:"id"`
	Data      []byte            `json:"data"`
	Topic     string            `json:"topic"`
	Timestamp time.Time         `json:"timestamp"`
	Headers   map[string]string `json:"headers"`
}

func main() {
	url := "http://localhost:3000/publish"
	topics := []string{"topic_1", "topic_2", "topic_3"}

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < 1000; i++ {
		topic := topics[rand.Intn(len(topics))]

		msg := Message{
			ID:        uuid.New().String(),
			Data:      []byte(fmt.Sprintf("message_%d", i)),
			Topic:     topic,
			Timestamp: time.Now(),
			Headers: map[string]string{
				"content-type": "text/plain",
				"sequence":     fmt.Sprintf("%d", i),
				"producer":     "test-publisher",
			},
		}

		payload, err := json.Marshal(msg)
		if err != nil {
			log.Fatal(err)
		}

		req, err := http.NewRequest("POST", url+"/"+topic, bytes.NewReader(payload))
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("unexpected status code: %d", resp.StatusCode)
		}
		resp.Body.Close()

		// Add some random delay between messages
		time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
	}
}
