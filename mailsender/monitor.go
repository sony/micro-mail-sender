package mailsender

import (
	"time"
)

// try fetch and process one message.  returns a flag whether a mail
// is processed, and next wait time in us.
func monitor1(app *App, mailbox mailboxManager, waitus int64) (bool, int64) {
	time.Sleep(time.Duration(waitus) * time.Millisecond)

	if !mailbox.hasUnreadLocalMail(app) {
		return false, 500
	}

	data, err := mailbox.fetchLocalMail(app)
	if err != nil {
		app.logger.Warnf("failed to fetch local mail: %+v", err)
		return false, 50
	}

	msg, err := parseLocalMail(app, data)
	if err != nil {
		app.logger.Warnw("failed to parse local mail: %+v %s", err, string(data))
		return false, 50
	}
	if msg == nil {
		app.logger.Debugw("no mail")
		return false, 500
	}

	msgid := getFailedMessageID(app, msg)
	if msgid != "" {
		m, err := getMessage(app, msgid)
		if err != nil {
			app.logger.Warnw("failed to retrieve message: %s %+v", msgid, err)
			return false, 50
		}
		if m == nil {
			app.logger.Infow("no message corresponding to returned message id",
				"uid", msgid)
			return false, 50
		}
		err = m.abandonMessage(app, "Undeliverable")
		if err != nil {
			app.logger.Warnw("failed to abandon message: %s %+v", msgid, err)
			return false, 50
		}
		return true, 50
	}

	app.logger.Debugw("ignore unrelated local mail")
	return false, 50
}

func monitorLoop(app *App) {
	waitus := int64(0)
	mailbox := newMailboxManager()
	for {
		_, waitus = monitor1(app, mailbox, waitus)
	}
}

func runMonitorLoop(app *App) {
	go monitorLoop(app)
}
