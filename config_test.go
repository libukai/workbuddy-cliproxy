package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestDefaultConfigUsesEmbeddedModelsAndDisablesPromptRewrite(t *testing.T) {
	resetPluginConfig(t)
	if promptRewriteEnabled() {
		t.Fatal("prompt rewriting must be disabled by default")
	}
	if got := len(configuredModels()); got != 16 {
		t.Fatalf("embedded model count = %d, want 16", got)
	}

	payload := []byte(`{"messages":[{"role":"system","content":"You are Claude Code, Anthropic's official CLI for Claude."}]}`)
	if got := rewriteSystemForUpstream(payload); string(got) != string(payload) {
		t.Fatalf("default config rewrote the prompt: %s", got)
	}
}

func TestReconfigureEnablesPromptRewrite(t *testing.T) {
	resetPluginConfig(t)
	raw := lifecyclePayload(t, "prompt_rewrite: true\n")
	if _, err := handleMethod(pluginabi.MethodPluginReconfigure, raw); err != nil {
		t.Fatalf("plugin reconfigure: %v", err)
	}
	if !promptRewriteEnabled() {
		t.Fatal("prompt rewriting was not enabled")
	}
	payload := []byte(`{"messages":[{"role":"system","content":"You are Claude Code, Anthropic's official CLI for Claude."}]}`)
	got := string(rewriteSystemForUpstream(payload))
	if !strings.Contains(got, "official CLI tool for Claude") {
		t.Fatalf("enabled prompt rewrite did not apply: %s", got)
	}
}

func TestReconfigureLoadsAbsoluteModelManifest(t *testing.T) {
	resetPluginConfig(t)
	manifestPath := filepath.Join(t.TempDir(), "models.yaml")
	manifest := []byte("models:\n  - id: test-model\n    display_name: Test Model\n    context_length: 4096\n    max_completion_tokens: 512\n")
	if err := os.WriteFile(manifestPath, manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	raw := lifecyclePayload(t, "model_manifest: "+manifestPath+"\n")
	if _, err := handleMethod(pluginabi.MethodPluginReconfigure, raw); err != nil {
		t.Fatalf("plugin reconfigure: %v", err)
	}
	models := configuredModels()
	if len(models) != 1 || models[0].ID != "test-model" || models[0].ContextLength != 4096 || models[0].MaxCompletionTokens != 512 {
		t.Fatalf("configured models = %+v", models)
	}
}

func TestReconfigureRejectsRelativeOrDuplicateManifest(t *testing.T) {
	resetPluginConfig(t)
	before := configuredModels()
	if _, err := handleMethod(pluginabi.MethodPluginReconfigure, lifecyclePayload(t, "model_manifest: models.yaml\n")); err == nil {
		t.Fatal("relative model_manifest was accepted")
	}
	assertModelCatalogEqual(t, configuredModels(), before)

	manifestPath := filepath.Join(t.TempDir(), "duplicate.yaml")
	manifest := []byte("models:\n  - id: duplicate\n    context_length: 4096\n  - id: duplicate\n    context_length: 4096\n")
	if err := os.WriteFile(manifestPath, manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := handleMethod(pluginabi.MethodPluginReconfigure, lifecyclePayload(t, "model_manifest: "+manifestPath+"\n")); err == nil {
		t.Fatal("duplicate model IDs were accepted")
	}
	assertModelCatalogEqual(t, configuredModels(), before)
}

func assertModelCatalogEqual(t *testing.T, got, want []pluginapi.ModelInfo) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("model count changed after rejected config: got %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index].ID != want[index].ID {
			t.Fatalf("model %d changed after rejected config: got %q, want %q", index, got[index].ID, want[index].ID)
		}
	}
}

func lifecyclePayload(t *testing.T, configYAML string) []byte {
	t.Helper()
	raw, err := json.Marshal(lifecycleRequest{ConfigYAML: []byte(configYAML)})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func resetPluginConfig(t *testing.T) {
	t.Helper()
	if err := configurePlugin(nil); err != nil {
		t.Fatalf("reset plugin config: %v", err)
	}
	t.Cleanup(func() {
		_ = configurePlugin(nil)
	})
}
