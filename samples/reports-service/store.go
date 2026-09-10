package main

import (
	"context"
	"time"

	"github.com/gocql/gocql"
)

// ReportRecord is the metadata recorded for each report pulled over SFTP.
// The sample only records size and timestamp, not the file's own content -
// see reports.go for why.
type ReportRecord struct {
	Filename string    `json:"filename"`
	Size     int64     `json:"size_bytes"`
	PulledAt time.Time `json:"pulled_at"`
}

type ReportStore struct {
	session *gocql.Session
}

func NewReportStore(session *gocql.Session) *ReportStore {
	return &ReportStore{session: session}
}

func (s *ReportStore) Migrate() error {
	return s.session.Query(`CREATE TABLE IF NOT EXISTS reports (
		filename text PRIMARY KEY,
		size_bytes bigint,
		pulled_at timestamp
	)`).Exec()
}

func (s *ReportStore) Save(ctx context.Context, r ReportRecord) error {
	return s.session.Query(
		`INSERT INTO reports (filename, size_bytes, pulled_at) VALUES (?, ?, ?)`,
		r.Filename, r.Size, r.PulledAt,
	).WithContext(ctx).Exec()
}

func (s *ReportStore) Get(ctx context.Context, filename string) (ReportRecord, error) {
	var r ReportRecord
	err := s.session.Query(`SELECT filename, size_bytes, pulled_at FROM reports WHERE filename = ?`, filename).
		WithContext(ctx).Scan(&r.Filename, &r.Size, &r.PulledAt)
	return r, err
}
