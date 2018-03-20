package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	web3 "github.com/chebykin/go-web3"
	"github.com/chebykin/go-web3/providers"
	"github.com/regcostajr/go-web3/dto"
)

// Ether multiplier
const METHER = 1000000000000000000

var chainMap *ChainMap
var connection *web3.Web3

func main() {
	connection = web3.NewWeb3(
		providers.NewHTTPProvider("127.0.0.1:8545", 10, false))
	// providers.NewWebSocketProvider("ws://127.0.0.1:8546"))
	// providers.NewIPCProvider("/tmp/parity.ipc"))

	cmd := os.Args[1]

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

	switch cmd {
	case "master":
		master()
	case "monkey":
		validatorIdStr := os.Args[2]
		validatorId, err := strconv.Atoi(validatorIdStr)
		if err != nil {
			panic("Wrong validator id, use only numbers")
		}

		monkey(validatorId)
	}
}

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

func monkey(validatorId int) {
	fmt.Println("monkey", validatorId)

	count := 500
	j := 0
	val := big.NewInt(0).Mul(big.NewInt(34), big.NewInt(1E16))

	jobs := make(chan sendOpts)
	results := make(chan sendResult)

	var wg sync.WaitGroup
	wg.Add(count)

	go func() {
		for i := 0; i < count; i++ {
			fmt.Println(<-results)
			wg.Done()
		}
	}()

	for w := 0; w < 50; w++ {
		go monkeyWorker(w, jobs, results)
	}

	fmt.Println("Scheduling jobs...")
	for i := 0; i < count; i++ {
		jobs <- sendOpts{
			Val:       val,
			Addressee: chainMap.Accounts[j],
		}

		fmt.Printf("Job #%d scheduled\n", i)

		j++
		if j == 5 {
			j = 0
		}
	}

	fmt.Println("Waiting for results...")

	wg.Wait()
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
				Error: true,
				Msg:   fmt.Sprintf("done, %s", txId),
			}
		}
	}
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
