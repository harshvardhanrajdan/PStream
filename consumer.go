package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	wsConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "websocket_connections_active",
		Help: "The number of active WebSocket connections",
	})

	messagesDelivered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messages_delivered_total",
			Help: "The total number of messages delivered to consumers",
		},
		[]string{"topic"},
	)
)

type Consumer interface {
	Start(context.Context) error
	Stop() error
	Subscribe(topic string) error
	Unsubscribe(topic string) error
}

type WSConsumer struct {
	addr      string
	upgrader  websocket.Upgrader
	clients   map[*websocket.Conn][]string // client -> subscribed topics
	clientsMu sync.RWMutex
	server    *http.Server
}

func NewWSConsumer(addr string) *WSConsumer {
	c := &WSConsumer{
		addr: addr,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // In production, implement proper origin checks
			},
		},
		clients: make(map[*websocket.Conn][]string),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", c.handleWebSocket)
	mux.Handle("/metrics", promhttp.Handler())

	c.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return c
}

func (c *WSConsumer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("failed to upgrade connection", "error", err)
		return
	}

	c.clientsMu.Lock()
	c.clients[conn] = make([]string, 0)
	c.clientsMu.Unlock()

	wsConnections.Inc()
	defer func() {
		c.clientsMu.Lock()
		delete(c.clients, conn)
		c.clientsMu.Unlock()
		conn.Close()
		wsConnections.Dec()
	}()

	// Handle incoming messages (subscribe/unsubscribe commands)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("websocket error", "error", err)
			}
			break
		}

		var cmd struct {
			Action string `json:"action"`
			Topic  string `json:"topic"`
		}

		if err := json.Unmarshal(msg, &cmd); err != nil {
			slog.Error("invalid command", "error", err)
			continue
		}

		switch cmd.Action {
		case "subscribe":
			c.handleSubscribe(conn, cmd.Topic)
		case "unsubscribe":
			c.handleUnsubscribe(conn, cmd.Topic)
		}
	}
}

func (c *WSConsumer) handleSubscribe(conn *websocket.Conn, topic string) {
	c.clientsMu.Lock()
	defer c.clientsMu.Unlock()

	topics := c.clients[conn]
	for _, t := range topics {
		if t == topic {
			return // Already subscribed
		}
	}
	c.clients[conn] = append(topics, topic)
}

func (c *WSConsumer) handleUnsubscribe(conn *websocket.Conn, topic string) {
	c.clientsMu.Lock()
	defer c.clientsMu.Unlock()

	topics := c.clients[conn]
	for i, t := range topics {
		if t == topic {
			c.clients[conn] = append(topics[:i], topics[i+1:]...)
			return
		}
	}
}

func (c *WSConsumer) Broadcast(msg Message) {
	c.clientsMu.RLock()
	defer c.clientsMu.RUnlock()

	for conn, topics := range c.clients {
		for _, topic := range topics {
			if topic == msg.Topic {
				err := conn.WriteJSON(msg)
				if err != nil {
					slog.Error("failed to send message", "error", err, "topic", msg.Topic)
					continue
				}
				messagesDelivered.WithLabelValues(msg.Topic).Inc()
				break
			}
		}
	}
}

func (c *WSConsumer) Start(ctx context.Context) error {
	slog.Info("WebSocket consumer starting", "addr", c.addr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.server.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shutdown server gracefully", "error", err)
		}
	}()

	if err := c.server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

func (c *WSConsumer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c.clientsMu.Lock()
	for conn := range c.clients {
		conn.Close()
	}
	c.clients = make(map[*websocket.Conn][]string)
	c.clientsMu.Unlock()

	return c.server.Shutdown(ctx)
}

func (c *WSConsumer) Subscribe(topic string) error {
	// This method is for the Consumer interface but not used in WebSocket implementation
	// as subscriptions are handled per-connection
	return nil
}

func (c *WSConsumer) Unsubscribe(topic string) error {
	// This method is for the Consumer interface but not used in WebSocket implementation
	// as subscriptions are handled per-connection
	return nil
}
