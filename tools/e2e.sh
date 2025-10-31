#!/bin/bash

set -e -x

export PATH="$HOME/.goenv/shims:$PATH"

is_app_ready() {
  {
    curl -s http://localhost:8333 >&2
  } || {
    echo "ng"
  }
  echo "ok"
}

docker-compose build
docker-compose down

# make sure ports are available

ports=( "5432" "8333" "8025" )

for port in "${ports[@]}"
do
  check=`sudo netstat -ntlp | grep LISTEN | grep "$port" || true`
  if ! [[ $check = "" ]]; then
    echo $check
    exit 1
  fi
done

docker-compose up -d

# wait till app is ready
limit=30
counter=0
while : ; do
  if [[ $limit == $counter ]]; then
    echo "app is not ready."
    exit 1
  fi
  counter=$((counter+1))
  res=$(is_app_ready)
  if [[ "$res" == "ok" ]]; then
    echo "api is ready"
    break
  else
    echo "waiting...${counter}"
    sleep 5
  fi
done

make e2etest
