// Fetches current chain blocks info into json file
let Web3 = require('web3');
let async = require('async');
let fs = require('fs');
const YAML = require('yamljs');

let name = process.argv[2];

if (!name) {
    console.log('ERROR: You need to specify name for a data file');
    process.exit(1);
}

const allConfig = YAML.load("./group_vars/all.yml");
let hostsMap = JSON.parse(fs.readFileSync("./tmp/latest/hostsMap.json"));
const cpusYaml = YAML.load("./tmp/latest/cpuinfo.txt");

const provider = new Web3.providers.HttpProvider(`http://${hostsMap.peers[1].ip}:8545`);
let web3 = new Web3();

web3.setProvider(provider);

async.waterfall([
    function (callback) {
        web3.eth.getBlock("latest", false, function (err, result) {
            callback(err, result);
        });
    }, function (lastBlock, callback) {

        let blocks = [];

        async.timesLimit(lastBlock.number, 50, function (n, next) {
            web3.eth.getBlock(n, false, function (err, block) {
                console.log(`Fetching block #${n}`);
                next(err, block);
            });
        }, function (err, blocks) {
            callback(err, blocks);
        });
    }
], function (err, rawBlocks) {
    if (err) {
        return next(err);
    }

    let blocks = {};

    for (let i = 0; i < rawBlocks.length; i++) {
        let b = rawBlocks[i];
        blocks[i] = {
            a: b.author,
            gL: b.gasLimit,
            gU: b.gasUsed,
            s: b.size,
            t: b.timestamp,
            n: b.number,
            tC: b.transactions.length,
        }
    }

    let data = {
        blocks,
        do: {
            size: allConfig.do_size,
            coresCount: allConfig.processor_count,
            image: allConfig.do_image_id
        },
        hardware: Object.keys(cpusYaml).sort().reduce((r, k) => (r[k] = cpusYaml[k], r), {})
    };

    fs.writeFileSync(`./data/${name}.json`, JSON.stringify(data, null, 2));
});
