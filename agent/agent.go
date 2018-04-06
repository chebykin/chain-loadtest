package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	web3 "github.com/regcostajr/go-web3"
	"github.com/regcostajr/go-web3/dto"
	"github.com/regcostajr/go-web3/providers"
)

var config *Configuration
var chainMap *ChainMap
var statsdClient statsd.Statter

// Agent starts http server to receive instructions from the master.
// It sends statistics to statsd daemon.
func main() {
	// Reading config file
	file, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Panicln("Unable to locate config.json file:", err)
	}

	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Panicln("Failed to decode config file:", err)
	}

	// Logs
	logFile, err := os.OpenFile(config.Logs, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		log.Panicln(err)
	}

	log.SetOutput(io.MultiWriter(os.Stderr, logFile))

	defer func() {
		e := logFile.Close()
		if e != nil {
			fmt.Fprintf(os.Stderr, "Problem closing the log file: %s\n", e)
		}
	}()

	// Reading chain map
	file, err = ioutil.ReadFile("./map.json")
	if err != nil {
		log.Panicln("Unable to read chain map file:", err)
	}

	err = json.Unmarshal(file, &chainMap)
	if err != nil {
		log.Panicln("Unable to parse chain map file:", err)
	}

	connection := web3.NewWeb3(
		providers.NewHTTPProvider(config.Endpoints.RPC, 10, false))

	coinbase, err := connection.Eth.GetCoinbase()
	if err != nil {
		log.Panicln("Failed to get coinbase", err)
	}
	log.Println("coinbase", coinbase)

	statsdClient, err = statsd.NewClient("127.0.0.1:8125", config.Me.Name)
	if err != nil {
		log.Panicln("Failed to connect statsd server:", err)
	}

	defer statsdClient.Close()

	server()
}

func server() {
	r := mux.NewRouter()
	r.Path("/ethSendRaw").HandlerFunc(ethSendRaw)
	r.Path("/personalSignAndSend").HandlerFunc(personalSignAndSendHandler)
	r.Path("/sendInternalSolidity").HandlerFunc(sendInternalSolidityHandler)
	addr := "0.0.0.0:8080"

	srv := &http.Server{
		Handler:      r,
		Addr:         addr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  20 * time.Second,
	}

	log.Println("Listening at", addr)
	log.Fatal(srv.ListenAndServe())
}

// Handlers
func personalSignAndSendHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("personalSignAndSend request")
	personalHandler(personalSignAndSendWorker, w, r)
}

func sendInternalSolidityHandler(w http.ResponseWriter, r *http.Request) {
	// TODO
}

