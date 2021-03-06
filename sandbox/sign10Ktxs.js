const EthereumTx = require('ethereumjs-tx');
const privateKey = Buffer.from('e331b6d69882b4cb4ea581d88e0b604039a3de5967688d3dcffdd2270c0fd109', 'hex');

let txs = [];
for (let i = 0; i < 10000; i++) {

    const txParams = {
        nonce: '0x00',
        gasPrice: '0x09184e72a000',
        gasLimit: '0x2710',
        to: '0x0000000000000000000000000000000000000000',
        value: '0x00',
        data: '0x7f7465737432000000000000000000000000000000000000000000000000000000600057',
        // EIP 155 chainId - mainnet: 1, ropsten: 3
        chainId: 3
    };

    const tx = new EthereumTx(txParams);
    tx.sign(privateKey);
    txs.push(tx.serialize())
}

console.log(txs[0].toString());