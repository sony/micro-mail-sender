package mailsender

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	_ "github.com/lib/pq" // require for Open postgres
)

const (
	// MessageWaiting indicates the message is queued and waiting to be sent.
	MessageWaiting = 0
	// MessageProcessing indicates the message is currently being sent.
	MessageProcessing = 1
	// MessageSent indicates the message has been successfully sent.
	MessageSent = 2
	// MessageAbandoned indicates the message sending was abandoned due to errors.
	MessageAbandoned = 3
)

// Message represents an email message stored in the queue.
type Message struct {
	uid        string
	packet     SendRequest
	status     int
	lastUpdate int64
}

// newDB creates a new database connection using the provided configuration.
func newDB(config *Config) (*sql.DB, error) {
	connStr := "host=" + config.DbHost +
		" user=" + config.DbUser +
		" password=" + config.DbPassword +
		" dbname=" + config.DbName +
		" sslmode=" + config.DbSSLMode
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open database connection")
	}
	err = db.Ping()
	if err != nil {
		err2 := db.Close()
		return nil, appendError(errors.WithStack(err), errors.WithStack(err2))
	}
	return db, err
}

// expandPersonalization expands a request with multiple personalizations into
// separate requests, one for each personalization.
func expandPersonalization(req SendRequest) []SendRequest {
	var rs []SendRequest
	for _, p := range req.Personalizations {
		r := req
		r.Personalizations = []Personalization{p}
		rs = append(rs, r)
	}
	return rs
}

// receiverEmails returns a concatenation of emails surrounded by "\x01" for
// substring matching. Personalization must already be expanded in req.
func receiverEmails(req *SendRequest) string {
	var ss []string
	for _, a := range req.Personalizations[0].To {
		if a.Email != "" {
			ss = append(ss, "\x01"+a.Email+"\x01")
		}
	}
	for _, a := range req.Personalizations[0].Cc {
		if a.Email != "" {
			ss = append(ss, "\x01"+a.Email+"\x01")
		}
	}
	for _, a := range req.Personalizations[0].Bcc {
		if a.Email != "" {
			ss = append(ss, "\x01"+a.Email+"\x01")
		}
	}
	return strings.Join(ss, ",")
}

