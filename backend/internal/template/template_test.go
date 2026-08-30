package template

import (
	"errors"
	"strings"
	"testing"
)

// A mapping is checked against the slots the template declares, so a template
// never has to defend itself against a half-filled configuration.
func TestValidate(t *testing.T) {
	queryColumns := []string{"region", "revenue"}

	tests := []struct {
		name      string
		template  Template
		config    Config
		columns   []string
		wantError bool
		wantText  string
	}{
		{
			name:     "complete mapping",
			template: Tabular{},
			config:   Config{Columns: map[string][]string{"columns": {"region", "revenue"}}},
			columns:  queryColumns,
		},
		{
			name:      "required columns slot left empty",
			template:  Tabular{},
			config:    Config{},
			columns:   queryColumns,
			wantError: true,
			wantText:  "Table columns",
		},
		{
			name:     "optional text slot left empty",
			template: Tabular{},
			config:   Config{Columns: map[string][]string{"columns": {"region"}}},
			columns:  queryColumns,
		},
		{
			name:     "mapped column the query does not return",
			template: Tabular{},
			config:   Config{Columns: map[string][]string{"columns": {"profit"}}},
			columns:  queryColumns,
			// The user has to know which mapping went stale, not just that one did.
			wantError: true,
			wantText:  `"profit"`,
		},
		{
			name:     "columns are not checked when the query has not run",
			template: Tabular{},
			config:   Config{Columns: map[string][]string{"columns": {"profit"}}},
			columns:  nil,
		},
		{
			name:     "single column slot given two columns",
			template: GroupedTotals{},
			config: Config{Columns: map[string][]string{
				"group":   {"region", "revenue"},
				"metrics": {"revenue"},
			}},
			columns:   queryColumns,
			wantError: true,
			wantText:  "single column",
		},
		{
			name:      "grouped template needs a grouping column",
			template:  GroupedTotals{},
			config:    Config{Columns: map[string][]string{"metrics": {"revenue"}}},
			columns:   queryColumns,
			wantError: true,
			wantText:  "Group rows by",
		},
		{
			name:      "kpi template needs a metric",
			template:  KPISummary{},
			config:    Config{Columns: map[string][]string{"columns": {"region"}}},
			columns:   queryColumns,
			wantError: true,
			wantText:  "Headline metrics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.template, tt.config, tt.columns)
			if tt.wantError {
				if err == nil {
					t.Fatal("got no error, want an invalid configuration")
				}
				if !errors.Is(err, ErrInvalidConfig) {
					t.Errorf("error %v does not wrap ErrInvalidConfig", err)
				}
				if !strings.Contains(err.Error(), tt.wantText) {
					t.Errorf("error %q does not mention %q", err, tt.wantText)
				}
				return
			}
			if err != nil {
				t.Fatalf("got error %v, want none", err)
			}
		})
	}
}

// A required text slot is only satisfied by text with something in it.
func TestValidateRequiredTextSlot(t *testing.T) {
	required := stubTemplate{slots: []Slot{
		{Key: "title", Label: "Title", Kind: SlotText, Required: true},
	}}

	if err := Validate(required, Config{Text: map[string]string{"title": "   "}}, nil); err == nil {
		t.Error("got no error for a whitespace-only required title, want one")
	}
	if err := Validate(required, Config{Text: map[string]string{"title": "Q3"}}, nil); err != nil {
		t.Errorf("got error %v for a filled title, want none", err)
	}
}

func TestValidateRejectsUnknownSlotKind(t *testing.T) {
	err := Validate(stubTemplate{slots: []Slot{{Key: "mystery", Kind: "colour"}}}, Config{}, nil)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("got %v, want an invalid configuration error", err)
	}
}

func TestConfigAccessors(t *testing.T) {
	cfg := Config{
		Columns: map[string][]string{"metrics": {"revenue", "units"}},
		Text:    map[string]string{"title": "  Q3 revenue  "},
	}

	if got := cfg.ColumnFor("metrics"); got != "revenue" {
		t.Errorf("ColumnFor: got %q, want the first mapped column", got)
	}
	if got := cfg.ColumnFor("missing"); got != "" {
		t.Errorf("ColumnFor on an unmapped slot: got %q, want an empty string", got)
	}
	if got := cfg.TextFor("title"); got != "Q3 revenue" {
		t.Errorf("TextFor: got %q, want the trimmed text", got)
	}
	if got := len(cfg.ColumnsFor("metrics")); got != 2 {
		t.Errorf("ColumnsFor: got %d columns, want 2", got)
	}
}

// stubTemplate exercises slot handling that no starter template declares.
type stubTemplate struct {
	slots []Slot
}

func (s stubTemplate) Meta() Meta {
	return Meta{ID: "stub", Name: "Stub", Slots: s.slots}
}

func (s stubTemplate) Render(Data, Config) ([]byte, error) { return nil, nil }
