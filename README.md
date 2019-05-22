_Mnm is Not Mail_

mnm provides the benefits of email without the huge risks of allowing 
anyone, anywhere, claiming any identity to send you any content, any number of times. 

mnm also offers electronic correspondence features missing from traditional email, 
including forms/surveys whose results are collected into tables, 
charts via [a JS chart library TBD], 
hyperlinks to messages &amp; other threads, 
slide deck layouts, 
and easy inclusion of additional recipients in existing threads. 
mnm creates HTML-formatted messages via Markdown (aka CommonMark), 
for rapid composition of rich text with graphical elements. 

mnm is an app that securely connects to any number of TMTP relay services via separate accounts, 
and stores all messages locally on your machine. 
TMTP accounts control who can send them correspondence. 
Organizations of any size can run TMTP services on public or private network sites, 
and may limit who participates in them. 
See also [Why TMTP?](https://github.com/networkimprov/mnm/blob/master/Rationale.md) 

This version of mnm is a localhost web app, 
i.e. it runs on personal devices and its UI runs in a browser. 
It's open source, and published [on GitHub](https://github.com/networkimprov/mnm-hammer). 

Complete documentation is provided within the app. 

### Status

_13 May 2019_ -
v0.3 is released. It has only been tested on Linux & MacOS, with Firefox.  
_New:_ tooltips on menu icons. Fixes a crash, and a handful of UI flaws.

_07 May 2019_ -
v0.2 is released. It has only been tested on Linux & MacOS, with Firefox.  
_New:_ a tour with cartoons covering essential features.

_19 April 2019_ -
v0.1 is released. It has only been tested on Linux & MacOS, with Firefox.

### Quick start

If you haven't received an invitation to join a TMTP service, you can set up your own.
See directions to install the server at https://github.com/networkimprov/mnm

1. Download latest preview  
|
[**MacOS**](https://github.com/networkimprov/mnm-hammer/releases/download/v0.3.0/mnm-app-macos-v0.3.0.tgz)
||
[**Linux**](https://github.com/networkimprov/mnm-hammer/releases/download/v0.3.0/mnm-app-linux-amd64-v0.3.0.tgz)
|  
[Release details](https://github.com/networkimprov/mnm-hammer/releases/latest)

1. Unpack download  
MacOS  
a) Open the browser downloads menu, find "mnm-app-macos-v0.3.0.tgz" and click "Open File"  
b) Open a Terminal window  
c) `cd ~/Downloads/mnm-hammer-v0.3.0`  
Linux  
a) `cd the_right_directory` # use appropriate name  
b) `tar xzf mnm-app-linux-amd64-v0.3.0.tgz`  
c) `cd mnm-hammer-v0.3.0`

1. Upgrade a prior release, if applicable  
a) Stop the prior app if it's running.  
MacOS  
b) `sudo ditto ../mnm-hammer-vX.Y.Z/store ./store` # use prior version  
Linux  
b) `sudo cp -a ../mnm-hammer-vX.Y.Z/store .` # use prior version

1. Start app  
a) `sudo ./mnm-hammer` # starts http on port 80  
or  
a) `./mnm-hammer --http [host]:port`  
To stop the app, Ctrl-C  
Note: the preview currently logs much of its traffic with the browser to the terminal window.

1. Connect Firefox  
Open a browser tab, go to `localhost` (or `host:port` if specified above).

### Version Numbering

Production releases: 1+ . 0 . 0+

Preview releases: _pp_ . 1+ . _pp_ (first & last from prior production release)

The second number is only used for previews. 
Most (hopefully all) preview features & changes appear in the following production release. 

### Build & Package

a) `go get github.com/networkimprov/mnm-hammer`  
b) `cd $GOPATH/src/github.com/networkimprov/mnm-hammer`  
c) `./webdeps.sh` # download browser modules  
d) `ln $GOPATH/bin/mnm-hammer mnm-hammer` # hard link necessary for packaging  
e) `./pkg.sh` # make distribution downloads

Building for Windows requires 2 additions to /usr/lib/go/src/syscall/syscall_windows.go:
```
var Open_FileShareDelete = false              // add 1 line
func Open(...) (...) {
	...
	sharemode := ...
	if Open_FileShareDelete {             // add 3 lines
		sharemode |= FILE_SHARE_DELETE
	}
```
See https://github.com/golang/go/issues/32088

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
b) `./mnm-hammer --test server:port --crash  dir:service:orderIdx:op` # crash here in test sequence  
c) `./mnm-hammer --test server:port --verify dir:service:orderIdx:count` # recover and verify result

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

### License

   Copyright 2018, 2019 Liam Breck  
   Published at https://github.com/networkimprov/mnm-hammer

   This Source Code Form is subject to the terms of the Mozilla Public  
   License, v. 2.0. If a copy of the MPL was not distributed with this  
   file, You can obtain one at http://mozilla.org/MPL/2.0/

