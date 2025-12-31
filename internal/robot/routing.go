// Package robot provides machine-readable output for AI agents and automation.
// routing.go implements agent scoring and routing strategies for work distribution.
package robot

import (
	"math"
	"strings"
	"time"

	"github.com/Dicklesworthstone/ntm/internal/config"
	"github.com/Dicklesworthstone/ntm/internal/tmux"
)

// RoutingConfig holds configuration for agent routing and scoring.
type RoutingConfig struct {
	// Scoring weights (must sum to 1.0)
	ContextWeight float64 `toml:"context_weight"` // Default: 0.4
	StateWeight   float64 `toml:"state_weight"`   // Default: 0.4
	RecencyWeight float64 `toml:"recency_weight"` // Default: 0.2

	// Affinity settings
	AffinityEnabled bool    `toml:"affinity_enabled"` // Default: false
	AffinityBonus   float64 `toml:"affinity_bonus"`   // Default: 20

	// Exclusion thresholds
	ExcludeContextAbove   float64 `toml:"exclude_context_above"`   // Default: 85
	ExcludeIfGenerating   bool    `toml:"exclude_if_generating"`   // Default: true
	ExcludeIfRateLimited  bool    `toml:"exclude_if_rate_limited"` // Default: true
	ExcludeIfErrorState   bool    `toml:"exclude_if_error"`        // Default: true
}

// DefaultRoutingConfig returns sensible default configuration.
func DefaultRoutingConfig() RoutingConfig {
	return RoutingConfig{
		ContextWeight:        0.4,
		StateWeight:          0.4,
		RecencyWeight:        0.2,
		AffinityEnabled:      false,
		AffinityBonus:        20.0,
		ExcludeContextAbove:  85.0,
		ExcludeIfGenerating:  true,
		ExcludeIfRateLimited: true,
		ExcludeIfErrorState:  true,
	}
}

// ScoredAgent represents an agent with its computed routing score.
type ScoredAgent struct {
	// Identity
	PaneID    string `json:"pane_id"`
	AgentType string `json:"agent_type"` // cc, cod, gmi
	PaneIndex int    `json:"pane_index"`

	// Current state
	State      AgentState `json:"state"`
	Confidence float64    `json:"confidence"`
	Velocity   float64    `json:"velocity"`

	// Context usage (from robot-context, 0-100)
	ContextUsage float64 `json:"context_usage"`

	// Last activity timestamp
	LastActivity time.Time `json:"last_activity"`

	// Health state
	HealthState HealthState `json:"health_state"`
	RateLimited bool        `json:"rate_limited"`

	// Scoring results
	Score       float64       `json:"score"`       // Final composite score (0-100)
	Excluded    bool          `json:"excluded"`    // If true, agent should not receive work
	ExcludeReason string      `json:"exclude_reason,omitempty"`
	ScoreDetail ScoreBreakdown `json:"score_detail,omitempty"`
}

// ScoreBreakdown shows how the score was calculated.
type ScoreBreakdown struct {
	ContextScore  float64 `json:"context_score"`  // 0-100
	StateScore    float64 `json:"state_score"`    // -100 to 100, normalized to 0-100
	RecencyScore  float64 `json:"recency_score"`  // 0-100
	AffinityBonus float64 `json:"affinity_bonus"` // 0-20 (if enabled)

	// Weighted contributions
	ContextContrib float64 `json:"context_contrib"`
	StateContrib   float64 `json:"state_contrib"`
	RecencyContrib float64 `json:"recency_contrib"`
}

// HealthState represents agent health status.
type HealthState string

const (
	HealthHealthy     HealthState = "healthy"
	HealthDegraded    HealthState = "degraded"
	HealthUnhealthy   HealthState = "unhealthy"
	HealthRateLimited HealthState = "rate_limited"
)

// AgentScorer scores agents for routing decisions.
type AgentScorer struct {
	config   RoutingConfig
	monitor  *ActivityMonitor
}

