#!/usr/bin/env bash

docker_image_name=dkglib_testnet

cur_path=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )

cd $cur_path
rm -rf ./vendor

gopath=$(whereis go | ggrep -oP '(?<=go: )(\S*)(?= .*)' -m 1)
PATH=$gopath:$gopath/bin:$PATH

echo $GOBIN

echo "--> Ensure dependencies have not been modified"
GO111MODULE=on go mod verify
GO111MODULE=on go mod vendor

GO111MODULE=off

docker build -t $docker_image_name .

chmod 0777 ./go.sum

rm -rf ./vendor