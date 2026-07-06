package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
)

// api_key_env regex: ^\$\w+$ OR empty (ADR-003). Rejects raw key strings.
var apiKeyEnvRegex = regexp.MustCompile(`^\$\w+$`)

// validAPIKeyEnv returns true if s matches ^\$\w+$ or is empty. ADR-003.
func validAPIKeyEnv(s string) bool {
	if s == "" {
		return true
	}
	return apiKeyEnvRegex.MatchString(s)
}

// ─── Providers ───

// handleGetProviders handles GET /api/config/providers.
// Returns providers with key_state derived at read time (never the raw key value).
// Traces U-API-02, NFR-SEC-01, FR-002.
func (s *Server) handleGetProviders(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"providers": []any{}})
		return
	}
	store := config.NewProviderStore(s.db)
	providers, err := store.Providers()
	if err != nil {
		log.Printf("GET /api/config/providers: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load providers")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

// ProviderRequest is the body for PUT /api/config/providers. The api_key_env
// field, if non-empty, MUST match ^\$\w+$ (ADR-003). A raw key string (e.g.
// sk-ant-...) fails the regex and is rejected with HTTP 400 (FR-002 acceptance b).
type ProviderRequest struct {
	Name            string                    `json:"name"`
	DisplayName     string                    `json:"display_name"`
	Enabled         bool                      `json:"enabled"`
	BaseURL         string                    `json:"base_url"`
	APIKeyEnv       string                    `json:"api_key_env"`
	DefaultModelID  string                    `json:"default_model_id"`
	NPMAdapter      string                    `json:"npm_adapter"`
	EnvVarSupported *bool                     `json:"env_var_supported"`
	PresetID        string                    `json:"preset_id"`
	Models          []config.ProviderModel    `json:"models"`
}

// handlePutProviders handles PUT /api/config/providers. Upserts the provider +
// models in one transaction. Emits RecordAuditEvent("__platform__", ...).
// Traces U-API-02, FR-002, FR-009, NFR-SEC-01, ADR-003, ADR-006, ADR-008.
func (s *Server) handlePutProviders(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "no_database", "Database not configured")
		return
	}

	var req ProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	if !validAPIKeyEnv(req.APIKeyEnv) {
		writeError(w, http.StatusBadRequest, "invalid_api_key_env",
			"api_key_env must match ^\\$\\w+$ or be empty (a $VAR reference, not a raw key)")
		return
	}
	if req.NPMAdapter == "" {
		req.NPMAdapter = "@ai-sdk/openai-compatible" // ADR-004 default
	}
	if req.PresetID == "" {
		req.PresetID = "custom"
	}
	if req.Models == nil {
		req.Models = []config.ProviderModel{}
	}
	envVarSupported := true
	if req.EnvVarSupported != nil {
		envVarSupported = *req.EnvVarSupported
	}

	p := config.ProviderConfig{
		Name:            req.Name,
		DisplayName:     req.DisplayName,
		Enabled:         req.Enabled,
		BaseURL:         req.BaseURL,
		APIKeyEnv:       req.APIKeyEnv,
		DefaultModelID:  req.DefaultModelID,
		NPMAdapter:      req.NPMAdapter,
		EnvVarSupported: envVarSupported,
		PresetID:        req.PresetID,
		Models:          req.Models,
	}

	store := config.NewProviderStore(s.db)
	if err := store.UpsertProvider(p); err != nil {
		log.Printf("PUT /api/config/providers: upsert %s: %v", req.Name, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save provider")
		return
	}

	// Audit (F-01 fix: RecordAuditEvent with __platform__ sentinel, not RecordEvent("","")).
	details := fmt.Sprintf(`{"action":"provider_upsert","name":%q,"enabled":%t,"api_key_env":%q}`,
		req.Name, req.Enabled, req.APIKeyEnv)
	if err := s.db.RecordAuditEvent("__platform__", db.EventProviderConfigMutated, "", "config", details); err != nil {
		log.Printf("PUT /api/config/providers: audit event failed: %v", err)
		// Audit failure is non-fatal (best-effort log, matches existing RecordEvent usage).
	}

	// Return the updated provider with recomputed key_state.
	updated, _ := store.Provider(req.Name)
	if updated == nil {
		updated = &p
	}
	writeJSON(w, http.StatusOK, updated)
}

