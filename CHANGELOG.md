# Changelog

All notable changes to this project are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.0] - 2026-09-26

### Features & Improvements

- **history**: add SMS and call history subsystem with Home Assistant card controls (`0c7f062`)
  - Introduce SMS and call history persistent storage and automatic recovery across restarts.
  - Add MQTT topics <prefix>/modem/<id>/sms/history and <prefix>/modem/<id>/call/history with JSON list and count.
  - Add MQTT command topics <prefix>/modem/<id>/sms/history/clear and <prefix>/modem/<id>/call/history/clear to clear logs.
  - Add Home Assistant Auto-Discovery entities: sensor.<modem>_sms_history, sensor.<modem>_call_history, button.<modem>_clear_sms_history, and button.<modem>_clear_call_history.
  - Enrich call lifecycle tracking with call status classification (missed, completed, rejected, busy, no_answer) and caller details.
  - Provide REST API endpoints POST /api/sms/inbox/clear, GET /api/call/history, and POST /api/call/history/clear.
  - Update Web UI control panel with Call History telemetry table and one-click clear buttons.
  - Update Home Assistant Lovelace card gsm2mqtt-card.js with collapsible SMS and call history menus, counter badges, and quick dialing.
- **call**: add incoming caller number sensor with automatic idle reset (`38ce364`)
  - Add sensor.<modem>_caller_number Home Assistant entity showing active caller phone number.
  - Reset caller number sensor to idle immediately upon call termination to prevent stale numbers.
  - Switch binary_sensor.<modem>_incoming_call to explicit ON/OFF payloads for immediate turn-off on call end.
  - Publish initial ended call status on gateway startup to ensure clean sensor state.
  - Update Home Assistant discovery, test suite, and integration documentation.
- **sms**: separate last SMS topic and implement multipart assembly timeout with late-part reassembly (`638de43`)
  - Add dedicated sms/last topic with retained flag for Home Assistant UI card, keeping sms/received non-retained to prevent false trigger pulses on reboot.
  - Clear broker retained cache on sms/received at startup to purge stale messages from prior versions.
  - Introduce assembly_timeout configuration parameter to flush partial multipart SMS after configurable wait duration.
  - Implement Assembler V2 with automatic reassembly of late-arriving segments into unified [Updated] messages.
  - Update Home Assistant discovery, configuration files, and documentation for new SMS topics and settings.
- **gateway**: add structured event stream, Home Assistant binary sensors, and gateway timezone support (`1a4df35`)
  - Add unified <prefix>/modem/<id>/event MQTT topic with structured event schema
  - Add Home Assistant Auto-Discovery for connected, problem, and low_balance binary sensors
  - Add Home Assistant Auto-Discovery for event entity supporting standard alert types
  - Add gateway host timezone auto-detection (/etc/TZ, $TZ) and system.timezone configuration
  - Format slog console logs and MQTT event timestamps in local time with ISO 8601 offset
  - Synchronize tariff daily and monthly reset schedules with gateway local midnight
  - Pass system TZ to gsm2mqtt environment in OpenWrt procd init script
- **mqtt**: add no_tls build tag and pure TCP MQTT engine for embedded targets (`7d97b35`)
  - Introduce no_tls build tag providing a lightweight, pure-TCP MQTT 3.1.1 client that eliminates crypto/tls and crypto/rand dependencies
  - Resolve early runtime Segmentation fault on legacy MIPS Linux 4.4 kernels caused by Go 1.24+ crypto/rand FIPS-140 initialization
  - Decouple net/http from internal/metrics and internal/api under no_api build tag to produce completely crypto-free binaries
  - Add build-mips and build-mipsel Makefile targets and configure OpenWrt MIPS/MIPSEL package compilation with no_api,no_tls
  - Configure Procd init script memory limits (GOMEMLIMIT=10MiB, GOGC=25) for resource-constrained 32MB RAM routers
- **installer**: Add universal one-line installer script for Linux and OpenWrt (`6bc438d`)
  - Add scripts/install.sh with automated platform, architecture, and package manager detection
  - Support native OPKG (.ipk) and APK (.apk) installation on OpenWrt with procd service registration
  - Support systemd service installation and gsm2mqtt user provisioning on Linux distros
  - Preserve existing user configurations at /etc/gsm2mqtt/gsm2mqtt.yaml
  - Document one-line installation prominently in README.md and OpenWrt guide

### Bug Fixes

- Release automations (`b7d91bc`)
- **mqtt**: re-publish online status and discovery on broker reconnect (`95d8785`)
  - Add OnConnect lifecycle callback to MQTT client configuration for Paho and TCP clients.
  - Re-publish gateway online status with QoS 1 and retained flag upon broker reconnection.
  - Re-publish Home Assistant discovery and dynamic alert recipients state on reconnect.
