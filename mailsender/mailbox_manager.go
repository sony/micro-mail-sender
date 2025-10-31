package mailsender

type mailboxManager interface {
	hasUnreadLocalMail(app *App) bool
	fetchLocalMail(app *App) (data []byte, rerr error)
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
