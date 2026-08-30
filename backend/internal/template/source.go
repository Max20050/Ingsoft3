package template

import "context"

// Source is where the rendering pipeline looks a template up, in place of a
// concrete Registry: List names what's available to a user, Get resolves one
// by ID. The built-in Registry isn't itself a Source — a Registry predates
// per-user templates and its List/Get take no userID — so RegistrySource
// adapts it, and CompositeSource layers a user's own templates and their
// archive state on top.
//
// See documents/v1/report-templates.md.
type Source interface {
	// List describes every template available to build a new report with, for
	// the given user, in the order they should be offered.
	List(ctx context.Context, userID string) ([]Meta, error)
	// Get resolves one template by ID for the given user. It returns
	// ErrUnknownTemplate if none matches — whether because the ID doesn't
	// exist at all or because it names another user's custom template.
	Get(ctx context.Context, userID, id string) (Template, error)
}

// RegistrySource adapts a Registry to Source. The starters are available to
// everyone, so it ignores userID.
type RegistrySource struct {
	registry *Registry
}

// NewRegistrySource wraps a Registry as a Source.
func NewRegistrySource(registry *Registry) RegistrySource {
	return RegistrySource{registry: registry}
}

func (s RegistrySource) List(_ context.Context, _ string) ([]Meta, error) {
	return s.registry.List(), nil
}

func (s RegistrySource) Get(_ context.Context, _ string, id string) (Template, error) {
	return s.registry.Get(id)
}

// CustomSource is what a store of user-owned custom templates provides to
// plug into a CompositeSource: list and resolve a user's own templates, both
// scoped to the owning user so one account never sees another's designs.
type CustomSource interface {
	List(ctx context.Context, userID string) ([]CustomTemplate, error)
	Get(ctx context.Context, userID, id string) (CustomTemplate, error)
}

// ArchiveSource is the per-user overlay of which template IDs — built-in or
// custom — a user has archived. Archiving doesn't touch the template itself,
// so it applies uniformly to both kinds.
type ArchiveSource interface {
	Archived(ctx context.Context, userID string) (map[string]bool, error)
}

// CompositeSource merges the built-in starters with a user's own custom
// templates, honoring their per-user archive state, into the one Source the
// report pipeline depends on. This is the seam the v2 builder plugs into: the
// pipeline never has to know a template came from a database row rather than
// Starters().
type CompositeSource struct {
	builtins *Registry
	custom   CustomSource
	archive  ArchiveSource
}

// NewCompositeSource builds a Source over the built-in registry and a user's
// saved custom templates.
func NewCompositeSource(builtins *Registry, custom CustomSource, archive ArchiveSource) *CompositeSource {
	return &CompositeSource{builtins: builtins, custom: custom, archive: archive}
}

// List returns the templates available to build a new report with: every
// built-in, then the user's own custom templates in the order they were
// created, minus whichever of either this user has archived. Archiving never
// removes a template — it only excludes it from this default listing.
func (s *CompositeSource) List(ctx context.Context, userID string) ([]Meta, error) {
	archived, err := s.archived(ctx, userID)
	if err != nil {
		return nil, err
	}

	metas := make([]Meta, 0, len(s.builtins.List()))
	for _, meta := range s.builtins.List() {
		if archived[meta.ID] {
			continue
		}
		metas = append(metas, meta)
	}

	customs, err := s.custom.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, c := range customs {
		if archived[c.ID] {
			continue
		}
		metas = append(metas, c.Meta())
	}

	return metas, nil
}

// Get resolves a template by ID, checking the built-ins first and falling
// back to the user's own custom templates. Archived templates still resolve —
// archiving only affects the default listing, never a report that already
// references the template.
func (s *CompositeSource) Get(ctx context.Context, userID, id string) (Template, error) {
	if t, err := s.builtins.Get(id); err == nil {
		return t, nil
	}
	return s.custom.Get(ctx, userID, id)
}

func (s *CompositeSource) archived(ctx context.Context, userID string) (map[string]bool, error) {
	if s.archive == nil {
		return map[string]bool{}, nil
	}
	return s.archive.Archived(ctx, userID)
}

// RenderWith resolves a template from a Source, checks the mapping against
// the data it's about to render, and produces the document — the Source
// equivalent of Registry.Render, for a pipeline that has to look templates up
// per user rather than in a fixed, process-wide set.
func RenderWith(ctx context.Context, source Source, userID, id string, data Data, cfg Config) ([]byte, error) {
	t, err := source.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if err := Validate(t, cfg, data.Columns); err != nil {
		return nil, err
	}
	return t.Render(data, cfg)
}

// DocumentWith resolves a template from a Source and projects the document it
// would render, the Source equivalent of Registry.Document.
func DocumentWith(ctx context.Context, source Source, userID, id string, data Data, cfg Config) (Doc, error) {
	t, err := source.Get(ctx, userID, id)
	if err != nil {
		return Doc{}, err
	}
	if err := Validate(t, cfg, data.Columns); err != nil {
		return Doc{}, err
	}
	return DocumentOf(t, data, cfg), nil
}

// ValidateConfigWith resolves a template from a Source and checks a slot
// mapping against it without needing the query's output columns, the Source
// equivalent of Registry.ValidateConfig.
func ValidateConfigWith(ctx context.Context, source Source, userID, id string, cfg Config) error {
	t, err := source.Get(ctx, userID, id)
	if err != nil {
		return err
	}
	return Validate(t, cfg, nil)
}
