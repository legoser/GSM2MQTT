# Changelog

All notable changes to this project are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.4] - 2026-09-25

### Features & Improvements

- **config**: support copying example configuration to working config with custom QoS (`0276457`)
  - Add CopyExampleConfig to generate working YAML configuration from template with custom QoS
  - Export ModemRunner.QoS() to inspect active modem runner QoS level
  - Update TestLoad_MainConfig to automatically bootstrap working config from template during CI
  - Add integration test verifying working config QoS applies across ModemRunner and GatewayManager
- **mqtt**: add configurable QoS levels, connection options, and dynamic discovery version (`87cfdf9`)
  - Add configurable MQTT QoS levels (0, 1, 2) across all publishers, subscribers, and LWT messages
  - Add MQTT broker connection parameters: clean_session, keep_alive, connect_timeout, auto_reconnect, and max_reconnect_interval
  - Support environment variable overrides for all MQTT connection parameters (GSM2MQTT_MQTT_*)
  - Replace hardcoded discovery version in Home Assistant MQTT discovery with dynamic versioning from internal/version
  - Update OpenWrt and example configuration templates with fully documented MQTT connection options

### Bug Fixes

- **scripts**: Resolve previous baseline tag and add pre-flight checks in prepare_release.py (`07e065a`)
  - Determine previous tag strictly preceding target release version
  - Validate working tree and tag collision before modifying CHANGELOG
  - Support --force flag in prepare_release.py and Makefile
- Deployment settings (`07370c3`)

### Documentation

- **guides**: Add comprehensive architecture, MQTT protocol, and hardware docs (`d68baa3`)
  - Add docs/architecture.md detailing component design, modem pool, and 5-layer security
  - Add docs/mqtt-topics.md with complete topic reference and JSON schemas
  - Add docs/modems.md with hardware matrix, wiring, and operator tariff presets
  - Add docs/development-and-release-guide.md with Git hooks and release cut lifecycle
  - Add .githooks/pre-commit and .githooks/commit-msg for automated testing and format validation
  - Add scripts/prepare_release.py and make prepare-release to automate version cuts
  - Update AGENTS.md to offload manual testing to pre-commit automation

### Other Product Changes

- ci(update package) (`56e3128`)
- ci(Choise runner) (`c81e98a`)
- ci(Choise runner) (`51f2c07`)

## [0.1.3-ge7a392d] - 2026-09-25

### Features & Improvements

- **openwrt**: add OPKG (.ipk) and APK (.apk) packaging and service integration (`85181ae`)
  - Add OpenWrt procd init script (deployments/openwrt/files/gsm2mqtt.init) supporting
    start, stop, restart, reload, enable, disable, and auto-respawn
  - Add UCI configuration template (deployments/openwrt/files/gsm2mqtt.config)
  - Add complete OpenWrt configuration file (deployments/openwrt/files/gsm2mqtt.yaml)
    with thorough English comments and documented allowed_at_commands allowlist
  - Sync main configs/gsm2mqtt.example.yaml with full English documentation,
    accounting fields, and raw AT command allowlist
  - Add official OpenWrt package recipe (deployments/openwrt/Makefile) for buildroot/SDK
  - Add portable package build script (scripts/build_openwrt_packages.sh) producing:
  * OPKG (.ipk) archives for OpenWrt <= 23.05 (x86_64, aarch64, armv7, mipsel, mips, riscv64)
  * APK (.apk) archives for OpenWrt >= 25.12 using apk-tools v3 / Alpine Docker fallback
  - Add make targets: package-openwrt, package-opkg, package-apk
  - Update GitHub and Forgejo release workflows to publish .ipk and .apk assets
  - Add OpenWrt deployment and driver guide in docs/openwrt.md and update README.md
  - Add regression tests in internal/config/loader_test.go validating config schemas

### Bug Fixes

- **core**: resolve concurrency data races and deadlocks (`e7a392d`)
  - Fixed RWMutex data race on `lastCurrency` read in gateway ops
  - Fixed critical deadlock in `tariff.Manager` by firing MQTT alerts outside of the mutex lock
  - Added 10-second timeouts to MQTT Connect, Publish, and Subscribe operations to prevent infinite blocking
  - Fixed goroutine leak by adding context timeouts to fallback voice call operations
  - Propagated execution contexts down to SMS PDU encoding and AT engine layers
- **security**: reject requests with empty API token and mask logs (`b3e6e25`)
  - Updated API auth middleware to return 403 Forbidden for protected routes if token is empty (fails closed instead of open)
  - Masked phone numbers in rate limiter and alert recipient logs (+7999***1234 pattern)
  - Removed privileged flag from development docker-compose file

### Performance Improvements

- **build**: add build-noapi and build-small targets for OpenWrt (`e8beacd`)
  - Introduced `build-noapi` Makefile target using `no_api` build tag to strip HTTP API server
  - Introduced `build-small` Makefile target for automated UPX compression
  - Reduced binary size by ~10% for storage-constrained OpenWrt environments

## [0.1.0] - 2026-09-24

First public release: GSM-to-MQTT gateway connecting GSM/3G/4G modems
to Home Assistant and any MQTT-based system via AT commands.
Single static binary, no runtime dependencies.

### Added

- SMS send and receive: Cyrillic (UCS-2), transliteration, multipart, delivery reports.
- Voice calls: dial, answer, hangup, DTMF send/receive.
- Multi-modem pool: round-robin, failover, best-signal, operator-match, SIM redundancy.
- Operator presets and tariff accounting (MTS, Megafon, Beeline, Tele2): balance parsing, SMS quotas.
- Automated diagnostics with MQTT alerts (SIM, network, signal, balance).
- Prometheus metrics (`GET /metrics`), embedded web dashboard and REST API, AT HTTP client.
- Home Assistant MQTT auto-discovery; security (rate limiting, whitelist, AT sanitization).
- `--version` / `-v` flag printing the ldflags-injected version.
- Automated release pipelines (Forgejo + GitHub Actions): versioned `tar.gz` per platform + `checksums.txt` on every `vX.Y.Z` tag.

## Release process

1. Merge `development` into `main`.
2. Generate changelog: run `make changelog` (or `python3 scripts/generate_release_notes.py --update-changelog`) to automatically collect changes from git commits into `CHANGELOG.md`.
3. Commit and tag the release: `git tag vX.Y.Z && git push origin main vX.Y.Z`.
4. CI builds `tar.gz` archives, OpenWrt packages (`.ipk` and `.apk`) and `checksums.txt`, automatically generates categorized release notes with full commit details, and publishes the Release in Forgejo and on GitHub.
5. Verify: `sha256sum -c checksums.txt && ./gsm2mqtt --version`.
