package mailsender

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandPersonalization(t *testing.T) {
	req := SendRequest{
		Personalizations: []Personalization{
			{
				To: []Addressee{{Email: "a@example.com"}},
			},
			{
				To: []Addressee{{Email: "b@example.com"}},
			},
		},
		Subject: "Test Subject",
		From:    Addressee{Email: "from@example.com"},
	}

	expanded := expandPersonalization(req)
	require.Equal(t, 2, len(expanded))

	require.Equal(t, 1, len(expanded[0].Personalizations))
	require.Equal(t, "a@example.com", expanded[0].Personalizations[0].To[0].Email)

	require.Equal(t, 1, len(expanded[1].Personalizations))
	require.Equal(t, "b@example.com", expanded[1].Personalizations[0].To[0].Email)
}

func TestExpandPersonalizationSingle(t *testing.T) {
	req := SendRequest{
		Personalizations: []Personalization{
			{
				To: []Addressee{{Email: "single@example.com"}},
			},
		},
	}

	expanded := expandPersonalization(req)
	require.Equal(t, 1, len(expanded))
	require.Equal(t, "single@example.com", expanded[0].Personalizations[0].To[0].Email)
}

func TestReceiverEmails(t *testing.T) {
	req := &SendRequest{
		Personalizations: []Personalization{
			{
				To:  []Addressee{{Email: "to@example.com"}},
				Cc:  []Addressee{{Email: "cc@example.com"}},
				Bcc: []Addressee{{Email: "bcc@example.com"}},
			},
		},
	}

	result := receiverEmails(req)
	require.Contains(t, result, "\x01to@example.com\x01")
	require.Contains(t, result, "\x01cc@example.com\x01")
	require.Contains(t, result, "\x01bcc@example.com\x01")
}

func TestReceiverEmailsEmpty(t *testing.T) {
	req := &SendRequest{
		Personalizations: []Personalization{
			{
				To: []Addressee{{Email: ""}},
			},
		},
	}

	result := receiverEmails(req)
	require.Equal(t, "", result)
}

func TestReceiverEmailsMultiple(t *testing.T) {
	req := &SendRequest{
		Personalizations: []Personalization{
			{
				To: []Addressee{
					{Email: "to1@example.com"},
					{Email: "to2@example.com"},
				},
				Cc: []Addressee{
					{Email: "cc1@example.com"},
				},
			},
		},
	}

	result := receiverEmails(req)
	require.Contains(t, result, "\x01to1@example.com\x01")
	require.Contains(t, result, "\x01to2@example.com\x01")
	require.Contains(t, result, "\x01cc1@example.com\x01")
}

func TestGetStatusString(t *testing.T) {
	require.Equal(t, "waiting", getStatusString(MessageWaiting))
	require.Equal(t, "processing", getStatusString(MessageProcessing))
	require.Equal(t, "sent", getStatusString(MessageSent))
	require.Equal(t, "abandoned", getStatusString(MessageAbandoned))
	require.Equal(t, "unknown", getStatusString(999))
}

func TestGetRecipients(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Personalizations: []Personalization{
				{
					To:  []Addressee{{Email: "to@example.com"}},
					Cc:  []Addressee{{Email: "cc@example.com"}},
					Bcc: []Addressee{{Email: "bcc@example.com"}},
				},
			},
		},
	}

	recipients := m.getRecipients()
	require.Equal(t, 3, len(recipients))
	require.Equal(t, "to@example.com", recipients[0])
	require.Equal(t, "cc@example.com", recipients[1])
	require.Equal(t, "bcc@example.com", recipients[2])
}

func TestGetRecipientsEmpty(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Personalizations: []Personalization{
				{
					To: []Addressee{},
				},
			},
		},
	}

	recipients := m.getRecipients()
	require.Equal(t, 0, len(recipients))
}

