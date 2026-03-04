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

func TestCreateNetworkAdapter_MACAssignedAfterRetry(t *testing.T) {
	mock := &mockPS{
		jsonResponses: []any{
			NetworkAdapter{Name: "eth0", VMName: "vm1", MacAddress: "000000000000", DynamicMacAddress: true},
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
	if mock.runJSONCalls != 2 {
		t.Errorf("expected 2 RunJSON calls, got %d", mock.runJSONCalls)
	}
}

func TestCreateNetworkAdapter_MACAssignedImmediately(t *testing.T) {
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

func TestCreateNetworkAdapter_MACNeverAssigned(t *testing.T) {
	responses := make([]any, 6)
	for i := range responses {
		responses[i] = NetworkAdapter{Name: "eth0", VMName: "vm1", MacAddress: "000000000000", DynamicMacAddress: true}
	}
	mock := &mockPS{jsonResponses: responses}
	c := newTestClient(mock)

	adapter, err := c.CreateNetworkAdapter(context.Background(), AdapterOptions{
		VMName: "vm1",
		Name:   "eth0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter.MacAddress != "000000000000" {
		t.Errorf("expected MAC 000000000000, got %s", adapter.MacAddress)
	}
	// 1 initial + 5 retries = 6 total
	if mock.runJSONCalls != 6 {
		t.Errorf("expected 6 RunJSON calls, got %d", mock.runJSONCalls)
	}
}
