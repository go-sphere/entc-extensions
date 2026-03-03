package pkgutil

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Constants for module detection
const (
	GoModFile     = "go.mod"
	ModulePrefix  = "module "
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
	// Simple parsing: find line starting with "module "
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, ModulePrefix) {
			return strings.TrimPrefix(line, ModulePrefix), nil
		}
	}
	return "", fmt.Errorf("module path not found in go.mod")
}

// ResolveEntPackage resolves the ent package path from various input options.
type Resolver struct {
	schemaPath     string
	entPackage     string
	entPackageFile string
}

// NewResolver creates a new package path resolver.
func NewResolver(schemaPath, entPackage, entPackageFile string) *Resolver {
	return &Resolver{
		schemaPath:     schemaPath,
		entPackage:     entPackage,
		entPackageFile: entPackageFile,
	}
}

// Resolve returns the ent package path based on the configured options.
func (r *Resolver) Resolve() (string, error) {
	// If ent_package is explicitly provided, use it
	if r.entPackage != "" {
		return r.entPackage, nil
	}

	// If ent_package_file is provided, detect the package from that file
	if r.entPackageFile != "" {
		return r.resolveFromFile(r.entPackageFile)
	}

	// Auto-detect from schema_path
	return r.resolveFromSchemaPath(r.schemaPath)
}

// resolveFromFile resolves ent package from a file path.
func (r *Resolver) resolveFromFile(absFilePath string) (string, error) {
	absPath, err := filepath.Abs(absFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve ent_package_file path: %w", err)
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

	// Remove the filename to get the package directory
	return path.Join(modulePath, filepath.Dir(relPath)), nil
}

// resolveFromSchemaPath resolves ent package from schema path.
func (r *Resolver) resolveFromSchemaPath(absSchemaPath string) (string, error) {
	absPath, err := filepath.Abs(absSchemaPath)
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
	entPackageDir := filepath.Dir(relPath)

	return path.Join(modulePath, entPackageDir, entDir), nil
}

// ResolveEntPackage is a convenience function that resolves the ent package path.
func ResolveEntPackage(schemaPath, entPackage string) (string, error) {
	r := NewResolver(schemaPath, entPackage, "")
	return r.Resolve()
}
