package redaction

import (
	"strings"
	"testing"
)

// Baseline performance numbers (AMD EPYC 7282, Go 1.25+):
//
// BenchmarkScanAndRedact_Short_NoSecrets-64            	    2904	   417153 ns/op
// BenchmarkScanAndRedact_Short_WithSecrets-64          	    2820	   392943 ns/op
// BenchmarkScanAndRedact_Medium_NoSecrets-64           	      42	 28265091 ns/op
// BenchmarkScanAndRedact_Medium_WithSecrets-64         	      69	 16879640 ns/op
// BenchmarkScanAndRedact_Large_NoSecrets-64            	       4	299269473 ns/op
// BenchmarkScanAndRedact_Large_WithSecrets-64          	       5	217843371 ns/op
// BenchmarkScanAndRedact_Adversarial_ReDoS-64          	    2786	   441854 ns/op
// BenchmarkScanAndRedact_Adversarial_ManyMatches-64    	     867	  1366036 ns/op
// BenchmarkModeComparison/Off-64                       	65845705	      16.70 ns/op
// BenchmarkModeComparison/Warn-64                      	   36267	    32094 ns/op
// BenchmarkModeComparison/Redact-64                    	   36717	    32564 ns/op
// BenchmarkModeComparison/Block-64                     	   37938	    33853 ns/op
//
// A significant regression (>3x slowdown) warrants investigation.

// ----------------------------------------------------------------------------
// Size-based benchmarks
// ----------------------------------------------------------------------------

// BenchmarkScanAndRedact_Short_NoSecrets measures performance on small inputs (1KB)
// with no secrets - the happy path.
func BenchmarkScanAndRedact_Short_NoSecrets(b *testing.B) {
	ResetPatterns()
	// 1KB of plain text without secrets.
	input := strings.Repeat("Hello world, this is a test message. ", 30) // ~1110 bytes
	if len(input) > 1024 {
		input = input[:1024]
	}

	cfg := Config{Mode: ModeRedact}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ScanAndRedact(input, cfg)
	}
}

// BenchmarkScanAndRedact_Short_WithSecrets measures performance on small inputs (1KB)
// with embedded secrets.
func BenchmarkScanAndRedact_Short_WithSecrets(b *testing.B) {
	ResetPatterns()
	// 1KB with embedded secrets.
	secret := "gh" + "p_" + strings.Repeat("x", 40)
	base := "API token: " + secret + " used here. "
	input := strings.Repeat(base, 20)
	if len(input) > 1024 {
		input = input[:1024]
	}

	cfg := Config{Mode: ModeRedact}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ScanAndRedact(input, cfg)
	}
}

// BenchmarkScanAndRedact_Medium_NoSecrets measures performance on medium inputs (50KB)
// without secrets.
func BenchmarkScanAndRedact_Medium_NoSecrets(b *testing.B) {
	ResetPatterns()
	// 50KB of plain text.
	input := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 1000)
	if len(input) > 50*1024 {
		input = input[:50*1024]
	}

	cfg := Config{Mode: ModeRedact}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ScanAndRedact(input, cfg)
	}
}

// BenchmarkScanAndRedact_Medium_WithSecrets measures performance on medium inputs (50KB)
// with scattered secrets.
func BenchmarkScanAndRedact_Medium_WithSecrets(b *testing.B) {
	ResetPatterns()
	// 50KB with secrets scattered throughout.
	secret := "gh" + "p_" + strings.Repeat("y", 40)
	cleanText := strings.Repeat("Normal text without secrets. ", 100)
	secretText := "Token: " + secret + " "
	// Insert secrets at ~10 positions.
	var builder strings.Builder
	for i := 0; i < 10; i++ {
		builder.WriteString(cleanText)
		builder.WriteString(secretText)
	}
	input := builder.String()
	if len(input) > 50*1024 {
		input = input[:50*1024]
	}

	cfg := Config{Mode: ModeRedact}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ScanAndRedact(input, cfg)
	}
}

// BenchmarkScanAndRedact_Large_NoSecrets measures performance on large inputs (500KB)
// without secrets.
func BenchmarkScanAndRedact_Large_NoSecrets(b *testing.B) {
	ResetPatterns()
	// 500KB of plain text.
	input := strings.Repeat("This is a long document with no sensitive content whatsoever. ", 8500)
	input = input[:500*1024]

	cfg := Config{Mode: ModeRedact}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ScanAndRedact(input, cfg)
	}
}

// BenchmarkScanAndRedact_Large_WithSecrets measures performance on large inputs (500KB)
// with secrets.
func BenchmarkScanAndRedact_Large_WithSecrets(b *testing.B) {
	ResetPatterns()
	// 500KB with secrets scattered throughout.
	ghToken := "gh" + "p_" + strings.Repeat("z", 40)
	awsKey := "AKIA" + strings.Repeat("A", 16)
	cleanText := strings.Repeat("Normal application output with logs. ", 200)
	secretText := "gh=" + ghToken + " aws=" + awsKey + " "

	var builder strings.Builder
	for i := 0; i < 50; i++ {
		builder.WriteString(cleanText)
		builder.WriteString(secretText)
	}
	input := builder.String()
	if len(input) > 500*1024 {
		input = input[:500*1024]
	}

	cfg := Config{Mode: ModeRedact}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ScanAndRedact(input, cfg)
	}
}

