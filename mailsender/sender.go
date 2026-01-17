package mailsender

import (
	"fmt"
	"net/smtp"
	"time"

	"github.com/cockroachdb/errors"
)

// senderLoop continuously dequeues and sends messages until signaled to stop.
func senderLoop(app *App) {
	var waitus int64
	for {
		select {
		case <-app.quitSenderHandler:
			return
		default:
			m, err := dequeueMessage(app)
			if err != nil {
				app.logger.Errorf("failed to dequeue message: %+v", err)
				waitus = 500
			} else if m == nil {
				waitus = 500
			} else {
				waitus = 50
				err := sendMesg(app, m)
				if err != nil {
					app.logger.Errorf("failed to send message: %+v", err)
				}
			}
			time.Sleep(time.Duration(waitus) * time.Millisecond)
		}
	}
}

// runSenderLoop starts the sender loop in a separate goroutine.
func runSenderLoop(app *App) {
	go senderLoop(app)
}

// sendMesg sends a message via the local SMTP server.
func sendMesg(app *App, m *Message) error {
	smtpServer := fmt.Sprintf("localhost:%d", app.config.SMTPPort)
	err := sendLocal(smtpServer, m)
	if err != nil {
		e := m.abandonMessage(app, err.Error())
		return appendError(err, e)
	}
	return m.sentMessage(app)
}

// sendLocal sends a message to the specified SMTP server.
func sendLocal(smtpServer string, m *Message) (rerr error) {
	clnt, err := smtp.Dial(smtpServer)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		err := clnt.Quit()
		if err != nil {
			rerr = appendError(rerr, errors.WithStack(err))
		}
	}()

	err = clnt.Hello("localhost")
	if err != nil {
		return errors.WithStack(err)
	}

	err = clnt.Mail(m.packet.From.Email)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, rcpt := range m.getRecipients() {
		err = clnt.Rcpt(rcpt)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	w, err := clnt.Data()
	if err != nil {
		return errors.WithStack(err)
	}

	msg, err := m.getMessageBody()
	if err != nil {
		return err
	}
	nwritten := 0
	for {
		n, err := w.Write(msg[nwritten:])
		if err != nil {
			return errors.WithStack(err)
		}
		if nwritten+n == len(msg) {
			break
		}
		nwritten += n
	}

	return errors.WithStack(w.Close())
}
