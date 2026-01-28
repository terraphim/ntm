// Package ensemble provides types and utilities for multi-agent reasoning ensembles.
// metrics.go provides a central ObservabilityMetrics struct that aggregates
// coverage, velocity, redundancy, and conflict metrics for ensemble runs.
package ensemble

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
)

// ObservabilityMetrics aggregates all ensemble observability metrics.
// It provides a unified interface for computing metrics from normalized
// mode outputs and budget tracker data.
type ObservabilityMetrics struct {
	// Coverage tracks which reasoning categories were invoked.
	Coverage *CoverageMap

	// Velocity tracks findings per token spent.
	Velocity *VelocityTracker

	// Redundancy measures output similarity across modes.
	Redundancy *RedundancyAnalysis

	// Conflicts tracks disagreement density across mode pairs.
	Conflicts *ConflictTracker

	// Budget holds token usage information from the BudgetTracker.
	Budget *BudgetState

	// ComputedAt is when the metrics were calculated.
	ComputedAt time.Time

	// ModeIDs lists the mode IDs that were analyzed (sorted for determinism).
	ModeIDs []string

	// catalog is used for coverage suggestions.
	catalog *ModeCatalog
}

// MetricsReport is the JSON-serializable output of ObservabilityMetrics.
type MetricsReport struct {
	// Coverage summary.
	Coverage *CoverageReport `json:"coverage,omitempty"`

	// Velocity summary.
	Velocity *VelocityReport `json:"velocity,omitempty"`

	// Redundancy summary.
	Redundancy *RedundancyAnalysis `json:"redundancy,omitempty"`

	// ConflictDensity summary.
	ConflictDensity *ConflictDensity `json:"conflict_density,omitempty"`

	// BudgetEfficiency is a summary of budget usage.
	BudgetEfficiency *BudgetEfficiency `json:"budget_efficiency,omitempty"`

	// ComputedAt is when the report was generated.
	ComputedAt string `json:"computed_at"`

	// ModeCount is the number of modes analyzed.
	ModeCount int `json:"mode_count"`

	// Suggestions are actionable recommendations.
	Suggestions []string `json:"suggestions,omitempty"`
}

// BudgetEfficiency summarizes token usage efficiency.
type BudgetEfficiency struct {
	TotalTokens      int     `json:"total_tokens"`
	TotalFindings    int     `json:"total_findings"`
	FindingsPerKTok  float64 `json:"findings_per_k_tokens"`
	BudgetUsedPct    float64 `json:"budget_used_pct"`
	Underperformers  []string `json:"underperformers,omitempty"`
	EfficiencyRating string   `json:"efficiency_rating"`
}

// NewObservabilityMetrics creates a new metrics aggregator.
func NewObservabilityMetrics(catalog *ModeCatalog) *ObservabilityMetrics {
	return &ObservabilityMetrics{
		Coverage:  NewCoverageMap(catalog),
		Velocity:  NewVelocityTracker(),
		Conflicts: NewConflictTracker(),
		catalog:   catalog,
		ModeIDs:   []string{},
	}
}

// ComputeFromOutputs calculates all metrics from normalized mode outputs.
// Optionally accepts budget state and audit report for richer analysis.
func (om *ObservabilityMetrics) ComputeFromOutputs(
	outputs []ModeOutput,
	budget *BudgetState,
	auditReport *AuditReport,
) error {
	if om == nil {
		return fmt.Errorf("metrics is nil")
	}

	om.ComputedAt = time.Now().UTC()
	om.Budget = budget

	// Extract mode IDs (sorted for determinism)
	modeIDs := make([]string, 0, len(outputs))
	for _, output := range outputs {
		modeIDs = append(modeIDs, output.ModeID)
	}
	sort.Strings(modeIDs)
	om.ModeIDs = modeIDs

	slog.Debug("computing observability metrics",
		"mode_ids", modeIDs,
		"output_count", len(outputs),
	)

	// Compute coverage
	for _, output := range outputs {
		om.Coverage.RecordMode(output.ModeID)
	}

	// Compute velocity (need token counts from budget or estimates)
	for _, output := range outputs {
		tokens := 0
		if budget != nil && budget.PerAgentSpent != nil {
			tokens = budget.PerAgentSpent[output.ModeID]
		}
		if tokens == 0 {
			// Fall back to estimation
			tokens = EstimateModeOutputTokens(&output)
		}
		om.Velocity.RecordOutput(output.ModeID, output, tokens)
	}

	// Compute redundancy
	om.Redundancy = CalculateRedundancy(outputs)

	// Compute conflict density
	if auditReport != nil {
		om.Conflicts.FromAudit(auditReport)
	} else {
		om.Conflicts.DetectConflicts(outputs)
	}

	return nil
}