// ─── Tiers ───

// handleGetTiers handles GET /api/config/tiers. Returns tier assignments + the
// server-side `resolved` mapping + stale_assignments (tiers whose resolved
// provider is disabled). Traces U-API-03, FR-010, app-design §6.3.
func (s *Server) handleGetTiers(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"tiers": []any{}, "stale_assignments": []any{}})
		return
	}
	store := config.NewProviderStore(s.db)
	tierStore := config.NewTierStore(s.db)
	providers, err := store.Providers()
	if err != nil {
		log.Printf("GET /api/config/tiers: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load providers")
		return
	}
	tierModels, err := tierStore.TierModels()
	if err != nil {
		log.Printf("GET /api/config/tiers: tier_models: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load tiers")
		return
	}

	providerByName := map[string]config.ProviderConfig{}
	for _, p := range providers {
		providerByName[p.Name] = p
	}

	type assignment struct {
		Provider         string `json:"provider"`
		ModelID          string `json:"model_id"`
		ProviderEnabled  bool   `json:"provider_enabled"`
	}
	type tierEntry struct {
		Tier         string       `json:"tier"`
		Assignments  []assignment `json:"assignments"`
		Resolved     *assignment  `json:"resolved,omitempty"`
	}

	// Build tiers (sorted by tier name for deterministic output).
	tierNames := make([]string, 0, len(tierModels))
	for tier := range tierModels {
		tierNames = append(tierNames, tier)
	}
	sort.Strings(tierNames)

	var tiers []tierEntry
	var stale []assignment
	for _, tier := range tierNames {
		entry := tierEntry{Tier: tier, Assignments: []assignment{}}
		providerToModel := tierModels[tier]
		providerNames := make([]string, 0, len(providerToModel))
		for pn := range providerToModel {
			providerNames = append(providerNames, pn)
		}
		sort.Strings(providerNames)

		var resolved *assignment
		for _, pn := range providerNames {
			p := providerByName[pn]
			a := assignment{Provider: pn, ModelID: providerToModel[pn], ProviderEnabled: p.Enabled}
			entry.Assignments = append(entry.Assignments, a)
			// Resolved = first enabled provider (alphabetical), per §5.1.
			if resolved == nil && p.Enabled {
				resolved = &assignment{Provider: pn, ModelID: providerToModel[pn], ProviderEnabled: true}
			}
		}
		entry.Resolved = resolved
		tiers = append(tiers, entry)

		// Stale: tier has assignments but none resolved (all providers disabled).
		if resolved == nil && len(entry.Assignments) > 0 {
			for _, a := range entry.Assignments {
				if !a.ProviderEnabled {
					stale = append(stale, assignment{
						Provider: a.Provider, ModelID: a.ModelID, ProviderEnabled: false,
					})
				}
			}
		}
	}
	if tiers == nil {
		tiers = []tierEntry{}
	}
	if stale == nil {
		stale = []assignment{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tiers":             tiers,
		"stale_assignments": stale,
	})
}

// TierRequest is the body for PUT /api/config/tiers.
type TierRequest struct {
	Tier       string `json:"tier"`
	Provider   string `json:"provider"`
	ModelID    string `json:"model_id"`
}

