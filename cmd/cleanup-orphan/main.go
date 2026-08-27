// cleanup-orphan removes a VM plus its VHD/ISO files using the provider's own
// WinRM client. Needed because the provider does not implement resource import,
// so a VM created by an apply that failed AFTER creation cannot be adopted into
// state and destroyed the normal way.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

func main() {
	host := flag.String("host", "dc-lab-srv2.ad.ipofficeuniverse.com", "Hyper-V host")
	realm := flag.String("realm", "AD.IPOFFICEUNIVERSE.COM", "Kerberos realm")
	ccache := flag.String("ccache", "/tmp/kc", "Kerberos ccache path")
	name := flag.String("name", "", "VM name to remove (required)")
	vmPath := flag.String("vm-path", `C:\ClusterStorage\Volume2\VMs`, "base VM directory")
	flag.Parse()

	if *name == "" {
		fmt.Fprintln(os.Stderr, "-name is required")
		os.Exit(2)
	}

	c, err := client.NewWinRMClient(client.ConnectionConfig{
		Host:      *host,
		Port:      5985,
		UseTLS:    false,
		Timeout:   120 * time.Second,
		AuthType:  client.AuthKerberos,
		Realm:     *realm,
		KrbCCache: *ccache,
		SPN:       "HTTP/" + *host,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	if vm, err := c.GetVM(ctx, *name); err != nil || vm == nil {
		fmt.Printf("VM %q not found (nothing to remove)\n", *name)
	} else {
		if err := c.DeleteVM(ctx, *name); err != nil {
			fmt.Fprintf(os.Stderr, "delete VM: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("removed VM %q\n", *name)
	}

	// Disk artifacts are NOT removed by Remove-VM; delete them explicitly.
	base := fmt.Sprintf(`%s\%s`, *vmPath, *name)
	for _, p := range []string{base + `\os.vhdx`, base + `\data.vhdx`} {
		if err := c.DeleteVHD(ctx, p); err != nil {
			fmt.Printf("  vhd %s: %v (ok if absent)\n", p, err)
		} else {
			fmt.Printf("  removed %s\n", p)
		}
	}
	if err := c.DeleteISO(ctx, base+`\cidata.iso`); err != nil {
		fmt.Printf("  iso: %v (ok if absent)\n", err)
	} else {
		fmt.Printf("  removed %s\\cidata.iso\n", base)
	}
}
