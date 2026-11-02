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

## Publication completed

[Container run 41213300732](https://github.com/uJZk/Xray-core/actions/runs/41213300732) succeeded, including both architecture builds, manifest publication, and image inspection. Published `ghcr.io/ujzk/xray-core:26.10.31-domain-stats-b1d1eebf` and `ghcr.io/ujzk/xray-core:latest`. Runtime code is from implementation commit `b1d1eebf`; subsequent commits only change workflows/documentation.

The initial full test run failed because `resources/geoip.dat` and `resources/geosite.dat` were absent before asset cache initialization. After the successful asset update, failed jobs of [test run 41213292216](https://github.com/uJZk/Xray-core/actions/runs/41213292216) were re-requested; check its latest attempt before claiming full CI success. Local targeted tests and format/protobuf checks passed. The old failed Release run remains historical; the newer Release run confirms resource dispatch now succeeds.

## Traffic CSV — 2026-11-02

Implemented optional `log.traffic` append-only CSV with the agreed six fields: `interval_start,interval_end,target,target_ip,uplink_bytes,downlink_bytes`. `target_sld` was explicitly removed. A background logger groups independent payload deltas by target/IP every minute and flushes on logger close. It shares the existing dispatcher/splice accounting hooks, not resettable Stats API values; no StatsService configuration or Python collector is required. Direct freedom TCP supplies the actual remote IP; unknown final IPs remain empty. See [domain-traffic.md](domain-traffic.md) for paths, permissions, append/retry/rotation behavior and limitations.

Verification completed:

- Targeted tests passed for `common/log`, `app/log`, `app/dispatcher`, `proxy`, `proxy/freedom`, and `infra/conf`.
- Race checks passed for `common/log`, `app/log`, `app/dispatcher`, `proxy/freedom`, and `infra/conf`.
- Built the complete binary and exercised local VLESS Vision + TLS with fallback: real splice confirmed in debug logs; domain and IPv4 requests each transferred a 1,120,000-byte HTTPS body and recorded the expected target/IP. No Stats config was enabled. Fallback-only requests produced no rows; shutdown flushed pending deltas, and restarting appended rows without repeating the header.
- Unit tests cover independent CSV counting across API counter reset, logger restart for existing recorders, concurrent updates, failed-write retry without loss/duplicate output, rotation, invalid headers and partial trailing lines, and protobuf/JSON config wiring.

CSV publication was requested with tie-up, explicitly authorizing commit, merge, push and deployment. Final targeted race, formatting, protobuf generation, and whitespace checks passed again. All 15 changed/new files were verified as belonging to this CSV implementation; local HEAD and origin/main both matched d8e1f9ee before committing. The Docker workflow will build the publication artifact from the committed source. See the publication result below when available.

## Traffic CSV continuity and network — 2026-11-02

- CSV now includes a `network` field from the target transport (e.g. `tcp` or `udp`). Consecutive intervals are merged only when target, target IP, and network match and the old end equals the new start. Gaps and distinct networks remain separate. Merging rewrites the file atomically and may copy a large history; rotate as needed.
- The seven-column format intentionally does not migrate older six-column CSV files. Rotate the old file or use a new path before upgrading, and update any CSV readers for the extra column. See [domain-traffic.md](domain-traffic.md).
- Built `./main`; race tests passed for `common/log`, `app/log`, `app/dispatcher`, `proxy`, `proxy/freedom`, and `infra/conf`. Coverage includes network separation, consecutive intervals, gaps, old-header rejection, retry and rotation.
- User requested publication of a new container image and explicitly declined a GitHub release. Use the Docker workflow against the exact pushed commit, publishing a unique tag and `latest`; check the workflow and image before claiming publication. No host deployment is configured.
- Publication succeeded: [Docker workflow 41404137160](https://github.com/uJZk/Xray-core/actions/runs/41404137160) built commit `7ed37b2761660e6e60989e0e58577b85656c4ef6`, pushed and inspected the multi-architecture image `ghcr.io/ujzk/xray-core:26.10.31-traffic-7ed37b27`, and updated `latest`. No GitHub release was created. The workflow reported an existing Dockerfile `FromPlatformFlagConstDisallowed` warning.
