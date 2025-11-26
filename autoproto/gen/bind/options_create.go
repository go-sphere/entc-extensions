package bind

import (
	_ "embed"
	"strings"
)

//go:embed options.go
var optionsFile string

func CreateOptionsFile(pkg string) string {
	return strings.Replace(optionsFile, "package bind", "package "+pkg, 1)
}
