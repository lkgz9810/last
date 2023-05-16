#!/bin/bash

echo "This will cause some downtime, are you sure you want to continue? Press enter to continue or CTRL+C to cancel"
read ACCEPT

set -euxo pipefail

BASE=$(readlink -f $(dirname "$(readlink -f $0)"))
mkdir -p /cs/data/opencdr

apt  install -y libxml2-utils

docker build -t opencdr_server ${BASE}/opencdr_server
docker stack rm opencdr  || true
while docker network inspect opencdr_default >/dev/null 2>&1 ; do sleep 1; done
docker stack deploy --compose-file ${BASE}/docker-compose.yml opencdr
${BASE}/wait-for-it.sh localhost:16384 -t 90
