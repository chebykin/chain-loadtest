package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/common"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"io/ioutil"
	"log"
	"encoding/json"
	"math/big"
	"context"
	"sync"
)

func main() {
	fmt.Println("hello")
	exec()
}

const COUNT = 50000;
const WORKERS = 24;

func exec() {
	var chainMap *ChainMap
	file, err := ioutil.ReadFile("../map.json")
	if err != nil {
		log.Panicln("Unable to read chain map file:", err)
	}
	err = json.Unmarshal(file, &chainMap)
	if err != nil {
		log.Panicln("Unable to parse chain map file:", err)
	}

	file, err = ioutil.ReadFile("./validator-0.json")
	if err != nil {
		log.Panicln("Unable to read chain map file:", err)
	}

	key, _ := keystore.DecryptKey([]byte(file), "vpeers/validator-0")

	fmt.Println(key.Address.String())

	client, err := getClient("http://127.0.0.1:8545")
	if err != nil {
		log.Panic(err)
	}

	nonce, err := client.PendingNonceAt(context.Background(), key.Address)
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("nonce", nonce)

	jobs := make(chan *types.Transaction)
	results := make(chan bool)
	var wg sync.WaitGroup
	wg.Add(COUNT)
	go func() {
		for i := 0; i < COUNT; i++ {
			<-results
			wg.Done()
		}
	}()

	for i := 0; i < WORKERS; i++ {
		go worker(jobs, results)
	}

	txs := make(types.Transactions, COUNT)
	signer := types.NewEIP155Signer(big.NewInt(int64(15054)))

	for i := 0; i < COUNT; i++ {
		//fmt.Println(i)
		tx := types.NewTransaction(nonce, common.StringToAddress(chainMap.Peers[0]),
			big.NewInt(1E16),
			uint64(21000), big.NewInt(1E9), []byte(""))

		signed_tx, _ := types.SignTx(tx, signer, key.PrivateKey)
		txs[i] = signed_tx
		nonce++
	}


	from, _ := types.Sender(signer, txs[0])
	fmt.Println("from", from.String())

	for _, tx := range txs {
		jobs <- tx
	}

	wg.Wait()
}

func worker(jobs <-chan *types.Transaction, results chan<- bool) {
	client, err := getClient("http://127.0.0.1:8545")
	if err != nil {
		log.Panic(err)
	}

	for tx := range jobs {
		err := client.SendTransaction(context.Background(), tx)
		if err != nil {
			log.Println(err)
			results <- false
		}
		results <- true
	}
}

func getClient(rawurl string) (*ethclient.Client, error){
	c, err := ethrpc.Dial(rawurl)
	if err != nil {
		return nil, err
	}
	return ethclient.NewClient(c), nil
}

type ChainMap struct {
	Master     string
	Peers      []string
	Validators []string
}