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
	require.Equal(t, "localhost", c.DbHost)
	require.Equal(t, "mailsender", c.DbName)

	c, err = ParseConfig(`{` +
		`"dbhost": "127.0.0.2",` +
		`"dbname": "mailsender_test"}`)
	require.Nil(t, err)
	require.Equal(t, "0.0.0.0", c.Host)
	require.Equal(t, 8333, c.Port)
	require.Equal(t, "local", c.MyDomain)
	require.Equal(t, "127.0.0.2", c.DbHost)
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
	require.Equal(t, "Invalid config file format: unexpected EOF",
		err.Error())
}
