// API service to handle transactions from our frontend.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/skip-mev/skipper/bot"
	uberatomic "go.uber.org/atomic"
)

var (
	pathTransaction = "/transaction"
)

var (
	ErrServerAlreadyStarted = errors.New("server was already started")
)

type RelayAPIConfig struct {
	Log        *logrus.Entry
	ListenAddr string
}

type RelayAPI struct {
	config      RelayAPIConfig
	srv         *http.Server
	srvStarted  uberatomic.Bool
	srvShutdown uberatomic.Bool
	bot         *bot.Bot
	log         *logrus.Entry
}

func NewRelayAPI(config RelayAPIConfig, bot *bot.Bot) *RelayAPI {
	return &RelayAPI{
		config: config,
		bot:    bot,
	}
}

func (api *RelayAPI) getRouter() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc(pathTransaction, api.handleTransaction).Methods(http.MethodPost)
	return r
}

func (api *RelayAPI) StartServer() error {
	if api.srvStarted.Swap(true) {
		return ErrServerAlreadyStarted
	}

	api.srv = &http.Server{
		Addr:    api.config.ListenAddr,
		Handler: api.getRouter(),
	}
	err := api.srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (api *RelayAPI) StopServer() error {
	if wasStopping := api.srvShutdown.Swap(true); wasStopping {
		return nil
	}
	return api.srv.Shutdown(context.Background())
}

func (api *RelayAPI) handleTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		api.RespondError(w, http.StatusBadRequest, "invalid content type")
		return
	}

	// TODO: handle transaction.
	w.WriteHeader(http.StatusOK)
}

func (api *RelayAPI) RespondError(w http.ResponseWriter, code int, message string) {
	api.Respond(w, code, HTTPErrorResp{code, message})
}

func (api *RelayAPI) RespondOK(w http.ResponseWriter, response any) {
	api.Respond(w, http.StatusOK, response)
}

func (api *RelayAPI) RespondMsg(w http.ResponseWriter, code int, msg string) {
	api.Respond(w, code, HTTPMessageResp{msg})
}

func (api *RelayAPI) Respond(w http.ResponseWriter, code int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if response == nil {
		return
	}

	// write the json response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.log.WithField("response", response).WithError(err).Error("Couldn't write response")
		http.Error(w, "", http.StatusInternalServerError)
	}
}
