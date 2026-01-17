package mailsender

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHasNoMail(t *testing.T) {
	require.True(t, hasNoMail("No mail for xxx"))
	require.True(t, hasNoMail("No mail for john"))
	require.True(t, hasNoMail("No mail for user@example.com"))
	require.False(t, hasNoMail("mail for xxx"))
	require.False(t, hasNoMail("You have mail"))
	require.False(t, hasNoMail(""))
	require.False(t, hasNoMail("No Mail for xxx")) // case sensitive
}

func createTestLogger(t *testing.T) *zap.SugaredLogger {
	logger, err := zap.NewDevelopment()
	require.Nil(t, err)
	return logger.Sugar()
}

func TestParseLocalMail(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	testmsg, err := os.ReadFile("../testdata/localmail1.txt")
	require.Nil(t, err)

	msg, err := parseLocalMail(app, testmsg)
	require.Nil(t, err)
	require.NotNil(t, msg)

	// Check that the message was parsed correctly
	from := msg.Header.Get("From")
	require.Contains(t, from, "mailer-daemon")
}

func TestParseLocalMailNoMail(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	// Test with "No mail for ..." message
	msg, err := parseLocalMail(app, []byte("No mail for user"))
	require.Nil(t, err)
	require.Nil(t, msg)
}

func TestParseLocalMailNoHeader(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	// Test with data that doesn't contain mail headers
	msg, err := parseLocalMail(app, []byte("just some random text without headers"))
	require.Nil(t, err)
	require.Nil(t, msg)
}

func TestGetFailedMessageIDInvalidFrom(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	// Create a mock mail message with invalid From address
	testmail := `From: invalid-address
To: test@example.com
Subject: Test

Body`
	msg, err := parseLocalMail(app, []byte("Header: value\r\n"+testmail))
	require.Nil(t, err)
	require.NotNil(t, msg)

	msgid := getFailedMessageID(app, msg)
	require.Equal(t, "", msgid)
}

func TestGetFailedMessageIDNotMailerDaemon(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	// Create a mock mail message that is not from mailer-daemon
	testmail := `From: someone@example.com
To: test@example.com
Subject: Test

Body`
	msg, err := parseLocalMail(app, []byte("Header: value\r\n"+testmail))
	require.Nil(t, err)
	require.NotNil(t, msg)

	msgid := getFailedMessageID(app, msg)
	require.Equal(t, "", msgid)
}

func TestGetFailedMessageIDNoMessageID(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	// Create a mock bounce message without Message-ID in body
	testmail := `From: mailer-daemon@example.com
To: test@example.com
Subject: Delivery Status Notification

The message could not be delivered.
No original message ID found.`
	msg, err := parseLocalMail(app, []byte("Header: value\r\n"+testmail))
	require.Nil(t, err)
	require.NotNil(t, msg)

	msgid := getFailedMessageID(app, msg)
	require.Equal(t, "", msgid)
}

func TestGetFailedMessageIDWithMessageID(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	// Create a mock bounce message with Message-ID in body
	testmail := `From: mailer-daemon@example.com
To: test@example.com
Subject: Delivery Status Notification

The message could not be delivered.
Message-ID: <test-message-id@example.com>
Original message follows.`
	msg, err := parseLocalMail(app, []byte("Header: value\r\n"+testmail))
	require.Nil(t, err)
	require.NotNil(t, msg)

	msgid := getFailedMessageID(app, msg)
	require.Equal(t, "test-message-id@example.com", msgid)
}

func TestLocalMailRegexp(t *testing.T) {
	// Test that the regex correctly finds mail headers
	testCases := []struct {
		input    string
		hasMatch bool
	}{
		{"From: test@example.com", true},
		{"To: test@example.com", true},
		{"Subject: Test", true},
		{"X-Custom-Header: value", true},
		{"Content-Type: text/plain", true},
		{"no colon here", false},
		{"", false},
	}

	for _, tc := range testCases {
		match := localMailRegexp.FindIndex([]byte(tc.input))
		if tc.hasMatch {
			require.NotNil(t, match, "Expected match for: %s", tc.input)
		} else {
			require.Nil(t, match, "Expected no match for: %s", tc.input)
		}
	}
}

