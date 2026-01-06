// Package robot provides machine-readable output for AI agents.
// cass_inject.go provides CASS (Cross-Agent Search) query functionality
// for injecting relevant historical context into agent prompts.
package robot

import (
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// CASSConfig holds configuration for CASS queries.
type CASSConfig struct {
	// Enabled controls whether CASS queries are performed.
	Enabled bool `json:"enabled"`

	// MaxResults limits the number of CASS hits to return.
	MaxResults int `json:"max_results"`

	// MaxAgeDays filters results to those within this many days.
	MaxAgeDays int `json:"max_age_days"`

	// MinRelevance is the minimum relevance score (0.0-1.0) to include results.
	// Note: CASS doesn't currently return relevance scores, so this is for future use.
	MinRelevance float64 `json:"min_relevance"`

	// PreferSameProject gives preference to results from the current workspace.
	PreferSameProject bool `json:"prefer_same_project"`

	// AgentFilter limits results to specific agent types (e.g., "claude", "codex").
	// Empty means all agents.
	AgentFilter []string `json:"agent_filter,omitempty"`
}

// DefaultCASSConfig returns sensible defaults for CASS queries.
func DefaultCASSConfig() CASSConfig {
	return CASSConfig{
		Enabled:           true,
		MaxResults:        5,
		MaxAgeDays:        30,
		MinRelevance:      0.0,
		PreferSameProject: true,
		AgentFilter:       nil,
	}
}

// CASSHit represents a single search result from CASS.
type CASSHit struct {
	// SourcePath is the path to the session file.
	SourcePath string `json:"source_path"`

	// LineNumber is the line in the session file.
	LineNumber int `json:"line_number"`

	// Agent is the agent type (e.g., "claude", "codex", "gemini").
	Agent string `json:"agent"`

	// Content is the matched content snippet (if available).
	Content string `json:"content,omitempty"`

	// Score is the relevance score (if available from CASS).
	Score float64 `json:"score,omitempty"`
}

// CASSQueryResult holds the results of a CASS query.
type CASSQueryResult struct {
	// Success indicates whether the query completed successfully.
	Success bool `json:"success"`

	// Query is the search query that was executed.
	Query string `json:"query"`

	// Hits contains the matching results.
	Hits []CASSHit `json:"hits"`

	// TotalMatches is the total number of matches (may be > len(Hits) if limited).
	TotalMatches int `json:"total_matches"`

	// QueryTime is how long the query took.
	QueryTime time.Duration `json:"query_time_ms"`

	// Error contains any error message.
	Error string `json:"error,omitempty"`

	// Keywords are the extracted keywords from the original prompt.
	Keywords []string `json:"keywords,omitempty"`
}

// cassSearchResponse matches the JSON structure returned by `cass search --json`.
type cassSearchResponse struct {
	Query        string `json:"query"`
	TotalMatches int    `json:"total_matches"`
	Hits         []struct {
		SourcePath string `json:"source_path"`
		LineNumber int    `json:"line_number"`
		Agent      string `json:"agent"`
		Content    string `json:"content,omitempty"`
		Score      float64 `json:"score,omitempty"`
	} `json:"hits"`
}

// QueryCASS queries CASS for relevant historical context based on the prompt.
// It extracts keywords from the prompt and searches for relevant past sessions.
func QueryCASS(prompt string, config CASSConfig) CASSQueryResult {
	start := time.Now()
	result := CASSQueryResult{
		Success: false,
		Query:   "",
		Hits:    []CASSHit{},
	}

	if !config.Enabled {
		result.Success = true
		return result
	}

	// Extract keywords from the prompt
	keywords := ExtractKeywords(prompt)
	result.Keywords = keywords

	if len(keywords) == 0 {
		result.Success = true
		result.Error = "no keywords extracted from prompt"
		return result
	}

	// Build the search query
	query := strings.Join(keywords, " ")
	result.Query = query

	// Check if CASS is available
	if !isCASSAvailable() {
		result.Error = "cass command not found"
		return result
	}

	// Build the cass search command
	args := []string{"search", query, "--json"}

	// Add limit
	if config.MaxResults > 0 {
		args = append(args, "--limit", itoa(config.MaxResults))
	}

	// Add age filter
	if config.MaxAgeDays > 0 {
		args = append(args, "--days", itoa(config.MaxAgeDays))
	}

	// Add agent filter
	for _, agent := range config.AgentFilter {
		args = append(args, "--agent", agent)
	}

	// Execute CASS search
	cmd := exec.Command("cass", args...)
	output, err := cmd.Output()
	result.QueryTime = time.Since(start)

	if err != nil {
		// Check if it's just no results vs actual error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Exit code 1 often means no results, which is fine
			result.Success = true
			return result
		}
		result.Error = err.Error()
		return result
	}

	// Parse the response
	var resp cassSearchResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		result.Error = "failed to parse CASS response: " + err.Error()
		return result
	}

	// Convert to our format
	result.TotalMatches = resp.TotalMatches
	for _, hit := range resp.Hits {
		result.Hits = append(result.Hits, CASSHit{
			SourcePath: hit.SourcePath,
			LineNumber: hit.LineNumber,
			Agent:      hit.Agent,
			Content:    hit.Content,
			Score:      hit.Score,
		})
	}

	result.Success = true
	return result
}

