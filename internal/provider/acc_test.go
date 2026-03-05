//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"hyperv": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, env := range []string{"HYPERV_HOST", "HYPERV_USER"} {
		if os.Getenv(env) == "" {
			t.Fatalf("%s must be set for acceptance tests", env)
		}
	}
	if os.Getenv("HYPERV_PASSWORD") == "" && os.Getenv("HYPERV_PASSWORD_FILE") == "" {
		t.Fatal("HYPERV_PASSWORD or HYPERV_PASSWORD_FILE must be set for acceptance tests")
	}
}

func testAccPassword() string {
	if p := os.Getenv("HYPERV_PASSWORD"); p != "" {
		return p
	}
	return readPasswordFile(os.Getenv("HYPERV_PASSWORD_FILE"))
}

func testAccProviderVars() config.Variables {
	return config.Variables{
		"host":     config.StringVariable(os.Getenv("HYPERV_HOST")),
		"user":     config.StringVariable(os.Getenv("HYPERV_USER")),
		"password": config.StringVariable(testAccPassword()),
	}
}

func testAccGuestCredentials() (string, string) {
	user := os.Getenv("HYPERV_GUEST_USER")
	if user == "" {
		user = "test"
	}
	password := os.Getenv("HYPERV_GUEST_PASSWORD")
	if password == "" {
		password = "T3stPass!"
	}
	return user, password
}

func testAccWinRMClient(t *testing.T) *client.WinRMClient {
	t.Helper()
	c, err := client.NewWinRMClient(client.ConnectionConfig{
		Host:     os.Getenv("HYPERV_HOST"),
		Port:     5985,
		UseTLS:   false,
		Insecure: true,
		AuthType: client.AuthNTLM,
		Username: os.Getenv("HYPERV_USER"),
		Password: testAccPassword(),
		Timeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("creating WinRM client: %s", err)
	}
	return c
}

// waitForVMIP polls Hyper-V for a VM's IPv4 address via integration services.
func waitForVMIP(ctx context.Context, ps client.PSExecutor, vmName string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	cmd := fmt.Sprintf(
		`(Get-VMNetworkAdapter -VMName %s -ErrorAction Stop).IPAddresses | Where-Object { $_ -match '^\d+\.\d+\.\d+\.\d+$' } | Select-Object -First 1`,
		client.EscapePSString(vmName),
	)
	for time.Now().Before(deadline) {
		stdout, _, err := ps.Run(ctx, cmd)
		if err == nil && stdout != "" {
			return strings.TrimSpace(stdout), nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return "", fmt.Errorf("timed out waiting for VM %q to get an IP address", vmName)
}

// sshHostname connects via SSH and runs hostname.
func sshHostname(addr, user, password string) (string, error) {
	cfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	conn, err := ssh.Dial("tcp", net.JoinHostPort(addr, "22"), cfg)
	if err != nil {
		return "", fmt.Errorf("ssh dial: %w", err)
	}
	defer conn.Close()

	sess, err := conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session: %w", err)
	}
	defer sess.Close()

	out, err := sess.Output("hostname")
	if err != nil {
		return "", fmt.Errorf("ssh hostname: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// psDirectHostname runs 'hostname' inside a VM via PowerShell Direct, polling until
// the guest responds or the timeout expires.
func psDirectHostname(ctx context.Context, t *testing.T, ps client.PSExecutor, vmName, guestUser, guestPassword string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	cmd := fmt.Sprintf(
		`$pw = ConvertTo-SecureString %s -AsPlainText -Force; `+
			`$cred = New-Object System.Management.Automation.PSCredential(%s, $pw); `+
			`Invoke-Command -VMName %s -Credential $cred -ScriptBlock { hostname } -ErrorAction Stop`,
		client.EscapePSString(guestPassword),
		client.EscapePSString(guestUser),
		client.EscapePSString(vmName),
	)
	var lastErr error
	for time.Now().Before(deadline) {
		stdout, _, err := ps.Run(ctx, cmd)
		if err == nil && stdout != "" {
			return strings.TrimSpace(stdout), nil
		}
		if err != nil {
			lastErr = err
			t.Logf("PS Direct not ready yet: %s", err)
		}
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return "", fmt.Errorf("context expired waiting for PS Direct on VM %q: %w", vmName, lastErr)
			}
			return "", ctx.Err()
		case <-time.After(15 * time.Second):
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("timed out waiting for PS Direct on VM %q: %w", vmName, lastErr)
	}
	return "", fmt.Errorf("timed out waiting for PS Direct on VM %q", vmName)
}

func TestAcc_VM_Basic(t *testing.T) {
	testAccPreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "hyperv_vm" "test" {
  name             = "tf-acc-basic"
  generation       = 2
  processor_count  = 2
  memory_startup_bytes = 1073741824
  state            = "Off"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hyperv_vm.test", "name", "tf-acc-basic"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "generation", "2"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "processor_count", "2"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "memory_startup_bytes", "1073741824"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "state", "Off"),
				),
			},
		},
	})
}

