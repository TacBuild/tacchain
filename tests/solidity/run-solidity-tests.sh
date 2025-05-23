#!/bin/bash -e

# set current directory to the script directory
cd "$(dirname "$0")" || exit


# remove existing data
rm -rf .test-solidity

if command -v yarn &>/dev/null; then
	yarn install
else
	curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | sudo apt-key add -
	echo "deb https://dl.yarnpkg.com/debian/ stable main" | sudo tee /etc/apt/sources.list.d/yarn.list
	sudo apt update && sudo apt install yarn
	yarn install
fi

yarn test --network cosmos "$@"
