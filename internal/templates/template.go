// Package templates provides prompt template loading, parsing, and variable substitution.
package templates

import (
	"fmt"
	"os"
	"time"
)

// Template represents a loaded prompt template with metadata.
type Template struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Variables   []VariableSpec    `yaml:"variables"`
	Tags        []string          `yaml:"tags,omitempty"`
	Body        string            `yaml:"-"` // The template body (not in frontmatter)
	Source      TemplateSource    `yaml:"-"` // Where this template came from
	SourcePath  string            `yaml:"-"` // File path if from file
}

// VariableSpec describes a template variable.
type VariableSpec struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
	Default     string `yaml:"default,omitempty"`
}

// TemplateSource indicates where a template was loaded from.
type TemplateSource int

const (
	SourceBuiltin TemplateSource = iota
	SourceUser
	SourceProject
)

func (s TemplateSource) String() string {
	switch s {
	case SourceBuiltin:
		return "builtin"
	case SourceUser:
		return "user"
	case SourceProject:
		return "project"
	default:
		return "unknown"
	}
}

// BuiltinVariables returns the built-in variables available in all templates.
func BuiltinVariables() map[string]string {
	now := time.Now()
	cwd, _ := os.Getwd()

	return map[string]string{
		"cwd":  cwd,
		"date": now.Format("2006-01-02"),
		"time": now.Format("15:04:05"),
	}
}

// ExecutionContext holds variables for template execution.
type ExecutionContext struct {
	// User-provided variables via --var flags
	Variables map[string]string

	// File content injection via --file flag
	FileContent string

	// Session name for {{session}} variable
	Session string

	// Clipboard content for {{clipboard}} variable
	Clipboard string
}

// Validate checks that all required variables are provided.
func (t *Template) Validate(ctx ExecutionContext) error {
	for _, v := range t.Variables {
		if !v.Required {
			continue
		}

		// Check if variable is provided via Variables map
		if _, ok := ctx.Variables[v.Name]; ok {
			continue
		}

		// Check if "file" variable is provided via FileContent
		if v.Name == "file" && ctx.FileContent != "" {
			continue
		}

		// Check if "session" variable is provided via Session
		if v.Name == "session" && ctx.Session != "" {
			continue
		}

		// Check if variable has a default value
		if v.Default != "" {
			continue
		}

		return fmt.Errorf("missing required variable: %s", v.Name)
	}
	return nil
}

// HasVariable checks if a variable is defined in the template.
func (t *Template) HasVariable(name string) bool {
	for _, v := range t.Variables {
		if v.Name == name {
			return true
		}
	}
	return false
}
