const fs = require('fs'),
    ini = require('ini'),
    async = require('async'),
    YAML = require('yamljs'),
    fetch = require('node-fetch');

const hostsFile = ini.parse(fs.readFileSync('./hosts.txt', 'utf-8'));
const allConfig = YAML.load("./group_vars/all");
const secret = allConfig.agent_secret;

let peers = [];

let peerKeys = Object.keys(hostsFile.peer);
let validatorKeys = Object.keys(hostsFile.validator);

for (let i = 0; i < peerKeys.length; i++) {
    let s = peerKeys[i];
    peers.push(s.substring(0, s.length - 5))
}

for (let i = 0; i < validatorKeys.length; i++) {
    let s = validatorKeys[i];
    peers.push(s.substring(0, s.length - 5))
}

let count = process.argv[2];

if (!count) {
    console.log('ERROR: You need to specify count (number) as a second arg');
    process.exit(1);
}

console.log('Count is', count);

let urls = [];

peers.forEach((p) => {
    urls.push(`http://${p}:8080/orders?sendtx=${count}&secret=${secret}`);
});

console.log("Urls are\n", urls.join("\n"));

let tasks = [];

urls.forEach((u) => {
    tasks.push((callback) => {
        fetch(u)
            .then((r) => {
                callback(null, r);
            })
            .catch((e) => {
                callback(e, null)
            })
    })
});

(async function()  {
    // TODO: in case of error it just exits, no debug info
    await async.parallel(tasks)
})();
