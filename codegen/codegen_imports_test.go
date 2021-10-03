// Package codegen contains Go code generation facilities.
package codegen

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	cmp_3 "github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
)

func TestGoToolsAssumedPackageName(t *testing.T) {
	tests := []struct {
		importPath string
		want       string
	}{
		{"x/y/z", "z"},
		{"x/y/z/v1", "z"},
		{"x/y/z/v8", "z"},
		{"x/y/go-z", "z"},
		{"x/y/happyface😊suffix", "happyface"},
	}
	for _, tt := range tests {
		t.Run(tt.importPath, func(t *testing.T) {
			if got := GoToolsAssumedPackageName(tt.importPath); got != tt.want {
				t.Errorf("GoToolsAssumedPackageName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	type registerExample struct {
		importPath string
		want       comparableImport
	}

	tests := []struct {
		name             string
		imports          *Imports
		registerExamples []registerExample
	}{
		{
			name: "simple",
			imports: buildImports(func(i *Imports) {
				i.Register("xyz")
			}),
			registerExamples: []registerExample{
				{
					importPath: "xyz",
					want: comparableImport{
						Path:        "xyz",
						PackageName: "xyz",
						Alias:       "",
					},
				},
			},
		},
		{
			name: "conflict",
			imports: buildImports(func(i *Imports) {
				i.Register("a/xyz")
				i.Register("b/xyz")
				i.Register("c/xyz")
			}),
			registerExamples: []registerExample{
				{
					importPath: "a/xyz",
					want: comparableImport{
						Path:        "a/xyz",
						PackageName: "xyz",
						Alias:       "",
					},
				},
				{
					importPath: "b/xyz",
					want: comparableImport{
						Path:        "b/xyz",
						PackageName: "xyz2",
						Alias:       "xyz2",
					},
				},
				{
					importPath: "c/xyz",
					want: comparableImport{
						Path:        "c/xyz",
						PackageName: "xyz3",
						Alias:       "xyz3",
					},
				},
			},
		},
		{
			name: "conflict",
			imports: buildImports(func(i *Imports) {
				i.Register("a/xyz")
				i.Register("b/xyz")
				i.Register("c/xyz")
			}),
			registerExamples: []registerExample{
				{
					importPath: "a/xyz",
					want: comparableImport{
						Path:        "a/xyz",
						PackageName: "xyz",
						Alias:       "",
					},
				},
				{
					importPath: "b/xyz",
					want: comparableImport{
						Path:        "b/xyz",
						PackageName: "xyz2",
						Alias:       "xyz2",
					},
				},
				{
					importPath: "c/xyz",
					want: comparableImport{
						Path:        "c/xyz",
						PackageName: "xyz3",
						Alias:       "xyz3",
					},
				},
			},
		},
		{
			name: "proto naming",
			imports: buildImports(func(i *Imports) {
				i.Register("a/b/c/my_friendly_go_proto")
			}),
			registerExamples: []registerExample{
				{
					importPath: "a/b/c/my_friendly_go_proto",
					want: comparableImport{
						Path:        "a/b/c/my_friendly_go_proto",
						PackageName: "my_friendlypb",
						Alias:       "my_friendlypb",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ttt := range tt.registerExamples {
				t.Run(ttt.importPath, func(t *testing.T) {
					got := tt.imports.Register(ttt.importPath)
					if diff := cmp_3.Diff(ttt.want, makeComparableImport(got), cmpOpts...); diff != "" {
						t.Errorf("unexpected diff (-want, +got):\n%s", diff)
					}
				})
			}
		})
	}
}

func TestRegisterAliases(t *testing.T) {
	type subTest struct {
		importPath, alias string
		want              comparableImport
		wantErr           bool
	}

	tests := []struct {
		name     string
		imports  *Imports
		subTests []subTest
	}{
		{
			name: "conflict",
			imports: buildImports(func(i *Imports) {
				i.Register("xyz")
			}),
			subTests: []subTest{
				{
					importPath: "b/xyz",
					alias:      "xyz",
					wantErr:    true,
				},
			},
		},
		{
			name:    "simple",
			imports: NewImports(),
			subTests: []subTest{
				{
					importPath: "b/xyz",
					alias:      "myalias",
					want: comparableImport{
						Path:        "b/xyz",
						PackageName: "myalias",
						Alias:       "myalias",
					},
				},
			},
		},
		{
			name: "multiple with same import path",
			imports: buildImports(func(i *Imports) {
				i.Register("b/xyz")
			}),
			subTests: []subTest{
				{
					importPath: "b/xyz",
					alias:      "xyz2",
					want: comparableImport{
						Path:        "b/xyz",
						PackageName: "xyz2",
						Alias:       "xyz2",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ttt := range tt.subTests {
				t.Run(ttt.importPath, func(t *testing.T) {
					got, err := tt.imports.RegisterAliased(ttt.importPath, ttt.alias)
					if gotErr := err != nil; gotErr != ttt.wantErr {
						t.Fatalf("got error %v, wantErr = %v", err, ttt.wantErr)
					}
					if err != nil {
						return
					}
					if diff := cmp_3.Diff(ttt.want, makeComparableImport(got), cmpOpts...); diff != "" {
						t.Errorf("unexpected diff (-want, +got):\n%s", diff)
					}
				})
			}
		})
	}
}

func TestImportsConcurrent(t *testing.T) {
	eg := &errgroup.Group{}
	im := NewImports()

	for i := 0; i < 50; i++ {
		i := i
		eg.Go(func() error {
			for j := 0; j < 300; j++ {
				for _, name := range []string{
					fmt.Sprintf("abc/v%d", j),
					fmt.Sprintf("my/lib%d/xyz", j),
					fmt.Sprintf("foobar"),
				} {
					if got, want := im.Register(name).Path(), name; got != want {
						t.Fatalf("i=%d, j=%d Register(%q) returned path %q, want %q", i, j, want, got, want)
					}
				}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("error running workers: %w", err)
	}

}

func buildImports(f func(i *Imports)) *Imports {
	imports := NewImports()
	f(imports)
	return imports
}

type comparableImport struct {
	Path        string
	PackageName string
	Alias       string
}

func makeComparableImport(i *Import) comparableImport {
	return comparableImport{
		Path:        i.Path(),
		Alias:       i.Alias(),
		PackageName: i.PackageName(),
	}
}

var cmpOpts = []cmp.Option{
	cmp.Transformer("Import", makeComparableImport),
}
