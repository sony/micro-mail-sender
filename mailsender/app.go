package mailsender

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

const (
	defaultPort = 8333
)

type App struct {
	config *Config
	db     *sql.DB
	logger *zap.SugaredLogger
}

type Addressee struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Personalization struct {
	To      []Addressee         `json:"to"`
	Cc      []Addressee         `json:"cc"`
	Bcc     []Addressee         `json:"bcc"`
	Subject string              `json:"subject"`
	Headers map[string][]string `json:"headers"`
	SendAt  int                 `json:"send_at"`
}

type Content struct {
	Type  string `json:"type"`  // mime-type
	Value string `json:"value"` // actual content
}

type Attachment struct {
	Content     string `json:"content"` // base64 encoded content
	Type        string `json:"type"`    // mime type
	Filename    string `json:"filename"`
	Disposition string `json:"disposition"`
	ContentID   string `json:"content_id"`
}

type SendRequest struct {
	Personalizations []Personalization `json:"personalizations"`
	From             Addressee         `json:"from"`
	ReplyTo          Addressee         `json:"reply_to"`
	Subject          string            `json:"subject"`
	Content          []Content         `json:"content"`
	Attachments      []Attachment      `json:"attachments"`
	SendAt           int               `json:"send_at"`
	BatchID          string            `json:"batch_id"`
}

type SearchResultItem struct {
	Uid        string       `json:"msg-id"`
	Status     string       `json:"status"`
	LastUpdate string       `json:"last-update"`
	Request    *SendRequest `json:"request"`
}

type SearchResult struct {
	Messages []*SearchResultItem `json:"messages"`
}

// RunServer enters server loop.  Only returns when something bad happens.
func RunServer(config *Config) (err error) {
	app := newApp(config)
	defer func() {
		err = appendError(err, app.Fini())
	}()
	server := newServer(app)
	return server.ListenAndServe()
}

func newApp(config *Config) *App {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	db, err := newDB(config)
	if err != nil {
		log.Fatalf("can't connect to db: %v", err)
	}
	app := &App{config: config,
		db:     db,
		logger: logger.Sugar()}
	return app
}

// Fini closes the DB connection.
func (app *App) Fini() error {
	return app.db.Close()
}

func newServer(app *App) *http.Server {
	host := app.config.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := app.config.Port
	if port == 0 {
		port = defaultPort
	}

	runSenderLoop(app)
	runMonitorLoop(app)

	router := newRouter(app)

	app.logger.Infow("starting server",
		"host", host,
		"port", port)

	return &http.Server{
		Handler:      router,
		Addr:         fmt.Sprintf("%s:%d", host, port),
		WriteTimeout: 60 * time.Second,
		ReadTimeout:  60 * time.Second,
	}
}

func newRouter(app *App) *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/", app.hHello).Methods("GET")
	router.HandleFunc("/v3/mail/send", app.hMailSend).Methods("POST")
	router.HandleFunc("/v3/messages", app.hMessages).Methods("GET").
		Queries("query", "{query}")
	router.HandleFunc("/v3/smtplog", app.hSmtpLog).Methods("GET").
		Queries("count", "{count}")
	return router
}

func returnJSON(w http.ResponseWriter, val interface{}) {
	js, err := json.Marshal(val)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(js)
}

func returnErr(app *App, w http.ResponseWriter, apperr *AppError) {
	if apperr.Code == 500 {
		app.logger.Errorw("Returning Internal Error (500):",
			"message", apperr.Message,
			"error", apperr.Internal)
	} else if apperr.Code >= 400 {
		app.logger.Infow("Returning Client Error:",
			"code", apperr.Code,
			"message", apperr.Error())

	}
	body := map[string]string{"code": strconv.Itoa(apperr.Code),
		"error": apperr.Error()}
	bodybytes, _ := json.Marshal(body)
	http.Error(w, string(bodybytes), apperr.Code)
}

func (app *App) checkApikey(r *http.Request) *AppError {
	auth := r.Header["Authorization"]
	if len(auth) == 0 {
		return AppErr(403, "No apikey given")
	}

	key := regexp.MustCompile(`Bearer *(.*)`).ReplaceAllString(auth[0], "$1")
	for _, id := range app.config.AppIDs {
		if key == id {
			return nil
		}
	}
	return AppErr(403, "Unrecognized apikey")
}

// Request handlers
func (app *App) hHello(w http.ResponseWriter, r *http.Request) {
	returnJSON(w, map[string]string{"version": "1"})
}

func (app *App) hMailSend(w http.ResponseWriter, r *http.Request) {
	apperr := app.checkApikey(r)
	if apperr != nil {
		returnErr(app, w, apperr)
		return
	}

	var req SendRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		returnErr(app, w, WrapErr(400, err))
		return
	}

	app.logger.Infow("Got MailSend request",
		"SendRequest", req)

	apperr = enqueueMessage(app, req)
	if apperr != nil {
		returnErr(app, w, apperr)
		return
	}
	returnJSON(w, map[string]string{"result": "ok"})
}

func (app *App) hMessages(w http.ResponseWriter, r *http.Request) {
	apperr := app.checkApikey(r)
	if apperr != nil {
		returnErr(app, w, apperr)
		return
	}

	q := r.FormValue("query")
	lim := r.FormValue("limit")

	app.logger.Infow("Got Messages request",
		"query", q, "limit", lim)

	criteria, apperr := parseQuery(q)
	if apperr != nil {
		returnErr(app, w, apperr)
		return
	}

	limit := 10
	var err error
	if lim != "" {
		limit, err = strconv.Atoi(lim)
		if err != nil {
			returnErr(app, w, WrapErr(400, err))
			return
		}
	}

	app.logger.Debugw("Query parse", "QNode",
		fmt.Sprintf("%#v", criteria), "limit", limit)

	msgs, apperr := searchMessages(app, criteria, limit)
	if apperr != nil {
		returnErr(app, w, apperr)
		return
	}

	var srs []*SearchResultItem
	for _, m := range msgs {
		var req *SendRequest
		if len(m.packet.Personalizations) > 0 {
			req = &m.packet
		}
		srs = append(srs, &SearchResultItem{
			Uid:        m.uid,
			Status:     m.getStatusString(),
			LastUpdate: m.getLastUpdateString(),
			Request:    req,
		})
	}

	returnJSON(w, &SearchResult{Messages: srs})
}

func (app *App) hSmtpLog(w http.ResponseWriter, r *http.Request) {
	apperr := app.checkApikey(r)
	if apperr != nil {
		returnErr(app, w, apperr)
		return
	}

	s_count := r.FormValue("count")
	if s_count == "" {
		s_count = "262144"
	}
	count, err := strconv.ParseInt(s_count, 10, 64)
	if err != nil {
		returnErr(app, w, WrapErr(400, err))
		return
	}

	logpath := app.config.SmtpLog

	file, err := os.Open(logpath)
	if err != nil {
		returnErr(app, w, WrapErr(400, err))
		return
	}

	lines := make([]string, count)
	i := int64(0)
	wraparound := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines[i] = scanner.Text()
		if i == count-1 {
			wraparound = true
			i = 0
		} else {
			i = i + 1
		}
	}

	total := i
	result := lines[0:i]
	if wraparound {
		total = count
		result = append(lines[i:count], result...)
	}

	returnJSON(w, map[string]interface{}{
		"count": total,
		"lines": result,
	})
}
