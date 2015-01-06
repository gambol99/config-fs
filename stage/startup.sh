#!/bin/sh
#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-12-23 14:45:43 +0000 (Tue, 23 Dec 2014)
#
#  vim:ts=2:sw=2:et
#

NAME="Configuration FS"
MOUNT_POINT=${MOUNT_POINT:-/config}
CONFIG_URL=${CONFIG_URL:-etcd://localhost:4001}
DISCOVERY_URL=${DISCOVERY_URL:-consul://localhost:8500}
VERBOSITY=${VERBOSITY:-3}

mkdir -p ${MOUNT_POINT}

echo "Starting ${NAME}, store: ${CONFIG_URL}, discovery: ${DISCOVERY_URL}"
bin/config-fs -logtostderr=true -v=${VERBOSITY} -store=${CONFIG_URL} -discovery ${DISCOVERY_URL}
