#!/bin/bash

docker run --rm -it -v `pwd`:/home/work -w /home/work  feed-reader:build-nodejs $@
