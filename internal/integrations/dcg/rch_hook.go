package dcg

import (
	"fmt"
	"strings"
)

const defaultRCHHookTimeout = 5000

// RCHHookOptions configures how the RCH hook is generated.
type RCHHookOptions struct {
	// BinaryPath is the path to the rch binary. If empty, "rch" is used (PATH lookup).
	BinaryPath string

	// Patterns are regex patterns used to match build commands for interception.
	Patterns []string

	// Timeout is the hook timeout in milliseconds. Default is 5000ms.
	Timeout int
}

// DefaultRCHHookOptions returns sensible defaults for RCH hook configuration.
func DefaultRCHHookOptions() RCHHookOptions {
	return RCHHookOptions{
		Timeout: defaultRCHHookTimeout,
	}
}

// ShouldConfigureRCHHooks determines if RCH hooks should be configured.
func ShouldConfigureRCHHooks(rchEnabled bool, patterns []string) bool {
	if !rchEnabled {
		return false
	}
	return len(cleanRCHPatterns(patterns)) > 0
}

// GenerateRCHHookEntry creates a Claude Code hook entry for RCH interception.
func GenerateRCHHookEntry(opts RCHHookOptions) (HookEntry, error) {
	patterns := cleanRCHPatterns(opts.Patterns)
	if len(patterns) == 0 {
		return HookEntry{}, fmt.Errorf("rch hook requires at least one intercept pattern")
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultRCHHookTimeout
	}

	rchBinary := strings.TrimSpace(opts.BinaryPath)
	if rchBinary == "" {
		rchBinary = "rch"
	}

	pattern := joinRCHPatterns(patterns)
	script := buildRCHHookScript(rchBinary, pattern)

	return HookEntry{
		Matcher: "Bash",
		Command: "sh -c " + shellQuote(script),
		Timeout: timeout,
	}, nil
}

func cleanRCHPatterns(patterns []string) []string {
	if len(patterns) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(patterns))
	cleaned := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if _, exists := seen[pattern]; exists {
			continue
		}
		seen[pattern] = struct{}{}
		cleaned = append(cleaned, pattern)
	}
	return cleaned
}

func joinRCHPatterns(patterns []string) string {
	if len(patterns) == 1 {
		return patterns[0]
	}
	parts := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		parts = append(parts, "("+pattern+")")
	}
	return strings.Join(parts, "|")
}

func buildRCHHookScript(rchBinary, pattern string) string {
	const denyJSON = `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"RCH intercepted build; local execution skipped."}}`

	var b strings.Builder
	b.WriteString(`command="${CLAUDE_TOOL_INPUT_command:-}"` + "\n")
	b.WriteString(`if [ -z "$command" ]; then exit 0; fi` + "\n")
	b.WriteString(`rch_bin=` + shellQuote(rchBinary) + "\n")
	b.WriteString(`case "$rch_bin" in` + "\n")
	b.WriteString(`  */*) if [ ! -x "$rch_bin" ]; then exit 0; fi ;;` + "\n")
	b.WriteString(`  *) if ! command -v "$rch_bin" >/dev/null 2>&1; then exit 0; fi ;;` + "\n")
	b.WriteString(`esac` + "\n")
	b.WriteString(`if ! printf "%s" "$command" | grep -Eq ` + shellQuote(pattern) + `; then exit 0; fi` + "\n")
	b.WriteString(`"$rch_bin" intercept -- $command 1>&2` + "\n")
	b.WriteString(`status=$?` + "\n")
	b.WriteString(`if [ $status -eq 0 ]; then` + "\n")
	b.WriteString(`  echo ` + shellQuote(denyJSON) + "\n")
	b.WriteString(`fi` + "\n")
	b.WriteString(`exit 0` + "\n")
	return b.String()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
