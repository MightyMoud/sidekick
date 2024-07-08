package utils

var SshKeysScript = `
	if [ "$#" -ne 1 ]; then
	    echo "Usage: $0 <hostname>"
	    exit 1
	fi

	HOSTNAME=$1

	ssh-keyscan -H $HOSTNAME >> ~/.ssh/known_hosts
	`

var DockerHandleScript = `
	appName=$1
	dockerUsername=$2
	projectFolder=$3

	docker build --tag $appName --no-cache --platform linux/amd64 $projectFolder

	docker tag $appName $dockerUsername/$appName

	docker push $dockerUsername/$appName
	`

var PreludeScript = `
	if [ "$#" -ne 1 ]; then
	    echo "Usage: $0 <hostname>"
	    exit 1
	fi

	HOSTNAME=$1

	ssh-keyscan -H $HOSTNAME >> ~/.ssh/known_hosts

	mkdir -p $HOME/.config
	mkdir -p $HOME/.config/sidekick

	CONFIG_FILE="$HOME/.config/sidekick/sidekick.yaml"
	if [ ! -e "$CONFIG_FILE" ]; then
	    touch "$CONFIG_FILE"
	fi
	`
