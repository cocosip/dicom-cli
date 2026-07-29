package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"
)

// Write serializes a validated configuration as YAML or JSON. An empty format
// selects JSON only for a .json path and YAML for every other extension.
func Write(path string, config Config, format string) error {
	if err := config.Validate(); err != nil {
		return err
	}

	if format == "" {
		format = "yaml"
		if filepath.Ext(path) == ".json" {
			format = "json"
		}
	}

	document := configurationDocument{
		Version:  config.Version,
		UIDRoot:  config.UIDRoot,
		Timeouts: durationDocumentFrom(config.Timeouts),
		Targets:  make(map[string]targetDocument, len(config.Targets)),
	}
	for name, target := range config.Targets {
		document.Targets[name] = targetDocument{
			Host:      target.Host,
			Port:      target.Port,
			CallingAE: target.CallingAETitle,
			CalledAE:  target.CalledAETitle,
			Timeouts:  durationDocumentFrom(target.Timeouts),
			TLS:       target.TLS,
			Proxy:     target.Proxy,
			Auth:      target.Auth,
		}
	}

	var (
		content []byte
		err     error
	)
	switch format {
	case "yaml", "yml":
		content, err = yaml.Marshal(document)
	case "json":
		content, err = json.MarshalIndent(document, "", "  ")
		if err == nil {
			content = append(content, '\n')
		}
	default:
		return fmt.Errorf("unsupported configuration format %q", format)
	}
	if err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	return os.WriteFile(path, content, 0o600)
}

type configurationDocument struct {
	Version  string                    `json:"version" yaml:"version"`
	UIDRoot  string                    `json:"uid,omitempty" yaml:"uid,omitempty"`
	Timeouts durationDocument          `json:"timeouts,omitempty" yaml:"timeouts,omitempty"`
	Targets  map[string]targetDocument `json:"targets" yaml:"targets"`
}

type targetDocument struct {
	Host      string           `json:"host" yaml:"host"`
	Port      int              `json:"port" yaml:"port"`
	CallingAE string           `json:"calling_ae" yaml:"calling_ae"`
	CalledAE  string           `json:"called_ae" yaml:"called_ae"`
	Timeouts  durationDocument `json:"timeouts,omitempty" yaml:"timeouts,omitempty"`
	TLS       TLSConfig        `json:"tls,omitempty" yaml:"tls,omitempty"`
	Proxy     ProxyConfig      `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	Auth      AuthConfig       `json:"auth,omitempty" yaml:"auth,omitempty"`
}

type durationDocument struct {
	Connect   string `json:"connect,omitempty" yaml:"connect,omitempty"`
	Associate string `json:"associate,omitempty" yaml:"associate,omitempty"`
	Idle      string `json:"idle,omitempty" yaml:"idle,omitempty"`
}

func durationDocumentFrom(timeouts Timeouts) durationDocument {
	return durationDocument{
		Connect:   formatDuration(timeouts.Connect),
		Associate: formatDuration(timeouts.Associate),
		Idle:      formatDuration(timeouts.Idle),
	}
}

func formatDuration(value time.Duration) string {
	if value == 0 {
		return ""
	}
	return value.String()
}
