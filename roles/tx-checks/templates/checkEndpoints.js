const Web3 = require('web3');
const async = require('async');
const fs = require('fs');
const keythereum = require("keythereum");
const EthereumTx = require('ethereumjs-tx');


const WORKERS = 4;
const COUNT = 100;

const chainMap = JSON.parse(fs.readFileSync("../map.json"));
const vKey = JSON.parse(fs.readFileSync("./validator-0.json"));
const privateKey = keythereum.recover(`vpeers/validator-0`, vKey);


let web3 = new Web3(new Web3.providers.HttpProvider('http://127.0.0.1:8545'));

function personal_sendTransaction() {
    return new Promise((resolve, reject) => {
        console.time("script");

        async.timesLimit(COUNT, WORKERS, function (n, next) {
            web3.eth.personal.sendTransaction({
                from: chainMap.peers[1],
                to: chainMap.peers[0],
                value: '100000000000000000',
                gas: 21000,
                gasPrice: 1000000000
            }, `vpeers/validator-0`, next)
        }, function (err, all) {
            console.timeEnd("script");
            if (err) {
                reject(err)
            } else {
                resolve(err)
            }
        });
    });
}

function eth_sendRawTransaction() {
    return new Promise((resolve, reject) => {
        console.time("script");

        web3.eth.getTransactionCount(chainMap.peers[1],
            (err, count) => {
                if (err) {
                    reject(err);
                    return;
                }

                let txs = [];
                // TODO: get account nonce

                for (let i = 0; i < COUNT; i++) {
                    const txParams = {
                        from: chainMap.peers[1],
                        to: chainMap.peers[0],
                        nonce: Web3.utils.toHex(count++),
                        gasPrice: Web3.utils.toHex(1000000000),
                        gasLimit: Web3.utils.toHex(21000),
                        value: Web3.utils.toHex(100000000000000000),
                        // EIP 155 chainId - mainnet: 1, ropsten: 3
                        chainId: 15054
                    };

                    const tx = new EthereumTx(txParams);
                    tx.sign(privateKey);
                    txs.push(tx.serialize())
                }

                async.timesLimit(COUNT, WORKERS, function (n, next) {
                    web3.eth.sendSignedTransaction(
                        '0x' + txs[n].toString('hex'), next)
                }, function (err, all) {
                    console.timeEnd("script");
                    if (err) {
                        reject(err)
                    } else {
                        resolve(err)
                    }
                });
            });


    });
}

(async () => {
    await eth_sendRawTransaction();
})();