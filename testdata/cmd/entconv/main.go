package main

import (
	"log"

	"github.com/go-sphere/entc-extensions/entconv"
)

func main() {
	if err := entconv.GenerateConverterFileWithOptions(); err != nil {
		log.Fatalf("error: %v", err)
	}
}
