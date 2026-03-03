module github.com/go-sphere/entc-extensions/testdata

go 1.26.0

replace github.com/go-sphere/entc-extensions/entconv => ../entconv

replace github.com/go-sphere/entc-extensions/entproto => ../entproto

replace github.com/go-sphere/entc-extensions/entcrud => ../entcrud

require (
	entgo.io/ent v0.14.5
	github.com/go-sphere/entc-extensions/entconv v0.0.0
	github.com/go-sphere/entc-extensions/entcrud v0.0.0-00010101000000-000000000000
	github.com/go-sphere/entc-extensions/entproto v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.36.11
)

require (
	ariga.io/atlas v0.32.1-0.20250325101103-175b25e1c1b9 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/bmatcuk/doublestar v1.3.4 // indirect
	github.com/go-openapi/inflect v0.19.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/hcl/v2 v2.18.1 // indirect
	github.com/jhump/protoreflect v1.18.0 // indirect
	github.com/jhump/protoreflect/v2 v2.0.0-beta.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/zclconf/go-cty v1.14.4 // indirect
	github.com/zclconf/go-cty-yaml v1.1.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/tools v0.42.0 // indirect
)
