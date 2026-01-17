package mailsender

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSampleSendRequest(t *testing.T) {
	// Test all sample request variants
	testCases := []struct {
		id          int
		expectedTo  string
		expectedCc  string
		hasAttach   bool
		contentType string
	}{
		{0, "foo@example.com", "bar@example.com", false, "text/plain"},
		{1, "foo@example.com", "bar@example.com", false, "text/plain"},
		{2, "foo@example.com", "bar@example.com", false, "text/html"},
		{3, "foo@example.com", "bar@example.com", false, "text/html"},
		{4, "foo@example.com", "bar@example.com", true, "text/plain"},
	}

	for _, tc := range testCases {
		req := sampleSendRequest(tc.id)
		require.NotNil(t, req.Personalizations)
		require.True(t, len(req.Personalizations) > 0)
		require.Equal(t, tc.expectedTo, req.Personalizations[0].To[0].Email)
		if len(req.Personalizations[0].Cc) > 0 {
			require.Equal(t, tc.expectedCc, req.Personalizations[0].Cc[0].Email)
		}
		require.Equal(t, "admin@example.com", req.From.Email)
		if tc.hasAttach {
			require.True(t, len(req.Attachments) > 0)
		}
	}
}

func TestSampleSendRequestDefault(t *testing.T) {
	// Test the default case (any number not in the switch)
	req := sampleSendRequest(999)
	require.NotNil(t, req)
}

func TestRemoveMessageIDFromText(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			"Message-Id: <test@example.com>\r\nFrom: sender",
			"From: sender",
		},
		{
			"Message-Id: <uuid-123@domain.com>\nSubject: Test",
			"Subject: Test",
		},
		{
			"No message ID here",
			"No message ID here",
		},
		{
			"From: sender\r\nMessage-Id: <abc@xyz>\r\nTo: receiver",
			"From: sender\r\nTo: receiver",
		},
	}

	for _, tc := range testCases {
		result := removeMessageIDFromText(tc.input)
		require.Equal(t, tc.expected, result)
	}
}

func TestRemoveMultipartBoundaryFromText(t *testing.T) {
	testCases := []struct {
		input       string
		description string
	}{
		{
			"Content-Type: multipart/alternative; boundary=abc123\r\n--abc123\r\nContent",
			"removes boundary from multipart/alternative",
		},
		{
			"Content-Type: multipart/mixed; boundary=def456\r\n--def456\r\nBody",
			"removes boundary from multipart/mixed",
		},
		{
			"No boundary here",
			"leaves text without boundary unchanged",
		},
	}

	for _, tc := range testCases {
		result := removeMultipartBoundaryFromText(tc.input)
		// Result should not contain the boundary value
		require.NotContains(t, result, "abc123")
		require.NotContains(t, result, "def456")
	}
}

func TestNewMockMailboxManager(t *testing.T) {
	// Test creating mock with default options
	mgr := newMockMailboxManager(mockMailboxMgrOpt{})
	require.NotNil(t, mgr)

	// Verify it's the correct type
	_, ok := mgr.(*mockMailboxMgr)
	require.True(t, ok)
}

func TestMockMailboxMgrHasUnreadLocalMail(t *testing.T) {
	// Test with false result
	mgr := newMockMailboxManager(mockMailboxMgrOpt{hasUnreadLocalMailResult: false})
	require.False(t, mgr.hasUnreadLocalMail(nil))

	// Test with true result
	mgr = newMockMailboxManager(mockMailboxMgrOpt{hasUnreadLocalMailResult: true})
	require.True(t, mgr.hasUnreadLocalMail(nil))
}

func TestMockMailboxMgrFetchLocalMail(t *testing.T) {
	// Test successful fetch
	mgr := newMockMailboxManager(mockMailboxMgrOpt{failedFetchLocalMail: false})
	data, err := mgr.fetchLocalMail(nil)
	require.Nil(t, err)
	require.Equal(t, []byte("data"), data)

	// Test failed fetch
	mgr = newMockMailboxManager(mockMailboxMgrOpt{failedFetchLocalMail: true})
	data, err = mgr.fetchLocalMail(nil)
	require.NotNil(t, err)
	require.Nil(t, data)
}

func TestMockMailboxMgrParseLocalMail(t *testing.T) {
	// Test successful parse with nil result
	mgr := newMockMailboxManager(mockMailboxMgrOpt{failedParseLocalMail: false, parseLocalMailValue: nil})
	msg, err := mgr.parseLocalMail(nil, nil)
	require.Nil(t, err)
	require.Nil(t, msg)

	// Test failed parse
	mgr = newMockMailboxManager(mockMailboxMgrOpt{failedParseLocalMail: true})
	msg, err = mgr.parseLocalMail(nil, nil)
	require.NotNil(t, err)
	require.Nil(t, msg)
}

func TestMockMailboxMgrGetFailedMessageID(t *testing.T) {
	// Test with empty result
	mgr := newMockMailboxManager(mockMailboxMgrOpt{getFailedMessageIDValue: ""})
	msgid := mgr.getFailedMessageID(nil, nil)
	require.Equal(t, "", msgid)

	// Test with value
	mgr = newMockMailboxManager(mockMailboxMgrOpt{getFailedMessageIDValue: "test-id"})
	msgid = mgr.getFailedMessageID(nil, nil)
	require.Equal(t, "test-id", msgid)
}
