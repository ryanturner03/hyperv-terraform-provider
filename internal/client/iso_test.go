package client

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildCreateISOScript_ReadsFromStdin(t *testing.T) {
	script := buildCreateISOScript(ISOOptions{
		Path:        `C:\VMs\test\seed.iso`,
		VolumeLabel: "cidata",
		Files: map[string]string{
			"meta-data": "instance-id: test\n",
			"user-data": "#cloud-config\n",
		},
	})

	for _, s := range []string{
		"$jsonInput",
		"ConvertFrom-Json",
		"$prop.Name",
		"FromBase64String",
		"VolumeName = 'cidata'",
	} {
		if !strings.Contains(script, s) {
			t.Errorf("script missing %q", s)
		}
	}

	// Script should NOT contain embedded base64 data (that goes via stdin now)
	b64 := base64.StdEncoding.EncodeToString([]byte("#cloud-config\n"))
	if strings.Contains(script, b64) {
		t.Error("script should not embed base64 data; file contents go via stdin")
	}
}

func TestBuildCreateISOScript_SubdirectorySupport(t *testing.T) {
	script := buildCreateISOScript(ISOOptions{
		Path:        `C:\VMs\test\config.iso`,
		VolumeLabel: "config-2",
		Files: map[string]string{
			"openstack/latest/meta_data.json": `{"uuid": "test"}`,
		},
	})

	if !strings.Contains(script, "New-Item -ItemType Directory") {
		t.Error("script should create parent directories for nested paths")
	}
	if !strings.Contains(script, "'config-2'") {
		t.Error("script should use the specified volume label")
	}
}

func TestBuildCreateISOStdinData(t *testing.T) {
	files := map[string]string{
		"meta-data": "instance-id: test\n",
		"user-data": "#cloud-config\nhostname: myvm\n",
	}
	stdinJSON := buildCreateISOStdinData(files)

	var payload map[string]string
	if err := json.Unmarshal([]byte(stdinJSON), &payload); err != nil {
		t.Fatalf("stdin data is not valid JSON: %v", err)
	}

	for name, content := range files {
		b64, ok := payload[name]
		if !ok {
			t.Errorf("missing file %q in stdin payload", name)
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Errorf("file %q: invalid base64: %v", name, err)
			continue
		}
		if string(decoded) != content {
			t.Errorf("file %q: got %q, want %q", name, decoded, content)
		}
	}
}
