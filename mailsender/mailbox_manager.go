package mailsender

import (
	"net/mail"
)

type mailboxManager interface {
	hasUnreadLocalMail(app *App) bool
	fetchLocalMail(app *App) (data []byte, rerr error)
	parseLocalMail(app *App, data []byte) (*mail.Message, error)
	getFailedMessageID(app *App, msg *mail.Message) string
}

type mailboxMgr struct{}

func newMailboxManager() mailboxManager {
	return &mailboxMgr{}
}

func (s *mailboxMgr) hasUnreadLocalMail(app *App) bool {
	return hasUnreadLocalMail(app)
}
func (s *mailboxMgr) fetchLocalMail(app *App) (data []byte, rerr error) {
	return fetchLocalMail(app)
}

func (s *mailboxMgr) parseLocalMail(app *App, data []byte) (*mail.Message, error) {
	return parseLocalMail(app, data)
}

func (s *mailboxMgr) getFailedMessageID(app *App, msg *mail.Message) string {
	return getFailedMessageID(app, msg)
}
