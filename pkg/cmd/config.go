package kubedump

import (
	"fmt"
	"os"
	"path"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

type Config struct {
	LogSyncTimeout   string
	ExcludeResources []schema.GroupVersionResource
	DefaultFilter    string
	DefaultNWorkers  int
}

func DefaultConfig() *Config {
	return &Config{
		LogSyncTimeout:   "2s",
		ExcludeResources: []schema.GroupVersionResource{},
		DefaultFilter:    "",
		DefaultNWorkers:  5,
	}
}

func ConfigFromFile(path string) (*Config, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return nil, fmt.Errorf("could not unmarshal config file: %w", err)
	}

	return config, nil
}

func ConfigFromDefaultFile() (*Config, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("could not get user config dir: %w", err)
	}

	path := path.Join(dir, "kubedump.yaml")

	return ConfigFromFile(path)
}