func personalHandler(worker Worker, w http.ResponseWriter, r *http.Request) {
	secret := r.URL.Query().Get("secret")
	if secret != config.Secret {
		log.Println("Wrong secret")
		err := errors.New("wrong secret")
		respondWithError(w, http.StatusUnauthorized, err)
		return
	}

	log.Println(r.URL.Query().Get("count"))
	count, err := strconv.Atoi(r.URL.Query().Get("count"))

	if err != nil {
		err := errors.New("wrong number of txs")
		log.Println("Wrong number of txs", err)
		respondWithError(w, http.StatusBadRequest, err)
		return
	}

	if count > config.TxLimit {
		err := errors.New(fmt.Sprintln(
			"request tx count exceeded limit of", config.TxLimit))
		log.Println(err)
		respondWithError(w, http.StatusBadRequest, err)
		return
	}

	conns := make(chan *web3.Web3)

	go func() {
		rpcType := r.URL.Query().Get("rpcType")

		for {
			switch rpcType {
			case "ws":
				conns <- web3.NewWeb3(providers.NewWebSocketProvider(config.Endpoints.WS))
				log.Println("New WS connection established")
			case "ipc":
				conns <- web3.NewWeb3(providers.NewIPCProvider(config.Endpoints.IPC))
				log.Println("New IPC connection established")
			default:
				conns <- web3.NewWeb3(providers.NewHTTPProvider(config.Endpoints.RPC, 10, false))
				log.Println("New HTTP connection established")
			}
		}
	}()

	if secret != config.Secret {
		log.Println("Wrong secret")
		err := errors.New("wrong secret")
		respondWithError(w, http.StatusUnauthorized, err)
		return
	}

	j := 0
	jLimit := len(chainMap.Peers)
	if jLimit < j {
		j = jLimit - 1
	}

	log.Println("Peers count", len(chainMap.Peers))
	val := big.NewInt(0).Mul(big.NewInt(34), big.NewInt(1E16))

	jobsCh := make(chan sendOpts)
	resultsCh := make(chan sendResult)

	defer close(jobsCh)
	defer close(resultsCh)

	results := make([]sendResult, count)

	var wg sync.WaitGroup
	wg.Add(count)

	go func() {
		for i := 0; i < count; i++ {
			select {
			case result := <-resultsCh:
				log.Println("<<<", result)
				statsdClient.Inc("txsend", 1, 1.0)

				if result.Error {
					statsdClient.Inc("txerr", 1, 1.0)
				}
				// TODO: push info to statsd
				results[i] = result
			case <-r.Context().Done():
				log.Printf("Request cancelled. Skipping result for #%d\n", i)
				// nothing
			}
			wg.Done()
		}
	}()

	log.Println("Launching", config.WorkersCount, "workers...")
	for w := 0; w < config.WorkersCount; w++ {
		go worker(conns, jobsCh, resultsCh)
	}

	log.Println("Scheduling jobs...")
	for i := 0; i < count; i++ {
		jobsCh <- sendOpts{
			Val:       val,
			Addressee: chainMap.Peers[j],
		}

		log.Printf("Job #%d scheduled\n", i)

		j++
		if j == jLimit {
			j = 0
		}

		select {
		case <-r.Context().Done():
			break
		default:
			// do nothing
		}
	}

	fmt.Println("Waiting for resultsCh...")

	wg.Wait()

	resultBytes, err := json.Marshal(results)
	if err != nil {
		e := errors.New(fmt.Sprintln("error occured while executing the order: ", err.Error()))
		log.Println(e)
		respondWithError(w, http.StatusInternalServerError, e)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, string(resultBytes))
}

func ethSendRaw(w http.ResponseWriter, r *http.Request) {
	secret := r.URL.Query().Get("secret")
	if secret != config.Secret {
		log.Println("Wrong secret")
		err := errors.New("wrong secret")
		respondWithError(w, http.StatusUnauthorized, err)
		return
	}

	log.Println(r.URL.Query().Get("count"))
	count, err := strconv.Atoi(r.URL.Query().Get("count"))

	if err != nil {
		err := errors.New("wrong number of txs")
		log.Println("Wrong number of txs", err)
		respondWithError(w, http.StatusBadRequest, err)
		return
	}

	if count > config.TxLimit {
		err := errors.New(fmt.Sprintln(
			"request tx count exceeded limit of", config.TxLimit))
		log.Println(err)
		respondWithError(w, http.StatusBadRequest, err)
		return
	}

	if secret != config.Secret {
		log.Println("Wrong secret")
		err := errors.New("wrong secret")
		respondWithError(w, http.StatusUnauthorized, err)
		return
	}

	j := 0
	jLimit := len(chainMap.Peers)
	if jLimit < j {
		j = jLimit - 1
	}

	log.Println("Peers count", len(chainMap.Peers))
	//val := big.NewInt(0).Mul(big.NewInt(34), big.NewInt(1E16))

	jobsCh := make(chan *types.Transaction)
	resultsCh := make(chan sendResult)

	defer close(jobsCh)
	defer close(resultsCh)

	results := make([]sendResult, count)

	var wg sync.WaitGroup
	wg.Add(count)

	file, err := ioutil.ReadFile(config.PeerKey)
	if err != nil {
		log.Panicln("Unable to read validator key file:", err)
	}
	key, _ := keystore.DecryptKey([]byte(file), "vpeers/validator-0")

	client, err := getClient("http://127.0.0.1:8545")
	if err != nil {
		log.Panic(err)
	}

	nonce, err := client.PendingNonceAt(context.Background(), key.Address)
	if err != nil {
		log.Panic(err)
	}

	txs := make(types.Transactions, count)
	signer := types.NewEIP155Signer(big.NewInt(int64(15054)))

	for i := 0; i < count; i++ {
		//fmt.Println(i)
		tx := types.NewTransaction(nonce, common.StringToAddress(chainMap.Peers[0]),
			big.NewInt(1E16),
			uint64(21000), big.NewInt(1E9), []byte(""))

		signed_tx, _ := types.SignTx(tx, signer, key.PrivateKey)
		txs[i] = signed_tx
		nonce++
	}

	go func() {
		for i := 0; i < count; i++ {
			select {
			case result := <-resultsCh:
				log.Println("<<<", result)
				statsdClient.Inc("txsend", 1, 1.0)

				if result.Error {
					statsdClient.Inc("txerr", 1, 1.0)
				}
				// TODO: push info to statsd
				results[i] = result
			case <-r.Context().Done():
				log.Printf("Request cancelled. Skipping result for #%d\n", i)
				// nothing
			}
			wg.Done()
		}
	}()

	log.Println("Launching", config.WorkersCount, "workers...")
	for w := 0; w < config.WorkersCount; w++ {
		go ethSendRawWorker(jobsCh, resultsCh)
	}

	log.Println("Scheduling jobs...")
	for i := 0; i < count; i++ {
		jobsCh <- txs[i]

		log.Printf("Job #%d scheduled\n", i)

		j++
		if j == jLimit {
			j = 0
		}

		select {
		case <-r.Context().Done():
			break
		default:
			// do nothing
		}
	}

	fmt.Println("Waiting for resultsCh...")

	wg.Wait()

	resultBytes, err := json.Marshal(results)
	if err != nil {
		e := errors.New(fmt.Sprintln("error occured while executing the order: ", err.Error()))
		log.Println(e)
		respondWithError(w, http.StatusInternalServerError, e)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, string(resultBytes))
}