func TestAddresseesFieldValue(t *testing.T) {
	m := &Message{}

	// Test without name
	result := m.addresseesFieldValue([]Addressee{
		{Email: "test@example.com"},
	})
	require.Equal(t, "test@example.com", result)

	// Test with name
	result = m.addresseesFieldValue([]Addressee{
		{Email: "test@example.com", Name: "Test User"},
	})
	require.Contains(t, result, "test@example.com")
	require.Contains(t, result, "Test User")

	// Test multiple addressees
	result = m.addresseesFieldValue([]Addressee{
		{Email: "a@example.com"},
		{Email: "b@example.com"},
	})
	require.Equal(t, "a@example.com, b@example.com", result)
}

func TestGetSingleContent(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "text/plain", Value: "Hello World"},
			},
		},
	}

	p := Personalization{}
	var buf bytes.Buffer
	m.getSingleContent(p, &buf)

	result := buf.String()
	require.Contains(t, result, "Hello World")
}

func TestGetSingleContentHTML(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "text/html", Value: "<h1>Hello</h1>"},
			},
		},
	}

	p := Personalization{}
	var buf bytes.Buffer
	m.getSingleContent(p, &buf)

	result := buf.String()
	require.Contains(t, result, "Content-Type: text/html")
	require.Contains(t, result, "<h1>Hello</h1>")
}

func TestGetSingleContentWithHeaders(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "text/plain", Value: "Hello"},
			},
		},
	}

	p := Personalization{
		Headers: map[string][]string{
			"X-Custom-Header": {"custom-value"},
		},
	}
	var buf bytes.Buffer
	m.getSingleContent(p, &buf)

	result := buf.String()
	require.Contains(t, result, "X-Custom-Header: custom-value")
}

func TestGetMultiContents(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "text/plain", Value: "Plain text"},
				{Type: "text/html", Value: "<p>HTML text</p>"},
			},
		},
	}

	p := Personalization{}
	var buf bytes.Buffer
	err := m.getMultiContents(p, &buf)
	require.Nil(t, err)

	result := buf.String()
	require.Contains(t, result, "multipart/alternative")
	require.Contains(t, result, "Plain text")
	require.Contains(t, result, "<p>HTML text</p>")
}

func TestGetMultiContentsWithAttachments(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "text/plain", Value: "Plain text"},
			},
			Attachments: []Attachment{
				{Type: "image/png", Content: "base64content"},
			},
		},
	}

	p := Personalization{}
	var buf bytes.Buffer
	err := m.getMultiContents(p, &buf)
	require.Nil(t, err)

	result := buf.String()
	require.Contains(t, result, "multipart/mixed")
	require.Contains(t, result, "base64content")
}

func TestGetMultiContentsEmptyType(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "", Value: "No type specified"},
				{Type: "text/html", Value: "<p>HTML</p>"},
			},
		},
	}

	p := Personalization{}
	var buf bytes.Buffer
	err := m.getMultiContents(p, &buf)
	require.Nil(t, err)

	result := buf.String()
	require.Contains(t, result, "text/plain")
	require.Contains(t, result, "No type specified")
}

func TestGetMessageBody(t *testing.T) {
	m := &Message{
		uid: "test-uid@local",
		packet: SendRequest{
			From: Addressee{Email: "from@example.com"},
			Personalizations: []Personalization{
				{
					To:      []Addressee{{Email: "to@example.com"}},
					Subject: "Test Subject",
				},
			},
			Content: []Content{
				{Type: "text/plain", Value: "Test body"},
			},
		},
	}

	body, err := m.getMessageBody()
	require.Nil(t, err)

	result := string(body)
	require.Contains(t, result, "From: from@example.com")
	require.Contains(t, result, "To: to@example.com")
	require.Contains(t, result, "Subject: Test Subject")
	require.Contains(t, result, "Message-Id: <test-uid@local>")
	require.Contains(t, result, "Test body")
}

func TestGetMessageBodyWithCcBcc(t *testing.T) {
	m := &Message{
		uid: "test-uid@local",
		packet: SendRequest{
			From: Addressee{Email: "from@example.com"},
			Personalizations: []Personalization{
				{
					To:  []Addressee{{Email: "to@example.com"}},
					Cc:  []Addressee{{Email: "cc@example.com"}},
					Bcc: []Addressee{{Email: "bcc@example.com"}},
				},
			},
			Subject: "Default Subject",
			Content: []Content{
				{Type: "text/plain", Value: "Test body"},
			},
		},
	}

	body, err := m.getMessageBody()
	require.Nil(t, err)

	result := string(body)
	require.Contains(t, result, "To: to@example.com")
	require.Contains(t, result, "Cc: cc@example.com")
	require.Contains(t, result, "Bcc: bcc@example.com")
	require.Contains(t, result, "Subject: Default Subject")
}

