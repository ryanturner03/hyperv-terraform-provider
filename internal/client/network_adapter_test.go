package client

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

// mockPS implements psExecutor with configurable responses.
type mockPS struct {
	mu           sync.Mutex
	runCalls     int
	runJSONCalls int
	// jsonResponses is a queue of objects to marshal and return from RunJSON.
	jsonResponses []any
}

func (m *mockPS) Run(ctx context.Context, command string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runCalls++
	return "", "", nil
}

func (m *mockPS) RunJSON(ctx context.Context, command string, result any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runJSONCalls >= len(m.jsonResponses) {
		return nil
	}
	resp := m.jsonResponses[m.runJSONCalls]
	m.runJSONCalls++
	data, _ := json.Marshal(resp)
	return json.Unmarshal(data, result)
}

func newTestClient(ps psExecutor) *WinRMClient {
	return &WinRMClient{
		ps:   ps,
		vmMu: make(map[string]*sync.Mutex),
	}
}

func TestCreateNetworkAdapter_ReturnsReadbackMAC(t *testing.T) {
	// After creation, GetNetworkAdapter is called once to read back state.
	// Whatever MAC Hyper-V reports is returned as-is.
	mock := &mockPS{
		jsonResponses: []any{
			NetworkAdapter{Name: "eth0", VMName: "vm1", MacAddress: "00155D010203", DynamicMacAddress: true},
		},
	}
	c := newTestClient(mock)

	adapter, err := c.CreateNetworkAdapter(context.Background(), AdapterOptions{
		VMName: "vm1",
		Name:   "eth0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter.MacAddress != "00155D010203" {
		t.Errorf("expected MAC 00155D010203, got %s", adapter.MacAddress)
	}
	if mock.runJSONCalls != 1 {
		t.Errorf("expected 1 RunJSON call, got %d", mock.runJSONCalls)
	}
}

func TestCreateNetworkAdapter_OffVM_ReturnsZeroMAC(t *testing.T) {
	// Hyper-V only assigns a dynamic MAC when the VM starts.
	// For an Off VM, 000000000000 is expected — no retry, no error.
	// The real MAC will appear on the next Read after the VM starts.
	mock := &mockPS{
		jsonResponses: []any{
			NetworkAdapter{Name: "eth0", VMName: "vm1", MacAddress: "000000000000", DynamicMacAddress: true},
		},
	}
	c := newTestClient(mock)

	adapter, err := c.CreateNetworkAdapter(context.Background(), AdapterOptions{
		VMName: "vm1",
		Name:   "eth0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter.MacAddress != "000000000000" {
		t.Errorf("expected MAC 000000000000 for Off VM, got %s", adapter.MacAddress)
	}
	// Single read, no retries
	if mock.runJSONCalls != 1 {
		t.Errorf("expected 1 RunJSON call (no retry), got %d", mock.runJSONCalls)
	}
}

func TestCreateNetworkAdapter_StaticMAC(t *testing.T) {
	// When a static MAC is provided, the read-back should reflect it.
	mock := &mockPS{
		jsonResponses: []any{
			NetworkAdapter{Name: "eth0", VMName: "vm1", MacAddress: "AABBCCDDEEFF", DynamicMacAddress: false},
		},
	}
	c := newTestClient(mock)

	adapter, err := c.CreateNetworkAdapter(context.Background(), AdapterOptions{
		VMName:     "vm1",
		Name:       "eth0",
		MacAddress: "AABBCCDDEEFF",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter.MacAddress != "AABBCCDDEEFF" {
		t.Errorf("expected MAC AABBCCDDEEFF, got %s", adapter.MacAddress)
	}
	if mock.runJSONCalls != 1 {
		t.Errorf("expected 1 RunJSON call, got %d", mock.runJSONCalls)
	}
}

func TestCreateNetworkAdapter_WithVlan(t *testing.T) {
	mock := &mockPS{
		jsonResponses: []any{
			NetworkAdapter{Name: "eth0", VMName: "vm1", MacAddress: "00155D010203", DynamicMacAddress: true},
		},
	}
	c := newTestClient(mock)

	adapter, err := c.CreateNetworkAdapter(context.Background(), AdapterOptions{
		VMName:    "vm1",
		Name:      "eth0",
		VlanID:    100,
		VlanIDSet: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1 Run for Add-VMNetworkAdapter + 1 Run for Set-VMNetworkAdapterVlan + 1 RunJSON for Get
	if mock.runCalls != 2 {
		t.Errorf("expected 2 Run calls (create + vlan), got %d", mock.runCalls)
	}
	if adapter.MacAddress != "00155D010203" {
		t.Errorf("expected MAC 00155D010203, got %s", adapter.MacAddress)
	}
}
