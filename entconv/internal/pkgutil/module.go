// Package pkgutil maps between Go module import paths and directories on disk
// for entconv.
package pkgutil

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// Constants for module detection
const (
	GoModFile     = "go.mod"
	SchemaDirName = "schema"
	EntDirName    = "ent"
)

// FindModuleRoot finds the module root by searching for go.mod file.
func FindModuleRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, GoModFile)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// GetModulePath reads the module path from go.mod.
func GetModulePath(moduleRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(moduleRoot, GoModFile))
	if err != nil {
		return "", err
	}
	modulePath := modfile.ModulePath(data)
	if modulePath == "" {
		return "", fmt.Errorf("module path not found in go.mod")
	}
	return modulePath, nil
}

// ResolveEntPackage returns entPackage when provided, or derives it from
// schemaPath relative to the enclosing module's go.mod.
func ResolveEntPackage(schemaPath, entPackage string) (string, error) {
	if entPackage != "" {
		return entPackage, nil
	}
	return resolveEntPackageFromSchema(schemaPath)
}

func resolveEntPackageFromSchema(schemaPath string) (string, error) {
	absPath, err := filepath.Abs(schemaPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve schema path: %w", err)
	}

	moduleRoot, err := FindModuleRoot(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to find module root: %w", err)
	}

	modulePath, err := GetModulePath(moduleRoot)
	if err != nil {
		return "", fmt.Errorf("failed to get module path: %w", err)
	}

	relPath, err := filepath.Rel(moduleRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}

	schemaDir := filepath.Base(absPath)
	entDir := strings.Replace(schemaDir, SchemaDirName, EntDirName, 1)
	entPackageDir := filepath.ToSlash(filepath.Dir(relPath))

	return path.Join(modulePath, entPackageDir, entDir), nil
}

// DiskDirForImportPath maps a fully-qualified import path that lives inside the
// module containing dir (found by walking up to its go.mod) to its absolute
// directory on disk. Import paths belonging to other modules are not resolved
// (the caller must decide how to handle that case).
func DiskDirForImportPath(dir, importPath string) (string, error) {
	moduleRoot, err := FindModuleRoot(dir)
	if err != nil {
		return "", err
	}
	modulePath, err := GetModulePath(moduleRoot)
	if err != nil {
		return "", err
	}
	// A module may use a major-version suffix (e.g. /v2); the import path shares
	// the module path prefix. Use the longest matching prefix so subpackages
	// resolve correctly.
	rel, ok := strings.CutPrefix(importPath, modulePath)
	if !ok || (rel != "" && !strings.HasPrefix(rel, "/")) {
		return "", fmt.Errorf("import path %q is not inside module %q", importPath, modulePath)
	}
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return moduleRoot, nil
	}
	return filepath.Join(moduleRoot, filepath.FromSlash(rel)), nil
}
