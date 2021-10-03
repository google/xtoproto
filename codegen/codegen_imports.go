// Package codegen contains Go code generation facilities.
package codegen

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const blankAlias = "_"

// Imports maintains a set of imports. The imports are safe for concurrent
// access.
type Imports struct {
	lock sync.Mutex
	defs []*importDef

	// Customization of import path -> package name.
	customAssumedPackageNameFunc func(importPath string) string
	customSuggestAliases         func(importPath string, callack func(alias string) (keepGoing bool))
}

type importDef struct {
	path        string
	packageName string
	isAlias     bool // path may differ
}

func (d *importDef) asImport() *Import {
	if d == nil {
		return nil
	}
	return &Import{
		path:        d.path,
		packageName: d.packageName,
		isAlias:     d.isAlias,
	}
}

// ImportsOption configures the behavior of Imports.
type ImportsOption struct {
	apply func(im *Imports)
}

// UseAssumedPackageNameFunc returns an Option to configure Imports to use a
// different function to infer a package name from an import path.
func UseAssumedPackageNameFunc(f func(importPath string) string) ImportsOption {
	return ImportsOption{apply: func(im *Imports) {
		im.customAssumedPackageNameFunc = f
	}}
}

// NewImports returns an initialized Imports value.
func NewImports(options ...ImportsOption) *Imports {
	i := &Imports{}
	for _, opt := range options {
		opt.apply(i)
	}
	return i
}

// AssumedPackageName returns the assumed package name of an import path.
// It does this using only string parsing of the import path.
//
// By default, this will call GoToolsAssumedPackageName(importPath). If
func (i *Imports) AssumedPackageName(importPath string) string {
	if i.customAssumedPackageNameFunc == nil {
		return GoToolsAssumedPackageName(importPath)
	}
	return i.customAssumedPackageNameFunc(importPath)
}

// suggestAlises should return the sequence of aliases Register should use to
// name its import.
//
// blank may be used to suggest the importPath be imported without an alias.
// This is typically the first suggestion.
func (i *Imports) suggestAliases(importPath string, callback func(alias string) (keepGoing bool)) {
	if i.customSuggestAliases != nil {
		i.customSuggestAliases(importPath, callback)
		return
	}
	assumedPackageName := i.AssumedPackageName(importPath)
	asAlias := func(packageName string) string {
		if packageName == assumedPackageName {
			return ""
		}
		return packageName
	}

	preferredPackageName := func(noise string) string {
		return assumedPackageName + noise
	}

	if withoutProtoSuffix := strings.TrimSuffix(assumedPackageName, "_go_proto"); withoutProtoSuffix != assumedPackageName {
		preferredPackageName = func(noise string) string {
			return withoutProtoSuffix + noise + "pb"
		}
	}

	for i := 1; ; i++ {
		noise := ""
		if i > 1 {
			noise = strconv.Itoa(i)
		}
		if !callback(asAlias(preferredPackageName(noise))) {
			break
		}
	}
}

// Register registers an import path so that it will be included in the output.
func (i *Imports) Register(importPath string) *Import {
	// First check to see if we already have an entry.
	if got := func() *Import {
		locked, unlock := i.lockedVersion()
		defer unlock()

		return locked.findByImportPath(importPath).asImport()
	}(); got != nil {
		return got
	}

	const iterationLimit = 5000
	count := 0
	// Import it:
	var result *Import
	var lastErr error
	i.suggestAliases(importPath, func(alias string) (keepGoing bool) {
		if count > iterationLimit {
			panic(fmt.Errorf("bug with *Imports: too many suggested aliases; last error = %w", lastErr))
		}
		count++
		if alias == "" {
			result, lastErr = i.registerUnaliased(importPath)
			return lastErr != nil
		}
		result, lastErr = i.RegisterAliased(importPath, alias)
		return lastErr != nil
	})
	return result
}

// Register registers an import path so that it will be included in the output.
//
// If there is already an import for the given alias, an error will be returned
// if it has a different import path.
func (i *Imports) RegisterAliased(importPath, alias string) (*Import, error) {
	if alias == "" {
		return nil, fmt.Errorf("alias must not be empty")
	}
	locked, unlock := i.lockedVersion()
	defer unlock()

	def := locked.findByPackageName(alias)
	if def != nil && def.path == importPath {
		return def.asImport(), nil
	}
	if def != nil && def.asImport().Alias() != blankAlias { // allow multiple _ aliases
		return nil, fmt.Errorf("alias %q already maps to import path %q, not %q as the caller requires", alias, def.path, importPath)
	}
	def = &importDef{
		path:        importPath,
		packageName: alias,
		isAlias:     true,
	}
	i.defs = append(i.defs, def)
	return def.asImport(), nil
}

