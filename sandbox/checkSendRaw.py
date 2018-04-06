# https://github.com/ethereum/web3.py/blob/master/docs/web3.eth.account.rst
import json
import threading
from web3 import Web3, HTTPProvider

w3 = Web3(HTTPProvider('http://127.0.0.1:8545'))
w3.eth.enable_unaudited_features()

keyfile = open('validator-0.json')
chainMap = json.load(open('../map.json'))
encrypted_key = keyfile.read()
private_key = w3.eth.account.decrypt(encrypted_key, 'vpeers/validator-0')
print(chainMap)

class SendHelper(threading.Thread):
    def __init__(self, txs):
        threading.Thread.__init__(self)
        self.txs = txs

    def run(self):
        w3 = Web3(HTTPProvider('http://127.0.0.1:8545'))
        for t in self.txs:
            w3.eth.sendRawTransaction(t)

def eth_sendRawTransaction():
    # TODO: get nonce
    txs = []
    # TODO: build txs list
    nonce = w3.eth.getTransactionCount(w3.toChecksumAddress(chainMap['peers'][1]))
    # nonce = w3.eth.getTransactionCount(chainMap['peers'][1])
    print("got nonce", nonce, "for", w3.toChecksumAddress(chainMap['peers'][1]))

    for i in range(0, 1200):
        tx = {
            'from': chainMap['peers'][0],
            'to': chainMap['peers'][1],
            'value': 100000000000000000,
            'gas': 21000,
            'gasPrice': 1000000000,
            'nonce': nonce,
            'chainId': 15054
        }
        nonce += 1
        signed = w3.eth.account.signTransaction(tx, private_key)
        txs.append(signed.rawTransaction)

    for t in txs:
        w3.eth.sendRawTransaction(t)


eth_sendRawTransaction()
