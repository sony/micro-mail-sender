package mailsender

import (
	"errors"
	"net"
	"regexp"
	"testing"

	"github.com/chrj/smtpd"
	"github.com/stretchr/testify/require"
)

// TestConfig test config
type TestConfig struct {
	smtpError      bool
	configOverride string
}

// TestApp test app
type TestApp struct {
	testConfig   *TestConfig
	app          *App
	smtpd        *smtpd.Server
	smtpdStopper chan bool
	sentMails    []*smtpd.Envelope
}

// Fini finish event
func (a *TestApp) Fini() {
	a.smtpdStopper <- true
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
		<-a.smtpdStopper
		_ = l.Close()
	}()
	go func() {
		_ = a.smtpd.Serve(l)
	}()
	return nil
}

func initTestBase(t *testing.T, tconf *TestConfig) *TestApp {
	if tconf == nil {
		tconf = &TestConfig{}
	}

	sConfig := tconf.configOverride
	if sConfig == "" {
		sConfig = `{"host":"localhost",` +
			`"smtp-port":2025,` +
			`"dbname":"mailsender_test",` +
			`"api-keys":["apikey"]}`
	}

	config, err := ParseConfig(sConfig)
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

// Message-Id is random and difficult to compare reliably.
func removeMessageIDFromText(text string) string {
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
