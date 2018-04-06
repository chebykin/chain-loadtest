const Web3 = require('web3');
const net = require('net');
const async = require('async');
const fs = require('fs');

const WORKERS = 4;
const COUNT = 100;

let chainMap = JSON.parse(fs.readFileSync("../map.json"));

let web3 = new Web3(new Web3.providers.HttpProvider('http://127.0.0.1:8545'));
// let web3 = new Web3(new Web3.providers.WebsocketProvider('ws://127.0.0.1:8546'));
// let web3 = new Web3(new Web3.providers.IpcProvider('/tmp/parity.ipc', net));

function exec() {
    return new Promise((resolve, reject) => {
        async.timesLimit(COUNT, WORKERS, function (n, next) {
            web3.eth.personal.sendTransaction({
                from: chainMap.peers[1],
                to: chainMap.peers[0],
                value: '100000000000000000',
                gas: 21000,
                gasPrice: 1000000000
            }, `vpeers/validator-0`, next)
        }, function (err, all) {
            if (err) {
                reject(err)
            } else {
                resolve(err)
            }
        });
    });

}

(async () => {
    console.time("script");
    await exec();
    console.timeEnd("script");
})();