func TestGetMessageBodyMultiContent(t *testing.T) {
	m := &Message{
		uid: "test-uid@local",
		packet: SendRequest{
			From: Addressee{Email: "from@example.com"},
			Personalizations: []Personalization{
				{
					To: []Addressee{{Email: "to@example.com"}},
				},
			},
			Subject: "Multi Content",
			Content: []Content{
				{Type: "text/plain", Value: "Plain"},
				{Type: "text/html", Value: "<p>HTML</p>"},
			},
		},
	}

	body, err := m.getMessageBody()
	require.Nil(t, err)

	result := string(body)
	require.Contains(t, result, "multipart/alternative")
}

func TestGetMessageBodyNoContent(t *testing.T) {
	m := &Message{
		uid: "test-uid@local",
		packet: SendRequest{
			From: Addressee{Email: "from@example.com"},
			Personalizations: []Personalization{
				{
					To: []Addressee{{Email: "to@example.com"}},
				},
			},
			Subject: "No Content",
			Content: []Content{},
		},
	}

	body, err := m.getMessageBody()
	require.Nil(t, err)

	result := string(body)
	require.Contains(t, result, "From: from@example.com")
	require.Contains(t, result, "Subject: No Content")
}

func TestBuildWhereClauseSender(t *testing.T) {
	leaf := &QLeaf{QuerySender, QueryEqual, "test@example.com"}
	clause, args := buildWhereClause(leaf, []any{})

	require.Equal(t, "sender = $1", clause)
	require.Equal(t, 1, len(args))
	require.Equal(t, "test@example.com", args[0])
}

func TestBuildWhereClauseSenderNotEqual(t *testing.T) {
	leaf := &QLeaf{QuerySender, QueryNotEqual, "test@example.com"}
	clause, args := buildWhereClause(leaf, []any{})

	require.Equal(t, "sender <> $1", clause)
	require.Equal(t, 1, len(args))
}

func TestBuildWhereClauseReceiver(t *testing.T) {
	leaf := &QLeaf{QueryReceiver, QueryEqual, "test@example.com"}
	clause, args := buildWhereClause(leaf, []any{})

	require.Contains(t, clause, "strpos(receivers, $1) > 0")
	require.Equal(t, "\x01test@example.com\x01", args[0])
}

func TestBuildWhereClauseReceiverNotEqual(t *testing.T) {
	leaf := &QLeaf{QueryReceiver, QueryNotEqual, "test@example.com"}
	clause, _ := buildWhereClause(leaf, []any{})

	require.Contains(t, clause, "strpos(receivers, $1) = 0")
}

func TestBuildWhereClauseSubject(t *testing.T) {
	leaf := &QLeaf{QuerySubject, QueryEqual, "Test Subject"}
	clause, args := buildWhereClause(leaf, []any{})

	require.Contains(t, clause, "strpos(subject, $1) > 0")
	require.Equal(t, "Test Subject", args[0])
}

func TestBuildWhereClauseSubjectNotEqual(t *testing.T) {
	leaf := &QLeaf{QuerySubject, QueryNotEqual, "Test Subject"}
	clause, _ := buildWhereClause(leaf, []any{})

	require.Contains(t, clause, "strpos(subject, $1) = 0")
}

func TestBuildWhereClauseStatus(t *testing.T) {
	testCases := []struct {
		value    string
		expected int
	}{
		{"waiting", MessageWaiting},
		{"processing", MessageProcessing},
		{"sent", MessageSent},
		{"abandoned", MessageAbandoned},
		{"unknown", 0},
	}

	for _, tc := range testCases {
		leaf := &QLeaf{QueryStatus, QueryEqual, tc.value}
		clause, args := buildWhereClause(leaf, []any{})

		require.Equal(t, "status = $1", clause)
		require.Equal(t, tc.expected, args[0])
	}
}

