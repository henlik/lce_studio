# Backup, reset and restore

Use **Backup & reset** in the top bar, from any workspace. These operations apply
to the **entire configured studio database**, not only the open project.
They are available in the studio's existing local, single-user operating mode.

## Create a backup without resetting

1. Open **Backup & reset**.
2. Choose **Create backup**. Editing and autosave pause while the dialog holds the
   snapshot. Wait for the retained-backup confirmation and download.
3. Keep the downloaded `lce-studio-…json` file outside the studio. **Download again**
   is available if the browser did not save the first download.
4. Select **Done** to return to editing. No application data is cleared.

The server also retains the backup in PostgreSQL. The **Restore** tab lists the
latest 100 backups; reset and restore do not delete backup history. Retained copies
protect against application reset; the downloaded copy is needed if the database
or its storage is lost. There is no automatic retention/deletion policy yet.

## Reset to an empty studio

1. Save or export unsaved edits in other tabs or browsers. This tab cannot capture
   their in-memory buffers.
2. Open **Backup & reset** and choose **Create backup**.
3. Review the number of database rows and browser drafts in the retained backup.
4. Type **RESET**, then select **Reset studio**.
5. The studio reloads with empty application catalogs and workspaces. Examples
   remain available through their explicit load actions.

The reset cannot proceed without a retained backup of the current database. If
another client saves after that snapshot, reset is rejected. Choose **Create fresh
backup**, review it, and confirm again. A failed backup never triggers a reset.

## Restore a backup

1. Open **Backup & reset → Restore**.
2. Upload a backup JSON file, or select a retained backup and click **Use selected
   backup**. The server validates the format, checksum and table/column structure.
3. Choose **Create backup** to preserve the current studio before replacing it.
4. Type **RESTORE**, then select **Restore studio**.
5. The studio reloads with the restored records, source drafts, testbenches and
   saved browser settings. Restore replaces current application data; it does not
   merge projects or catalog versions.

A restore that violates database constraints rolls back in full. The current
studio and its safety backup remain intact.

## What is included

| Area | Included |
| --- | --- |
| PostgreSQL application data | Projects, simulation runs, component packages and evaluations, data contracts and validations, state machines and runs, delivery models and runs, workflow packages and runs, system designs, component definitions, system executions, replays, evidence, configuration packages |
| This browser | IndexedDB workspace drafts, saved recovery checkpoints, current mounted workspace buffers in this tab, configuration files and testbenches, ribbon preferences |
| Recovery metadata | Format version, creation time, immutable backup ID, database hash and complete-backup checksum |

Application binaries/source, Git repositories, environment variables, credentials,
external infrastructure, unrelated database tables, browser cookies and other
origins are outside this operation. Built-in examples are code assets and stay.
Other browsers' local drafts and other tabs' unsaved buffers are not included.
Backups contain studio data, including any sensitive content authored in it; they
are JSON files, not encrypted or signed release artifacts.

The archive format is `lce-studio-backup/1`, with a maximum serialized size of
128 MiB and 200,000 database rows. Larger studios need an operator-managed backup;
the button refuses to clear data when its backup limit is exceeded. The `database`
and draft `data` fields are JSON **strings** so a browser round trip does not round
64-bit seeds or other large integers. Do not rewrite them or edit checksums.

## Interrupted operations and other tabs

PostgreSQL replacement is atomic. PostgreSQL and browser storage cannot share one
transaction, so the browser records a recovery intent before submitting reset or
restore. If the browser closes, its storage fails, or a response is lost, reopening
the studio presents **Resume recovery**. The same backup ID makes retries
idempotent: they finish the original operation without clearing newly written data
again. Keep the safety backup until recovery finishes.

Autosave remains paused during recovery. Existing tabs retain their visible
buffers, but their old browser generation and API epoch cannot overwrite the reset
or restored studio. Export any edits you still need and reload those tabs. An
older application build may need to be closed for the draft-store upgrade.

PostgreSQL advisory request leases coordinate cooperating studio servers sharing
one schema. The browser sends `X-Studio-Epoch` on normal API requests. CLI clients
without that header remain compatible, so stop external writers and direct SQL
jobs before maintenance; they are outside browser stale-session protection.

## API

All routes are under `/api/studio`. POST bodies use `application/json`; the
existing same-origin write protection applies. These are local administrative
operations, not a multi-user authorization boundary.

| Method / route | Request / result |
| --- | --- |
| GET `/status` | Current epoch and last committed reset/restore identifiers |
| GET `/backups` | Latest 100 retained backup summaries |
| GET `/backup?id=…` | Complete retained archive |
| POST `/backup` | `{epoch, browser: {drafts, preferences}}`; returns and retains a complete archive |
| POST `/validate-backup` | `{backup}`; validates the archive and returns table counts without restoring |
| POST `/reset` | `{epoch, backupId, confirmation: "RESET"}` |
| POST `/restore` | `{epoch, backupId, confirmation: "RESTORE", backup}` |

A stale epoch or changed database snapshot returns HTTP 409 without replacement.
Invalid confirmation, damaged backup or failed restore returns an error without
partial database deletion. Backup history and the control epoch are outside the
18-table application reset allowlist. Adding a persisted application table requires
updating that allowlist, restore order, archive version/migration policy and tests.

## Acceptance

Automated PostgreSQL tests cover every persisted table, large integer preservation,
retained history, unrelated-table preservation, foreign-key rollback, cross-server
request leases, stale sessions and idempotent recovery. Chromium acceptance covers
the actual backup/download/reset/restore UI, a simulated browser-storage failure
after server commit, recovery after reload, a stale tab, a damaged file, retained
backup selection and mobile overflow.

**M1 100%; weighted roadmap 33%, unchanged.** This is studio usability and recovery
work; it does not add live orchestration or runtime verification milestone credit.