// NewAgentScorer creates a new agent scorer with the given configuration.
func NewAgentScorer(cfg RoutingConfig) *AgentScorer {
	return &AgentScorer{
		config:  cfg,
		monitor: NewActivityMonitor(nil),
	}
}

// NewAgentScorerFromConfig creates a scorer using config file settings.
func NewAgentScorerFromConfig(cfg *config.Config) *AgentScorer {
	routingCfg := DefaultRoutingConfig()

	// TODO: Load from config.Config when routing section is added
	// For now, use defaults

	return NewAgentScorer(routingCfg)
}

// ScoreAgents calculates scores for all agents in a session.
func (s *AgentScorer) ScoreAgents(session string, prompt string) ([]ScoredAgent, error) {
	// Get all panes
	panes, err := tmux.GetPanes(session)
	if err != nil {
		return nil, err
	}

	var scored []ScoredAgent

	for _, pane := range panes {
		// Skip user pane
		agentType := detectAgentTypeFromTitle(pane.Title)
		if agentType == "" {
			continue
		}

		// Get activity state
		classifier := s.monitor.GetOrCreate(pane.ID)
		classifier.SetAgentType(agentType)
		activity, err := classifier.Classify()
		if err != nil {
			// If we can't classify, skip this agent
			continue
		}

		// Build scored agent
		agent := ScoredAgent{
			PaneID:       pane.ID,
			AgentType:    agentType,
			PaneIndex:    pane.Index,
			State:        activity.State,
			Confidence:   activity.Confidence,
			Velocity:     activity.Velocity,
			LastActivity: activity.LastOutput,
			HealthState:  deriveHealthState(activity.State),
			RateLimited:  false, // TODO: Detect from patterns
		}

		// Calculate score components
		agent.ScoreDetail = s.calculateScoreComponents(&agent, prompt)

		// Check exclusion rules first
		excluded, reason := s.checkExclusion(&agent)
		if excluded {
			agent.Excluded = true
			agent.ExcludeReason = reason
			agent.Score = 0
		} else {
			// Calculate final score
			agent.Score = s.calculateFinalScore(&agent)
		}

		scored = append(scored, agent)
	}

	return scored, nil
}

// calculateScoreComponents computes individual score components.
func (s *AgentScorer) calculateScoreComponents(agent *ScoredAgent, prompt string) ScoreBreakdown {
	breakdown := ScoreBreakdown{}

	// 1. Context Score (0-100)
	// Higher is better - agents with more room for context
	breakdown.ContextScore = 100 - agent.ContextUsage
	if breakdown.ContextScore < 0 {
		breakdown.ContextScore = 0
	}

	// 2. State Score (-100 to 100, then normalized to 0-100)
	rawStateScore := s.stateToScore(agent.State)
	// Normalize -100 to 100 range to 0 to 100
	breakdown.StateScore = (rawStateScore + 100) / 2

	// 3. Recency Score (0-100)
	breakdown.RecencyScore = s.recencyToScore(agent.LastActivity)

	// 4. Affinity Bonus (0-20)
	if s.config.AffinityEnabled && prompt != "" {
		breakdown.AffinityBonus = s.calculateAffinity(agent, prompt)
	}

	// Calculate weighted contributions
	breakdown.ContextContrib = breakdown.ContextScore * s.config.ContextWeight
	breakdown.StateContrib = breakdown.StateScore * s.config.StateWeight
	breakdown.RecencyContrib = breakdown.RecencyScore * s.config.RecencyWeight

	return breakdown
}

// stateToScore converts agent state to a score (-100 to 100).
func (s *AgentScorer) stateToScore(state AgentState) float64 {
	switch state {
	case StateWaiting:
		return 100 // Ready for work
	case StateThinking:
		return 50 // May become available soon
	case StateGenerating:
		return 0 // Currently busy
	case StateStalled:
		return -50 // May need attention
	case StateError:
		return -100 // Excluded
	case StateUnknown:
		return 25 // Uncertain, slightly prefer known states
	default:
		return 0
	}
}

