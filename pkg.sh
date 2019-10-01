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

app="$(basename "$PWD")"
files=("$app" LICENSE formspec test-in.json web/*.* web/img/*.*)

go build
ver=($("./$app" --version))

echo "packaging ${ver[@]} with:"
ls -sd "${files[@]}"

ln -s "$app" "../$app-${ver[-2]}" || test -L "../$app-${ver[-2]}"
files=($(printf "$app-${ver[-2]}/%s " "${files[@]}"))
fileswin=("${files[@]}")
fileswin[0]+=.exe

for pf in "${bins[@]}"; do
   export $pf
   go build
   echo -n "$GOOS-$GOARCH built "
   dst="$app/mnm-app-$GOOS-$GOARCH-${ver[-2]}"
   if [ $GOOS = windows ]; then
      (cd ..; zip -rq "$dst.zip" "${fileswin[@]}")
      zip -dq ../"$dst.zip" '*/web/gui.*'
      rm "$app.exe"
   else
      (cd ..; tar -czf "$dst.tgz" "${files[@]}")
   fi
   echo "& packaged"
done

rm "../$app-${ver[-2]}"

GOOS='' GOARCH='' go build

