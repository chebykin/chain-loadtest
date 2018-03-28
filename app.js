const keythereum = require("keythereum");
const fs = require("fs");
YAML = require('yamljs');

const time = new Date().getTime();
const dir = `./tmp/${time}`;

const allConfig = YAML.load("./group_vars/all");
const peersCount = allConfig['peers_count'];
const validatorsCount = allConfig['validators_count'];

let accounts = [];

if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir);
}

function generate(name) {
    let dk = keythereum.create({keyBytes: 32, ivBytes: 16});

    // NOTE: name used as a password
    let obj = keythereum.dump(name, dk.privateKey, dk.salt, dk.iv);

    fs.writeFileSync(`${dir}/${name}.json`, JSON.stringify(obj));
    return keythereum.privateKeyToAddress(dk.privateKey);
}

console.log('Generating peer accounts....');

for (let i = 0; i < peersCount; i++) {
    accounts.push(generate(`peer-${i}`));
}

console.log('Generating peer accounts....');

for (let i = 0; i < validatorsCount; i++) {
    accounts.push(generate(`validator-${i}`));
}

console.log('Generating master account ....');

let master = generate('master');

let chainSpec = JSON.parse(fs.readFileSync("./templates/spec.json"));
chainSpec.engine.authorityRound.params.validators.list = accounts;
chainSpec.accounts[master] = {
    balance: "252460800000000000000000000"
};

for (let i = 0; i < accounts.length; i++) {
    chainSpec.accounts[accounts[i]] = {
        balance: "100000000000000000000000"
    };
}

fs.writeFileSync(`${dir}/spec.json`, JSON.stringify(chainSpec, null, 2));

console.log('Generating chain map...');

let chainMap = {
    master,
    accounts
};

fs.writeFileSync(`${dir}/map.json`, JSON.stringify(chainMap, null, 2));

console.log('Copying Parity configs...');

fs.createReadStream('./templates/local.toml').pipe(fs.createWriteStream(`./tmp/${time}/local.toml`));

console.log('Saving timestamp to all config ...');

allConfig.timestamp = time;
fs.writeFileSync("./group_vars/all", YAML.stringify(allConfig));

console.log('Creating latest symlink...');

try {
    fs.unlinkSync('./tmp/latest');
} catch (e) {
}
fs.symlinkSync(`./${time}`, './tmp/latest');

console.log(`\nDone!\n\nGenerated keys and configs are located at ${dir}.\n`);