// recencyToScore converts last activity time to a score (0-100).
func (s *AgentScorer) recencyToScore(lastActivity time.Time) float64 {
	if lastActivity.IsZero() {
		return 50 // Unknown, neutral score
	}

	age := time.Since(lastActivity)

	// Recent activity (< 1 min): Lower score - agent is "hot" but busy
	if age < time.Minute {
		return 20
	}

	// Medium (1-5 min): Moderate score
	if age < 5*time.Minute {
		return 50
	}

	// Idle (> 5 min): Higher score - ready for work
	if age < 30*time.Minute {
		return 80
	}

	// Very idle (> 30 min): Might be stale, but still available
	return 70
}

// calculateAffinity calculates affinity bonus based on prompt matching.
func (s *AgentScorer) calculateAffinity(agent *ScoredAgent, prompt string) float64 {
	// TODO: Implement affinity matching
	// This would look at the agent's recent work and compare to the prompt
	// For now, return 0
	return 0
}

// checkExclusion checks if an agent should be excluded from routing.
func (s *AgentScorer) checkExclusion(agent *ScoredAgent) (bool, string) {
	// Error state always excluded
	if agent.State == StateError {
		return true, "agent in ERROR state"
	}

	// Rate limited
	if s.config.ExcludeIfRateLimited && agent.RateLimited {
		return true, "agent is rate limited"
	}

	// Unhealthy
	if agent.HealthState == HealthUnhealthy {
		return true, "agent is unhealthy"
	}

	// High context usage
	if agent.ContextUsage > s.config.ExcludeContextAbove {
		return true, "context usage above threshold"
	}

	// Currently generating
	if s.config.ExcludeIfGenerating && agent.State == StateGenerating {
		return true, "agent is currently generating"
	}

	return false, ""
}

// calculateFinalScore computes the final routing score.
func (s *AgentScorer) calculateFinalScore(agent *ScoredAgent) float64 {
	d := agent.ScoreDetail

	// Sum weighted components
	score := d.ContextContrib + d.StateContrib + d.RecencyContrib

	// Add affinity bonus
	score += d.AffinityBonus

	// Clamp to 0-100 range
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return math.Round(score*100) / 100 // Round to 2 decimal places
}

// deriveHealthState derives health state from activity state.
func deriveHealthState(state AgentState) HealthState {
	switch state {
	case StateWaiting, StateThinking, StateGenerating:
		return HealthHealthy
	case StateStalled:
		return HealthDegraded
	case StateError:
		return HealthUnhealthy
	default:
		return HealthHealthy
	}
}

// GetBestAgent returns the agent with the highest score.
func (s *AgentScorer) GetBestAgent(scored []ScoredAgent) *ScoredAgent {
	var best *ScoredAgent

	for i := range scored {
		if scored[i].Excluded {
			continue
		}
		if best == nil || scored[i].Score > best.Score {
			best = &scored[i]
		}
	}

	return best
}

// GetAvailableAgents returns all non-excluded agents sorted by score.
func (s *AgentScorer) GetAvailableAgents(scored []ScoredAgent) []ScoredAgent {
	var available []ScoredAgent

	for _, agent := range scored {
		if !agent.Excluded {
			available = append(available, agent)
		}
	}

	// Sort by score descending
	for i := 0; i < len(available); i++ {
		for j := i + 1; j < len(available); j++ {
			if available[j].Score > available[i].Score {
				available[i], available[j] = available[j], available[i]
			}
		}
	}

	return available
}

// FilterByType filters agents by agent type (cc, cod, gmi).
func FilterByType(agents []ScoredAgent, agentType string) []ScoredAgent {
	if agentType == "" {
		return agents
	}

	var filtered []ScoredAgent
	for _, agent := range agents {
		if strings.EqualFold(agent.AgentType, agentType) {
			filtered = append(filtered, agent)
		}
	}
	return filtered
}

// FilterByPanes filters agents by pane indices.
func FilterByPanes(agents []ScoredAgent, paneIndices []int) []ScoredAgent {
	if len(paneIndices) == 0 {
		return agents
	}

	indexSet := make(map[int]bool)
	for _, idx := range paneIndices {
		indexSet[idx] = true
	}

	var filtered []ScoredAgent
	for _, agent := range agents {
		if indexSet[agent.PaneIndex] {
			filtered = append(filtered, agent)
		}
	}
	return filtered
}

