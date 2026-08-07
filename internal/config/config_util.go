package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func envString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func envInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback, fmt.Errorf(
			"%s must be an integer: %w",
			key,
			err,
		)
	}

	return value, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback, fmt.Errorf(
			"%s must be a valid duration: %w",
			key,
			err,
		)
	}

	return value, nil
}

func envBool(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback, fmt.Errorf(
			"%s must be a boolean: %w",
			key,
			err,
		)
	}

	return value, nil
}
