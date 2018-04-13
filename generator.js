let name = process.argv[2];

if (!name) {
    console.log('ERROR: You need to specify name for a data file');
    process.exit(1);
}

let ejs = require('ejs');
let fs = require('fs');

let template = fs.readFileSync("./templates/report.ejs", "utf-8");
let data = JSON.parse(fs.readFileSync(`./aura-loadtest-results/data/${name}.json`));

let hostsMap = JSON.parse(fs.readFileSync("./tmp/latest/hostsMap.json"));
let elMap = JSON.parse(fs.readFileSync("./tmp/latest/elMap.json"));

let renderData = {
    name,
    digitalocean: data.do,
    hardware: data.hardware,
    chainMap: data.chainMap,
    runs: []
};

let totalSeconds = 0;
let totalMaxTxPerBlock = [];
let totalAvgPerBlock = 0;
let totalAvgPerSecond = 0;

for (let run of data.runs) {
    for (let subRun of run.subRuns) {
        subRun.perNode = run.perNode;
        subRun.totalSent = run.perNode * Object.keys(data.chainMap).length;
        subRun.blocks = [];
        subRun.time = data.blocks[subRun.end].t - data.blocks[subRun.start - 1].t;
        subRun.max = 0;

        subRun.avgPerBlock = Math.round(subRun.totalSent / (subRun.end - subRun.start + 1));
        subRun.avgPerSecond = Math.round(subRun.totalSent / subRun.time);

        for (let i = subRun.end + 1; i >= subRun.start - 1; i--) {
            let b = data.blocks[i];
            b.m = elMap[b.a];

            if (b.tC > subRun.max) {
                subRun.max = b.tC
            }

            subRun.blocks.push(b);
        }

        // TODO: calculate summary
        totalSeconds += subRun.time;
        totalMaxTxPerBlock.push(subRun.max);
        totalAvgPerBlock += subRun.avgPerBlock;
        totalAvgPerSecond += subRun.avgPerSecond;

        renderData.runs.push(subRun);
    }
}

let c = data.runs.length;

renderData.avg = {
    seconds: Math.round(totalSeconds / c),
    maxTxPerBlock: Math.max(...totalMaxTxPerBlock),
    avgPerBlock: Math.round(totalAvgPerBlock / c),
    avgPerSecond: Math.round(totalAvgPerSecond / c),
};

let html = ejs.render(template, renderData);
fs.writeFileSync(`./aura-loadtest-results/docs/${name}.html`, html);