//go:build integration

package mailsender

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
			removeMultipartBoundaryFromText(removeMessageIDFromText(string(body))))

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

	msgid := ms[0].MsgID

	criteria = &QLeaf{QueryMessageID, QueryEqual, msgid}
	ms2, apperr := searchMessages(tapp.app, criteria, 10)
	require.Nil(t, apperr)
	require.Equal(t, 1, len(ms2))
	require.Equal(t, ms[0], ms2[0])

	criteria = &QLeaf{QueryMessageID, QueryNotEqual, msgid}
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