// ----------------------------------------------------------------------------
// Adversarial benchmarks (ReDoS resistance)
// ----------------------------------------------------------------------------

// BenchmarkScanAndRedact_Adversarial_ReDoS tests regex performance with
// pathological input patterns that could cause catastrophic backtracking
// in naive regex implementations.
func BenchmarkScanAndRedact_Adversarial_ReDoS(b *testing.B) {
	ResetPatterns()
	// Patterns that could cause backtracking in poorly written regexes:
	// - Long sequences of 'a' followed by something that almost matches.
	// - Near-matches that require the regex engine to retry many positions.
	input := strings.Repeat("a", 1000) + "password=" + strings.Repeat("a", 100)

	cfg := Config{Mode: ModeRedact}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ScanAndRedact(input, cfg)
	}
}

// BenchmarkScanAndRedact_Adversarial_ManyMatches tests performance when
// there are many overlapping potential matches.
func BenchmarkScanAndRedact_Adversarial_ManyMatches(b *testing.B) {
	ResetPatterns()
	// Many password-like patterns that all match.
	var builder strings.Builder
	for i := 0; i < 100; i++ {
		builder.WriteString("password=secret")
		builder.WriteString(strings.Repeat("x", 10))
		builder.WriteString(" ")
	}
	input := builder.String()

	cfg := Config{Mode: ModeRedact}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ScanAndRedact(input, cfg)
	}
}

// BenchmarkScanAndRedact_Adversarial_NearMisses tests performance with
// input that looks like secrets but just misses the pattern.
func BenchmarkScanAndRedact_Adversarial_NearMisses(b *testing.B) {
	ResetPatterns()
	// Near-misses: patterns that almost match but don't quite.
	// ghp_ followed by too few characters.
	nearMisses := []string{
		"ghp_short",        // too short
		"ghp_" + "x",       // way too short
		"AKIA" + "SHORT",   // too short
		"sk-" + "short",    // too short
		"password=",        // empty value
		"api_key=short",    // too short
		"Bearer x",         // too short
		"eyJhbGci",         // incomplete JWT
	}
	input := strings.Join(nearMisses, " ")
	input = strings.Repeat(input+" ", 100)

	cfg := Config{Mode: ModeRedact}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ScanAndRedact(input, cfg)
	}
}

// ----------------------------------------------------------------------------
// Mode comparison benchmarks
// ----------------------------------------------------------------------------

// BenchmarkModeComparison compares performance across different modes.
func BenchmarkModeComparison(b *testing.B) {
	ResetPatterns()
	secret := "gh" + "p_" + strings.Repeat("a", 40)
	input := "Normal text. Token: " + secret + " More text."

	modes := []struct {
		name string
		mode Mode
	}{
		{"Off", ModeOff},
		{"Warn", ModeWarn},
		{"Redact", ModeRedact},
		{"Block", ModeBlock},
	}

	for _, m := range modes {
		b.Run(m.name, func(b *testing.B) {
			cfg := Config{Mode: m.mode}
			for i := 0; i < b.N; i++ {
				_ = ScanAndRedact(input, cfg)
			}
		})
	}
}

// BenchmarkAllowlist compares performance with and without allowlist.
func BenchmarkAllowlist(b *testing.B) {
	ResetPatterns()
	secret := "gh" + "p_" + strings.Repeat("b", 40)
	input := "Token: " + secret + " used for testing."

	b.Run("NoAllowlist", func(b *testing.B) {
		cfg := Config{Mode: ModeRedact}
		for i := 0; i < b.N; i++ {
			_ = ScanAndRedact(input, cfg)
		}
	})

	b.Run("WithAllowlist", func(b *testing.B) {
		cfg := Config{
			Mode:      ModeRedact,
			Allowlist: []string{secret},
		}
		for i := 0; i < b.N; i++ {
			_ = ScanAndRedact(input, cfg)
		}
	})
}

// BenchmarkPatternCount measures overhead of pattern matching.
func BenchmarkPatternCount(b *testing.B) {
	ResetPatterns()
	input := strings.Repeat("Normal text without any matches. ", 100)

	b.Run("AllPatterns", func(b *testing.B) {
		cfg := Config{Mode: ModeRedact}
		for i := 0; i < b.N; i++ {
			_ = ScanAndRedact(input, cfg)
		}
	})

	b.Run("DisabledMostPatterns", func(b *testing.B) {
		cfg := Config{
			Mode: ModeRedact,
			DisabledCategories: []Category{
				CategoryOpenAIKey,
				CategoryAnthropicKey,
				CategoryGitHubToken,
				CategoryAWSAccessKey,
				CategoryAWSSecretKey,
				CategoryJWT,
				CategoryBearerToken,
				CategoryPrivateKey,
				CategoryDatabaseURL,
				CategoryGenericAPIKey,
				CategoryGenericSecret,
			},
		}
		for i := 0; i < b.N; i++ {
			_ = ScanAndRedact(input, cfg)
		}
	})
}
