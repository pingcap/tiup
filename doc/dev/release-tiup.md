# Releasing TiUP

The `release-tiup` workflow (`.github/workflows/release-tiup.yaml`) builds the TiUP
driver and components, publishes seven components to a TiUP mirror, and uploads
the all-in-one tarball to the GitHub release.

## Mirror environments

The mirror address and the signing key are bound per GitHub Environment, so a job
can only use the pair for the mirror it publishes to:

| Environment | Trigger | Secrets |
| --- | --- | --- |
| `staging` | `workflow_dispatch` with `staging-mirror: true` | `TIUP_SERVER`, `TIUP_COMP_KEY` |
| `production` | `push` to `master`, `push` to `ci/gh-*`, `release`, `workflow_dispatch` with `staging-mirror: false` | `TIUP_SERVER`, `TIUP_COMP_KEY` |

- `TIUP_SERVER` — mirror base URL. Examples: `http://tiup.pingcap.net:8988`
  (staging) and `http://tiup.pingcap.net:8987` (production). A trailing slash is
  tolerated.
- `TIUP_COMP_KEY` — base64 of that mirror's `private.json` owner key. Staging and
  production have **different** owner keys; never use one for the other.

The environments intentionally have no required reviewers and no deployment
branch policy: a release runs four matrix jobs, so a reviewer gate would produce
four approvals per release.

## Setup or change

Requires repository admin.

```bash
# create the environments (no protection rules)
gh api -X PUT repos/pingcap/tiup/environments/staging
gh api -X PUT repos/pingcap/tiup/environments/production

# mirror URL
gh secret set TIUP_SERVER --env staging --repo pingcap/tiup --body "http://tiup.pingcap.net:8988"
gh secret set TIUP_SERVER --env production --repo pingcap/tiup --body "http://tiup.pingcap.net:8987"

# owner key, base64-encoded on one line
base64 < /path/to/staging-private.json | tr -d '\n' \
  | gh secret set TIUP_COMP_KEY --env staging --repo pingcap/tiup
base64 < /path/to/production-private.json | tr -d '\n' \
  | gh secret set TIUP_COMP_KEY --env production --repo pingcap/tiup
```

To change a mirror address or rotate a key, update that environment's secret
only; no workflow change is needed.

## How the workflow runs

1. `resolve` selects `staging` only for a `workflow_dispatch` with
   `staging-mirror: true`; every other trigger selects `production`.
2. `bootstrap` checks out the release ref on a GitHub-hosted runner, builds the
   `tiup` driver with `make tiup`, and uploads `bin/tiup` as a short-lived
   artifact.
3. Each `release` matrix job (linux/darwin × amd64/arm64) downloads that driver,
   fetches `root.json` from the selected mirror, builds and packages the
   components, then runs `tiup mirror set` and `tiup mirror publish`.
4. `brew-upgrade` runs after a `release` event.

If `TIUP_SERVER` or `TIUP_COMP_KEY` is empty, the job fails before any publish;
there is no fallback to another mirror or another key.

## Verify a staging run

1. Run the workflow with `git-ref: master` and `staging-mirror: true`.
2. Check that the run page shows the `staging` deployment environment.
3. Check the logs: `bootstrap` contains `make tiup`; `release` contains
   `tiup --version` and does **not** request `tiup-linux-amd64.tar.gz`; the
   `root.json` request goes to the staging mirror.
4. Check the published component manifest: `signatures[].keyid` must equal the
   staging `index.json` owner key id.

## Rollback

- Revert the workflow commit to restore the previous behavior, or
- change only the environment secret to point at another mirror.
- Re-running a staging or other non-formal version overwrites it; released
  `vX.Y.Z` versions are immutable.

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| `environment is missing TIUP_SERVER or TIUP_COMP_KEY` | secrets not configured on that environment |
| `The server refused, make sure you have access to this component` | wrong owner key for this mirror, or the key was rotated |
| `root.json` fetch fails | mirror unreachable, or `https` used against an endpoint that only serves HTTP |
| A matrix job fails right after packaging | artifact download failed, or the mirror rejected the upload |