// GetReport generates a comprehensive metrics report.
func (om *ObservabilityMetrics) GetReport() *MetricsReport {
	if om == nil {
		return &MetricsReport{
			ComputedAt:  time.Now().UTC().Format(time.RFC3339),
			Suggestions: []string{"No metrics available"},
		}
	}

	report := &MetricsReport{
		ComputedAt: om.ComputedAt.Format(time.RFC3339),
		ModeCount:  len(om.ModeIDs),
	}

	// Coverage
	if om.Coverage != nil {
		report.Coverage = om.Coverage.CalculateCoverage()
	}

	// Velocity
	if om.Velocity != nil {
		report.Velocity = om.Velocity.CalculateVelocity()
	}

	// Redundancy
	report.Redundancy = om.Redundancy

	// Conflict density
	if om.Conflicts != nil {
		totalPairs := (len(om.ModeIDs) * (len(om.ModeIDs) - 1)) / 2
		report.ConflictDensity = om.Conflicts.GetDensity(totalPairs)
	}

	// Budget efficiency
	report.BudgetEfficiency = om.calculateBudgetEfficiency()

	// Generate suggestions
	report.Suggestions = om.generateSuggestions(report)

	return report
}

// calculateBudgetEfficiency computes budget usage metrics.
func (om *ObservabilityMetrics) calculateBudgetEfficiency() *BudgetEfficiency {
	if om.Velocity == nil {
		return nil
	}

	velocityReport := om.Velocity.CalculateVelocity()

	totalTokens := 0
	totalFindings := len(om.Velocity.uniqueFindings)
	for _, entry := range velocityReport.PerMode {
		totalTokens += entry.TokensSpent
	}

	efficiency := &BudgetEfficiency{
		TotalTokens:     totalTokens,
		TotalFindings:   totalFindings,
		Underperformers: velocityReport.LowPerformers,
	}

	if totalTokens > 0 {
		efficiency.FindingsPerKTok = float64(totalFindings) / float64(totalTokens) * 1000
	}

	if om.Budget != nil && om.Budget.TotalLimit > 0 {
		efficiency.BudgetUsedPct = float64(om.Budget.TotalSpent) / float64(om.Budget.TotalLimit) * 100
	}

	// Rate efficiency
	switch {
	case efficiency.FindingsPerKTok >= 3.0:
		efficiency.EfficiencyRating = "excellent"
	case efficiency.FindingsPerKTok >= 2.0:
		efficiency.EfficiencyRating = "good"
	case efficiency.FindingsPerKTok >= 1.0:
		efficiency.EfficiencyRating = "acceptable"
	default:
		efficiency.EfficiencyRating = "low"
	}

	slog.Debug("budget efficiency calculated",
		"total_tokens", totalTokens,
		"total_findings", totalFindings,
		"findings_per_k_tok", efficiency.FindingsPerKTok,
		"rating", efficiency.EfficiencyRating,
	)

	return efficiency
}

// generateSuggestions creates actionable recommendations based on metrics.
func (om *ObservabilityMetrics) generateSuggestions(report *MetricsReport) []string {
	var suggestions []string

	// Coverage suggestions
	if report.Coverage != nil && len(report.Coverage.BlindSpots) > 0 {
		for _, blindSpot := range report.Coverage.BlindSpots {
			suggestions = append(suggestions,
				fmt.Sprintf("Consider adding %s reasoning (%s) coverage",
					blindSpot.String(), blindSpot.CategoryLetter()))
		}
		slog.Info("coverage blind spots detected",
			"blind_spots", len(report.Coverage.BlindSpots),
		)
	}

	// Velocity suggestions
	if report.Velocity != nil && len(report.Velocity.LowPerformers) > 0 {
		for _, mode := range report.Velocity.LowPerformers {
			suggestions = append(suggestions,
				fmt.Sprintf("%s has low findings velocity - consider early stop or replacement", mode))
		}
		slog.Info("velocity threshold crossed",
			"low_performers", report.Velocity.LowPerformers,
		)
	}

	// Redundancy suggestions
	if report.Redundancy != nil && report.Redundancy.OverallScore >= 0.5 {
		suggestions = append(suggestions,
			"High redundancy detected - consider more diverse mode selection")
		slog.Info("high redundancy detected",
			"overall_score", report.Redundancy.OverallScore,
		)
	}

	// Conflict density suggestions
	if report.ConflictDensity != nil && report.ConflictDensity.UnresolvedConflicts > 0 {
		suggestions = append(suggestions,
			fmt.Sprintf("%d unresolved conflicts - review high-conflict pairs in synthesis report",
				report.ConflictDensity.UnresolvedConflicts))
	}

	// Budget efficiency suggestions
	if report.BudgetEfficiency != nil {
		if report.BudgetEfficiency.EfficiencyRating == "low" {
			suggestions = append(suggestions,
				"Low budget efficiency - consider adjusting mode selection for better findings/token ratio")
		}
		if report.BudgetEfficiency.BudgetUsedPct > 90 {
			suggestions = append(suggestions,
				"Budget nearly exhausted - consider increasing limits for future runs")
		}
	}

	return suggestions
}

