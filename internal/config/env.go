// Package config provides shared configuration utilities.
package config

import "os"

// GetEnv returns the value of the environment variable named by the key,
// or fallback if the variable is not set.
func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
