// Package codegen contains Go code generation facilities.
package codegen

import "testing"

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
