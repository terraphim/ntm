package templates

// builtinTemplates holds the default templates embedded in the binary.
var builtinTemplates = []*Template{
	{
		Name:        "code_review",
		Description: "Review code for quality issues",
		Variables: []VariableSpec{
			{Name: "file", Description: "File content to review", Required: true},
			{Name: "focus", Description: "Specific area to focus on"},
		},
		Tags:   []string{"review", "quality"},
		Source: SourceBuiltin,
		Body: `Review the following code for:
- Code quality issues
- Potential bugs
- Performance concerns
- Security vulnerabilities
- Readability and maintainability

{{#focus}}
Focus especially on: {{focus}}
{{/focus}}

---

{{file}}`,
	},
	{
		Name:        "explain",
		Description: "Explain how code works",
		Variables: []VariableSpec{
			{Name: "file", Description: "File content to explain", Required: true},
		},
		Tags:   []string{"explain", "understand"},
		Source: SourceBuiltin,
		Body: `Explain how the following code works in detail.

Walk through:
1. The overall purpose and design
2. The control flow and data transformations
3. Key functions and their responsibilities
4. Any non-obvious patterns or techniques used

---

{{file}}`,
	},
	{
		Name:        "refactor",
		Description: "Refactor code for better quality",
		Variables: []VariableSpec{
			{Name: "file", Description: "File content to refactor", Required: true},
			{Name: "goal", Description: "Refactoring goal (e.g., 'simplify', 'extract functions')"},
		},
		Tags:   []string{"refactor", "improve"},
		Source: SourceBuiltin,
		Body: `Refactor the following code to improve:
- Code structure and organization
- Readability and naming
- Removal of duplication
- Simplification of complex logic

{{#goal}}
Primary goal: {{goal}}
{{/goal}}

Preserve the existing functionality while making these improvements.

---

{{file}}`,
	},
	{
		Name:        "test",
		Description: "Write tests for code",
		Variables: []VariableSpec{
			{Name: "file", Description: "File content to test", Required: true},
			{Name: "framework", Description: "Test framework to use (e.g., 'jest', 'pytest', 'go test')"},
		},
		Tags:   []string{"test", "quality"},
		Source: SourceBuiltin,
		Body: `Write comprehensive tests for the following code.

{{#framework}}
Use the {{framework}} testing framework.
{{/framework}}

Include:
- Unit tests for individual functions
- Edge case handling
- Error condition tests
- Integration tests where appropriate

---

{{file}}`,
	},
	{
		Name:        "document",
		Description: "Add documentation to code",
		Variables: []VariableSpec{
			{Name: "file", Description: "File content to document", Required: true},
			{Name: "style", Description: "Documentation style (e.g., 'jsdoc', 'godoc', 'docstring')"},
		},
		Tags:   []string{"docs", "documentation"},
		Source: SourceBuiltin,
		Body: `Add comprehensive documentation to the following code.

{{#style}}
Use {{style}} style documentation.
{{/style}}

Include:
- File/module level documentation
- Function/method docstrings
- Parameter and return value descriptions
- Usage examples where helpful
- Any important notes or warnings

---

{{file}}`,
	},
	{
		Name:        "fix",
		Description: "Fix a specific issue in code",
		Variables: []VariableSpec{
			{Name: "file", Description: "File content with the issue", Required: true},
			{Name: "issue", Description: "Description of the issue to fix", Required: true},
		},
		Tags:   []string{"fix", "bug"},
		Source: SourceBuiltin,
		Body: `Fix the following issue in the code:

**Issue:** {{issue}}

Provide:
1. Root cause analysis
2. The fix with explanation
3. Any related changes needed

---

{{file}}`,
	},
	{
		Name:        "implement",
		Description: "Implement a feature or function",
		Variables: []VariableSpec{
			{Name: "description", Description: "What to implement", Required: true},
			{Name: "file", Description: "Existing code context"},
			{Name: "language", Description: "Programming language to use"},
		},
		Tags:   []string{"implement", "feature"},
		Source: SourceBuiltin,
		Body: `Implement the following:

**Description:** {{description}}

{{#language}}
Use {{language}}.
{{/language}}

{{#file}}
Here is the existing code context:

---

{{file}}
{{/file}}`,
	},
	{
		Name:        "optimize",
		Description: "Optimize code for performance",
		Variables: []VariableSpec{
			{Name: "file", Description: "File content to optimize", Required: true},
			{Name: "metric", Description: "Metric to optimize (e.g., 'time', 'memory', 'both')"},
		},
		Tags:   []string{"optimize", "performance"},
		Source: SourceBuiltin,
		Body: `Optimize the following code for better performance.

{{#metric}}
Focus on: {{metric}}
{{/metric}}

Consider:
- Algorithm efficiency (time complexity)
- Memory usage
- I/O operations
- Caching opportunities
- Parallelization potential

Explain the optimizations and their expected impact.

---

{{file}}`,
	},
}

// GetBuiltin returns a builtin template by name, or nil if not found.
func GetBuiltin(name string) *Template {
	for _, t := range builtinTemplates {
		if t.Name == name {
			// Return a copy to prevent modification
			copy := *t
			return &copy
		}
	}
	return nil
}

// ListBuiltins returns all builtin templates.
func ListBuiltins() []*Template {
	// Return copies to prevent modification
	result := make([]*Template, len(builtinTemplates))
	for i, t := range builtinTemplates {
		copy := *t
		result[i] = &copy
	}
	return result
}
