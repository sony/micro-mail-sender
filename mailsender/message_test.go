package mailsender

import (
	"errors"
	"net"
	"regexp"
	"testing"

	"github.com/chrj/smtpd"
	"github.com/stretchr/testify/require"
)

type TestConfig struct {
	smtpError      bool
	configOverride string
}

type TestApp struct {
	testConfig   *TestConfig
	app          *App
	smtpd        *smtpd.Server
	smtpdStopper chan bool
	sentMails    []*smtpd.Envelope
}

func initTestBase(t *testing.T, tconf *TestConfig) *TestApp {
	if tconf == nil {
		tconf = &TestConfig{}
	}

	s_config := tconf.configOverride
	if s_config == "" {
		s_config = `{"host":"localhost",` +
			`"smtp-port":2025,` +
			`"dbname":"mailsender_test",` +
			`"api-keys":["apikey"]}`
	}

	config, err := ParseConfig(s_config)
	require.Nil(t, err)

	app := newApp(config)

	// clean up the db
	_, err = app.db.Exec("delete from messages")
	require.Nil(t, err)
	_, err = app.db.Exec("delete from bodies")
	require.Nil(t, err)

	testapp := TestApp{
		testConfig: tconf,
		app:        app,
	}

	// start the mock smtpd
	testapp.smtpd = &smtpd.Server{
		Handler: testapp.handleMail,
	}

	err = testapp.runSmtpdBackground("localhost:2025")
	require.Nil(t, err)

	return &testapp
}

func (a *TestApp) handleMail(peer smtpd.Peer, env smtpd.Envelope) error {
	a.app.logger.Debugw("handlemail",
		"peer", peer,
		"env", env)
	a.sentMails = append(a.sentMails, &env)
	if a.testConfig.smtpError {
		return errors.New("simulated smtp error")
	}
	return nil
}

func (a *TestApp) runSmtpdBackground(addr string) error {
	a.smtpdStopper = make(chan bool)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		defer close(a.smtpdStopper)
		_ = <-a.smtpdStopper
		l.Close()
	}()
	go a.smtpd.Serve(l)
	return nil
}

// Message-Id is random and difficult to compare reliably.
func removeMessageIdFromText(text string) string {
	return regexp.MustCompile(`Message-Id: <[^>]*>\r?\n`).ReplaceAllString(text, "")
}

// boundary is random and difficult to compare reliably.
func removeMultipartBoundaryFromText(text string) string {
	for {
		re := regexp.MustCompile(`Content-Type: multipart/[\w-]+; boundary=(?P<boundary>[0-9a-f]+)\r?\n`)
		submatches := re.FindStringSubmatch(text)
		if submatches == nil {
			return text
		}
		text = regexp.MustCompile(submatches[1]).ReplaceAllString(text, "")
	}
}

func (a *TestApp) Fini() {
	a.smtpdStopper <- true
}

func sampleSendRequest(n int) SendRequest {
	var v SendRequest
	switch n {
	default:
	case 0:
		v = SendRequest{
			Personalizations: []Personalization{
				{
					To: []Addressee{
						{
							Email: "foo@example.com",
						},
					},
					Cc: []Addressee{
						{
							Email: "bar@example.com",
						},
					},
				},
			},
			Subject: "test mail",
			From: Addressee{
				Email: "admin@example.com",
			},
			Content: []Content{
				{
					Type:  "text/plain",
					Value: "This is a test mail body",
				},
			},
		}
	case 1:
		v = SendRequest{
			Personalizations: []Personalization{
				{
					To: []Addressee{
						{
							Email: "foo@example.com",
						},
					},
					Cc: []Addressee{
						{
							Email: "bar@example.com",
						},
					},
				},
				{
					To: []Addressee{
						{
							Email: "baz@example.com",
						},
						{
							Email: "ar@example.com",
						},
					},
				},
			},
			Subject: "[URGENT] change your password",
			From: Addressee{
				Email: "admin@example.com",
			},
			Content: []Content{
				{
					Type:  "text/plain",
					Value: "This is a test mail body",
				},
			},
		}
	case 2:
		v = SendRequest{
			Personalizations: []Personalization{
				{
					To: []Addressee{
						{
							Email: "foo@example.com",
						},
					},
					Cc: []Addressee{
						{
							Email: "bar@example.com",
						},
					},
				},
			},
			Subject: "test mail",
			From: Addressee{
				Email: "admin@example.com",
			},
			Content: []Content{
				{
					Type:  "text/html",
					Value: "<H1>Hello</H1>",
				},
			},
		}
	case 3:
		v = SendRequest{
			Personalizations: []Personalization{
				{
					To: []Addressee{
						{
							Email: "foo@example.com",
						},
					},
					Cc: []Addressee{
						{
							Email: "bar@example.com",
						},
					},
				},
			},
			Subject: "test mail",
			From: Addressee{
				Email: "admin@example.com",
			},
			Content: []Content{
				{
					Type:  "text/html",
					Value: "<H1>Hello</H1>",
				},
				{
					Type:  "text/text",
					Value: "hello",
				},
			},
		}
	case 4:
		v = SendRequest{
			Personalizations: []Personalization{
				{
					To: []Addressee{
						{
							Email: "foo@example.com",
						},
					},
					Cc: []Addressee{
						{
							Email: "bar@example.com",
						},
					},
				},
			},
			Subject: "test mail",
			From: Addressee{
				Email: "admin@example.com",
			},
			Content: []Content{
				{
					Type:  "text/plain",
					Value: "Please see the attachments",
				},
				{
					Type:  "text/html",
					Value: "<p>Please see the attachments</p>",
				},
			},
			Attachments: []Attachment{
				{
					Content: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+P+/HgAFhAJ/wlseKgAAAABJRU5ErkJggg==",
					Type:    "image/png",
				},
				{
					Content: "R0lGODlhAQABAIAAAP///wAAACH5BAEAAAAALAAAAAABAAEAAAICRAEAOw==",
					Type:    "image/gif",
				},
			},
		}

	}
	return v
}

