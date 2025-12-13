package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	sessionDirName = "sessions"
	fileExtension  = ".json"
)

// StorageDir returns the path to the session storage directory.
// Uses XDG_DATA_HOME if set, otherwise ~/.local/share/ntm/sessions/
func StorageDir() string {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return sessionDirName // Fallback to current dir
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "ntm", sessionDirName)
}

// Save writes a session state to disk.
func Save(state *SessionState, opts SaveOptions) (string, error) {
	unlock, err := acquireLock()
	if err != nil {
		return "", err
	}
	defer unlock()

	dir := StorageDir()

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create sessions directory: %w", err)
	}

	// Determine filename
	name := opts.Name
	if name == "" {
		name = state.Name
	}

	// Sanitize filename
	name = sanitizeFilename(name)
	filename := name + fileExtension
	path := filepath.Join(dir, filename)

	// Check if file exists
	if !opts.Overwrite {
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("session '%s' already saved (use --overwrite to replace)", name)
		}
	}

	// Marshal state
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize session state: %w", err)
	}

	// Atomic write: write to temp file then rename
	tmpFile, err := os.CreateTemp(dir, "session-*.tmp")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup on error
	defer func() {
		_ = tmpFile.Close()
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = tmpFile.Write(data); err != nil {
		return "", fmt.Errorf("writing to temp file: %w", err)
	}
	if err = tmpFile.Sync(); err != nil {
		return "", fmt.Errorf("syncing temp file: %w", err)
	}
	if err = tmpFile.Close(); err != nil {
		return "", fmt.Errorf("closing temp file: %w", err)
	}

	if err = os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("renaming session file: %w", err)
	}

	return path, nil
}

// Load reads a session state from disk.
func Load(name string) (*SessionState, error) {
	name = sanitizeFilename(name)
	path := filepath.Join(StorageDir(), name+fileExtension)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no saved session named '%s'", name)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &state, nil
}

// Delete removes a saved session.
func Delete(name string) error {
	unlock, err := acquireLock()
	if err != nil {
		return err
	}
	defer unlock()

	name = sanitizeFilename(name)
	path := filepath.Join(StorageDir(), name+fileExtension)

	err = os.Remove(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("no saved session named '%s'", name)
	}
	return err
}

// SavedSession represents a saved session entry.
type SavedSession struct {
	Name      string    `json:"name"`
	SavedAt   time.Time `json:"saved_at"`
	WorkDir   string    `json:"cwd"`
	Agents    int       `json:"agents"`
	GitBranch string    `json:"git_branch,omitempty"`
	FilePath  string    `json:"file_path"`
	FileSize  int64     `json:"file_size"`
}

// List returns all saved sessions.
func List() ([]SavedSession, error) {
	unlock, err := acquireLock()
	if err != nil {
		return nil, err
	}
	defer unlock()

	dir := StorageDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SavedSession{}, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []SavedSession
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), fileExtension) {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), fileExtension)
		path := filepath.Join(dir, entry.Name())

		// Load minimal info
		state, err := Load(name)
		if err != nil {
			continue // Skip corrupted files
		}

		info, _ := entry.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}

		sessions = append(sessions, SavedSession{
			Name:      state.Name,
			SavedAt:   state.SavedAt,
			WorkDir:   state.WorkDir,
			Agents:    state.Agents.Total(),
			GitBranch: state.GitBranch,
			FilePath:  path,
			FileSize:  size,
		})
	}

	// Sort by save time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].SavedAt.After(sessions[j].SavedAt)
	})

	return sessions, nil
}

// Exists checks if a saved session exists.
func Exists(name string) bool {
	name = sanitizeFilename(name)
	path := filepath.Join(StorageDir(), name+fileExtension)
	_, err := os.Stat(path)
	return err == nil
}

// sanitizeFilename removes or replaces characters not suitable for filenames.
func sanitizeFilename(name string) string {
	// Replace problematic characters
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
}
