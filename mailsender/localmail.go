package mailsender

import (
	"bytes"
	"io"
	"net/mail"
	"os/exec"
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"
)

// Return true if there're unread mails
func hasUnreadLocalMail(app *App) bool {
	err := exec.Command("/usr/bin/mail", "-e").Run()
	return err == nil
}

// Retrieve one mail.  If there's no mail, "No mail for ..." is returned.
func fetchLocalMail(app *App) (data []byte, rerr error) {
	cmd := exec.Command("/usr/bin/mail")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	defer func() {
		// make sure to close.
		// stdin has already been closed in normal cases.
		_ = stdin.Close()
	}()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	defer func() {
		// make sure to close.
		// stdout has already been closed in normal cases.
		_ = stdout.Close()
	}()

	err = cmd.Start()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	_, err = stdin.Write([]byte("type 1"))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rerr = appendError(rerr, errors.WithStack(stdin.Close()))

	defer func() {
		rerr = appendError(rerr, errors.WithStack(cmd.Wait()))
	}()

	out, err := io.ReadAll(stdout)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if hasNoMail(string(out)) {
		return nil, nil
	}

	return out, nil
}

func hasNoMail(text string) bool {
	return strings.HasPrefix(text, "No mail for")
}

var localMailRegexp = regexp.MustCompile(`(?m)^[A-Za-z0-9-]+: *`)

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
	if hasNoMail(string(data)) {
		return nil, nil
	}

	ind := localMailRegexp.FindIndex(data)
	if ind == nil {
		return nil, nil
	}

	message, err := mail.ReadMessage(bytes.NewReader(data[ind[0]:]))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return message, nil
}

var messageIDRegexp = regexp.MustCompile(`(?mi)^Message-ID:[ \t]*<(\S*)>$`)

func getFailedMessageID(app *App, msg *mail.Message) string {
	from := msg.Header.Get("from")
	addr, err := mail.ParseAddress(from)
	if err != nil {
		app.logger.Debugw("invalid from address in reply mail",
			"from", from)
		return ""
	}

	if !strings.HasPrefix(addr.Address, "mailer-daemon@") {
		// not from mailer daemon
		return ""
	}

	// The original message id is in one of the MIME parts.  For now,
	// we assume there's no other parts that contains Message-ID header.
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		app.logger.Errorf("failed to read message body %+v", err)
		return ""
	}

	b := messageIDRegexp.FindSubmatch(body)
	if b == nil {
		// no message-id
		return ""
	}
	if len(b) != 2 {
		return ""
	}
	return string(b[1])
}
