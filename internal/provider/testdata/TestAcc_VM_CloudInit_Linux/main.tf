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
  path        = "C:\\VMs\\tf-acc-cloudinit\\os.vhdx"
  type        = "Differencing"
  parent_path = var.base_image
}

resource "hyperv_iso" "seed" {
  depends_on   = [hyperv_vhd.os]
  path         = "C:\\VMs\\tf-acc-cloudinit\\seed.iso"
  volume_label = "cidata"
  files = {
    "meta-data" = "instance-id: tf-acc-cloudinit\nlocal-hostname: tf-acc-cloudinit\n"
    "user-data" = <<-EOF
      #cloud-config
      hostname: tf-acc-cloudinit
      ssh_pwauth: true
      users:
        - name: ${var.guest_user}
          plain_text_passwd: ${var.guest_password}
          lock_passwd: false
          shell: /bin/bash
    EOF
  }
}

resource "hyperv_vm" "test" {
  name                 = "tf-acc-cloudinit"
  generation           = 2
  processor_count      = 2
  memory_startup_bytes = 1073741824
  secure_boot_enabled  = false
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
