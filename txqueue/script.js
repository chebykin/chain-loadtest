const provider = new Web3.providers.WebsocketProvider('ws://174.138.7.70:8546');
const provider2 = new Web3.providers.WebsocketProvider('ws://174.138.7.70:8546');
const web3 = new Web3(provider);

let elMap = { '0xcce44589fccb86d4ef56759b1ee6819d64d57f36': 'ams3 - validator-2',
    '0xf11aa0e1d16b0dfedd0b983bf39468b80560c83b': 'ams3 - validator-6',
    '0xb6a1c7292ce4fb5c42950b717a9ba5e690accf08': 'lon1 - validator-1',
    '0xcfee23889a652bbfe2bf5c19cc2097de8eb72869': 'fra1 - validator-3',
    '0x1c846a6d801872db5a1f55f148342558167b0ad1': 'fra1 - validator-4',
    '0xf9c2d91d8f9799d6828e5cd3965c7a7458796b66': 'nyc1 - validator-11',
    '0xb4f8a894b5c83d6f11ed3e135e0b32d5194bfd92': 'nyc1 - validator-14',
    '0x84487d832c2f8efb264c8a1d04365d38a0c7a6de': 'sfo2 - validator-5',
    '0x5791e923c46d511eea1874652b2aff9d0687f192': 'tor1 - validator-0',
    '0x93637898bdbe030bf8501adb52483c907231d7ce': 'tor1 - validator-10',
    '0xf91992804305cda495af9686d9e13f4dd5ab8578': 'tor1 - validator-13',
    '0xe8264a79a2cd70e1dce40f1a0742cfb96b07317b': 'blr1 - validator-7',
    '0x45a92ca0ae2add898438883ec9eb39fd665df724': 'blr1 - validator-9',
    '0xb8add477db57aef898a8fbfdc98d8ae69421e761': 'sgp1 - validator-8',
    '0xfc384492896e3992fcc49f38941aeeb69a5b5be9': 'sgp1 - validator-12' };

$(function () {
    getWeb3.then(() => {
        let subscription = web3.eth.subscribe('newBlockHeaders')
            .on("data", function (block) {

                web3.eth.getBlock(block.number)
                    .then((block) => {
                        let now = new Date(block.timestamp * 1000);
                        let time = `${now.getUTCHours()}:${now.getUTCMinutes()}:${now.getUTCSeconds()}`;

                        console.log(
                            `${time}#${block.number}: ${block.transactions.length} txs; %c${elMap[block.miner.toLowerCase()]}`
                            , 'background: #222; color: #bada55');
                    });
            });

        function setTransactionsCount(id = 1) {
            provider2.send({
                "method":"parity_pendingTransactions",
                "params":[],
                "id":id++,
                "jsonrpc":"2.0"
            }, (err, res) => {
                    if (err) {
                        console.error(err);
                    } else {
                        $("#tx-count").text(res.result.length);
                        // provider2.send({
                        //     "method":"parity_localTransactions",
                        //     "params":[],
                        //     "id":id++,
                        //     "jsonrpc":"2.0"
                        // }, (err, res) => {
                        //     if (err) {
                        //         console.error(err);
                        //     } else {
                        //         console.log(res);
                        //         $("#tx-local").text(res.result.length);
                                setTimeout(() => {
                                    setTransactionsCount(id);
                                }, 1000);
                        //     }
                        // });
                    }
                });
        }

        setTransactionsCount();
    })
});

const getWeb3 = new Promise(function (resolve, reject) {
    // Wait for loading completion to avoid race conditions with web3 injection timing.
    let results;
    let web3 = window.web3;

    // Checking if Web3 has been injected by the browser (Mist/MetaMask)
    if (typeof web3 !== 'undefined') {
        // Use Mist/MetaMask's provider.

        web3 = new Web3(window.web3.currentProvider);
        window.web4 = web3;

        results = {
            web3,
        };

        console.log('Injected web3 detected.');

        resolve(results);
    } else {
        reject();
    }
});