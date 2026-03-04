package entproto

import "entgo.io/ent/entc/gen"

var (
	snake  = gen.Funcs["snake"].(func(string) string)
	pascal = gen.Funcs["pascal"].(func(string) string)
)