// ExtractKeywords extracts meaningful keywords from a prompt for CASS search.
// It filters out common stop words and short words, focusing on technical terms.
func ExtractKeywords(prompt string) []string {
	// Convert to lowercase for processing
	text := strings.ToLower(prompt)

	// Remove code blocks to avoid searching for code syntax
	text = removeCodeBlocks(text)

	// Tokenize: split on non-alphanumeric characters
	words := tokenize(text)

	// Filter words
	var keywords []string
	seen := make(map[string]bool)

	for _, word := range words {
		// Skip short words
		if len(word) < 3 {
			continue
		}

		// Skip stop words
		if isStopWord(word) {
			continue
		}

		// Skip if already seen (deduplicate)
		if seen[word] {
			continue
		}
		seen[word] = true

		keywords = append(keywords, word)
	}

	// Limit to most relevant keywords (first 10)
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}

	return keywords
}

// tokenize splits text into words, handling code identifiers like snake_case.
func tokenize(text string) []string {
	// Split on whitespace and punctuation, but keep underscores in identifiers
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// removeCodeBlocks removes markdown code blocks from text.
func removeCodeBlocks(text string) string {
	// Remove fenced code blocks
	re := regexp.MustCompile("(?s)```.*?```")
	text = re.ReplaceAllString(text, " ")

	// Remove inline code
	re = regexp.MustCompile("`[^`]+`")
	text = re.ReplaceAllString(text, " ")

	return text
}

// isStopWord returns true if the word is a common stop word.
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		// Common English stop words
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"as": true, "is": true, "was": true, "are": true, "were": true,
		"been": true, "be": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true,
		"this": true, "that": true, "these": true, "those": true,
		"it": true, "its": true, "they": true, "them": true, "their": true,
		"we": true, "you": true, "your": true, "our": true, "my": true,
		"me": true, "him": true, "her": true, "his": true, "she": true,
		"he": true, "i": true, "all": true, "each": true, "every": true,
		"both": true, "few": true, "more": true, "most": true, "other": true,
		"some": true, "such": true, "no": true, "nor": true, "not": true,
		"only": true, "own": true, "same": true, "so": true, "than": true,
		"too": true, "very": true, "just": true, "also": true, "now": true,
		"can": true, "get": true, "got": true, "how": true, "what": true,
		"when": true, "where": true, "which": true, "who": true, "why": true,
		"new": true, "use": true, "used": true, "using": true,
		"make": true, "made": true, "like": true, "want": true, "need": true,
		"please": true, "help": true, "here": true, "there": true,

		// Common coding task words (too generic to search)
		"code": true, "file": true, "function": true, "method": true,
		"class": true, "variable": true, "add": true, "create": true,
		"update": true, "delete": true, "remove": true, "change": true,
		"fix": true, "bug": true, "error": true, "test": true, "write": true,
		"read": true, "run": true, "start": true, "stop": true,
	}

	return stopWords[word]
}

// isCASSAvailable checks if the cass command is available.
func isCASSAvailable() bool {
	_, err := exec.LookPath("cass")
	return err == nil
}

// itoa converts int to string (simple helper to avoid strconv import for small use).
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var result []byte
	neg := i < 0
	if neg {
		i = -i
	}

	for i > 0 {
		result = append([]byte{byte('0' + i%10)}, result...)
		i /= 10
	}

	if neg {
		result = append([]byte{'-'}, result...)
	}

	return string(result)
}

// =============================================================================
// Relevance Filtering
// =============================================================================

