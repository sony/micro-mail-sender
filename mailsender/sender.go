package mailsender

import (
	"fmt"
	"time"

	smtp "github.com/emersion/go-smtp"
)

func senderLoop(app *App) {
	var waitus int64
	for {
		m, err := dequeueMessage(app)
		if err != nil {
			app.logger.Errorw("dequeueMessage failed",
				"error", err)
			waitus = 500
		} else if m == nil {
			waitus = 500
		} else {
			waitus = 50
			err := sendMesg(app, m)
			if err != nil {
				app.logger.Errorw("message send failed",
					"error", err)
			}
		}
		time.Sleep(time.Duration(waitus) * time.Millisecond)
	}
}

func runSenderLoop(app *App) {
	go senderLoop(app)
}

func sendMesg(app *App, m *Message) error {
	smtpserver := fmt.Sprintf("localhost:%d", app.config.SmtpPort)
	err := sendLocal(smtpserver, m)
	if err != nil {
		e := m.abandonMessage(app, err.Error())
		if e != nil {
			app.logger.Errorw("Error on abandoning message",
				"uid", m.uid,
				"error", e.Error())
		}
		return err
	}
	e := m.sentMessage(app)
	if e != nil {
		app.logger.Errorw("Error on marking message sent",
			"uid", m.uid,
			"error", e.Error())
		return e
	}
	return nil
}

func sendLocal(smtpserver string, m *Message) (rerr error) {
	clnt, err := smtp.Dial(smtpserver)
	if err != nil {
		return err
	}
	defer func() {
		rerr = appendError(rerr, clnt.Close())
	}()

	err = clnt.Hello("localhost")
	if err != nil {
		return err
	}

	err = clnt.Mail(m.packet.From.Email, nil)
	if err != nil {
		return err
	}

	for _, rcpt := range m.getRecipients() {
		err = clnt.Rcpt(rcpt)
		if err != nil {
			return err
		}
	}

	w, err := clnt.Data()
	if err != nil {
		return err
	}

	msg, err := m.getMessageBody()
	if err != nil {
		return err
	}
	nwritten := 0
	for {
		n, err := w.Write(msg[nwritten:])
		if err != nil {
			return err
		}
		if nwritten+n == len(msg) {
			break
		}
		nwritten += n
	}

	err = w.Close()
	if err != nil {
		return err
	}
	return clnt.Quit()
}
