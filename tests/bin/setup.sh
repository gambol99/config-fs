#!/bin/bash

NAME="config-fs"
ROOT_KEY="/env/testing"
MOUNT="./mount"

annonce() {
  [ -n "$1" ] && echo "** $@"
}

failed() {
  annonce "[failed] $@"
  exit 1
}

check() {
  if [ -n "$1" ]; then
    echo -n "check: $2 "
    eval "$1 >/dev/null"
    if [ $? -ne 0 ]; then
      echo "[failed]"
      exit 1
    fi
    echo "[passed]"
  fi
}

perform_clean() {
  annonce "perform a cleanup"
  make clean
  [ -d "etcd-v2.0.0-linux-amd64" ] && rm -rf etcd-v2.0.0-linux-amd64/
  pkill -9 etcd 2>/dev/null
  pkill -9 config-fs >/dev/null
}

perform_build() {
  export PATH=$PATH:${GOPATH}/bin
  annonce "performing the compilation of ${NAME}"
  go get
  go get github.com/tools/godep
  make || failed "unable to compile the ${NAME} binary"
  gzip stage/config-fs -c > config-fs.gz
}

perform_setup() {
  annonce "downloading the etcd service for tests"
  if [ ! -f "etcd-v2.0.0-linux-amd64.tar.gz" ]; then
    curl -skL https://github.com/coreos/etcd/releases/download/v2.0.0/etcd-v2.0.0-linux-amd64.tar.gz > etcd-v2.0.0-linux-amd64.tar.gz
  fi
  tar zxf etcd-v2.0.0-linux-amd64.tar.gz
  annonce "starting the etcd service"
  nohup etcd-v2.0.0-linux-amd64/etcd > etcd.log 2>&1 &
  [ $? -ne 0 ] && failed "unable to start the etcd service"
  ETCD="etcd-v2.0.0-linux-amd64/etcdctl --peers 127.0.0.1:4001 --no-sync"
  annonce "waiting for etcd to startup"
  sleep 3
  annonce "starting the config-fs service"
  $ETCD set ${ROOT_KEY}/created "`date`"
  mkdir -p mnt/
  nohup stage/config-fs -store=etcd://127.0.0.1:4001 -root=${ROOT_KEY} -mount=${MOUNT} > test.log 2>&1 &
  [ $? -ne 0 ] && failed "unable to start the ${NAME} service"
  sleep 3
}

perform_tests() {
  make test
}

perform_clean
perform_build
perform_setup
perform_tests
