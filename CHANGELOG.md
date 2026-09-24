# Changelog

All notable changes to this project are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `--version` / `-v` flag printing the ldflags-injected version (`gsm2mqtt <version> (built <time>)`).
- Automated release workflows (Forgejo + GitHub Actions): versioned `tar.gz` archives per platform + `checksums.txt` on every `vX.Y.Z` tag.

## Release process

1. Merge `development` into `main`.
2. Tag the release: `git tag vX.Y.Z && git push origin main vX.Y.Z`.
3. CI builds `gsm2mqtt-<version>-linux-{amd64,arm64,riscv64}.tar.gz` + `checksums.txt`
   and publishes a Release with the same name in Forgejo and (via push-mirror) on GitHub.
4. Verify: `sha256sum -c checksums.txt && ./gsm2mqtt --version`.
