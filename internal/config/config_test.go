package config

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultConfigUsesDIMSETimeoutDefaults(t *testing.T) {
	config := DefaultConfig()

	if config.Version != VersionV1 {
		t.Fatalf("Version = %q, want %q", config.Version, VersionV1)
	}
	if config.Timeouts.Connect != 10*time.Second {
		t.Fatalf("Connect = %s, want 10s", config.Timeouts.Connect)
	}
	if config.Timeouts.Associate != 30*time.Second {
		t.Fatalf("Associate = %s, want 30s", config.Timeouts.Associate)
	}
	if config.Timeouts.Idle != 5*time.Minute {
		t.Fatalf("Idle = %s, want 5m", config.Timeouts.Idle)
	}
}

func TestConfigValidateAcceptsNamedTargetAndReservedFields(t *testing.T) {
	config := DefaultConfig()
	config.UIDRoot = "1.2.826.0.1.3680043.10.543"
	config.Targets = map[string]PACSTarget{
		"archive": {
			Host:           "pacs.example.test",
			Port:           104,
			CallingAETitle: "DICOMCLI",
			CalledAETitle:  "ARCHIVE",
			TLS: TLSConfig{
				Enabled:    true,
				ServerName: "pacs.example.test",
			},
			Proxy: ProxyConfig{URL: "http://proxy.example.test:8080"},
			Auth:  AuthConfig{Username: "operator", Password: "reserved"},
		},
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestConfigValidateRejectsInvalidTargetFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{
			name: "port below range",
			mutate: func(config *Config) {
				config.Targets["archive"] = PACSTarget{Port: 0, CallingAETitle: "DICOMCLI", CalledAETitle: "ARCHIVE"}
			},
			want: "port",
		},
		{
			name: "port above range",
			mutate: func(config *Config) {
				config.Targets["archive"] = PACSTarget{Port: 65536, CallingAETitle: "DICOMCLI", CalledAETitle: "ARCHIVE"}
			},
			want: "port",
		},
		{
			name: "empty calling AE title",
			mutate: func(config *Config) {
				config.Targets["archive"] = PACSTarget{Port: 104, CalledAETitle: "ARCHIVE"}
			},
			want: "calling_ae",
		},
		{
			name: "long called AE title",
			mutate: func(config *Config) {
				config.Targets["archive"] = PACSTarget{Port: 104, CallingAETitle: "DICOMCLI", CalledAETitle: "SEVENTEEN-CHARS!!"}
			},
			want: "called_ae",
		},
		{
			name: "control character in calling AE title",
			mutate: func(config *Config) {
				config.Targets["archive"] = PACSTarget{Port: 104, CallingAETitle: "DICOM\nCLI", CalledAETitle: "ARCHIVE"}
			},
			want: "calling_ae",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.mutate(&config)

			err := config.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %q, want field %q", err, tt.want)
			}
		})
	}
}

func TestConfigValidateRejectsNegativeDurationsAndMalformedUIDRoot(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{
			name: "negative connect timeout",
			mutate: func(config *Config) {
				config.Timeouts.Connect = -time.Second
			},
			want: "connect",
		},
		{
			name: "negative target associate timeout",
			mutate: func(config *Config) {
				config.Targets["archive"] = PACSTarget{Port: 104, CallingAETitle: "DICOMCLI", CalledAETitle: "ARCHIVE", Timeouts: Timeouts{Associate: -time.Second}}
			},
			want: "associate",
		},
		{
			name: "negative idle timeout",
			mutate: func(config *Config) {
				config.Timeouts.Idle = -time.Second
			},
			want: "idle",
		},
		{
			name: "empty UID arc",
			mutate: func(config *Config) {
				config.UIDRoot = "1..2"
			},
			want: "uid",
		},
		{
			name: "non-numeric UID arc",
			mutate: func(config *Config) {
				config.UIDRoot = "1.2.a"
			},
			want: "uid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.mutate(&config)

			err := config.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %q, want field %q", err, tt.want)
			}
		})
	}
}
