const fetch = require('node-fetch');
const async = require('async');
const YAML = require('yamljs');
const assert = require('assert');
const fs = require('fs');

const allConfig = YAML.load("./group_vars/all.yml");

assert.ok(typeof process.env.DO_API_TOKEN === 'string');
assert.ok(process.env.DO_API_TOKEN.length > 0);

function fetchIPs() {
    let ids = [];

    return new Promise((resolve, reject) => {
        console.log("Fetching droplet IDs...");
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
                    ids.push(droplet.id);
                });
                return Promise.resolve(ids);
            })
            .then(ids => {
                async.each(ids, (id, callback) => {
                    fetch(`https://api.digitalocean.com/v2/droplets/${id}/actions`, {
                        method: 'post',
                        headers: {
                            'Authorization': 'Bearer ' + process.env.DO_API_TOKEN,
                            'Content-Type': 'application/json'
                        },
                        body: JSON.stringify({
                            "type": "power_on",
                        })

                    })
                        .then(res => res.json())
                        .then(json => {
                            console.log('ok', json);
                            callback(null, json)
                        })
                        .catch(err => {
                            console.log('error', err);
                            callback(err)
                        });

                }, (err, done) => {
                    if (err) {
                        return resolve(err);
                    }

                    console.log(done);
                });
            })
            .catch(reject)
    });
}

(async () => {
    await fetchIPs();
})();