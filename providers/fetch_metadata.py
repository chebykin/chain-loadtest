import yaml
import json
import os

from dotenv import load_dotenv
from enum import Enum
from libcloud.compute.types import Provider
from libcloud.compute.providers import get_driver

load_dotenv(dotenv_path='./.env')

ComputeEngine = get_driver(Provider.GCE)
# Note that the 'PEM file' argument can either be the JSON format or
# the P12 format.
driver = ComputeEngine(os.getenv('GCE_SERVICE_ACCOUNT'),
                       os.getenv('GCE_KEY_PATH'),
                       project=os.getenv('GCE_PROJECT_ID'))

peers = {}
validators = {}
observers = {}


class NodeType(Enum):
    OBSERVER = 1
    VALIDATOR = 2
    PEER = 3


map = {
    'validators': {},
    'peers': {},
    'observers': {}
}

nodes = driver.list_nodes()


def provider_name(original):
    if original == 'gce':
        return 'gcloud'


def cook_value(fetched_v, node_type):
    addresses = get_addresses()

    map_v = {
        'ip': fetched_v.public_ips[0],
        'name': fetched_v.name,
        'provider': provider_name(fetched_v.driver.type),
        'region': fetched_v.extra['zone'].name,
        'image': fetched_v.image,
        'size': fetched_v.size
    }

    if node_type is NodeType.VALIDATOR:
        map_v['engine_signer_address'] = addresses[fetched_v.name]
    elif node_type is NodeType.PEER:
        map_v['agent_address'] = addresses[fetched_v.name]

    return map_v


# fetch address from validator/peer json keys
def get_addresses():
    a = {}

    for e in os.listdir("./tmp/latest/"):
        if e.startswith("peer-") or e.startswith('validator-'):
            with open("./tmp/latest/" + e, 'r') as file:
                key = json.load(file)
                a[e.split('.')[0]] = key['address']
    return a


for node in nodes:
    if node.name.startswith('peer'):
        peers[node.name] = node
    elif node.name.startswith('validator'):
        validators[node.name] = node
    elif node.name.startswith('observer'):
        observers[node.name] = node

# writing hosts.txt
with open('hosts.txt', 'w') as hosts:
    hosts.write('[observer]\n')
    for observer in observers.values():
        hosts.write('{:s} name={:s} type=observer\n'
                    .format(observer.public_ips[0], observer.name))
    hosts.write('\n')

    hosts.write('[peer]\n')
    for peer in peers.values():
        hosts.write('{:s} name={:s} type=peer\n'
                    .format(peer.public_ips[0], peer.name))
    hosts.write('\n')

    hosts.write('[validator]\n')
    for validator in validators.values():
        hosts.write('{:s} name={:s} type=validator\n'
                    .format(validator.public_ips[0], validator.name))
    hosts.write('\n')


# writing map.json
for fetched_v in validators.values():
    map['validators'][fetched_v.name] = cook_value(fetched_v, NodeType.VALIDATOR)

for fetched_v in peers.values():
    map['peers'][fetched_v.name] = cook_value(fetched_v, NodeType.PEER)

for fetched_v in observers.values():
    map['observers'][fetched_v.name] = cook_value(fetched_v, NodeType.OBSERVER)

with open('tmp/latest/map.yml', 'w') as outfile:
    yaml.dump(map, outfile, default_flow_style=False)
