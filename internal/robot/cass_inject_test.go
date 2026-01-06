package robot

import (
	"testing"
)

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		minWords int // minimum expected keywords
		maxWords int // maximum expected keywords
		contains []string
		excludes []string
	}{
		{
			name:     "simple prompt",
			prompt:   "Fix the authentication bug in the login handler",
			minWords: 2,
			maxWords: 5,
			contains: []string{"authentication", "login", "handler"},
			excludes: []string{"the", "in", "fix", "bug"}, // stop words
		},
		{
			name:     "technical prompt",
			prompt:   "Implement retry logic with exponential backoff for database connections",
			minWords: 3,
			maxWords: 8,
			contains: []string{"retry", "logic", "exponential", "backoff", "database", "connections"},
			excludes: []string{"with", "for"},
		},
		{
			name:     "prompt with code block",
			prompt:   "Fix this function:\n```go\nfunc hello() { return }\n```\nThe return statement is wrong",
			minWords: 1,
			maxWords: 5,
			contains: []string{"return", "statement", "wrong"},
			excludes: []string{"func", "hello"}, // code block content should be removed
		},
		{
			name:     "prompt with inline code",
			prompt:   "The `getUserByID` function returns nil when user is not found",
			minWords: 2,
			maxWords: 6,
			contains: []string{"returns", "nil", "user", "found"},
			excludes: []string{"getuserbyid"}, // inline code should be removed
		},
		{
			name:     "empty prompt",
			prompt:   "",
			minWords: 0,
			maxWords: 0,
		},
		{
			name:     "only stop words",
			prompt:   "the and or but",
			minWords: 0,
			maxWords: 0,
		},
		{
			name:     "snake_case identifiers",
			prompt:   "Update the user_profile and order_items tables",
			minWords: 2,
			maxWords: 5,
			contains: []string{"user_profile", "order_items", "tables"},
		},
		{
			name:     "kebab-case identifiers",
			prompt:   "Check the api-gateway and load-balancer configs",
			minWords: 2,
			maxWords: 5,
			contains: []string{"api-gateway", "load-balancer", "configs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keywords := ExtractKeywords(tt.prompt)

			// Check count bounds
			if len(keywords) < tt.minWords {
				t.Errorf("ExtractKeywords() got %d keywords, want at least %d\nKeywords: %v",
					len(keywords), tt.minWords, keywords)
			}
			if len(keywords) > tt.maxWords {
				t.Errorf("ExtractKeywords() got %d keywords, want at most %d\nKeywords: %v",
					len(keywords), tt.maxWords, keywords)
			}

			// Check required keywords
			keywordSet := make(map[string]bool)
			for _, k := range keywords {
				keywordSet[k] = true
			}

			for _, required := range tt.contains {
				if !keywordSet[required] {
					t.Errorf("ExtractKeywords() missing required keyword %q\nKeywords: %v",
						required, keywords)
				}
			}

			// Check excluded keywords (stop words)
			for _, excluded := range tt.excludes {
				if keywordSet[excluded] {
					t.Errorf("ExtractKeywords() should not contain stop word %q\nKeywords: %v",
						excluded, keywords)
				}
			}
		})
	}
}

func TestExtractKeywords_Deduplication(t *testing.T) {
	prompt := "user user user authentication authentication"
	keywords := ExtractKeywords(prompt)

	// Count occurrences
	counts := make(map[string]int)
	for _, k := range keywords {
		counts[k]++
	}

	for word, count := range counts {
		if count > 1 {
			t.Errorf("ExtractKeywords() has duplicate keyword %q (count: %d)", word, count)
		}
	}
}

