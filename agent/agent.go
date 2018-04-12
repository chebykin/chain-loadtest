package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"sort"
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
	"github.com/pubnub/go/messaging"
	web3 "github.com/regcostajr/go-web3"
	"github.com/regcostajr/go-web3/dto"
	"github.com/regcostajr/go-web3/providers"
)

var config *Configuration
var chainMap *ChainMap
var statsdClient statsd.Statter
var pubnub *messaging.Pubnub

const (
	pnPubKey  = "demo"
	pnSubKey  = "demo"
	pnChannel = "statistics"
)

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

	pubnub = messaging.NewPubnub(pnPubKey, pnSubKey, "", "", false, "", nil)

	defer statsdClient.Close()

	server()
}

func server() {
	r := mux.NewRouter()
	r.Path("/ethSendRaw").HandlerFunc(ethSendRaw)
	r.Path("/personalSignAndSend").HandlerFunc(personalSignAndSendHandler)
	r.Path("/progress").HandlerFunc(progressHandler)
	r.Path("/workpackage").HandlerFunc(workpackageHandler)
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

func progressHandler(w http.ResponseWriter, r *http.Request) {
}

func workpackageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Receive workload")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
	}
	success := make(chan []byte)
	error := make(chan []byte)

	go pubnub.Publish("work", struct {
		Name      string `json:"name"`
		Payload   string `json:"payload"`
		Timestamp int64  `json:"timestamp"`
	}{
		Name:      config.Me.Name,
		Payload:   string(body),
		Timestamp: time.Now().Unix() * 1000,
	}, success, error)
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
	key, err := keystore.DecryptKey([]byte(file), config.Me.Password)
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("Decoded key address", key.Address)

	client, err := getClient(fmt.Sprintf("http://%s", config.Endpoints.RPC))
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
		payload := make([]byte, 200)
		rand.Read(payload)
		tx := types.NewTransaction(nonce, common.BytesToAddress([]byte(chainMap.Peers[0])),
			big.NewInt(1E16),
			uint64(50000), big.NewInt(1E9), payload)

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

	msgs := make(chan counterMessage, 100*1000)
	go counter(r.Context(), msgs)

	log.Println("Launching", config.WorkersCount, "workers...")
	for w := 0; w < config.WorkersCount; w++ {
		go ethSendRawWorker(uint8(w), msgs, jobsCh, resultsCh)
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

func ethSendRawWorker(id uint8, countMsgs chan<- counterMessage,
	jobs <-chan *types.Transaction, results chan<- sendResult) {

	client, err := getClient(fmt.Sprintf("http://%s", config.Endpoints.RPC))
	if err != nil {
		log.Panic(err)
	}

	for tx := range jobs {
		err := client.SendTransaction(context.Background(), tx)
		countMsgs <- counterMessage{id}
		if err != nil {
			log.Println(err)
			results <- sendResult{true, err.Error()}
			return
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

func counter(ctx context.Context, msgs <-chan counterMessage) {
	store := make(map[uint8]int)
	total := 0
	pubnub := messaging.NewPubnub(pnPubKey, pnSubKey, "", "", false, "", nil)

	writeTick := func() {
		var buffer bytes.Buffer
		now := time.Now()
		buffer.WriteString(now.Format("15:04:05 "))

		keys := make([]int, len(store))
		i := 0

		for k := range store {
			keys[i] = int(k)
			i++
		}
		sort.Ints(keys)

		for k := range keys {
			v := store[uint8(k)]
			buffer.WriteString(fmt.Sprintf("W%d: %d; ", k, v))
		}

		buffer.WriteString(fmt.Sprintf("total: %d\n", total))

		f, err := os.OpenFile(config.TickerLog,
			os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		defer f.Close()

		if err != nil {
			log.Panic(err)
		}

		success := make(chan []byte)
		error := make(chan []byte)

		go pubnub.Publish(pnChannel, struct {
			Eon       map[string]int `json:"eon"`
			Timestamp int64          `json:"_eonDatetime"`
		}{
			Eon: map[string]int{
				config.Me.Name: total,
			},
			Timestamp: time.Now().Unix() * 1000,
		}, success, error)

		_, err = f.Write(buffer.Bytes())
		if err != nil {
			log.Panic(err)
		}

		store = make(map[uint8]int)
		total = 0
	}

	ticker := time.NewTicker(time.Millisecond * 1000)

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				writeTick()
				return
			case <-ticker.C:
				writeTick()
			case msg := <-msgs:
				store[msg.WorkerID]++
				total++
			}
		}
	}()
}

type counterMessage struct {
	WorkerID uint8
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
	TickerLog    string
	Secret       string
	TxLimit      int
	WorkersCount int
}

type Worker func(conns <-chan *web3.Web3,
	msgs <-chan sendOpts, results chan<- sendResult)
