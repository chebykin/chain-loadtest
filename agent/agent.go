package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"strconv"
	"sync"

	web3 "github.com/chebykin/go-web3"
	"github.com/chebykin/go-web3/providers"
	"github.com/regcostajr/go-web3/dto"
	"github.com/gorilla/mux"
	"net/http"
	"time"
	"log"
	"errors"
)

// Ether multiplier
const METHER = 1000000000000000000
// TODO: should be configured using env
// How many working threads parity has sat in rpc config section
const WORKERS_COUNT = 4

var chainMap *ChainMap
var connection *web3.Web3

// Agent starts http server to receive instructions from the master.
// It sends statistics to statsd daemon.
func main() {
	connection = web3.NewWeb3(
		providers.NewHTTPProvider("127.0.0.1:8545", 10, false))
	// providers.NewWebSocketProvider("ws://127.0.0.1:8546"))
	// providers.NewIPCProvider("/tmp/parity.ipc"))

	abs, _ := filepath.Abs("./tmp/latest/map.json")
	fmt.Println(abs)

	file, err := ioutil.ReadFile(abs)
	if err != nil {
		panic("Unable to read chain map file")
	}

	err = json.Unmarshal(file, &chainMap)
	if err != nil {
		panic(fmt.Sprintln("Unable to parse chain map file:", err))
	}

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
	// TODO: add a secret key validation
	// TODO: parse request query for instructions

	log.Println(r.URL.Query().Get("sendtx"))
	count, err := strconv.Atoi(r.URL.Query().Get("sendtx"))
	if err != nil {
		log.Println("Wrong number of txs", err)
		respondWithError(w, http.StatusBadRequest, errors.New("Wrong number of txs"))
		return
	}

	result := monkey(count)
	resultBytes, err := json.Marshal(result)
	if err != nil {
		e := errors.New(fmt.Sprintln("Error occured while executing the order: ", err.Error()))
		log.Println(e)
		respondWithError(w, http.StatusInternalServerError, e)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, string(resultBytes))
}

// TODO: move to another script
func master() {
	var wg sync.WaitGroup
	val := big.NewInt(0).Mul(big.NewInt(200), big.NewInt(1E18))

	for _, account := range chainMap.Accounts {
		wg.Add(1)
		go func(a string) {
			txId, err := connection.Personal.SendTransaction(&dto.TransactionParameters{
				From:  chainMap.Master,
				To:    a,
				Value: val,
			}, "master")
			if err != nil {
				fmt.Println("Failed to send tx", err)
			} else {
				fmt.Println("done", txId)
			}
			wg.Done()
		}(account)
	}

	wg.Wait()
}

// Monkey will start a loop which sends either to each peer and validator
func monkey(count int) []sendResult {
	j := 0
	val := big.NewInt(0).Mul(big.NewInt(34), big.NewInt(1E16))

	jobsCh := make(chan sendOpts)
	resultsCh := make(chan sendResult)

	results := make([]sendResult, count)

	var wg sync.WaitGroup
	wg.Add(count)

	go func() {
		for i := 0; i < count; i++ {
			result := <-resultsCh
			log.Println("<<<", result)
			results[i] = result
			wg.Done()
		}
	}()

	log.Println("Launching", WORKERS_COUNT, "workers...")
	for w := 0; w < WORKERS_COUNT; w++ {
		go monkeyWorker(w, jobsCh, resultsCh)
	}

	log.Println("Scheduling jobs...")
	for i := 0; i < count; i++ {
		jobsCh <- sendOpts{
			Val:       val,
			Addressee: chainMap.Accounts[j],
		}

		log.Printf("Job #%d scheduled\n", i)

		j++
		if j == 5 {
			j = 0
		}
	}

	fmt.Println("Waiting for resultsCh...")

	wg.Wait()

	return results
}

func monkeyWorker(id int, msgs <-chan sendOpts, results chan<- sendResult) {
	// TODO: create an own client
	// connection := web3.NewWeb3(
	// 	providers.NewHTTPProvider("127.0.0.1:8545", 10, false))
	// providers.NewWebSocketProvider("ws://127.0.0.1:8546"))
	// providers.NewIPCProvider("/tmp/parity.ipc"))

	for m := range msgs {
		txId, err := connection.Personal.SendTransaction(&dto.TransactionParameters{
			From:  chainMap.Master,
			To:    m.Addressee,
			Value: m.Val,
			Gas:   big.NewInt(21999),
		}, "master")

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
	Master   string
	Accounts []string
}
