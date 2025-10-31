//go:build integration

package mailsender

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetFailedMessageID(t *testing.T) {
	config, _ := ParseConfig(`{"host":"localhost","dbname":"mailsender_test"}`)
	app := newApp(config)

	testmsg, err := os.ReadFile("../testdata/localmail1.txt")
	require.Nil(t, err)

	msg, err := parseLocalMail(app, testmsg)
	require.Nil(t, err)
	require.NotNil(t, msg)

	msgid := getFailedMessageID(app, msg)
	require.Equal(t, "CALN0JNFe31bscLfLs8q4Rkn+Ci94umj6_5+R5b8ABWWxeof4VA@mail.example.com",
		msgid)
}
