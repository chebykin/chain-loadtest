const provider = new Web3.providers.WebsocketProvider(`ws://${hostsMap.peers[1].ip}:8546`);
const web3 = new Web3(provider);

pubnub = new PubNub({
    subscribeKey: 'sub-c-d37cdf2e-3cbc-11e8-a2e8-d2288b7dcaaf'
});

$(function () {
    let txQueueChart = c3.generate({
        bindto: "#txqueue",
        data: {
            columns: [
            ],
            type: 'bar'
        },
        bar: {
            width: {
                ratio: 0.5 // this makes bar width 50% of length between ticks
            }
            // or
            //width: 100 // this makes bar width 100px
        }
    });

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

    pubSub();
    subAllNodes(txQueueChart);
});

function subAllNodes(chart) {
    let ips = [...hostsMap.peers.map(h => h.ip), ...hostsMap.validators.map(h => h.ip)];
    console.log(ips);
    let providers = [];

    let setupPeers = function(h) {
        let provider = new Web3.providers.WebsocketProvider(`ws://${h.ip}:8546`);
        function setTransactionsCount(id = 1) {
            provider.send({
                "method": "parity_pendingTransactions",
                "params": [],
                "id": id++,
                "jsonrpc": "2.0"
            }, (err, res) => {
                if (err) {
                    console.error(err);
                } else {
                    chart.load({
                        columns: [[`${h.name} - ${h.region}`, res.result.length]]
                    });
                }

                setTimeout(() => {
                    setTransactionsCount(id);
                }, 500);
            });
        }
        setTransactionsCount()
    };
    hostsMap.peers.forEach(setupPeers);
    hostsMap.validators.forEach(setupPeers);
}

function pubSub() {
    eon.chart({
        pubnub: pubnub,
        channels: ['statistics'],
        limit: 20,
        generate: {
            bindto: '#peers'
        },
        flow: false,
        grid: {
            x: {
                show: false
            },
            y: {
                show: true
            }
        }
    });
}
