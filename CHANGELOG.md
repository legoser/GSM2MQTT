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
2. Write the release notes: move `[Unreleased]` entries into a new `## [X.Y.Z] - YYYY-MM-DD` section above.
   The release workflows publish exactly this section as the release description.
3. Commit, then tag the release: `git tag vX.Y.Z && git push origin main vX.Y.Z`.
3. CI builds `gsm2mqtt-<version>-linux-{amd64,arm64,riscv64}.tar.gz` + `checksums.txt`
   and publishes a Release with the same name in Forgejo and (via push-mirror) on GitHub.
4. Verify: `sha256sum -c checksums.txt && ./gsm2mqtt --version`.
