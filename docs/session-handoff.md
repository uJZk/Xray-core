# Session handoff — 2026-10-31

## Implemented

- Target domain traffic counters aggregate all users through the existing StatsService. When sniffing does not supply a domain, count the target IPv4/IPv6 instead. The `domain>>>` prefix is retained for both. See [domain-traffic.md](domain-traffic.md).
- Count cached sniffing bytes once and update nested domain/user writers when Vision downlink uses splice. Fallback has no proxy target and is excluded. Payload counters do not include ingress TLS or Vision padding, and do not measure remote acknowledgements.
- Hourly upstream stable-release polling creates a PR, merges when conflict-free and permitted by GitHub, then calls the container workflow using the exact merge SHA. See [upstream-sync.md](upstream-sync.md).
- Existing container workflow supports reusable calls while retaining manual and Release triggers.

## Verification

- `go test ./app/dispatcher ./proxy ./proxy/vless/inbound ./proxy/freedom` passed.
- `go build -o /tmp/xray-domain-stats ./main` and binary version smoke check passed.
- `node --check .github/scripts/sync-upstream.cjs` passed.
- `node .github/scripts/sync-upstream.test.cjs` passed: included release, conflicts, successful merge, existing PR, merge rejection.
- `actionlint` passed for the sync and Docker workflows (shellcheck/pyflakes disabled).
- YAML parsing and `git diff --check` passed.
- Real Vision/fallback end-to-end traffic has not been exercised.

## Operations

Default branch is `main`. Actions settings must allow workflows to create PRs, and merge commits must be enabled. Current CLI credentials cannot read Actions administration settings (HTTP 403); verify these settings through a repository administrator if automation is denied.

After pushing, publish the initial container through the existing manual Docker workflow, since the current upstream release is already contained in this fork and the sync workflow will skip it. Monitor the multi-architecture build in Actions. Subsequent new upstream stable releases use the automatic sync and publication path. No service/container host deployment is configured in this repository.

## GitHub execution results

- Implementation commit: `b1d1eebf507d0368e2211957f5dc747d8a44ab5f`, fast-forwarded to `main` and pushed; temporary `work` branch removed.
- [Upstream sync run](https://github.com/uJZk/Xray-core/actions/runs/41213320841) succeeded, confirming `v26.10.31` is already included. Real PR creation/merge remains to be exercised by a future release.
- [Initial container publication](https://github.com/uJZk/Xray-core/actions/runs/41213300732) was dispatched for the implementation commit with tag `26.10.31-domain-stats-b1d1eebf` and `latest=true`.
- GitHub format/protobuf checks passed on the implementation commit.
- Existing [Build and Release run](https://github.com/uJZk/Xray-core/actions/runs/41213292196) failed in its unchanged `check-assets` job: empty initial cache led to a workflow dispatch denied with HTTP 403, requiring `actions: write`. Subsequently fixed at the user's request by granting `actions: write` only to the `check-assets` job; rerun verification follows below.

## Release resource dispatch permission fix

The `check-assets` job in `.github/workflows/release.yml` now explicitly grants `contents: read` and `actions: write`, allowing it to dispatch the existing asset update workflow when caches are missing. No other job permissions were expanded.

Verified on GitHub: [Release run 41213710923](https://github.com/uJZk/Xray-core/actions/runs/41213710923) successfully completed the asset dispatch step; [dispatched asset update 41213720858](https://github.com/uJZk/Xray-core/actions/runs/41213720858) completed successfully. The original HTTP 403 is resolved. The permission fix commit is `1fd9cd57c3cd64c6eb95cc9bbed9c76a0f4f115a`.
