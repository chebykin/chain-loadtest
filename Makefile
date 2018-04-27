ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

# The commands are aligned according workflow
configs:
	node app.js

instances:
	python3 providers/create.py

fetch:
	python3 providers/fetch_metadata.py

chain:
	ansible-playbook -i hosts.txt chain.yml

bootstrap: configs instances fetch chain

# Agent related
agent-clean:
	rm -f agent/agent

agent-rebuild-image:
	cd agent && docker build --no-cache -t mygolang:1.10 .

agent-linux:
	$ docker run -v $(ROOT_DIR)/agent:/usr/src/agent -w /usr/src/agent \
	 -e GOOS=linux -e GOARCH=amd64 mygolang:1.10 go build -v

agent-deploy:
	ansible-playbook -i hosts.txt agent.yml

agent-update: agent-clean agent-linux agent-deploy

# The rest of commands
tx-queue:
	cd txqueue && node server.js

gcloud-options:
	gcloud compute regions list
	gcloud compute images list
	gcloud compute machine-types list --filter="zone:europe-west3-a"

create-do:
	ansible-playbook createDoNodes.yml
