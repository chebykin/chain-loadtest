do:
	ansible-playbook createDoNodes.yml

generate-keys:
	node app.js

seed-nodes:
	ansible-playbook -i hosts.txt validator.yml