#!/bin/bash

set -euxo pipefail

BASE=$(readlink -f $(dirname "$(readlink -f $0)"))

docker build -t opencdr_server ${BASE}/opencdr_server
docker service update --force --update-order start-first opencdr_server
