### mnm is not mail

<img width="300" hspace="32" align="right" src="https://user-images.githubusercontent.com/458838/65545951-535f6980-decb-11e9-8f46-6122198097b0.png">  

To those battling cybercrime, __email is a jungle__. 
It allows anyone, anywhere, claiming any identity to send you any content, any number of times. 

To the rest of us, preoccupied with effective communication and productivity, __email is a desert__.

We've been adopting instant messaging, discussion boards, wikis, and other options for two decades. 
But email remains ubiquitous, because it's __decentralized__ and rests on __open standards__. 

To all who agree that it's time to retire email, the mnm project is building a legitimate replacement: 
a client (see below), a [server](https://github.com/networkimprov/mnm), and 
a [simple protocol](https://github.com/networkimprov/mnm/blob/master/Protocol.md) between them. 

mnm has two major goals.

1. To provide a far safer correspondence model, where you:  
\+ choose the organizations/sites that relay your correspondence  
\+ select which members of a site can correspond with you  
\+ always know from which site a message originated  
\+ can block anyone with whom you've made contact  
\+ may leave a site and never see traffic from it again  
See [_How It Works_](Howitworks.md) for diagrams of the model.

2. To offer capabilities missing in traditional email, including:  
\+ forms/surveys whose results are collected into tables  
\+ data-driven charts via [a JS chart library TBD]  
\+ slide deck layouts  
\+ hashtags and private tags  
\+ hyperlinks to messages &amp; other threads  
\+ message formatting &amp; layout via Markdown (aka CommonMark)  
\+ many more features to foster efficiency, creativity, focus, and understanding  

Further reading: [_Why TMTP?_](https://github.com/networkimprov/mnm/blob/master/Rationale.md) 


### Status

_20 October 2019_ -
v0.6 is released for Windows 7/8/10 & MacOS & Linux. It has been tested with Chrome & Firefox.  
_New:_ search, message tags, file viewer, UI colors, logo, and more. Fixes a panic and many UI flaws.  
[Release details](https://github.com/networkimprov/mnm-hammer/releases/latest)

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


### Quick Start

If you haven't received an invitation to join a TMTP service, you can set up your own.
See the [mnm relay server](https://github.com/networkimprov/mnm).

You'll need Firefox or Chrome. (I endorse [Firefox](https://www.mozilla.org/en-US/firefox/) :-)

#### _Windows_ &bull; [MacOS](#macos) &bull; [Linux](#linux)

1. Download & save latest preview  
[mnm-app-windows-amd64-v0.6.0.zip](https://github.com/networkimprov/mnm-hammer/releases/download/v0.6.0/mnm-app-windows-amd64-v0.6.0.zip)

1. Unpack download  
a) Open the browser downloads menu, find the above file and click "Open File".  
b) Drag the item "mnm-hammer-v0.6.0" to the Downloads folder in the left-hand pane.  
c) Open the Windows menu (bottom-left on screen), right-click "Command Prompt", and select "Run as administrator".  
d) You'll see a warning "Do you want to allow the following program..."; click "Yes".  
e) In the command window, enter `cd %UserProfile%\Downloads\mnm-hammer-v0.6.0`  

1. If upgrading, move the data from a prior release  
a) Stop the prior app if it's running.  
b) Enter `move ..\mnm-hammer-vX.Y.Z\store .\store` &:: &nbsp;_X.Y.Z_ is a prior release, e.g. `0.5.0`  

1. Start app  
Note: the app currently logs much of its traffic with the browser to the command window.  
a) Enter `mnm-hammer.exe`  
b) If you see a Windows Firewall warning, check "Public networks...", then click "Allow access".  

   To stop the app, type `Ctrl-C`  

1. Connect Firefox or Chrome  
Open a browser tab, go to `localhost`.  

#### _MacOS_

1. Download & save latest preview  
[mnm-app-macos-v0.6.0.tgz](https://github.com/networkimprov/mnm-hammer/releases/download/v0.6.0/mnm-app-macos-v0.6.0.tgz)

1. Unpack download  
a) Open the browser downloads menu, find the file above and click "Open File".  
b) In the Finder, open the Go menu and select Utilities.  
c) In the Utilities list, double-click Terminal.  
d) In the terminal window, enter `cd ~/Downloads/mnm-hammer-v0.6.0`  

1. If upgrading, replicate the data from a prior release  
a) Stop the prior app if it's running.  
b) Enter `sudo ditto ../mnm-hammer-vX.Y.Z/store ./store` # _X.Y.Z_ is a prior release, e.g. `0.5.0`  

1. Start app  
Note: the app currently logs much of its traffic with the browser to the terminal window.  
a) Enter `sudo ./mnm-hammer` # starts http on port 80  
or  
a) Enter `./mnm-hammer --http :Port` # _Port_ is a number over 1027  

   To stop the app, type `Ctrl-C`  

1. Connect Firefox or Chrome  
Open a browser tab, go to `localhost` (or `localhost:Port` if specified above).  

#### _Linux_

1. Download & save latest preview  
[mnm-app-linux-amd64-v0.6.0.tgz](https://github.com/networkimprov/mnm-hammer/releases/download/v0.6.0/mnm-app-linux-amd64-v0.6.0.tgz)

1. Unpack download  
a) `cd the_relevant_directory` # use appropriate name  
b) `tar xzf mnm-app-linux-amd64-v0.6.0.tgz`  
c) `cd mnm-hammer-v0.6.0`  

1. If you're upgrading, replicate the data from a prior release  
a) Stop the prior app if it's running.  
b) `sudo cp -a ../mnm-hammer-vX.Y.Z/store .` # _X.Y.Z_ is a prior release, e.g. `0.5.0`  

1. Start app  
Note: the app currently logs much of its traffic with the browser to the terminal window.  
a) `sudo ./mnm-hammer` # starts http on port 80  
or  
a) `./mnm-hammer --http :port`  

   To stop the app, type `Ctrl-C`  

1. Connect Firefox or Chrome  
Open a browser tab, go to `localhost` (or `localhost:port` if specified above).  


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


### License

   Copyright 2018, 2019 Liam Breck  
   Published at https://github.com/networkimprov/mnm-hammer

   This Source Code Form is subject to the terms of the Mozilla Public  
   License, v. 2.0. If a copy of the MPL was not distributed with this  
   file, You can obtain one at http://mozilla.org/MPL/2.0/

