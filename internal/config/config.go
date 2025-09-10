package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"
)

var (
	ErrParameterNotSet = errors.New("config parameter is not set")
)

type Config struct {
	LogLevel             string
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
	JWTSecret            string
	JWTTTL               time.Duration
}

func getenvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func NewConfig() (*Config, error) {
	logLevel := flag.String(
		"log-level",
		getenvOr("LOG_LEVEL", "info"),
		"log level (default: info)",
	)
	runAddress := flag.String(
		"a",
		getenvOr("RUN_ADDRESS", ":8080"),
		"listen address",
	)
	databaseURI := flag.String(
		"d",
		getenvOr("DATABASE_URI", ""),
		"database connection string",
	)
	accrualSysAddress := flag.String(
		"r",
		getenvOr("ACCRUAL_SYSTEM_ADDRESS", ""),
		"accrual system address",
	)
	JWTSecret := flag.String(
		"jwt-secret",
		getenvOr("JWT_SECRET", ""),
		"jwt secret key for token encryption",
	)
	JWTTTL := flag.String(
		"jwt-ttl",
		getenvOr("JWT_TTL", "24h"),
		"jwt token ttl (default: 24h)",
	)

	flag.Parse()

	if *databaseURI == "" {
		return nil, fmt.Errorf("database URI error %w", ErrParameterNotSet)
	}

	if *accrualSysAddress == "" {
		return nil, fmt.Errorf(
			"accrual system address error %w",
			ErrParameterNotSet,
		)
	}

	if *JWTSecret == "" {
		return nil, fmt.Errorf("no jwt token set %w", ErrParameterNotSet)
	}

	jwtTTLDuration, err := time.ParseDuration(*JWTTTL)
	if err != nil {
		return nil, fmt.Errorf("invalid jwt ttl %q: %w", *JWTTTL, err)
	}

	return &Config{
		LogLevel:             *logLevel,
		RunAddress:           *runAddress,
		DatabaseURI:          *databaseURI,
		AccrualSystemAddress: *accrualSysAddress,
		JWTSecret:            *JWTSecret,
		JWTTTL:               jwtTTLDuration,
	}, nil
}

func (c *Config) String() string {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stdout, err)
		os.Exit(0)
	}
	return string(b)
}
