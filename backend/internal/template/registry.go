package template

import "fmt"

// Registry is the set of templates the rendering layer can use, in the order
// they're offered to the user.
//
// It is the seam a future template source plugs into: a drag-and-drop builder
// supplies values that implement Template, they get registered here, and
// nothing downstream changes.
type Registry struct {
	order []Template
	byID  map[string]Template
}

// NewRegistry registers the given templates. It panics on a duplicate ID,
// because that's a wiring mistake and the registry is built at startup.
func NewRegistry(templates ...Template) *Registry {
	r := &Registry{byID: make(map[string]Template, len(templates))}
	for _, t := range templates {
		id := t.Meta().ID
		if _, exists := r.byID[id]; exists {
			panic(fmt.Sprintf("template: duplicate template ID %q", id))
		}
		r.byID[id] = t
		r.order = append(r.order, t)
	}
	return r
}

// Starters returns the built-in templates, in the order they're presented.
func Starters() []Template {
	return []Template{Tabular{}, GroupedTotals{}, KPISummary{}}
}

// Get returns a registered template. It returns ErrUnknownTemplate if the ID
// isn't registered.
func (r *Registry) Get(id string) (Template, error) {
	t, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTemplate, id)
	}
	return t, nil
}

// List describes every registered template, in registration order.
func (r *Registry) List() []Meta {
	metas := make([]Meta, 0, len(r.order))
	for _, t := range r.order {
		metas = append(metas, t.Meta())
	}
	return metas
}

// ValidateConfig checks a slot mapping against a template without needing the
// query's output columns, which is what saving a report configuration can do.
func (r *Registry) ValidateConfig(id string, cfg Config) error {
	t, err := r.Get(id)
	if err != nil {
		return err
	}
	return Validate(t, cfg, nil)
}

// Render resolves a template, checks the mapping against the data it's about to
// render, and produces the document.
func (r *Registry) Render(id string, data Data, cfg Config) ([]byte, error) {
	t, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	if err := Validate(t, cfg, data.Columns); err != nil {
		return nil, err
	}
	return t.Render(data, cfg)
}

// Document resolves a template and projects the document it would render, as
// the structure the file formats are built from. It gates on the same mapping
// check as Render, so a report that renders is a report that downloads.
func (r *Registry) Document(id string, data Data, cfg Config) (Doc, error) {
	t, err := r.Get(id)
	if err != nil {
		return Doc{}, err
	}
	if err := Validate(t, cfg, data.Columns); err != nil {
		return Doc{}, err
	}
	return DocumentOf(t, data, cfg), nil
}
