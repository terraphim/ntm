package templates

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse parses a template from markdown content with YAML frontmatter.
// Format:
//
//	---
//	name: template_name
//	description: What this template does
//	variables:
//	  - name: file
//	    description: File path to review
//	    required: true
//	---
//	The template body with {{variable}} placeholders.
func Parse(content string) (*Template, error) {
	tmpl := &Template{}

	// Check for frontmatter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			// Parse YAML frontmatter
			if err := yaml.Unmarshal([]byte(parts[1]), tmpl); err != nil {
				return nil, err
			}
			tmpl.Body = strings.TrimSpace(parts[2])
		} else {
			// No valid frontmatter, treat entire content as body
			tmpl.Body = strings.TrimSpace(content)
		}
	} else {
		// No frontmatter, entire content is body
		tmpl.Body = strings.TrimSpace(content)
	}

	return tmpl, nil
}

// Execute substitutes variables in the template body.
func (t *Template) Execute(ctx ExecutionContext) (string, error) {
	// Validate required variables
	if err := t.Validate(ctx); err != nil {
		return "", err
	}

	// Build variable map: defaults < builtins < user vars < special vars
	vars := make(map[string]string)

	// Apply defaults from template definition
	for _, v := range t.Variables {
		if v.Default != "" {
			vars[v.Name] = v.Default
		}
	}

	// Apply builtin variables
	for k, v := range BuiltinVariables() {
		vars[k] = v
	}

	// Apply user-provided variables
	for k, v := range ctx.Variables {
		vars[k] = v
	}

	// Apply special context variables
	if ctx.FileContent != "" {
		vars["file"] = ctx.FileContent
	}
	if ctx.Session != "" {
		vars["session"] = ctx.Session
	}
	if ctx.Clipboard != "" {
		vars["clipboard"] = ctx.Clipboard
	}

	// Perform substitution
	result := t.Body

	// First, expand conditionals {{#var}}...{{/var}}
	result = expandConditionals(result, vars)

	// Then, substitute simple variables {{var}}
	result = substituteVariables(result, vars)

	return result, nil
}

// substituteVariables replaces {{variable}} placeholders with values.
func substituteVariables(body string, vars map[string]string) string {
	// Match {{variable}} but not {{#var}} or {{/var}}
	re := regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

	return re.ReplaceAllStringFunc(body, func(match string) string {
		// Extract variable name
		name := match[2 : len(match)-2]

		// Check if it's a conditional marker (starts with # or /)
		if strings.HasPrefix(name, "#") || strings.HasPrefix(name, "/") {
			return match // Leave conditional markers alone
		}

		if val, ok := vars[name]; ok {
			return val
		}
		return match // Leave unmatched variables as-is
	})
}

// expandConditionals handles {{#variable}}...{{/variable}} blocks.
// If the variable is set and non-empty, the block content is included.
// Otherwise, the entire block is removed.
func expandConditionals(body string, vars map[string]string) string {
	// Find opening tags
	openRe := regexp.MustCompile(`\{\{#([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

	// Process until no more matches (handles nested conditionals)
	for {
		matches := openRe.FindStringSubmatchIndex(body)
		if matches == nil {
			break // No more opening tags
		}

		// Extract variable name
		varName := body[matches[2]:matches[3]]
		openStart := matches[0]
		openEnd := matches[1]

		// Find matching closing tag
		closeTag := "{{/" + varName + "}}"
		closeStart := strings.Index(body[openEnd:], closeTag)
		if closeStart == -1 {
			// No matching close tag, leave as-is and skip
			break
		}
		closeStart += openEnd
		closeEnd := closeStart + len(closeTag)

		// Extract content between tags
		content := body[openEnd:closeStart]

		// Determine replacement
		var replacement string
		if val, ok := vars[varName]; ok && val != "" {
			replacement = content
		}
		// else: replacement is empty string, removing the block

		// Rebuild body
		body = body[:openStart] + replacement + body[closeEnd:]
	}

	return body
}

// ExtractVariables finds all variable references in a template body.
// Returns both simple variables ({{var}}) and conditional variables ({{#var}}).
func ExtractVariables(body string) []string {
	seen := make(map[string]bool)
	var vars []string

	// Match simple variables
	simpleRe := regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)
	for _, match := range simpleRe.FindAllStringSubmatch(body, -1) {
		name := match[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}

	// Match conditional variables
	condRe := regexp.MustCompile(`\{\{#([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)
	for _, match := range condRe.FindAllStringSubmatch(body, -1) {
		name := match[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}

	return vars
}
