// Package config provides shared configuration utilities.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadEnvFile loads environment variables from the given file path.
// Lines starting with '#' or empty lines are ignored.
// Each line should be in the format KEY=VALUE.
// Existing environment variables are NOT overwritten.
// If the file does not exist, no error is returned.
func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open env file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, rest, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value, _, ok := strings.Cut(rest, "#")
		if !ok {
			value = rest
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// Don't overwrite existing environment variables
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set env %s: %w", key, err)
			}
		}
	}

	return scanner.Err()
}

// GetEnv returns the value of the environment variable named by the key,
// or fallback if the variable is not set.
func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