func TestBuildWhereClauseStatusNotEqual(t *testing.T) {
	leaf := &QLeaf{QueryStatus, QueryNotEqual, "sent"}
	clause, args := buildWhereClause(leaf, []any{})

	require.Equal(t, "status <> $1", clause)
	require.Equal(t, MessageSent, args[0])
}

func TestBuildWhereClauseMessageID(t *testing.T) {
	leaf := &QLeaf{QueryMessageID, QueryEqual, "msg-123"}
	clause, args := buildWhereClause(leaf, []any{})

	require.Equal(t, "uid = $1", clause)
	require.Equal(t, "msg-123", args[0])
}

func TestBuildWhereClauseMessageIDNotEqual(t *testing.T) {
	leaf := &QLeaf{QueryMessageID, QueryNotEqual, "msg-123"}
	clause, _ := buildWhereClause(leaf, []any{})

	require.Equal(t, "uid <> $1", clause)
}

func TestBuildWhereClauseAND(t *testing.T) {
	expr := &QExpr{
		qtype: QueryAnd,
		a:     &QLeaf{QuerySender, QueryEqual, "a@example.com"},
		b:     &QLeaf{QueryReceiver, QueryEqual, "b@example.com"},
	}
	clause, args := buildWhereClause(expr, []any{})

	require.Contains(t, clause, "AND")
	require.Equal(t, 2, len(args))
}

func TestBuildWhereClauseOR(t *testing.T) {
	expr := &QExpr{
		qtype: QueryOr,
		a:     &QLeaf{QuerySender, QueryEqual, "a@example.com"},
		b:     &QLeaf{QuerySender, QueryEqual, "b@example.com"},
	}
	clause, args := buildWhereClause(expr, []any{})

	require.Contains(t, clause, "OR")
	require.Equal(t, 2, len(args))
}

func TestBuildWhereClauseWithExistingArgs(t *testing.T) {
	leaf := &QLeaf{QuerySender, QueryEqual, "test@example.com"}
	clause, args := buildWhereClause(leaf, []any{"existing"})

	require.Equal(t, "sender = $2", clause)
	require.Equal(t, 2, len(args))
	require.Equal(t, "existing", args[0])
	require.Equal(t, "test@example.com", args[1])
}

func TestBuildQueryExpr(t *testing.T) {
	expr := &QExpr{
		qtype: QueryAnd,
		a:     &QLeaf{QuerySender, QueryEqual, "a@example.com"},
		b:     &QLeaf{QueryReceiver, QueryEqual, "b@example.com"},
	}

	clause, args := buildQueryExpr(expr, "AND", []any{})
	require.True(t, strings.HasPrefix(clause, "("))
	require.True(t, strings.HasSuffix(clause, ")"))
	require.Contains(t, clause, " AND ")
	require.Equal(t, 2, len(args))
}

func TestBuildQuerySender(t *testing.T) {
	leaf := &QLeaf{QuerySender, QueryEqual, "test@example.com"}
	clause, args := buildQuerySender(leaf, []any{})

	require.Equal(t, "sender = $1", clause)
	require.Equal(t, "test@example.com", args[0])
}

func TestBuildQueryReceiver(t *testing.T) {
	leaf := &QLeaf{QueryReceiver, QueryEqual, "test@example.com"}
	clause, args := buildQueryReceiver(leaf, []any{})

	require.Equal(t, "strpos(receivers, $1) > 0", clause)
	require.Equal(t, "\x01test@example.com\x01", args[0])
}

func TestBuildQuerySubject(t *testing.T) {
	leaf := &QLeaf{QuerySubject, QueryEqual, "Test"}
	clause, args := buildQuerySubject(leaf, []any{})

	require.Equal(t, "strpos(subject, $1) > 0", clause)
	require.Equal(t, "Test", args[0])
}

func TestBuildQueryStatus(t *testing.T) {
	leaf := &QLeaf{QueryStatus, QueryEqual, "sent"}
	clause, args := buildQueryStatus(leaf, []any{})

	require.Equal(t, "status = $1", clause)
	require.Equal(t, MessageSent, args[0])
}

