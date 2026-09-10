package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Order is the domain type persisted in Postgres and cached in Redis.
type Order struct {
	ID           int64     `json:"id"`
	CustomerName string    `json:"customer_name"`
	Item         string    `json:"item"`
	Quantity     int       `json:"quantity"`
	CreatedAt    time.Time `json:"created_at"`
}

// ErrOrderNotFound is returned by OrderStore.Get when no order has the
// requested id.
var ErrOrderNotFound = errors.New("order not found")

// OrderStore is a cache-aside repository: reads check Redis first and only
// fall through to Postgres on a miss, populating the cache afterward so the
// next read for the same order is a cache hit.
type OrderStore struct {
	db    *sql.DB
	cache *redis.Client
}

func NewOrderStore(db *sql.DB, cache *redis.Client) *OrderStore {
	return &OrderStore{db: db, cache: cache}
}

// Migrate creates the orders table if it doesn't already exist. A real
// service would use a migration tool; this is a sample, so a single
// idempotent DDL statement keeps it self-contained.
func (s *OrderStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			customer_name TEXT NOT NULL,
			item TEXT NOT NULL,
			quantity INT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	return err
}

func (s *OrderStore) Create(ctx context.Context, o Order) (Order, error) {
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO orders (customer_name, item, quantity) VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		o.CustomerName, o.Item, o.Quantity,
	).Scan(&o.ID, &o.CreatedAt)
	if err != nil {
		return Order{}, err
	}
	return o, nil
}

func cacheKey(id int64) string { return fmt.Sprintf("order:%d", id) }

// Get reads through Redis: a cache hit skips Postgres entirely. A miss
// queries Postgres and populates the cache for next time.
func (s *OrderStore) Get(ctx context.Context, id int64) (Order, error) {
	if cached, err := s.cache.Get(ctx, cacheKey(id)).Result(); err == nil {
		var o Order
		if jsonErr := json.Unmarshal([]byte(cached), &o); jsonErr == nil {
			return o, nil
		}
	}

	var o Order
	err := s.db.QueryRowContext(ctx,
		`SELECT id, customer_name, item, quantity, created_at FROM orders WHERE id = $1`, id,
	).Scan(&o.ID, &o.CustomerName, &o.Item, &o.Quantity, &o.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, err
	}

	if data, err := json.Marshal(o); err == nil {
		s.cache.Set(ctx, cacheKey(id), data, 5*time.Minute)
	}
	return o, nil
}
