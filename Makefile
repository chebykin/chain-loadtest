do:
	ansible-playbook createDoNodes.yml

chain:
	ansible-playbook -i hosts.txt chain.yml

generate-keys:
	node app.js

seed-nodes:
	ansible-playbook -i hosts.txt validator.yml

agent-linux:
	GOOS=linux GOARCH=amd64 go build -o agent/agent agent/agent.go