// enqueueMessage1 inserts a single SendRequest into the queue. Must be called
// within a transaction.
func enqueueMessage1(app *App, req SendRequest, tx *sql.Tx) error {
	b, err := json.Marshal(req)
	if err != nil {
		return errors.WithStack(err)
	}
	packet := string(b)
	uid := fmt.Sprintf("%s@%s", uuid.New().String(), app.config.MyDomain)
	lastUpdate := time.Now().Unix()

	bid := uuid.New().String()
	_, err = tx.Exec(`insert into bodies (bid, packet) `+
		`values ($1, $2)`,
		bid, packet)
	if err != nil {
		return errors.WithStack(err)
	}

	subject := req.Subject

	if req.Personalizations[0].Subject != "" {
		subject = req.Personalizations[0].Subject
	}

	_, err = tx.Exec(`insert into messages (`+
		` uid, bid, sender, receivers, subject, status, last_update) `+
		`values ($1, $2, $3, $4, $5, $6, $7)`,
		uid, bid, req.From.Email,
		receiverEmails(&req), subject,
		MessageWaiting, lastUpdate)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// enqueueMessage adds a send request to the message queue.
func enqueueMessage(app *App, req SendRequest) *AppError {
	tx, err := app.db.Begin()
	if err != nil {
		return WrapErr(500, errors.WithStack(err))
	}
	for _, r := range expandPersonalization(req) {
		err = enqueueMessage1(app, r, tx)
		if err != nil {
			err2 := tx.Rollback()
			return WrapErr(500, appendError(err, errors.WithStack(err2)))
		}
	}
	err = tx.Commit()
	if err != nil {
		return WrapErr(500, errors.WithStack(err))
	}
	return nil
}

// extractMessage extracts a message from a database row. It may return a nil
// message if no rows are found.
func extractMessage(app *App, row *sql.Row) (*Message, error) {
	var m Message
	var spacket any
	err := row.Scan(&m.uid, &spacket, &m.status, &m.lastUpdate)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.WithStack(err)
	}
	if spacket == nil {
		m.packet = SendRequest{}
	} else {
		err = json.Unmarshal([]byte(spacket.(string)), &m.packet)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	return &m, nil
}

// getMessage retrieves a message by its unique ID.
func getMessage(app *App, uid string) (*Message, error) {
	row := app.db.QueryRow(`select uid, packet, status, last_update `+
		`from messages `+
		`left join bodies on messages.bid = bodies.bid `+
		`where uid = $1`,
		uid)
	return extractMessage(app, row)
}

// dequeueMessage retrieves and marks a waiting message as processing.
func dequeueMessage(app *App) (*Message, error) {
	row := app.db.QueryRow(`update messages set `+
		`status = $1 `+
		`where uid in (select uid from messages `+
		`              where status = $2 `+
		`              order by last_update `+
		`              limit 1)`+
		`returning uid, bid, status, last_update`,
		MessageProcessing, MessageWaiting)
	var m Message
	var bid string
	err := row.Scan(&m.uid, &bid, &m.status, &m.lastUpdate)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.WithStack(err)
	}
	row = app.db.QueryRow(`select packet from bodies `+
		`where bid = $1`,
		bid)
	var spacket any
	err = row.Scan(&spacket)
	if err != nil {
		if err == sql.ErrNoRows {
			spacket = ""
		}
		return nil, errors.WithStack(err)
	}
	if spacket == nil {
		m.packet = SendRequest{}
	} else {
		err = json.Unmarshal([]byte(spacket.(string)), &m.packet)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	return &m, nil
}

// abandonMessage marks the message as abandoned with the given error message.
func (m *Message) abandonMessage(app *App, errmsg string) error {
	_, err := app.db.Exec(`update messages set `+
		`status = $1, `+
		`last_error = $2, `+
		`last_update = $3 `+
		`where uid = $4`,
		MessageAbandoned, errmsg, time.Now().Unix(), m.uid)
	if err != nil {
		return errors.WithStack(err)
	}
	return m.cleanMessageBody(app)
}

// sentMessage marks the message as successfully sent.
func (m *Message) sentMessage(app *App) error {
	_, err := app.db.Exec(`update messages set `+
		`status = $1, `+
		`last_error = $2, `+
		`last_update = $3 `+
		`where uid = $4`,
		MessageSent, "", time.Now().Unix(), m.uid)
	if err != nil {
		return errors.WithStack(err)
	}
	return m.cleanMessageBody(app)
}

// cleanMessageBody removes the message body if no other messages reference it.
func (m *Message) cleanMessageBody(app *App) (rerr error) {
	// Remove it only iff no other metadata refers to the body
	tx, err := app.db.Begin()
	if err != nil {
		return errors.WithStack(err)
	}

	rows, err := tx.Query(`select uid, bid from messages `+
		`where messages.bid = (select bid from messages `+
		`                      where uid = $1)`,
		m.uid)
	if err != nil {
		err2 := tx.Rollback()
		return appendError(errors.WithStack(err), errors.WithStack(err2))
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			rerr = appendError(rerr, errors.WithStack(err))
		}
	}()

	var uid string
	var bid string
	count := 0
	for rows.Next() {
		err = rows.Scan(&uid, &bid)
		if err != nil {
			err2 := tx.Rollback()
			return appendError(errors.WithStack(err), errors.WithStack(err2))
		}
		count++
	}
	if count == 1 {
		_, err := tx.Exec(`delete from bodies where bid = $1`, bid)
		if err != nil {
			err2 := tx.Rollback()
			return appendError(errors.WithStack(err), errors.WithStack(err2))
		}
	}
	err = tx.Commit()
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// searchMessages searches for messages matching the given criteria.
func searchMessages(app *App, criteria QNode, limit int) (msgs []SearchResultItem, apperr *AppError) {
	q := `select uid, sender, receivers, subject, status, last_update from messages where `
	var params []any
	conds, params := buildWhereClause(criteria, params)
	q = q + conds + " order by last_update desc "
	if limit > 0 {
		q = q + " limit " + strconv.Itoa(limit)
	}

	rows, err := app.db.Query(q, params...)
	if err != nil {
		return []SearchResultItem{}, WrapErr(500, errors.WithStack(err))
	}

	defer func() {
		e := rows.Close()
		if e != nil {
			if apperr != nil {
				apperr.Internal = appendError(apperr.Internal, errors.WithStack(e))
				return
			}
			apperr = WrapErr(500, errors.WithStack(e))
		}
	}()

	var ms []SearchResultItem

	for rows.Next() {

		var m SearchResultItem
		receivers := ""
		status := 0
		err = rows.Scan(&m.MsgID, &m.FromEmail, &receivers, &m.Subject, &status, &m.LastTimestamp)
		if err != nil {
			return []SearchResultItem{}, WrapErr(500, errors.WithStack(err))
		}

		m.Status = getStatusString(status)
		receivers = strings.ReplaceAll(receivers, "\x01", "")
		toList := strings.Split(receivers, ",")
		if len(toList) > 0 {
			m.ToEmail = toList[0]
		}

		ms = append(ms, m)
	}
	return ms, nil
}

// buildWhereClause builds a SQL WHERE clause from a query node.
func buildWhereClause(qn QNode, args []any) (string, []any) {
	switch qn.getType() {
	case QueryLeaf:
		ql := qn.(*QLeaf)
		switch ql.kind {
		case QuerySender:
			return buildQuerySender(ql, args)
		case QueryReceiver:
			return buildQueryReceiver(ql, args)
		case QuerySubject:
			return buildQuerySubject(ql, args)
		case QueryStatus:
			return buildQueryStatus(ql, args)
		case QueryMessageID:
			return buildQueryMessageID(ql, args)
		default:
			panic("bad QueryKind")
		}
	case QueryAnd:
		return buildQueryExpr(qn.(*QExpr), "AND", args)
	case QueryOr:
		return buildQueryExpr(qn.(*QExpr), "OR", args)
	default:
		panic("bad QueryType")
	}
}

// buildQueryExpr builds a SQL expression for a compound query with a conjunction.
func buildQueryExpr(qe *QExpr, conj string, args []any) (string, []any) {
	s1, args := buildWhereClause(qe.a, args)
	s2, args := buildWhereClause(qe.b, args)
	s := "(" + s1 + " " + conj + " " + s2 + ")"
	return s, args
}

// buildQuerySender builds a SQL condition for sender field queries.
func buildQuerySender(ql *QLeaf, args []any) (string, []any) {
	s := "sender "
	switch ql.op {
	case QueryEqual:
		s = s + "="
	case QueryNotEqual:
		s = s + "<>"
	}
	s = s + " $" + strconv.Itoa(len(args)+1)
	return s, append(args, ql.value)
}

// buildQueryReceiver builds a SQL condition for receiver field queries.
func buildQueryReceiver(ql *QLeaf, args []any) (string, []any) {
	s := "strpos(receivers, $" + strconv.Itoa(len(args)+1) + ")"
	switch ql.op {
	case QueryEqual:
		s = s + " > 0"
	case QueryNotEqual:
		s = s + " = 0"
	}
	return s, append(args, "\x01"+ql.value+"\x01")
}

// buildQuerySubject builds a SQL condition for subject field queries.
func buildQuerySubject(ql *QLeaf, args []any) (string, []any) {
	s := "strpos(subject, $" + strconv.Itoa(len(args)+1) + ")"
	switch ql.op {
	case QueryEqual:
		s = s + " > 0"
	case QueryNotEqual:
		s = s + " = 0"
	}
	return s, append(args, ql.value)
}

// buildQueryStatus builds a SQL condition for status field queries.
func buildQueryStatus(ql *QLeaf, args []any) (string, []any) {
	st := 0
	switch ql.value {
	case "waiting":
		st = MessageWaiting
	case "processing":
		st = MessageProcessing
	case "sent":
		st = MessageSent
	case "abandoned":
		st = MessageAbandoned
	}
	s := "status "
	switch ql.op {
	case QueryEqual:
		s = s + "="
	case QueryNotEqual:
		s = s + "<>"
	}
	s = s + " $" + strconv.Itoa(len(args)+1)
	return s, append(args, st)
}

// buildQueryMessageID builds a SQL condition for message ID queries.
func buildQueryMessageID(ql *QLeaf, args []any) (string, []any) {
	s := "uid "
	switch ql.op {
	case QueryEqual:
		s = s + "="
	case QueryNotEqual:
		s = s + "<>"
	}
	s = s + " $" + strconv.Itoa(len(args)+1)
	return s, append(args, ql.value)
}

// getStatusString converts a message status code to its string representation.
func getStatusString(status int) string {
	r := "unknown"
	switch status {
	case MessageWaiting:
		r = "waiting"
	case MessageProcessing:
		r = "processing"
	case MessageSent:
		r = "sent"
	case MessageAbandoned:
		r = "abandoned"
	}
	return r
}

// getRecipients returns all recipient email addresses (To, Cc, and Bcc).
func (m *Message) getRecipients() []string {
	var rs []string
	p := m.packet.Personalizations[0]
	for _, r := range p.To {
		rs = append(rs, r.Email)
	}
	for _, r := range p.Cc {
		rs = append(rs, r.Email)
	}
	for _, r := range p.Bcc {
		rs = append(rs, r.Email)
	}
	return rs
}

// getMultiContents writes multipart MIME content to the buffer.
func (m *Message) getMultiContents(p Personalization, buf *bytes.Buffer) error {
	if p.Headers == nil {
		p.Headers = map[string][]string{}
	}

	msg := bytes.NewBuffer(nil)
	mWriter := multipart.NewWriter(msg)

	if len(m.packet.Attachments) > 0 {
		p.Headers["Content-Type"] = []string{
			"multipart/mixed; boundary=" + mWriter.Boundary(),
		}
		for hname, hvals := range p.Headers {
			for _, hval := range hvals {
				buf.WriteString(hname)
				buf.WriteString(": ")
				buf.WriteString(hval)
				buf.WriteString("\r\n")
			}
		}
		buf.WriteString("\r\n")

		abuf := bytes.NewBuffer(nil)
		aWriter := multipart.NewWriter(abuf)

		mhdr := make(textproto.MIMEHeader)
		mhdr.Set("Content-Type", "multipart/alternative; boundary="+
			aWriter.Boundary())
		writer, err := mWriter.CreatePart(mhdr)
		if err != nil {
			return errors.WithStack(err)
		}

		for _, c := range m.packet.Content {
			if c.Type == "" {
				c.Type = "text/plain"
			}
			zhdr := make(textproto.MIMEHeader)
			zhdr.Set("Content-Type", c.Type)
			zwriter, err := aWriter.CreatePart(zhdr)
			if err != nil {
				return errors.WithStack(err)
			}
			_, err = zwriter.Write([]byte(c.Value))
			if err != nil {
				return errors.WithStack(err)
			}
		}
		err = aWriter.Close()
		if err != nil {
			return errors.WithStack(err)
		}
		_, err = writer.Write([]byte(abuf.Bytes()))
		if err != nil {
			return errors.WithStack(err)
		}

		for _, a := range m.packet.Attachments {
			if a.Type == "" {
				a.Type = "text/plain"
			}
			mhdr := make(textproto.MIMEHeader)
			mhdr.Set("Content-Type", a.Type)
			writer, err := mWriter.CreatePart(mhdr)
			if err != nil {
				return errors.WithStack(err)
			}
			_, err = writer.Write([]byte(a.Content))
			if err != nil {
				return errors.WithStack(err)
			}
		}

		err = mWriter.Close()
		if err != nil {
			return errors.WithStack(err)
		}
		_, err = buf.Write(msg.Bytes())
		if err != nil {
			return errors.WithStack(err)
		}
	} else {
		p.Headers["Content-Type"] = []string{
			"multipart/alternative; boundary=" + mWriter.Boundary(),
		}

		for hname, hvals := range p.Headers {
			for _, hval := range hvals {
				buf.WriteString(hname)
				buf.WriteString(": ")
				buf.WriteString(hval)
				buf.WriteString("\r\n")
			}
		}
		buf.WriteString("\r\n")

		for _, c := range m.packet.Content {
			if c.Type == "" {
				c.Type = "text/plain"
			}
			mhdr := make(textproto.MIMEHeader)
			mhdr.Set("Content-Type", c.Type)
			writer, err := mWriter.CreatePart(mhdr)
			if err != nil {
				return errors.WithStack(err)
			}
			_, err = writer.Write([]byte(c.Value))
			if err != nil {
				return errors.WithStack(err)
			}
		}
		err := mWriter.Close()
		if err != nil {
			return errors.WithStack(err)
		}
		buf.Write(msg.Bytes())
	}
	return nil
}

// getSingleContent writes single-part content to the buffer.
func (m *Message) getSingleContent(p Personalization, buf *bytes.Buffer) {
	c := m.packet.Content[0]
	if c.Type != "" && c.Type != "text/plain" {
		if p.Headers == nil {
			p.Headers = map[string][]string{}
		}
		p.Headers["Content-Type"] = []string{c.Type}
	}

	for hname, hvals := range p.Headers {
		for _, hval := range hvals {
			buf.WriteString(hname)
			buf.WriteString(": ")
			buf.WriteString(hval)
			buf.WriteString("\r\n")
		}
	}

	buf.WriteString("\r\n")

	buf.WriteString(c.Value)
}

// addresseesFieldValue formats a list of addressees for an email header field.
func (m *Message) addresseesFieldValue(addressees []Addressee) string {
	items := []string{}
	for _, addressee := range addressees {

		if addressee.Name == "" {
			items = append(items, addressee.Email)
		} else {
			name := mime.BEncoding.Encode("utf-8", addressee.Name)
			value := fmt.Sprintf("%s <%s>", name, addressee.Email)
			items = append(items, value)
		}

	}
	return strings.Join(items, ", ")
}

// getMessageBody constructs and returns the RFC 822 formatted message body.
func (m *Message) getMessageBody() ([]byte, error) {
	var buf bytes.Buffer
	p := m.packet.Personalizations[0]

	buf.WriteString("From: ")
	buf.WriteString(m.addresseesFieldValue([]Addressee{m.packet.From}))
	buf.WriteString("\r\n")

	if len(p.To) > 0 {
		buf.WriteString("To: ")
		buf.WriteString(m.addresseesFieldValue(p.To))
		buf.WriteString("\r\n")
	}

	if len(p.Cc) > 0 {
		buf.WriteString("Cc: ")
		buf.WriteString(m.addresseesFieldValue(p.Cc))
		buf.WriteString("\r\n")
	}

	if len(p.Bcc) > 0 {
		buf.WriteString("Bcc: ")
		buf.WriteString(m.addresseesFieldValue(p.Bcc))
		buf.WriteString("\r\n")
	}

	buf.WriteString("Subject: ")
	if p.Subject != "" {
		buf.WriteString(p.Subject)
	} else {
		buf.WriteString(m.packet.Subject)
	}
	buf.WriteString("\r\n")
	buf.WriteString(fmt.Sprintf("Message-Id: <%s>\r\n", m.uid))

	if len(m.packet.Content) > 1 {
		if err := m.getMultiContents(p, &buf); err != nil {
			return nil, err
		}
	} else if len(m.packet.Content) == 1 {
		m.getSingleContent(p, &buf)
	}

	return buf.Bytes(), nil
}
