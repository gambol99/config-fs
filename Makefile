#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-12-23 14:44:43 +0000 (Tue, 23 Dec 2014)
#
#  vim:ts=2:sw=2:et
#
NAME="config-fs"
AUTHOR=gambol99
VERSION=0.0.1

build:
	go build -o stage/${NAME}
	docker build -t ${AUTHOR}/${NAME} .

clean:
	rm -f ./stage/${NAME}
	go clean

release:
	rm -rf release
	mkdir release
	GOOS=linux go build -o release/$(NAME)
	cd release && tar -zcf $(NAME)_$(VERSION)_linux_$(HARDWARE).tgz $(NAME)
	rm release/$(NAME)

.PHONY: build


