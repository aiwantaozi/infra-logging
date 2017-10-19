package provider

import (
	"fmt"

	infraconfig "github.com/aiwantaozi/infra-logging/config"
)

type LogProvider interface {
	ApplyConfig(infraconfig.InfraLoggingConfig) error
	Run()
	Stop() error
	Reload() error
	GetName() string
}

var (
	providers map[string]LogProvider
)

func GetProvider(name string) LogProvider {
	if provider, ok := providers[name]; ok {
		return provider
	}
	return providers["fluentd"]
}

func RegisterProvider(name string, provider LogProvider) error {
	if providers == nil {
		providers = make(map[string]LogProvider)
	}
	if _, exists := providers[name]; exists {
		return fmt.Errorf("provider already registered")
	}
	providers[name] = provider
	return nil
}
