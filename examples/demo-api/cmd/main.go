package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type order struct {
	ID             string `json:"id"`
	UserID         string `json:"userId"`
	SKU            string `json:"sku"`
	Quantity       int    `json:"quantity"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	CreatedAt      string `json:"createdAt"`
}

type checkoutRequest struct {
	UserID   string `json:"userId"`
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

type app struct {
	mu     sync.Mutex
	orders []order
	active int
}

func main() {
	addr := env("DEMO_API_ADDR", ":9090")
	a := &app{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /metrics", a.metrics)
	mux.HandleFunc("POST /checkout", a.checkout)
	mux.HandleFunc("GET /orders", a.ordersByQuery)
	mux.HandleFunc("POST /reset", a.reset)
	mux.HandleFunc("POST /login", a.login)

	log.Printf("wreckr demo api listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func (a *app) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *app) metrics(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	orders := len(a.orders)
	active := a.active
	a.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte("wreckr_demo_api_up 1\n"))
	_, _ = w.Write([]byte("wreckr_demo_api_orders_total " + strconv.Itoa(orders) + "\n"))
	_, _ = w.Write([]byte("wreckr_demo_api_active_requests " + strconv.Itoa(active) + "\n"))
}

func (a *app) checkout(w http.ResponseWriter, r *http.Request) {
	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	// Intentional production-style bug: the idempotency key is stored but not enforced.
	time.Sleep(80 * time.Millisecond)

	a.mu.Lock()
	defer a.mu.Unlock()
	o := order{
		ID:             "ord_" + strconv.Itoa(len(a.orders)+1),
		UserID:         req.UserID,
		SKU:            req.SKU,
		Quantity:       req.Quantity,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
		CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	a.orders = append(a.orders, o)
	writeJSON(w, http.StatusCreated, o)
}

func (a *app) ordersByQuery(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	sku := r.URL.Query().Get("sku")

	a.mu.Lock()
	defer a.mu.Unlock()
	var matches []order
	for _, o := range a.orders {
		if userID != "" && o.UserID != userID {
			continue
		}
		if sku != "" && o.SKU != sku {
			continue
		}
		matches = append(matches, o)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count":  len(matches),
		"orders": matches,
	})
}

func (a *app) reset(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.orders = nil
	a.active = 0
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	a.active++
	active := a.active
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.active--
		a.mu.Unlock()
	}()

	time.Sleep(40 * time.Millisecond)
	if active > 5 {
		// Intentional bug: overloaded login should return 429, but this service crashes semantically with 500.
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "auth pool exhausted"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": "demo-token"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func env(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
