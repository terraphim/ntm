package agent

import (
	"strings"
	"testing"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser returned nil")
	}
}

func TestNewParserWithConfig(t *testing.T) {
	cfg := ParserConfig{
		ContextLowThreshold: 30.0,
		SampleLength:        200,
	}
	p := NewParserWithConfig(cfg)
	if p == nil {
		t.Fatal("NewParserWithConfig returned nil")
	}
}

func TestParser_Parse_EmptyOutput(t *testing.T) {
	p := NewParser()
	state, err := p.Parse("")

	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if state == nil {
		t.Fatal("Parse returned nil state")
	}
	if state.Type != AgentTypeUnknown {
		t.Errorf("Expected AgentTypeUnknown for empty output, got %v", state.Type)
	}
	// Empty output should have low confidence
	if state.Confidence > 0.5 {
		t.Errorf("Expected low confidence for empty output, got %f", state.Confidence)
	}
}

func TestParser_DetectAgentType_Claude(t *testing.T) {
	p := NewParser()

	outputs := []string{
		"Claude Opus 4.5 is ready",
		"Using sonnet 3.5 for this task",
		"Haiku model loaded",
	}

	for _, output := range outputs {
		t.Run(output, func(t *testing.T) {
			agentType := p.DetectAgentType(output)
			if agentType != AgentTypeClaudeCode {
				t.Errorf("DetectAgentType(%q) = %v, want %v", output, agentType, AgentTypeClaudeCode)
			}
		})
	}
}

func TestParser_DetectAgentType_Codex(t *testing.T) {
	p := NewParser()

	outputs := []string{
		"47% context left · ? for shortcuts",
		"OpenAI Codex CLI ready",
		"GPT-4 turbo model",
	}

	for _, output := range outputs {
		t.Run(output, func(t *testing.T) {
			agentType := p.DetectAgentType(output)
			if agentType != AgentTypeCodex {
				t.Errorf("DetectAgentType(%q) = %v, want %v", output, agentType, AgentTypeCodex)
			}
		})
	}
}

func TestParser_DetectAgentType_Gemini(t *testing.T) {
	p := NewParser()

	outputs := []string{
		"gemini-2.0-flash-preview ready",
		"YOLO mode: ON",
		"Google AI Studio connected",
	}

	for _, output := range outputs {
		t.Run(output, func(t *testing.T) {
			agentType := p.DetectAgentType(output)
			if agentType != AgentTypeGemini {
				t.Errorf("DetectAgentType(%q) = %v, want %v", output, agentType, AgentTypeGemini)
			}
		})
	}
}

func TestParser_Parse_RateLimited_Claude(t *testing.T) {
	p := NewParser()
	output := `Claude Opus 4.5 ready
Processing your request...
You've hit your limit. Please wait and try again later.`

	state, err := p.Parse(output)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if state.Type != AgentTypeClaudeCode {
		t.Errorf("Type = %v, want %v", state.Type, AgentTypeClaudeCode)
	}
	if !state.IsRateLimited {
		t.Error("Expected IsRateLimited to be true")
	}
	if len(state.LimitIndicators) == 0 {
		t.Error("Expected LimitIndicators to be populated")
	}
	if state.GetRecommendation() != RecommendRateLimitedWait {
		t.Errorf("Recommendation = %v, want %v", state.GetRecommendation(), RecommendRateLimitedWait)
	}
}

func TestParser_Parse_Working_CodeBlock(t *testing.T) {
	p := NewParser()
	output := `Claude Opus 4.5 ready
Let me write some code for you:
` + "```go" + `
package main

func main() {
    fmt.Println("Hello, World!")
}
` + "```"

	state, err := p.Parse(output)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if !state.IsWorking {
		t.Error("Expected IsWorking to be true when code block present")
	}
	if state.GetRecommendation() != RecommendDoNotInterrupt {
		t.Errorf("Recommendation = %v, want %v", state.GetRecommendation(), RecommendDoNotInterrupt)
	}
}

