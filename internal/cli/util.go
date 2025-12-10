package cli

import (
	"strings"
)

// parseEditorCommand splits the editor string into command and arguments.
// It handles simple spaces.
func parseEditorCommand(editor string) (string, []string) {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}
