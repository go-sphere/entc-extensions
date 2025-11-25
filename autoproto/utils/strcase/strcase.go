package strcase

import "entgo.io/ent/entc/gen"

func ToSnake(str string) string {
	return gen.Funcs["snake"].(func(s string) string)(str)
}