func TestQueueMessage(t *testing.T) {
	tapp := initTestBase(t, nil)
	defer tapp.Fini()

	type expectedMessage struct {
		id  int
		msg string
	}
	expectedMessages := []expectedMessage{
		{id: 0, msg: `From: admin@example.com` + "\r\n" +
			`To: foo@example.com` + "\r\n" +
			`Cc: bar@example.com` + "\r\n" +
			`Subject: test mail` + "\r\n" +
			"\r\n" +
			`This is a test mail body`},
		{id: 2, msg: `From: admin@example.com` + "\r\n" +
			`To: foo@example.com` + "\r\n" +
			`Cc: bar@example.com` + "\r\n" +
			`Subject: test mail` + "\r\n" +
			`Content-Type: text/html` + "\r\n" +
			"\r\n" +
			`<H1>Hello</H1>`},
		{id: 3, msg: `From: admin@example.com` + "\r\n" +
			`To: foo@example.com` + "\r\n" +
			`Cc: bar@example.com` + "\r\n" +
			`Subject: test mail` + "\r\n" +
			`Content-Type: multipart/alternative; boundary=` + "\r\n" +
			"\r\n" +
			"--\r\n" +
			`Content-Type: text/html` + "\r\n" +
			"\r\n" +
			`<H1>Hello</H1>` + "\r\n" +
			`--` + "\r\n" +
			`Content-Type: text/text` + "\r\n" +
			"\r\n" +
			`hello` + "\r\n" +
			`----` + "\r\n"},
		{id: 4, msg: `From: admin@example.com` + "\r\n" +
			`To: foo@example.com` + "\r\n" +
			`Cc: bar@example.com` + "\r\n" +
			`Subject: test mail` + "\r\n" +
			`Content-Type: multipart/mixed; boundary=` + "\r\n" +
			"\r\n" +
			"--\r\n" +
			`Content-Type: multipart/alternative; boundary=` + "\r\n" +
			"\r\n" +
			`--` + "\r\n" +
			`Content-Type: text/plain` + "\r\n" +
			"\r\n" +
			`Please see the attachments` + "\r\n" +
			`--` + "\r\n" +
			`Content-Type: text/html` + "\r\n" +
			"\r\n" +
			`<p>Please see the attachments</p>` + "\r\n" +
			`----` + "\r\n" +
			"\r\n" +
			`--` + "\r\n" +
			`Content-Type: image/png` + "\r\n" +
			"\r\n" +
			`iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+P+/HgAFhAJ/wlseKgAAAABJRU5ErkJggg==` + "\r\n" +
			`--` + "\r\n" +
			`Content-Type: image/gif` + "\r\n" +
			"\r\n" +
			`R0lGODlhAQABAIAAAP///wAAACH5BAEAAAAALAAAAAABAAEAAAICRAEAOw==` + "\r\n" +
			`----` + "\r\n"},
	}

	for _, expected := range expectedMessages {

		apperr := enqueueMessage(tapp.app, sampleSendRequest(expected.id))
		require.Nil(t, apperr)

		m, err := dequeueMessage(tapp.app)
		require.Nil(t, err)
		require.Equal(t, sampleSendRequest(expected.id), m.packet)

		require.Equal(t, []string{
			"foo@example.com",
			"bar@example.com",
		}, m.getRecipients())

		body, err := m.getMessageBody()
		require.Nil(t, err)

		require.Equal(t, expected.msg,
			removeMultipartBoundaryFromText(removeMessageIdFromText(string(body))))

		m, err = dequeueMessage(tapp.app)
		require.Nil(t, err)
		require.Nil(t, m)
	}
}

