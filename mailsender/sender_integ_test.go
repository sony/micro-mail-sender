//go:build integration

package mailsender

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSendMessageSuccess(t *testing.T) {
	tapp := initTestBase(t, &TestConfig{smtpError: false})
	defer tapp.Fini()

	mes := Message{
		packet: sampleSendRequest(0),
		status: MessageProcessing,
	}

	err := sendMesg(tapp.app, &mes)
	require.Nil(t, err)

	require.Equal(t, 1, len(tapp.sentMails))
	require.Equal(t, "admin@example.com", tapp.sentMails[0].Sender)
	require.Equal(t, []string{"foo@example.com", "bar@example.com"},
		tapp.sentMails[0].Recipients)
	require.Nil(t, err)
	require.Equal(t, `From: admin@example.com`+"\n"+
		`To: foo@example.com`+"\n"+
		`Cc: bar@example.com`+"\n"+
		`Subject: test mail`+"\n"+
		"\n"+
		`This is a test mail body`+"\n",
		removeMessageIDFromText(string(tapp.sentMails[0].Data)))
}

func TestSendMessageError(t *testing.T) {
	tapp := initTestBase(t, &TestConfig{smtpError: true})
	defer tapp.Fini()

	mes := Message{
		packet: sampleSendRequest(0),
		status: MessageProcessing,
	}

	err := sendMesg(tapp.app, &mes)
	require.NotNil(t, err)
}

func TestMessageStatusAfterSend(t *testing.T) {
	tapp := initTestBase(t, nil)
	defer tapp.Fini()

	apperr := enqueueMessage(tapp.app, sampleSendRequest(0))
	require.Nil(t, apperr)

	m, err := dequeueMessage(tapp.app)
	require.Nil(t, err)

	uid := m.uid
	require.Equal(t, MessageProcessing, m.status)

	err = sendMesg(tapp.app, m)
	require.Nil(t, err)

	m, err = getMessage(tapp.app, uid)
	require.Nil(t, err)
	require.Equal(t, MessageSent, m.status)
}

func Test_runSenderLoop(t *testing.T) {
	tapp := initTestBase(t, nil)
	defer tapp.Fini()

	runSenderLoop(tapp.app)
	apperr := enqueueMessage(tapp.app, sampleSendRequest(0))
	require.Nil(t, apperr)
	time.Sleep(1 * time.Second)
	tapp.app.quitSenderHandler <- true
	require.True(t, true)
}
