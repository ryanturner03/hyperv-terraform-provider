package client

import (
	"strings"
	"testing"
)

func TestBuildCreateISOScript_FlatFiles(t *testing.T) {
	script := buildCreateISOScript(ISOOptions{
		Path:        `C:\VMs\test\seed.iso`,
		VolumeLabel: "cidata",
		Files: map[string]string{
			"meta-data": "instance-id: test\n",
			"user-data": "#cloud-config\n",
		},
	})

	for _, s := range []string{
		"Join-Path $tempDir 'meta-data'",
		"Join-Path $tempDir 'user-data'",
		"VolumeName = 'cidata'",
	} {
		if !strings.Contains(script, s) {
			t.Errorf("script missing %q", s)
		}
	}
}

func TestBuildCreateISOScript_SubdirectoryPaths(t *testing.T) {
	script := buildCreateISOScript(ISOOptions{
		Path:        `C:\VMs\test\config.iso`,
		VolumeLabel: "config-2",
		Files: map[string]string{
			"openstack/latest/meta_data.json": `{"uuid": "test"}`,
			"openstack/latest/user_data":      "#cloud-config\n",
		},
	})

	// Must create parent directories before writing files
	if !strings.Contains(script, "New-Item -ItemType Directory") {
		t.Error("script should create parent directories for nested paths")
	}
	if !strings.Contains(script, "'openstack/latest/meta_data.json'") {
		t.Error("script should reference the full nested path")
	}
	if !strings.Contains(script, "'config-2'") {
		t.Error("script should use the specified volume label")
	}
}
