package config

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/MichielDean/devteam/internal/db"
)

// ProviderStore reads and writes ProviderConfig aggregates (provider + models)
// to the DB. The store only ever sees api_key_env references ($VAR), never raw
// key values (NFR-SEC-01). See business-logic-model §1.1, U-DATA-03.
type ProviderStore struct {
	db *db.DB
}

// NewProviderStore constructs a ProviderStore backed by the given DB.
func NewProviderStore(database *db.DB) *ProviderStore {
	return &ProviderStore{db: database}
}

// Providers returns all provider configs with their child models, ordered by name.
// KeyState is derived at read time via DeriveKeyState (never persisted).
func (s *ProviderStore) Providers() ([]ProviderConfig, error) {
	rows, err := s.db.Query(
		`SELECT name, display_name, enabled, base_url, api_key_env, default_model_id, npm_adapter, env_var_supported, preset_id
		 FROM providers ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("querying providers: %w", err)
	}
	defer rows.Close()

	var providers []ProviderConfig
	for rows.Next() {
		var p ProviderConfig
		var enabled, envVar int
		if err := rows.Scan(&p.Name, &p.DisplayName, &enabled, &p.BaseURL, &p.APIKeyEnv, &p.DefaultModelID, &p.NPMAdapter, &envVar, &p.PresetID); err != nil {
			return nil, fmt.Errorf("scanning provider: %w", err)
		}
		p.Enabled = enabled == 1
		p.EnvVarSupported = envVar == 1
		p.Models = []ProviderModel{} // initialize to empty (not nil) for JSON [] serialization
		p.KeyState = DeriveKeyState(p.APIKeyEnv)
		providers = append(providers, p)
	}
	if providers == nil {
		return []ProviderConfig{}, nil
	}

	// Load models for each provider in one query, then attach.
	modelRows, err := s.db.Query(
		`SELECT provider_name, model_id, friendly_name FROM provider_models ORDER BY provider_name, model_id`)
	if err != nil {
		return nil, fmt.Errorf("querying provider_models: %w", err)
	}
	defer modelRows.Close()
	modelsByProvider := map[string][]ProviderModel{}
	for modelRows.Next() {
		var m ProviderModel
		if err := modelRows.Scan(&m.ProviderName, &m.ModelID, &m.FriendlyName); err != nil {
			return nil, fmt.Errorf("scanning provider_model: %w", err)
		}
		modelsByProvider[m.ProviderName] = append(modelsByProvider[m.ProviderName], m)
	}
	for i := range providers {
		providers[i].Models = modelsByProvider[providers[i].Name]
		if providers[i].Models == nil {
			providers[i].Models = []ProviderModel{}
		}
	}
	return providers, nil
}

// Provider returns a single provider by name, or (nil, nil) if not found.
func (s *ProviderStore) Provider(name string) (*ProviderConfig, error) {
	providers, err := s.Providers()
	if err != nil {
		return nil, err
	}
	for _, p := range providers {
		if p.Name == name {
			cp := p
			return &cp, nil
		}
	}
	return nil, nil
}

// UpsertProvider inserts or updates a provider and its models in a single
// transaction. Models are replaced (delete-then-insert) to keep the aggregate
// consistent. No raw API key value is written — only the api_key_env reference.
func (s *ProviderStore) UpsertProvider(p ProviderConfig) error {
	tx, err := s.db.Conn().Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	enabled := 0
	if p.Enabled {
		enabled = 1
	}
	envVar := 1
	if !p.EnvVarSupported {
		envVar = 0
	}
	// Upsert provider row.
	_, err = tx.Exec(
		`INSERT INTO providers (name, display_name, enabled, base_url, api_key_env, default_model_id, npm_adapter, env_var_supported, preset_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (name) DO UPDATE SET
		   display_name = EXCLUDED.display_name,
		   enabled = EXCLUDED.enabled,
		   base_url = EXCLUDED.base_url,
		   api_key_env = EXCLUDED.api_key_env,
		   default_model_id = EXCLUDED.default_model_id,
		   npm_adapter = EXCLUDED.npm_adapter,
		   env_var_supported = EXCLUDED.env_var_supported,
		   preset_id = EXCLUDED.preset_id`,
		p.Name, p.DisplayName, enabled, p.BaseURL, p.APIKeyEnv, p.DefaultModelID, p.NPMAdapter, envVar, p.PresetID)
	if err != nil {
		return fmt.Errorf("upserting provider %s: %w", p.Name, err)
	}

	// Replace models (cascade-style: delete all, insert all).
	if _, err = tx.Exec("DELETE FROM provider_models WHERE provider_name = $1", p.Name); err != nil {
		return fmt.Errorf("deleting models for %s: %w", p.Name, err)
	}
	for _, m := range p.Models {
		if _, err = tx.Exec(
			`INSERT INTO provider_models (provider_name, model_id, friendly_name) VALUES ($1, $2, $3) ON CONFLICT (provider_name, model_id) DO UPDATE SET friendly_name = EXCLUDED.friendly_name`,
			p.Name, m.ModelID, m.FriendlyName); err != nil {
			return fmt.Errorf("inserting model %s/%s: %w", p.Name, m.ModelID, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit provider upsert: %w", err)
	}
	return nil
}

// DeleteProvider removes a provider; child models/tier_models/role_overrides
// cascade-delete via FK. Returns (nil) if the provider didn't exist.
func (s *ProviderStore) DeleteProvider(name string) error {
	_, err := s.db.Exec("DELETE FROM providers WHERE name = $1", name)
	if err != nil {
		return fmt.Errorf("deleting provider %s: %w", name, err)
	}
	return nil
}

// TierStore reads and writes tier_models and role_overrides, and resolves a
// tier to a (provider, model) per app-design §5.1. See U-DATA-04.
type TierStore struct {
	db *db.DB
}

// NewTierStore constructs a TierStore backed by the given DB.
func NewTierStore(database *db.DB) *TierStore {
	return &TierStore{db: database}
}

// TierModels returns all tier→provider→model assignments, as a map keyed by
// tier, then provider. e.g. tierModels["opus"]["anthropic"] = "claude-opus-4".
func (s *TierStore) TierModels() (map[string]map[string]string, error) {
	rows, err := s.db.Query("SELECT tier, provider_name, model_id FROM tier_models")
	if err != nil {
		return nil, fmt.Errorf("querying tier_models: %w", err)
	}
	defer rows.Close()
	result := map[string]map[string]string{}
	for rows.Next() {
		var tier, provider, model string
		if err := rows.Scan(&tier, &provider, &model); err != nil {
			return nil, fmt.Errorf("scanning tier_model: %w", err)
		}
		if result[tier] == nil {
			result[tier] = map[string]string{}
		}
		result[tier][provider] = model
	}
	return result, nil
}

// UpsertTierModel inserts or updates a (tier, provider) → model mapping.
// Validates modelID is non-empty.
func (s *TierStore) UpsertTierModel(tier, provider, modelID string) error {
	if tier == "" || provider == "" || modelID == "" {
		return fmt.Errorf("tier, provider, and model_id are all required")
	}
	_, err := s.db.Exec(
		`INSERT INTO tier_models (tier, provider_name, model_id) VALUES ($1, $2, $3)
		 ON CONFLICT (tier, provider_name) DO UPDATE SET model_id = EXCLUDED.model_id`,
		tier, provider, modelID)
	if err != nil {
		return fmt.Errorf("upserting tier_model %s/%s: %w", tier, provider, err)
	}
	return nil
}

// RoleOverrides returns all per-role explicit overrides, keyed by role.
func (s *TierStore) RoleOverrides() (map[string]RoleOverride, error) {
	rows, err := s.db.Query("SELECT role, provider_name, model_id FROM role_overrides")
	if err != nil {
		return nil, fmt.Errorf("querying role_overrides: %w", err)
	}
	defer rows.Close()
	result := map[string]RoleOverride{}
	for rows.Next() {
		var ro RoleOverride
		if err := rows.Scan(&ro.Role, &ro.ProviderName, &ro.ModelID); err != nil {
			return nil, fmt.Errorf("scanning role_override: %w", err)
		}
		result[ro.Role] = ro
	}
	return result, nil
}

// UpsertRoleOverride sets a per-role provider+model override. If providerName
// is empty, the override is removed (reverts to tier resolution per FR-007 c).
func (s *TierStore) UpsertRoleOverride(role, providerName, modelID string) error {
	if role == "" {
		return fmt.Errorf("role is required")
	}
	if providerName == "" {
		// provider="" means remove the override (business-logic-model §2.3).
		_, err := s.db.Exec("DELETE FROM role_overrides WHERE role = $1", role)
		if err != nil {
			return fmt.Errorf("removing role_override %s: %w", role, err)
		}
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO role_overrides (role, provider_name, model_id) VALUES ($1, $2, $3)
		 ON CONFLICT (role) DO UPDATE SET provider_name = EXCLUDED.provider_name, model_id = EXCLUDED.model_id`,
		role, providerName, modelID)
	if err != nil {
		return fmt.Errorf("upserting role_override %s: %w", role, err)
	}
	return nil
}

// ResolveTier implements app-design §5.1: the active provider is the first
// ENABLED provider (alphabetical by name) that has a tier_models row for the
// given tier. Returns (provider, model) or (nil, "") if none found.
//
// This is the one non-obvious step in the resolution algorithm. A tier maps to
// a model PER provider, but dispatch needs ONE provider. The selection rule is
// deterministic: enabled providers, alphabetical by name, first with a tier row.
func (s *TierStore) ResolveTier(tier string, providers []ProviderConfig) (*ProviderConfig, string) {
	if tier == "" {
		return nil, ""
	}
	// Load the tier→provider→model map once.
	tierModels, err := s.TierModels()
	if err != nil {
		return nil, ""
	}
	providerToModel, ok := tierModels[tier]
	if !ok {
		return nil, ""
	}
	// Sort enabled providers alphabetically (deterministic).
	var enabled []ProviderConfig
	for _, p := range providers {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}
	sort.Slice(enabled, func(i, j int) bool { return enabled[i].Name < enabled[j].Name })
	for _, p := range enabled {
		if model, ok := providerToModel[p.Name]; ok {
			return &p, model
		}
	}
	return nil, ""
}

// ModelExistsForProvider checks whether modelID is in the provider's
// provider_models list. Used by handlers to validate tier/override writes.
func (s *TierStore) ModelExistsForProvider(providerName, modelID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM provider_models WHERE provider_name = $1 AND model_id = $2",
		providerName, modelID).Scan(&count)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("checking model %s/%s: %w", providerName, modelID, err)
	}
	return count > 0, nil
}

// ProviderEnabled checks whether a provider row exists and is enabled.
func (s *TierStore) ProviderEnabled(providerName string) (bool, error) {
	var enabled int
	err := s.db.QueryRow("SELECT enabled FROM providers WHERE name = $1", providerName).Scan(&enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("checking provider %s enabled: %w", providerName, err)
	}
	return enabled == 1, nil
}

// stripDollar removes a leading "$" from an env-var reference (ADR-003).
// Tolerates bare names (backward compat): "ANTHROPIC_API_KEY" → "ANTHROPIC_API_KEY".
func stripDollar(s string) string {
	return strings.TrimPrefix(s, "$")
}