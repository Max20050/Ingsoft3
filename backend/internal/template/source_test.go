package template

import (
	"context"
	"errors"
	"testing"
)

// fakeCustomSource is an in-memory CustomSource, standing in for CustomStore
// so CompositeSource's merging logic can be tested without a database.
type fakeCustomSource struct {
	byUser map[string][]CustomTemplate
}

func (f fakeCustomSource) List(_ context.Context, userID string) ([]CustomTemplate, error) {
	return f.byUser[userID], nil
}

func (f fakeCustomSource) Get(_ context.Context, userID, id string) (CustomTemplate, error) {
	for _, t := range f.byUser[userID] {
		if t.ID == id {
			return t, nil
		}
	}
	return CustomTemplate{}, ErrUnknownTemplate
}

// fakeArchiveSource is an in-memory ArchiveSource.
type fakeArchiveSource struct {
	byUser map[string]map[string]bool
}

func (f fakeArchiveSource) Archived(_ context.Context, userID string) (map[string]bool, error) {
	return f.byUser[userID], nil
}

func TestCompositeSourceMergesBuiltinsAndCustomTemplates(t *testing.T) {
	registry := NewRegistry(Starters()...)
	custom := fakeCustomSource{byUser: map[string][]CustomTemplate{
		"u-1": {{ID: "c-1", UserID: "u-1", Name: "Mine"}},
		"u-2": {{ID: "c-2", UserID: "u-2", Name: "Someone else's"}},
	}}
	source := NewCompositeSource(registry, custom, fakeArchiveSource{})

	metas, err := source.List(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(metas) != len(registry.List())+1 {
		t.Fatalf("got %d templates, want every starter plus u-1's one custom template", len(metas))
	}

	var sawOwn, sawOther bool
	for _, m := range metas {
		if m.ID == "c-1" {
			sawOwn = true
		}
		if m.ID == "c-2" {
			sawOther = true
		}
	}
	if !sawOwn {
		t.Error("u-1's own custom template is missing from their list")
	}
	if sawOther {
		t.Error("u-1 can see u-2's custom template — custom templates must be private per user")
	}
}

func TestCompositeSourceExcludesArchivedFromDefaultListing(t *testing.T) {
	registry := NewRegistry(Starters()...)
	custom := fakeCustomSource{byUser: map[string][]CustomTemplate{
		"u-1": {{ID: "c-1", UserID: "u-1", Name: "Mine"}},
	}}
	archive := fakeArchiveSource{byUser: map[string]map[string]bool{
		"u-1": {"tabular": true, "c-1": true},
	}}
	source := NewCompositeSource(registry, custom, archive)

	metas, err := source.List(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	for _, m := range metas {
		if m.ID == "tabular" || m.ID == "c-1" {
			t.Errorf("archived template %q is still in the default listing", m.ID)
		}
	}
	if len(metas) != len(registry.List())-1 {
		t.Fatalf("got %d templates, want every starter but the archived one", len(metas))
	}

	// A different user's archive state doesn't affect this one.
	metas, err = source.List(context.Background(), "u-2")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	found := false
	for _, m := range metas {
		if m.ID == "tabular" {
			found = true
		}
	}
	if !found {
		t.Error("u-2 should still see the starter u-1 archived — archiving is per user")
	}
}

// Archiving a template never breaks Get: a report that already references an
// archived template still renders. Get isn't affected by archive state at all.
func TestCompositeSourceGetIgnoresArchiveState(t *testing.T) {
	registry := NewRegistry(Starters()...)
	custom := fakeCustomSource{byUser: map[string][]CustomTemplate{
		"u-1": {{ID: "c-1", UserID: "u-1", Name: "Mine", Blocks: []BlockDef{
			{ID: "b1", Kind: BlockText, Title: "Note"},
		}}},
	}}
	archive := fakeArchiveSource{byUser: map[string]map[string]bool{
		"u-1": {"tabular": true, "c-1": true},
	}}
	source := NewCompositeSource(registry, custom, archive)

	if _, err := source.Get(context.Background(), "u-1", "tabular"); err != nil {
		t.Errorf("Get on an archived built-in returned error: %v", err)
	}
	if _, err := source.Get(context.Background(), "u-1", "c-1"); err != nil {
		t.Errorf("Get on an archived custom template returned error: %v", err)
	}
}

func TestCompositeSourceGetUnknownTemplate(t *testing.T) {
	registry := NewRegistry(Starters()...)
	custom := fakeCustomSource{byUser: map[string][]CustomTemplate{}}
	source := NewCompositeSource(registry, custom, fakeArchiveSource{})

	if _, err := source.Get(context.Background(), "u-1", "nope"); !errors.Is(err, ErrUnknownTemplate) {
		t.Errorf("got %v, want ErrUnknownTemplate", err)
	}
}

// A user can never resolve another user's custom template by ID.
func TestCompositeSourceGetIsolatesUsers(t *testing.T) {
	registry := NewRegistry(Starters()...)
	custom := fakeCustomSource{byUser: map[string][]CustomTemplate{
		"u-1": {{ID: "c-1", UserID: "u-1", Name: "Mine"}},
	}}
	source := NewCompositeSource(registry, custom, fakeArchiveSource{})

	if _, err := source.Get(context.Background(), "u-2", "c-1"); !errors.Is(err, ErrUnknownTemplate) {
		t.Errorf("got %v, want ErrUnknownTemplate for another user's custom template", err)
	}
}

func TestRegistrySourceIgnoresUserID(t *testing.T) {
	source := NewRegistrySource(NewRegistry(Starters()...))

	metasA, err := source.List(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	metasB, err := source.List(context.Background(), "u-2")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(metasA) != len(metasB) {
		t.Errorf("got different template counts for different users, want the starters for everyone")
	}

	if _, err := source.Get(context.Background(), "anyone", "tabular"); err != nil {
		t.Errorf("Get returned error: %v", err)
	}
}

func TestRenderWithAndDocumentWithValidateTheMapping(t *testing.T) {
	source := NewRegistrySource(NewRegistry(Starters()...))
	data := Data{Columns: []string{"region"}}

	_, err := RenderWith(context.Background(), source, "u-1", "tabular", data,
		Config{Columns: map[string][]string{"columns": {"profit"}}})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("RenderWith: got %v, want ErrInvalidConfig for a column the query does not return", err)
	}

	_, err = DocumentWith(context.Background(), source, "u-1", "tabular", data,
		Config{Columns: map[string][]string{"columns": {"profit"}}})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("DocumentWith: got %v, want ErrInvalidConfig for a column the query does not return", err)
	}

	if _, err := RenderWith(context.Background(), source, "u-1", "nope", data, Config{}); !errors.Is(err, ErrUnknownTemplate) {
		t.Errorf("RenderWith: got %v, want ErrUnknownTemplate", err)
	}
}

func TestValidateConfigWithDoesNotNeedQueryColumns(t *testing.T) {
	source := NewRegistrySource(NewRegistry(Starters()...))

	if err := ValidateConfigWith(context.Background(), source, "u-1", "tabular", Config{}); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("got %v for an empty mapping, want ErrInvalidConfig", err)
	}
	cfg := Config{Columns: map[string][]string{"columns": {"anything"}}}
	if err := ValidateConfigWith(context.Background(), source, "u-1", "tabular", cfg); err != nil {
		t.Errorf("got error %v, want none — the query has not run yet", err)
	}
}