func TestParser_Parse_Idle_Claude(t *testing.T) {
	p := NewParser()
	output := `Task completed successfully.
What would you like me to do next?
Human: `

	state, err := p.Parse(output)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if !state.IsIdle {
		t.Error("Expected IsIdle to be true when prompt present")
	}
	if state.IsWorking {
		t.Error("Expected IsWorking to be false when idle")
	}
	if state.GetRecommendation() != RecommendSafeToRestart {
		t.Errorf("Recommendation = %v, want %v", state.GetRecommendation(), RecommendSafeToRestart)
	}
}

func TestParser_Parse_Codex_ContextExtraction(t *testing.T) {
	p := NewParser()
	output := `Processing your request...
Token usage: total=150,000 input=140,000 output=10,000
47% context left · ? for shortcuts
codex> `

	state, err := p.Parse(output)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if state.Type != AgentTypeCodex {
		t.Errorf("Type = %v, want %v", state.Type, AgentTypeCodex)
	}
	if state.ContextRemaining == nil {
		t.Fatal("Expected ContextRemaining to be set")
	}
	if *state.ContextRemaining != 47.0 {
		t.Errorf("ContextRemaining = %f, want 47.0", *state.ContextRemaining)
	}
	if state.TokensUsed == nil {
		t.Fatal("Expected TokensUsed to be set")
	}
	if *state.TokensUsed != 150000 {
		t.Errorf("TokensUsed = %d, want 150000", *state.TokensUsed)
	}
}

func TestParser_Parse_Codex_LowContext(t *testing.T) {
	p := NewParser()
	output := `Some work done...
10% context left · ? for shortcuts
codex> `

	state, err := p.Parse(output)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if state.ContextRemaining == nil {
		t.Fatal("Expected ContextRemaining to be set")
	}
	if *state.ContextRemaining != 10.0 {
		t.Errorf("ContextRemaining = %f, want 10.0", *state.ContextRemaining)
	}
	if !state.IsContextLow {
		t.Error("Expected IsContextLow to be true (10% < 20% threshold)")
	}
}

func TestParser_Parse_Gemini_Memory(t *testing.T) {
	p := NewParser()
	output := `gemini-2.0-flash-preview /model | 396.8 MB
Processing request...`

	state, err := p.Parse(output)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if state.Type != AgentTypeGemini {
		t.Errorf("Type = %v, want %v", state.Type, AgentTypeGemini)
	}
	if state.MemoryMB == nil {
		t.Fatal("Expected MemoryMB to be set")
	}
	if *state.MemoryMB != 396.8 {
		t.Errorf("MemoryMB = %f, want 396.8", *state.MemoryMB)
	}
}

func TestParser_Parse_WorkingWithLowContext(t *testing.T) {
	p := NewParser()
	output := `5% context left · ? for shortcuts
Writing to file.go...
` + "```go" + `
func example() {}
` + "```"

	state, err := p.Parse(output)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if !state.IsWorking {
		t.Error("Expected IsWorking to be true")
	}
	if !state.IsContextLow {
		t.Error("Expected IsContextLow to be true")
	}
	if state.GetRecommendation() != RecommendContextLowContinue {
		t.Errorf("Recommendation = %v, want %v", state.GetRecommendation(), RecommendContextLowContinue)
	}
}

func TestParser_Parse_ANSIStripping(t *testing.T) {
	p := NewParser()
	// Include ANSI color codes
	output := "\x1b[32mClaude Opus 4.5 ready\x1b[0m\n\x1b[1;31mYou've hit your limit\x1b[0m"

	state, err := p.Parse(output)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Should still detect patterns after stripping ANSI
	if state.Type != AgentTypeClaudeCode {
		t.Errorf("Type = %v, want %v (ANSI codes should be stripped)", state.Type, AgentTypeClaudeCode)
	}
	if !state.IsRateLimited {
		t.Error("Expected IsRateLimited to be true (pattern should match after ANSI stripping)")
	}
}

func TestParser_Parse_RawSample(t *testing.T) {
	p := NewParserWithConfig(ParserConfig{
		ContextLowThreshold: 20.0,
		SampleLength:        50,
	})

	// Create output longer than sample length
	output := strings.Repeat("x", 100)
	state, err := p.Parse(output)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(state.RawSample) != 50 {
		t.Errorf("RawSample length = %d, want 50", len(state.RawSample))
	}
}

