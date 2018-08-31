package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"
	"github.com/ubiq/spectrum-backend/models"
	"github.com/ubiq/spectrum-backend/storage"
)

type Config struct {
	Enabled bool   `json:"enabled"`
	Port    string `json:"port"`
}

type ApiServer struct {
	backend *storage.MongoDB
	port    string
}

type AccountTxn struct {
	Txns  []models.Transaction `bson:"txns" json:"txns"`
	Total int                  `bson:"total" json:"total"`
}

type AccountTokenTransfer struct {
	Txns  []models.TokenTransfer `bson:"txns" json:"txns"`
	Total int                    `bson:"total" json:"total"`
}

type BlockRes struct {
	Blocks []models.Block `bson:"blocks" json:"blocks"`
	Total  int            `bson:"total" json:"total"`
}

type UncleRes struct {
	Uncles []models.Uncle `bson:"uncles" json:"uncles"`
	Total  int            `bson:"total" json:"total"`
}

func New(backend *storage.MongoDB, cfg *Config) *ApiServer {
	return &ApiServer{backend, cfg.Port}
}

func (a *ApiServer) Start() {
	log.Warnf("Starting api on port: %v", a.port)

	r := mux.NewRouter()
	r.HandleFunc("/status", a.getStore).Methods("GET")
	r.HandleFunc("/block/{number}", a.getBlockByNumber).Methods("GET")
	r.HandleFunc("/blockbyhash/{hash}", a.getBlockByHash).Methods("GET")
	r.HandleFunc("/latest", a.getLatestBlock).Methods("GET")
	r.HandleFunc("/latestblocks/{limit}", a.getLatestBlocks).Methods("GET")
	r.HandleFunc("/latestforkedblocks/{limit}", a.getLatestForkedBlocks).Methods("GET")
	r.HandleFunc("/latesttransactions/{limit}", a.getLatestTransactions).Methods("GET")
	r.HandleFunc("/latestaccounttxns/{hash}", a.getLatestTransactionsByAccount).Methods("GET")
	r.HandleFunc("/latestaccounttokentxns/{hash}", a.getLatestTokenTransfersByAccount).Methods("GET")
	r.HandleFunc("/latesttokentransfers/{limit}", a.getLatestTokenTransfers).Methods("GET")
	r.HandleFunc("/latestuncles/{limit}", a.getLatestUncles).Methods("GET")
	r.HandleFunc("/transaction/{hash}", a.getTransactionByHash).Methods("GET")
	r.HandleFunc("/uncle/{hash}", a.getUncleByHash).Methods("GET")

	handler := cors.Default().Handler(r)
	if err := http.ListenAndServe("0.0.0.0:"+a.port, handler); err != nil {
		log.Fatal(err)
	}
}

func (a *ApiServer) getBlockByHash(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	block, err := a.backend.BlockByHash(params["hash"])
	if err != nil {
		a.sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.sendJson(w, http.StatusOK, block)
}

func (a *ApiServer) getBlockByNumber(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	number, uerr := strconv.ParseUint(params["number"], 10, 64)
	if uerr != nil {
		a.sendError(w, http.StatusBadRequest, uerr.Error())
		return
	}
	block, err := a.backend.BlockByNumber(number)
	if err != nil {
		a.sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.sendJson(w, http.StatusOK, block)
}

func (a *ApiServer) getLatestBlock(w http.ResponseWriter, r *http.Request) {
	blocks, err := a.backend.LatestBlock()
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.sendJson(w, http.StatusOK, blocks)
}

func (a *ApiServer) getLatestBlocks(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	limit, err := strconv.Atoi(params["limit"])
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if limit > 1000 {
		limit = 1000
	}
	blocks, err := a.backend.LatestBlocks(limit)

	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	count, err := a.backend.TotalBlockCount()
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var res BlockRes
	res.Blocks = blocks
	res.Total = count

	a.sendJson(w, http.StatusOK, res)
}

func (a *ApiServer) getLatestForkedBlocks(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	limit, err := strconv.Atoi(params["limit"])
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if limit > 1000 {
		limit = 1000
	}
	blocks, err := a.backend.LatestForkedBlocks(limit)
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.sendJson(w, http.StatusOK, blocks)
}

func (a *ApiServer) getLatestTransactions(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	limit, err := strconv.Atoi(params["limit"])
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if limit > 1000 {
		limit = 1000
	}
	txns, err := a.backend.LatestTransactions(limit)
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	count, err := a.backend.TotalTxnCount()
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var res AccountTxn
	res.Txns = txns
	res.Total = count

	a.sendJson(w, http.StatusOK, res)
}

func (a *ApiServer) getLatestTransactionsByAccount(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	txns, err := a.backend.LatestTransactionsByAccount(params["hash"])
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	count, err := a.backend.TxnCount(params["hash"])
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var res AccountTxn
	res.Txns = txns
	res.Total = count

	a.sendJson(w, http.StatusOK, res)
}

func (a *ApiServer) getLatestTokenTransfersByAccount(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	txns, err := a.backend.LatestTokenTransfersByAccount(params["hash"])
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	count, err := a.backend.TokenTransferCount(params["hash"])
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var res AccountTokenTransfer
	res.Txns = txns
	res.Total = count

	a.sendJson(w, http.StatusOK, res)
}

func (a *ApiServer) getLatestTokenTransfers(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	limit, err := strconv.Atoi(params["limit"])
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if limit > 1000 {
		limit = 1000
	}
	transfers, err := a.backend.LatestTokenTransfers(limit)
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	count, err := a.backend.TokenTransferCount(params["hash"])
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var res AccountTokenTransfer
	res.Txns = transfers
	res.Total = count

	a.sendJson(w, http.StatusOK, res)
}

func (a *ApiServer) getLatestUncles(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	limit, err := strconv.Atoi(params["limit"])
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if limit > 1000 {
		limit = 1000
	}
	uncles, err := a.backend.LatestUncles(limit)
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	count, err := a.backend.TotalUncleCount()
	if err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var res UncleRes
	res.Uncles = uncles
	res.Total = count

	a.sendJson(w, http.StatusOK, res)
}

func (a *ApiServer) getTransactionByHash(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	txn, err := a.backend.TransactionByHash(params["hash"])
	if err != nil {
		a.sendError(w, http.StatusOK, err.Error())
		return
	}
	a.sendJson(w, http.StatusOK, txn)
}

func (a *ApiServer) getUncleByHash(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	uncle, err := a.backend.UncleByHash(params["hash"])
	if err != nil {
		a.sendError(w, http.StatusOK, err.Error())
		return
	}
	a.sendJson(w, http.StatusOK, uncle)
}

func (a *ApiServer) getStore(w http.ResponseWriter, r *http.Request) {
	store, err := a.backend.Store()
	if err != nil {
		a.sendError(w, http.StatusOK, err.Error())
		return
	}
	a.sendJson(w, http.StatusOK, store)
}

func (a *ApiServer) sendError(w http.ResponseWriter, code int, msg string) {
	a.sendJson(w, code, map[string]string{"error": msg})
}

func (a *ApiServer) sendJson(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
