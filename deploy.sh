#!/usr/bin/sh
echo "Start deploy"
cd ~/circleci-aws || exit
git pull
go build -o main
./main
echo "Deploy end"