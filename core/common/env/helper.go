package env

import (
	"math"
	"os"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	log "github.com/sirupsen/logrus"
)

func Bool(env string, defaultValue bool) bool {
	if env == "" {
		return defaultValue
	}

	e := os.Getenv(env)
	if e == "" {
		return defaultValue
	}

	p, err := strconv.ParseBool(e)
	if err != nil {
		log.Errorf("invalid %s: %s", env, e)
		return defaultValue
	}

	return p
}

func Int64(env string, defaultValue int64) int64 {
	if env == "" {
		return defaultValue
	}

	e := os.Getenv(env)
	if e == "" {
		return defaultValue
	}

	num, err := strconv.ParseInt(e, 10, 64)
	if err != nil {
		log.Errorf("invalid %s: %s", env, e)
		return defaultValue
	}

	return num
}

func Float64(env string, defaultValue float64) float64 {
	if env == "" {
		return defaultValue
	}

	e := os.Getenv(env)
	if e == "" {
		return defaultValue
	}

	num, err := strconv.ParseFloat(e, 64)
	if err != nil {
		log.Errorf("invalid %s: %s", env, e)
		return defaultValue
	}

	return num
}

func String(env, defaultValue string) string {
	if env == "" {
		return defaultValue
	}

	e := os.Getenv(env)
	if e == "" {
		return defaultValue
	}

	return e
}

func JSON[T any](env string, defaultValue T) T {
	if env == "" {
		return defaultValue
	}

	e := os.Getenv(env)
	if e == "" {
		return defaultValue
	}

	var t T
	if err := sonic.UnmarshalString(e, &t); err != nil {
		log.Errorf("invalid %s: %s", env, e)
		return defaultValue
	}

	return t
}

// PositiveInt64 reads a positive decimal integer, retaining the default for invalid limits.
func PositiveInt64(name string, defaultValue int64) int64 {
	value := Int64(name, defaultValue)
	if value <= 0 || value == math.MaxInt64 {
		log.Errorf("invalid %s: expected a positive integer below MaxInt64", name)
		return defaultValue
	}

	return value
}

// PositiveDuration reads a Go duration such as 500ms or 10m.
func PositiveDuration(name string, defaultValue time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultValue
	}

	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		log.Errorf("invalid %s: expected a positive duration", name)
		return defaultValue
	}

	return value
}
