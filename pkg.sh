#!/bin/bash
#  Copyright 2019 Liam Breck
#  Published at https://github.com/networkimprov/mnm-hammer
#
#  This Source Code Form is subject to the terms of the Mozilla Public
#  License, v. 2.0. If a copy of the MPL was not distributed with this
#  file, You can obtain one at http://mozilla.org/MPL/2.0/

set -e

bins=(
   'GOOS=linux   GOARCH=amd64'
   'GOOS=darwin  GOARCH=amd64'
   'GOOS=windows GOARCH=amd64'
)

go build
app="$(basename "$PWD")"
appdir=mnm-app
files=("$app" App LICENSE formspec test-in.json web/*.* web/img/*.*)
fileswin=("${files[@]}")
fileswin[0]+=.exe
fileswin[1]+=.cmd
ver=($("./$app" --version))
symln="$appdir-${ver[-2]}"

ln -s "$app" "../$symln" || test -L "../$symln"

for pf in "${bins[@]}"; do
   export $pf
   echo -n "--- ${ver[@]} $GOOS-$GOARCH: build"
   go build -a
   echo " & package ---"
   dst="$app/$appdir-$GOOS-$GOARCH-${ver[-2]}"
   if [ $GOOS = windows ]; then
      ls -sd "${fileswin[@]}"
      (cd ..; zip -rq "$dst.zip" "${fileswin[@]/#/$symln/}")
      (cd ..; zip -dq "$dst.zip" '*/web/gui.*')
      rm "$app.exe"
   else
      ls -sd "${files[@]}"
      (cd ..; tar -czf "$dst.tgz" "${files[@]/#/$symln/}")
   fi
done

rm "../$symln"

GOOS='' GOARCH='' go build

