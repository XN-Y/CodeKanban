package utils

import (
	"reflect"
	"testing"
)

func TestNormalizeWebSessionQuickInputConfig(t *testing.T) {
	input := WebSessionQuickInputConfig{
		Pinned: []string{"  Alpha  ", "", "Beta", "Alpha"},
		Recent: []string{" One ", "Two", "One", "", "Three", "Four", "Five", "Six", "Seven"},
	}

	got := NormalizeWebSessionQuickInputConfig(input)
	want := WebSessionQuickInputConfig{
		Pinned: []string{"Alpha", "Beta"},
		Recent: []string{"One", "Two", "Three", "Four", "Five", "Six"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeWebSessionQuickInputConfig() = %#v, want %#v", got, want)
	}
}

func TestNormalizeWebSessionQuickInputConfigEmpty(t *testing.T) {
	got := NormalizeWebSessionQuickInputConfig(WebSessionQuickInputConfig{})
	want := WebSessionQuickInputConfig{
		Pinned: []string{},
		Recent: []string{},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeWebSessionQuickInputConfig() = %#v, want %#v", got, want)
	}
}

func TestNormalizeDeveloperConfigDefaultsActiveCallTimeout(t *testing.T) {
	got := NormalizeDeveloperConfig(DeveloperConfig{})
	if got.WebSessionCodexDefaultSyncMode != "fast" {
		t.Fatalf("expected default sync mode fast, got %q", got.WebSessionCodexDefaultSyncMode)
	}
	if got.WebSessionActiveCallTimeout.EnabledMode != SettingModeDefault {
		t.Fatalf("expected enabled mode default, got %q", got.WebSessionActiveCallTimeout.EnabledMode)
	}
	if got.WebSessionActiveCallTimeout.TimeoutSeconds != defaultWebSessionActiveCallTimeoutSeconds {
		t.Fatalf("expected timeout %d, got %d", defaultWebSessionActiveCallTimeoutSeconds, got.WebSessionActiveCallTimeout.TimeoutSeconds)
	}
	if got.WebSessionActiveCallTimeout.PromptTemplate != DefaultWebSessionActiveCallTimeoutPrompt {
		t.Fatalf("expected default prompt template, got %q", got.WebSessionActiveCallTimeout.PromptTemplate)
	}
	if !got.WebSessionActiveCallTimeout.CallKinds.UseDefault {
		t.Fatal("expected useDefault call kinds to be enabled")
	}
	if !got.WebSessionActiveCallTimeout.CallKinds.MCP || !got.WebSessionActiveCallTimeout.CallKinds.Command || !got.WebSessionActiveCallTimeout.CallKinds.Tool {
		t.Fatalf("expected default call kinds to all be enabled, got %#v", got.WebSessionActiveCallTimeout.CallKinds)
	}
}

func TestNormalizeWebSessionActiveCallTimeoutConfigClampsAndTrims(t *testing.T) {
	got := NormalizeWebSessionActiveCallTimeoutConfig(WebSessionActiveCallTimeoutConfig{
		EnabledMode:    SettingMode("ON"),
		TimeoutSeconds: 2,
		PromptTemplate: "  custom ${call} / ${duration}  ",
		CallKinds: WebSessionActiveCallTimeoutKindsConfig{
			UseDefault: false,
			MCP:        true,
		},
	})
	if got.EnabledMode != SettingModeOn {
		t.Fatalf("expected enabled mode on, got %q", got.EnabledMode)
	}
	if got.TimeoutSeconds != minWebSessionActiveCallTimeoutSeconds {
		t.Fatalf("expected timeout to clamp to %d, got %d", minWebSessionActiveCallTimeoutSeconds, got.TimeoutSeconds)
	}
	if got.PromptTemplate != "custom ${call} / ${duration}" {
		t.Fatalf("expected prompt template to be trimmed, got %q", got.PromptTemplate)
	}
	if got.CallKinds.UseDefault {
		t.Fatal("expected useDefault to remain disabled")
	}
	if !got.CallKinds.MCP || got.CallKinds.Command || got.CallKinds.Tool {
		t.Fatalf("expected only MCP to remain enabled, got %#v", got.CallKinds)
	}
}

func TestMergeDeveloperConfigPreservesNestedTimeoutConfigForLegacyPayloads(t *testing.T) {
	current := NormalizeDeveloperConfig(DeveloperConfig{
		WebSessionCodexDefaultSyncMode: "deep",
		WebSessionActiveCallTimeout: WebSessionActiveCallTimeoutConfig{
			EnabledMode:    SettingModeOff,
			TimeoutSeconds: 120,
			PromptTemplate: "resume ${call}",
			CallKinds: WebSessionActiveCallTimeoutKindsConfig{
				UseDefault: false,
				Command:    true,
			},
		},
	})

	merged := MergeDeveloperConfig(current, DeveloperConfig{
		EnableTerminalScrollback:      true,
		RenameSessionTitleEachCommand: true,
	})

	if !merged.EnableTerminalScrollback || !merged.RenameSessionTitleEachCommand {
		t.Fatalf("expected top-level developer config fields to update, got %#v", merged)
	}
	if merged.WebSessionActiveCallTimeout != current.WebSessionActiveCallTimeout {
		t.Fatalf("expected nested timeout config to be preserved, got %#v", merged.WebSessionActiveCallTimeout)
	}
}
