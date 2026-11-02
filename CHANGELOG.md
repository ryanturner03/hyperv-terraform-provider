# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow
[Semantic Versioning](https://semver.org/).

## [0.2.0] - 2026-11-02

### Added
- GitHub release pipeline: GoReleaser builds signed, checksummed archives for
  Linux, macOS, and Windows (amd64/arm64) on every `v*` tag.
- `scripts/install.sh` and `scripts/install.ps1` install a released binary into
  Terraform's local plugin directory with checksum verification.
- CI workflow running `go vet`, `go test`, and a GoReleaser snapshot build on
  pull requests.

### Fixed
- `hyperv_vm` with `generation = 1` can now be created. Secure-boot attributes
  left unset no longer remain unknown after apply, `secure_boot_enabled = false`
  is accepted on Gen 1, and the provider never calls `Get-/Set-VMFirmware` on
  Gen 1 VMs. `secure_boot_enabled = true` or a `secure_boot_template` on Gen 1
  is still rejected with a clear message.
