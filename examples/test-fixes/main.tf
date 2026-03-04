# Test config for issues #4/#5/#6/#7/#8
#
# Prerequisites:
#   - A Hyper-V host reachable via WinRM
#   - A virtual switch named "vSwitch" (or change the variable below)
#   - The provider binary installed locally
#
# Usage:
#   export TF_VAR_host="192.168.1.100"
#   export TF_VAR_username="Administrator"
#   export TF_VAR_password="YourPassword"
#   terraform init
#   terraform apply
#
# What to verify:
#   1. mac_address output is a real MAC (not 000000000000)     — issue #8
#   2. No "inconsistent result after apply" error              — issue #7
#   3. Warning: "First boot device not set — drive not attached yet" — issue #4/#5
#   4. On second apply, first_boot_device is set (drive exists now)
#   5. terraform destroy cleans up everything

terraform {
  required_providers {
    hyperv = {
      source = "ryan/hyperv"
    }
  }
}

variable "host" {
  type        = string
  description = "Hyper-V host address"
}

variable "username" {
  type    = string
  default = "Administrator"
}

variable "password" {
  type      = string
  sensitive = true
}

variable "switch_name" {
  type        = string
  default     = "vSwitch"
  description = "Existing virtual switch to connect the NIC to"
}

provider "hyperv" {
  host      = var.host
  auth_type = "ntlm"
  username  = var.username
  password  = var.password
}

# --- Test 1: MAC address readback (issue #8) ---

resource "hyperv_vm" "mac_test" {
  name                 = "tf-mac-test"
  generation           = 2
  processor_count      = 1
  memory_startup_bytes = 536870912 # 512MB
  state                = "Off"
}

resource "hyperv_network_adapter" "mac_test" {
  vm_name             = hyperv_vm.mac_test.name
  name                = "TestNIC"
  switch_name         = var.switch_name
  dynamic_mac_address = true
}

output "mac_address" {
  description = "Should be a real MAC like 00155DXXXXXX, NOT 000000000000"
  value       = hyperv_network_adapter.mac_test.mac_address
}

# --- Test 2: first_boot_device deferred + state preserved (issues #4/#5/#7) ---

resource "hyperv_vhd" "boot_disk" {
  path       = "C:\\VMs\\tf-boot-test\\os.vhdx"
  size_bytes = 1073741824 # 1GB
  type       = "Dynamic"
}

resource "hyperv_vm" "boot_test" {
  name                 = "tf-boot-test"
  generation           = 2
  processor_count      = 1
  memory_startup_bytes = 536870912
  state                = "Off"

  # This references controller 0:0, but the hard drive below hasn't been
  # created yet on first apply. The provider should:
  #   - Warn (not error) about the drive not being attached yet
  #   - Preserve first_boot_device in state (no "inconsistent result" error)
  #   - Set the boot device on the second terraform apply
  first_boot_device {
    device_type         = "HardDiskDrive"
    controller_number   = 0
    controller_location = 0
  }
}

resource "hyperv_hard_drive" "boot_disk" {
  vm_name             = hyperv_vm.boot_test.name
  controller_type     = "SCSI"
  controller_number   = 0
  controller_location = 0
  path                = hyperv_vhd.boot_disk.path
}
