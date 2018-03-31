package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"strconv"
	"sync"

	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/chebykin/go-web3"
	"github.com/chebykin/go-web3/providers"
	"github.com/gorilla/mux"
	"github.com/regcostajr/go-web3/dto"
)

// Ether multiplier
const METHER = 1000000000000000000

// TODO: should be configured using env
// How many working threads parity has sat in rpc config section

var config *Configuration
var chainMap *ChainMap
var connection *web3.Web3
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

	// Reading chain map
	file, err = ioutil.ReadFile("./map.json")
	if err != nil {
		log.Panicln("Unable to read chain map file:", err)
	}

	err = json.Unmarshal(file, &chainMap)
	if err != nil {
		log.Panicln("Unable to parse chain map file:", err)
	}

	connection = web3.NewWeb3(
		providers.NewHTTPProvider(config.RpcEndpoint, 10, false))

	coinbase, err := connection.Eth.GetCoinbase()
	if err != nil {
		log.Panicln("Failed to get coinbase", err)
	}
	log.Println("coinbase", coinbase)

	statsdClient, err = statsd.NewClient("127.0.0.1:8125", "test-client")
	if err != nil {
		log.Panicln("Failed to connect statsd server:", err)
	}

	defer statsdClient.Close()

	server()
}

func server() {
	r := mux.NewRouter()
	r.Path("/orders").HandlerFunc(ordersHandler)
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

func ordersHandler(w http.ResponseWriter, r *http.Request) {
	secret := r.URL.Query().Get("secret")
	if secret != config.Secret {
		log.Println("Wrong secret")
		respondWithError(w, http.StatusBadRequest, errors.New("wrong secret"))
		return
	}

	log.Println(r.URL.Query().Get("sendtx"))
	count, err := strconv.Atoi(r.URL.Query().Get("sendtx"))
	if err != nil {
		log.Println("Wrong number of txs", err)
		respondWithError(w, http.StatusBadRequest, errors.New("wrong number of txs"))
		return
	}

	if count > config.TxLimit {
		msg := fmt.Sprintln("request tx count exceeded limit of", config.TxLimit)
		log.Println(msg)
		respondWithError(w, http.StatusBadRequest, errors.New(msg))
		return
	}

	result := monkey(count, r.Context())
	resultBytes, err := json.Marshal(result)
	if err != nil {
		e := errors.New(fmt.Sprintln("error occured while executing the order: ", err.Error()))
		log.Println(e)
		respondWithError(w, http.StatusInternalServerError, e)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, string(resultBytes))
}

// Monkey will start a loop which sends either to each peer and validator
func monkey(count int, ctx context.Context) []sendResult {
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
			case <-ctx.Done():
				log.Printf("Request cancelled. Skipping result for #%d\n", i)
				// nothing
			}
			wg.Done()
		}
	}()

	log.Println("Launching", config.WorkersCount, "workers...")
	for w := 0; w < config.WorkersCount; w++ {
		go monkeyWorker(w, jobsCh, resultsCh)
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
		case <-ctx.Done():
			break
		default:
			// do nothing
		}
	}

	fmt.Println("Waiting for resultsCh...")

	wg.Wait()

	return results
}

func monkeyWorker(id int, msgs <-chan sendOpts, results chan<- sendResult) {
	// TODO: create an own client
	connection := web3.NewWeb3(
		providers.NewHTTPProvider(config.RpcEndpoint, 10, false))
	// providers.NewWebSocketProvider("ws://127.0.0.1:8546"))
	// providers.NewIPCProvider("/tmp/parity.ipc"))

	for m := range msgs {
		txId, err := connection.Personal.SendTransaction(&dto.TransactionParameters{
			From:  config.Me.Address,
			To:    m.Addressee,
			Value: m.Val,
			Gas:   big.NewInt(21999),
		}, config.Me.Password)

		if err != nil {
			results <- sendResult{
				Error: true,
				Msg:   err.Error(),
			}
		} else {
			results <- sendResult{
				Error: false,
				Msg:   fmt.Sprintf("done, %s", txId),
			}
		}
	}
}

func respondWithError(w http.ResponseWriter, httpStatus int, err error) {
	w.WriteHeader(httpStatus)
	fmt.Fprintf(w, err.Error())
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
		Address  string
		Password string
	}
	RpcEndpoint  string
	Secret       string
	TxLimit      int
	WorkersCount int
}
