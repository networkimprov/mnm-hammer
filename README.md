### mnm is not mail

Email allows anyone, anywhere, claiming any identity to send you any content, any number of times.  
_mnm does not._

Email is a cybercrime gateway.<img width="300" align="right" src="https://user-images.githubusercontent.com/458838/65545951-535f6980-decb-11e9-8f46-6122198097b0.png">  
_mnm is not._

mnm is a safer, more productive way to correspond with people you know, 
and make contact with those you don't.

mnm offers features missing from traditional email, including:  
\+ forms/surveys whose results are collected into tables  
\+ charts via [a JS chart library TBD]  
\+ slide deck layouts  
\+ hyperlinks to messages &amp; other threads  
\+ easy addition of recipients to existing threads  
\+ message formatting &amp; layout via Markdown (aka CommonMark)  

mnm is a TMTP client. See also the [mnm relay server](https://github.com/networkimprov/mnm), 
and [Why TMTP?](https://github.com/networkimprov/mnm/blob/master/Rationale.md) 

This version of mnm is a localhost web app, 
i.e. it runs on personal devices and its GUI appears in a browser. 

Complete documentation is provided within the app. 

### Status

_29 May 2019_ -
v0.5 is released. It fixes a panic on Windows, but is otherwise identical to v0.4.

_25 May 2019_ -
v0.4 is released. It has been tested on Windows 7 & MacOS & Linux, but only with Firefox.  
_New:_ Windows support. Fixes a crash-recovery failure, and a TMTP handling error.

_13 May 2019_ -
v0.3 is released. It has only been tested on Linux & MacOS, with Firefox.  
_New:_ tooltips on menu icons. Fixes a panic, and a handful of UI flaws.

_07 May 2019_ -
v0.2 is released. It has only been tested on Linux & MacOS, with Firefox.  
_New:_ a tour with cartoons covering essential features.

_19 April 2019_ -
v0.1 is released. It has only been tested on Linux & MacOS, with Firefox.

### Quick start

If you haven't received an invitation to join a TMTP service, you can set up your own.
See directions to install the server at https://github.com/networkimprov/mnm

1. Download & save latest preview  
|
[**Windows**](https://github.com/networkimprov/mnm-hammer/releases/download/v0.5.0/mnm-app-windows-amd64-v0.5.0.zip)
||
  [**MacOS**](https://github.com/networkimprov/mnm-hammer/releases/download/v0.5.0/mnm-app-macos-v0.5.0.tgz)
||
  [**Linux**](https://github.com/networkimprov/mnm-hammer/releases/download/v0.5.0/mnm-app-linux-amd64-v0.5.0.tgz)
|  
[Release details](https://github.com/networkimprov/mnm-hammer/releases/latest)

   Also install Firefox if you don't have it: https://www.mozilla.org/en-US/firefox/

1. Unpack download  
Windows  
a) Open the browser downloads menu, find "mnm-app-windows-amd64-v0.5.0.zip" and click "Open File".  
b) Drag the item "mnm-hammer-v0.5.0" to the Downloads folder in the left-hand pane.  
c) Open the Windows menu (bottom-left on screen), right-click "Command Prompt", and select "Run as administrator".  
d) You'll see a warning "Do you want to allow the following program..."; click "Yes".  
e) `cd %UserProfile%\Downloads\mnm-hammer-v0.5.0`  
MacOS  
a) Open the browser downloads menu, find "mnm-app-macos-v0.5.0.tgz" and click "Open File".  
b) Open a Terminal window.  
c) `cd ~/Downloads/mnm-hammer-v0.5.0`  
Linux  
a) `cd the_right_directory` # use appropriate name  
b) `tar xzf mnm-app-linux-amd64-v0.5.0.tgz`  
c) `cd mnm-hammer-v0.5.0`

1. Upgrade a prior release, if applicable  
a) Stop the prior app if it's running.  
MacOS  
b) `sudo ditto ../mnm-hammer-vX.Y.Z/store ./store` # use prior version  
Linux  
b) `sudo cp -a ../mnm-hammer-vX.Y.Z/store .` # use prior version

1. Start app  
Windows  
a) `mnm-hammer.exe`  
b) If you see a Windows Firewall warning, choose "Public networks...", then click "Allow access".  
MacOS & Linux  
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
b) `cd $GOPATH/src/github.com/networkimprov/mnm-hammer` # project directory can be moved out of $GOPATH  
c) `./webdeps.sh` # download browser modules  
d) `./pkg.sh` # make release downloads for all platforms

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

