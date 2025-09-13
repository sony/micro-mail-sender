package mailsender

import (
	"bytes"
	"io/ioutil"
	"net/mail"
	"os/exec"
	"regexp"
)

// Return true if there're unread mails
func hasUnreadLocalMail(app *App) bool {
	err := exec.Command("/usr/bin/mail", "-e").Run()
	return err == nil
}

// Retrieve one mail.  If there's no mail, "No mail for ..." is returned.
func fetchLocalMail(app *App) (data []byte, rerr error) {
	cmd := exec.Command("/usr/bin/mail")

	stdin, _ := cmd.StdinPipe()
	defer func() {
		rerr = appendError(rerr, stdin.Close())
	}()
	stdout, _ := cmd.StdoutPipe()
	defer func() {
		rerr = appendError(rerr, stdout.Close())
	}()

	err := cmd.Start()
	if err != nil {
		return nil, appendError(rerr, err)
	}

	_, err = stdin.Write([]byte("type 1"))
	if err != nil {
		rerr = appendError(rerr, err)
		rerr = appendError(rerr, stdin.Close())
		rerr = appendError(rerr, cmd.Wait())
		return nil, rerr
	}
	rerr = appendError(rerr, stdin.Close())

	defer func() {
		rerr = appendError(rerr, cmd.Wait())
	}()

	out, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, appendError(rerr, err)
	}

	x, err := regexp.Match("^No mail for", out)
	if err != nil {
		return nil, appendError(rerr, err)
	}
	if x {
		return nil, nil
	}

	return out, nil
}

// Parse
// The output consists of messages from 'mail' program, followed
// by the actual message.
//
//	"/var/mail/root": 3 messages 3 new
//	>N  1 mailer-daemon@example.com ...
//	 N  2 root@example.com   ...
//	 N  3 mailer-daemon@example.com ...
//	Return-Path: <mailer-daemon@example.com>
//	Delivered-To: root@localhost
//	Received: ....
//
// We skip the first part, then parse the rest with net/mail.
//
// It can reutrn nil, nil if the data says "No mail for ...".
func parseLocalMail(app *App, data []byte) (*mail.Message, error) {
	if m, _ := regexp.Match("^No mail for ", data); m {
		return nil, nil
	}

	ind := regexp.MustCompile(`(?m)^[A-Za-z0-9-]+: *`).FindIndex(data)
	if ind == nil {
		return nil, nil
	}

	return mail.ReadMessage(bytes.NewReader(data[ind[0]:]))
}

func getFailedMessageId(app *App, msg *mail.Message) string {
	from := msg.Header.Get("from")
	addr, err := mail.ParseAddress(from)
	if err != nil {
		app.logger.Debugw("Invalid from address in reply mail",
			"from", from)
		return ""
	}

	m, err := regexp.MatchString("^mailer-daemon@", addr.Address)
	if err != nil {
		app.logger.Debugw("Failed to match from address",
			"from", from, "err", err)
		return ""
	}
	if !m {
		// no, it's not from mailer-daemon
		return ""
	}

	// The original message id is in one of the MIME parts.  For now,
	// we assume there's no other parts that contains Message-ID header.
	body, err := ioutil.ReadAll(msg.Body)
	if err != nil {
		app.logger.Debugw("Failed to read message body",
			"err", err)
		return ""
	}

	b := regexp.
		MustCompile(`(?mi)^Message-ID:[ \t]*<(\S*)>$`).
		FindSubmatch(body)
	if b == nil {
		// no message-id
		return ""
	}
	if len(b) != 2 {
		return ""
	}
	return string(b[1])
}
