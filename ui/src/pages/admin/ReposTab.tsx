import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listRepos, createRepoAdmin, updateRepoAdmin, deleteRepoAdmin } from '../../api/client';
import type { AvailableRepo, RepoInput } from '../../types/admin';
import { Button, Badge, Card } from '../../ui/primitives';
import { Modal } from '../../ui/primitives';
import { useToast } from '../../components/Toast';

const inputClass =
  'w-full px-3 py-1.5 text-sm rounded-[var(--radius-md)] bg-[var(--color-surface)] text-[var(--color-text-primary)] border border-[var(--color-border-subtle)] focus:border-[var(--color-accent)] focus:outline-none transition-colors';
const labelClass = 'block text-xs font-medium text-[var(--color-text-secondary)] mb-1';

type EditingState = { mode: 'create' } | { mode: 'edit'; repo: AvailableRepo } | null;

export default function ReposTab() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [editing, setEditing] = useState<EditingState>(null);
  const [deleteTarget, setDeleteTarget] = useState<AvailableRepo | null>(null);
  const [form, setForm] = useState<RepoInput>({ name: '', url: '', branch: 'main', description: '', primary: false });

  const { data: repos = [] } = useQuery<AvailableRepo[]>({
    queryKey: ['repos-admin'],
    queryFn: listRepos,
  });

  const createMutation = useMutation({
    mutationFn: createRepoAdmin,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['repos-admin'] });
      queryClient.invalidateQueries({ queryKey: ['repos'] });
      addToast('success', 'Repo created');
      setEditing(null);
    },
    onError: (err: Error) => addToast('error', err.message),
  });

  const updateMutation = useMutation({
    mutationFn: ({ name, input }: { name: string; input: RepoInput }) => updateRepoAdmin(name, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['repos-admin'] });
      queryClient.invalidateQueries({ queryKey: ['repos'] });
      addToast('success', 'Repo updated');
      setEditing(null);
    },
    onError: (err: Error) => addToast('error', err.message),
  });

  const deleteMutation = useMutation({
    mutationFn: deleteRepoAdmin,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['repos-admin'] });
      queryClient.invalidateQueries({ queryKey: ['repos'] });
      addToast('success', 'Repo deleted');
      setDeleteTarget(null);
    },
    onError: (err: Error) => addToast('error', err.message),
  });

  const openCreate = () => {
    setForm({ name: '', url: '', branch: 'main', description: '', primary: false });
    setEditing({ mode: 'create' });
  };

  const openEdit = (repo: AvailableRepo) => {
    setForm({ name: repo.name, url: repo.url, branch: 'main', description: repo.description, primary: repo.primary });
    setEditing({ mode: 'edit', repo });
  };

  const submitForm = () => {
    if (!form.name.trim() || !form.url.trim()) {
      addToast('error', 'Name and URL are required');
      return;
    }
    if (editing?.mode === 'create') {
      createMutation.mutate(form);
    } else if (editing?.mode === 'edit') {
      updateMutation.mutate({ name: editing.repo.name, input: form });
    }
  };

  return (
    <div data-testid="repos-tab">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-medium text-[var(--color-text-secondary)]">Repository Registry ({repos.length})</h3>
        <Button variant="primary" size="sm" onClick={openCreate} data-testid="repos-add-button">Add Repo</Button>
      </div>

      {repos.length === 0 ? (
        <Card className="p-6 text-center">
          <p className="text-sm text-[var(--color-text-tertiary)]" data-testid="repos-empty">No repos registered. Click "Add Repo" to create one.</p>
        </Card>
      ) : (
        <div className="space-y-2" data-testid="repos-list">
          {repos.map((repo) => (
            <Card key={repo.name} className="p-3 flex items-center justify-between">
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-[var(--color-text-primary)]">{repo.name}</span>
                  {repo.primary && <Badge color="blue">primary</Badge>}
                </div>
                <div className="text-xs text-[var(--color-text-tertiary)] mt-0.5">{repo.url}</div>
                {repo.description && <div className="text-xs text-[var(--color-text-secondary)] mt-0.5">{repo.description}</div>}
              </div>
              <div className="flex gap-1">
                <Button variant="secondary" size="sm" onClick={() => openEdit(repo)} data-testid={`repos-edit-${repo.name}`}>Edit</Button>
                <Button variant="danger" size="sm" onClick={() => setDeleteTarget(repo)} data-testid={`repos-delete-${repo.name}`}>Delete</Button>
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* Add/Edit modal */}
      <Modal open={editing !== null} onClose={() => setEditing(null)} title={editing?.mode === 'create' ? 'Add Repo' : `Edit ${editing?.repo.name ?? ''}`} data-testid="repos-modal">
        <div className="space-y-3">
          <div>
            <label className={labelClass}>Name</label>
            <input
              type="text"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              disabled={editing?.mode === 'edit'}
              className={inputClass}
              data-testid="repos-form-name"
            />
          </div>
          <div>
            <label className={labelClass}>URL</label>
            <input
              type="text"
              value={form.url}
              onChange={(e) => setForm({ ...form, url: e.target.value })}
              className={inputClass}
              data-testid="repos-form-url"
            />
          </div>
          <div>
            <label className={labelClass}>Description</label>
            <input
              type="text"
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              className={inputClass}
              data-testid="repos-form-description"
            />
          </div>
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={form.primary}
              onChange={(e) => setForm({ ...form, primary: e.target.checked })}
              id="repos-form-primary"
              data-testid="repos-form-primary"
            />
            <label htmlFor="repos-form-primary" className="text-sm text-[var(--color-text-secondary)]">Primary repo</label>
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" size="sm" onClick={() => setEditing(null)} data-testid="repos-form-cancel">Cancel</Button>
            <Button variant="primary" size="sm" onClick={submitForm} data-testid="repos-form-save">Save</Button>
          </div>
        </div>
      </Modal>

      {/* Delete confirm modal */}
      <Modal open={deleteTarget !== null} onClose={() => setDeleteTarget(null)} title="Confirm Delete" data-testid="repos-delete-modal">
        <p className="text-sm text-[var(--color-text-secondary)] mb-4">
          Are you sure you want to delete <strong>{deleteTarget?.name}</strong>? This cannot be undone.
        </p>
        <div className="flex justify-end gap-2">
          <Button variant="secondary" size="sm" onClick={() => setDeleteTarget(null)} data-testid="repos-delete-cancel">Cancel</Button>
          <Button variant="danger" size="sm" onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.name)} data-testid="repos-delete-confirm">Delete</Button>
        </div>
      </Modal>
    </div>
  );
}