func ethSendRawWorker(jobs <-chan *types.Transaction, results chan<- sendResult) {
	client, err := getClient("http://127.0.0.1:8545")
	if err != nil {
		log.Panic(err)
	}

	for tx := range jobs {
		err := client.SendTransaction(context.Background(), tx)
		if err != nil {
			log.Println(err)
			results <- sendResult{true, err.Error()}
		}
		results <- sendResult{false, "ok"}
	}
}

func personalSignAndSendWorker(conns <-chan *web3.Web3,
	msgs <-chan sendOpts, results chan<- sendResult) {
	connection := <-conns
	defer connection.Provider.Close()

	for m := range msgs {
		txID, err := connection.Personal.SendTransaction(&dto.TransactionParameters{
			From:  config.Me.Address,
			To:    m.Addressee,
			Value: m.Val,
			Gas:   big.NewInt(21000),
		}, config.Me.Password)

		if err != nil {
			results <- sendResult{
				Error: true,
				Msg:   err.Error(),
			}
		} else {
			results <- sendResult{
				Error: false,
				Msg:   fmt.Sprintf("done, %s", txID),
			}
		}
	}
}

func respondWithError(w http.ResponseWriter, httpStatus int, err error) {
	w.WriteHeader(httpStatus)
	fmt.Fprintf(w, err.Error())
}

func getClient(rawUrl string) (*ethclient.Client, error) {
	c, err := ethrpc.Dial(rawUrl)
	if err != nil {
		return nil, err
	}
	return ethclient.NewClient(c), nil
}

type sendOpts struct {
	Val       *big.Int
	Addressee string
}

type sendResult struct {
	Error bool
	Msg   string
}

func (s *sendResult) String() string {
	return s.Msg
}

type ChainMap struct {
	Master     string
	Peers      []string
	Validators []string
}

type Configuration struct {
	Me struct {
		Name     string
		Address  string
		Password string
	}
	Endpoints struct {
		RPC string
		WS  string
		IPC string
	}
	PeerKey      string
	Logs         string
	Secret       string
	TxLimit      int
	WorkersCount int
}

type Worker func(conns <-chan *web3.Web3,
	msgs <-chan sendOpts, results chan<- sendResult)