func TestSearchMessage(t *testing.T) {
	tapp := initTestBase(t, nil)
	defer tapp.Fini()

	apperr := enqueueMessage(tapp.app, sampleSendRequest(0))
	require.Nil(t, apperr)
	apperr = enqueueMessage(tapp.app, sampleSendRequest(1))
	require.Nil(t, apperr)

	var criteria QNode
	criteria = &QLeaf{QuerySender, QueryEqual, "admin@example.com"}
	ms, apperr := searchMessages(tapp.app, criteria, 0)
	require.Nil(t, apperr)
	require.Equal(t, 3, len(ms))

	criteria = &QLeaf{QuerySender, QueryEqual, "admin@example.com"}
	ms, apperr = searchMessages(tapp.app, criteria, 2)
	require.Nil(t, apperr)
	require.Equal(t, 2, len(ms))

	criteria = &QLeaf{QuerySender, QueryEqual, "admin@example.co.jp"}
	ms, apperr = searchMessages(tapp.app, criteria, 1)
	require.Nil(t, apperr)
	require.Equal(t, 0, len(ms))

	criteria = &QExpr{QueryAnd,
		&QLeaf{QuerySender, QueryEqual, "admin@example.com"},
		&QLeaf{QuerySubject, QueryEqual, "test"},
	}
	ms, apperr = searchMessages(tapp.app, criteria, 2)
	require.Nil(t, apperr)
	require.Equal(t, 1, len(ms))

	msgid := ms[0].uid

	criteria = &QLeaf{QueryMessageId, QueryEqual, msgid}
	ms2, apperr := searchMessages(tapp.app, criteria, 10)
	require.Nil(t, apperr)
	require.Equal(t, 1, len(ms2))
	require.Equal(t, ms[0], ms2[0])

	criteria = &QLeaf{QueryMessageId, QueryNotEqual, msgid}
	ms2, apperr = searchMessages(tapp.app, criteria, 10)
	require.Nil(t, apperr)
	require.Equal(t, 2, len(ms2))

	criteria = &QExpr{QueryAnd,
		&QLeaf{QuerySender, QueryEqual, "admin@example.com"},
		&QLeaf{QuerySubject, QueryEqual, "lorem ipsum"},
	}
	ms, apperr = searchMessages(tapp.app, criteria, 10)
	require.Nil(t, apperr)
	require.Equal(t, 0, len(ms))

	criteria = &QExpr{QueryOr,
		&QLeaf{QuerySubject, QueryEqual, "test"},
		&QLeaf{QuerySubject, QueryEqual, "lorem ipsum"},
	}
	ms, apperr = searchMessages(tapp.app, criteria, 10)
	require.Nil(t, apperr)
	require.Equal(t, 1, len(ms))

	criteria = &QLeaf{QueryReceiver, QueryEqual, "bar@example.com"}
	ms, apperr = searchMessages(tapp.app, criteria, 0)
	require.Nil(t, apperr)
	require.Equal(t, 2, len(ms))

	criteria = &QLeaf{QueryReceiver, QueryNotEqual, "bar@example.com"}
	ms, apperr = searchMessages(tapp.app, criteria, 0)
	require.Nil(t, apperr)
	require.Equal(t, 1, len(ms))

	criteria = &QLeaf{QueryReceiver, QueryEqual, "ar@example.com"}
	ms, apperr = searchMessages(tapp.app, criteria, 0)
	require.Nil(t, apperr)
	require.Equal(t, 1, len(ms))

	criteria = &QLeaf{QueryStatus, QueryEqual, "waiting"}
	ms, apperr = searchMessages(tapp.app, criteria, 0)
	require.Nil(t, apperr)
	require.Equal(t, 3, len(ms))
}
