const fetch = require('node-fetch');
const async = require('async');
const YAML = require('yamljs');
const assert = require('assert');
const fs = require('fs');

const allConfig = YAML.load("./group_vars/all.yml");
const map = YAML.load(`./maps/${allConfig.map}.yml`);

assert.ok(typeof process.env.DO_API_TOKEN === 'string');
assert.ok(process.env.DO_API_TOKEN.length > 0);

function createNodes(map) {
    console.log("Creating nodes...");
    assert.ok(Array.isArray(map));
    return new Promise((resolve, reject) => {
        async.each(map, (region, callback) => {
            let names = [...region.peers, ...region.validators];
            if (names.length === 0) {
                callback();
                return;
            }

            fetch(`https://api.digitalocean.com/v2/droplets`,
                {
                    method: 'post',
                    body: JSON.stringify({
                        names: names,
                        region: region.region,
                        size: allConfig.do_size,
                        image: allConfig.do_image_id,
                        ssh_keys: [allConfig.do_key_id],
                        tags: [`tn-${allConfig.timestamp}`]
                    }),
                    headers: {
                        'Authorization': 'Bearer ' + process.env.DO_API_TOKEN,
                        'Content-Type': 'application/json'
                    }
                })
                .then(async(res) => {
                    if (res.status !== 202) {
                        console.log('Bad response (ignored)', await res.text());
                    }
                    callback();
                })
                .catch(reject);

        }, (err) => {
            if (err) {
                reject(err);
            }
            resolve();
        });
    });
}

function fetchIPs() {
    let validators = [];
    let peers = [];

    return new Promise((resolve, reject) => {
        console.log("Fetching droplet IPs...");
        fetch(`https://api.digitalocean.com/v2/droplets?tag_name=tn-${allConfig.timestamp}&per_page=50`,
            {
                method: 'get',
                headers: {
                    'Authorization': 'Bearer ' + process.env.DO_API_TOKEN,
                    'Content-Type': 'application/json'
                }
            })
            .then(res => res.json())
            .then(json => {
                json.droplets.forEach(droplet => {
                    if (droplet.name.startsWith("validator")) {
                        validators.push({
                            name: droplet.name,
                            ip: droplet.networks.v4[0].ip_address,
                            region: droplet.region.slug
                        })
                    } else if (droplet.name.startsWith("peer")) {
                        peers.push({
                            name: droplet.name,
                            ip: droplet.networks.v4[0].ip_address,
                            region: droplet.region.slug
                        })                    } else {
                        reject(new Error(`Unexpected name: ${droplet.name}`))
                    }
                });

                console.log("Generating hosts.txt file...");

                peers.sort((astr, bstr) => {
                    let a = astr.name.substring(5);
                    let b = bstr.name.substring(5);
                    return parseInt(a) - parseInt(b);
                });

                validators.sort((astr, bstr) => {
                    let a = astr.name.substring(10);
                    let b = bstr.name.substring(10);
                    return parseInt(a) - parseInt(b);
                });

                let content = `[peer]\n`;

                peers.forEach(peer => {
                    content += `${peer.ip} name=${peer.name} type=peer\n`
                });

                content += `\n[validator]\n`;

                validators.forEach(validator => {
                    content += `${validator.ip} name=${validator.name} type=validator\n`
                });

                fs.writeFileSync("./hosts.txt", content);
                fs.writeFileSync("./tmp/latest/hostsMap.json", JSON.stringify({
                    peers,
                    validators
                }), null, 2);
                console.log('Generating hosts.txt done');
            })
            .catch(reject)
    });
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

(async () => {
    await createNodes(map);
    console.log('Sleep 30 seconds...');
    await sleep(30 * 1000);
    await fetchIPs();
})();