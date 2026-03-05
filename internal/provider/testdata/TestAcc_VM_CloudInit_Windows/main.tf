variable "host" {
  type = string
}

variable "user" {
  type = string
}

variable "password" {
  type      = string
  sensitive = true
}

variable "base_image" {
  type = string
}

variable "guest_user" {
  type = string
}

variable "guest_password" {
  type      = string
  sensitive = true
}

provider "hyperv" {
  host      = var.host
  port      = 5985
  use_tls   = false
  insecure  = true
  auth_type = "ntlm"
  username  = var.user
  password  = var.password
}

resource "hyperv_vhd" "os" {
  path        = "C:\\VMs\\tf-acc-cloudinit-win\\os.vhdx"
  type        = "Differencing"
  parent_path = var.base_image
}

resource "hyperv_iso" "seed" {
  depends_on   = [hyperv_vhd.os]
  path         = "C:\\VMs\\tf-acc-cloudinit-win\\seed.iso"
  volume_label = "cidata"
  files = {
    "meta-data" = "instance-id: tf-acc-cloudinit-win\n"
    "user-data" = <<-EOF
      #cloud-config
      set_hostname: tf-acc-ci-win
      users:
        - name: ${var.guest_user}
          passwd: ${var.guest_password}
      runcmd:
        - powershell -Command "Set-Content -Path C:\cloudbase-init-ok -Value 'tf-acc-test-marker'"
    EOF
  }
}

resource "hyperv_vm" "test" {
  name                 = "tf-acc-cloudinit-win"
  generation           = 2
  processor_count      = 4
  memory_startup_bytes = 4294967296
  secure_boot_enabled  = true
  state                = "Running"

  first_boot_device = {
    device_type         = "HardDiskDrive"
    controller_number   = 0
    controller_location = 0
  }

  hard_drive {
    path                = hyperv_vhd.os.path
    controller_type     = "SCSI"
    controller_number   = 0
    controller_location = 0
  }

  dvd_drive {
    path                = hyperv_iso.seed.path
    controller_number   = 0
    controller_location = 1
  }

  network_adapter {
    name                = "Network Adapter"
    switch_name         = "Default Switch"
    dynamic_mac_address = true
  }
}
