package template

import (
	"errors"
	"strings"
	"testing"
)

func TestStartersAreRegistered(t *testing.T) {
	registry := NewRegistry(Starters()...)

	metas := registry.List()
	if len(metas) < 2 {
		t.Fatalf("got %d templates, want at least the 2 the product promises", len(metas))
	}

	for _, meta := range metas {
		if meta.ID == "" || meta.Name == "" || meta.Description == "" {
			t.Errorf("template %+v is missing user-facing detail", meta)
		}
		if len(meta.Slots) == 0 {
			t.Errorf("template %q declares no slots", meta.ID)
		}
		if _, err := registry.Get(meta.ID); err != nil {
			t.Errorf("listed template %q is not retrievable: %v", meta.ID, err)
		}
	}
}

// Every slot has to be renderable by the UI, which means a known kind and a
// label to put beside its control.
func TestStarterSlotsAreWellFormed(t *testing.T) {
	for _, template := range Starters() {
		meta := template.Meta()
		seen := make(map[string]bool)
		for _, slot := range meta.Slots {
			if slot.Label == "" || slot.Description == "" {
				t.Errorf("%s: slot %q is missing a label or description", meta.ID, slot.Key)
			}
			switch slot.Kind {
			case SlotText, SlotColumn, SlotColumns:
			default:
				t.Errorf("%s: slot %q has unknown kind %q", meta.ID, slot.Key, slot.Kind)
			}
			if seen[slot.Key] {
				t.Errorf("%s: slot %q is declared twice", meta.ID, slot.Key)
			}
			seen[slot.Key] = true
		}
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	_, err := NewRegistry(Starters()...).Get("drag-and-drop")
	if !errors.Is(err, ErrUnknownTemplate) {
		t.Fatalf("got %v, want ErrUnknownTemplate", err)
	}
	if !strings.Contains(err.Error(), "drag-and-drop") {
		t.Errorf("error %q does not name the template asked for", err)
	}
}

// Registering is all it takes to plug a template in: nothing in the rendering
// path knows about the starter types.
func TestRegistryRendersAnyRegisteredTemplate(t *testing.T) {
	registry := NewRegistry(fixedTemplate{})

	html, err := registry.Render("fixed", Data{Columns: []string{"a"}}, Config{})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if string(html) != "<p>fixed</p>" {
		t.Errorf("got %q, want the template's own output", html)
	}
}

func TestRegistryRenderChecksConfig(t *testing.T) {
	registry := NewRegistry(Starters()...)

	// "profit" isn't in the query output, so this must fail before rendering.
	_, err := registry.Render("tabular",
		Data{Columns: []string{"region"}},
		Config{Columns: map[string][]string{"columns": {"profit"}}})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("got %v, want ErrInvalidConfig", err)
	}
}

func TestRegistryValidateConfig(t *testing.T) {
	registry := NewRegistry(Starters()...)

	if err := registry.ValidateConfig("tabular", Config{}); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("got %v for an empty mapping, want ErrInvalidConfig", err)
	}
	if err := registry.ValidateConfig("nope", Config{}); !errors.Is(err, ErrUnknownTemplate) {
		t.Errorf("got %v for an unregistered ID, want ErrUnknownTemplate", err)
	}
	// Column names can't be checked yet — the report's query hasn't run.
	cfg := Config{Columns: map[string][]string{"columns": {"anything"}}}
	if err := registry.ValidateConfig("tabular", cfg); err != nil {
		t.Errorf("got error %v, want none", err)
	}
}

func TestNewRegistryPanicsOnDuplicateID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("registering two templates with one ID did not panic")
		}
	}()
	NewRegistry(fixedTemplate{}, fixedTemplate{})
}

// fixedTemplate is a minimal Template: proof that the interface is the only
// thing the registry and the rendering path depend on.
type fixedTemplate struct{}

func (fixedTemplate) Meta() Meta {
	return Meta{ID: "fixed", Name: "Fixed", Description: "Always the same output."}
}

func (fixedTemplate) Render(Data, Config) ([]byte, error) { return []byte("<p>fixed</p>"), nil }
