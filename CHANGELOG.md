# Changelog

All notable changes to this project are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