// ExcludePanes excludes specific pane indices from the list.
func ExcludePanes(agents []ScoredAgent, excludeIndices []int) []ScoredAgent {
	if len(excludeIndices) == 0 {
		return agents
	}

	excludeSet := make(map[int]bool)
	for _, idx := range excludeIndices {
		excludeSet[idx] = true
	}

	var filtered []ScoredAgent
	for _, agent := range agents {
		if !excludeSet[agent.PaneIndex] {
			filtered = append(filtered, agent)
		}
	}
	return filtered
}

// =============================================================================
// Routing Strategies
// =============================================================================

// StrategyName represents a routing strategy identifier.
type StrategyName string

const (
	// StrategyLeastLoaded selects agent with highest score (default).
	StrategyLeastLoaded StrategyName = "least-loaded"

	// StrategyFirstAvailable selects first agent in WAITING state.
	StrategyFirstAvailable StrategyName = "first-available"

	// StrategyRoundRobin rotates through agents in order.
	StrategyRoundRobin StrategyName = "round-robin"

	// StrategyRoundRobinAvailable rotates but skips busy/unhealthy agents.
	StrategyRoundRobinAvailable StrategyName = "round-robin-available"

	// StrategyRandom randomly selects among available agents.
	StrategyRandom StrategyName = "random"

	// StrategySticky prefers same agent for related tasks.
	StrategySticky StrategyName = "sticky"

	// StrategyExplicit uses user-specified pane directly.
	StrategyExplicit StrategyName = "explicit"
)

// RoutingContext provides context for routing decisions.
type RoutingContext struct {
	Prompt       string // For affinity matching
	LastAgent    string // For sticky routing (pane ID of last used agent)
	ExcludePanes []int  // Pane indices to exclude
	ExplicitPane int    // For explicit routing (-1 = not set)
}

// RoutingStrategy defines the interface for routing strategies.
type RoutingStrategy interface {
	// Name returns the strategy identifier.
	Name() StrategyName

	// Select chooses an agent from the candidates.
	// Returns nil if no suitable agent found.
	Select(agents []ScoredAgent, ctx RoutingContext) *ScoredAgent
}

// RoutingResult represents the outcome of a routing decision.
type RoutingResult struct {
	Selected    *ScoredAgent   `json:"selected,omitempty"`
	Strategy    StrategyName   `json:"strategy"`
	Candidates  []ScoredAgent  `json:"candidates"`
	Excluded    []ScoredAgent  `json:"excluded,omitempty"`
	FallbackUsed bool          `json:"fallback_used"`
	Reason      string         `json:"reason,omitempty"`
}

// =============================================================================
// Strategy Implementations
// =============================================================================

// LeastLoadedStrategy selects the agent with the highest score.
type LeastLoadedStrategy struct{}

func (s *LeastLoadedStrategy) Name() StrategyName {
	return StrategyLeastLoaded
}

func (s *LeastLoadedStrategy) Select(agents []ScoredAgent, ctx RoutingContext) *ScoredAgent {
	var best *ScoredAgent
	for i := range agents {
		if agents[i].Excluded {
			continue
		}
		if best == nil || agents[i].Score > best.Score {
			best = &agents[i]
		}
	}
	return best
}

// FirstAvailableStrategy selects the first agent in WAITING state.
type FirstAvailableStrategy struct{}

func (s *FirstAvailableStrategy) Name() StrategyName {
	return StrategyFirstAvailable
}

func (s *FirstAvailableStrategy) Select(agents []ScoredAgent, ctx RoutingContext) *ScoredAgent {
	for i := range agents {
		if agents[i].Excluded {
			continue
		}
		if agents[i].State == StateWaiting {
			return &agents[i]
		}
	}
	return nil
}

// RoundRobinStrategy rotates through agents in order.
type RoundRobinStrategy struct {
	lastIndex int
}

