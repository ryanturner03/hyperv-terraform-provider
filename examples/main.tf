terraform {
  required_providers {
    hyperv = {
      source = "ryan/hyperv"
    }
  }
}

# Domain-joined host with Kerberos
provider "hyperv" {
  alias     = "dc_host"
  host      = "hyperv01.domain.local"
  auth_type = "kerberos"
  realm     = "DOMAIN.LOCAL"
}

# Standalone host with NTLM
provider "hyperv" {
  alias     = "lab_host"
  host      = "192.168.1.100"
  auth_type = "ntlm"
  username  = "Administrator"
  password  = var.lab_password
}

variable "lab_password" {
  type      = string
  sensitive = true
}

# Create a virtual switch
resource "hyperv_virtual_switch" "internal" {
  provider = hyperv.lab_host
  name     = "InternalSwitch"
  type     = "Internal"
}

# Create a VHD
resource "hyperv_vhd" "os_disk" {
  provider   = hyperv.lab_host
  path       = "C:\\VMs\\web-server\\os.vhdx"
  size_bytes = 53687091200 # 50GB
  type       = "Dynamic"
}

# Create a VM
resource "hyperv_vm" "web" {
  provider             = hyperv.lab_host
  name                 = "web-server"
  generation           = 2
  processor_count      = 4
  memory_startup_bytes = 4294967296 # 4GB
  dynamic_memory       = true
  memory_minimum_bytes = 2147483648 # 2GB
  memory_maximum_bytes = 8589934592 # 8GB
  state                = "Running"
}

# Attach a network adapter
resource "hyperv_network_adapter" "web_nic" {
  provider    = hyperv.lab_host
  name        = "Primary"
  vm_name     = hyperv_vm.web.name
  switch_name = hyperv_virtual_switch.internal.name
}

# Look up an existing VM
data "hyperv_vm" "existing" {
  provider = hyperv.dc_host
  name     = "domain-controller"
}
