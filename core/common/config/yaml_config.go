package config

import (
	"os"
)

func LoadYAMLConfigData() ([]byte, error) {
	// Check if file exists
	info, err := os.Stat(ConfigFilePath)
	if os.IsNotExist(err) {
		return nil, err
	}

	if info.IsDir() {
		return nil, nil
	}

	return os.ReadFile(ConfigFilePath)
}
