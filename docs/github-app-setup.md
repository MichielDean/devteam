# GitHub App Setup Runbook

**Feature**: github-authorization-integration
**Stage**: U-12 / FR-RUNBOOK-01/02
**Audience**: operator (P-1) + onboarding operator (P-2)

This is the **sole setup mechanism** for the GitHub App authorization feature (NFR-OPS-02, C-13). No code path automates App creation + org install + key placement. Follow it linearly; each section ends with a checkable outcome.

---

## §1 Prerequisites

Confirm each before starting:

- [ ] GitHub account with admin access to the `MichielDean` org.
- [ ] The `devteam` binary built and on PATH (or invokable as `./devteam`).
- [ ] Postgres running on `localhost:5432` with the `devteam` DB (existing — reused, not added by this feature).
- [ ] A text editor for `devteam.yaml` and `devteam.env`.

**Checkable outcome**: `devteam version` prints `devteam 0.4.0`; `psql -h localhost -U devteam -d devteam -c "SELECT 1"` returns `1`.

Continue to §2.

---

## §2 App manifest creation

Create the GitHub App in the org:

1. Go to **GitHub → Settings → Developer settings → GitHub Apps → New GitHub App** (under the `MichielDean` org, not your personal account).
2. **GitHub App name**: `devteam` (or `devteam-prod` if you want to distinguish environments).
3. **Homepage URL**: `https://github.com/MichielDean/devteam`.
4. **Webhook**: **Disable webhooks** — uncheck "Active" (this feature does NOT add a webhook receiver; C-08, scope §5).
5. **Repository permissions**:
   - **Contents**: Read-only (for branch/SHA resolution, FR-PR-01).
   - **Metadata**: Read-only (always required).
   - **Pull requests**: Read & write (for CreatePR/ReadyPR/CommentPR, FR-PR-02..04).
   - **Administration**: Read-only (for repo discovery).
   - Do NOT grant wildcard `repo/*` — least privilege (scope §3, infra-specs §2.2).
6. **Account permissions**: none (the App acts as a machine identity, not a user).
7. **Create GitHub App**.
8. **Note the App ID**: on the App's general settings page, find "App ID" (an integer like `123456`). You'll put it in `devteam.yaml` as `github.app_id`.
9. **Generate a private key**: scroll down to "Private keys" → **Generate a private key**. A `.pem` file downloads. This is the App private key (NOT a PAT, NOT the master key).

**Checkable outcome**: you have an App ID (integer) and a downloaded `.pem` file.

Continue to §3.

---

## §3 Master key + App key placement

The feature uses two distinct keys — do not confuse them:

| Key | Purpose | Where it lives | Env var |
|-----|---------|----------------|---------|
| **Master key** (32 bytes raw) | Encrypts all credentials at rest (AES-256-GCM) | `/etc/devteam/master.key` | `DEVTEAM_MASTER_KEY_FILE` |
| **App private key** (PEM) | Signs JWTs to authenticate as the GitHub App | `/etc/devteam/github-app.private-key.pem` | `github.private_key_path` in `devteam.yaml` |

