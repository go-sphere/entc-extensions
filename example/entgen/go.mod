module github.com/go-sphere/entc-extensions/example/entgen

go 1.25

replace (
	github.com/go-sphere/entc-extensions/autoproto => ../../autoproto
	github.com/go-sphere/entc-extensions/entgen => ../../entgen
)

require (
	entgo.io/ent v0.14.5
	github.com/go-sphere/entc-extensions/autoproto v0.0.0-00010101000000-000000000000
	github.com/go-sphere/entc-extensions/entgen v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.36.10
)

require (
	ariga.io/atlas v0.38.0 // indirect
	entgo.io/contrib v0.7.0 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/bmatcuk/doublestar v1.3.4 // indirect
	github.com/go-openapi/inflect v0.21.3 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/hcl/v2 v2.24.0 // indirect
	github.com/jhump/protoreflect v1.10.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/zclconf/go-cty v1.17.0 // indirect
	github.com/zclconf/go-cty-yaml v1.1.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/tools v0.39.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
