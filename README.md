# Hyper-V Terraform Provider

A Terraform provider for managing Hyper-V resources (VMs, VHDs, virtual switches, network adapters) across one or more Windows Hyper-V hosts via WinRM.

## Features

- **Virtual Machines** - Create, configure, and manage VM lifecycle including power state, processors, memory (static or dynamic), and checkpoints
- **Virtual Hard Disks** - Create Dynamic, Fixed, or Differencing VHDs/VHDXs
- **Hard Drives** - Attach VHD/VHDX disks to VMs on IDE or SCSI controllers
- **ISO Images** - Create ISO 9660 images (e.g., cloud-init seed ISOs) via IMAPI2
- **DVD Drives** - Mount ISO images in VM DVD drives
- **Virtual Switches** - Create External, Internal, or Private virtual switches
- **Network Adapters** - Attach and configure VM network adapters with VLAN support
- **Guest Initialization** - Deploy Linux (cloud-init) and Windows (cloudbase-init) VMs with static IP configuration via [NoCloud seed ISOs](docs/cloud-init.md)
- **Data Sources** - Read-only lookup for all resource types
- **Multi-Host** - Manage resources across multiple Hyper-V hosts using provider aliases
- **Authentication** - Kerberos, NTLM, and Basic auth over WinRM (HTTP or HTTPS)

## Requirements

- Terraform >= 1.0
- Go >= 1.23 (for building)
- Windows Hyper-V host with WinRM enabled

### WinRM Host Setup

On each Hyper-V host, run in an elevated PowerShell:

```powershell
Enable-PSRemoting -Force -SkipNetworkProfileCheck

# For HTTP (non-TLS) connections:
winrm set winrm/config/service '@{AllowUnencrypted="true"}'
winrm set winrm/config/service/auth '@{Basic="true"}'

Restart-Service WinRM
```

## Usage

```hcl
terraform {
  required_providers {
    hyperv = {
      source = "ryan/hyperv"
    }
  }
}

provider "hyperv" {
  host      = "192.168.1.100"
  port      = 5985
  use_tls   = false
  auth_type = "ntlm"
  username  = "Administrator"
  password  = var.hyperv_password
}

variable "hyperv_password" {
  type      = string
  sensitive = true
}

resource "hyperv_virtual_switch" "internal" {
  name = "InternalSwitch"
  type = "Internal"
}

resource "hyperv_vm" "web" {
  name                 = "web-server"
  generation           = 2
  processor_count      = 4
  memory_startup_bytes = 4294967296 # 4GB
  dynamic_memory       = true
  memory_minimum_bytes = 2147483648 # 2GB
  memory_maximum_bytes = 8589934592 # 8GB
  state                = "Running"
}

resource "hyperv_vhd" "os_disk" {
  path       = "C:\\VMs\\web-server\\os.vhdx"
  size_bytes = 53687091200 # 50GB
  type       = "Dynamic"
}

resource "hyperv_network_adapter" "web_nic" {
  name        = "Primary"
  vm_name     = hyperv_vm.web.name
  switch_name = hyperv_virtual_switch.internal.name
}
```

## Provider Configuration

| Attribute   | Required | Default | Description                                      |
|-------------|----------|---------|--------------------------------------------------|
| `host`      | Yes      |         | Hyper-V host FQDN or IP address                  |
| `port`      | No       | 5986    | WinRM port                                       |
| `use_tls`   | No       | true    | Use HTTPS for WinRM                              |
| `insecure`  | No       | false   | Skip TLS certificate verification                |
| `timeout`   | No       | "30s"   | WinRM operation timeout                          |
| `auth_type` | Yes      |         | Authentication method: `kerberos`, `ntlm`, `basic` |
| `username`  | No       |         | Username (required for ntlm/basic)               |
| `password`  | No       |         | Password (required for ntlm/basic, or set `HYPERV_PASSWORD` env var) |
| `realm`     | No       |         | Kerberos realm (required for kerberos)            |

## Multi-Host Management

Use provider aliases to manage resources across multiple hosts:

```hcl
provider "hyperv" {
  alias     = "dc_host"
  host      = "hyperv01.domain.local"
  auth_type = "kerberos"
  realm     = "DOMAIN.LOCAL"
}

provider "hyperv" {
  alias     = "lab_host"
  host      = "192.168.1.100"
  auth_type = "ntlm"
  username  = "Administrator"
  password  = var.lab_password
}

resource "hyperv_vm" "dc_vm" {
  provider = hyperv.dc_host
  name     = "new-dc"
  # ...
}

resource "hyperv_vm" "lab_vm" {
  provider = hyperv.lab_host
  name     = "test-vm"
  # ...
}
```

## Building

```bash
go build -o terraform-provider-hyperv .
```

For local development, add a dev override to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "ryan/hyperv" = "/path/to/hyperv-terraform-provider"
  }
  direct {}
}
```
