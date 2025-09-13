package mailsender

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

const (
	MessageWaiting    = 0
	MessageProcessing = 1
	MessageSent       = 2
	MessageAbandoned  = 3
)

type Message struct {
	uid        string
	packet     SendRequest
	status     int
	lastUpdate int64
}

func newDB(config *Config) (*sql.DB, error) {
	connStr := "host=" + config.DbHost +
		" user=" + config.DbUser +
		" password=" + config.DbPassword +
		" dbname=" + config.DbName +
		" sslmode=" + config.DbSSLMode
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, appendError(err, db.Close())
	}
	return db, err
}

func expandPersonalization(req SendRequest) []SendRequest {
	var rs []SendRequest
	for _, p := range req.Personalizations {
		r := req
		r.Personalizations = []Personalization{p}
		rs = append(rs, r)
	}
	return rs
}

// personalization is already expanded in req
// returns concatenatino of emails, surrounded by "\x01" for the later
// substring match.
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

// Insert single SendRequest into the queue.  Must be called within
// transition.
func enqueueMessage1(app *App, req SendRequest, tx *sql.Tx) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	packet := string(b)
	uid := fmt.Sprintf("%s@%s", uuid.New().String(), app.config.MyDomain)
	lastUpdate := time.Now().Unix()

	bid := uuid.New().String()
	_, err = tx.Exec(`insert into bodies (bid, packet) `+
		`values ($1, $2)`,
		bid, packet)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`insert into messages (`+
		` uid, bid, sender, receivers, subject, status, last_update) `+
		`values ($1, $2, $3, $4, $5, $6, $7)`,
		uid, bid, req.From.Email,
		receiverEmails(&req), req.Subject,
		MessageWaiting, lastUpdate)
	if err != nil {
		return err
	}
	return nil
}

func enqueueMessage(app *App, req SendRequest) *AppError {
	tx, err := app.db.Begin()
	if err != nil {
		return WrapErr(500, err)
	}
	for _, r := range expandPersonalization(req) {
		err = enqueueMessage1(app, r, tx)
		if err != nil {
			err2 := tx.Rollback()
			if err2 != nil {
				return WrapErr(500, err2)
			}
			return WrapErr(500, err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return WrapErr(500, err)
	}
	return nil
}

// common routine to extract message.  may return nil message.
func extractMessage(app *App, row *sql.Row) (*Message, error) {
	var m Message
	var spacket interface{}
	err := row.Scan(&m.uid, &spacket, &m.status, &m.lastUpdate)
	if err != nil {
		if regexp.MustCompile("no rows in result set").
			MatchString(err.Error()) {
			return nil, nil
		}
		return nil, err
	}
	if spacket == nil {
		m.packet = SendRequest{}
	} else {
		err = json.Unmarshal([]byte(spacket.(string)), &m.packet)
		if err != nil {
			return nil, err
		}
	}
	return &m, nil
}

func getMessage(app *App, uid string) (*Message, error) {
	row := app.db.QueryRow(`select uid, packet, status, last_update `+
		`from messages `+
		`left join bodies on messages.bid = bodies.bid `+
		`where uid = $1`,
		uid)
	return extractMessage(app, row)
}

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
		if regexp.MustCompile("no rows in result set").
			MatchString(err.Error()) {
			return nil, nil
		}
		return nil, err
	}
	row = app.db.QueryRow(`select packet from bodies `+
		`where bid = $1`,
		bid)
	var spacket interface{}
	err = row.Scan(&spacket)
	if err != nil {
		if regexp.MustCompile("no rows in result set").
			MatchString(err.Error()) {
			spacket = ""
		}
		return nil, err
	}
	if spacket == nil {
		m.packet = SendRequest{}
	} else {
		err = json.Unmarshal([]byte(spacket.(string)), &m.packet)
		if err != nil {
			return nil, err
		}
	}
	return &m, nil
}

func (m *Message) abandonMessage(app *App, errmsg string) error {
	_, err := app.db.Exec(`update messages set `+
		`status = $1, `+
		`last_error = $2, `+
		`last_update = $3 `+
		`where uid = $4`,
		MessageAbandoned, errmsg, time.Now().Unix(), m.uid)
	if err != nil {
		return err
	}
	return m.cleanMessageBody(app)
}

func (m *Message) sentMessage(app *App) error {
	_, err := app.db.Exec(`update messages set `+
		`status = $1, `+
		`last_error = $2, `+
		`last_update = $3 `+
		`where uid = $4`,
		MessageSent, "", time.Now().Unix(), m.uid)
	if err != nil {
		return err
	}
	return m.cleanMessageBody(app)
}

func (m *Message) cleanMessageBody(app *App) (rerr error) {
	// Remove it only iff no other metadata refers to the body
	tx, err := app.db.Begin()
	if err != nil {
		return err
	}

	rows, err := tx.Query(`select uid, bid from messages `+
		`where messages.bid = (select bid from messages `+
		`                      where uid = $1)`,
		m.uid)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			return err2
		}
		return err
	}
	defer func() {
		rerr = appendError(rerr, rows.Close())
	}()

	var uid string
	var bid string
	count := 0
	for {
		if !rows.Next() {
			break
		}
		err = rows.Scan(&uid, &bid)
		if err != nil {
			err2 := tx.Rollback()
			if err2 != nil {
				return err2
			}
			return err
		}
		count++
	}
	if count == 1 {
		_, err := tx.Exec(`delete from bodies where bid = $1`, bid)
		if err != nil {
			err2 := tx.Rollback()
			if err2 != nil {
				return err2
			}
			return err
		}
	}
	err = tx.Commit()
	return err
}

