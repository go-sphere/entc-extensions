package conf

import (
	"github.com/go-sphere/entc-extensions/entcrud/internal/inspect"
)

// FilesConf holds configuration for generating multiple Go files.
type FilesConf struct {
	Dir                  string
	Package              string
	RemoveBeforeGenerate bool
	StrictTypeCheck      bool
	ExtraImports         []inspect.Import
	Entities             []*EntityConf
}

// NewFilesConf creates a new FilesConf with the specified directory, package name, and entities.
func NewFilesConf(dir string, pkg string, entities ...*EntityConf) *FilesConf {
	return &FilesConf{
		Dir:                  dir,
		Package:              pkg,
		RemoveBeforeGenerate: false,
		StrictTypeCheck:      true,
		ExtraImports:         nil,
		Entities:             entities,
	}
}

type FilesOption func(*FilesConf)

// Apply applies file-level options and returns itself for chaining.
func (c *FilesConf) Apply(opts ...FilesOption) *FilesConf {
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// WithStrictTypeCheck controls whether incompatible field types fail generation.
// Default is true.
func WithStrictTypeCheck(strict bool) FilesOption {
	return func(c *FilesConf) {
		c.StrictTypeCheck = strict
	}
}

// EntityConf holds configuration for generating binding functions between different data structures.
type EntityConf struct {
	Source                any
	Target                any
	Actions               []any
	IgnoreFields          []string
	CustomFieldConverters map[string]any
}

// NewEntity creates a new EntityConf with auto-detected package names.
func NewEntity(source, target any, actions []any, opts ...Option) *EntityConf {
	ctx := &EntityConf{
		Source:                source,
		Target:                target,
		Actions:               actions,
		IgnoreFields:          nil,
		CustomFieldConverters: nil,
	}
	for _, opt := range opts {
		opt(ctx)
	}
	return ctx
}

// Option is a function that modifies EntityConf.
type Option func(*EntityConf)

// WithIgnoreFields specifies field names to ignore during generation.
func WithIgnoreFields(fields ...string) Option {
	return func(c *EntityConf) {
		c.IgnoreFields = fields
	}
}

// WithCustomFieldConverter sets custom field converters.
func WithCustomFieldConverter(field string, converter any) Option {
	return func(c *EntityConf) {
		if c.CustomFieldConverters == nil {
			c.CustomFieldConverters = make(map[string]any)
		}
		c.CustomFieldConverters[field] = converter
	}
}
