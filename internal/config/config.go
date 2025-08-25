package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
)

var (
	ErrParameterNotSet = errors.New("config parameter is not set")
)

type Config struct {
	LogLevel             string
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
}

func NewConfig() (*Config, error) {
	logLevel := flag.String("log-level", "info", "log level (default: info)")
	runAddress := flag.String("a", ":8080", "listen address")
	databaseURI := flag.String("d", "", "database connection string")
	accrualSysAddress := flag.String("r", "", "accrual system address")

	flag.Parse()

	finalLogLevel := *logLevel
	if env := os.Getenv("LOG_LEVEL"); env != "" {
		finalLogLevel = env
	}

	finalRunAddress := *runAddress
	if env := os.Getenv("RUN_ADDRESS"); env != "" {
		finalRunAddress = env
	}

	finalDatabaseURI := *databaseURI
	if env := os.Getenv("DATABASE_URI"); env != "" {
		finalDatabaseURI = env
	}

	finalAccrualSysAddress := *accrualSysAddress
	if env := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); env != "" {
		finalAccrualSysAddress = env
	}

	if finalDatabaseURI == "" {
		return nil, fmt.Errorf("database URI error %w", ErrParameterNotSet)
	}

	if finalAccrualSysAddress == "" {
		return nil, fmt.Errorf("accrual system address error %w", ErrParameterNotSet)
	}

	return &Config{
		LogLevel:             finalLogLevel,
		RunAddress:           finalRunAddress,
		DatabaseURI:          finalDatabaseURI,
		AccrualSystemAddress: finalAccrualSysAddress,
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
