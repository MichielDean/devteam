import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getConfigProviders,
  putConfigProvider,
  getConfigTiers,
  putConfigTier,
  getConfigRoleOverrides,
  putConfigRoleOverride,
  ApiError,
} from '../api/client';
import { useToast } from '../components/Toast';
import type {
  ProviderConfigDTO,
  ProviderRequest,
  TierRequest,
  RoleOverrideRequest,
} from '../types';
import { PRESETS, AGENT_LABELS, AGENT_TIERS } from '../types';

// EffectTimingBanner — communicates that config changes take effect at the next
// agent dispatch (NFR-OP-01). Sits above both sections (D6-6, M1/M2).
function EffectTimingBanner() {
  return (
    <div
      data-testid="effect-timing-banner"
      className="mb-6 p-3 rounded-[var(--radius-md)] text-sm"
      style={{
        backgroundColor: 'var(--color-surface-active)',
        color: 'var(--color-text-secondary)',
        border: '1px solid var(--color-border-subtle)',
      }}
    >
      Config changes take effect at the next agent dispatch. Running sessions keep their config.
    </div>
  );
}

// KeyStateDisplay renders the provider's key state ("set" / "not set" / "not required").
// Never shows the raw key value (NFR-SEC-01, R-09). Traces I-08/I-09.
function KeyStateDisplay({ provider }: { provider: ProviderConfigDTO }) {
  const label =
    provider.key_state === 'set'
      ? `Key: set (${provider.api_key_env})`
      : provider.key_state === 'not_set'
        ? `Key: not set (${provider.api_key_env})`
        : 'Key: not required (local/keyless)';
  const color =
    provider.key_state === 'set'
      ? 'var(--color-success, green)'
      : provider.key_state === 'not_set'
        ? 'var(--color-warning, orange)'
        : 'var(--color-text-tertiary)';
  return (
    <span data-testid={`key-state-${provider.name}`} style={{ color }} className="text-xs">
      {label}
    </span>
  );
}

// ProviderCard renders one provider. Copilot (env_var_supported=false) shows the
// reduced surface: no Edit button, "View setup instructions" link, harness-env copy
// (OD-1/OD-3, ADR-005). Driven by the env_var_supported flag — no `if provider ===
// "copilot"` branch in UI code (NFR-INTEG-01).
function ProviderCard({
  provider,
  onEdit,
  onToggle,
}: {
  provider: ProviderConfigDTO;
  onEdit: () => void;
  onToggle: (enabled: boolean) => void;
}) {
  return (
    <div
      data-testid={`provider-card-${provider.name}`}
      className="p-4 rounded-[var(--radius-md)]"
      style={{
        backgroundColor: 'var(--color-surface-raised)',
        border: '1px solid var(--color-border-subtle)',
        boxShadow: 'var(--shadow-sm)',
      }}
    >
      <div className="flex items-start justify-between mb-3">
        <div>
          <div className="font-medium text-[var(--color-text-primary)]">{provider.display_name}</div>
          <div className="text-xs text-[var(--color-text-tertiary)]">{provider.name}</div>
        </div>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={provider.enabled}
            onChange={(e) => onToggle(e.target.checked)}
            data-testid={`toggle-${provider.name}`}
          />
          <span className="text-[var(--color-text-secondary)]">
            {provider.enabled ? 'Enabled' : 'Disabled'}
          </span>
        </label>
      </div>
      <div className="space-y-1 mb-3">
        <KeyStateDisplay provider={provider} />
        {provider.default_model_id && (
          <div className="text-xs">
            <span className="text-[var(--color-text-tertiary)]">Default: </span>
            <span className="px-1.5 py-0.5 rounded text-[var(--color-text-secondary)]" style={{ backgroundColor: 'var(--color-surface-active)' }}>
              {provider.default_model_id}
            </span>
          </div>
        )}
        {provider.models.length > 0 && (
          <div className="text-xs text-[var(--color-text-tertiary)]">
            Models: {provider.models.map((m) => m.model_id).join(', ')}
          </div>
        )}
      </div>
      {provider.env_var_supported ? (
        <button
          onClick={onEdit}
          data-testid={`edit-${provider.name}`}
          className="text-sm text-[var(--color-accent)] hover:underline"
        >
          Edit
        </button>
      ) : (
        <div className="text-xs text-[var(--color-text-tertiary)]">
          Auth: configure via harness env
          <a
            href="https://opencode.ai/docs/providers/copilot"
            target="_blank"
            rel="noreferrer"
            className="ml-1 text-[var(--color-accent)] hover:underline"
          >
            View setup instructions
          </a>
        </div>
      )}
    </div>
  );
}

