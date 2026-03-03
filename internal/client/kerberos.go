package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/credentials"
	"github.com/jcmturner/gokrb5/v8/spnego"
	"github.com/masterzen/winrm"
	"github.com/masterzen/winrm/soap"
)

// krbTransporter implements winrm.Transporter using gokrb5's spnego.Client
// for proper SPNEGO challenge-response (401 Negotiate) handling.
//
// The upstream winrm.ClientKerberos sets the SPNEGO token on the initial
// request but does not handle the 401 + WWW-Authenticate: Negotiate
// challenge that WinRM servers return. spnego.Client.Do() handles this
// automatically by detecting the 401, setting the token, and replaying
// the request.
type krbTransporter struct {
	transport http.RoundTripper
	krbClient *client.Client
	spn       string
	url       string // "http(s)://host:port/wsman"
	username  string
	password  string
	realm     string
	krbConf   string
	krbCCache string
}

// Transport initializes the HTTP transport and Kerberos client.
// Called once by winrm.NewClientWithParameters during client creation.
func (t *krbTransporter) Transport(endpoint *winrm.Endpoint) error {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: endpoint.Insecure,
			ServerName:         endpoint.TLSServerName,
		},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: endpoint.Timeout,
	}

	if len(endpoint.CACert) > 0 {
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(endpoint.CACert) {
			return fmt.Errorf("unable to read CA certificates")
		}
		transport.TLSClientConfig.RootCAs = certPool
	}

	t.transport = transport

	cfg, err := config.Load(t.krbConf)
	if err != nil {
		return fmt.Errorf("unable to load krb5 config %s: %w", t.krbConf, err)
	}

	if t.krbCCache != "" {
		b, err := os.ReadFile(t.krbCCache)
		if err != nil {
			return fmt.Errorf("unable to read ccache file %s: %w", t.krbCCache, err)
		}
		cc := new(credentials.CCache)
		if err := cc.Unmarshal(b); err != nil {
			return fmt.Errorf("unable to parse ccache file %s: %w", t.krbCCache, err)
		}
		t.krbClient, err = client.NewFromCCache(cc, cfg, client.DisablePAFXFAST(true))
		if err != nil {
			return fmt.Errorf("unable to create kerberos client from ccache: %w", err)
		}
	} else {
		t.krbClient = client.NewWithPassword(t.username, t.realm, t.password, cfg,
			client.DisablePAFXFAST(true), client.AssumePreAuthentication(true))
	}

	return nil
}

// Post sends a SOAP message to the WinRM endpoint using SPNEGO authentication.
// It sets the SPNEGO token upfront and handles a single 401 challenge-response
// retry. This avoids the infinite loop in spnego.Client.Do() that occurs when
// the server keeps returning 401 Negotiate (e.g., bad SPN or expired ticket).
func (t *krbTransporter) Post(_ *winrm.Client, request *soap.SoapMessage) (string, error) {
	httpClient := &http.Client{Transport: t.transport}
	reqBody := request.String()

	req, err := http.NewRequest("POST", t.url, strings.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("impossible to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/soap+xml;charset=UTF-8")

	if err := spnego.SetSPNEGOHeader(t.krbClient, req, t.spn); err != nil {
		return "", fmt.Errorf("unable to set SPNEGO header: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("kerberos request failed: %w", err)
	}

	// Handle 401 challenge-response: retry once with a fresh SPNEGO token.
	if resp.StatusCode == http.StatusUnauthorized &&
		resp.Header.Get("WWW-Authenticate") == "Negotiate" {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		req, err = http.NewRequest("POST", t.url, strings.NewReader(reqBody))
		if err != nil {
			return "", fmt.Errorf("impossible to create retry request: %w", err)
		}
		req.Header.Set("Content-Type", "application/soap+xml;charset=UTF-8")

		if err := spnego.SetSPNEGOHeader(t.krbClient, req, t.spn); err != nil {
			return "", fmt.Errorf("unable to set SPNEGO header on retry: %w", err)
		}

		resp, err = httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("kerberos retry request failed: %w", err)
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		wwwAuth := resp.Header.Get("WWW-Authenticate")
		return "", fmt.Errorf("http error %d (WWW-Authenticate: %q, SPN: %s, URL: %s): %s",
			resp.StatusCode, wwwAuth, t.spn, t.url, string(body))
	}

	return string(body), nil
}