// Render produces a human-readable metrics dashboard.
func (om *ObservabilityMetrics) Render() string {
	if om == nil {
		return "No metrics data available"
	}

	var b strings.Builder
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	b.WriteString("                   OBSERVABILITY METRICS DASHBOARD              \n")
	b.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	// Coverage section
	if om.Coverage != nil {
		b.WriteString(om.Coverage.Render())
		b.WriteString("\n")
	}

	// Velocity section
	if om.Velocity != nil {
		b.WriteString(om.Velocity.Render())
		b.WriteString("\n")
	}

	// Redundancy section
	if om.Redundancy != nil {
		b.WriteString(om.Redundancy.Render())
		b.WriteString("\n")
	}

	// Conflict density section
	if om.Conflicts != nil {
		b.WriteString(om.renderConflictDensity())
		b.WriteString("\n")
	}

	// Budget efficiency section
	b.WriteString(om.renderBudgetEfficiency())

	// Suggestions section
	report := om.GetReport()
	if len(report.Suggestions) > 0 {
		b.WriteString("\n───────────────────────────────────────────────────────────────\n")
		b.WriteString("ACTION ITEMS:\n")
		for i, suggestion := range report.Suggestions {
			fmt.Fprintf(&b, " %d. %s\n", i+1, suggestion)
		}
	}

	b.WriteString("\n═══════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(&b, "Report generated: %s\n", om.ComputedAt.Format(time.RFC3339))

	return b.String()
}

// renderConflictDensity produces a human-readable conflict density section.
func (om *ObservabilityMetrics) renderConflictDensity() string {
	if om.Conflicts == nil {
		return "Conflict Analysis: No data available\n"
	}

	totalPairs := (len(om.ModeIDs) * (len(om.ModeIDs) - 1)) / 2
	density := om.Conflicts.GetDensity(totalPairs)

	var b strings.Builder
	b.WriteString("Conflict Analysis:\n")
	fmt.Fprintf(&b, "Total Conflicts: %d\n", density.TotalConflicts)
	fmt.Fprintf(&b, "Resolved: %d\n", density.ResolvedConflicts)
	fmt.Fprintf(&b, "Unresolved: %d\n", density.UnresolvedConflicts)
	if totalPairs > 0 {
		fmt.Fprintf(&b, "Conflicts per Pair: %.2f\n", density.ConflictsPerPair)
	}
	if len(density.HighConflictPairs) > 0 {
		b.WriteString("\nHigh-Conflict Pairs:\n")
		for _, pair := range density.HighConflictPairs {
			fmt.Fprintf(&b, "  %s\n", pair)
		}
	}
	fmt.Fprintf(&b, "Source: %s\n", density.Source)

	return b.String()
}

// renderBudgetEfficiency produces a human-readable budget efficiency section.
func (om *ObservabilityMetrics) renderBudgetEfficiency() string {
	efficiency := om.calculateBudgetEfficiency()
	if efficiency == nil {
		return "Budget Efficiency: No data available\n"
	}

	var b strings.Builder
	b.WriteString("Budget Efficiency:\n")
	fmt.Fprintf(&b, "Total Tokens: %d\n", efficiency.TotalTokens)
	fmt.Fprintf(&b, "Total Unique Findings: %d\n", efficiency.TotalFindings)
	fmt.Fprintf(&b, "Findings per 1K Tokens: %.2f\n", efficiency.FindingsPerKTok)
	fmt.Fprintf(&b, "Rating: %s\n", efficiency.EfficiencyRating)

	if om.Budget != nil && om.Budget.TotalLimit > 0 {
		fmt.Fprintf(&b, "Budget Used: %.1f%% (%d/%d tokens)\n",
			efficiency.BudgetUsedPct, om.Budget.TotalSpent, om.Budget.TotalLimit)
	}

	if len(efficiency.Underperformers) > 0 {
		b.WriteString("Underperformers: ")
		b.WriteString(strings.Join(efficiency.Underperformers, ", "))
		b.WriteString("\n")
	}

	return b.String()
}

// PostRunReport generates a comprehensive post-run metrics report.
// This is suitable for persisting to a file or returning via API.
func (om *ObservabilityMetrics) PostRunReport() string {
	if om == nil {
		return "No metrics data available for post-run report"
	}

	report := om.GetReport()

	var b strings.Builder
	b.WriteString("# Ensemble Metrics Report\n\n")
	fmt.Fprintf(&b, "Generated: %s\n", report.ComputedAt)
	fmt.Fprintf(&b, "Modes Analyzed: %d\n\n", report.ModeCount)

	// Coverage section
	if report.Coverage != nil {
		b.WriteString("## Category Coverage\n\n")
		fmt.Fprintf(&b, "Overall Coverage: %.0f%%\n\n", report.Coverage.Overall*100)
		if len(report.Coverage.BlindSpots) > 0 {
			b.WriteString("**Blind Spots:** ")
			blindSpotNames := make([]string, 0, len(report.Coverage.BlindSpots))
			for _, bs := range report.Coverage.BlindSpots {
				blindSpotNames = append(blindSpotNames, fmt.Sprintf("%s (%s)", bs.String(), bs.CategoryLetter()))
			}
			b.WriteString(strings.Join(blindSpotNames, ", "))
			b.WriteString("\n\n")
		}
	}

	// Velocity section
	if report.Velocity != nil {
		b.WriteString("## Findings Velocity\n\n")
		fmt.Fprintf(&b, "Overall: %.2f findings per 1K tokens\n\n", report.Velocity.Overall)
		if len(report.Velocity.HighPerformers) > 0 {
			b.WriteString("**High Performers:** ")
			b.WriteString(strings.Join(report.Velocity.HighPerformers, ", "))
			b.WriteString("\n")
		}
		if len(report.Velocity.LowPerformers) > 0 {
			b.WriteString("**Low Performers:** ")
			b.WriteString(strings.Join(report.Velocity.LowPerformers, ", "))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Redundancy section
	if report.Redundancy != nil {
		b.WriteString("## Redundancy Analysis\n\n")
		fmt.Fprintf(&b, "Overall Score: %.2f (%s)\n\n", report.Redundancy.OverallScore,
			interpretScore(report.Redundancy.OverallScore))
		highPairs := report.Redundancy.GetHighRedundancyPairs(0.5)
		if len(highPairs) > 0 {
			b.WriteString("**High Redundancy Pairs:**\n")
			for _, pair := range highPairs {
				fmt.Fprintf(&b, "- %s ↔ %s: %.0f%%\n", pair.ModeA, pair.ModeB, pair.Similarity*100)
			}
			b.WriteString("\n")
		}
	}

	// Conflict density section
	if report.ConflictDensity != nil {
		b.WriteString("## Conflict Density\n\n")
		fmt.Fprintf(&b, "Total: %d | Resolved: %d | Unresolved: %d\n",
			report.ConflictDensity.TotalConflicts,
			report.ConflictDensity.ResolvedConflicts,
			report.ConflictDensity.UnresolvedConflicts)
		if len(report.ConflictDensity.HighConflictPairs) > 0 {
			b.WriteString("\n**High-Conflict Pairs:** ")
			b.WriteString(strings.Join(report.ConflictDensity.HighConflictPairs, ", "))
		}
		b.WriteString("\n\n")
	}

	// Budget efficiency section
	if report.BudgetEfficiency != nil {
		b.WriteString("## Budget Efficiency\n\n")
		fmt.Fprintf(&b, "- Tokens Used: %d\n", report.BudgetEfficiency.TotalTokens)
		fmt.Fprintf(&b, "- Unique Findings: %d\n", report.BudgetEfficiency.TotalFindings)
		fmt.Fprintf(&b, "- Efficiency: %.2f findings/1K tokens (%s)\n",
			report.BudgetEfficiency.FindingsPerKTok, report.BudgetEfficiency.EfficiencyRating)
		b.WriteString("\n")
	}

	// Suggestions section
	if len(report.Suggestions) > 0 {
		b.WriteString("## Recommendations\n\n")
		for i, suggestion := range report.Suggestions {
			fmt.Fprintf(&b, "%d. %s\n", i+1, suggestion)
		}
	}

	return b.String()
}
