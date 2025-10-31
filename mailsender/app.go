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
	defaultPort           = 8333
	jsonParseErrorMessage = `{"errors":[{"message":"fail to parse response body to json"}]}`
)

// App is application
type App struct {
	config *Config
	db     *sql.DB
	logger *zap.SugaredLogger
}

// ErrorResponse is content of error response
type ErrorResponse struct {
	Errors []Error `json:"errors"`
}

// Error is error item on response.
type Error struct {
	Message string  `json:"message"`
	Field   *string `json:"field,omitempty"`
}

// Addressee is address of a mail.
type Addressee struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Personalization hold Personalization information
type Personalization struct {
	To      []Addressee         `json:"to"`
	Cc      []Addressee         `json:"cc"`
	Bcc     []Addressee         `json:"bcc"`
	Subject string              `json:"subject"`
	Headers map[string][]string `json:"headers"`
}

// Content hold content information
type Content struct {
	Type  string `json:"type"`  // mime-type
	Value string `json:"value"` // actual content
}

// Attachment hold attachment information
type Attachment struct {
	Content     string `json:"content"` // base64 encoded content
	Type        string `json:"type"`    // mime type
	Filename    string `json:"filename"`
	Disposition string `json:"disposition"`
	ContentID   string `json:"content_id"`
}

// SendRequest is request body of mail send
type SendRequest struct {
	Personalizations []Personalization `json:"personalizations"`
	From             Addressee         `json:"from"`
	ReplyTo          Addressee         `json:"reply_to"`
	Subject          string            `json:"subject"`
	Content          []Content         `json:"content"`
	Attachments      []Attachment      `json:"attachments"`
}

// SearchResultItem is item of search result item
type SearchResultItem struct {
	FromEmail     string `json:"from_email"`
	MsgID         string `json:"msg_id"`
	Subject       string `json:"subject"`
	ToEmail       string `json:"to_email"`
	Status        string `json:"status"`
	LastTimestamp int    `json:"last_timestamp"`
}

// SearchResult is item of search result
type SearchResult struct {
	Messages []SearchResultItem `json:"messages"`
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
	router.HandleFunc("/v3/smtplog", app.hSMTPLog).Methods("GET").
		Queries("count", "{count}")
	return router
}

func returnJSON(w http.ResponseWriter, val any) {
	js, err := json.Marshal(val)
	if err != nil {
		http.Error(w, jsonParseErrorMessage, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(js)
}

func returnErrWithField(app *App, w http.ResponseWriter, apperr *AppError) {
	if apperr.Code == 500 {
		app.logger.Errorw("Returning Internal Error (500):",
			"message", apperr.Message,
			"error", apperr.Internal)

		res := ErrorResponse{
			Errors: []Error{{
				Message: apperr.Error(),
			}},
		}
		bodybytes, err := json.Marshal(res)
		if err != nil {
			http.Error(w, jsonParseErrorMessage, http.StatusInternalServerError)
			return
		}
		http.Error(w, string(bodybytes), http.StatusInternalServerError)
	}

	res := ErrorResponse{
		Errors: []Error{{
			Message: apperr.Error(),
		}},
	}
	bodybytes, err := json.Marshal(res)
	if err != nil {
		http.Error(w, jsonParseErrorMessage, http.StatusInternalServerError)
		return
	}

	http.Error(w, string(bodybytes), apperr.Code)
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

	res := ErrorResponse{
		Errors: []Error{{
			Message: apperr.Error(),
		}},
	}
	bodybytes, err := json.Marshal(res)
	if err != nil {
		http.Error(w, jsonParseErrorMessage, http.StatusInternalServerError)
		return
	}
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
		returnErrWithField(app, w, apperr)
		return
	}

	var req SendRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		returnErrWithField(app, w, WrapErr(400, err))
		return
	}

	app.logger.Infow("Got MailSend request",
		"SendRequest", req)

	apperr = enqueueMessage(app, req)
	if apperr != nil {
		returnErrWithField(app, w, apperr)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte{})
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

	returnJSON(w, &SearchResult{Messages: msgs})
}

func (app *App) hSMTPLog(w http.ResponseWriter, r *http.Request) {
	apperr := app.checkApikey(r)
	if apperr != nil {
		returnErr(app, w, apperr)
		return
	}

	sCount := r.FormValue("count")
	if sCount == "" {
		sCount = "262144"
	}
	count, err := strconv.ParseInt(sCount, 10, 64)
	if err != nil {
		returnErr(app, w, WrapErr(400, err))
		return
	}

	logpath := app.config.SMTPLog

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

	returnJSON(w, map[string]any{
		"count": total,
		"lines": result,
	})
}
