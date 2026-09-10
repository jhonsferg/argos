package argosgorm_test

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	argosgorm "github.com/jhonsferg/argos/integrations/gorm"
)

type item struct {
	ID   uint
	Name string
}

func setTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return exp
}

func openTestDB(t *testing.T, opts ...argosgorm.Option) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	if err := db.Use(argosgorm.New(opts...)); err != nil {
		t.Fatalf("db.Use: %v", err)
	}
	if err := db.AutoMigrate(&item{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

func spanNames(spans tracetest.SpanStubs) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name
	}
	return names
}

func TestPlugin_Create(t *testing.T) {
	exp := setTracer(t)
	db := openTestDB(t)
	exp.Reset()

	if err := db.Create(&item{Name: "widget"}).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Name != "create" {
		t.Fatalf("spans = %v, want [create]", spanNames(spans))
	}
	var hasTable bool
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "db.collection.name" && kv.Value.AsString() == "items" {
			hasTable = true
		}
	}
	if !hasTable {
		t.Error("missing db.collection.name=items attribute")
	}
}

func TestPlugin_QueryAndUpdateAndDelete(t *testing.T) {
	exp := setTracer(t)
	db := openTestDB(t)

	rec := item{Name: "widget"}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}
	exp.Reset()

	var got item
	if err := db.First(&got, rec.ID).Error; err != nil {
		t.Fatalf("First: %v", err)
	}
	if err := db.Model(&got).Update("Name", "gadget").Error; err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := db.Delete(&got).Error; err != nil {
		t.Fatalf("Delete: %v", err)
	}

	names := spanNames(exp.GetSpans())
	want := []string{"query", "update", "delete"}
	if len(names) != len(want) {
		t.Fatalf("spans = %v, want %v", names, want)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("span[%d] = %q, want %q", i, names[i], n)
		}
	}
}

func TestPlugin_QueryTextOptIn(t *testing.T) {
	exp := setTracer(t)
	db := openTestDB(t, argosgorm.WithQueryText(true))
	exp.Reset()

	if err := db.Create(&item{Name: "widget"}).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}

	var found bool
	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "db.query.text" {
			found = true
		}
	}
	if !found {
		t.Error("expected db.query.text attribute when WithQueryText(true) is set")
	}
}

func TestPlugin_MaskedQueryTextOptIn(t *testing.T) {
	exp := setTracer(t)
	db := openTestDB(t, argosgorm.WithMaskedQueryText(true))
	exp.Reset()

	// GORM's query builder (Create/Find/...) already parameterizes with "?"
	// placeholders - a hand-written Raw query is what actually surfaces a
	// literal value worth masking.
	var items []item
	if err := db.Raw("SELECT * FROM items WHERE name = 'top-secret-name'").Scan(&items).Error; err != nil {
		t.Fatalf("Raw: %v", err)
	}

	var got string
	var found bool
	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "db.query.text" {
			got, found = kv.Value.AsString(), true
		}
	}
	if !found {
		t.Fatal("expected db.query.text attribute when WithMaskedQueryText(true) is set")
	}
	if strings.Contains(got, "top-secret-name") {
		t.Errorf("db.query.text = %q, still contains the literal value", got)
	}
}

func TestPlugin_RecordNotFoundIsNotAnError(t *testing.T) {
	exp := setTracer(t)
	db := openTestDB(t)
	exp.Reset()

	var got item
	err := db.First(&got, 999).Error
	if err == nil {
		t.Fatal("expected gorm.ErrRecordNotFound")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code == codes.Error {
		t.Error("ErrRecordNotFound must not be recorded as a span error")
	}
}

func TestPlugin_RecordsRealErrors(t *testing.T) {
	exp := setTracer(t)
	db := openTestDB(t)
	exp.Reset()

	// Raw with invalid SQL forces a real driver error.
	err := db.Exec("NOT VALID SQL").Error
	if err == nil {
		t.Fatal("expected a syntax error")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", spans[0].Status.Code, codes.Error)
	}
}