func TestAcc_VM_CloudInit_Linux(t *testing.T) {
	testAccPreCheck(t)

	baseImage := os.Getenv("HYPERV_BASE_IMAGE_LINUX")
	if baseImage == "" {
		t.Fatal("HYPERV_BASE_IMAGE_LINUX must be set for this test")
	}

	guestUser, guestPassword := testAccGuestCredentials()
	winrm := testAccWinRMClient(t)

	vars := testAccProviderVars()
	vars["base_image"] = config.StringVariable(baseImage)
	vars["guest_user"] = config.StringVariable(guestUser)
	vars["guest_password"] = config.StringVariable(guestPassword)

	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ConfigDirectory:          config.TestNameDirectory(),
				ConfigVariables:          vars,
				Check: resource.ComposeAggregateTestCheckFunc(
					// VHD
					resource.TestCheckResourceAttr("hyperv_vhd.os", "path", `C:\VMs\tf-acc-cloudinit\os.vhdx`),
					resource.TestCheckResourceAttr("hyperv_vhd.os", "type", "Differencing"),
					resource.TestCheckResourceAttr("hyperv_vhd.os", "parent_path", baseImage),

					// ISO
					resource.TestCheckResourceAttr("hyperv_iso.seed", "path", `C:\VMs\tf-acc-cloudinit\seed.iso`),
					resource.TestCheckResourceAttr("hyperv_iso.seed", "volume_label", "cidata"),
					resource.TestCheckResourceAttrSet("hyperv_iso.seed", "content_hash"),

					// VM core
					resource.TestCheckResourceAttr("hyperv_vm.test", "name", "tf-acc-cloudinit"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "generation", "2"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "state", "Running"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "processor_count", "2"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "secure_boot_enabled", "false"),

					// first_boot_device
					resource.TestCheckResourceAttr("hyperv_vm.test", "first_boot_device.device_type", "HardDiskDrive"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "first_boot_device.controller_number", "0"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "first_boot_device.controller_location", "0"),

					// inline hard_drive
					resource.TestCheckResourceAttr("hyperv_vm.test", "hard_drive.0.path", `C:\VMs\tf-acc-cloudinit\os.vhdx`),
					resource.TestCheckResourceAttr("hyperv_vm.test", "hard_drive.0.controller_type", "SCSI"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "hard_drive.0.controller_number", "0"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "hard_drive.0.controller_location", "0"),

					// inline dvd_drive
					resource.TestCheckResourceAttr("hyperv_vm.test", "dvd_drive.0.path", `C:\VMs\tf-acc-cloudinit\seed.iso`),
					resource.TestCheckResourceAttr("hyperv_vm.test", "dvd_drive.0.controller_number", "0"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "dvd_drive.0.controller_location", "1"),

					// inline network_adapter
					resource.TestCheckResourceAttr("hyperv_vm.test", "network_adapter.0.name", "Network Adapter"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "network_adapter.0.switch_name", "Default Switch"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "network_adapter.0.dynamic_mac_address", "true"),

					// SSH into the VM and verify cloud-init set the hostname
					func(_ *terraform.State) error {
						ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
						defer cancel()

						t.Log("Waiting for VM to get an IP address...")
						ip, err := waitForVMIP(ctx, winrm.PS(), "tf-acc-cloudinit", 3*time.Minute)
						if err != nil {
							return err
						}
						t.Logf("VM IP: %s", ip)

						// Retry SSH — cloud-init may still be running
						var hostname string
						deadline := time.Now().Add(2 * time.Minute)
						for time.Now().Before(deadline) {
							hostname, err = sshHostname(ip, guestUser, guestPassword)
							if err == nil {
								break
							}
							t.Logf("SSH not ready yet: %s", err)
							time.Sleep(10 * time.Second)
						}
						if err != nil {
							return fmt.Errorf("SSH failed after retries: %w", err)
						}

						t.Logf("Hostname: %s", hostname)
						if hostname != "tf-acc-cloudinit" {
							return fmt.Errorf("expected hostname %q, got %q", "tf-acc-cloudinit", hostname)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAcc_VM_CloudInit_Windows(t *testing.T) {
	testAccPreCheck(t)

	baseImage := os.Getenv("HYPERV_BASE_IMAGE_WINDOWS")
	if baseImage == "" {
		t.Fatal("HYPERV_BASE_IMAGE_WINDOWS must be set for this test")
	}

	guestUser, guestPassword := testAccGuestCredentials()
	winrm := testAccWinRMClient(t)

	vars := testAccProviderVars()
	vars["base_image"] = config.StringVariable(baseImage)
	vars["guest_user"] = config.StringVariable(guestUser)
	vars["guest_password"] = config.StringVariable(guestPassword)

	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ConfigDirectory:          config.TestNameDirectory(),
				ConfigVariables:          vars,
				Check: resource.ComposeAggregateTestCheckFunc(
					// VHD
					resource.TestCheckResourceAttr("hyperv_vhd.os", "path", `C:\VMs\tf-acc-cloudinit-win\os.vhdx`),
					resource.TestCheckResourceAttr("hyperv_vhd.os", "type", "Differencing"),
					resource.TestCheckResourceAttr("hyperv_vhd.os", "parent_path", baseImage),

					// ISO
					resource.TestCheckResourceAttr("hyperv_iso.seed", "path", `C:\VMs\tf-acc-cloudinit-win\seed.iso`),
					resource.TestCheckResourceAttr("hyperv_iso.seed", "volume_label", "cidata"),
					resource.TestCheckResourceAttrSet("hyperv_iso.seed", "content_hash"),

					// VM core
					resource.TestCheckResourceAttr("hyperv_vm.test", "name", "tf-acc-cloudinit-win"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "generation", "2"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "state", "Running"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "processor_count", "4"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "secure_boot_enabled", "true"),

					// first_boot_device
					resource.TestCheckResourceAttr("hyperv_vm.test", "first_boot_device.device_type", "HardDiskDrive"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "first_boot_device.controller_number", "0"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "first_boot_device.controller_location", "0"),

					// inline hard_drive
					resource.TestCheckResourceAttr("hyperv_vm.test", "hard_drive.0.path", `C:\VMs\tf-acc-cloudinit-win\os.vhdx`),
					resource.TestCheckResourceAttr("hyperv_vm.test", "hard_drive.0.controller_type", "SCSI"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "hard_drive.0.controller_number", "0"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "hard_drive.0.controller_location", "0"),

					// inline dvd_drive
					resource.TestCheckResourceAttr("hyperv_vm.test", "dvd_drive.0.path", `C:\VMs\tf-acc-cloudinit-win\seed.iso`),
					resource.TestCheckResourceAttr("hyperv_vm.test", "dvd_drive.0.controller_number", "0"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "dvd_drive.0.controller_location", "1"),

					// inline network_adapter
					resource.TestCheckResourceAttr("hyperv_vm.test", "network_adapter.0.name", "Network Adapter"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "network_adapter.0.switch_name", "Default Switch"),
					resource.TestCheckResourceAttr("hyperv_vm.test", "network_adapter.0.dynamic_mac_address", "true"),

					// PowerShell Direct into the VM and verify cloudbase-init set the hostname
					func(_ *terraform.State) error {
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
						defer cancel()

						t.Log("Waiting for Windows VM via PowerShell Direct...")
						hostname, err := psDirectHostname(ctx, t, winrm.PS(), "tf-acc-cloudinit-win", guestUser, guestPassword, 10*time.Minute)
						if err != nil {
							return err
						}

						t.Logf("Hostname: %s", hostname)
						if !strings.EqualFold(hostname, "tf-acc-ci-win") {
							return fmt.Errorf("expected hostname %q, got %q", "tf-acc-ci-win", hostname)
						}
						return nil
					},
				),
			},
		},
	})
}

func testAccProviderConfig() string {
	host := os.Getenv("HYPERV_HOST")
	user := os.Getenv("HYPERV_USER")
	password := testAccPassword()

	return `
provider "hyperv" {
  host      = "` + host + `"
  port      = 5985
  use_tls   = false
  insecure  = true
  auth_type = "ntlm"
  username  = "` + user + `"
  password  = "` + password + `"
}
`
}

func readPasswordFile(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}
