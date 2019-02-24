#!/bin/bash

set -e

CDN_UIKIT=https://cdnjs.cloudflare.com/ajax/libs/uikit/3.0.0-beta.40
CDN_VUE=https://cdnjs.cloudflare.com/ajax/libs/vue/2.5.22
CDN_MDI=https://cdnjs.cloudflare.com/ajax/libs/markdown-it/8.4.2
CDN_LUX=https://unpkg.com/luxon@1.11.3
CDN_VFG=https://unpkg.com/vue-form-generator@2.3.4

cd web

curl --silent --show-error --location \
     -o uikit-30.min.css      "$CDN_UIKIT/css/uikit.min.css" \
     -o uikit-30.min.js       "$CDN_UIKIT/js/uikit.min.js" \
     -o uikit-icons-30.min.js "$CDN_UIKIT/js/uikit-icons.min.js" \
     -o vue-25.js             "$CDN_VUE/vue.js" \
     -o markdown-it-84.js     "$CDN_MDI/markdown-it.js" \
     -o luxon-111.js          "$CDN_LUX/build/global/luxon.js" \
     -o vue-formgen-23.js     "$CDN_VFG/dist/vfg.js" \
     -o vue-formgen-23.css    "$CDN_VFG/dist/vfg.css"