func (s *RoundRobinStrategy) Name() StrategyName {
	return StrategyRoundRobin
}

func (s *RoundRobinStrategy) Select(agents []ScoredAgent, ctx RoutingContext) *ScoredAgent {
	if len(agents) == 0 {
		return nil
	}

	// Start from next agent after last used
	startIdx := (s.lastIndex + 1) % len(agents)

	// Round-robin ignores exclusion - use all agents
	selected := &agents[startIdx]
	s.lastIndex = startIdx
	return selected
}

// RoundRobinAvailableStrategy rotates but skips busy/unhealthy agents.
type RoundRobinAvailableStrategy struct {
	lastIndex int
}

func (s *RoundRobinAvailableStrategy) Name() StrategyName {
	return StrategyRoundRobinAvailable
}

func (s *RoundRobinAvailableStrategy) Select(agents []ScoredAgent, ctx RoutingContext) *ScoredAgent {
	if len(agents) == 0 {
		return nil
	}

	// Try to find next available agent starting from last index
	for i := 0; i < len(agents); i++ {
		idx := (s.lastIndex + 1 + i) % len(agents)
		if !agents[idx].Excluded {
			s.lastIndex = idx
			return &agents[idx]
		}
	}

	return nil
}

// RandomStrategy randomly selects among available agents.
type RandomStrategy struct {
	randFunc func(int) int // Injected for testing
}

func (s *RandomStrategy) Name() StrategyName {
	return StrategyRandom
}

func (s *RandomStrategy) Select(agents []ScoredAgent, ctx RoutingContext) *ScoredAgent {
	// Collect available agents
	var available []*ScoredAgent
	for i := range agents {
		if !agents[i].Excluded {
			available = append(available, &agents[i])
		}
	}

	if len(available) == 0 {
		return nil
	}

	// Use injected random function or simple modulo
	idx := 0
	if s.randFunc != nil {
		idx = s.randFunc(len(available))
	} else {
		// Deterministic fallback for testing
		idx = len(available) / 2
	}

	return available[idx]
}

// StickyStrategy prefers the same agent for related tasks.
type StickyStrategy struct {
	fallback RoutingStrategy
}

func NewStickyStrategy() *StickyStrategy {
	return &StickyStrategy{
		fallback: &LeastLoadedStrategy{},
	}
}

func (s *StickyStrategy) Name() StrategyName {
	return StrategySticky
}

func (s *StickyStrategy) Select(agents []ScoredAgent, ctx RoutingContext) *ScoredAgent {
	// If we have a last agent, prefer it if still available
	if ctx.LastAgent != "" {
		for i := range agents {
			if agents[i].PaneID == ctx.LastAgent && !agents[i].Excluded {
				return &agents[i]
			}
		}
	}

	// Fall back to least-loaded
	return s.fallback.Select(agents, ctx)
}

// ExplicitStrategy uses user-specified pane directly.
type ExplicitStrategy struct{}

func (s *ExplicitStrategy) Name() StrategyName {
	return StrategyExplicit
}

func (s *ExplicitStrategy) Select(agents []ScoredAgent, ctx RoutingContext) *ScoredAgent {
	if ctx.ExplicitPane < 0 {
		return nil
	}

	for i := range agents {
		if agents[i].PaneIndex == ctx.ExplicitPane {
			return &agents[i]
		}
	}

	return nil
}

// =============================================================================
// Router
// =============================================================================

// Router applies routing strategies to select agents.
type Router struct {
	strategies    map[StrategyName]RoutingStrategy
	defaultStrat  RoutingStrategy
	fallbackOrder []RoutingStrategy
}

