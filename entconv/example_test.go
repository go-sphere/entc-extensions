package entconv

func ExampleNewOptions() {
	options := NewOptions(
		WithSchemaPath("./internal/pkg/database/schema"),
		WithEntPackagePath("example.com/project/internal/pkg/database/ent"),
		WithIDType("int64"),
		WithProtoFile("./api/entpb/entpb.pb.go"),
		WithConvPackage("entmap"),
		WithProtoPackagePath("example.com/project/api/entpb"),
		WithProtoAlias("entpb"),
		WithOutDir("./internal/pkg/render/entmap"),
	)
	_ = options
}
