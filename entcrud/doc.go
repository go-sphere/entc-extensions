// Package entgen generates "bind" helpers that copy fields from protobuf
// messages into ent mutation builders such as ent.UserCreate and
// ent.UserUpdateOne. Its import path is
// github.com/go-sphere/entc-extensions/entcrud; the package name is entgen.
//
// entgen is a plain code generator that works by reflection on real types, so
// it runs from a small main program that imports the already generated ent and
// protobuf packages. Describe the output with
// [github.com/go-sphere/entc-extensions/entcrud/conf] and call [BindFiles].
//
// # Usage
//
//	import (
//		"log"
//
//		entgen "github.com/go-sphere/entc-extensions/entcrud"
//		"github.com/go-sphere/entc-extensions/entcrud/conf"
//
//		"example.com/app/api/entpb"
//		"example.com/app/ent"
//		"example.com/app/ent/user"
//		"example.com/app/internal/conv"
//	)
//
//	func main() {
//		cfg := conf.NewFilesConf("./internal/entbind", "entbind",
//			conf.NewEntity(
//				ent.User{}, entpb.User{},
//				[]any{ent.UserCreate{}, ent.UserUpdateOne{}},
//				conf.WithCustomFieldConverter(user.FieldBirthday, conv.ToEntUserBirthday),
//			),
//		)
//		if err := entgen.BindFiles(cfg); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// For the configuration above the generated package entbind contains:
//
//	func CreateUser(source *ent.UserCreate, target *entpb.User, options ...Option) *ent.UserCreate
//	func UpdateOneUser(source *ent.UserUpdateOne, target *entpb.User, options ...Option) *ent.UserUpdateOne
//
// plus an options.go file with Option, NewBindOptions, IgnoreField,
// KeepFieldsOnly, IgnoreSetZeroField, and ClearOnNilField, which callers pass
// to tune binding per call. Field names in those options are the ent field
// constants, such as user.FieldName.
//
// # Field matching
//
// A protobuf field is bound when the ent entity has a struct field with the
// same snake_case name and the action type has a matching SetXxx method. Fields
// missing on either side, fields without a setter (such as the ID or immutable
// fields), and edges are skipped. Non-pointer protobuf values are set even when
// zero unless the call passes IgnoreSetZeroField for that field. Pointer
// (proto3 optional) values are set when non-nil; a nil value clears the ent
// field only when the call passes ClearOnNilField and the builder has a
// ClearXxx method. Values are converted with a Go conversion when the types
// differ but are convertible. Incompatible types fail generation unless a
// custom converter is registered; see [conf.WithStrictTypeCheck].
//
// The generated code refers to the builder types as ent.XxxCreate, so the ent
// package must keep its default package name ent.
//
// # Related packages
//
// [github.com/go-sphere/entc-extensions/entconv] generates whole-entity
// converters (ToProtoT/ToEntT) instead of builder helpers, and
// [github.com/go-sphere/entc-extensions/entproto] generates the .proto files
// both packages consume.
package entgen
