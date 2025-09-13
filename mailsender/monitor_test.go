package mailsender

import (
	"log"
	"net"
	"os/exec"
	"os/user"
	"testing"

	"github.com/chrj/smtpd"
	smtp "github.com/emersion/go-smtp"
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

	err = testmon.runSmtpBackground("localhost:2025")
	require.Nil(t, err)

	return &testmon
}

func (m *TestMonitor) runSmtpBackground(addr string) error {
	m.smtpdStopper = make(chan bool)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		defer close(m.smtpdStopper)
		<-m.smtpdStopper
		l.Close()
	}()
	go m.smtpd.Serve(l)
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
	defer clnt.Close()

	err = clnt.Hello("localhost")
	if err != nil {
		return err
	}

	err = clnt.Mail("mailer-daemon@localhost", nil)
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

	waitus := int64(100)
	for i := 0; i < 10; i++ {
		processed, w := monitor1(tmon.app, waitus)
		if processed {
			break
		}
		waitus = w
	}

	m, err = getMessage(tmon.app, tmon.msgid)
	require.Nil(t, err)
	require.Equal(t, MessageAbandoned, m.status)

}
