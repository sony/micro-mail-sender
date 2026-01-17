package mailsender

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func createTestApp(t *testing.T) *App {
	logger, err := zap.NewDevelopment()
	require.Nil(t, err)

	config := &Config{
		AppIDs: []string{"valid-api-key", "another-key"},
	}

	return &App{
		config: config,
		logger: logger.Sugar(),
	}
}

func TestReturnJSON(t *testing.T) {
	// Test successful JSON response
	rr := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	returnJSON(rr, data)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var result map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.Nil(t, err)
	require.Equal(t, "value", result["key"])
}

func TestReturnJSONStruct(t *testing.T) {
	rr := httptest.NewRecorder()
	data := SearchResult{
		Messages: []SearchResultItem{
			{MsgID: "123", Status: "sent"},
		},
	}
	returnJSON(rr, data)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "123")
	require.Contains(t, rr.Body.String(), "sent")
}

func TestReturnJSONMarshalError(t *testing.T) {
	rr := httptest.NewRecorder()
	// Create an unmarshallable value (function)
	returnJSON(rr, func() {})

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Contains(t, rr.Body.String(), "failed to parse")
}

func TestReturnErrBadRequest(t *testing.T) {
	app := createTestApp(t)

	// Test 400 error
	rr := httptest.NewRecorder()
	apperr := AppErr(http.StatusBadRequest, "bad request")
	returnErr(app, rr, apperr)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), "bad request")
}

func TestReturnErrInternalServerError(t *testing.T) {
	app := createTestApp(t)

	rr := httptest.NewRecorder()
	apperr := AppErr(http.StatusInternalServerError, "internal error")
	returnErr(app, rr, apperr)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Contains(t, rr.Body.String(), "internal error")
}

func TestReturnErrForbidden(t *testing.T) {
	app := createTestApp(t)

	rr := httptest.NewRecorder()
	apperr := AppErr(http.StatusForbidden, "access denied")
	returnErr(app, rr, apperr)

	require.Equal(t, http.StatusForbidden, rr.Code)
	require.Contains(t, rr.Body.String(), "access denied")
}

func TestCheckApikeyValid(t *testing.T) {
	app := createTestApp(t)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-api-key")

	apperr := app.checkApikey(req)
	require.Nil(t, apperr)
}

func TestCheckApikeyValidAlternate(t *testing.T) {
	app := createTestApp(t)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer another-key")

	apperr := app.checkApikey(req)
	require.Nil(t, apperr)
}

func TestCheckApikeyNoHeader(t *testing.T) {
	app := createTestApp(t)

	req := httptest.NewRequest("GET", "/", nil)
	// No Authorization header

	apperr := app.checkApikey(req)
	require.NotNil(t, apperr)
	require.Equal(t, 403, apperr.Code)
	require.Equal(t, "no api key given", apperr.Message)
}

func TestCheckApikeyInvalid(t *testing.T) {
	app := createTestApp(t)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")

	apperr := app.checkApikey(req)
	require.NotNil(t, apperr)
	require.Equal(t, 403, apperr.Code)
	require.Equal(t, "unrecognized api key", apperr.Message)
}

func TestCheckApikeyBearerWithSpaces(t *testing.T) {
	app := createTestApp(t)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer   valid-api-key")

	apperr := app.checkApikey(req)
	require.Nil(t, apperr)
}

func TestHHello(t *testing.T) {
	app := createTestApp(t)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	app.hHello(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.Nil(t, err)
	require.Equal(t, "1", result["version"])
}

func TestNewRouter(t *testing.T) {
	app := createTestApp(t)
	router := newRouter(app)
	require.NotNil(t, router)
}

func TestBearerRegexp(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Bearer mytoken", "mytoken"},
		{"Bearer   spaced-token", "spaced-token"},
		{"Bearer", ""},
		{"Bearer abc123", "abc123"},
	}

	for _, tc := range testCases {
		result := bearerRegexp.ReplaceAllString(tc.input, "$1")
		require.Equal(t, tc.expected, result, "Input: %s", tc.input)
	}
}

func TestCreateLogger(t *testing.T) {
	logger, err := createLogger()
	require.Nil(t, err)
	require.NotNil(t, logger)
}

func TestErrorResponse(t *testing.T) {
	resp := ErrorResponse{
		Errors: []Error{
			{Message: "error1"},
			{Message: "error2"},
		},
	}

	data, err := json.Marshal(resp)
	require.Nil(t, err)
	require.Contains(t, string(data), "error1")
	require.Contains(t, string(data), "error2")
}

func TestErrorResponseWithField(t *testing.T) {
	field := "email"
	resp := ErrorResponse{
		Errors: []Error{
			{Message: "invalid email", Field: &field},
		},
	}

	data, err := json.Marshal(resp)
	require.Nil(t, err)
	require.Contains(t, string(data), "invalid email")
	require.Contains(t, string(data), "email")
}

func TestAddresseeSerialization(t *testing.T) {
	addr := Addressee{
		Email: "test@example.com",
		Name:  "Test User",
	}

	data, err := json.Marshal(addr)
	require.Nil(t, err)
	require.Contains(t, string(data), "test@example.com")
	require.Contains(t, string(data), "Test User")
}

