ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

play-do:
	ansible-playbook createDoNodes.yml

play-chain:
	ansible-playbook -i hosts.txt chain.yml

play-checks:
	ansible-playbook -i hosts.txt tx-checks.yml

generate-keys:
	node app.js

seed-nodes:
	ansible-playbook -i hosts.txt validator.yml

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

tx-queue:
	cd txqueue && node server.js
