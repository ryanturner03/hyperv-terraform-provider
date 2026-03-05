terraform {
  required_providers {
    hyperv = {
      source = "ryan/hyperv"
    }
  }
}

provider "hyperv" {
  host      = "192.168.1.202"
  port      = 5985
  use_tls   = false
  auth_type = "ntlm"
  username  = "packer"
  password  = var.hyperv_password
}

variable "hyperv_password" {
  type      = string
  sensitive = true
}

resource "hyperv_vhd" "os" {
  path        = "C:\\VMs\\tf-acc-inline\\os.vhdx"
  type        = "Differencing"
  parent_path = "C:\\VMs\\base-images\\debian-12-gen2.vhdx"
}

resource "hyperv_vm" "test" {
  name                 = "tf-acc-inline"
  generation           = 2
  processor_count      = 2
  memory_startup_bytes = 1073741824 # 1GB
  state                = "Off"

  hard_drive {
    path                = hyperv_vhd.os.path
    controller_type     = "SCSI"
    controller_number   = 0
    controller_location = 0
  }

  network_adapter {
    name        = "Network Adapter"
    switch_name = "Default Switch"
  }
}
