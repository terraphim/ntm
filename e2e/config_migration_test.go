//go:build e2e
// +build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findValidationResultByType(report *ValidationReport, resultType string) *ValidationResult {
	if report == nil {
		return nil
	}
	for i := range report.Results {
		if report.Results[i].Type == resultType {
			return &report.Results[i]
		}
	}
	return nil
}

func warningByField(result *ValidationResult, field string) *ValidationIssue {
	if result == nil {
		return nil
	}
	for i := range result.Warnings {
		if result.Warnings[i].Field == field {
			return &result.Warnings[i]
		}
	}
	return nil
}

func hasErrorContaining(result *ValidationResult, needle string) bool {
	if result == nil {
		return false
	}
	for _, issue := range result.Errors {
		if strings.Contains(issue.Message, needle) {
			return true
		}
	}
	return false
}

// TestConfigValidateFixCreatesMissingTemplatesDir verifies the --fix remediation path.
func TestConfigValidateFixCreatesMissingTemplatesDir(t *testing.T) {
	SkipIfShort(t)
	SkipIfNoNTM(t)

	suite := NewValidateTestSuite(t, "config-migration-fix-templates")
	defer suite.Teardown()

	if err := suite.Setup(); err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Setup failed: %v", err)
	}

	const projectConfig = `[templates]
dir = "templates"
`
	if err := suite.CreateFile(".ntm/config.toml", projectConfig); err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Failed to write project config: %v", err)
	}

	templatesDir := filepath.Join(suite.NtmDir(), "templates")
	if _, err := os.Stat(templatesDir); !os.IsNotExist(err) {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Expected templates dir to be absent before validate --fix, err=%v", err)
	}

	before, err := suite.RunValidate("--all")
	if err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] validate before --fix failed: %v", err)
	}

	projectBefore := findValidationResultByType(before, "project")
	if projectBefore == nil {
		t.Fatal("[E2E-CONFIG-MIGRATION] Expected project validation result")
	}
	missingTemplatesWarning := warningByField(projectBefore, "templates.dir")
	if missingTemplatesWarning == nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Expected templates.dir warning before --fix, warnings=%+v", projectBefore.Warnings)
	}
	if !missingTemplatesWarning.Fixable {
		t.Fatal("[E2E-CONFIG-MIGRATION] Expected templates.dir warning to be marked fixable")
	}

	after, err := suite.RunValidate("--all", "--fix")
	if err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] validate with --fix failed: %v", err)
	}

	projectAfter := findValidationResultByType(after, "project")
	if projectAfter == nil {
		t.Fatal("[E2E-CONFIG-MIGRATION] Expected project validation result after --fix")
	}
	foundCreatedInfo := false
	for _, info := range projectAfter.Info {
		if strings.Contains(info, "created templates dir:") {
			foundCreatedInfo = true
			break
		}
	}
	if !foundCreatedInfo {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Expected created templates dir info after --fix, info=%v", projectAfter.Info)
	}

	if _, err := os.Stat(templatesDir); err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Expected templates dir to exist after --fix: %v", err)
	}

	suite.Logger().Log("[E2E-CONFIG-MIGRATION] --fix created missing templates dir successfully")
}

// TestConfigValidateDeprecatedStrategyUpgradePath verifies migration guidance for deprecated strategies.
func TestConfigValidateDeprecatedStrategyUpgradePath(t *testing.T) {
	SkipIfShort(t)
	SkipIfNoNTM(t)

	suite := NewValidateTestSuite(t, "config-migration-deprecated-strategy")
	defer suite.Teardown()

	if err := suite.Setup(); err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Setup failed: %v", err)
	}

	xdgConfigHome := filepath.Join(suite.TempDir(), "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
	mainConfigDir := filepath.Join(xdgConfigHome, "ntm")
	if err := os.MkdirAll(mainConfigDir, 0o755); err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Failed creating XDG config dir: %v", err)
	}

	projectsBase := filepath.Join(suite.TempDir(), "projects")
	if err := os.MkdirAll(projectsBase, 0o755); err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Failed creating projects base: %v", err)
	}

	deprecatedConfig := `projects_base = "` + projectsBase + `"

[ensemble.synthesis]
strategy = "debate"
`
	mainConfigPath := filepath.Join(mainConfigDir, "config.toml")
	if err := os.WriteFile(mainConfigPath, []byte(deprecatedConfig), 0o644); err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Failed writing deprecated config: %v", err)
	}

	before, err := suite.RunValidate("--all")
	if err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] validate with deprecated strategy failed unexpectedly: %v", err)
	}
	mainBefore := findValidationResultByType(before, "main")
	if mainBefore == nil {
		t.Fatal("[E2E-CONFIG-MIGRATION] Expected main validation result")
	}
	if !hasErrorContaining(mainBefore, `strategy "debate" is deprecated; use "dialectical" instead`) {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Expected deprecated-strategy migration hint, errors=%+v", mainBefore.Errors)
	}

	upgradedConfig := strings.Replace(deprecatedConfig, `"debate"`, `"dialectical"`, 1)
	if err := os.WriteFile(mainConfigPath, []byte(upgradedConfig), 0o644); err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Failed writing upgraded config: %v", err)
	}

	after, err := suite.RunValidate("--all")
	if err != nil {
		t.Fatalf("[E2E-CONFIG-MIGRATION] validate with upgraded strategy failed unexpectedly: %v", err)
	}
	mainAfter := findValidationResultByType(after, "main")
	if mainAfter == nil {
		t.Fatal("[E2E-CONFIG-MIGRATION] Expected main validation result after upgrade")
	}
	if hasErrorContaining(mainAfter, "deprecated") {
		t.Fatalf("[E2E-CONFIG-MIGRATION] Deprecated strategy error persisted after upgrade, errors=%+v", mainAfter.Errors)
	}

	suite.Logger().Log("[E2E-CONFIG-MIGRATION] Deprecated strategy upgrade path validated")
}
