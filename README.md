### mnm is not mail[<img width="200" hspace="50" align="right" src="https://user-images.githubusercontent.com/458838/65545951-535f6980-decb-11e9-8f46-6122198097b0.png">](https://mnmnotmail.org)

The mnm project is building a legitimate replacement for email: 
a client (see below), a [server](https://github.com/networkimprov/mnm), and 
a [simple protocol](https://github.com/networkimprov/mnm/blob/master/Protocol.md) between them. 

Learn more at [mnmnotmail.org](https://mnmnotmail.org). 

[**Download the mnm client app**](https://mnmnotmail.org/#quick-start) 


### Status

_09 March 2021_ - the [online demo](https://mnmnotmail.org/demo.html) is released. 
It has been tested on Chrome & Firefox.

[_03 January 2021_ - v0.9](https://github.com/networkimprov/mnm-hammer/releases/latest)
is released. It fixes a panic and a few flaws, but is otherwise identical to v0.8.

_15 November 2020_ - v0.8
is released for Windows 7+ & MacOS & Linux. Its UI has been tested on Chrome & Firefox.  
_New:_ Markdown reference panel, "Todo" tag on new threads, menu of filled-form tables, 
and dozens of other enhancements and fixes.

_20 April 2020_ - v0.7
is released for Windows 7+ & MacOS & Linux. It has been tested with Chrome & Firefox.  
_New:_ slide deck layouts, replication to multiple PCs, simplified startup.

_20 October 2019_ -
v0.6 is released for Windows 7+ & MacOS & Linux. It has been tested with Chrome & Firefox.  
_New:_ search, message tags, file viewer, UI colors, logo, and more. Fixes many UI flaws.

_29 May 2019_ -
v0.5 is released. It fixes a panic on Windows, but is otherwise identical to v0.4.

_25 May 2019_ -
v0.4 is released. It has been tested on Windows 7 & MacOS & Linux, with Firefox.  
_New:_ Windows support. Fixes a crash-recovery failure, and a TMTP handling error.

_13 May 2019_ -
v0.3 is released. It has been tested on Linux & MacOS, with Firefox.  
_New:_ tooltips on menu icons. Fixes a panic, and a handful of UI flaws.

_07 May 2019_ -
v0.2 is released. It has been tested on Linux & MacOS, with Firefox.  
_New:_ a tour with cartoons covering essential features.

_19 April 2019_ -
v0.1 is released. It has been tested on Linux & MacOS, with Firefox.


### Version Numbering

Production releases: 1+ . 0 . 0+

Preview releases: _pp_ . 1+ . _pp_ (first & last from prior production release)

The second number is only used for previews. 
Most (hopefully all) preview features & changes appear in the following production release. 


### Build & Package

Requires Go 1.13.3+

a) `go get github.com/networkimprov/mnm-hammer`  
b) `cd $GOPATH/src/github.com/networkimprov/mnm-hammer` # project directory can be moved out of $GOPATH  
c) `./webdeps.sh` # download browser modules  
d) Edit _kVersionDate_ in main.go  
e) `./pkg.sh` # make release downloads for all platforms

Building for Windows requires patches to the Go source (which do not affect other programs):  
go-winfsd.patch fixes [#32088](https://github.com/golang/go/issues/32088)  
go-winstat.patch fixes [#9611](https://github.com/golang/go/issues/9611)  
Apply patches with: `cp go*.patch /.../go && (cd /.../go && git apply go*.patch)`


### Testing

An automated test sequence is defined in test-in.json. 
It creates accounts Blue and Gold, which then exchange messages. 
It yields occasional false positives due to loose synchronization between the two accounts. 
After a test pass completes, the app provides http on port 8123 (unless --http is given):  
`./mnm-hammer --test server:port` # server:port is a TMTP service  
To access a previous test pass:  
`(cd test-run/TPD/ && ../../mnm-hammer --http :8123)` # TPD is a directory name

Crash testing  
a) `./mnm-hammer --test server:port --crash  init` # make test directory  
b) `./mnm-hammer --test server:port --crash  dir:service:order:op[:sender:order]` # crash here in test sequence  
c) `./mnm-hammer --test server:port --verify dir:service:order:count` # recover and verify result

`./test-crash.sh server:port [ item_index ]` # collection of crash/verify runs in single directory

#### Code Coverage

a) `go test -c -covermode=count -coverpkg ./...`  
b) `go build`  
c) `./mnm-hammer.test --test localhost:443 -test.coverprofile mnm-hammer.cov`  
. . . \# this test pass directory is TPD below  
d) `go tool cover -html=test-run/TPD/mnm-hammer.cov -o web/coverage.html`  
e) `(cd test-run/TPD/ && ../../mnm-hammer --http :8123)`  
f) Open a browser tab, go to `localhost:8123/w/coverage.html`

Ref: https://www.elastic.co/blog/code-coverage-for-your-golang-system-tests


### Demo Files

To generate the demo files:  
a) `cp web/{gui.vue,service-demo.html}`  
b) `cp web/docs{,-demo}.html`  
c) `git apply web/*demo.patch`  

To recreate the `web/...-demo.patch` files after changing the demo files:  
a) `git diff --no-index web/gui.vue web/service-demo.html > web/service-demo.patch`  
b) `git diff --no-index web/docs.html web/docs-demo.html > web/docs-demo.patch`  
c) Edit the patches to use `a/web/...-demo.html` as the origin path.  

To create a JSON object for use in `web/data-demo.js` from an mnm client instance:  
a) Edit `web/gui.vue` to insert `<script src="/w/demodata.js"></script>` after all other `<head>` scripts.  
b) Quit and restart the app, then _Shift-Reload_ the page at `http://localhost:8123`.  
c) Invoke `http://localhost:8123/#demodata` and wait while it steps through each account.  
d) Open the web console and copy the JSON result.  


### License

   Copyright 2018, 2020 Liam Breck  
   Published at https://github.com/networkimprov/mnm-hammer

   This Source Code Form is subject to the terms of the Mozilla Public  
   License, v. 2.0. If a copy of the MPL was not distributed with this  
   file, You can obtain one at http://mozilla.org/MPL/2.0/

