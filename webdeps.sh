#!/bin/bash
#  Copyright 2018, 2019 Liam Breck
#  Published at https://github.com/networkimprov/mnm-hammer
#
#  This Source Code Form is subject to the terms of the Mozilla Public
#  License, v. 2.0. If a copy of the MPL was not distributed with this
#  file, You can obtain one at http://mozilla.org/MPL/2.0/

set -e

MNM_BIN=https://github.com/networkimprov/mnm-hammer/releases/download/v0.6.0
CDN_UIK=https://cdnjs.cloudflare.com/ajax/libs/uikit/3.0.3
CDN_VUE=https://cdnjs.cloudflare.com/ajax/libs/vue/2.6.11
CDN_MDI=https://cdnjs.cloudflare.com/ajax/libs/markdown-it/11.0.0
CDN_LUX=https://unpkg.com/luxon@1.24.1
CDN_VFG=https://unpkg.com/vue-form-generator@2.3.4

cd web

curl --silent --show-error --location \
     -o uikit-30.min.css      "$CDN_UIK/css/uikit.min.css" \
     -o uikit-30.min.js       "$CDN_UIK/js/uikit.min.js" \
     -o uikit-icons-30.min.js "$CDN_UIK/js/uikit-icons.min.js" \
     -o vue-26.js             "$CDN_VUE/vue.js" \
     -o markdown-it-11x.js    "$CDN_MDI/markdown-it.js" \
     -o luxon-1x.js           "$CDN_LUX/build/global/luxon.js" \
     -o vue-formgen-23.js     "$CDN_VFG/dist/vfg.js" \
     -o vue-formgen-23.css    "$CDN_VFG/dist/vfg.css"

curl --silent --show-error --location "$MNM_BIN/mnm-webimg.tar" | tar x