- **tariff**: ensure configuration file limits take precedence over persisted state (`ca50908`)
  - Prevent stale JSON state from overwriting updated YAML and UCI limits on daemon startup
  - Separate static configuration parameters from runtime accounting usage counters and balance
  - Add LowBalance flag and MinBalanceAlert threshold to tariff UsageStatus snapshot
- **gateway**: publish disconnected state and update Home Assistant when modem is offline (`5d1f495`)
  - Publish retained 'disconnected' health state to gsm2mqtt/modem/<id>/health on runner startup, disconnect, and retry.
  - Clear stale active modems count in gsm2mqtt/gateway/modems (count: 0) when modem port is closed or missing.
  - Add 'disconnected' enum option to Home Assistant status sensor discovery payload.
  - Ensure health telemetry updates are published with retained flag in MQTT.
  - Document 'disconnected' modem health state, retained MQTT topics, and Lovelace card status badge in docs/home-assistant-integration.md and docs/mqtt-topics.md.
  - Add unit test TestModemRunner_DisconnectedStatePublished verifying state transitions.
- **sms**: fix UCS-2 PDU chunking for UTF-16 surrogate pairs and emojis (`85168f9`)
  - Account for 4-byte surrogate pairs (emojis > 0xFFFF) in UCS-2 message length calculations
  - Ensure user data length (UDL) never exceeds the 140-byte 3GPP limit when emojis are present
  - Prevent AT+CMGS rejection with CMS ERROR: operation not supported on emoji messages
- **modem**: fix Neoway M590 SMS sending and AT+CPMS storage handling (`53b298b`)
  - Enforce SIM card storage (SM) and block unsupported ME storage queries on Neoway M590 modems
  - Add a 100ms pacing delay after '>' prompt in AT PDU transmission for 9600 baud serial reliability
  - Ensure primary SM storage is restored after multi-storage offline message synchronization
- **gateway**: improve modem retry loop, support MQTT QoS 2, and safeguard OpenWrt configs (`8452676`)
  - Trigger modem runner retry backoff when driver initialization fails instead of proceeding silently
  - Add full MQTT 3.1.1 QoS 2 handshake (PUBREC/PUBREL/PUBCOMP) to pure-TCP client engine
  - Protect existing /etc/gsm2mqtt/gsm2mqtt.yaml configs during OpenWrt OPKG/APK upgrades via sample templates
  - Install automatic OpenWrt hotplug symlink script mapping USB serial devices to /dev/ttyGSM
  - Document modular build tags (no_api, no_tls) and embedded MIPS targets in README.md
- **packaging**: Add Raspberry Pi 3 OPKG architecture and upgrade GitHub release action to v3 (`78bb834`)
  - Add aarch64_cortex-a53 and aarch64_cortex-a72 targets for ARM64 OpenWrt OPKG packages
  - Upgrade softprops/action-gh-release from v2 to v3 for Node.js 24 compatibility
  - Enforce GNU tar format and standard archive entry order in OPKG generator
  - Update OpenWrt deployment documentation with device-specific architecture guidance

### Documentation

- **readme**: document full Unicode & Emoji support and Neoway M590 modem (`e1e9c90`)
  - Add documentation for full Emoji and multi-byte SMP Unicode handling via UTF-16 surrogate pairs in UCS-2 PDU mode.
  - Explain 3GPP 140-byte boundary segmentation preventing modem payload rejection (+CMS ERROR).
  - Add Neoway M590 / M590E to the supported modems matrix.

## [0.1.4] - 2026-09-25

### Features & Improvements

- **installer**: Add universal one-line installer script for Linux and OpenWrt (`25873c4`)
  - Add scripts/install.sh with automated platform, architecture, and package manager detection
  - Support native OPKG (.ipk) and APK (.apk) installation on OpenWrt with procd service registration
  - Support systemd service installation and gsm2mqtt user provisioning on Linux distros
  - Preserve existing user configurations at /etc/gsm2mqtt/gsm2mqtt.yaml
  - Document one-line installation prominently in README.md and OpenWrt guide
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

- **packaging**: Add Raspberry Pi 3 OPKG architecture and upgrade GitHub release action to v3 (`28239b6`)
  - Add aarch64_cortex-a53 and aarch64_cortex-a72 targets for ARM64 OpenWrt OPKG packages
  - Upgrade softprops/action-gh-release from v2 to v3 for Node.js 24 compatibility
  - Enforce GNU tar format and standard archive entry order in OPKG generator
  - Update OpenWrt deployment documentation with device-specific architecture guidance
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
