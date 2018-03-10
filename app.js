const keythereum = require("keythereum");
const fs = require("fs");

const count = 10;
const time = new Date().getTime();
const dir = `./tmp/${time}`;

let addresses = [];

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

for (let i = 0; i < count; i++) {
    addresses.push(generate(`validator-${i}`));
}

let master = generate('master');

let chainSpec = JSON.parse(fs.readFileSync("./templates/spec.json"));
chainSpec.engine.authorityRound.params.validators.list = addresses;
chainSpec.accounts[master] = "252460800000000000000000000";

fs.writeFileSync(`${dir}/spec.json`, JSON.stringify(chainSpec, null, 2));

console.log(`Generated keys and configs are located at ${dir}.`);