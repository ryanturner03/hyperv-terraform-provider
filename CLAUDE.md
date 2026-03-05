# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

A custom Terraform provider for managing Hyper-V virtual machines over WinRM. Standout features: **inline resource blocks** (hard drives, DVD drives, network adapters defined directly in the VM resource) and **cloud-init/cloudbase-init support** via NoCloud seed ISOs. Supports multi-host management with Kerberos, NTLM, and Basic authentication.

## Build & Test Commands

```bash
# Build
go build -o terraform-provider-hyperv .

# Run all unit tests
go test ./...

# Run a single test
go test -v -count=1 -run TestFunctionName ./internal/resources/

# Acceptance tests (require live Hyper-V host)
# Set env vars in .env then use set -a to export them:
set -a && source .env && set +a
TF_ACC=1 go test -v -count=1 -tags=acceptance -run TestAcc ./internal/provider/

# Dev override: add to ~/.terraformrc to use local build
# provider_installation {
#   dev_overrides {
#     "registry.terraform.io/ryan/hyperv" = "/path/to/this/repo"
#   }
# }
```

No Makefile — standard `go build`/`go test` workflow.

## Architecture

Three-layer design with clean separation:

**Provider layer** (`internal/provider/`) — Registers all resources and data sources, handles provider configuration (host, auth, TLS), creates the WinRM client during `Configure()` and passes it to resources as provider data.

**Resource layer** (`internal/resources/`, `internal/datasources/`) — Terraform CRUD implementations using the Plugin Framework. Each resource follows the same pattern: define model struct with `tfsdk` tags → Schema() → Configure() → Create/Read/Update/Delete. Data sources in `datasources/` are read-only equivalents.

**Client layer** (`internal/client/`) — All Hyper-V communication. `HyperVClient` interface in `client.go` defines every operation. `WinRMClient` implements it by building PowerShell commands and executing them over WinRM via `PowerShellRunner`. Includes per-VM mutex locks and automatic retry on transient Hyper-V conflicts (`retryOnConflict`).

### Inline Blocks

The VM resource (`resources/vm.go`) supports optional inline `hard_drive`, `dvd_drive`, and `network_adapter` blocks, with helpers in `resources/vm_inline.go`. These are created atomically with the VM — if any sub-resource fails, the entire VM is deleted (no orphans).

Updates use diff-based logic keyed by:
- Hard drives: `(ControllerType, ControllerNumber, ControllerLocation)`
- DVD drives: `(ControllerNumber, ControllerLocation)`
- Network adapters: `Name`

Helper functions: `diffAndApplyHardDrives()`, `diffAndApplyDVDDrives()`, `diffAndApplyNetworkAdapters()`.

### Cloud-Init / ISO

The `hyperv_iso` resource creates NoCloud seed ISOs with `meta-data`, `user-data`, and optional `network-config`. Mounted via `hyperv_dvd_drive`. Volume label must be `cidata`. See `docs/cloud-init.md` for the full guide covering Linux cloud-init and Windows cloudbase-init.

## Hyper-V Platform Behavior

- Dynamic MAC addresses are assigned when the VM **starts**, not when the adapter is created. Off VMs will show MAC `000000000000` — this is expected, not a bug.
- Gen2 VMs use SCSI controllers and support Secure Boot. Gen1 VMs use IDE (max 4 devices) and have no Secure Boot.
- **`Set-VMFirmware` resets sibling settings.** Each call to `Set-VMFirmware` resets any settings not included in that call. For example, `-FirstBootDevice` resets Secure Boot back to default, and `-EnableSecureBoot Off` resets the boot order. Always combine all firmware settings (boot device, secure boot, template) into a **single** `Set-VMFirmware` call. See `buildSetVMFirmwareCommand()` in `internal/client/vm_firmware.go`.
- PowerShell commands executed remotely may wrap single objects in arrays — the client's `RunJSON` handles this with array unwrapping.
- Hyper-V has transient lock conflicts during concurrent operations — the client retries these automatically (3 attempts, 3s delay).

## WinRM Gotchas

- **Deserialized types break `-is` checks.** Objects returned over WinRM are deserialized (e.g., `Deserialized.Microsoft.HyperV.PowerShell.HardDiskDrive`), so `-is [Microsoft.HyperV.PowerShell.HardDiskDrive]` returns `$false`. Use `$obj.PSObject.TypeNames -like '*HardDiskDrive*'` or match by `Id`/properties instead.
- **Plan value preservation.** When a value was just set by the provider (e.g., `first_boot_device` during Create), prefer preserving the planned value in state rather than relying on a read-back that may fail due to WinRM deserialization. See the `first_boot_device` handling in Create and Read in `vm.go`.

## Conventions

- Use `EscapePSString()` from `internal/client/powershell.go` for all user-provided strings in PowerShell commands (prevents injection).
- Use `UseStateForUnknown()` plan modifier on computed attributes to prevent unnecessary drift.
- Use `RequiresReplace()` plan modifier on immutable fields that require resource recreation.
- Acceptance tests use a `.env` file with: `HYPERV_HOST`, `HYPERV_USER`, `HYPERV_PASSWORD` (or `HYPERV_PASSWORD_FILE`), `HYPERV_BASE_IMAGE_LINUX`, `HYPERV_BASE_IMAGE_WINDOWS`, `HYPERV_GUEST_USER`, `HYPERV_GUEST_PASSWORD`. Load with `set -a && source .env && set +a` to export. Use `testAccPreCheck(t)` and `testAccProviderConfig()` helpers from `internal/provider/acc_test.go`.
