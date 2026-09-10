package argosgorm

import "gorm.io/gorm"

// Plugin is a gorm.Plugin adding spans/metrics around Create, Query, Update,
// Delete, Row, and Raw operations.
type Plugin struct {
	cfg *config
}

// New builds a Plugin. Register it once per *gorm.DB via db.Use(New(...)).
func New(opts ...Option) *Plugin {
	return &Plugin{cfg: newConfig(opts...)}
}

// Name implements gorm.Plugin.
func (p *Plugin) Name() string { return "argos:instrumentation" }

// Initialize implements gorm.Plugin, registering the before/after callback
// pairs GORM's own callback chain invokes for each operation kind.
func (p *Plugin) Initialize(db *gorm.DB) error {
	// gorm's processor/callback types are unexported, so each operation is
	// wired individually rather than looped over a slice of them.
	if err := db.Callback().Create().Before("gorm:create").Register("argos:before_create", p.cfg.before("create")); err != nil {
		return err
	}
	if err := db.Callback().Create().After("gorm:create").Register("argos:after_create", p.cfg.after("create")); err != nil {
		return err
	}
	if err := db.Callback().Query().Before("gorm:query").Register("argos:before_query", p.cfg.before("query")); err != nil {
		return err
	}
	if err := db.Callback().Query().After("gorm:query").Register("argos:after_query", p.cfg.after("query")); err != nil {
		return err
	}
	if err := db.Callback().Update().Before("gorm:update").Register("argos:before_update", p.cfg.before("update")); err != nil {
		return err
	}
	if err := db.Callback().Update().After("gorm:update").Register("argos:after_update", p.cfg.after("update")); err != nil {
		return err
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register("argos:before_delete", p.cfg.before("delete")); err != nil {
		return err
	}
	if err := db.Callback().Delete().After("gorm:delete").Register("argos:after_delete", p.cfg.after("delete")); err != nil {
		return err
	}
	if err := db.Callback().Row().Before("gorm:row").Register("argos:before_row", p.cfg.before("row")); err != nil {
		return err
	}
	if err := db.Callback().Row().After("gorm:row").Register("argos:after_row", p.cfg.after("row")); err != nil {
		return err
	}
	if err := db.Callback().Raw().Before("gorm:raw").Register("argos:before_raw", p.cfg.before("raw")); err != nil {
		return err
	}
	if err := db.Callback().Raw().After("gorm:raw").Register("argos:after_raw", p.cfg.after("raw")); err != nil {
		return err
	}
	return nil
}
