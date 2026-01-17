package mailsender

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
)

const (
	// EnvDbHost is the environment variable name for the database host.
	EnvDbHost = "DB_HOST"
	// EnvDbName is the environment variable name for the database name.
	EnvDbName = "DB_NAME"
	// EnvDbUser is the environment variable name for the database user.
	EnvDbUser = "DB_USER"
	// EnvDbPassword is the environment variable name for the database password.
	EnvDbPassword = "DB_PASSWORD"
	// EnvDbSSLMode is the environment variable name for the database SSL mode.
	EnvDbSSLMode = "DB_SSLMODE"
)

// Config holds the application configuration settings.
type Config struct {
	AppIDs     []string          `json:"api-keys"`
	MyDomain   string            `json:"mydomain"`
	Host       string            `json:"host"`
	Port       int               `json:"port"`
	DbHost     string            `json:"dbhost"`
	DbName     string            `json:"dbname"`
	DbUser     string            `json:"dbuser"`
	DbPassword string            `json:"dbpassword"`
	DbSSLMode  string            `json:"dbsslmode"`
	SMTPPort   int               `json:"smtp-port"`
	SMTPLog    string            `json:"smtp-log"`
	RelayHost  string            `json:"relayhost"`
	RelayUser  string            `json:"relayuser"`
	RelayPass  string            `json:"relaypass"`
	Others     map[string]string `json:"others"`
}

// DefaultConfig returns the default configuration values.
func DefaultConfig() *Config {
	return &Config{Host: "0.0.0.0",
		Port:       8333,
		MyDomain:   "local",
		DbHost:     "localhost",
		DbName:     "mailsender",
		DbUser:     "ms",
		DbPassword: "bd9838864bdbbf1c7cd39a0e394c50cd1d0d516c",
		DbSSLMode:  "disable",
		SMTPPort:   25,
		SMTPLog:    "/var/log/mail.log",
		AppIDs:     []string{},
	}
}

// ParseConfig reads specified configuration file.
func ParseConfig(configStr string) (*Config, error) {
	config := DefaultConfig()

	if configStr == "" {
		return overwriteConfigFromEnv(config), nil
	}
	decoder := json.NewDecoder(strings.NewReader(configStr))
	err := decoder.Decode(config)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return overwriteConfigFromEnv(config), nil
}

// overwriteConfigFromEnv overrides configuration values with environment
// variables when they are set.
func overwriteConfigFromEnv(config *Config) *Config {
	if value, found := os.LookupEnv(EnvDbHost); found {
		config.DbHost = value
	}
	if value, found := os.LookupEnv(EnvDbName); found {
		config.DbName = value
	}
	if value, found := os.LookupEnv(EnvDbUser); found {
		config.DbUser = value
	}
	if value, found := os.LookupEnv(EnvDbPassword); found {
		config.DbPassword = value
	}
	if value, found := os.LookupEnv(EnvDbSSLMode); found {
		config.DbSSLMode = value
	}
	return config
}
