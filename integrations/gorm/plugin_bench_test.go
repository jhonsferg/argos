package argosgorm_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	argosgorm "github.com/jhonsferg/argos/integrations/gorm"
)

func newBenchDB(b *testing.B, withPlugin bool) *gorm.DB {
	b.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		b.Fatal(err)
	}
	if withPlugin {
		if err := db.Use(argosgorm.New()); err != nil {
			b.Fatal(err)
		}
	}
	if err := db.AutoMigrate(&item{}); err != nil {
		b.Fatal(err)
	}
	return db
}

// BenchmarkCreate_Baseline measures GORM with no Argos plugin registered -
// the "without Argos" comparison point.
func BenchmarkCreate_Baseline(b *testing.B) {
	db := newBenchDB(b, false)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := db.Create(&item{Name: "widget"}).Error; err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCreate_Instrumented measures the same operation with the Plugin
// registered, documenting the allocation cost it adds per call.
func BenchmarkCreate_Instrumented(b *testing.B) {
	db := newBenchDB(b, true)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := db.Create(&item{Name: "widget"}).Error; err != nil {
			b.Fatal(err)
		}
	}
}