func TestPersonalizationSerialization(t *testing.T) {
	p := Personalization{
		To:      []Addressee{{Email: "to@example.com"}},
		Cc:      []Addressee{{Email: "cc@example.com"}},
		Subject: "Test Subject",
	}

	data, err := json.Marshal(p)
	require.Nil(t, err)
	require.Contains(t, string(data), "to@example.com")
	require.Contains(t, string(data), "cc@example.com")
	require.Contains(t, string(data), "Test Subject")
}

func TestContentSerialization(t *testing.T) {
	c := Content{
		Type:  "text/plain",
		Value: "Hello World",
	}

	data, err := json.Marshal(c)
	require.Nil(t, err)
	require.Contains(t, string(data), "text/plain")
	require.Contains(t, string(data), "Hello World")
}

func TestAttachmentSerialization(t *testing.T) {
	a := Attachment{
		Content:     "base64data",
		Type:        "image/png",
		Filename:    "image.png",
		Disposition: "attachment",
		ContentID:   "cid123",
	}

	data, err := json.Marshal(a)
	require.Nil(t, err)
	require.Contains(t, string(data), "base64data")
	require.Contains(t, string(data), "image/png")
	require.Contains(t, string(data), "image.png")
}

func TestSendRequestSerialization(t *testing.T) {
	sr := SendRequest{
		Personalizations: []Personalization{
			{To: []Addressee{{Email: "to@example.com"}}},
		},
		From:    Addressee{Email: "from@example.com"},
		Subject: "Test",
		Content: []Content{{Type: "text/plain", Value: "Body"}},
	}

	data, err := json.Marshal(sr)
	require.Nil(t, err)
	require.Contains(t, string(data), "to@example.com")
	require.Contains(t, string(data), "from@example.com")
}

func TestSearchResultItemSerialization(t *testing.T) {
	item := SearchResultItem{
		FromEmail:     "from@example.com",
		MsgID:         "msg123",
		Subject:       "Test Subject",
		ToEmail:       "to@example.com",
		Status:        "sent",
		LastTimestamp: 1234567890,
	}

	data, err := json.Marshal(item)
	require.Nil(t, err)
	require.Contains(t, string(data), "from@example.com")
	require.Contains(t, string(data), "msg123")
	require.Contains(t, string(data), "sent")
}

func TestSearchResultSerialization(t *testing.T) {
	sr := SearchResult{
		Messages: []SearchResultItem{
			{MsgID: "msg1", Status: "sent"},
			{MsgID: "msg2", Status: "waiting"},
		},
	}

	data, err := json.Marshal(sr)
	require.Nil(t, err)
	require.Contains(t, string(data), "msg1")
	require.Contains(t, string(data), "msg2")
}

func TestReturnJSONEmptyData(t *testing.T) {
	rr := httptest.NewRecorder()
	returnJSON(rr, nil)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "null", rr.Body.String())
}

func TestReturnJSONComplexNestedStruct(t *testing.T) {
	rr := httptest.NewRecorder()
	data := SendRequest{
		Personalizations: []Personalization{
			{
				To:      []Addressee{{Email: "to1@example.com", Name: "To User 1"}},
				Cc:      []Addressee{{Email: "cc@example.com"}},
				Bcc:     []Addressee{{Email: "bcc@example.com"}},
				Subject: "Personalized Subject",
				Headers: map[string][]string{"X-Custom": {"value1", "value2"}},
			},
		},
		From:    Addressee{Email: "from@example.com", Name: "Sender"},
		ReplyTo: Addressee{Email: "reply@example.com"},
		Subject: "Default Subject",
		Content: []Content{
			{Type: "text/plain", Value: "Plain text"},
			{Type: "text/html", Value: "<p>HTML</p>"},
		},
		Attachments: []Attachment{
			{Content: "base64", Type: "image/png", Filename: "img.png"},
		},
	}
	returnJSON(rr, data)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "to1@example.com")
	require.Contains(t, rr.Body.String(), "Personalized Subject")
}

func TestReturnErrWithInternalError(t *testing.T) {
	app := createTestApp(t)

	rr := httptest.NewRecorder()
	// Create an AppError with an internal error
	apperr := WrapErr(http.StatusInternalServerError, http.ErrContentLength)
	returnErr(app, rr, apperr)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestReturnErrNotFound(t *testing.T) {
	app := createTestApp(t)

	rr := httptest.NewRecorder()
	apperr := AppErr(http.StatusNotFound, "resource not found")
	returnErr(app, rr, apperr)

	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Contains(t, rr.Body.String(), "resource not found")
}

func TestReturnErrUnauthorized(t *testing.T) {
	app := createTestApp(t)

	rr := httptest.NewRecorder()
	apperr := AppErr(http.StatusUnauthorized, "unauthorized")
	returnErr(app, rr, apperr)

	require.Equal(t, http.StatusUnauthorized, rr.Code)
	require.Contains(t, rr.Body.String(), "unauthorized")
}
