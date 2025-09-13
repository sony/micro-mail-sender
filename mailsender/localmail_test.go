package mailsender

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetFailedMessageId(t *testing.T) {
	config, _ := ParseConfig(`{"host":"localhost"}`)
	app := newApp(config)

	testmsg, err := ioutil.ReadFile("../testdata/localmail1.txt")
	require.Nil(t, err)

	msg, err := parseLocalMail(app, testmsg)
	require.Nil(t, err)
	require.NotNil(t, msg)

	msgid := getFailedMessageId(app, msg)
	require.Equal(t, "CALN0JNFe31bscLfLs8q4Rkn+Ci94umj6_5+R5b8ABWWxeof4VA@mail.example.com",
		msgid)
}
