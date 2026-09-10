package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	argoslog "github.com/jhonsferg/argos/log"
)

// Item is a stock-tracked catalog entry.
type Item struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Stock int    `json:"stock"`
}

// ErrItemNotFound is returned by ItemStore.Get when no item has the
// requested id.
var ErrItemNotFound = errors.New("item not found")

// ErrInsufficientStock is returned by ItemStore.ReserveStock when quantity
// exceeds the item's current stock.
var ErrInsufficientStock = errors.New("insufficient stock")

// ItemStore is a cache-aside repository over Postgres, same pattern as
// orders-api's OrderStore. It logs through core/log's global default
// directly - no Logger instance is threaded through the constructor, since
// argos.Init has already wired core/log by the time this runs.
type ItemStore struct {
	db    *sql.DB
	cache *redis.Client
}

func NewItemStore(db *sql.DB, cache *redis.Client) *ItemStore {
	return &ItemStore{db: db, cache: cache}
}

// Migrate creates the items table if it doesn't already exist, seeding it
// with a couple of sample rows so the sample is usable immediately.
func (s *ItemStore) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS items (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			stock INT NOT NULL
		)`); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO items (id, name, stock) VALUES
			('widget', 'Widget', 100),
			('gadget', 'Gadget', 50)
		ON CONFLICT (id) DO NOTHING`)
	return err
}

func cacheKey(id string) string { return fmt.Sprintf("item:%s", id) }

// Get reads through Redis: a cache hit skips Postgres entirely.
func (s *ItemStore) Get(ctx context.Context, id string) (Item, error) {
	if cached, err := s.cache.Get(ctx, cacheKey(id)).Result(); err == nil {
		var it Item
		if jsonErr := json.Unmarshal([]byte(cached), &it); jsonErr == nil {
			return it, nil
		}
	}

	var it Item
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, stock FROM items WHERE id = $1`, id,
	).Scan(&it.ID, &it.Name, &it.Stock)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrItemNotFound
	}
	if err != nil {
		argoslog.Error(ctx, "item lookup failed", err, argoslog.F("item_id", id))
		return Item{}, err
	}

	if data, err := json.Marshal(it); err == nil {
		s.cache.Set(ctx, cacheKey(id), data, 5*time.Minute)
	}
	return it, nil
}

// ReserveStock atomically decrements an item's stock by quantity, failing
// with ErrInsufficientStock if that would take it negative, and evicts the
// cache entry so the next Get reflects the new stock level rather than a
// stale cached value.
func (s *ItemStore) ReserveStock(ctx context.Context, itemID string, quantity int) (remaining int, err error) {
	err = s.db.QueryRowContext(ctx, `
		UPDATE items SET stock = stock - $1
		WHERE id = $2 AND stock >= $1
		RETURNING stock`,
		quantity, itemID,
	).Scan(&remaining)
	if errors.Is(err, sql.ErrNoRows) {
		// Either the item doesn't exist, or it does but stock < quantity -
		// distinguish the two so the caller can report the right error.
		var exists bool
		if checkErr := s.db.QueryRowContext(ctx, `SELECT true FROM items WHERE id = $1`, itemID).Scan(&exists); checkErr == nil && exists {
			return 0, ErrInsufficientStock
		}
		return 0, ErrItemNotFound
	}
	if err != nil {
		argoslog.Error(ctx, "reserve stock failed", err, argoslog.F("item_id", itemID), argoslog.F("quantity", quantity))
		return 0, err
	}

	s.cache.Del(ctx, cacheKey(itemID))
	return remaining, nil
}
