#!/bin/bash

set -e -x

# check docker-compose
if command -v docker-compose &> /dev/null; then
  exit 0
fi

# install docker-compose
sudo curl -L "https://github.com/docker/compose/releases/download/v2.27.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
docker-compose -v
