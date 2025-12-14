package dashboard

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// applyDashboardEnvOverrides applies environment variable overrides to dashboard configuration
func applyDashboardEnvOverrides(m *Model) {
	if m == nil {
		return
	}

	if v := os.Getenv("NTM_DASHBOARD_REFRESH"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			m.refreshInterval = d
		}
	}

	if seconds, ok := envPositiveInt("NTM_DASH_PANE_REFRESH_SECS"); ok {
		m.paneRefreshInterval = time.Duration(seconds) * time.Second
	}
	if seconds, ok := envPositiveInt("NTM_DASH_CONTEXT_REFRESH_SECS"); ok {
		m.contextRefreshInterval = time.Duration(seconds) * time.Second
	}
	if seconds, ok := envPositiveInt("NTM_DASH_ALERTS_REFRESH_SECS"); ok {
		m.alertsRefreshInterval = time.Duration(seconds) * time.Second
	}
	if seconds, ok := envPositiveInt("NTM_DASH_BEADS_REFRESH_SECS"); ok {
		m.beadsRefreshInterval = time.Duration(seconds) * time.Second
	}
	if seconds, ok := envPositiveInt("NTM_DASH_CASS_REFRESH_SECS"); ok {
		m.cassContextRefreshInterval = time.Duration(seconds) * time.Second
	}

	if lines, ok := envPositiveInt("NTM_DASH_CAPTURE_LINES"); ok {
		m.paneOutputLines = lines
	}
	if budget, ok := envNonNegativeInt("NTM_DASH_CAPTURE_BUDGET"); ok {
		m.paneOutputCaptureBudget = budget
	}
}

func envPositiveInt(name string) (int, bool) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0, false
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, false
	}

	return parsed, true
}

func envNonNegativeInt(name string) (int, bool) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0, false
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, false
	}

	return parsed, true
}