// FilterConfig holds configuration for relevance filtering.
type FilterConfig struct {
	// MinRelevance is the minimum relevance score (0.0-1.0) to include results.
	// Default: 0.7
	MinRelevance float64 `json:"min_relevance"`

	// MaxItems is the maximum number of items to return after filtering.
	MaxItems int `json:"max_items"`

	// PreferSameProject boosts scores for results from the same workspace.
	PreferSameProject bool `json:"prefer_same_project"`

	// CurrentWorkspace is the current project path for same-project preference.
	CurrentWorkspace string `json:"current_workspace,omitempty"`

	// MaxAgeDays filters out results older than this many days.
	// 0 means no age limit.
	MaxAgeDays int `json:"max_age_days"`

	// RecencyBoost is the weight given to recency (0.0-1.0).
	// Higher values favor newer results more strongly.
	// Default: 0.3
	RecencyBoost float64 `json:"recency_boost"`
}

// DefaultFilterConfig returns sensible defaults for relevance filtering.
func DefaultFilterConfig() FilterConfig {
	return FilterConfig{
		MinRelevance:      0.7,
		MaxItems:          5,
		PreferSameProject: true,
		MaxAgeDays:        30,
		RecencyBoost:      0.3,
	}
}

// ScoredHit wraps a CASSHit with a computed relevance score.
type ScoredHit struct {
	CASSHit
	// ComputedScore is the relevance score computed by filtering (0.0-1.0).
	ComputedScore float64 `json:"computed_score"`
	// ScoreDetail explains how the score was computed.
	ScoreDetail CASSScoreDetail `json:"score_detail"`
}

// CASSScoreDetail shows the components of a CASS relevance score.
type CASSScoreDetail struct {
	// BaseScore is the initial score from search result position.
	BaseScore float64 `json:"base_score"`
	// RecencyBonus is added for newer results.
	RecencyBonus float64 `json:"recency_bonus"`
	// ProjectBonus is added for same-project matches.
	ProjectBonus float64 `json:"project_bonus"`
	// AgePenalty is subtracted for older results.
	AgePenalty float64 `json:"age_penalty"`
}

// FilterResult holds the results of filtering CASS hits.
type FilterResult struct {
	// Hits are the filtered and scored results.
	Hits []ScoredHit `json:"hits"`
	// OriginalCount is how many results were received before filtering.
	OriginalCount int `json:"original_count"`
	// FilteredCount is how many results passed the filter.
	FilteredCount int `json:"filtered_count"`
	// RemovedByScore is how many were removed for low relevance.
	RemovedByScore int `json:"removed_by_score"`
	// RemovedByAge is how many were removed for being too old.
	RemovedByAge int `json:"removed_by_age"`
}

// FilterResults filters and scores CASS hits based on relevance criteria.
// It applies recency preference, same-project preference, and minimum score thresholds.
func FilterResults(hits []CASSHit, config FilterConfig) FilterResult {
	result := FilterResult{
		Hits:          []ScoredHit{},
		OriginalCount: len(hits),
	}

	if len(hits) == 0 {
		return result
	}

	now := time.Now()
	maxAgeTime := now.AddDate(0, 0, -config.MaxAgeDays)

	// Score and filter each hit
	var scored []ScoredHit
	for i, hit := range hits {
		// Extract date from source path to compute age
		sessionDate := extractSessionDate(hit.SourcePath)

		// Apply age filter if configured
		if config.MaxAgeDays > 0 && !sessionDate.IsZero() && sessionDate.Before(maxAgeTime) {
			result.RemovedByAge++
			continue
		}

		// Compute score components
		breakdown := CASSScoreDetail{}

		// Base score from position (earlier = higher, normalized 0.5-1.0)
		// Position 0 gets 1.0, position N-1 gets 0.5
		if len(hits) > 1 {
			breakdown.BaseScore = 1.0 - (float64(i) * 0.5 / float64(len(hits)-1))
		} else {
			breakdown.BaseScore = 1.0
		}

		// If CASS returned a score, use it as base instead
		if hit.Score > 0 {
			breakdown.BaseScore = normalizeScore(hit.Score)
		}

		// Recency bonus (newer = higher)
		if !sessionDate.IsZero() {
			age := now.Sub(sessionDate)
			maxAge := time.Duration(config.MaxAgeDays) * 24 * time.Hour
			if maxAge > 0 && age < maxAge {
				// Recency bonus: 1.0 for today, 0.0 for max age
				recencyFactor := 1.0 - (float64(age) / float64(maxAge))
				breakdown.RecencyBonus = recencyFactor * config.RecencyBoost
			}
		}

		// Same-project bonus
		if config.PreferSameProject && config.CurrentWorkspace != "" {
			if isSameProject(hit.SourcePath, config.CurrentWorkspace) {
				breakdown.ProjectBonus = 0.15 // 15% bonus for same project
			}
		}

		// Compute final score (capped at 1.0)
		finalScore := breakdown.BaseScore + breakdown.RecencyBonus + breakdown.ProjectBonus - breakdown.AgePenalty
		if finalScore > 1.0 {
			finalScore = 1.0
		}
		if finalScore < 0 {
			finalScore = 0
		}

		// Apply minimum relevance threshold
		if finalScore < config.MinRelevance {
			result.RemovedByScore++
			continue
		}

		scored = append(scored, ScoredHit{
			CASSHit:       hit,
			ComputedScore: finalScore,
			ScoreDetail:   breakdown,
		})
	}

	// Sort by computed score (highest first)
	sortScoredHits(scored)

	// Apply max items limit
	if config.MaxItems > 0 && len(scored) > config.MaxItems {
		scored = scored[:config.MaxItems]
	}

	result.Hits = scored
	result.FilteredCount = len(scored)

	return result
}

