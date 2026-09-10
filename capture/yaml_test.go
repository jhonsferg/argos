package capture

import "testing"

func TestYAMLRule_Apply(t *testing.T) {
	t.Run("nil receiver is a no-op", func(t *testing.T) {
		r := Rule{Enabled: true, MaxBytes: 10, OnErrorOnly: true}
		want := r
		var y *YAMLRule
		y.Apply(&r)
		if r != want {
			t.Errorf("Apply(nil) changed r: got %+v, want %+v", r, want)
		}
	})

	t.Run("empty YAMLRule leaves every field untouched", func(t *testing.T) {
		r := Rule{Enabled: true, MaxBytes: 10, OnErrorOnly: true}
		want := r
		(&YAMLRule{}).Apply(&r)
		if r != want {
			t.Errorf("Apply({}) changed r: got %+v, want %+v", r, want)
		}
	})

	t.Run("only the set fields overlay", func(t *testing.T) {
		r := Rule{Enabled: false, MaxBytes: 10, OnErrorOnly: false}
		enabled := true
		(&YAMLRule{Enabled: &enabled}).Apply(&r)
		want := Rule{Enabled: true, MaxBytes: 10, OnErrorOnly: false}
		if r != want {
			t.Errorf("got %+v, want %+v", r, want)
		}
	})

	t.Run("every field set overlays all of them", func(t *testing.T) {
		r := Rule{}
		enabled, onErrorOnly, maxBytes := true, true, 42
		(&YAMLRule{Enabled: &enabled, MaxBytes: &maxBytes, OnErrorOnly: &onErrorOnly}).Apply(&r)
		want := Rule{Enabled: true, MaxBytes: 42, OnErrorOnly: true}
		if r != want {
			t.Errorf("got %+v, want %+v", r, want)
		}
	})
}

func TestYAMLHeaderRule_Apply(t *testing.T) {
	t.Run("nil receiver is a no-op", func(t *testing.T) {
		r := HeaderRule{Enabled: true, Exclude: []string{"Authorization"}}
		var y *YAMLHeaderRule
		y.Apply(&r)
		if !r.Enabled || len(r.Exclude) != 1 || r.Exclude[0] != "Authorization" {
			t.Errorf("Apply(nil) changed r: got %+v", r)
		}
	})

	t.Run("Enabled and OnErrorOnly overlay independently of the lists", func(t *testing.T) {
		r := HeaderRule{Enabled: false, OnErrorOnly: false}
		enabled, onErrorOnly := true, true
		(&YAMLHeaderRule{Enabled: &enabled, OnErrorOnly: &onErrorOnly}).Apply(&r)
		if !r.Enabled || !r.OnErrorOnly {
			t.Errorf("got %+v, want both true", r)
		}
	})

	t.Run("omitted Exclude/Mask leave the existing lists alone", func(t *testing.T) {
		r := HeaderRule{Exclude: []string{"Authorization"}, Mask: []string{"X-Api-Key"}}
		(&YAMLHeaderRule{}).Apply(&r)
		if len(r.Exclude) != 1 || r.Exclude[0] != "Authorization" {
			t.Errorf("Exclude changed: got %v", r.Exclude)
		}
		if len(r.Mask) != 1 || r.Mask[0] != "X-Api-Key" {
			t.Errorf("Mask changed: got %v", r.Mask)
		}
	})

	t.Run("an explicit empty list clears the existing one", func(t *testing.T) {
		r := HeaderRule{Exclude: []string{"Authorization"}}
		(&YAMLHeaderRule{Exclude: []string{}}).Apply(&r)
		if len(r.Exclude) != 0 {
			t.Errorf("expected Exclude cleared, got %v", r.Exclude)
		}
	})

	t.Run("a non-empty list replaces the existing one wholesale", func(t *testing.T) {
		r := HeaderRule{Exclude: []string{"Authorization"}}
		(&YAMLHeaderRule{Exclude: []string{"X-Api-Key", "X-Other"}}).Apply(&r)
		want := []string{"X-Api-Key", "X-Other"}
		if len(r.Exclude) != len(want) || r.Exclude[0] != want[0] || r.Exclude[1] != want[1] {
			t.Errorf("got %v, want %v", r.Exclude, want)
		}
	})
}

func TestYAMLHTTPRules_Apply(t *testing.T) {
	t.Run("zero value leaves every sub-rule untouched", func(t *testing.T) {
		r := HTTPRules{
			RequestBody:     Rule{Enabled: true},
			ResponseHeaders: HeaderRule{Enabled: true, Mask: []string{"X-Api-Key"}},
		}
		want := r
		YAMLHTTPRules{}.Apply(&r)
		if r.RequestBody != want.RequestBody {
			t.Errorf("RequestBody changed: got %+v, want %+v", r.RequestBody, want.RequestBody)
		}
		if r.ResponseHeaders.Enabled != want.ResponseHeaders.Enabled || len(r.ResponseHeaders.Mask) != 1 {
			t.Errorf("ResponseHeaders changed: got %+v, want %+v", r.ResponseHeaders, want.ResponseHeaders)
		}
	})

	t.Run("each sub-rule overlays independently", func(t *testing.T) {
		r := HTTPRules{}
		enabled := true
		YAMLHTTPRules{
			RequestBody:    &YAMLRule{Enabled: &enabled},
			RequestHeaders: &YAMLHeaderRule{Exclude: []string{"Authorization"}},
		}.Apply(&r)

		if !r.RequestBody.Enabled {
			t.Error("RequestBody.Enabled not applied")
		}
		if r.ResponseBody.Enabled {
			t.Error("ResponseBody should be untouched")
		}
		if len(r.RequestHeaders.Exclude) != 1 || r.RequestHeaders.Exclude[0] != "Authorization" {
			t.Errorf("RequestHeaders.Exclude not applied: got %v", r.RequestHeaders.Exclude)
		}
		if r.ResponseHeaders.Enabled {
			t.Error("ResponseHeaders should be untouched")
		}
	})
}
