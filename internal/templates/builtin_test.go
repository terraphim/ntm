package templates

import (
	"testing"
)

func TestGetBuiltin(t *testing.T) {
	tests := []struct {
		name      string
		want      bool
		wantName  string
	}{
		{name: "code_review", want: true, wantName: "code_review"},
		{name: "explain", want: true, wantName: "explain"},
		{name: "refactor", want: true, wantName: "refactor"},
		{name: "test", want: true, wantName: "test"},
		{name: "document", want: true, wantName: "document"},
		{name: "fix", want: true, wantName: "fix"},
		{name: "implement", want: true, wantName: "implement"},
		{name: "optimize", want: true, wantName: "optimize"},
		{name: "nonexistent", want: false, wantName: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := GetBuiltin(tt.name)
			if tt.want {
				if tmpl == nil {
					t.Errorf("GetBuiltin(%q) = nil, want template", tt.name)
				} else if tmpl.Name != tt.wantName {
					t.Errorf("GetBuiltin(%q).Name = %q, want %q", tt.name, tmpl.Name, tt.wantName)
				}
			} else {
				if tmpl != nil {
					t.Errorf("GetBuiltin(%q) = %v, want nil", tt.name, tmpl)
				}
			}
		})
	}
}

func TestListBuiltins(t *testing.T) {
	builtins := ListBuiltins()

	if len(builtins) < 5 {
		t.Errorf("len(ListBuiltins()) = %d, want at least 5", len(builtins))
	}

	// Check all have required fields
	for _, tmpl := range builtins {
		if tmpl.Name == "" {
			t.Error("Builtin template has empty Name")
		}
		if tmpl.Body == "" {
			t.Errorf("Builtin template %q has empty Body", tmpl.Name)
		}
		if tmpl.Source != SourceBuiltin {
			t.Errorf("Builtin template %q has Source = %v, want SourceBuiltin", tmpl.Name, tmpl.Source)
		}
	}
}

func TestBuiltinTemplates_Executable(t *testing.T) {
	// Test that all builtin templates can execute with their required variables
	builtins := ListBuiltins()

	for _, tmpl := range builtins {
		t.Run(tmpl.Name, func(t *testing.T) {
			// Build context with all required variables
			ctx := ExecutionContext{
				Variables: make(map[string]string),
			}

			for _, v := range tmpl.Variables {
				if v.Required {
					// Set all required variables, including "file"
					ctx.Variables[v.Name] = "// Sample test content for " + v.Name
				}
			}

			// Also set FileContent for templates that use {{file}}
			ctx.FileContent = "// Sample code content\nfunc main() {}"

			result, err := tmpl.Execute(ctx)
			if err != nil {
				t.Errorf("Execute() failed: %v", err)
			}

			if result == "" {
				t.Error("Execute() returned empty result")
			}
		})
	}
}

func TestGetBuiltin_ReturnsCopy(t *testing.T) {
	// Verify GetBuiltin returns a copy, not the original
	tmpl1 := GetBuiltin("code_review")
	tmpl2 := GetBuiltin("code_review")

	tmpl1.Name = "modified"

	if tmpl2.Name == "modified" {
		t.Error("GetBuiltin should return a copy, not the original")
	}
}
