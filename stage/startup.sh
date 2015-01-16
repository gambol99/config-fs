#!/bin/sh
#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-12-23 14:45:43 +0000 (Tue, 23 Dec 2014)
#
#  vim:ts=2:sw=2:et
#

NAME="Configuration FS"
MOUNT=${MOUNT_POINT:-/config}
VERBOSITY=${VERBOSITY:-3}

mkdir -p ${MOUNT}

echo "Starting ${NAME}"
bin/config-fs -logtostderr=true -v=${VERBOSITY} $@
