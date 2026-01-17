package mailsender

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewMailboxManager(t *testing.T) {
	mgr := newMailboxManager()
	require.NotNil(t, mgr)

	// Verify it's the correct type
	_, ok := mgr.(*mailboxMgr)
	require.True(t, ok)
}

func TestMailboxMgrParseLocalMail(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.Nil(t, err)

	app := &App{logger: logger.Sugar()}
	mgr := newMailboxManager()

	testmsg, err := os.ReadFile("../testdata/localmail1.txt")
	require.Nil(t, err)

	msg, err := mgr.parseLocalMail(app, testmsg)
	require.Nil(t, err)
	require.NotNil(t, msg)
}

func TestMailboxMgrGetFailedMessageID(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.Nil(t, err)

	app := &App{logger: logger.Sugar()}
	mgr := newMailboxManager()

	testmsg, err := os.ReadFile("../testdata/localmail1.txt")
	require.Nil(t, err)

	msg, err := mgr.parseLocalMail(app, testmsg)
	require.Nil(t, err)
	require.NotNil(t, msg)

	msgid := mgr.getFailedMessageID(app, msg)
	require.Equal(t, "CALN0JNFe31bscLfLs8q4Rkn+Ci94umj6_5+R5b8ABWWxeof4VA@mail.example.com", msgid)
}