func TestParser_Parse_ConfidenceScores(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		minConfidence float64
		maxConfidence float64
	}{
		{
			name:          "empty output has low confidence",
			output:        "",
			minConfidence: 0.0,
			maxConfidence: 0.3,
		},
		{
			name:          "codex with percentage has high confidence",
			output:        "OpenAI Codex\n47% context left · ? for shortcuts\ncodex> ",
			minConfidence: 0.7,
			maxConfidence: 1.0,
		},
		{
			name:          "rate limited has boosted confidence",
			output:        "Claude Opus 4.5\nYou've hit your limit",
			minConfidence: 0.6,
			maxConfidence: 1.0,
		},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := p.Parse(tt.output)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if state.Confidence < tt.minConfidence || state.Confidence > tt.maxConfidence {
				t.Errorf("Confidence = %f, want [%f, %f]",
					state.Confidence, tt.minConfidence, tt.maxConfidence)
			}
		})
	}
}

func TestParser_Parse_ErrorDetection(t *testing.T) {
	p := NewParser()
	output := `Claude Opus 4.5 ready
error: permission denied accessing /etc/passwd
Fatal: cannot continue`

	state, err := p.Parse(output)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if !state.IsInError {
		t.Error("Expected IsInError to be true")
	}
	if state.GetRecommendation() != RecommendErrorState {
		t.Errorf("Recommendation = %v, want %v", state.GetRecommendation(), RecommendErrorState)
	}
}

func TestParser_Parse_FileOperations(t *testing.T) {
	operations := []string{
		"Writing to internal/api/handler.go",
		"Created new file test.go",
		"Modified config.yaml",
		"Reading package.json",
		"Searching for pattern",
		"Running go test ./...",
	}

	p := NewParser()
	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			output := "Claude Opus 4.5\n" + op
			state, err := p.Parse(output)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if !state.IsWorking {
				t.Errorf("Expected IsWorking for %q", op)
			}
		})
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{5, 5, 5},
		{0, -1, -1},
	}

	for _, tt := range tests {
		if got := minInt(tt.a, tt.b); got != tt.want {
			t.Errorf("minInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// Test real-world-like outputs
func TestParser_RealWorldScenarios(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name        string
		output      string
		wantType    AgentType
		wantWorking bool
		wantIdle    bool
		wantLimited bool
	}{
		{
			name: "claude writing file",
			output: `Claude Opus 4.5 ready
I'll help you create that function.
Writing to internal/util/helper.go
` + "```go" + `
package util

func Helper() string {
    return "hello"
}
` + "```" + `
Done!`,
			wantType:    AgentTypeClaudeCode,
			wantWorking: true,
			wantIdle:    false,
			wantLimited: false,
		},
		{
			name: "codex waiting for input",
			output: `Completed refactoring.
Token usage: total=50,000 input=45,000 output=5,000
72% context left · ? for shortcuts
codex> `,
			wantType:    AgentTypeCodex,
			wantWorking: false,
			wantIdle:    true,
			wantLimited: false,
		},
		{
			name: "gemini rate limited",
			output: `gemini-2.0-flash-preview ready
Processing...
Error: quota exceeded. Please try again in 1 minute.`,
			wantType:    AgentTypeGemini,
			wantWorking: false,
			wantIdle:    false,
			wantLimited: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := p.Parse(tt.output)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if state.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", state.Type, tt.wantType)
			}
			if state.IsWorking != tt.wantWorking {
				t.Errorf("IsWorking = %v, want %v", state.IsWorking, tt.wantWorking)
			}
			if state.IsIdle != tt.wantIdle {
				t.Errorf("IsIdle = %v, want %v", state.IsIdle, tt.wantIdle)
			}
			if state.IsRateLimited != tt.wantLimited {
				t.Errorf("IsRateLimited = %v, want %v", state.IsRateLimited, tt.wantLimited)
			}
		})
	}
}
