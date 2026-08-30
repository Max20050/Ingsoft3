// Package render turns a report's document into the files a client asked for.
//
// A report is configured with one or more output formats. Rendering takes the
// document a template projected — title, blocks, tables, totals — and produces
// one Artifact per format, in memory: a scheduled report is generated on a
// server that shouldn't need a writable disk, and the delivery layer wants
// bytes to attach, not paths to clean up.
//
// Formats are pluggable. Each is a Renderer registered in renderers; nothing
// outside this package names a format except the user's own configuration, so
// adding one (say HTML or Markdown) is a new file and one registry entry.
//
// See documents/v1/report-rendering.md.
package render

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Max20050/docuwave/internal/template"
)

// Format identifies an output file format.
type Format string

const (
	FormatPDF  Format = "pdf"
	FormatXLSX Format = "xlsx"
	FormatCSV  Format = "csv"
)

// ErrUnknownFormat is returned for a format this server can't produce. It's the
// caller's to fix, so it carries the name they asked for.
var ErrUnknownFormat = errors.New("unsupported output format")

// ErrNoFormats is returned when a report would be rendered into nothing.
var ErrNoFormats = errors.New("choose at least one output format")

// Renderer produces one format's bytes from a document. Implementations must be
// safe to share: one value per format is registered at package level and used
// concurrently.
type Renderer interface {
	// Extension is the file extension the format uses, without the dot.
	Extension() string
	// ContentType is the MIME type the file is served and attached with.
	ContentType() string
	// Render produces the complete file.
	Render(template.Doc) ([]byte, error)
}

// registration pairs a format with the renderer that produces it.
type registration struct {
	format   Format
	renderer Renderer
}

// renderers is the format registry, in the order formats are offered to the
// user and stored on a report.
var renderers = []registration{
	{FormatPDF, pdfRenderer{}},
	{FormatXLSX, xlsxRenderer{}},
	{FormatCSV, csvRenderer{}},
}

// Formats lists every format a report can be configured with.
func Formats() []Format {
	formats := make([]Format, 0, len(renderers))
	for _, entry := range renderers {
		formats = append(formats, entry.format)
	}
	return formats
}

// rendererFor looks up the renderer for a format.
func rendererFor(format Format) (Renderer, error) {
	for _, entry := range renderers {
		if entry.format == format {
			return entry.renderer, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownFormat, string(format))
}

// ParseFormat reads a stored or requested format name.
func ParseFormat(value string) (Format, error) {
	format := Format(strings.ToLower(strings.TrimSpace(value)))
	if _, err := rendererFor(format); err != nil {
		return "", err
	}
	return format, nil
}

// ParseFormats reads a report's configured formats. Duplicates are dropped and
// the result is in registry order, so a report's formats are stored and shown
// the same way however the client listed them.
func ParseFormats(values []string) ([]Format, error) {
	chosen := make(map[Format]bool, len(values))
	for _, value := range values {
		format, err := ParseFormat(value)
		if err != nil {
			return nil, err
		}
		chosen[format] = true
	}
	if len(chosen) == 0 {
		return nil, ErrNoFormats
	}

	formats := make([]Format, 0, len(chosen))
	for _, format := range Formats() {
		if chosen[format] {
			formats = append(formats, format)
		}
	}
	return formats, nil
}

// Strings renders formats for storage and for JSON.
func Strings(formats []Format) []string {
	values := make([]string, 0, len(formats))
	for _, format := range formats {
		values = append(values, string(format))
	}
	return values
}

// Includes reports whether a report configured for these formats can be asked
// for that one.
func Includes(formats []Format, format Format) bool {
	return slices.Contains(formats, format)
}

// Artifact is a rendered report file held in memory, ready for the delivery
// layer to attach or a browser to download.
type Artifact struct {
	Format      Format `json:"format"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Bytes       []byte `json:"-"`
}

// Render produces one file from a document. name is the report's name, which
// becomes the filename the recipient sees.
func Render(doc template.Doc, format Format, name string) (Artifact, error) {
	renderer, err := rendererFor(format)
	if err != nil {
		return Artifact{}, err
	}

	content, err := renderer.Render(doc)
	if err != nil {
		return Artifact{}, fmt.Errorf("render %s: %w", format, err)
	}

	return Artifact{
		Format:      format,
		Filename:    Filename(name, doc.GeneratedAt, renderer.Extension()),
		ContentType: renderer.ContentType(),
		Bytes:       content,
	}, nil
}

// RenderAll produces every format a report is configured for. It stops at the
// first failure: a delivery carrying some of the files a client asked for isn't
// the report they configured.
func RenderAll(doc template.Doc, formats []Format, name string) ([]Artifact, error) {
	if len(formats) == 0 {
		return nil, ErrNoFormats
	}

	artifacts := make([]Artifact, 0, len(formats))
	for _, format := range formats {
		artifact, err := Render(doc, format, name)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

// maxSlugLength keeps a filename comfortably inside what mail clients and file
// systems handle once the date and extension are added.
const maxSlugLength = 60

// Filename names a report's file: the report's own name, slugged, dated so a
// recurring report's files don't collide in a downloads folder.
func Filename(name string, at time.Time, extension string) string {
	slug := slugify(name)
	if slug == "" {
		slug = "report"
	}
	if !at.IsZero() {
		slug += "-" + at.UTC().Format("2006-01-02")
	}
	return slug + "." + extension
}

// slugify reduces a report name to lowercase words joined by hyphens. Report
// names are the user's own text, so this is also what keeps a name out of a
// Content-Disposition header or a file path.
func slugify(name string) string {
	var slug strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			slug.WriteRune(r)
			hyphen = false
		case !hyphen && slug.Len() > 0:
			slug.WriteByte('-')
			hyphen = true
		}
		if slug.Len() >= maxSlugLength {
			break
		}
	}
	return strings.Trim(slug.String(), "-")
}
