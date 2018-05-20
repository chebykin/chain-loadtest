from libcloud.compute.types import Provider
from libcloud.compute.providers import get_driver
from dotenv import load_dotenv

import threading
import yaml
import pprint
import os

load_dotenv(dotenv_path='./.env')

pp = pprint.PrettyPrinter(indent=4)

compute_engine = get_driver(Provider.GCE)

with open("./group_vars/all.yml".format(), 'r') as allConfig:
    try:
        allConfig = yaml.load(allConfig)
    except yaml.YAMLError as exc:
        print(exc)
        exit(1)

with open("./maps/{:s}.yml".format(allConfig['map']), 'r') as chainMap:
    try:
        chainMap = yaml.load(chainMap)
    except yaml.YAMLError as exc:
        print(exc)
        exit(1)


class CreateNodeConfig():
    def __init__(self, name, provider, size, image, location, tags):
        self.name = name
        self.provider = provider
        self.size = size
        self.image = image
        self.location = location
        self.tags = tags


class CreateNode(threading.Thread):
    def __init__(self, config):
        assert isinstance(config, CreateNodeConfig)
        threading.Thread.__init__(self)
        self.config = config

    def run(self):
        print("creating... " + self.config.name)

        if self.config.provider == 'gcloud':

            compute_engine = get_driver(Provider.GCE)
            driver = compute_engine(os.getenv('GCE_SERVICE_ACCOUNT'),
                                    os.getenv('GCE_KEY_PATH'),
                                    project=os.getenv('GCE_PROJECT_ID'))
            res = driver.create_node(self.config.name,
                                     self.config.size,
                                     self.config.image,
                                     location=self.config.location,
                                     ex_tags=self.config.tags)
        elif self.config.provider == 'do':
            cls = get_driver(Provider.DIGITAL_OCEAN)

            driver = cls(os.getenv('DO_API_TOKEN'), api_version='v2')

            print(chainMap['defaults']['do']['key_id'])
            res = driver.create_node(name=self.config.name,
                                     size=type('obj', (object,), {'name': self.config.size}),
                                     image=type('obj', (object,), {'id': self.config.image}),
                                     location=type('obj', (object,), {'id': self.config.location}),
                                     ex_create_attr={
                                         'tags': self.config.tags,
                                         'ssh_keys': [int(chainMap['defaults']['do']['key_id'])]
                                     })
        else:
            raise Exception('Provider not specified for ' + self.config.name)

        print(res)


instances = []

provider = chainMap['defaults']['provider']

defaults = chainMap['defaults']
image = defaults[provider]['image']
size = defaults[provider]['size']

peer_size = defaults[provider]['size']
peer_location = defaults[provider]['location']

validator_size = defaults[provider]['size']
validator_location = defaults[provider]['location']

observer_size = defaults[provider]['size']
observer_location = defaults[provider]['location']

if 'peers' in chainMap['defaults']:
    peer_defaults = chainMap['defaults']['peers']
    if 'size' in peer_defaults:
        peer_size = peer_defaults['size']
    if 'location' in peer_defaults:
        peer_location = peer_defaults['location']

if 'validators' in chainMap['defaults']:
    validator_defaults = chainMap['defaults']['validators']
    if 'size' in validator_defaults:
        validator_size = validator_defaults['size']
    if 'location' in validator_defaults:
        validator_location = validator_defaults['location']

if 'observers' in chainMap['defaults']:
    observer_defaults = chainMap['defaults']['observers']
    if 'size' in observer_defaults:
        observer_size = observer_defaults['size']
    if 'location' in observer_defaults:
        observer_location = observer_defaults['location']

peer_tags = ['peer-node', 'tn-{:d}'.format(allConfig['timestamp'])]
validator_tags = ['validator-node', 'tn-{:d}'.format(allConfig['timestamp'])]
observer_tags = ['observer-node', 'tn-{:d}'.format(allConfig['timestamp'])]

print(chainMap)

for key, val in chainMap.items():
    if key.startswith('peer-'):
        tags = peer_tags

        if val is None:
            val = {}

        size = val['size'] if 'size' in val else peer_size
        location = val['location'] if 'location' in val else peer_location
    elif key.startswith('validator-'):
        tags = validator_tags

        if val is None:
            val = {}

        size = val['size'] if 'size' in val else validator_size
        location = val['location'] if 'location' in val else validator_location
    elif key.startswith('observer-'):
        tags = observer_tags

        if val is None:
            val = {}

        size = val['size'] if 'size' in val else observer_size
        location = val['location'] if 'location' in val else observer_location
    else:
        continue

    instance = CreateNodeConfig(
        name=key,
        provider=provider,
        size=size,
        image=image,
        location=location,
        tags=tags
    )

    instances.append(instance)

pp.pprint(instances)

jobs = []
for v in instances:
    jobs.append(CreateNode(v))

for j in jobs:
    print("starting job...", j.name)
    j.start()

for j in jobs:
    print("joining job...", j.name)
    j.join()

print("done")
