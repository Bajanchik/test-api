package apiserver

import (
	"database/sql"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"test-api/internal/app/store"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type APIServer struct {
	config *Config
	logger *logrus.Logger
	router *mux.Router
	store  *store.Store
}

func New(config *Config) *APIServer {
	return &APIServer{
		config: config,
		logger: logrus.New(),
		router: mux.NewRouter(),
	}
}

func (s *APIServer) Start() error {
	if err := s.configureLogger(); err != nil {
		return err
	}

	s.configureRouter()

	if err := s.configureStore(); err != nil {
		return err
	}

	s.logger.Info("starting api server")

	return http.ListenAndServe(s.config.BindAddr, s.router)
}

func (s *APIServer) configureLogger() error {
	level, err := logrus.ParseLevel(s.config.LogLevel)
	if err != nil {
		return err
	}

	s.logger.SetLevel(level)

	return nil
}

func (s *APIServer) configureRouter() {
	s.router.HandleFunc("/hello", s.handleHello())
	s.router.HandleFunc("/shorten", s.createShortLink())
	s.router.HandleFunc("/{shortUrl}", s.getLongLink())
}

func (s *APIServer) configureStore() error {
	st := store.New(s.config.Store)
	if err := st.Open(); err != nil {
		return err
	}

	s.store = st

	return nil

}

func (s *APIServer) handleHello() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Hello")
	}
}

type ShortLink struct {
	ShortURL string `json:"short_url"`
	LongURL  string `json:"long_url"`
}

var shortLinks = make(map[string]string)
var db *sql.DB

func randomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
	chars := make([]byte, length)
	for i := 0; i < length; i++ {
		chars[i] = charset[rand.Intn(len(charset))]
	}
	return string(chars)
}

func (s *APIServer) createShortLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		longURL := r.FormValue("long_url")
		if longURL == "" {
			http.Error(w, "Missing long_url parameter", http.StatusBadRequest)
			return
		}

		shortURL := randomString(10)

		shortLinks[shortURL] = longURL

		_, err := db.Exec("INSERT INTO short_links (short_url, long_url) VALUES ($1, $2)", shortURL, longURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := ShortLink{
			ShortURL: shortURL,
			LongURL:  longURL,
		}

		jsonResponse, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(jsonResponse)
	}
}

func (s *APIServer) getLongLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shortURL := strings.TrimPrefix(r.URL.Path, "/")

		longURL, ok := shortLinks[shortURL]
		if ok {
			//http.Redirect(w, r, longURL, http.StatusMovedPermanently)
			return
		}

		row := db.QueryRow("SELECT long_url FROM short_links WHERE short_url = $1", shortURL)
		err := row.Scan(&longURL)
		if err != nil {
			http.Error(w, "Short URL not found", http.StatusNotFound)
			return
		}

		http.Redirect(w, r, longURL, http.StatusMovedPermanently)
	}
}