func TestBuildQueryMessageID(t *testing.T) {
	leaf := &QLeaf{QueryMessageID, QueryEqual, "uid-123"}
	clause, args := buildQueryMessageID(leaf, []any{})

	require.Equal(t, "uid = $1", clause)
	require.Equal(t, "uid-123", args[0])
}

func TestGetMessageBodyWithPersonalizationSubject(t *testing.T) {
	m := &Message{
		uid: "test-uid@local",
		packet: SendRequest{
			From: Addressee{Email: "from@example.com"},
			Personalizations: []Personalization{
				{
					To:      []Addressee{{Email: "to@example.com"}},
					Subject: "", // Empty personalization subject should use packet subject
				},
			},
			Subject: "Packet Subject",
			Content: []Content{
				{Type: "text/plain", Value: "Test body"},
			},
		},
	}

	body, err := m.getMessageBody()
	require.Nil(t, err)

	result := string(body)
	require.Contains(t, result, "Subject: Packet Subject")
}

func TestGetMultiContentsAttachmentEmptyType(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "text/plain", Value: "Plain text"},
			},
			Attachments: []Attachment{
				{Type: "", Content: "attachment content"}, // Empty type defaults to text/plain
			},
		},
	}

	p := Personalization{}
	var buf bytes.Buffer
	err := m.getMultiContents(p, &buf)
	require.Nil(t, err)

	result := buf.String()
	require.Contains(t, result, "multipart/mixed")
	require.Contains(t, result, "attachment content")
}

func TestGetMultiContentsContentEmptyTypeAttachments(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "", Value: "No type specified"}, // Empty type defaults to text/plain
			},
			Attachments: []Attachment{
				{Type: "image/png", Content: "imagedata"},
			},
		},
	}

	p := Personalization{}
	var buf bytes.Buffer
	err := m.getMultiContents(p, &buf)
	require.Nil(t, err)

	result := buf.String()
	require.Contains(t, result, "text/plain")
}

func TestGetMessageBodyWithNamedAddressees(t *testing.T) {
	m := &Message{
		uid: "test-uid@local",
		packet: SendRequest{
			From: Addressee{Email: "from@example.com", Name: "Sender Name"},
			Personalizations: []Personalization{
				{
					To:  []Addressee{{Email: "to@example.com", Name: "To Name"}},
					Cc:  []Addressee{{Email: "cc@example.com", Name: "CC Name"}},
					Bcc: []Addressee{{Email: "bcc@example.com", Name: "BCC Name"}},
				},
			},
			Subject: "Test Subject",
			Content: []Content{
				{Type: "text/plain", Value: "Test body"},
			},
		},
	}

	body, err := m.getMessageBody()
	require.Nil(t, err)

	result := string(body)
	require.Contains(t, result, "Sender Name")
	require.Contains(t, result, "To Name")
	require.Contains(t, result, "CC Name")
	require.Contains(t, result, "BCC Name")
}

func TestGetMessageBodyEmptyTo(t *testing.T) {
	m := &Message{
		uid: "test-uid@local",
		packet: SendRequest{
			From: Addressee{Email: "from@example.com"},
			Personalizations: []Personalization{
				{
					To: []Addressee{}, // Empty To list
				},
			},
			Subject: "Test Subject",
			Content: []Content{
				{Type: "text/plain", Value: "Test body"},
			},
		},
	}

	body, err := m.getMessageBody()
	require.Nil(t, err)

	result := string(body)
	require.Contains(t, result, "From: from@example.com")
	require.NotContains(t, result, "To:") // Should not have To: header
}

func TestReceiverEmailsWithMultipleBcc(t *testing.T) {
	req := &SendRequest{
		Personalizations: []Personalization{
			{
				Bcc: []Addressee{
					{Email: "bcc1@example.com"},
					{Email: "bcc2@example.com"},
				},
			},
		},
	}

	result := receiverEmails(req)
	require.Contains(t, result, "\x01bcc1@example.com\x01")
	require.Contains(t, result, "\x01bcc2@example.com\x01")
}

