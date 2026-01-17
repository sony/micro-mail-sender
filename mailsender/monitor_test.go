package mailsender

import (
	"log"
	"net"
	"net/mail"
	"net/smtp"
	"os/exec"
	"os/user"
	"testing"
	"time"

	"github.com/chrj/smtpd"
	"github.com/stretchr/testify/require"
)

// This test requires local MTA is running on port 25, and "/usr/bin/mail".
// On Debian, 'apt install mailutils' sets them up.

type TestMonitor struct {
	app          *App
	msgid        string
	smtpd        *smtpd.Server
	smtpdStopper chan bool
}

func InitTestMonitor(t *testing.T) *TestMonitor {
	err := exec.Command("/usr/bin/mail", "--usage").Run()
	if err != nil {
		log.Print("/usr/bin/mail isn't available.  Skipping the test.")
		return nil
	}

	config, err := ParseConfig(`{"host": "localhost",` +
		` "smtp-port": 2025,` +
		` "dbname":"mailsender_test"}`)
	require.Nil(t, err)
	app := newApp(config)

	// clean up the db
	_, err = app.db.Exec("delete from messages")
	require.Nil(t, err)
	_, err = app.db.Exec("delete from bodies")
	require.Nil(t, err)

	testmon := TestMonitor{
		app: app,
	}

	testmon.smtpd = &smtpd.Server{
		Handler: testmon.handleMail,
	}

	err = testmon.runSMTPBackground("localhost:2025")
	require.Nil(t, err)

	return &testmon
}

func (m *TestMonitor) runSMTPBackground(addr string) error {
	m.smtpdStopper = make(chan bool)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		defer close(m.smtpdStopper)
		<-m.smtpdStopper
		_ = l.Close()
	}()
	go func() {
		_ = m.smtpd.Serve(l)
	}()
	return nil
}

func (m *TestMonitor) handleMail(peer smtpd.Peer, env smtpd.Envelope) error {
	m.app.logger.Debugw("handlemail",
		"peer", peer,
		"body", string(env.Data))
	err := sendReturnMail(string(env.Data))
	if err != nil {
		m.app.logger.Debugw("return mail failed",
			"error", err)
	} else {
		m.app.logger.Debugw("return mail sent")
	}
	return err
}

func (m *TestMonitor) Fini() {
	m.smtpdStopper <- true
}

