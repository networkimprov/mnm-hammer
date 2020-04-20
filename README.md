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

[_20 April 2020_ - v0.7](https://github.com/networkimprov/mnm-hammer/releases/latest)
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


### Quick Start

You'll need Firefox or Chrome. (I endorse [Firefox](https://www.mozilla.org/en-US/firefox/) :-)

You'll need an invitation to a TMTP service. 
To run your own service, see the [mnm TMTP server](https://github.com/networkimprov/mnm).

#### _Windows_ &bull; [MacOS](#macos) &bull; [Linux](#linux)

1. Download & save  
a) Click [mnm-app-windows-amd64-v0.7.0.zip](https://github.com/networkimprov/mnm-hammer/releases/download/v0.7.0/mnm-app-windows-amd64-v0.7.0.zip).  
b) Open the browser downloads menu, find the above file and click "Open File".  
c) Drag the item `mnm-app-v0.7.0` to the `Downloads` folder in the left-hand pane.

1. If a previous version is running  
a) Go to its log window and press _Ctrl-C_ and then _Y_ to stop it.

1. Start app  
a) Open the `mnm-app-v0.7.0` folder now in `Downloads`, and double-click `App.cmd`.  
b) You'll see a notice, "The publisher could not be verified..." Click "Run".  
c) You'll see a system notice, "Do you want to allow ...?" Click "Yes".  
+&nbsp; You'll see the mnm log window.  
+&nbsp; If you have a previous version in `Downloads`, it will offer to update it.  
+&nbsp; If the app fails, it will offer to restart it.  
+&nbsp; To stop the app, press _Ctrl-C_ and then _Y_ (closes the window).

1. Connect Firefox or Chrome  
a) Right-click [localhost:8123](http://localhost:8123/), and select "Open link in new tab".  
+&nbsp; You'll see the landing page, with a tour.

#### _MacOS_

1. Download & save  
a) Click [mnm-app-macos-v0.7.0.tgz](https://github.com/networkimprov/mnm-hammer/releases/download/v0.7.0/mnm-app-macos-v0.7.0.tgz).  
b) Open the browser downloads menu, find the file above and click "Open File".

1. If a previous version is running  
a) Go to its log window and press _Ctrl-C_ to stop it, then close the window.

1. Start app  
a) Open the `mnm-app-v0.7.0` folder now in `Downloads`, Ctrl-click on `App`, and select "Open".  
b) You'll see a notice, "_App_ is from an unidentified developer..." Click "Open".  
+&nbsp; You'll see the mnm log window.  
+&nbsp; If you have a previous version in `Downloads`, it will offer to update it.  
+&nbsp; If the app fails, it will offer to restart it.  
+&nbsp; To stop the app, press _Ctrl-C_, then close the window.

1. Connect Firefox or Chrome  
a) Ctrl-click (or two-finger tap) [localhost:8123](http://localhost:8123/), and select "Open link in new tab".  
+&nbsp; You'll see the landing page, with a tour.

#### _Linux_

1. Download & save  
a) Click [mnm-app-linux-amd64-v0.7.0.tgz](https://github.com/networkimprov/mnm-hammer/releases/download/v0.7.0/mnm-app-linux-amd64-v0.7.0.tgz).  
b) Extract the downloaded file, e.g. `tar xzf mnm-app-linux-amd64-v0.7.0.tgz`

1. If a previous version is running  
a) Go to its log window and press _Ctrl-C_ to stop it.

1. Start app  
a) Open the extracted `mnm-app-v0.7.0` folder, and double-click `App`.  
+&nbsp; You'll see the mnm log window.  
+&nbsp; If you have a previous version in the parent folder, it will offer to update it.  
+&nbsp; If the app fails, it will offer to restart it.  
+&nbsp; To stop the app, press _Ctrl-C_ (closes the window).

1. Connect Firefox or Chrome  
a) Right-click [localhost:8123](http://localhost:8123/), and select "Open link in new tab".  
+&nbsp; You'll see the landing page, with a tour.


### Installation Notes

On Windows, the app needs Administrator privileges to create symlinks and configure the firewall. 
`App.cmd` creates the firewall configuration. To inspect it, run:  
`netsh advfirewall firewall show rule name=mnm-hammer verbose`

To start the app using a different TCP port, run:  
MacOS & Linux (as any user): `./mnm-hammer -http :8123`  
Windows (as administrator): `mnm-hammer.exe -http :8123`

Updating from a previous version moves the folder `mnm-app-v0.X.0/store` to the new version, 
and leaves the previous version otherwise untouched. 
Launching `App.cmd` or `App` in the previous version will offer to update to it, 
moving the `store` folder back again (not recommended).


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

   Copyright 2018, 2020 Liam Breck  
   Published at https://github.com/networkimprov/mnm-hammer

   This Source Code Form is subject to the terms of the Mozilla Public  
   License, v. 2.0. If a copy of the MPL was not distributed with this  
   file, You can obtain one at http://mozilla.org/MPL/2.0/