// Searching
func searchMessages(app *App, criteria QNode, limit int) (msgs []Message, apperr *AppError) {
	q := `select uid, packet, status, last_update ` +
		`from messages ` +
		`left join bodies on messages.bid = bodies.bid ` +
		`where `
	var params []interface{}
	conds, params := buildWhereClause(criteria, params)
	q = q + conds + " order by last_update desc "
	if limit > 0 {
		q = q + " limit " + strconv.Itoa(limit)
	}

	rows, err := app.db.Query(q, params...)
	if err != nil {
		return []Message{}, WrapErr(500, err)
	}

	defer func() {
		e := rows.Close()
		if e != nil {
			if apperr != nil {
				apperr.Internal = appendError(apperr.Internal, e)
				return
			}
			apperr = WrapErr(500, e)
		}
	}()

	var ms []Message

	for {
		if !rows.Next() {
			break
		}

		var m Message
		var spacket interface{}
		err = rows.Scan(&m.uid, &spacket, &m.status, &m.lastUpdate)
		if err != nil {
			return []Message{}, WrapErr(500, err)
		}
		if spacket == nil {
			m.packet = SendRequest{}
		} else {
			err = json.Unmarshal([]byte(spacket.(string)), &m.packet)
			if err != nil {
				return []Message{}, WrapErr(500, err)
			}
		}
		ms = append(ms, m)
	}
	return ms, nil
}

func buildWhereClause(qn QNode, args []interface{}) (string, []interface{}) {
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
		case QueryMessageId:
			return buildQueryMessageId(ql, args)
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

func buildQueryExpr(qe *QExpr, conj string, args []interface{}) (string, []interface{}) {
	s1, args := buildWhereClause(qe.a, args)
	s2, args := buildWhereClause(qe.b, args)
	s := "(" + s1 + " " + conj + " " + s2 + ")"
	return s, args
}

func buildQuerySender(ql *QLeaf, args []interface{}) (string, []interface{}) {
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

func buildQueryReceiver(ql *QLeaf, args []interface{}) (string, []interface{}) {
	s := "strpos(receivers, $" + strconv.Itoa(len(args)+1) + ")"
	switch ql.op {
	case QueryEqual:
		s = s + " > 0"
	case QueryNotEqual:
		s = s + " = 0"
	}
	return s, append(args, "\x01"+ql.value+"\x01")
}

func buildQuerySubject(ql *QLeaf, args []interface{}) (string, []interface{}) {
	s := "strpos(subject, $" + strconv.Itoa(len(args)+1) + ")"
	switch ql.op {
	case QueryEqual:
		s = s + " > 0"
	case QueryNotEqual:
		s = s + " = 0"
	}
	return s, append(args, ql.value)
}

func buildQueryStatus(ql *QLeaf, args []interface{}) (string, []interface{}) {
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

func buildQueryMessageId(ql *QLeaf, args []interface{}) (string, []interface{}) {
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

// convenience accessor
func (m *Message) getStatusString() string {
	r := "unknown"
	switch m.status {
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

func (m *Message) getLastUpdateString() string {
	return time.Unix(m.lastUpdate, 0).Format(time.RFC3339)
}

//
// Actual sending
//

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
			return err
		}

		for _, c := range m.packet.Content {
			if c.Type == "" {
				c.Type = "text/plain"
			}
			zhdr := make(textproto.MIMEHeader)
			zhdr.Set("Content-Type", c.Type)
			zwriter, err := aWriter.CreatePart(zhdr)
			if err != nil {
				return err
			}
			_, err = zwriter.Write([]byte(c.Value))
			if err != nil {
				return err
			}
		}
		err = aWriter.Close()
		if err != nil {
			return err
		}
		_, err = writer.Write([]byte(abuf.Bytes()))
		if err != nil {
			return err
		}

		for _, a := range m.packet.Attachments {
			if a.Type == "" {
				a.Type = "text/plain"
			}
			mhdr := make(textproto.MIMEHeader)
			mhdr.Set("Content-Type", a.Type)
			writer, err := mWriter.CreatePart(mhdr)
			if err != nil {
				return err
			}
			_, err = writer.Write([]byte(a.Content))
			if err != nil {
				return err
			}
		}

		err = mWriter.Close()
		if err != nil {
			return err
		}
		_, err = buf.Write(msg.Bytes())
		if err != nil {
			return err
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
				return err
			}
			_, err = writer.Write([]byte(c.Value))
			if err != nil {
				return err
			}
		}
		err := mWriter.Close()
		if err != nil {
			return err
		}
		buf.Write(msg.Bytes())
	}
	return nil
}

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

func (m *Message) getMessageBody() ([]byte, error) {
	var buf bytes.Buffer
	p := m.packet.Personalizations[0]

	buf.WriteString("From: ")
	buf.WriteString(m.packet.From.Email)
	buf.WriteString("\r\n")

	for _, r := range p.To {
		buf.WriteString("To: ")
		buf.WriteString(r.Email)
		buf.WriteString("\r\n")
	}
	for _, r := range p.Cc {
		buf.WriteString("Cc: ")
		buf.WriteString(r.Email)
		buf.WriteString("\r\n")
	}
	for _, r := range p.Bcc {
		buf.WriteString("Bcc: ")
		buf.WriteString(r.Email)
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