func TestAddresseesFieldValueUTF8Name(t *testing.T) {
	m := &Message{}

	result := m.addresseesFieldValue([]Addressee{
		{Email: "test@example.com", Name: "日本語"},
	})
	require.Contains(t, result, "test@example.com")
	// UTF-8 name should be encoded
	require.NotEqual(t, "日本語 <test@example.com>", result)
}

func TestGetRecipientsMultiple(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Personalizations: []Personalization{
				{
					To: []Addressee{
						{Email: "to1@example.com"},
						{Email: "to2@example.com"},
					},
					Cc: []Addressee{
						{Email: "cc1@example.com"},
					},
					Bcc: []Addressee{
						{Email: "bcc1@example.com"},
						{Email: "bcc2@example.com"},
					},
				},
			},
		},
	}

	recipients := m.getRecipients()
	require.Equal(t, 5, len(recipients))
}

func TestBuildWhereClausePanicBadQueryKind(t *testing.T) {
	// Test panic for invalid QueryKind
	defer func() {
		r := recover()
		require.NotNil(t, r)
		require.Equal(t, "bad QueryKind", r)
	}()

	leaf := &QLeaf{kind: QueryKind(999), op: QueryEqual, value: "test"}
	buildWhereClause(leaf, []any{})
}

func TestBuildWhereClausePanicBadQueryType(t *testing.T) {
	// Test panic for invalid QueryType
	defer func() {
		r := recover()
		require.NotNil(t, r)
		require.Equal(t, "bad QueryType", r)
	}()

	// Create a QExpr with an invalid qtype
	expr := &QExpr{qtype: QueryType(999), a: nil, b: nil}
	buildWhereClause(expr, []any{})
}

func TestGetMultiContentsWithHeaders(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "text/plain", Value: "Plain text"},
				{Type: "text/html", Value: "<p>HTML</p>"},
			},
		},
	}

	p := Personalization{
		Headers: map[string][]string{
			"X-Custom":  {"value1"},
			"X-Another": {"value2", "value3"},
		},
	}
	var buf bytes.Buffer
	err := m.getMultiContents(p, &buf)
	require.Nil(t, err)

	result := buf.String()
	require.Contains(t, result, "X-Custom: value1")
	require.Contains(t, result, "X-Another: value2")
	require.Contains(t, result, "multipart/alternative")
}

func TestGetMultiContentsWithAttachmentsAndHeaders(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "text/plain", Value: "Plain text"},
			},
			Attachments: []Attachment{
				{Type: "application/pdf", Content: "pdf-content"},
			},
		},
	}

	p := Personalization{
		Headers: map[string][]string{
			"X-Priority": {"1"},
		},
	}
	var buf bytes.Buffer
	err := m.getMultiContents(p, &buf)
	require.Nil(t, err)

	result := buf.String()
	require.Contains(t, result, "X-Priority: 1")
	require.Contains(t, result, "multipart/mixed")
	require.Contains(t, result, "pdf-content")
}

func TestGetMultiContentsMultipleAttachments(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "text/plain", Value: "Email body"},
			},
			Attachments: []Attachment{
				{Type: "image/png", Content: "png-data"},
				{Type: "image/jpeg", Content: "jpeg-data"},
			},
		},
	}

	p := Personalization{}
	var buf bytes.Buffer
	err := m.getMultiContents(p, &buf)
	require.Nil(t, err)

	result := buf.String()
	require.Contains(t, result, "png-data")
	require.Contains(t, result, "jpeg-data")
}

func TestGetMultiContentsMultipleContent(t *testing.T) {
	m := &Message{
		packet: SendRequest{
			Content: []Content{
				{Type: "text/plain", Value: "Plain version"},
				{Type: "text/html", Value: "<b>HTML version</b>"},
				{Type: "text/calendar", Value: "BEGIN:VCALENDAR"},
			},
		},
	}

	p := Personalization{}
	var buf bytes.Buffer
	err := m.getMultiContents(p, &buf)
	require.Nil(t, err)

	result := buf.String()
	require.Contains(t, result, "Plain version")
	require.Contains(t, result, "<b>HTML version</b>")
	require.Contains(t, result, "BEGIN:VCALENDAR")
}
