#!/bin/bash

NAME="config-fs"

say() {
    [ -n "$1" ] && echo "** $1"
}

failed() {
    say "Failed: $@"
    exit 1
}

perform_setup() {
    say "Downloading Etcd Service"
    curl -ksL https://github.com/coreos/etcd/releases/download/v2.0.0/etcd-v2.0.0-linux-amd64.tar.gz > etcd.tar.gz
    tar zxf etcd.tar.gz

    say "Starting Etcd on 127.0.0.1:4001"
    nohup etcd-v2.0.0-linux-amd64/etcd > etcd.log 2>&1 > /dev/null &
    if [ $? -ne 0 ]; then
        failed "unable to start the etcd service"
    fi

    say "Waiting for Etcd to start up"
    sleep 3
}

perform_build() {
    say "Building the ${NAME} binary"
    make || failed "unable to compile the project"
}

perform_tests() {
    say "Starting the tests"
}

perform_build
perform_setup
perform_tests