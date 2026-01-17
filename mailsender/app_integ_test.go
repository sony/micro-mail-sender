//go:build integration

package mailsender

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	simplejson "github.com/bitly/go-simplejson"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestApp(t *testing.T, tconf *TestConfig) *TestApp {
	tapp := initTestBase(t, tconf)
	require.NotNil(t, tapp)
	return tapp
}

func doRequest(t *testing.T, router *mux.Router,
	method string, path string, body string,
	expectedCode int) *httptest.ResponseRecorder {
	req, err := http.NewRequest(method, path, strings.NewReader(body))
	require.Nil(t, err)

	if expectedCode != http.StatusForbidden {
		req.Header["Authorization"] = []string{"Bearer apikey"}
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, expectedCode, rr.Code,
		fmt.Sprintf("%s %s: %s", method, path, body))
	return rr
}

func jsonBody(t *testing.T, rr *httptest.ResponseRecorder) *simplejson.Json {
	json, err := simplejson.NewJson(rr.Body.Bytes())
	require.Nil(t, err)
	return json
}

func J(jsonstr string) *simplejson.Json {
	json, err := simplejson.NewJson([]byte(jsonstr))
	if err != nil {
		log.Fatalf("Json literal parse error: %v", err)
	}
	return json
}

func TestMailSendQueue(t *testing.T) {
	tapp := initTestApp(t, nil)
	defer tapp.Fini()
	router := newRouter(tapp.app)

	{
		doRequest(t, router, "POST", "/v3/mail/send",
			`{`+
				`"personalizations":[`+
				`  {"to":[{`+
				`          "email":"to@example.com"`+
				`         }],`+
				`   "subject":"test mail"`+
				`  }`+
				`],`+
				`"from": { "email":"from@example.com" },`+
				`"content": [`+
				`  { "type":"text/plain",`+
				`    "value":"test mail body"`+
				`  }`+
				`]`+
				`}`,
			http.StatusAccepted)
	}

	{
		doRequest(t, router, "POST", "/v3/mail/send",
			`{invalid`,
			http.StatusBadRequest)
	}

	{
		doRequest(t, router, "POST", "/v3/mail/send", `{}`, http.StatusForbidden)
	}
}

// Remove empty values to make it easier to compare.
// simplejson doesn't provide a constructor from map[]interface{}, so we strip
// simplejson and returns internal map.
func pruneJSONMap(json *simplejson.Json) map[string]interface{} {
	m, err := json.Map()
	if err != nil || m == nil {
		return nil
	}
	x, ok := pruneJSONItem(m).(map[string]interface{})
	if !ok {
		return nil
	}
	return x
}

func pruneJSONItem(x interface{}) interface{} {
	if m, ok := x.(map[string]interface{}); ok {
		m1 := map[string]interface{}{}
		for key, val := range m {
			// these two fields get random values so we
			// overwrites them
			if key == "last_timestamp" {
				m1[key] = 0
			} else if key == "msg_id" {
				m1[key] = "XXX"
			} else if val != nil && val != "" && val != "0" {
				v1 := pruneJSONItem(val)
				if v1 != nil && v1 != "" && v1 != "0" {
					m1[key] = v1
				}
			}
		}

		if len(m1) > 0 {
			return m1
		}

		return nil
	}
	if a, ok := x.([]interface{}); ok {
		var a1 []interface{}
		for _, val := range a {
			v1 := pruneJSONItem(val)
			if v1 != nil {
				a1 = append(a1, v1)
			}
		}
		if a1 != nil {
			return a1
		}

		return nil
	}
	return x
}

