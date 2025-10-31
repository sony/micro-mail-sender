#!/bin/bash

set -e -x

# check docker
if command -v docker &> /dev/null; then
  sudo docker system prune -f
  exit 0
fi

# install docker
sudo apt install apt-transport-https ca-certificates curl software-properties-common -y
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
yes '' | sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu focal stable"
sudo apt update
apt-cache policy docker-ce
sudo apt install docker-ce -y

# set permission for docker
sudo chmod 666 /var/run/docker.sock
