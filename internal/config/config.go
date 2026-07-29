// Package config defines runtime configuration independent of file loading.
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const VersionV1 = "v1"

// Config contains the runtime settings stored in dicom-cli configuration files.
type Config struct {
	Version  string                `json:"version" yaml:"version" mapstructure:"version"`
	UIDRoot  string                `json:"uid" yaml:"uid" mapstructure:"uid"`
	Timeouts Timeouts              `json:"timeouts" yaml:"timeouts" mapstructure:"timeouts"`
	Targets  map[string]PACSTarget `json:"targets" yaml:"targets" mapstructure:"targets"`
}

// Timeouts contains DIMSE connection phase and I/O idle timeouts. A zero value
// is unset and allows a target to inherit its enclosing configuration value.
type Timeouts struct {
	Connect   time.Duration `json:"connect" yaml:"connect" mapstructure:"connect"`
	Associate time.Duration `json:"associate" yaml:"associate" mapstructure:"associate"`
	Idle      time.Duration `json:"idle" yaml:"idle" mapstructure:"idle"`
}

// PACSTarget is a named remote PACS connection definition.
type PACSTarget struct {
	Host           string      `json:"host" yaml:"host" mapstructure:"host"`
	Port           int         `json:"port" yaml:"port" mapstructure:"port"`
	CallingAETitle string      `json:"calling_ae" yaml:"calling_ae" mapstructure:"calling_ae"`
	CalledAETitle  string      `json:"called_ae" yaml:"called_ae" mapstructure:"called_ae"`
	Timeouts       Timeouts    `json:"timeouts" yaml:"timeouts" mapstructure:"timeouts"`
	TLS            TLSConfig   `json:"tls" yaml:"tls" mapstructure:"tls"`
	Proxy          ProxyConfig `json:"proxy" yaml:"proxy" mapstructure:"proxy"`
	Auth           AuthConfig  `json:"auth" yaml:"auth" mapstructure:"auth"`
}

// TLSConfig reserves transport security settings for a later TLS implementation.
type TLSConfig struct {
	Enabled            bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	ServerName         string `json:"server_name" yaml:"server_name" mapstructure:"server_name"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify" yaml:"insecure_skip_verify" mapstructure:"insecure_skip_verify"`
	CAFile             string `json:"ca_file" yaml:"ca_file" mapstructure:"ca_file"`
	CertFile           string `json:"cert_file" yaml:"cert_file" mapstructure:"cert_file"`
	KeyFile            string `json:"key_file" yaml:"key_file" mapstructure:"key_file"`
}

// ProxyConfig reserves proxy settings for a later proxy transport implementation.
type ProxyConfig struct {
	URL      string `json:"url" yaml:"url" mapstructure:"url"`
	Username string `json:"username" yaml:"username" mapstructure:"username"`
	Password string `json:"password" yaml:"password" mapstructure:"password"`
}

// AuthConfig reserves application authentication settings for a later implementation.
type AuthConfig struct {
	Username string `json:"username" yaml:"username" mapstructure:"username"`
	Password string `json:"password" yaml:"password" mapstructure:"password"`
	Token    string `json:"token" yaml:"token" mapstructure:"token"`
}

// DefaultConfig returns the versioned defaults used when no value is configured.
func DefaultConfig() Config {
	return Config{
		Version: VersionV1,
		Timeouts: Timeouts{
			Connect:   10 * time.Second,
			Associate: 30 * time.Second,
			Idle:      5 * time.Minute,
		},
		Targets: make(map[string]PACSTarget),
	}
}

// Validate checks values that can be verified without loading a configuration file.
func (config Config) Validate() error {
	var validationErrors []error

	if config.Version != VersionV1 {
		validationErrors = append(validationErrors, fmt.Errorf("version must be %q", VersionV1))
	}
	if !validOIDRoot(config.UIDRoot) {
		validationErrors = append(validationErrors, fmt.Errorf("uid must be a dotted numeric OID"))
	}
	validationErrors = append(validationErrors, validateTimeouts("timeouts", config.Timeouts)...)

	for name, target := range config.Targets {
		fieldPrefix := "targets." + name
		if target.Port < 1 || target.Port > 65535 {
			validationErrors = append(validationErrors, fmt.Errorf("%s.port must be between 1 and 65535", fieldPrefix))
		}
		if !validAETitle(target.CallingAETitle) {
			validationErrors = append(validationErrors, fmt.Errorf("%s.calling_ae must be 1 to 16 printable ASCII characters", fieldPrefix))
		}
		if !validAETitle(target.CalledAETitle) {
			validationErrors = append(validationErrors, fmt.Errorf("%s.called_ae must be 1 to 16 printable ASCII characters", fieldPrefix))
		}
		validationErrors = append(validationErrors, validateTimeouts(fieldPrefix+".timeouts", target.Timeouts)...)
	}

	return errors.Join(validationErrors...)
}

func validateTimeouts(prefix string, timeouts Timeouts) []error {
	var validationErrors []error
	for _, timeout := range []struct {
		name  string
		value time.Duration
	}{
		{name: "connect", value: timeouts.Connect},
		{name: "associate", value: timeouts.Associate},
		{name: "idle", value: timeouts.Idle},
	} {
		if timeout.value < 0 {
			validationErrors = append(validationErrors, fmt.Errorf("%s.%s must be positive", prefix, timeout.name))
		}
	}

	return validationErrors
}

func validAETitle(value string) bool {
	if len(value) == 0 || len(value) > 16 {
		return false
	}

	for _, character := range []byte(value) {
		if character < 0x20 || character > 0x7e {
			return false
		}
	}

	return true
}

func validOIDRoot(value string) bool {
	if value == "" {
		return true
	}

	for _, arc := range strings.Split(value, ".") {
		if arc == "" {
			return false
		}
		for _, character := range []byte(arc) {
			if character < '0' || character > '9' {
				return false
			}
		}
	}

	return true
}
