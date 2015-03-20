#!/bin/bash

NAME="config-fs"
ROOT_KEY="/env/testing"
MOUNT="./mount"
ETCD_VERSION="https://github.com/coreos/etcd/releases/download/v2.0.0/etcd-v2.0.0-linux-amd64.tar.gz"
CONSUL_VERSION="https://dl.bintray.com/mitchellh/consul/0.5.0_linux_amd64.zip"

TMPDIR=$(mktemp -d)
[ -z "$TMPDIR" ] && failed "unable to create a temporary directory"

annonce() {
  [ -n "$1" ] && echo "**** $@"
}

failed() {
  annonce "[failed] $@"
  exit 1
}

download_package() {
  annonce "downloading package: ${1}"
  curl -skL "$1" > "$2" || failed "failed to download the package: ${1}"
}

setup_etcd() {
  src="${TMPDIR}/etcd.tar.gz"
  download_package "${ETCD_VERSION}" "${src}"
  tar zxf "${src}" -C "${TMPDIR}" || failed "failed to extract the etcd software"

  annonce "starting the etcd service"
  ${TMPDIR}/etcd-*-linux-amd64/etcd > /dev/null 2>&1 &
  [ $? -ne 0 ] && failed "unable to start the etcd service"
  sleep 4
}

setup_consul() {
  src="${TMPDIR}/consul.zip"
  download_package "${CONSUL_VERSION}" "${src}"
  $(cd ${TMPDIR}; unzip ${src})

  annonce "starting the consul service"
  ${TMPDIR}/consul agent -server -bootstrap -bind=127.0.0.1 -client=127.0.0.1 -data-dir=/tmp > /dev/null 2>&1 &

  [ $? -ne 0 ] && failed "unable to start the consul service"
  sleep 4
}

setup_deps() {
  apt-get update -y
  apt-get install -y unzip
}

setup() {
  annonce "provisioning the dependencies"
  setup_deps
  annonce "provisioning the etcd service"
  setup_etcd
  annonce "provisioning the consul service"
  setup_consul
}

setup_tests() {
  make test
}

setup && setup_tests
