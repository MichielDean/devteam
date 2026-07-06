package config

// ConfigMerger overlays DB providers with YAML seed providers (DB wins on name
// conflict; YAML-only rows are added). The DB is authoritative for admin-UI-managed
// state; YAML is additive seed for unmanaged providers. See app-design §4.4, U-DATA-05.
//
// Empty DB + empty YAML → empty result → ResolveProvider returns nil,nil (FR-006).
type ConfigMerger struct {
	providerStore *ProviderStore
}

// NewConfigMerger constructs a ConfigMerger backed by the given ProviderStore.
// The providerStore may be nil (YAML-only config); in that case MergeProviders
// returns the YAML providers unchanged.
func NewConfigMerger(store *ProviderStore) *ConfigMerger {
	return &ConfigMerger{providerStore: store}
}

// MergeProviders returns the merged provider list (DB + YAML, DB wins on name).
// yamlProviders may be empty/nil (DB-only). If the DB store is nil, returns YAML.
func (m *ConfigMerger) MergeProviders(yamlProviders []ProviderConfig) []ProviderConfig {
	if m.providerStore == nil {
		return dedupAndSort(yamlProviders)
	}
	dbProviders, err := m.providerStore.Providers()
	if err != nil {
		// DB read failure is non-fatal: fall back to YAML-only (graceful degradation).
		// The dispatch path will surface a clearer error if the YAML config is also empty.
		return dedupAndSort(yamlProviders)
	}
	if len(dbProviders) == 0 && len(yamlProviders) == 0 {
		return []ProviderConfig{}
	}

	// DB wins on name conflict; YAML-only rows are added.
	dbByName := map[string]ProviderConfig{}
	for _, p := range dbProviders {
		dbByName[p.Name] = p
	}
	merged := append([]ProviderConfig{}, dbProviders...)
	for _, yp := range yamlProviders {
		if _, exists := dbByName[yp.Name]; !exists {
			merged = append(merged, yp)
		}
	}
	return dedupAndSort(merged)
}

// dedupAndSort removes duplicate names (keeping the first occurrence) and sorts
// by name for deterministic resolution output.
func dedupAndSort(providers []ProviderConfig) []ProviderConfig {
	if len(providers) == 0 {
		return []ProviderConfig{}
	}
	seen := map[string]bool{}
	result := make([]ProviderConfig, 0, len(providers))
	for _, p := range providers {
		if seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		result = append(result, p)
	}
	// Stable sort by name (deterministic; matches store ordering).
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j-1].Name > result[j].Name; j-- {
			result[j-1], result[j] = result[j], result[j-1]
		}
	}
	return result
}