func TestMessageIDRegexp(t *testing.T) {
	// Test that the Message-ID regex works correctly
	testCases := []struct {
		input    string
		expected string
	}{
		{"Message-ID: <test@example.com>", "test@example.com"},
		{"Message-Id: <test@example.com>", "test@example.com"},
		{"message-id: <test@example.com>", "test@example.com"},
		{"MESSAGE-ID: <test@example.com>", "test@example.com"},
		{"Message-ID: <uuid-12345@domain.com>", "uuid-12345@domain.com"},
		{"No message ID here", ""},
	}

	for _, tc := range testCases {
		matches := messageIDRegexp.FindSubmatch([]byte(tc.input))
		if tc.expected == "" {
			require.Nil(t, matches, "Expected no match for: %s", tc.input)
		} else {
			require.NotNil(t, matches, "Expected match for: %s", tc.input)
			require.Equal(t, 2, len(matches))
			require.Equal(t, tc.expected, string(matches[1]))
		}
	}
}

func TestParseLocalMailWithRealBounce(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	// Load and parse the real test data
	testmsg, err := os.ReadFile("../testdata/localmail1.txt")
	require.Nil(t, err)

	msg, err := parseLocalMail(app, testmsg)
	require.Nil(t, err)
	require.NotNil(t, msg)

	// Verify the message is parsed as a bounce from mailer-daemon
	from := msg.Header.Get("From")
	require.True(t, strings.Contains(strings.ToLower(from), "mailer-daemon"))

	// Verify the subject indicates a delivery failure
	subject := msg.Header.Get("Subject")
	require.Contains(t, subject, "Delivery Status Notification")
}

func TestParseLocalMailMalformedHeader(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	// Create malformed mail data with a header but invalid format that will cause
	// mail.ReadMessage to fail. The header regex will match, but the message is malformed.
	malformed := []byte("From: test\r\n\x00invalid binary data")

	msg, err := parseLocalMail(app, malformed)
	// This should return an error because the message is malformed
	require.NotNil(t, err)
	require.Nil(t, msg)
}

func TestParseLocalMailEmptyBody(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	// Empty mail with header but no body
	emptyMail := []byte("From: test@example.com\r\nTo: recipient@example.com\r\n\r\n")
	msg, err := parseLocalMail(app, emptyMail)
	require.Nil(t, err)
	require.NotNil(t, msg)
}

func TestParseLocalMailHeaderOnly(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	// Mail with just headers (no body separator)
	headerOnly := []byte("From: test@example.com\r\nSubject: Test\r\n\r\n")
	msg, err := parseLocalMail(app, headerOnly)
	require.Nil(t, err)
	require.NotNil(t, msg)
	require.Equal(t, "test@example.com", msg.Header.Get("From"))
}

func TestGetFailedMessageIDWithTabInHeader(t *testing.T) {
	logger := createTestLogger(t)
	app := &App{logger: logger}

	// Test Message-ID with tab after colon
	testmail := `From: mailer-daemon@example.com
To: test@example.com
Subject: Delivery Status Notification

Message-ID:	<tabbed-id@example.com>
`
	msg, err := parseLocalMail(app, []byte("Header: value\r\n"+testmail))
	require.Nil(t, err)
	require.NotNil(t, msg)

	msgid := getFailedMessageID(app, msg)
	require.Equal(t, "tabbed-id@example.com", msgid)
}

func TestMessageIDRegexpEdgeCases(t *testing.T) {
	// Test edge cases for the Message-ID regex
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"standard", "Message-ID: <test@example.com>", "test@example.com"},
		{"with tab", "Message-ID:\t<tab@example.com>", "tab@example.com"},
		{"multiple spaces", "Message-ID:   <spaces@example.com>", "spaces@example.com"},
		{"complex id", "Message-ID: <uuid-1234-5678@domain.subdomain.com>", "uuid-1234-5678@domain.subdomain.com"},
		{"no angle brackets", "Message-ID: test@example.com", ""},
		{"empty id", "Message-ID: <>", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := messageIDRegexp.FindSubmatch([]byte(tc.input))
			if tc.expected == "" {
				if len(matches) > 1 {
					require.Equal(t, "", string(matches[1]))
				}
			} else {
				require.NotNil(t, matches, "Expected match for: %s", tc.input)
				require.Equal(t, tc.expected, string(matches[1]))
			}
		})
	}
}