// extractSessionDate attempts to extract a date from a CASS session file path.
// Session paths typically contain dates like: .../2025/12/05/session-....jsonl
func extractSessionDate(path string) time.Time {
	// Look for date patterns in the path
	// Common formats: /2025/12/05/ or /2025-12-05/ or session-2025-12-05
	datePatterns := []string{
		`/(\d{4})/(\d{2})/(\d{2})/`,      // /2025/12/05/
		`/(\d{4})-(\d{2})-(\d{2})/`,      // /2025-12-05/
		`session-(\d{4})-(\d{2})-(\d{2})`, // session-2025-12-05
		`(\d{4})-(\d{2})-(\d{2})T`,        // 2025-12-05T (ISO timestamp in filename)
	}

	for _, pattern := range datePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(path)
		if len(matches) >= 4 {
			year := parseIntOrZero(matches[1])
			month := parseIntOrZero(matches[2])
			day := parseIntOrZero(matches[3])
			if year > 0 && month >= 1 && month <= 12 && day >= 1 && day <= 31 {
				return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			}
		}
	}

	return time.Time{} // Zero time if no date found
}

// parseIntOrZero parses a string as int, returning 0 on error.
func parseIntOrZero(s string) int {
	result := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

// isSameProject checks if a session path is from the same project as the current workspace.
func isSameProject(sessionPath, currentWorkspace string) bool {
	// Normalize paths for comparison
	sessionPath = strings.ToLower(sessionPath)
	currentWorkspace = strings.ToLower(currentWorkspace)

	// Check if the session path contains the workspace directory name
	// This is a heuristic - session paths often include the project name
	if currentWorkspace == "" {
		return false
	}

	// Extract the last component of the workspace path (project name)
	parts := strings.Split(currentWorkspace, "/")
	if len(parts) > 0 {
		projectName := parts[len(parts)-1]
		if projectName != "" && strings.Contains(sessionPath, projectName) {
			return true
		}
	}

	return false
}

// normalizeScore normalizes a CASS score to 0.0-1.0 range.
// CASS scores can vary in range depending on the search algorithm.
func normalizeScore(score float64) float64 {
	// Assume CASS returns scores in 0-100 or 0-1 range
	if score > 1.0 {
		// Likely 0-100 scale, normalize
		return score / 100.0
	}
	return score
}

// sortScoredHits sorts hits by computed score in descending order (highest first).
func sortScoredHits(hits []ScoredHit) {
	// Simple insertion sort for small slices (typically <20 items)
	for i := 1; i < len(hits); i++ {
		key := hits[i]
		j := i - 1
		for j >= 0 && hits[j].ComputedScore < key.ComputedScore {
			hits[j+1] = hits[j]
			j--
		}
		hits[j+1] = key
	}
}

// QueryAndFilterCASS combines QueryCASS and FilterResults in one call.
// This is a convenience function for the common use case.
func QueryAndFilterCASS(prompt string, queryConfig CASSConfig, filterConfig FilterConfig) (CASSQueryResult, FilterResult) {
	queryResult := QueryCASS(prompt, queryConfig)

	if !queryResult.Success || len(queryResult.Hits) == 0 {
		return queryResult, FilterResult{OriginalCount: 0}
	}

	filterResult := FilterResults(queryResult.Hits, filterConfig)
	return queryResult, filterResult
}
