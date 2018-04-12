let name = process.argv[2];

if (!name) {
    console.log('ERROR: You need to specify name for a data file');
    process.exit(1);
}

let ejs = require('ejs');
let fs = require('fs');

const TX_NUM = 30000;

let template = fs.readFileSync("./templates/report.ejs", "utf-8");
let data = JSON.parse(fs.readFileSync(`./data/${name}.json`));

let hostsMap = JSON.parse(fs.readFileSync("./tmp/latest/hostsMap.json"));
let elMap = JSON.parse(fs.readFileSync("./tmp/latest/elMap.json"));

let renderData = {
    name,
    digitalocean: data.do,
    hardware: data.hardware,
    runs: []
};

let totalSeconds = 0;
let totalMaxTxPerBlock = [];
let totalAvgPerBlock = 0;
let totalAvgPerSecond = 0;

for (let run of data.runs) {
    run.blocks = [];
    run.time = data.blocks[run.end].t - data.blocks[run.start - 1].t;
    run.max = 0;

    run.avgPerBlock = Math.round(TX_NUM / (run.end - run.start + 1));
    run.avgPerSecond = Math.round(TX_NUM/run.time);

    for (let i = run.end + 1; i >= run.start - 1; i--) {
        let b = data.blocks[i];
        b.m = elMap[b.a];

        if (b.tC > run.max) {
            run.max = b.tC
        }

        run.blocks.push(b);
    }

    // TODO: calculate summary
    totalSeconds += run.time;
    totalMaxTxPerBlock.push(run.max);
    totalAvgPerBlock += run.avgPerBlock;
    totalAvgPerSecond += run.avgPerSecond;

    renderData.runs.push(run);
}

let c = data.runs.length;

renderData.avg = {
    seconds: Math.round(totalSeconds / c),
    maxTxPerBlock: Math.max(...totalMaxTxPerBlock),
    avgPerBlock: Math.round(totalAvgPerBlock / c),
    avgPerSecond: Math.round(totalAvgPerSecond / c),
};

let html = ejs.render(template, renderData);
fs.writeFileSync(`./docs/${name}.html`, html);