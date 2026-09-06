package entproto

import "entgo.io/ent/entc"

func ExampleNewExtension() {
	extension, err := NewExtension(
		WithProtoDir("./proto"),
		WithAutoFill(),
	)
	if err != nil {
		panic(err)
	}
	_ = entc.Extensions(extension)
}
