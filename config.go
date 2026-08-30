package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
	"gopkg.in/yaml.v3"
)

//go:embed models.yaml
var embeddedModelManifest []byte

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
}

type pluginConfig struct {
	PromptRewrite bool   `yaml:"prompt_rewrite"`
	ModelManifest string `yaml:"model_manifest"`
}

type modelManifest struct {
	Models []modelSpec `yaml:"models"`
}

type modelSpec struct {
	ID                  string `yaml:"id"`
	DisplayName         string `yaml:"display_name"`
	ContextLength       int64  `yaml:"context_length"`
	MaxCompletionTokens int64  `yaml:"max_completion_tokens"`
	LastVerified        string `yaml:"last_verified,omitempty"`
}

var configuredState = struct {
	sync.RWMutex
	config pluginConfig
	models []pluginapi.ModelInfo
}{}

func init() {
	if err := configurePlugin(nil); err != nil {
		configuredState.Lock()
		configuredState.models = nil
		configuredState.Unlock()
	}
}

func configurePlugin(raw []byte) error {
	var request lifecycleRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &request); err != nil {
			return fmt.Errorf("decode plugin lifecycle request: %w", err)
		}
	}
	cfg := pluginConfig{}
	if len(request.ConfigYAML) > 0 {
		if err := yaml.Unmarshal(request.ConfigYAML, &cfg); err != nil {
			return fmt.Errorf("decode workbuddy plugin config: %w", err)
		}
	}
	cfg.ModelManifest = strings.TrimSpace(cfg.ModelManifest)
	manifestBytes := embeddedModelManifest
	if cfg.ModelManifest != "" {
		if !filepath.IsAbs(cfg.ModelManifest) {
			return fmt.Errorf("model_manifest must be an absolute path")
		}
		data, errRead := os.ReadFile(cfg.ModelManifest)
		if errRead != nil {
			return fmt.Errorf("read model_manifest: %w", errRead)
		}
		manifestBytes = data
	}
	models, errModels := parseModelManifest(manifestBytes)
	if errModels != nil {
		return errModels
	}
	configuredState.Lock()
	configuredState.config = cfg
	configuredState.models = models
	configuredState.Unlock()
	return nil
}

func parseModelManifest(raw []byte) ([]pluginapi.ModelInfo, error) {
	var manifest modelManifest
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("decode model manifest: %w", err)
	}
	if len(manifest.Models) == 0 {
		return nil, fmt.Errorf("model manifest must contain at least one model")
	}
	seen := make(map[string]struct{}, len(manifest.Models))
	models := make([]pluginapi.ModelInfo, 0, len(manifest.Models))
	for index, spec := range manifest.Models {
		spec.ID = strings.TrimSpace(spec.ID)
		if spec.ID == "" {
			return nil, fmt.Errorf("model manifest entry %d has an empty id", index)
		}
		if _, exists := seen[spec.ID]; exists {
			return nil, fmt.Errorf("model manifest contains duplicate id %q", spec.ID)
		}
		seen[spec.ID] = struct{}{}
		if spec.ContextLength <= 0 {
			return nil, fmt.Errorf("model %q context_length must be greater than zero", spec.ID)
		}
		if spec.MaxCompletionTokens <= 0 {
			spec.MaxCompletionTokens = 8192
		}
		displayName := strings.TrimSpace(spec.DisplayName)
		if displayName == "" {
			displayName = spec.ID
		}
		models = append(models, pluginapi.ModelInfo{
			ID:                         spec.ID,
			Object:                     "model",
			OwnedBy:                    providerName,
			DisplayName:                displayName,
			Name:                       spec.ID,
			SupportedGenerationMethods: []string{"chat"},
			ContextLength:              spec.ContextLength,
			MaxCompletionTokens:        spec.MaxCompletionTokens,
			UserDefined:                true,
		})
	}
	return models, nil
}

func configuredModels() []pluginapi.ModelInfo {
	configuredState.RLock()
	defer configuredState.RUnlock()
	models := make([]pluginapi.ModelInfo, len(configuredState.models))
	copy(models, configuredState.models)
	return models
}

func promptRewriteEnabled() bool {
	configuredState.RLock()
	defer configuredState.RUnlock()
	return configuredState.config.PromptRewrite
}