// NewRouter creates a new router with all strategies registered.
func NewRouter() *Router {
	r := &Router{
		strategies:   make(map[StrategyName]RoutingStrategy),
		defaultStrat: &LeastLoadedStrategy{},
	}

	// Register all strategies
	r.RegisterStrategy(&LeastLoadedStrategy{})
	r.RegisterStrategy(&FirstAvailableStrategy{})
	r.RegisterStrategy(&RoundRobinStrategy{})
	r.RegisterStrategy(&RoundRobinAvailableStrategy{})
	r.RegisterStrategy(&RandomStrategy{})
	r.RegisterStrategy(NewStickyStrategy())
	r.RegisterStrategy(&ExplicitStrategy{})

	// Default fallback order
	r.fallbackOrder = []RoutingStrategy{
		&LeastLoadedStrategy{},     // Try best score first
		&FirstAvailableStrategy{},  // Then any waiting agent
	}

	return r
}

// RegisterStrategy registers a routing strategy.
func (r *Router) RegisterStrategy(s RoutingStrategy) {
	r.strategies[s.Name()] = s
}

// GetStrategy returns a strategy by name, or the default if not found.
func (r *Router) GetStrategy(name StrategyName) RoutingStrategy {
	if s, ok := r.strategies[name]; ok {
		return s
	}
	return r.defaultStrat
}

// Route selects an agent using the specified strategy.
func (r *Router) Route(agents []ScoredAgent, strategy StrategyName, ctx RoutingContext) RoutingResult {
	result := RoutingResult{
		Strategy:   strategy,
		Candidates: filterExcluded(agents, false),
		Excluded:   filterExcluded(agents, true),
	}

	// Apply exclusion from context
	if len(ctx.ExcludePanes) > 0 {
		agents = ExcludePanes(agents, ctx.ExcludePanes)
	}

	// Get the strategy
	strat := r.GetStrategy(strategy)

	// Try primary strategy
	selected := strat.Select(agents, ctx)
	if selected != nil {
		result.Selected = selected
		result.Reason = "primary strategy succeeded"
		return result
	}

	// Try fallback chain
	for _, fb := range r.fallbackOrder {
		if fb.Name() == strategy {
			continue // Skip if same as primary
		}
		selected = fb.Select(agents, ctx)
		if selected != nil {
			result.Selected = selected
			result.FallbackUsed = true
			result.Reason = "fallback to " + string(fb.Name())
			return result
		}
	}

	result.Reason = "no suitable agent found"
	return result
}

// RouteWithRelaxation tries routing with progressively relaxed constraints.
func (r *Router) RouteWithRelaxation(agents []ScoredAgent, strategy StrategyName, ctx RoutingContext) RoutingResult {
	// First try with normal constraints
	result := r.Route(agents, strategy, ctx)
	if result.Selected != nil {
		return result
	}

	// Relax constraint: include THINKING agents
	relaxedAgents := make([]ScoredAgent, len(agents))
	copy(relaxedAgents, agents)
	for i := range relaxedAgents {
		if relaxedAgents[i].ExcludeReason == "agent is currently generating" &&
			relaxedAgents[i].State == StateThinking {
			relaxedAgents[i].Excluded = false
			relaxedAgents[i].ExcludeReason = ""
		}
	}

	result = r.Route(relaxedAgents, strategy, ctx)
	if result.Selected != nil {
		result.Reason = "relaxed constraints (included THINKING)"
		return result
	}

	return result
}

// filterExcluded returns agents filtered by exclusion status.
func filterExcluded(agents []ScoredAgent, excluded bool) []ScoredAgent {
	var result []ScoredAgent
	for _, a := range agents {
		if a.Excluded == excluded {
			result = append(result, a)
		}
	}
	return result
}

// GetStrategyNames returns all available strategy names.
func GetStrategyNames() []StrategyName {
	return []StrategyName{
		StrategyLeastLoaded,
		StrategyFirstAvailable,
		StrategyRoundRobin,
		StrategyRoundRobinAvailable,
		StrategyRandom,
		StrategySticky,
		StrategyExplicit,
	}
}

// IsValidStrategy checks if a strategy name is valid.
func IsValidStrategy(name StrategyName) bool {
	switch name {
	case StrategyLeastLoaded, StrategyFirstAvailable, StrategyRoundRobin,
		StrategyRoundRobinAvailable, StrategyRandom, StrategySticky, StrategyExplicit:
		return true
	default:
		return false
	}
}
