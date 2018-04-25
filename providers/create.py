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
        else:
            raise Exception('Provider not specified for', self.name)

        res = driver.create_node(self.config.name,
                                      self.config.size,
                                      self.config.image,
                                      location=self.config.location,
                                      ex_tags=self.config.tags)
        print(res)


instances = []
size = chainMap['defaults']['size']
location = chainMap['defaults']['location']
image = chainMap['defaults']['image']
provider = chainMap['defaults']['provider']

peer_tags = ['peer-node', 'tn-{:d}'.format(allConfig['timestamp'])]
validator_tags = ['validator-node', 'tn-{:d}'.format(allConfig['timestamp'])]

print(chainMap)

for key, val in chainMap.items():
    if key.startswith('peer-'):
        peer = CreateNodeConfig(
            name=key,
            provider=provider,
            size=size,
            image=image,
            location=location,
            tags=peer_tags
        )

        instances.append(peer)

    elif key.startswith('validator-'):
        validator = CreateNodeConfig(
            name=key,
            provider=provider,
            size=size,
            image=image,
            location=location,
            tags=validator_tags
        )

        instances.append(validator)

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