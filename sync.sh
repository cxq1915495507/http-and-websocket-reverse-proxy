#!/bin/sh
basepath=$(cd `dirname $0`; pwd)
rsync -avrz ${basepath}"/" --delete root@101.132.190.194:/root/go/src/httpDistri
