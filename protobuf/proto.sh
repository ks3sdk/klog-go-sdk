#!/bin/bash

filelist=`ls`
current_path=$(pwd)
parent_path=${current_path%/*%/*%/*%/*}
export GOPATH=${parent_path%/*}
echo $GOPATH

for filename in $filelist
do
    if [ "${filename##*.}" = "proto" ]; then
        `protoc --proto_path=${current_path} --go_out=plugins=grpc:. *.proto`
    fi
done