func TestExtractKeywords_MaxLimit(t *testing.T) {
	// Generate a prompt with many unique words
	prompt := "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen"
	keywords := ExtractKeywords(prompt)

	if len(keywords) > 10 {
		t.Errorf("ExtractKeywords() returned %d keywords, should be limited to 10", len(keywords))
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "simple words",
			text: "hello world",
			want: []string{"hello", "world"},
		},
		{
			name: "with punctuation",
			text: "hello, world!",
			want: []string{"hello", "world"},
		},
		{
			name: "snake_case",
			text: "user_profile",
			want: []string{"user_profile"},
		},
		{
			name: "kebab-case",
			text: "api-gateway",
			want: []string{"api-gateway"},
		},
		{
			name: "mixed",
			text: "user_profile api-gateway normalWord",
			want: []string{"user_profile", "api-gateway", "normalWord"},
		},
		{
			name: "with numbers",
			text: "error404 v2api",
			want: []string{"error404", "v2api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenize(tt.text)
			if len(got) != len(tt.want) {
				t.Errorf("tokenize() got %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("tokenize()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRemoveCodeBlocks(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "fenced code block",
			text: "before ```go\ncode here\n``` after",
			want: "before   after",
		},
		{
			name: "inline code",
			text: "the `function` name",
			want: "the   name",
		},
		{
			name: "multiple code blocks",
			text: "start ```code1``` middle ```code2``` end",
			want: "start   middle   end",
		},
		{
			name: "no code",
			text: "plain text here",
			want: "plain text here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeCodeBlocks(tt.text)
			if got != tt.want {
				t.Errorf("removeCodeBlocks() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsStopWord(t *testing.T) {
	// Test some stop words
	stopWords := []string{"the", "a", "is", "are", "and", "or", "but", "in", "on"}
	for _, word := range stopWords {
		if !isStopWord(word) {
			t.Errorf("isStopWord(%q) = false, want true", word)
		}
	}

	// Test some non-stop words
	nonStopWords := []string{"database", "authentication", "handler", "retry", "exponential"}
	for _, word := range nonStopWords {
		if isStopWord(word) {
			t.Errorf("isStopWord(%q) = true, want false", word)
		}
	}
}

func TestDefaultCASSConfig(t *testing.T) {
	config := DefaultCASSConfig()

	if !config.Enabled {
		t.Error("DefaultCASSConfig().Enabled should be true")
	}
	if config.MaxResults != 5 {
		t.Errorf("DefaultCASSConfig().MaxResults = %d, want 5", config.MaxResults)
	}
	if config.MaxAgeDays != 30 {
		t.Errorf("DefaultCASSConfig().MaxAgeDays = %d, want 30", config.MaxAgeDays)
	}
	if !config.PreferSameProject {
		t.Error("DefaultCASSConfig().PreferSameProject should be true")
	}
}

func TestQueryCASS_Disabled(t *testing.T) {
	config := CASSConfig{
		Enabled: false,
	}

	result := QueryCASS("test prompt", config)

	if !result.Success {
		t.Error("QueryCASS with disabled config should succeed")
	}
	if len(result.Hits) != 0 {
		t.Error("QueryCASS with disabled config should return no hits")
	}
}

func TestQueryCASS_EmptyKeywords(t *testing.T) {
	config := DefaultCASSConfig()

	// Prompt with only stop words should extract no keywords
	result := QueryCASS("the and or but", config)

	if !result.Success {
		t.Error("QueryCASS with no keywords should still succeed")
	}
	if result.Error != "no keywords extracted from prompt" {
		t.Errorf("QueryCASS error = %q, want 'no keywords extracted from prompt'", result.Error)
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{-1, "-1"},
		{-100, "-100"},
		{12345, "12345"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := itoa(tt.input)
			if got != tt.want {
				t.Errorf("itoa(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Relevance Filtering Tests
// =============================================================================

func TestDefaultFilterConfig(t *testing.T) {
	config := DefaultFilterConfig()

	if config.MinRelevance != 0.7 {
		t.Errorf("DefaultFilterConfig().MinRelevance = %f, want 0.7", config.MinRelevance)
	}
	if config.MaxItems != 5 {
		t.Errorf("DefaultFilterConfig().MaxItems = %d, want 5", config.MaxItems)
	}
	if !config.PreferSameProject {
		t.Error("DefaultFilterConfig().PreferSameProject should be true")
	}
	if config.MaxAgeDays != 30 {
		t.Errorf("DefaultFilterConfig().MaxAgeDays = %d, want 30", config.MaxAgeDays)
	}
	if config.RecencyBoost != 0.3 {
		t.Errorf("DefaultFilterConfig().RecencyBoost = %f, want 0.3", config.RecencyBoost)
	}
}

func TestFilterResults_Empty(t *testing.T) {
	config := DefaultFilterConfig()
	result := FilterResults([]CASSHit{}, config)

	if result.OriginalCount != 0 {
		t.Errorf("FilterResults() OriginalCount = %d, want 0", result.OriginalCount)
	}
	if result.FilteredCount != 0 {
		t.Errorf("FilterResults() FilteredCount = %d, want 0", result.FilteredCount)
	}
	if len(result.Hits) != 0 {
		t.Errorf("FilterResults() len(Hits) = %d, want 0", len(result.Hits))
	}
}

func TestFilterResults_BasicScoring(t *testing.T) {
	hits := []CASSHit{
		{SourcePath: "/path/to/session1.jsonl", Agent: "claude"},
		{SourcePath: "/path/to/session2.jsonl", Agent: "codex"},
		{SourcePath: "/path/to/session3.jsonl", Agent: "gemini"},
	}

	// Use low MinRelevance to ensure all pass
	config := FilterConfig{
		MinRelevance: 0.0,
		MaxItems:     10,
		MaxAgeDays:   0, // Disable age filtering
	}

	result := FilterResults(hits, config)

	if result.OriginalCount != 3 {
		t.Errorf("FilterResults() OriginalCount = %d, want 3", result.OriginalCount)
	}
	if result.FilteredCount != 3 {
		t.Errorf("FilterResults() FilteredCount = %d, want 3", result.FilteredCount)
	}

	// First result should have highest score (position-based)
	if len(result.Hits) < 1 {
		t.Fatal("Expected at least 1 hit")
	}
	if result.Hits[0].ComputedScore < result.Hits[len(result.Hits)-1].ComputedScore {
		t.Error("First hit should have higher score than last hit")
	}
}

func TestFilterResults_MinRelevance(t *testing.T) {
	hits := []CASSHit{
		{SourcePath: "/path/to/session1.jsonl", Agent: "claude"},
		{SourcePath: "/path/to/session2.jsonl", Agent: "codex"},
		{SourcePath: "/path/to/session3.jsonl", Agent: "gemini"},
	}

	// High MinRelevance should filter out lower-scored results
	config := FilterConfig{
		MinRelevance: 0.95, // Very high threshold
		MaxItems:     10,
		MaxAgeDays:   0,
	}

	result := FilterResults(hits, config)

	// Only the top result(s) should pass the high threshold
	if result.FilteredCount > 1 {
		t.Errorf("FilterResults() with high MinRelevance should filter most results, got %d", result.FilteredCount)
	}
	if result.RemovedByScore < 2 {
		t.Errorf("FilterResults() RemovedByScore = %d, expected at least 2", result.RemovedByScore)
	}
}

func TestFilterResults_MaxItems(t *testing.T) {
	hits := make([]CASSHit, 10)
	for i := range hits {
		hits[i] = CASSHit{SourcePath: "/path/to/session.jsonl", Agent: "claude"}
	}

	config := FilterConfig{
		MinRelevance: 0.0,
		MaxItems:     3, // Limit to 3
		MaxAgeDays:   0,
	}

	result := FilterResults(hits, config)

	if len(result.Hits) != 3 {
		t.Errorf("FilterResults() len(Hits) = %d, want 3", len(result.Hits))
	}
}

func TestFilterResults_SameProjectPreference(t *testing.T) {
	hits := []CASSHit{
		{SourcePath: "/some/other/project/session.jsonl", Agent: "claude"},
		{SourcePath: "/users/test/myproject/session.jsonl", Agent: "codex"},
	}

	config := FilterConfig{
		MinRelevance:      0.0,
		MaxItems:          10,
		MaxAgeDays:        0,
		PreferSameProject: true,
		CurrentWorkspace:  "/users/test/myproject",
	}

	result := FilterResults(hits, config)

	// The same-project hit should have a bonus and thus higher score
	if len(result.Hits) < 2 {
		t.Fatal("Expected 2 hits")
	}

	// Find the myproject hit and check it has project bonus
	var myprojectHit *ScoredHit
	for i := range result.Hits {
		if result.Hits[i].SourcePath == "/users/test/myproject/session.jsonl" {
			myprojectHit = &result.Hits[i]
			break
		}
	}

	if myprojectHit == nil {
		t.Fatal("Expected to find myproject hit")
	}
	if myprojectHit.ScoreDetail.ProjectBonus == 0 {
		t.Error("Same-project hit should have ProjectBonus > 0")
	}
}

func TestExtractSessionDate(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string // Expected date in YYYY-MM-DD format, empty if no date
	}{
		{
			name: "date in path components",
			path: "/Users/test/.codex/sessions/2025/12/05/session.jsonl",
			want: "2025-12-05",
		},
		{
			name: "date with dashes in path",
			path: "/some/path/2025-12-05/session.jsonl",
			want: "2025-12-05",
		},
		{
			name: "date in session filename",
			path: "/some/path/session-2025-12-05-abc123.jsonl",
			want: "2025-12-05",
		},
		{
			name: "ISO timestamp in filename",
			path: "/some/path/session-2025-12-05T14-30-00.json",
			want: "2025-12-05",
		},
		{
			name: "no date in path",
			path: "/some/path/session.jsonl",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSessionDate(tt.path)
			gotStr := ""
			if !got.IsZero() {
				gotStr = got.Format("2006-01-02")
			}
			if gotStr != tt.want {
				t.Errorf("extractSessionDate(%q) = %q, want %q", tt.path, gotStr, tt.want)
			}
		})
	}
}

func TestIsSameProject(t *testing.T) {
	tests := []struct {
		name             string
		sessionPath      string
		currentWorkspace string
		want             bool
	}{
		{
			name:             "matching project name",
			sessionPath:      "/users/test/.codex/myproject/session.jsonl",
			currentWorkspace: "/users/dev/myproject",
			want:             true,
		},
		{
			name:             "case insensitive match",
			sessionPath:      "/users/test/MyProject/session.jsonl",
			currentWorkspace: "/users/dev/myproject",
			want:             true,
		},
		{
			name:             "no match",
			sessionPath:      "/users/test/otherproject/session.jsonl",
			currentWorkspace: "/users/dev/myproject",
			want:             false,
		},
		{
			name:             "empty workspace",
			sessionPath:      "/users/test/myproject/session.jsonl",
			currentWorkspace: "",
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSameProject(tt.sessionPath, tt.currentWorkspace)
			if got != tt.want {
				t.Errorf("isSameProject(%q, %q) = %v, want %v",
					tt.sessionPath, tt.currentWorkspace, got, tt.want)
			}
		})
	}
}

func TestNormalizeScore(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{0.5, 0.5},    // Already 0-1 scale
		{1.0, 1.0},    // Already 0-1 scale
		{50.0, 0.5},   // 0-100 scale
		{100.0, 1.0},  // 0-100 scale
		{0.0, 0.0},    // Zero
	}

	for _, tt := range tests {
		got := normalizeScore(tt.input)
		if got != tt.want {
			t.Errorf("normalizeScore(%f) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestSortScoredHits(t *testing.T) {
	hits := []ScoredHit{
		{ComputedScore: 0.5},
		{ComputedScore: 0.9},
		{ComputedScore: 0.7},
		{ComputedScore: 0.3},
	}

	sortScoredHits(hits)

	// Should be sorted descending
	for i := 1; i < len(hits); i++ {
		if hits[i-1].ComputedScore < hits[i].ComputedScore {
			t.Errorf("sortScoredHits() not sorted descending: %f < %f at positions %d, %d",
				hits[i-1].ComputedScore, hits[i].ComputedScore, i-1, i)
		}
	}

	// First should be highest
	if hits[0].ComputedScore != 0.9 {
		t.Errorf("sortScoredHits() first item score = %f, want 0.9", hits[0].ComputedScore)
	}
}
