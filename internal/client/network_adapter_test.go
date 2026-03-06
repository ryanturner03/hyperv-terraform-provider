package client

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

// mockPS implements PSExecutor with configurable responses.
type mockPS struct {
	mu           sync.Mutex
	runCalls     int
	runJSONCalls int
	// jsonResponses is a queue of objects to marshal and return from RunJSON.
	jsonResponses []any
	// runStdout, if set, is returned by Run instead of "".
	runStdout string
}

func (m *mockPS) Run(ctx context.Context, command string) (string, string, error) {
	return m.RunWithInput(ctx, command, "")
}

func (m *mockPS) RunWithInput(ctx context.Context, command string, stdin string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runCalls++
	return m.runStdout, "", nil
}

func (m *mockPS) RunJSON(ctx context.Context, command string, result any) error {
	return m.RunJSONWithInput(ctx, command, "", result)
}

func (m *mockPS) RunJSONWithInput(ctx context.Context, command string, stdin string, result any) error {
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

func newTestClient(ps PSExecutor) *WinRMClient {
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

func TestListNetworkAdapters_SingleAdapter(t *testing.T) {
	singleJSON := `{"Name":"eth0","VMName":"vm1","SwitchName":"Default Switch","MacAddress":"00155D010203","DynamicMacAddressEnabled":true,"VlanID":0}`
	mock := &mockPS{runStdout: singleJSON}
	c := newTestClient(mock)

	adapters, err := c.ListNetworkAdapters(context.Background(), "vm1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(adapters) != 1 {
		t.Fatalf("expected 1 adapter, got %d", len(adapters))
	}
	if adapters[0].Name != "eth0" {
		t.Errorf("expected name eth0, got %s", adapters[0].Name)
	}
	if adapters[0].MacAddress != "00155D010203" {
		t.Errorf("expected MAC 00155D010203, got %s", adapters[0].MacAddress)
	}
}

func TestListNetworkAdapters_MultipleAdapters(t *testing.T) {
	arrayJSON := `[{"Name":"eth0","VMName":"vm1","SwitchName":"Default Switch","MacAddress":"00155D010203","DynamicMacAddressEnabled":true,"VlanID":0},{"Name":"eth1","VMName":"vm1","SwitchName":"Internal","MacAddress":"00155D040506","DynamicMacAddressEnabled":true,"VlanID":100}]`
	mock := &mockPS{runStdout: arrayJSON}
	c := newTestClient(mock)

	adapters, err := c.ListNetworkAdapters(context.Background(), "vm1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(adapters) != 2 {
		t.Fatalf("expected 2 adapters, got %d", len(adapters))
	}
	if adapters[0].Name != "eth0" {
		t.Errorf("expected first adapter name eth0, got %s", adapters[0].Name)
	}
	if adapters[1].Name != "eth1" {
		t.Errorf("expected second adapter name eth1, got %s", adapters[1].Name)
	}
	if adapters[1].VlanID != 100 {
		t.Errorf("expected second adapter VLAN 100, got %d", adapters[1].VlanID)
	}
}

func TestListNetworkAdapters_Empty(t *testing.T) {
	mock := &mockPS{runStdout: ""}
	c := newTestClient(mock)

	adapters, err := c.ListNetworkAdapters(context.Background(), "vm1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapters != nil {
		t.Errorf("expected nil adapters for empty output, got %v", adapters)
	}
}
