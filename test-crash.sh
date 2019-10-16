#!/bin/bash
#  Copyright 2019 Liam Breck
#  Published at https://github.com/networkimprov/mnm-hammer
#
#  This Source Code Form is subject to the terms of the Mozilla Public
#  License, v. 2.0. If a copy of the MPL was not distributed with this
#  file, You can obtain one at http://mozilla.org/MPL/2.0/

set -e

if [ $# -lt 1 -o $# -gt 2 ]; then
   echo "usage: $0 tmtp_host:port [ item_index ]"
   exit 1
fi

host="$1"

# SvcId Orders[n].Name Orders-count transaction-name
list=(
   'Blue thread_save.a    1 store-draft-thread'
   'Blue thread_discard.a 1 delete-draft-thread'
   'Blue poll_ack.a       1 store-sent-thread         Blue thread_send.a'
   'Blue poll_delivery.a  2 store-fwd-received-thread Gold forward_send.a'
   'Blue poll_delivery.a  2 store-fwd-notify-thread   Gold forward_send.a'
   'Blue poll_delivery.a  2 store-confirm-thread      Gold forward_send.a'
   'Blue thread_open.a    1 touch-thread'
   'Gold poll_delivery.b  1 store-received-thread     Blue thread_send.a'
   'Gold forward_send.a   1 store-fwd-sent-thread'
)

if [ $# -eq 2 ]; then
   test "$2" -eq "$2"
   list=( "${list[$2]}" )
fi

dir=$(./mnm-hammer -test "$host" -crash init)

echo -e "crash testing in $dir \n"

for rec in "${list[@]}"; do
   set $rec
   ./mnm-hammer -test "$host" -crash  "$dir:$1:$2:$4:$5:$6"
   ./mnm-hammer -test "$host" -verify "$dir:$1:$2:$3" || test $? -ne 33 # tolerate crash
   echo
done

echo "crash testing complete"
