# VM Initialization on Hyper-V with Terraform

This guide covers deploying Linux and Windows VMs with automatic guest configuration on Hyper-V using this provider. Both operating systems use the same mechanism — a seed ISO with volume label `cidata` containing `meta-data`, `user-data`, and optionally `network-config`:

- **Linux** uses [cloud-init](https://cloudinit.readthedocs.io/en/latest/reference/datasources/nocloud.html) with the NoCloud data source
- **Windows** uses [cloudbase-init](https://cloudbase-init.readthedocs.io/en/latest/) with the NoCloud data source

## Workflow

1. **Packer** builds a base VHDX image with the guest agent pre-installed (cloud-init for Linux, cloudbase-init for Windows)
2. **Terraform** creates a differencing disk from the base image, attaches it to a VM, and mounts a seed ISO
3. On first boot, the guest agent reads the seed ISO and configures the VM (hostname, users, network, etc.)

## Image Requirements

Base images must have the guest initialization agent installed and configured for the NoCloud data source.

### Linux (cloud-init)

- cloud-init installed with `datasource_list: [NoCloud, ConfigDrive, None]`
- Hyper-V guest tools installed (`linux-cloud-tools-common`, `linux-tools-virtual`)
- cloud-init state cleaned so it runs fresh on each clone

> **Do NOT use Azure-specific cloud images** (e.g., `noble-server-cloudimg-amd64-azure.vhd.tar.gz`). These have cloud-init locked to `datasource_list: [Azure]` and will never detect the NoCloud `cidata` ISO.

### Windows (cloudbase-init)

- [Cloudbase-init](https://cloudbase.it/cloudbase-init/) installed
- Configured for the NoCloud data source in `cloudbase-init.conf`:

```ini
[DEFAULT]
metadata_services=cloudbaseinit.metadata.services.nocloudservice.NoCloudConfigDriveService
config_drive_cdrom=true
config_drive_raw_hhd=true
config_drive_vfat=true
plugins=cloudbaseinit.plugins.common.sethostname.SetHostNamePlugin,cloudbaseinit.plugins.common.networkconfig.NetworkConfigPlugin,cloudbaseinit.plugins.common.userdata.UserDataPlugin
```

The `NetworkConfigPlugin` is required for static IP assignment. Without it, cloudbase-init will ignore the `network-config` file in the seed ISO.

## Generation 1 vs Generation 2 VMs

| | Gen 1 | Gen 2 |
|---|---|---|
| Firmware | BIOS (legacy) | UEFI |
| Controllers | IDE + SCSI | SCSI only |
| OS disk | IDE (required for boot) | SCSI |
| DVD drive | IDE | SCSI |
| Boot config | `Set-VMBios` | `Set-VMFirmware` |
| Secure Boot | N/A | On by default — must disable for Linux |
| Max IDE devices | 4 (2 controllers x 2 locations) | N/A |
| Max SCSI devices | 64 per controller, 4 controllers | 64 per controller, 4 controllers |
| Packer base image | `ubuntu-2404-gen1.vhdx` | `ubuntu-2404-gen2.vhdx` |

- **Gen 1**: Simpler setup. No Secure Boot to deal with. OS disk must be on IDE.
- **Gen 2**: Required for UEFI, Secure Boot, larger disks (>2TB boot), and PXE boot from SCSI. SCSI-only.

## Terraform Configuration

### Linux — Gen 2 with Static IP

```hcl
resource "hyperv_vhd" "linux_os" {
  path        = "C:\\VMs\\my-linux-server\\os.vhdx"
  type        = "Differencing"
  parent_path = "C:\\VMs\\base-images\\ubuntu-2404-gen2.vhdx"
}

resource "hyperv_iso" "linux_seed" {
  path         = "C:\\VMs\\my-linux-server\\seed.iso"
  volume_label = "cidata"
  files = {
    "meta-data" = <<-EOF
      instance-id: my-linux-server
      local-hostname: my-linux-server
    EOF
    "user-data" = <<-EOF
      #cloud-config
      hostname: my-linux-server
      users:
        - name: admin
          sudo: ALL=(ALL) NOPASSWD:ALL
          ssh_authorized_keys:
            - ssh-ed25519 AAAA... your-key
    EOF
    "network-config" = <<-EOF
      version: 2
      ethernets:
        eth0:
          addresses:
            - 192.168.1.50/24
          routes:
            - to: default
              via: 192.168.1.1
          nameservers:
            addresses:
              - 8.8.8.8
              - 8.8.4.4
    EOF
  }
}

resource "hyperv_vm" "linux" {
  name                 = "my-linux-server"
  generation           = 2
  processor_count      = 4
  memory_startup_bytes = 2147483648  # 2GB
  dynamic_memory       = false
  state                = "Running"
  secure_boot_enabled  = false  # Required for Linux

  first_boot_device = {
    device_type         = "HardDiskDrive"
    controller_number   = 0
    controller_location = 0
  }

  hard_drive {
    path                = hyperv_vhd.linux_os.path
    controller_type     = "SCSI"
    controller_number   = 0
    controller_location = 0
  }

  dvd_drive {
    path                = hyperv_iso.linux_seed.path
    controller_number   = 0
    controller_location = 1
  }

  network_adapter {
    name        = "Network Adapter"
    switch_name = "LAN"
  }
}
```

### Windows — Gen 2 with Static IP

```hcl
resource "hyperv_vhd" "windows_os" {
  path        = "C:\\VMs\\my-windows-server\\os.vhdx"
  type        = "Differencing"
  parent_path = "C:\\VMs\\base-images\\windows-server-2022-gen2.vhdx"
}

resource "hyperv_iso" "windows_seed" {
  path         = "C:\\VMs\\my-windows-server\\seed.iso"
  volume_label = "cidata"
  files = {
    "meta-data" = <<-EOF
      instance-id: my-windows-server
      local-hostname: my-windows-server
    EOF
    "user-data" = <<-EOF
      #cloud-config
      set_hostname: my-windows-server
      users:
        - name: Administrator
          passwd: YourSecurePassword123
      runcmd:
        - powershell -Command "Write-Output 'cloudbase-init complete'"
    EOF
    "network-config" = <<-EOF
      version: 2
      ethernets:
        Ethernet:
          addresses:
            - 192.168.1.51/24
          routes:
            - to: default
              via: 192.168.1.1
          nameservers:
            addresses:
              - 8.8.8.8
              - 8.8.4.4
    EOF
  }
}

resource "hyperv_vm" "windows" {
  name                 = "my-windows-server"
  generation           = 2
  processor_count      = 4
  memory_startup_bytes = 4294967296  # 4GB
  dynamic_memory       = false
  state                = "Running"
  secure_boot_enabled  = true

  first_boot_device = {
    device_type         = "HardDiskDrive"
    controller_number   = 0
    controller_location = 0
  }

  hard_drive {
    path                = hyperv_vhd.windows_os.path
    controller_type     = "SCSI"
    controller_number   = 0
    controller_location = 0
  }

  dvd_drive {
    path                = hyperv_iso.windows_seed.path
    controller_number   = 0
    controller_location = 1
  }

  network_adapter {
    name        = "Network Adapter"
    switch_name = "LAN"
  }
}
```

### Linux — Gen 1

```hcl
resource "hyperv_vm" "server" {
  name                 = "my-server"
  generation           = 1
  processor_count      = 4
  memory_startup_bytes = 2147483648  # 2GB
  dynamic_memory       = false
  state                = "Off"
}

resource "hyperv_vhd" "os_disk" {
  path        = "C:\\VMs\\my-server\\os.vhdx"
  type        = "Differencing"
  parent_path = "C:\\VMs\\base-images\\ubuntu-2404-gen1.vhdx"
}

# OS disk on IDE controller 0 (required for Gen 1 boot)
resource "hyperv_hard_drive" "os_disk" {
  vm_name             = hyperv_vm.server.name
  controller_type     = "IDE"
  controller_number   = 0
  controller_location = 0
  path                = hyperv_vhd.os_disk.path
}

resource "hyperv_iso" "cloudinit" {
  path         = "C:\\VMs\\my-server\\seed.iso"
  volume_label = "cidata"
  files = {
    "meta-data" = <<-EOF
      instance-id: my-server
      local-hostname: my-server
    EOF
    "user-data" = <<-EOF
      #cloud-config
      hostname: my-server
      users:
        - name: admin
          sudo: ALL=(ALL) NOPASSWD:ALL
          ssh_authorized_keys:
            - ssh-ed25519 AAAA... your-key
    EOF
    "network-config" = <<-EOF
      version: 2
      ethernets:
        eth0:
          addresses:
            - 192.168.1.50/24
          routes:
            - to: default
              via: 192.168.1.1
          nameservers:
            addresses:
              - 8.8.8.8
              - 8.8.4.4
    EOF
  }
}

# Seed ISO on IDE controller 1
resource "hyperv_dvd_drive" "cloudinit" {
  vm_name           = hyperv_vm.server.name
  controller_number = 1
  path              = hyperv_iso.cloudinit.path
}
```

## IDE Controller Layout (Gen 1)

Gen 1 VMs have exactly 2 IDE controllers, each with 2 locations:

| Controller | Location | Typical Use |
|------------|----------|-------------|
| 0 | 0 | OS disk (first boot device) |
| 0 | 1 | Data disk or DVD drive |
| 1 | 0 | DVD drive (cloud-init ISO) |
| 1 | 1 | Additional device |

This limits Gen 1 VMs to 4 IDE devices total. For additional disks, use SCSI:

```hcl
resource "hyperv_hard_drive" "data_disk" {
  vm_name         = hyperv_vm.server.name
  controller_type = "SCSI"
  path            = hyperv_vhd.data_disk.path
  # controller_number and controller_location auto-assigned
}
```

> **Note:** Gen 1 VMs cannot boot from SCSI. The OS disk must be on IDE.

## Seed ISO Requirements

The `hyperv_iso` resource creates the seed ISO. For the guest agent to detect it:

1. **Volume label must be `cidata`** — both cloud-init and cloudbase-init look for this label.
2. **Required files:**
   - `meta-data` — instance identity (at minimum: `instance-id` and `local-hostname`)
   - `user-data` — guest configuration (see format notes below)
3. **Optional files:**
   - `network-config` — network configuration in [Netplan v2 format](https://netplan.readthedocs.io/en/stable/netplan-yaml/)

### user-data format

| OS | Format | Example |
|----|--------|---------|
| Linux | cloud-config YAML | Must start with `#cloud-config` |
| Windows | cloud-config YAML | Must start with `#cloud-config` — uses `set_hostname`, `users`, `runcmd` plugins |
| Windows (alt) | PowerShell script | Start with `#ps1_sysnative` for raw PowerShell execution |

> **Recommended:** Use `#cloud-config` YAML for both Linux and Windows. The `#ps1_sysnative` format works but bypasses cloudbase-init's plugin system (hostname, user management, etc.).

### network-config format

Both cloud-init and cloudbase-init support Netplan v2 format:

```yaml
version: 2
ethernets:
  eth0:                       # Linux interface name
    addresses:
      - 192.168.1.50/24
    routes:
      - to: default
        via: 192.168.1.1
    nameservers:
      addresses:
        - 8.8.8.8
        - 8.8.4.4
```

**Interface naming:** Linux uses `eth0` (or `ens*` depending on the distro). Windows uses the adapter name as shown in `Get-NetAdapter` — typically `Ethernet`. Use the name that matches your base image.

## Troubleshooting

### Guest agent doesn't detect the seed ISO
- **Check the data source config.** Linux images need `NoCloud` in `datasource_list`. Windows images need `NoCloudConfigDriveService` in `metadata_services`. Azure-specific Linux images will NOT work.
- Verify volume label is exactly `cidata`.
- On Gen 1, the DVD drive should be on IDE (not SCSI) for detection during early boot.
- On Gen 2, the DVD drive should be on SCSI controller 0.

### Static IP not applied (Linux)
- Verify `user-data` starts with `#cloud-config` (no leading whitespace or BOM).
- Check `meta-data` has a unique `instance-id`. Cloud-init skips re-runs if the instance-id matches a previous run. Change it to force re-initialization.
- Verify the `network-config` interface name matches the OS (check `ip link` inside the VM).

### Static IP not applied (Windows)
- Verify `NetworkConfigPlugin` is in the cloudbase-init plugins list. Without it, the `network-config` file is ignored.
- Verify `config_drive_cdrom=true` is set in `cloudbase-init.conf`.
- Check the interface name in `network-config` matches `Get-NetAdapter` output (typically `Ethernet`).
- Cloudbase-init logs: `C:\Program Files\Cloudbase Solutions\Cloudbase-Init\log\cloudbase-init.log`

### VM doesn't boot (Gen 1)
- Verify boot order: `Get-VMBios -VMName "my-server"` — IDE should be first.

### VM doesn't boot (Gen 2)
- **Linux:** Disable Secure Boot: `Set-VMFirmware -VMName "my-server" -EnableSecureBoot Off`
- **Windows:** Ensure Secure Boot template is `MicrosoftWindows`.
- Check boot device: `Get-VMFirmware -VMName "my-server"` — the OS disk should be first.
- Verify the base image has a GPT partition table with an EFI System Partition.

### Verifying initialization

**Linux:**
```bash
cloud-init status --long
cat /run/cloud-init/result.json
cat /var/log/cloud-init-output.log
```

**Windows:**
```powershell
Get-Content "C:\Program Files\Cloudbase Solutions\Cloudbase-Init\log\cloudbase-init.log"
Get-NetIPAddress -InterfaceAlias Ethernet
```