func (i *Imports) registerUnaliased(importPath string) (*Import, error) {
	assumedPackageName := i.AssumedPackageName(importPath)

	locked, unlock := i.lockedVersion()
	defer unlock()

	def := locked.findByPackageName(assumedPackageName)
	if def != nil && def.path == importPath {
		if def.isAlias {
			return nil, fmt.Errorf("%q already exists as an import aliased to %q, cannot register as unaliased", def.path, def.packageName)
		}
		return def.asImport(), nil
	}
	if def != nil {
		return nil, fmt.Errorf("alias %q already maps to import path %q, not %q as the caller requires", assumedPackageName, def.path, importPath)
	}
	def = &importDef{
		path:        importPath,
		packageName: assumedPackageName,
		isAlias:     false,
	}
	i.defs = append(i.defs, def)
	return def.asImport(), nil
}

// FindByPackageName returns the definition by package name, the name used in
// the source code identifiers.
func (i *Imports) FindByImportPath(importPath string) *Import {
	l, unlock := i.lockedVersion()
	defer unlock()
	return l.findByImportPath(importPath).asImport()
}

// FindByPackageName returns the definition by package name, the name used in
// the source code identifiers.
func (i *Imports) FindByPackageName(packageName string) *Import {
	l, unlock := i.lockedVersion()
	defer unlock()
	return l.findByPackageName(packageName).asImport()
}

func (i *Imports) lockedVersion() (locked lockedImports, unlock func()) {
	i.lock.Lock()
	return lockedImports{i}, i.lock.Unlock
}

type lockedImports struct {
	i *Imports
}

func (i lockedImports) findByImportPath(importPath string) *importDef {
	for _, def := range i.i.defs {
		if def.path == importPath {
			return def
		}
	}
	return nil
}

// findByPackageName returns the definition by package name, the name used in
// the source code identifiers.
func (i lockedImports) findByPackageName(packageName string) *importDef {
	for _, def := range i.i.defs {
		if def.packageName == packageName {
			return def
		}
	}
	return nil
}

// Import corresponds to a single import in a Go file.
type Import struct {
	path        string
	packageName string
	isAlias     bool // path may differ
}

// Path returns the quoted part of the import statement.
//
// For an import like `import xyz "x/y/z"`, Path() returns "x/y/z".
func (s *Import) Path() string { return s.path }

// Alias returns the explicit package name of the import or ""
//
// For an import like `import xyz "x/y/z"`, Alias() returns "xyz".
//
// For an import like `import "x/y/z"`, Alias() returns "".
func (s *Import) Alias() string {
	if !s.isAlias {
		return ""
	}
	return s.packageName
}

// PackageName returns the prefix that can be used by identifiers to refer to
// symbols from the package.
//
// For an import like `import xyz "x/y/z"`, PackageName() returns "xyz".
//
// For an import like `import "x/y/z"`, PackageName() would probably return "z".
// The package name is set by the Imports object that created the Import.
// See the UseAssumedPackageNameFunc function for more of an explanation.
//
// If Alias() returns non-empty, PackageName() will be equal to Alias().
func (s *Import) PackageName() string {
	return s.packageName
}

// GoFragment returns the Go fragment for the import statement.
//
// The return value will be something like `a "x/y/z"` for aliased imports and
// `"xyz"` for non-aliased imports.
func (s *Import) GoFragment() string {
	if s.isAlias {
		return fmt.Sprintf("%s %q", s.packageName, s.path)
	}
	return fmt.Sprintf("%q", s.path)
}

// GoToolsAssumedPackageName returns the assumed package name of an import path.
// It does this using only string parsing of the import path.
//
// It picks the last element of the path that does not look like a major
// version, and then picks the valid identifier off the start of that element.
// It is used to determine if a local rename should be added to an import for
// clarity.
//
// This function is copied from
// https://pkg.go.dev/golang.org/x/tools/internal/imports#ImportPathToAssumedName.
func GoToolsAssumedPackageName(importPath string) string {
	base := path.Base(importPath)
	if strings.HasPrefix(base, "v") {
		if _, err := strconv.Atoi(base[1:]); err == nil {
			dir := path.Dir(importPath)
			if dir != "." {
				base = path.Base(dir)
			}
		}
	}
	base = strings.TrimPrefix(base, "go-")
	if i := strings.IndexFunc(base, notIdentifier); i >= 0 {
		base = base[:i]
	}
	return base
}

// notIdentifier reports whether ch is an invalid identifier character.
func notIdentifier(ch rune) bool {
	return !('a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' ||
		'0' <= ch && ch <= '9' ||
		ch == '_' ||
		ch >= utf8.RuneSelf && (unicode.IsLetter(ch) || unicode.IsDigit(ch)))
}