// validateAPIKeyEnv enforces ^\$\w+$ or empty (ADR-003). Returns the error message
// or empty string if valid. Traces FR-002 acceptance b.
function validateAPIKeyEnv(value: string): string {
  if (value === '') return '';
  if (!/^\$\w+$/.test(value)) {
    return 'Must be a $VAR reference (e.g. $ANTHROPIC_API_KEY) or empty';
  }
  return '';
}

// EditProviderDrawer — edits a provider's fields. Key section shows KeyStateDisplay
// with a "Set reference" affordance. Client validation on blur (ADR-003). Traces M4, I-05…I-13.
function EditProviderDrawer({
  provider,
  onClose,
  onSave,
}: {
  provider: ProviderConfigDTO | null;
  onClose: () => void;
  onSave: (req: ProviderRequest) => void;
}) {
  const [displayName, setDisplayName] = useState(provider?.display_name ?? '');
  const [baseURL, setBaseURL] = useState(provider?.base_url ?? '');
  const [apiKeyEnv, setApiKeyEnv] = useState(provider?.api_key_env ?? '');
  const [defaultModel, setDefaultModel] = useState(provider?.default_model_id ?? '');
  const [enabled, setEnabled] = useState(provider?.enabled ?? false);
  const [keyError, setKeyError] = useState('');
  const [showKeyInput, setShowKeyInput] = useState(false);

  if (!provider) return null;

  const handleSave = () => {
    const err = validateAPIKeyEnv(apiKeyEnv);
    if (err) {
      setKeyError(err);
      return;
    }
    onSave({
      name: provider.name,
      display_name: displayName,
      enabled,
      base_url: baseURL,
      api_key_env: apiKeyEnv,
      default_model_id: defaultModel,
      npm_adapter: provider.npm_adapter || '@ai-sdk/openai-compatible',
      env_var_supported: provider.env_var_supported,
      preset_id: provider.preset_id || 'custom',
      models: provider.models,
    });
  };

  return (
    <div
      data-testid="edit-provider-drawer"
      className="fixed inset-y-0 right-0 w-full max-w-md p-6 overflow-y-auto"
      style={{
        backgroundColor: 'var(--color-surface-raised)',
        boxShadow: 'var(--shadow-lg)',
        borderLeft: '1px solid var(--color-border-subtle)',
      }}
    >
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-medium text-[var(--color-text-primary)]">Edit {provider.display_name}</h3>
        <button onClick={onClose} className="text-[var(--color-text-tertiary)] hover:text-[var(--color-text-primary)]">
          ✕
        </button>
      </div>
      <div className="space-y-4">
        <label className="block">
          <span className="text-sm text-[var(--color-text-secondary)]">Display name</span>
          <input
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            className="mt-1 block w-full px-3 py-2 rounded border"
            style={{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border-subtle)' }}
          />
        </label>
        <label className="block">
          <span className="text-sm text-[var(--color-text-secondary)]">Base URL</span>
          <input
            type="text"
            value={baseURL}
            onChange={(e) => setBaseURL(e.target.value)}
            placeholder="https://api.example.com/v1"
            className="mt-1 block w-full px-3 py-2 rounded border"
            style={{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border-subtle)' }}
          />
        </label>
        <div className="block">
          <span className="text-sm text-[var(--color-text-secondary)]">API key reference</span>
          <div className="mt-1">
            <KeyStateDisplay provider={provider} />
          </div>
          {!showKeyInput ? (
            <button
              onClick={() => setShowKeyInput(true)}
              className="mt-2 text-sm text-[var(--color-accent)] hover:underline"
            >
              {provider.api_key_env ? 'Replace reference' : 'Set reference'}
            </button>
          ) : (
            <input
              type="text"
              value={apiKeyEnv}
              onChange={(e) => setApiKeyEnv(e.target.value)}
              onBlur={() => setKeyError(validateAPIKeyEnv(apiKeyEnv))}
              placeholder="$ANTHROPIC_API_KEY"
              className="mt-2 block w-full px-3 py-2 rounded border"
              style={{ backgroundColor: 'var(--color-surface)', borderColor: keyError ? 'red' : 'var(--color-border-subtle)' }}
              data-testid="api-key-env-input"
            />
          )}
          {keyError && <div className="text-xs text-red-500 mt-1">{keyError}</div>}
        </div>
        <label className="block">
          <span className="text-sm text-[var(--color-text-secondary)]">Default model</span>
          <input
            type="text"
            value={defaultModel}
            onChange={(e) => setDefaultModel(e.target.value)}
            className="mt-1 block w-full px-3 py-2 rounded border"
            style={{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border-subtle)' }}
          />
        </label>
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          <span className="text-sm text-[var(--color-text-secondary)]">Enabled</span>
        </label>
        <div className="flex gap-2 pt-4">
          <button
            onClick={handleSave}
            data-testid="save-provider"
            className="px-4 py-2 rounded text-sm font-medium text-white"
            style={{ backgroundColor: 'var(--color-accent)' }}
          >
            Save
          </button>
          <button
            onClick={onClose}
            className="px-4 py-2 rounded text-sm"
            style={{ backgroundColor: 'var(--color-surface-active)', color: 'var(--color-text-secondary)' }}
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

// AddProviderModal — preset dropdown pre-fills the form. Traces M3, I-02/I-03/I-04.
function AddProviderModal({
  onClose,
  onAdd,
}: {
  onClose: () => void;
  onAdd: (req: ProviderRequest) => void;
}) {
  const [presetId, setPresetId] = useState('anthropic');
  const preset = PRESETS.find((p) => p.id === presetId) ?? PRESETS[0];

  const handleAdd = () => {
    const envVarSupported = !('env_var_supported' in preset) || (preset as any).env_var_supported !== false;
    onAdd({
      name: presetId === 'custom' ? 'custom-provider' : presetId,
      display_name: preset.label,
      enabled: presetId !== 'copilot', // copilot disabled by default (fast-follow)
      base_url: preset.base_url,
      api_key_env: preset.api_key_env,
      default_model_id: preset.default_model_id,
      npm_adapter: '@ai-sdk/openai-compatible',
      env_var_supported: envVarSupported,
      preset_id: presetId,
      models: preset.default_model_id ? [{ model_id: preset.default_model_id, friendly_name: preset.default_model_id }] : [],
    });
  };

  return (
    <div
      data-testid="add-provider-modal"
      className="fixed inset-0 flex items-center justify-center z-40"
      style={{ backgroundColor: 'rgba(0,0,0,0.5)' }}
      onClick={onClose}
    >
      <div
        className="w-full max-w-md p-6 rounded-[var(--radius-lg)]"
        style={{ backgroundColor: 'var(--color-surface-raised)' }}
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="text-lg font-medium text-[var(--color-text-primary)] mb-4">Add Provider</h3>
        <label className="block mb-4">
          <span className="text-sm text-[var(--color-text-secondary)]">Preset</span>
          <select
            value={presetId}
            onChange={(e) => setPresetId(e.target.value)}
            className="mt-1 block w-full px-3 py-2 rounded border"
            style={{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border-subtle)' }}
          >
            {PRESETS.map((p) => (
              <option key={p.id} value={p.id}>
                {p.label}
              </option>
            ))}
          </select>
        </label>
        <div className="text-xs text-[var(--color-text-tertiary)] mb-4">
          {presetId === 'custom'
            ? 'Custom provider — fill in all fields in the edit drawer.'
            : `Pre-fills: ${preset.base_url || '(no base URL)'} · ${preset.api_key_env || '(no key required)'}`}
        </div>
        <div className="flex gap-2">
          <button
            onClick={handleAdd}
            data-testid="add-provider-confirm"
            className="px-4 py-2 rounded text-sm font-medium text-white"
            style={{ backgroundColor: 'var(--color-accent)' }}
          >
            Add
          </button>
          <button
            onClick={onClose}
            className="px-4 py-2 rounded text-sm"
            style={{ backgroundColor: 'var(--color-surface-active)', color: 'var(--color-text-secondary)' }}
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

// TierMatrix — one row per tier; shows provider, resolved model, "Change" affordance.
// Section disabled until ≥1 provider enabled. Traces M2 tier section, I-19.
function TierMatrix({
  tiers,
  providers,
  onChangeTier,
}: {
  tiers: import('../types').TierEntry[];
  providers: ProviderConfigDTO[];
  onChangeTier: (req: TierRequest) => void;
}) {
  const enabledProviders = providers.filter((p) => p.enabled);
  const anyEnabled = enabledProviders.length > 0;

  return (
    <div data-testid="tier-matrix" className={!anyEnabled ? 'opacity-50 pointer-events-none' : ''}>
      <div className="text-xs text-[var(--color-text-tertiary)] mb-3">
        Tiers come from the agent roster. Unassigned tiers fall back to the provider's default model.
      </div>
      {tiers.length === 0 ? (
        <div className="text-sm text-[var(--color-text-tertiary)]">No tier assignments yet.</div>
      ) : (
        <div className="space-y-2">
          {tiers.map((tier) => (
            <div
              key={tier.tier}
              data-testid={`tier-row-${tier.tier}`}
              className="flex items-center justify-between p-3 rounded"
              style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border-subtle)' }}
            >
              <div>
                <div className="font-medium text-[var(--color-text-primary)] capitalize">{tier.tier}</div>
                {tier.resolved ? (
                  <div className="text-xs text-[var(--color-text-secondary)]">
                    → {tier.resolved.provider}/{tier.resolved.model_id}
                  </div>
                ) : (
                  <div className="text-xs text-[var(--color-warning, orange)]">
                    No enabled provider for this tier
                  </div>
                )}
              </div>
              {anyEnabled && (
                <select
                  className="px-2 py-1 text-sm rounded border"
                  style={{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border-subtle)' }}
                  value={tier.resolved?.provider ?? ''}
                  onChange={(e) => {
                    const p = enabledProviders.find((p) => p.name === e.target.value);
                    if (p && p.models[0]) {
                      onChangeTier({ tier: tier.tier, provider: p.name, model_id: p.models[0].model_id });
                    }
                  }}
                  data-testid={`tier-select-${tier.tier}`}
                >
                  <option value="">— select —</option>
                  {enabledProviders.map((p) => (
                    <option key={p.name} value={p.name}>
                      {p.display_name} ({p.default_model_id})
                    </option>
                  ))}
                </select>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// StaleAssignmentAlert — renders for each stale_assignments entry. Traces M6, I-25/I-26.
function StaleAssignmentAlert({
  stale,
  onReenable,
}: {
  stale: import('../types').StaleAssignment[];
  onReenable: (providerName: string) => void;
}) {
  if (stale.length === 0) return null;
  return (
    <div data-testid="stale-assignment-alerts" className="mb-4 space-y-2">
      {stale.map((s, i) => (
        <div
          key={i}
          className="p-3 rounded text-sm"
          style={{
            backgroundColor: 'var(--color-surface-active)',
            border: '1px solid var(--color-warning, orange)',
            color: 'var(--color-text-secondary)',
          }}
        >
          Tier assignment to <strong>{s.provider}</strong> is stale (provider disabled).
          <button
            onClick={() => onReenable(s.provider)}
            className="ml-2 text-[var(--color-accent)] hover:underline"
          >
            Re-enable {s.provider}
          </button>
        </div>
      ))}
    </div>
  );
}

// RoleOverridesEditor — per-role explicit provider+model override. Traces I-19…I-26.
function RoleOverridesEditor({
  providers,
  overrides,
  onSetOverride,
  onRemoveOverride,
}: {
  providers: ProviderConfigDTO[];
  overrides: import('../types').RoleOverrideDTO[];
  onSetOverride: (req: RoleOverrideRequest) => void;
  onRemoveOverride: (role: string) => void;
}) {
  const enabledProviders = providers.filter((p) => p.enabled);
  const [selectedRole, setSelectedRole] = useState('');
  const [selectedProvider, setSelectedProvider] = useState('');
  const [selectedModel, setSelectedModel] = useState('');

  return (
    <div data-testid="role-overrides-editor" className="mt-6">
      <h4 className="text-sm font-medium text-[var(--color-text-primary)] mb-2">Per-role overrides</h4>
      <div className="text-xs text-[var(--color-text-tertiary)] mb-3">
        An override wins over the tier default. Removing the override reverts to tier resolution.
      </div>
      {overrides.length > 0 && (
        <div className="space-y-1 mb-3">
          {overrides.map((ro) => (
            <div
              key={ro.role}
              data-testid={`override-${ro.role}`}
              className="flex items-center justify-between text-sm p-2 rounded"
              style={{ backgroundColor: 'var(--color-surface)' }}
            >
              <span>
                <strong>{AGENT_LABELS[ro.role] ?? ro.role}</strong> → {ro.provider}/{ro.model_id}
              </span>
              <button
                onClick={() => onRemoveOverride(ro.role)}
                className="text-xs text-[var(--color-text-tertiary)] hover:text-red-500"
                data-testid={`remove-override-${ro.role}`}
              >
                remove
              </button>
            </div>
          ))}
        </div>
      )}
      <div className="flex gap-2 items-end">
        <label className="block">
          <span className="text-xs text-[var(--color-text-tertiary)]">Role</span>
          <select
            value={selectedRole}
            onChange={(e) => setSelectedRole(e.target.value)}
            className="block px-2 py-1 text-sm rounded border"
            style={{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border-subtle)' }}
          >
            <option value="">— role —</option>
            {Object.keys(AGENT_LABELS).map((r) => (
              <option key={r} value={r}>
                {AGENT_LABELS[r]} ({AGENT_TIERS[r] ?? '—'})
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="text-xs text-[var(--color-text-tertiary)]">Provider</span>
          <select
            value={selectedProvider}
            onChange={(e) => {
              setSelectedProvider(e.target.value);
              const p = enabledProviders.find((p) => p.name === e.target.value);
              setSelectedModel(p?.models[0]?.model_id ?? '');
            }}
            className="block px-2 py-1 text-sm rounded border"
            style={{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border-subtle)' }}
          >
            <option value="">— provider —</option>
            {enabledProviders.map((p) => (
              <option key={p.name} value={p.name}>
                {p.display_name}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="text-xs text-[var(--color-text-tertiary)]">Model</span>
          <select
            value={selectedModel}
            onChange={(e) => setSelectedModel(e.target.value)}
            className="block px-2 py-1 text-sm rounded border"
            style={{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border-subtle)' }}
          >
            <option value="">— model —</option>
            {enabledProviders
              .find((p) => p.name === selectedProvider)
              ?.models.map((m) => (
                <option key={m.model_id} value={m.model_id}>
                  {m.model_id}
                </option>
              ))}
          </select>
        </label>
        <button
          onClick={() => {
            if (selectedRole && selectedProvider && selectedModel) {
              onSetOverride({ role: selectedRole, provider: selectedProvider, model_id: selectedModel });
              setSelectedRole('');
              setSelectedProvider('');
              setSelectedModel('');
            }
          }}
          disabled={!selectedRole || !selectedProvider || !selectedModel}
          className="px-3 py-1 text-sm rounded text-white disabled:opacity-50"
          style={{ backgroundColor: 'var(--color-accent)' }}
          data-testid="set-override"
        >
          Set
        </button>
      </div>
    </div>
  );
}

// AdminProvidersPage — the two-persona admin page (Operator + Team Lead).
// Traces U-UI-02…05, H5 (split responsibility), D6-1.
export default function AdminProvidersPage() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [editingProvider, setEditingProvider] = useState<ProviderConfigDTO | null>(null);
  const [showAddModal, setShowAddModal] = useState(false);

  const providersQuery = useQuery({
    queryKey: ['config-providers'],
    queryFn: getConfigProviders,
  });
  const tiersQuery = useQuery({
    queryKey: ['config-tiers'],
    queryFn: getConfigTiers,
  });
  const overridesQuery = useQuery({
    queryKey: ['config-role-overrides'],
    queryFn: getConfigRoleOverrides,
  });

  const invalidateConfig = () => {
    queryClient.invalidateQueries({ queryKey: ['config-providers'] });
    queryClient.invalidateQueries({ queryKey: ['config-tiers'] }); // resolved + stale recompute
    queryClient.invalidateQueries({ queryKey: ['config-role-overrides'] });
  };

  const providerMutation = useMutation({
    mutationFn: putConfigProvider,
    onSuccess: () => {
      invalidateConfig();
      addToast('success', 'Provider saved');
      setEditingProvider(null);
      setShowAddModal(false);
    },
    onError: (err: Error) => {
      addToast('error', err instanceof ApiError ? err.details ?? err.code : err.message);
    },
  });
  const tierMutation = useMutation({
    mutationFn: putConfigTier,
    onSuccess: () => {
      invalidateConfig();
      addToast('success', 'Tier updated');
    },
    onError: (err: Error) => addToast('error', err.message),
  });
  const overrideMutation = useMutation({
    mutationFn: putConfigRoleOverride,
    onSuccess: () => {
      invalidateConfig();
      addToast('success', 'Override saved');
    },
    onError: (err: Error) => addToast('error', err.message),
  });

  const providers = providersQuery.data?.providers ?? [];
  const tiers = tiersQuery.data?.tiers ?? [];
  const stale = tiersQuery.data?.stale_assignments ?? [];
  const overrides = overridesQuery.data?.role_overrides ?? [];

  return (
    <div data-testid="admin-providers-page">
      <h2 className="text-xl font-medium text-[var(--color-text-primary)] mb-4">Provider Configuration</h2>
      <EffectTimingBanner />

      {providersQuery.error && (
        <div className="text-red-500 mb-4">Failed to load providers: {(providersQuery.error as Error).message}</div>
      )}

      {/* Operator section — Providers & API Keys */}
      <section data-testid="operator-section" className="mb-8">
        <h3 className="text-lg font-medium text-[var(--color-text-primary)] mb-3">Providers & API Keys</h3>
        <div className="flex items-center justify-between mb-3">
          <span className="text-sm text-[var(--color-text-tertiary)]">Manage LLM providers and their API key references.</span>
          <button
            onClick={() => setShowAddModal(true)}
            data-testid="add-provider-button"
            className="px-3 py-1.5 text-sm rounded text-white"
            style={{ backgroundColor: 'var(--color-accent)' }}
          >
            + Add Provider
          </button>
        </div>
        {providersQuery.isLoading ? (
          <div className="text-[var(--color-text-tertiary)]">Loading providers…</div>
        ) : providers.length === 0 ? (
          <div className="text-[var(--color-text-tertiary)] p-4 rounded" style={{ backgroundColor: 'var(--color-surface)' }}>
            No providers configured. Click "+ Add Provider" to get started.
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {providers.map((p) => (
              <ProviderCard
                key={p.name}
                provider={p}
                onEdit={() => setEditingProvider(p)}
                onToggle={(enabled) =>
                  providerMutation.mutate({
                    name: p.name,
                    display_name: p.display_name,
                    enabled,
                    base_url: p.base_url,
                    api_key_env: p.api_key_env,
                    default_model_id: p.default_model_id,
                    npm_adapter: p.npm_adapter,
                    env_var_supported: p.env_var_supported,
                    preset_id: p.preset_id,
                    models: p.models,
                  })
                }
              />
            ))}
          </div>
        )}
      </section>

      {/* Team Lead section — Tier → Model Assignment */}
      <section data-testid="team-lead-section">
        <h3 className="text-lg font-medium text-[var(--color-text-primary)] mb-3">Tier → Model Assignment</h3>
        <StaleAssignmentAlert
          stale={stale}
          onReenable={(name) => {
            const p = providers.find((p) => p.name === name);
            if (p) {
              providerMutation.mutate({
                name: p.name,
                display_name: p.display_name,
                enabled: true,
                base_url: p.base_url,
                api_key_env: p.api_key_env,
                default_model_id: p.default_model_id,
                npm_adapter: p.npm_adapter,
                env_var_supported: p.env_var_supported,
                preset_id: p.preset_id,
                models: p.models,
              });
            }
          }}
        />
        <TierMatrix tiers={tiers} providers={providers} onChangeTier={(req) => tierMutation.mutate(req)} />
        <RoleOverridesEditor
          providers={providers}
          overrides={overrides}
          onSetOverride={(req) => overrideMutation.mutate(req)}
          onRemoveOverride={(role) => overrideMutation.mutate({ role, provider: '', model_id: '' })}
        />
      </section>

      {showAddModal && (
        <AddProviderModal
          onClose={() => setShowAddModal(false)}
          onAdd={(req) => providerMutation.mutate(req)}
        />
      )}
      {editingProvider && (
        <EditProviderDrawer
          provider={editingProvider}
          onClose={() => setEditingProvider(null)}
          onSave={(req) => providerMutation.mutate(req)}
        />
      )}
    </div>
  );
}