The master key NEVER goes in `devteam.yaml` or the DB (C-04, FR-CRED-01). The App key NEVER goes in the DB plaintext (it's encrypted with the master key).

### 3.1 Create the custody directory

```sh
mkdir -p /etc/devteam
chmod 0700 /etc/devteam
```

**Checkable outcome**: `ls -ld /etc/devteam` shows `drwx------` owned by you.

### 3.2 Generate the master key

```sh
head -c 32 /dev/urandom > /etc/devteam/master.key
chmod 0400 /etc/devteam/master.key
```

The file must be **exactly 32 bytes**. A trailing newline (33 bytes) is rejected at load with `ErrKeyFileInvalid` — if you accidentally add one, `truncate -s 32 /etc/devteam/master.key`.

**Checkable outcome**: `wc -c /etc/devteam/master.key` prints `32`.

### 3.3 Place the App private key

Copy the `.pem` file you downloaded in §2 to the custody directory:

```sh
cp ~/Downloads/devteam-private-key-*.pem /etc/devteam/github-app.private-key.pem
chmod 0400 /etc/devteam/github-app.private-key.pem
```

**Checkable outcome**: `ls -l /etc/devteam/github-app.private-key.pem` shows `-r--------` and the file contains `-----BEGIN RSA PRIVATE KEY-----`.

### 3.4 Set the master key env var

Add to `devteam.env` (the file the systemd unit reads via `EnvironmentFile=`):

```sh
DEVTEAM_MASTER_KEY_FILE=/etc/devteam/master.key
```

**No `daemon-reload` is required** — the unit file itself is unchanged; only the env file gains a line, and `EnvironmentFile=` is re-read at every restart (infra-specs §6.1, finding 3).

**Checkable outcome**: `grep DEVTEAM_MASTER_KEY_FILE devteam.env` prints the line.

Continue to §4.

---

## §4 App installation on the `MichielDean` org

Install the App on the org so it can access repos:

1. Go to **GitHub → Settings → Developer settings → GitHub Apps → [your App] → Install App**.
2. Choose **`MichielDean`** (the org).
3. **Repository access**: select **Only select repositories** and pick the repos you want devteam to manage (least privilege — do NOT grant all repos unless you mean it).
4. **Install**.
5. **Note the installation ID**: after install, the URL will be `https://github.com/organizations/MichielDean/settings/installations/{INSTALLATION_ID}`. The number in the URL is the installation ID. Alternatively, `curl -H "Authorization: Bearer <JWT>" https://api.github.com/app/installations` lists installations with their IDs.

**Checkable outcome**: you have an installation ID (integer).

Continue to §5.

---

## §5 `devteam.yaml` configuration

Add the `github:` block to `devteam.yaml`:

```yaml
github:
  provider: native              # 'native' (default, go-github) | 'gh' (fallback adapter)
  app_id: <your App ID from §2>
  installation_id: <your installation ID from §4>
  private_key_path: /etc/devteam/github-app.private-key.pem
  token_cache_ttl: 9m           # < 60m (GitHub's installation-token lifetime); default 9m
  pat_fallback:
    enabled: false              # set true after §8 (PAT fallback setup)
  conflict_poll_max_retries: 5  # NFR-PERF-02; default 5, hard ceiling 10
  conflict_poll_max_duration: 60s  # NFR-PERF-02; default 60s, hard ceiling 300s
```

Validation at load (interaction-spec §4.1): missing `app_id`/`installation_id`/`private_key_path` fails fast with exit 1 and a pointer to this section. `token_cache_ttl >= 60m` fails with `"must be < 60m (GitHub limit)"`.

**Checkable outcome**: `devteam auth health` (next section) loads config without error.

Continue to §6.

---

## §6 First-run verification

```sh
devteam auth health
```

Expected success (M-1, interaction-spec §3.1):

```
✓ machine identity alive
  token source: refreshed
  app_id: <your App ID>
  installation_id: <your installation ID>
```

Then verify repo discovery:

```sh
devteam repo list
```

Expected (M-2): a MANAGED group (empty initially) and an available-but-unmanaged group listing the repos the App installation can access.

**Checkable outcome**: `devteam auth health` exits 0; `devteam repo list` shows discovered repos.

If either fails, go to §11 (Troubleshooting, symptom-indexed).

Continue to §7 for key management.

---

## §7 Key management

### §7.1 App private-key rotation (NFR-SEC-03)

Rotation is a data operation — no code change, no process restart (ADR-10):

1. Go to **GitHub → Settings → Developer settings → GitHub Apps → [your App] → Private keys → Generate a new private key**. A new `.pem` downloads.
2. Replace the file at the configured path:
   ```sh
   cp ~/Downloads/devteam-private-key-*.pem /etc/devteam/github-app.private-key.pem
   chmod 0400 /etc/devteam/github-app.private-key.pem
   ```
3. Run the rotation command:
   ```sh
   devteam auth rotate-key
   ```
   This stores the new key (encrypting with the master key), marks the old key rotated, and emits a `CREDENTIAL_ROTATED` audit event — all in one transaction.
4. **No restart required**: the next JWT mint reads the new key from the credstore on-demand (ADR-10). The current in-process token cache is unaffected; it stays valid until its TTL, then refreshes using the new key.

**Checkable outcome**: `devteam auth rotate-key` prints `✓ app private key rotated` with a fingerprint; `devteam auth health` still succeeds.

### §7.2 Master key backup

The master key is the **load-bearing backup** — the `credentials` ciphertext is inert without it (infra-specs §5.1, finding 2). Back it up once at first setup:

```sh
cp /etc/devteam/master.key <your-offsite-backup-location>
```

Acceptable locations: encrypted USB, password-manager attachment, off-site backup. The backup is **your responsibility** — the system does NOT automate backup (automating backup = creating a second attack surface, NFR-SEC-02).

**Checkable outcome**: a second copy of `master.key` exists outside `/etc/devteam/`.

### §7.3 Master key loss (recovery)

If `master.key` is lost, **ALL stored credentials are unrecoverable** (NFR-SEC-02). There is no escrow, no KDF-from-password. Recovery:

1. Re-generate the master key (§3.2).
2. Rotate the App private key on GitHub (§7.1 step 1) and place the new `.pem` at the configured path.
3. Run `devteam auth rotate-key` to store the new App key (encrypted with the new master key).
4. If you used a PAT (§8), re-store it via `devteam auth store-pat`.
5. `devteam auth health` should now succeed.

**Checkable outcome**: `devteam auth health` ✓ after the recovery sequence.

### §7.4 Master key compromise (re-encryption)

If the master key is suspected compromised, rotate it by re-encrypting all `credentials` rows:

1. Generate a new master key (§3.2) and place it at `DEVTEAM_MASTER_KEY_FILE`.
2. For each active credential row, decrypt with the OLD key and re-encrypt with the NEW key. This is a documented runbook operation (SQL + Go snippet), executed as a one-off — NOT an automated CLI command in MVP's surface.
3. Emit a `CREDENTIAL_ROTATED` audit event with `details="master_key_rotated"` (no values).

**Checkable outcome**: `devteam auth health` ✓ with the new master key; `audit_events` shows the `CREDENTIAL_ROTATED` row.

Continue to §8 if you want PAT fallback (optional); else §9.

---

## §8 PAT fallback setup (optional)

PAT fallback (FR-AUTH-02) keeps the pipeline moving when the App token exchange fails transiently (network, 401, 5xx). It does NOT engage on 404/403 (revoked/suspended installations surface loudly — ADR-09).

1. Create a PAT on GitHub: **Settings → Developer settings → Personal access tokens → Fine-grained tokens → Generate new token**. Scope it to the same repos the App manages; grant `Contents: Read`, `Pull requests: Read & write`.
2. Enable PAT fallback in `devteam.yaml`:
   ```yaml
   github:
     pat_fallback:
       enabled: true
   ```
3. Store the PAT (read from stdin — never arg, never env, to avoid shell-history leakage):
   ```sh
   devteam auth store-pat
   # paste the PAT, press Enter, then Ctrl-D (or blank line)
   ```
   This encrypts the PAT with the master key and emits a `CREDENTIAL_STORED` audit event.
4. Verify: temporarily revoke the App installation on GitHub, then `devteam auth health`. It should print `⚠ machine identity alive (degraded)` with `token source: fallback (PAT)`. Re-enable the App installation afterward.

**Checkable outcome**: `devteam auth health` shows `fallback (PAT)` when the primary identity fails.

Continue to §9.

---

## §9 App-key rejection troubleshooting

**Symptom**: `devteam auth health` prints:
```
✗ key_rejected: app private key rejected (401) (see docs/github-app-setup.md §9)
```

**Cause**: the App private key at `github.private_key_path` was rejected by GitHub (401 from token exchange). The key may have been rotated on GitHub but the local file not updated.

**Fix**:
1. Rotate the key on GitHub (§7.1 step 1) — or re-download the existing key if you lost the file.
2. Place the new `.pem` at the configured path (§3.3).
3. `devteam auth rotate-key` to store the new key.
4. Re-run: `devteam auth health` (expect ✓).

**Checkable outcome**: `devteam auth health` ✓.

---

## §10 Installation revoked/suspended

**Symptom**: `devteam auth health` prints:
```
✗ installation_not_found: installation #N not reachable: 404 (see docs/github-app-setup.md §10)
```
or
```
✗ installation_suspended: installation #N suspended by org admin (see docs/github-app-setup.md §10)
```

**Cause**: the App installation was revoked (404) or suspended (403) by an org admin. PAT fallback is NOT engaged — this is not a transient failure (ADR-09).

**Fix**:
1. Re-install the App on the org (§4) — or ask the org admin to re-enable it.
2. Update `installation_id` in `devteam.yaml` if it changed (a new installation has a new ID).
3. Re-run: `devteam auth health` (expect ✓).

**Checkable outcome**: `devteam auth health` ✓.

---

## §11 `gh` CLI fallback troubleshooting

**Symptom**: `devteam auth health` prints:
```
✗ gh_cli_not_found: provider: gh requires the gh CLI on PATH (see docs/github-app-setup.md §11)
```

**Cause**: `github.provider: gh` is set in `devteam.yaml` but the `gh` binary is not on `$PATH` (NFR-PORT-03).

**Fix** — one of:
1. Install `gh`: see https://github.com/cli/cli#installation, then `gh auth login`.
2. Switch to native: set `github.provider: native` in `devteam.yaml` (the default and the recommended end-state — `gh` is no longer a hard runtime dependency post-feature, C-05).

**Checkable outcome**: `devteam auth health` ✓.

---

## §12 Uninstall/recovery

To remove the feature's footprint (does NOT uninstall Postgres or the binary — those are shared):

1. **Uninstall the App** on GitHub: **Settings → Developer settings → GitHub Apps → [your App] → Advanced → Uninstall** (or the org's installed-apps settings).
2. **Delete the `credentials` rows** (they're now inert anyway — the App is gone):
   ```sh
   psql -h localhost -U devteam -d devteam -c "DELETE FROM credentials;"
   ```
3. **Rotate the master key** (§7.4) or delete it:
   ```sh
   rm /etc/devteam/master.key /etc/devteam/github-app.private-key.pem
   rmdir /etc/devteam
   ```
4. **Remove the `github:` block** from `devteam.yaml` and the `DEVTEAM_MASTER_KEY_FILE` line from `devteam.env`.

**Checkable outcome**: `devteam auth health` fails with a config error (block absent); `psql ... -c "SELECT count(*) FROM credentials"` returns `0`.

---

## Notes

- **No automated install**: there is no `devteam auth setup` command that creates the App or installs it on the org. This runbook is the sole setup mechanism (NFR-OPS-02, C-13). A static check greps `internal/` for any function calling GitHub's App-creation endpoints → zero hits.
- **Key loss is unrecoverable**: the master key has no escrow. Your backup (§7.2) is the recovery mechanism.
- **`gh` stays installed**: `provider: native` makes `gh` optional, but `provider: gh` fallback still uses it. Uninstalling `gh` is a follow-up, not a feature deliverable (infra-specs §6.3).
- **Windows ACL**: `os.Chmod(0400)` is a no-op on Windows; the master key file's ACL is your responsibility (NFR-PORT-01, infra-specs §3.2). The system does NOT attempt programmatic ACL management.

*End of runbook.*