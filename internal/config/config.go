package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Node      NodeConfig      `yaml:"node"`
	Tunnel    TunnelConfig    `yaml:"tunnel"`
	Listeners []ListenerConfig `yaml:"listeners"`
	Targets   map[string]string `yaml:"targets"`
}

type NodeConfig struct {
	ID string `yaml:"id"`
}

type TunnelConfig struct {
	Listen string `yaml:"listen"`

	Peer PeerConfig `yaml:"peer"`

	TLS TLSConfig `yaml:"tls"`

	Reconnect ReconnectConfig `yaml:"reconnect"`

	KeepAlive KeepAliveConfig `yaml:"keepalive"`
}

type PeerConfig struct {
	Address string `yaml:"address"`
	Connect bool   `yaml:"connect"`
}

type TLSConfig struct {
	CA         string `yaml:"ca"`
	Cert       string `yaml:"cert"`
	Key        string `yaml:"key"`
	ServerName string `yaml:"server_name"`
}

type ReconnectConfig struct {
	Initial time.Duration `yaml:"initial"`
	Max     time.Duration `yaml:"max"`
}

type KeepAliveConfig struct {
	Interval time.Duration `yaml:"interval"`
}

type ListenerConfig struct {
	Name   string `yaml:"name"`
	Listen string `yaml:"listen"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.Node.ID) == "" {
		return fmt.Errorf("node.id must not be empty")
	}

	if strings.TrimSpace(c.Tunnel.Listen) == "" {
		return fmt.Errorf("tunnel.listen must not be empty")
	}

	if c.Tunnel.Peer.Connect && strings.TrimSpace(c.Tunnel.Peer.Address) == "" {
		return fmt.Errorf("tunnel.peer.address must be configured when peer.connect=true")
	}

	if c.Tunnel.TLS.CA == "" {
		return fmt.Errorf("tunnel.tls.ca must be configured")
	}

	if c.Tunnel.TLS.Cert == "" {
		return fmt.Errorf("tunnel.tls.cert must be configured")
	}

	if c.Tunnel.TLS.Key == "" {
		return fmt.Errorf("tunnel.tls.key must be configured")
	}

	if c.Tunnel.Reconnect.Initial <= 0 {
		c.Tunnel.Reconnect.Initial = time.Second
	}

	if c.Tunnel.Reconnect.Max <= 0 {
		c.Tunnel.Reconnect.Max = 30 * time.Second
	}

	if c.Tunnel.Reconnect.Max < c.Tunnel.Reconnect.Initial {
		return fmt.Errorf("tunnel.reconnect.max must be >= initial")
	}

	if c.Tunnel.KeepAlive.Interval <= 0 {
		c.Tunnel.KeepAlive.Interval = 30 * time.Second
	}

	seen := make(map[string]struct{})

	for _, listener := range c.Listeners {
		if listener.Name == "" {
			return fmt.Errorf("listener name must not be empty")
		}

		if listener.Listen == "" {
			return fmt.Errorf("listener %q has no listen address", listener.Name)
		}

		if _, exists := seen[listener.Name]; exists {
			return fmt.Errorf("duplicate listener name %q", listener.Name)
		}

		seen[listener.Name] = struct{}{}
	}

	for name, target := range c.Targets {
		if name == "" {
			return fmt.Errorf("target name must not be empty")
		}

		if target == "" {
			return fmt.Errorf("target %q has empty address", name)
		}
	}

	return nil
}

func (c *Config) TLSClientConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(c.Tunnel.TLS.Cert, c.Tunnel.TLS.Key)
	if err != nil {
		return nil, fmt.Errorf("load client certificate: %w", err)
	}

	caData, err := os.ReadFile(c.Tunnel.TLS.CA)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}

	roots := x509.NewCertPool()

	if !roots.AppendCertsFromPEM(caData) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		RootCAs:      roots,
		Certificates: []tls.Certificate{cert},
		ServerName:   c.Tunnel.TLS.ServerName,
		// We authenticate both directions using certificates.
		// Do not use InsecureSkipVerify.
		//
		// The server certificate must match ServerName.
	}, nil
}

func (c *Config) TLSServerConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(c.Tunnel.TLS.Cert, c.Tunnel.TLS.Key)
	if err != nil {
		return nil, fmt.Errorf("load server certificate: %w", err)
	}

	caData, err := os.ReadFile(c.Tunnel.TLS.CA)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}

	clientCAs := x509.NewCertPool()

	if !clientCAs.AppendCertsFromPEM(caData) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}, nil
}