package gofile

import (
	"go/format"
	"os"

	"golang.org/x/tools/imports"
)

func WriteFile(fileName string, content []byte) error {
	content, err := imports.Process(fileName, content, nil)
	if err != nil {
		return err
	}
	content, err = format.Source(content)
	if err != nil {
		return err
	}
	err = os.WriteFile(fileName, content, 0o644)
	if err != nil {
		return err
	}
	return nil
}
