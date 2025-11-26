package conf

import (
	"github.com/go-sphere/entc-extensions/entgen/inspect"
)

type FilesConf struct {
	Dir                  string
	Package              string
	RemoveBeforeGenerate bool
	Entities             []*EntityConf
	ExtraImports         [][2]string
}

// EntityConf holds configuration for generating binding functions between different data structures.
// It's commonly used for generating code that converts between ORM entities and Protocol Buffer messages.
type EntityConf struct {
	Source        any      // ent entity, e.g. ent.Example
	Target        any      // protobuf entity, e.g. entpb.Example
	Actions       []any    // ent operation, e.g. ent.ExampleCreate, ent.ExampleUpdateOne
	IgnoreFields  []string // fields to ignore, e.g.  example.FieldID, example.FieldCreatedAt
	SourcePkgName string   // package name of Source, e.g. "ent"
	TargetPkgName string   // package name of Target, e.g. "entpb"
}

type EntityConfOption func(*EntityConf)

// NewEntity creates a new configuration for binding function generation.
// It automatically determines package names from the provided Source and Target types.
func NewEntity(source, target any, actions []any, opts ...EntityConfOption) *EntityConf {
	ctx := &EntityConf{
		Source:        source,
		Target:        target,
		Actions:       actions,
		IgnoreFields:  nil,
		SourcePkgName: inspect.ExtractPackageName(source),
		TargetPkgName: inspect.ExtractPackageName(target),
	}
	for _, opt := range opts {
		opt(ctx)
	}
	return ctx
}

// WithSourcePkgName sets a custom package name for the Source type.
// Returns the modified configuration for method chaining.
func WithSourcePkgName(pkgName string) EntityConfOption {
	return func(conf *EntityConf) {
		conf.SourcePkgName = pkgName
	}
}

// WithTargetPkgName sets a custom package name for the Target type.
// Returns the modified configuration for method chaining.
func WithTargetPkgName(pkgName string) EntityConfOption {
	return func(conf *EntityConf) {
		conf.TargetPkgName = pkgName
	}
}

// WithIgnoreFields specifies field names that should be ignored during binding generation.
// Returns the modified configuration for method chaining.
func WithIgnoreFields(fields ...string) EntityConfOption {
	return func(conf *EntityConf) {
		conf.IgnoreFields = fields
	}
}
