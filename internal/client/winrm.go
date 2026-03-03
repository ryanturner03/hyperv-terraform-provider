package client

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/masterzen/winrm"
)

// AuthType represents the WinRM authentication method.
type AuthType string

const (
	AuthKerberos AuthType = "kerberos"
	AuthNTLM     AuthType = "ntlm"
	AuthBasic    AuthType = "basic"
)

// ConnectionConfig holds the configuration needed to establish a WinRM connection.
type ConnectionConfig struct {
	Host      string
	Port      int
	UseTLS    bool
	Insecure  bool
	Timeout   time.Duration
	AuthType  AuthType
	Username  string
	Password  string
	Realm     string
	KrbConfig string
	KrbCCache string
	SPN       string
}

// WinRMClient wraps a PowerShellRunner to execute commands on a remote Hyper-V host.
type WinRMClient struct {
	ps   *PowerShellRunner
	mu   sync.Mutex             // protects vmMu map
	vmMu map[string]*sync.Mutex // per-VM locks
}

// vmLock acquires a per-VM mutex and returns an unlock function.
// This serializes all mutating operations targeting the same VM within
// the provider process, preventing Hyper-V lock conflicts.
func (c *WinRMClient) vmLock(name string) func() {
	c.mu.Lock()
	m, ok := c.vmMu[name]
	if !ok {
		m = &sync.Mutex{}
		c.vmMu[name] = m
	}
	c.mu.Unlock()
	m.Lock()
	return m.Unlock
}

// PS returns the underlying PowerShellRunner for executing commands.
func (c *WinRMClient) PS() *PowerShellRunner {
	return c.ps
}

// NewWinRMClient creates a new WinRM client using the provided connection configuration.
func NewWinRMClient(cfg ConnectionConfig) (*WinRMClient, error) {
	endpoint := winrm.NewEndpoint(cfg.Host, cfg.Port, cfg.UseTLS, cfg.Insecure, nil, nil, nil, cfg.Timeout)

	var client *winrm.Client
	var err error

	switch cfg.AuthType {
	case AuthBasic:
		if cfg.Username == "" || cfg.Password == "" {
			return nil, fmt.Errorf("username and password required for basic auth")
		}
		client, err = winrm.NewClient(endpoint, cfg.Username, cfg.Password)

	case AuthNTLM:
		if cfg.Username == "" || cfg.Password == "" {
			return nil, fmt.Errorf("username and password required for NTLM auth")
		}
		params := *winrm.DefaultParameters
		params.TransportDecorator = func() winrm.Transporter {
			return &winrm.ClientNTLM{}
		}
		client, err = winrm.NewClientWithParameters(endpoint, cfg.Username, cfg.Password, &params)

	case AuthKerberos:
		if cfg.Realm == "" {
			return nil, fmt.Errorf("realm required for kerberos auth")
		}
		krbConf := cfg.KrbConfig
		if krbConf == "" {
			krbConf = "/etc/krb5.conf"
		}
		spn := cfg.SPN
		if spn == "" {
			spn = "HTTP/" + cfg.Host
		}
		proto := protoFromTLS(cfg.UseTLS)
		params := *winrm.DefaultParameters
		params.TransportDecorator = func() winrm.Transporter {
			return &krbTransporter{
				username:  cfg.Username,
				password:  cfg.Password,
				realm:     cfg.Realm,
				krbConf:   krbConf,
				krbCCache: cfg.KrbCCache,
				spn:       spn,
				url:       fmt.Sprintf("%s://%s:%d/wsman", proto, cfg.Host, cfg.Port),
			}
		}
		client, err = winrm.NewClientWithParameters(endpoint, "", "", &params)

	default:
		return nil, fmt.Errorf("unsupported auth type: %s", cfg.AuthType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create winrm client: %w", err)
	}

	return &WinRMClient{
		ps:   NewPowerShellRunner(client),
		vmMu: make(map[string]*sync.Mutex),
	}, nil
}

// retryOnConflict retries an operation that may fail due to concurrent VM
// modifications. Hyper-V serializes changes per VM, so parallel Terraform
// operations (e.g., removing a hard drive and DVD drive simultaneously) can
// cause transient "OperationFailed" / "Rolling back" errors.
func retryOnConflict(ctx context.Context, maxAttempts int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < maxAttempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if !isConflictError(err) {
			return err
		}
		if i < maxAttempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return err
}

func isConflictError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Rolling back the virtual machine configuration") ||
		strings.Contains(msg, "is being modified by another operation") ||
		strings.Contains(msg, "Failed to modify device") ||
		strings.Contains(msg, "InvalidState,Microsoft.HyperV.PowerShell")
}

func protoFromTLS(useTLS bool) string {
	if useTLS {
		return "https"
	}
	return "http"
}
