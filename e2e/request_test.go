//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sony/micro-mail-sender/mailsender"
	"github.com/stretchr/testify/assert"
)

const endpoint = "http://localhost:8333"
const mailhogEndpoint = "http://mailhog:8025"
const apiKey = "apikey"

func TestE2E(t *testing.T) {
	params := testMailSend(t)
	testQueryMessages(t, params)
	testSmtplog(t)
}

func testSmtplog(t *testing.T) {
	url := fmt.Sprintf("%s/v3/smtplog?count=1", endpoint)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Add("Authorization", "Bearer "+apiKey)
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = res.Body.Close()
	}()
	assert.Equal(t, 200, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	resBody := struct {
		Count int `json:"count"`
	}{}
	err = json.Unmarshal(body, &resBody)
	if err != nil {
		panic(err)
	}
	assert.Equal(t, 1, resBody.Count)
}

func testQueryMessages(t *testing.T, params mailsender.SendRequest) {
	resBody := struct {
		Messages []map[string]any `json:"messages"`
	}{}

	msgID := ""

	// from_email
	{
		query := url.QueryEscape(fmt.Sprintf(`from_email="%s"`, params.From.Email))
		url := fmt.Sprintf("%s/v3/messages?query=%s", endpoint, query)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			panic(err)
		}

		req.Header.Add("Authorization", "Bearer "+apiKey)

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer func() {
			_ = res.Body.Close()
		}()
		assert.Equal(t, 200, res.StatusCode)

		body, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(body, &resBody)
		if err != nil {
			panic(err)
		}

		assert.Equal(t, 1, len(resBody.Messages))
		msgID = resBody.Messages[0]["msg_id"].(string)

	}

	// status
	{
		query := url.QueryEscape(fmt.Sprintf(`from_email="%s" AND status="%s"`, params.From.Email, "sent"))
		url := fmt.Sprintf("%s/v3/messages?query=%s", endpoint, query)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			panic(err)
		}

		req.Header.Add("Authorization", "Bearer "+apiKey)

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer func() {
			_ = res.Body.Close()
		}()
		assert.Equal(t, 200, res.StatusCode)

		body, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(body, &resBody)
		if err != nil {
			panic(err)
		}

		assert.Equal(t, 1, len(resBody.Messages))
	}

	// msg_id
	{
		query := url.QueryEscape(fmt.Sprintf(`msg_id="%s"`, msgID))
		url := fmt.Sprintf("%s/v3/messages?query=%s", endpoint, query)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			panic(err)
		}

		req.Header.Add("Authorization", "Bearer "+apiKey)

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer func() {
			_ = res.Body.Close()
		}()
		assert.Equal(t, 200, res.StatusCode)

		body, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(body, &resBody)
		if err != nil {
			panic(err)
		}

		assert.Equal(t, 1, len(resBody.Messages))
	}

	// to_email
	{
		query := url.QueryEscape(fmt.Sprintf(`to_email="%s"`, params.Personalizations[0].To[0].Email))
		url := fmt.Sprintf("%s/v3/messages?query=%s", endpoint, query)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			panic(err)
		}

		req.Header.Add("Authorization", "Bearer "+apiKey)

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer func() {
			_ = res.Body.Close()
		}()
		assert.Equal(t, 200, res.StatusCode)

		body, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(body, &resBody)
		if err != nil {
			panic(err)
		}

		assert.Equal(t, 1, len(resBody.Messages))
	}

	// subject
	{
		query := url.QueryEscape(fmt.Sprintf(`subject="%s"`, params.Personalizations[0].Subject))
		url := fmt.Sprintf("%s/v3/messages?query=%s", endpoint, query)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			panic(err)
		}

		req.Header.Add("Authorization", "Bearer "+apiKey)

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer func() {
			_ = res.Body.Close()
		}()
		assert.Equal(t, 200, res.StatusCode)

		body, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(body, &resBody)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 1, len(resBody.Messages))
	}

}

func testMailSend(t *testing.T) mailsender.SendRequest {
	uuid, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}

	id := uuid.String()

	params := mailsender.SendRequest{
		Personalizations: []mailsender.Personalization{
			{
				To: []mailsender.Addressee{{
					Email: "to" + id + "@example.com",
				}},
				Subject: id,
			},
		},
		From: mailsender.Addressee{
			Email: "from" + id + "@example.com",
		},
		Content: []mailsender.Content{
			{
				Type:  "text/plain",
				Value: id + " test mail body",
			},
		},
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		panic(err)
	}

	url := fmt.Sprintf("%s/v3/mail/send", endpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(paramsJSON)))
	if err != nil {
		panic(err)
	}

	req.Header.Add("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = res.Body.Close()
	}()
	assert.Equal(t, http.StatusAccepted, res.StatusCode)

	{
		resBody := struct {
			Count int `json:"count"`
		}{}

		time.Sleep(1 * time.Second)
		hogURL := fmt.Sprintf("%s/api/v2/search?kind=containing&query=%s", mailhogEndpoint, id)
		req, err := http.NewRequest("GET", hogURL, nil)
		if err != nil {
			panic(err)
		}
		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer func() {
			_ = res.Body.Close()
		}()
		assert.Equal(t, 200, res.StatusCode)

		body, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(body, &resBody)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, 1, resBody.Count)
	}

	return params
}
