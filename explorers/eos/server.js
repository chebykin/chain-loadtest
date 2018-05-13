const express = require('express');
const async = require('async');
const app = express();

const API_SERVER = 'http://167.99.39.175:8888';

app.set('views', './views');
app.set('view engine', 'ejs');
app.use('/scripts', express.static('scripts'));

api = require('eosjs-api');

options = {
    httpEndpoint: API_SERVER,
    debug: true,
    logger: {
        log: console.log,
        error: console.error,
        debug: console.debug
    },
    fetchConfiguration: {}
};

eos = api.Localnet(options);

app.get('/', function (req, res) {
    async.waterfall([
        function (callback) {
            eos.getInfo(function (err, result) {
                callback(err, result);
            });
        }, function (lastBlock, callback) {
            let to = parseInt(req.query.to);
            if (to > 0) {
                lastBlock = {head_block_num: to}
            }

            let blockCount = 30;

            let count = parseInt(req.query.count);
            if (count > 0) {
                blockCount = count;
            }

            if (lastBlock.number - blockCount < 0) {
                blockCount = lastBlock.number + 1;
            }

            console.log('latestBlock', lastBlock.head_block_num, 'count', blockCount);

            async.times(blockCount, function (n, next) {
                eos.getBlock(lastBlock.head_block_num - n, function (err, block) {
                    next(err, block);
                });
            }, function (err, blocks) {
                callback(err, blocks);
            });
        }
    ], function (err, blocks) {
        if (err) {
            return next(err);
        }

        res.render('index', {
            blocks: blocks
        });

    });
});

app.listen(9000, () => console.log('Example app listening on port 9000!'));