func sendReturnMail(returnmail string) error {
	smtpserver := "localhost:25"
	clnt, err := smtp.Dial(smtpserver)
	if err != nil {
		return err
	}
	defer func() {
		_ = clnt.Close()
	}()

	err = clnt.Hello("localhost")
	if err != nil {
		return err
	}

	err = clnt.Mail("mailer-daemon@localhost")
	if err != nil {
		return err
	}

	me, err := user.Current()
	if err != nil {
		return err
	}

	err = clnt.Rcpt(me.Username)
	if err != nil {
		return err
	}

	w, err := clnt.Data()
	if err != nil {
		return err
	}

	msg := []byte("From: <mailer-daemon@localmail>\r\n" +
		"To: <" + me.Username + ">\r\n" +
		"Subject: Delivery Status Notification (Failure)\r\n" +
		"Content-Type: multipart/report; boundary=\"boundary\"\r\n" +
		"\r\n" +
		"--boundary\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Message not delivered.\r\n" +
		"--boundary\r\n" +
		"Content-Type: message/rfc822\r\n" +
		"\r\n" +
		returnmail +
		"--boundary--\r\n")

	nwritten := 0
	for {
		n, err := w.Write(msg[nwritten:])
		if err != nil {
			return err
		}
		if nwritten+n == len(msg) {
			break
		}
		nwritten += n
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return clnt.Quit()
}

func TestMonitorMessage(t *testing.T) {
	tmon := InitTestMonitor(t)
	if tmon == nil {
		t.Skip()
	}
	defer tmon.Fini()

	sr := SendRequest{
		Personalizations: []Personalization{
			{
				To: []Addressee{
					{
						Email: "nonexistentlocalmailaccount@example.com",
					},
				},
			},
		},
		Subject: "test mail",
		From: Addressee{
			Email: "me@example.com",
		},
		Content: []Content{
			{
				Type:  "text/plain",
				Value: "This is a test mail body",
			},
		},
	}

	apperr := enqueueMessage(tmon.app, sr)
	require.Nil(t, apperr)

	m, err := dequeueMessage(tmon.app)
	require.Nil(t, err)

	tmon.msgid = m.uid

	err = sendMesg(tmon.app, m)
	require.Nil(t, err)

	mailBox := newMailboxManager()
	waitus := int64(100)
	for range 20 {
		processed, w := monitor1(tmon.app, mailBox, waitus)
		if processed {
			break
		}
		waitus = w
	}

	m, err = getMessage(tmon.app, tmon.msgid)
	require.Nil(t, err)
	require.Equal(t, MessageAbandoned, m.status)
}

func Test_runMonitorLoop(t *testing.T) {
	tmon := InitTestMonitor(t)
	if tmon == nil {
		t.Skip()
	}
	defer tmon.Fini()

	sr := SendRequest{
		Personalizations: []Personalization{
			{
				To: []Addressee{
					{
						Email: "nonexistentlocalmailaccount@example.com",
					},
				},
			},
		},
		Subject: "test mail",
		From: Addressee{
			Email: "me@example.com",
		},
		Content: []Content{
			{
				Type:  "text/plain",
				Value: "This is a test mail body",
			},
		},
	}

	apperr := enqueueMessage(tmon.app, sr)
	require.Nil(t, apperr)

	m, err := dequeueMessage(tmon.app)
	require.Nil(t, err)

	tmon.msgid = m.uid

	err = sendMesg(tmon.app, m)
	require.Nil(t, err)

	runMonitorLoop(tmon.app)

	time.Sleep(1 * time.Second)

	tmon.app.quitMonitorHandler <- true
	require.True(t, true)
}

func TestMonitor1WithMock(t *testing.T) {
	tmon := InitTestMonitor(t)
	if tmon == nil {
		t.Skip()
	}
	defer tmon.Fini()
	{
		mailBox := newMockMailboxManager(mockMailboxMgrOpt{hasUnreadLocalMailResult: false})
		res, code := monitor1(tmon.app, mailBox, 1)
		require.False(t, res, "false hasUnreadLocalMailResult")
		require.Equal(t, int64(500), code)
	}

	{
		mailBox := newMockMailboxManager(mockMailboxMgrOpt{hasUnreadLocalMailResult: true, failedFetchLocalMail: true})
		res, code := monitor1(tmon.app, mailBox, 1)
		require.False(t, res, "error with fetchLocalMail")
		require.Equal(t, int64(50), code)
	}

	{
		mailBox := newMockMailboxManager(mockMailboxMgrOpt{hasUnreadLocalMailResult: true, failedParseLocalMail: true})
		res, code := monitor1(tmon.app, mailBox, 1)
		require.False(t, res, "error with parseLocalMail")
		require.Equal(t, int64(50), code)
	}

	{
		mailBox := newMockMailboxManager(mockMailboxMgrOpt{hasUnreadLocalMailResult: true, parseLocalMailValue: nil})
		res, code := monitor1(tmon.app, mailBox, 1)
		require.False(t, res, "parseLocalMail value are null")
		require.Equal(t, int64(500), code)
	}

	{
		mailBox := newMockMailboxManager(mockMailboxMgrOpt{hasUnreadLocalMailResult: true, parseLocalMailValue: &mail.Message{}, getFailedMessageIDValue: ""})
		res, code := monitor1(tmon.app, mailBox, 1)
		require.False(t, res, "getFailedMessageID value are empty")
		require.Equal(t, int64(50), code)
	}

	{
		mailBox := newMockMailboxManager(mockMailboxMgrOpt{hasUnreadLocalMailResult: true, parseLocalMailValue: &mail.Message{}, getFailedMessageIDValue: "id"})
		res, code := monitor1(tmon.app, mailBox, 1)
		require.False(t, res, "getFailedMessageID is not empty")
		require.Equal(t, int64(50), code)
	}
}
