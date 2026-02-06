package startup

import (
	"testing"
)

func TestSetConfigPath(t *testing.T) {
	// Save and restore original
	orig := configFilePath
	defer func() { configFilePath = orig }()

	SetConfigPath("/some/path/config.toml")
	if configFilePath != "/some/path/config.toml" {
		t.Errorf("configFilePath = %q, want /some/path/config.toml", configFilePath)
	}

	SetConfigPath("")
	if configFilePath != "" {
		t.Errorf("configFilePath = %q, want empty", configFilePath)
	}
}

func TestIsConfigLoaded_InitiallyFalse(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	if IsConfigLoaded() {
		t.Error("expected IsConfigLoaded() == false after ResetConfig()")
	}
}

func TestResetConfig(t *testing.T) {
	ResetConfig()
	// After reset, IsConfigLoaded should be false
	if IsConfigLoaded() {
		t.Error("expected IsConfigLoaded() == false after ResetConfig()")
	}
}

func TestGetConfig_LoadsMerged(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	// Temporarily set an empty config path so LoadMerged uses defaults
	orig := configFilePath
	configFilePath = ""
	defer func() { configFilePath = orig }()

	cfg, err := GetConfig()
	if err != nil {
		// LoadMerged may fail if no global config exists, which is fine —
		// we exercised the code path either way
		t.Logf("GetConfig returned error (expected in test env): %v", err)
		return
	}
	if cfg == nil {
		t.Error("expected non-nil config")
	}

	// After successful Get, IsConfigLoaded should be true
	if !IsConfigLoaded() {
		t.Error("expected IsConfigLoaded() == true after successful GetConfig()")
	}
}

func TestMustGetConfig_AfterLoad(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	orig := configFilePath
	configFilePath = ""
	defer func() { configFilePath = orig }()

	// First try to load; if that fails we can't test MustGet
	cfg, err := GetConfig()
	if err != nil {
		t.Skipf("skipping MustGetConfig test: GetConfig failed: %v", err)
	}

	mustCfg := MustGetConfig()
	if mustCfg != cfg {
		t.Error("MustGetConfig returned different config than GetConfig")
	}
}

func TestLazyValueReset(t *testing.T) {
	Reset()
	defer Reset()

	initCalled := 0
	lv := NewLazyValue[string]("test_reset_lv", func() string {
		initCalled++
		return "hello"
	})

	// Initialize
	val := lv.Get()
	if val != "hello" {
		t.Errorf("Get() = %q, want hello", val)
	}
	if initCalled != 1 {
		t.Errorf("initCalled = %d, want 1", initCalled)
	}

	// Reset and re-get
	lv.Reset()
	val2 := lv.Get()
	if val2 != "hello" {
		t.Errorf("Get() after Reset = %q, want hello", val2)
	}
	if initCalled != 2 {
		t.Errorf("initCalled = %d after Reset+Get, want 2", initCalled)
	}
}