// handlePutTiers handles PUT /api/config/tiers. Upserts a (tier, provider, model)
// row. Validates the model exists in the provider's models and the provider is
// enabled. Emits RecordAuditEvent(EventTierConfigMutated). Traces U-API-03, FR-010.
func (s *Server) handlePutTiers(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "no_database", "Database not configured")
		return
	}
	var req TierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}
	if req.Tier == "" || req.Provider == "" || req.ModelID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "tier, provider, and model_id are required")
		return
	}
	tierStore := config.NewTierStore(s.db)

	// Validate provider is enabled (error prevention at the point of action, I-20).
	enabled, err := tierStore.ProviderEnabled(req.Provider)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check provider")
		return
	}
	if !enabled {
		writeError(w, http.StatusBadRequest, "provider_disabled",
			fmt.Sprintf("provider %s is not enabled (enable it before assigning a tier to it)", req.Provider))
		return
	}
	// Validate model exists in the provider's models list.
	exists, err := tierStore.ModelExistsForProvider(req.Provider, req.ModelID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check model")
		return
	}
	if !exists {
		writeError(w, http.StatusBadRequest, "unknown_model",
			fmt.Sprintf("model %s is not in provider %s's model list", req.ModelID, req.Provider))
		return
	}

	if err := tierStore.UpsertTierModel(req.Tier, req.Provider, req.ModelID); err != nil {
		log.Printf("PUT /api/config/tiers: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save tier")
		return
	}
	details := fmt.Sprintf(`{"action":"tier_upsert","tier":%q,"provider":%q,"model_id":%q}`,
		req.Tier, req.Provider, req.ModelID)
	s.db.RecordAuditEvent("__platform__", db.EventTierConfigMutated, "", "config", details)
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved", "tier": req.Tier, "provider": req.Provider, "model_id": req.ModelID})
}

// ─── Role Overrides ───

// handleGetRoleOverrides handles GET /api/config/role-overrides.
func (s *Server) handleGetRoleOverrides(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"role_overrides": []any{}})
		return
	}
	tierStore := config.NewTierStore(s.db)
	overrides, err := tierStore.RoleOverrides()
	if err != nil {
		log.Printf("GET /api/config/role-overrides: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load role overrides")
		return
	}
	// Convert map to sorted slice for deterministic output.
	roles := make([]string, 0, len(overrides))
	for r := range overrides {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	list := make([]config.RoleOverride, 0, len(overrides))
	for _, r := range roles {
		list = append(list, overrides[r])
	}
	writeJSON(w, http.StatusOK, map[string]any{"role_overrides": list})
}

// RoleOverrideRequest is the body for PUT /api/config/role-overrides.
// provider="" removes the override (reverts to tier resolution per FR-007 c).
type RoleOverrideRequest struct {
	Role       string `json:"role"`
	Provider   string `json:"provider"`
	ModelID    string `json:"model_id"`
}

// handlePutRoleOverrides handles PUT /api/config/role-overrides. Traces U-API-03, FR-010.
func (s *Server) handlePutRoleOverrides(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "no_database", "Database not configured")
		return
	}
	var req RoleOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON body")
		return
	}
	if req.Role == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "role is required")
		return
	}
	tierStore := config.NewTierStore(s.db)

	// provider="" means remove the override (FR-007 c: override removal reverts to tier).
	if req.Provider == "" {
		if err := tierStore.UpsertRoleOverride(req.Role, "", ""); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to remove override")
			return
		}
		details := fmt.Sprintf(`{"action":"role_override_remove","role":%q}`, req.Role)
		s.db.RecordAuditEvent("__platform__", db.EventTierConfigMutated, "", "config", details)
		writeJSON(w, http.StatusOK, map[string]any{"status": "removed", "role": req.Role})
		return
	}

	// Validate provider is enabled + model exists.
	enabled, err := tierStore.ProviderEnabled(req.Provider)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check provider")
		return
	}
	if !enabled {
		writeError(w, http.StatusBadRequest, "provider_disabled",
			fmt.Sprintf("provider %s is not enabled", req.Provider))
		return
	}
	if req.ModelID != "" {
		exists, err := tierStore.ModelExistsForProvider(req.Provider, req.ModelID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check model")
			return
		}
		if !exists {
			writeError(w, http.StatusBadRequest, "unknown_model",
				fmt.Sprintf("model %s is not in provider %s's model list", req.ModelID, req.Provider))
			return
		}
	}

	if err := tierStore.UpsertRoleOverride(req.Role, req.Provider, req.ModelID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save override")
		return
	}
	details := fmt.Sprintf(`{"action":"role_override_upsert","role":%q,"provider":%q,"model_id":%q}`,
		req.Role, req.Provider, req.ModelID)
	s.db.RecordAuditEvent("__platform__", db.EventTierConfigMutated, "", "config", details)
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved", "role": req.Role, "provider": req.Provider, "model_id": req.ModelID})
}