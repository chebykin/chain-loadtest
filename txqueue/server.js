const express = require('express');
const app = express();
const fs = require('fs');

app.set('views', './views');
app.set('view engine', 'ejs');
app.use('/scripts', express.static('scripts'));

app.get('/', function (req, res) {
    res.render('index', {
        hostsMap: JSON.parse(fs.readFileSync("../tmp/latest/hostsMap.json")),
        elMap: JSON.parse(fs.readFileSync("../tmp/latest/elMap.json"))
    })
});

app.listen(9000, () => console.log('Example app listening on port 9000!'));