package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	messageCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messages_produced_total",
			Help: "The total number of produced messages",
		},
		[]string{"topic"},
	)
)

type Producer interface {
	Start(context.Context) error
	Stop() error
}

type HTTPProducer struct {
	listenAddr string
	server     *http.Server
	producech  chan<- Message
	validators []MessageValidator
	mu         sync.RWMutex
}

func NewHTTPProducer(listenAddr string, producech chan Message) *HTTPProducer {
	p := &HTTPProducer{
		listenAddr: listenAddr,
		producech:  producech,
		validators: make([]MessageValidator, 0),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/publish/", p.handlePublish)
	mux.Handle("/metrics", promhttp.Handler())

	p.server = &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	return p
}

func (p *HTTPProducer) AddValidator(v MessageValidator) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.validators = append(p.validators, v)
}

func (p *HTTPProducer) validateMessage(msg Message) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, validator := range p.validators {
		if err := validator(msg); err != nil {
			return &ErrInvalidMessage{Reason: err.Error()}
		}
	}
	return nil
}

func (p *HTTPProducer) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var (
		path  = strings.TrimPrefix(r.URL.Path, "/publish/")
		topic = path
	)

	if topic == "" {
		http.Error(w, "topic is required", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	msg := Message{
		ID:        uuid.New().String(),
		Data:      body,
		Topic:     topic,
		Timestamp: time.Now(),
		Headers:   FromHTTPHeader(r.Header),
	}

	if err := p.validateMessage(msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Use context with timeout for message production
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	select {
	case p.producech <- msg:
		messageCounter.WithLabelValues(topic).Inc()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": msg.ID})
	case <-ctx.Done():
		http.Error(w, "timeout publishing message", http.StatusGatewayTimeout)
	}
}

func (p *HTTPProducer) Start(ctx context.Context) error {
	slog.Info("HTTP producer starting", "addr", p.listenAddr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := p.server.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shutdown server gracefully", "error", err)
		}
	}()

	if err := p.server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

func (p *HTTPProducer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return p.server.Shutdown(ctx)
}
