#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-12-23 14:44:43 +0000 (Tue, 23 Dec 2014)
#
#  vim:ts=2:sw=2:et
#
NAME=config-fs
AUTHOR=gambol99
VERSION=$(shell awk '/const Version/ { print $$4 }' version.go | sed 's/"//g')

build:
	go get github.com/tools/godep
	godep go build -o stage/${NAME}

docker: build
	sudo docker build -t ${AUTHOR}/${NAME} .

clean:
	rm -f ./stage/${NAME}
	go clean

test: build
	go get github.com/stretchr/testify

changelog:
	git log $(shell git tag | tail -n1)..HEAD --no-merges --format=%B > changelog

all: clean changelog build docker

release:
	rm -rf release
	mkdir release
	GOOS=linux godep go build -o release/$(NAME)
	cd release && tar -zcf $(NAME)_$(VERSION)_linux_$(HARDWARE).tgz $(NAME)
	go chanagelog
	cp changelog release/changelog
	rm release/$(NAME)

.PHONY: build release changelog
