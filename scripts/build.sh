#!/bin/bash
go clean -x -cache -testcache
go build -x -v -o vginstance ../vguardbft
echo $?
./vginstance -h

