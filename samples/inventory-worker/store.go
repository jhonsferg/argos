package main

import (
	"context"
	"time"

	"github.com/gocql/gocql"
	"go.mongodb.org/mongo-driver/v2/mongo"

	argoslog "github.com/jhonsferg/argos/log"
)

// AuditRecord is one MongoDB document per stock-change event processed -
// the full event payload, kept for later inspection.
type AuditRecord struct {
	ItemID      string    `bson:"item_id"`
	Remaining   int       `bson:"remaining"`
	ProcessedAt time.Time `bson:"processed_at"`
}

// AuditStore writes an audit document per event to MongoDB. It logs
// through core/log's global default directly - no Logger instance is
// threaded through the constructor, same pattern as inventory-api's
// ItemStore.
type AuditStore struct {
	coll *mongo.Collection
}

func NewAuditStore(db *mongo.Database) *AuditStore {
	return &AuditStore{coll: db.Collection("stock_audit")}
}

func (s *AuditStore) Record(ctx context.Context, evt StockChanged) error {
	_, err := s.coll.InsertOne(ctx, AuditRecord{
		ItemID:      evt.ItemID,
		Remaining:   evt.Remaining,
		ProcessedAt: time.Now().UTC(),
	})
	if err != nil {
		argoslog.Error(ctx, "audit record insert failed", err, argoslog.F("item_id", evt.ItemID))
	}
	return err
}

// AnalyticsStore appends one row per event to Cassandra, partitioned by
// item_id - a shape suited to "how has this item's stock moved over time"
// queries, the kind of access pattern Cassandra is actually good at.
type AnalyticsStore struct {
	session *gocql.Session
}

func NewAnalyticsStore(session *gocql.Session) *AnalyticsStore {
	return &AnalyticsStore{session: session}
}

func (s *AnalyticsStore) Migrate() error {
	return s.session.Query(`CREATE TABLE IF NOT EXISTS stock_changes (
		item_id text,
		changed_at timestamp,
		remaining int,
		PRIMARY KEY (item_id, changed_at)
	) WITH CLUSTERING ORDER BY (changed_at DESC)`).Exec()
}

func (s *AnalyticsStore) Append(ctx context.Context, evt StockChanged) error {
	err := s.session.Query(
		`INSERT INTO stock_changes (item_id, changed_at, remaining) VALUES (?, ?, ?)`,
		evt.ItemID, time.Now().UTC(), evt.Remaining,
	).WithContext(ctx).Exec()
	if err != nil {
		argoslog.Error(ctx, "analytics append failed", err, argoslog.F("item_id", evt.ItemID))
	}
	return err
}
