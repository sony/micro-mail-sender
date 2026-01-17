package mailsender

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()

	require.Equal(t, "0.0.0.0", c.Host)
	require.Equal(t, 8333, c.Port)
	require.Equal(t, "local", c.MyDomain)
	require.Equal(t, "localhost", c.DbHost)
	require.Equal(t, "mailsender", c.DbName)
}

func TestParseConfig(t *testing.T) {
	c, err := ParseConfig(`{` +
		`"host":"smtp.example.com",` +
		`"port":2525,` +
		`"mydomain": "example.com"}`)
	require.Nil(t, err)
	require.Equal(t, "smtp.example.com", c.Host)
	require.Equal(t, 2525, c.Port)
	require.Equal(t, "example.com", c.MyDomain)
	require.Equal(t, "mailsender", c.DbName)

	c, err = ParseConfig(`{` +
		`"dbname": "mailsender_test"}`)
	require.Nil(t, err)
	require.Equal(t, "0.0.0.0", c.Host)
	require.Equal(t, 8333, c.Port)
	require.Equal(t, "local", c.MyDomain)
	require.Equal(t, "mailsender_test", c.DbName)

	c, err = ParseConfig(`{` +
		`"relayhost": "relayhost",` +
		`"relayuser": "username",` +
		`"relaypass": "password",` +
		`"others":{"key1":"value1","key2":"value2"}}`)
	require.Nil(t, err)
	require.Equal(t, "relayhost", c.RelayHost)
	require.Equal(t, "username", c.RelayUser)
	require.Equal(t, "password", c.RelayPass)
	require.Equal(t, "value1", c.Others["key1"])
}

func TestParseConfigInvalid(t *testing.T) {
	_, err := ParseConfig(`{`)
	require.NotNil(t, err)
	require.Equal(t, "unexpected EOF", err.Error())
}

func TestParseConfigEmpty(t *testing.T) {
	c, err := ParseConfig("")
	require.Nil(t, err)
	require.Equal(t, "0.0.0.0", c.Host)
	require.Equal(t, 8333, c.Port)
	require.Equal(t, "local", c.MyDomain)
}

func TestOverwriteConfigFromEnv(t *testing.T) {
	// Set environment variables for testing
	t.Setenv(EnvDbHost, "testhost")
	t.Setenv(EnvDbName, "testdb")
	t.Setenv(EnvDbUser, "testuser")
	t.Setenv(EnvDbPassword, "testpass")
	t.Setenv(EnvDbSSLMode, "require")

	c, err := ParseConfig("")
	require.Nil(t, err)

	require.Equal(t, "testhost", c.DbHost)
	require.Equal(t, "testdb", c.DbName)
	require.Equal(t, "testuser", c.DbUser)
	require.Equal(t, "testpass", c.DbPassword)
	require.Equal(t, "require", c.DbSSLMode)
}

func TestOverwriteConfigFromEnvPartial(t *testing.T) {
	// Only set some environment variables
	t.Setenv(EnvDbHost, "partialhost")

	c, err := ParseConfig("")
	require.Nil(t, err)

	require.Equal(t, "partialhost", c.DbHost)
	require.Equal(t, "mailsender", c.DbName) // Default value
	require.Equal(t, "ms", c.DbUser)         // Default value
}
