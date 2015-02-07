
#### **Building**

Assuming the following GO environment

    [jest@starfury config-fs]$ set | grep GO
    GOPATH=/home/jest/go
    GOROOT=/usr/lib/golang

    cd $GOPATH
    mkdir -p src/github.com/gambol99
    cd src/github.com/gambol99
    git clone https://github.com/gambol99/config-fs.git
    cd config-fs && make

An alternative would be to build inside a golang container

    cd /tmp
    git clone https://github.com/gambol99/config-fs.git
    cd config-fs
    docker run --rm -v "$PWD":/go/src/github.com/gambol99/config-fs \
      -w /go/src/github.com/gambol99/config-fs -e GOOS=linux golang:1.3.3 make
    stage/config-fs --help