func TestAppMessages(t *testing.T) {
	tapp := initTestApp(t, nil)
	defer tapp.Fini()
	router := newRouter(tapp.app)

	e1 := `{` +
		`"personalizations":[` +
		`  {"to":[{` +
		`          "email":"to@example.com"` +
		`         }],` +
		`   "subject":"test mail",` +
		`   "send_at":0` +
		`  }` +
		`],` +
		`"from": { "email":"from@example.com" },` +
		`"content": [` +
		`  { "type":"text/plain",` +
		`    "value":"test mail body"` +
		`  }` +
		`],` +
		`"send_at":0` +
		`}`

	f1 := `{` +
		`"from_email": "from@example.com",` +
		`"msg_id":"XXX",` +
		`"subject":"test mail",` +
		`"to_email":"to@example.com",` +
		`"status": "waiting",` +
		`"last_timestamp":0` +
		`}`

	doRequest(t, router, "POST", "/v3/mail/send", e1,
		http.StatusAccepted)

	rr := doRequest(t, router, "GET",
		"/v3/messages?query=status%3D%22waiting%22", "",
		http.StatusOK)

	require.Equal(t, pruneJSONMap(J(`{"messages":[`+f1+`]}`)),
		pruneJSONMap(jsonBody(t, rr)))

	rr = doRequest(t, router, "GET",
		"/v3/messages?query=from_email%3D%22from@example.com%22", "",
		http.StatusOK)
	require.Equal(t, pruneJSONMap(J(`{"messages":[`+f1+`]}`)),
		pruneJSONMap(jsonBody(t, rr)))

	rr = doRequest(t, router, "GET",
		"/v3/messages?query=to_email%3D%22to@example.com%22", "",
		http.StatusOK)
	require.Equal(t, pruneJSONMap(J(`{"messages":[`+f1+`]}`)),
		pruneJSONMap(jsonBody(t, rr)))

	rr = doRequest(t, router, "GET",
		"/v3/messages?query=from_email!%3D%22from@example.com%22", "",
		http.StatusOK)
	require.Equal(t, pruneJSONMap(J(`{"messages":[]}`)),
		pruneJSONMap(jsonBody(t, rr)))

	rr = doRequest(t, router, "GET",
		"/v3/messages?query=from_email%3D%22from@example.com%22%20AND%20to_email%3D%22to@example.com%22", "",
		http.StatusOK)
	require.Equal(t, pruneJSONMap(J(`{"messages":[`+f1+`]}`)),
		pruneJSONMap(jsonBody(t, rr)))

	// clean message body
	jb, err := jsonBody(t, rr).Map()
	require.Nil(t, err)
	msgid := jb["messages"].([]interface{})[0].(map[string]interface{})["msg_id"]
	msgidS, ok := msgid.(string)
	require.True(t, ok)
	msg, err := getMessage(tapp.app, msgidS)
	require.Nil(t, err)
	err = msg.cleanMessageBody(tapp.app)
	require.Nil(t, err)

	// search empty bodied message
	rr = doRequest(t, router, "GET",
		"/v3/messages?query=status%3D%22waiting%22", "",
		http.StatusOK)
	require.Equal(t, pruneJSONMap(J(`{"messages":[`+f1+`]}`)),
		pruneJSONMap(jsonBody(t, rr)))

	doRequest(t, router, "GET", "/v3/messages?query=very-=invalid=invalid", "", http.StatusBadRequest)
	doRequest(t, router, "GET", "/v3/messages?query=test", ``, http.StatusForbidden)

}
func TestSMTPLog(t *testing.T) {
	tapp := initTestApp(t, &TestConfig{configOverride: `{"host":"localhost",` +
		`"dbname":"mailsender_test",` +
		`"smtp-log":"../testdata/mail.log.dummy",` +
		`"api-keys":["apikey"]}`})
	defer tapp.Fini()

	router := newRouter(tapp.app)
	rr := doRequest(t, router, "GET", "/v3/smtplog?count=5", "",
		http.StatusOK)
	require.Equal(t, pruneJSONMap(J(`{"count":5,`+
		`"lines":["line 2","line 3","line 4","line 5","line 6"]}`)),
		pruneJSONMap(jsonBody(t, rr)))
}

func TestHello(t *testing.T) {
	tapp := initTestApp(t, &TestConfig{configOverride: `{"host":"localhost",` +
		`"dbname":"mailsender_test",` +
		`"smtp-log":"../testdata/mail.log.dummy",` +
		`"api-keys":["apikey"]}`})
	defer tapp.Fini()

	router := newRouter(tapp.app)
	rr := doRequest(t, router, "GET", "/", "",
		http.StatusOK)
	require.Equal(t, pruneJSONMap(J(`{"version":"1"}`)),
		pruneJSONMap(jsonBody(t, rr)))
}

func TestReturnErr(t *testing.T) {
	tapp := initTestBase(t, nil)
	defer tapp.Fini()

	{
		rr := httptest.NewRecorder()
		returnErr(tapp.app, rr, &AppError{Code: http.StatusInternalServerError})
		require.Equal(t, http.StatusInternalServerError, rr.Result().StatusCode)
	}

	{
		rr := httptest.NewRecorder()
		returnErr(tapp.app, rr, &AppError{Code: http.StatusBadRequest})
		require.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	}
}

func Test_newServer(t *testing.T) {
	tapp := initTestBase(t, nil)
	defer tapp.Fini()
	server := newServer(tapp.app)
	tapp.app.Fini()
	require.NotNil(t, server)
}

func Test_newApp(t *testing.T) {
	{
		config, err := ParseConfig(`{"host":"localhost","dbname":"mailsender_test"}`)
		require.Nil(t, err)
		app := newApp(config)
		require.NotNil(t, app)
	}

	{
		config, err := ParseConfig(`{"dbname":"notfound"}`)
		require.Nil(t, err)
		assert.Panics(t, func() { newApp(config) })
	}
}
