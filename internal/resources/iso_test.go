package resources

import "testing"

func TestComputeContentHash_Deterministic(t *testing.T) {
	files := map[string]string{
		"meta-data":      "instance-id: test-vm\nlocal-hostname: test-vm\n",
		"user-data":      "#cloud-config\npackages:\n  - curl\n",
		"network-config": "version: 2\nethernets:\n  eth0:\n    dhcp4: true\n",
	}

	hash1 := computeContentHash("cidata", files)
	hash2 := computeContentHash("cidata", files)

	if hash1 != hash2 {
		t.Errorf("expected deterministic hash, got %q and %q", hash1, hash2)
	}
}

func TestComputeContentHash_DifferentFiles(t *testing.T) {
	files1 := map[string]string{
		"meta-data": "instance-id: vm1\n",
	}
	files2 := map[string]string{
		"meta-data": "instance-id: vm2\n",
	}

	hash1 := computeContentHash("cidata", files1)
	hash2 := computeContentHash("cidata", files2)

	if hash1 == hash2 {
		t.Error("expected different hashes for different file contents")
	}
}

func TestComputeContentHash_DifferentVolumeLabel(t *testing.T) {
	files := map[string]string{
		"meta-data": "instance-id: test\n",
	}

	hash1 := computeContentHash("cidata", files)
	hash2 := computeContentHash("CIDATA", files)

	if hash1 == hash2 {
		t.Error("expected different hashes for different volume labels")
	}
}

func TestComputeContentHash_EmptyFiles(t *testing.T) {
	hash := computeContentHash("cidata", map[string]string{})
	if hash == "" {
		t.Error("expected non-empty hash for empty files map")
	}
}

func TestComputeContentHash_FileOrderIrrelevant(t *testing.T) {
	// Go maps are unordered, but we sort keys internally.
	// Verify by creating maps with different insertion patterns.
	files1 := map[string]string{
		"a-file": "aaa",
		"b-file": "bbb",
		"c-file": "ccc",
	}
	files2 := map[string]string{
		"c-file": "ccc",
		"a-file": "aaa",
		"b-file": "bbb",
	}

	hash1 := computeContentHash("label", files1)
	hash2 := computeContentHash("label", files2)

	if hash1 != hash2 {
		t.Errorf("expected same hash regardless of map order, got %q and %q", hash1, hash2)
	}
}
