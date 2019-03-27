_Mnm is Not Mail_

mnm provides the benefits of email without the huge risks of allowing 
anyone, anywhere, claiming any identity to send you any content, any number of times. 

mnm also offers electronic correspondence features missing from traditional email, 
including forms/surveys which may be filled out and returned, 
charts via [Chart.js or Vega-Lite], hyperlinks to messages, and slide shows. 
It creates HTML-formatted messages via Markdown, which enables 
mouseless (i.e. rapid) composition of rich text with graphical elements. 

mnm is a client application that securely connects to 
any number of TMTP relay services via separate accounts. 
TMTP service accounts control who can send them correspondence. 
Organizations of any size can run TMTP services on public or private network sites, 
and may limit who participates in them. 
See also [Why TMTP?](https://github.com/networkimprov/mnm/blob/master/Rationale.md) 

This version of mnm is a localhost web app (i.e. it runs on personal devices and its UI runs 
in a browser). 
It's written in Go &amp; Vue.js. 
The source is [on GitHub](https://github.com/networkimprov/mnm-hammer). 

### Status

A first release is in progress.

### Quick start

1. Follow steps to install & start TCP server at https://github.com/networkimprov/mnm

1. Build client  
a) go get github.com/networkimprov/mnm-hammer  
b) cd $GOPATH/src/github.com/networkimprov/mnm-hammer  
c) ./webdeps.sh # download browser modules  
d) go build mnm-hammer

1. Start client  
a) ./mnm-hammer --test server_host:port [--http [host]:port] # http default ":80" may require sudo  
b) ctrl-C to stop client

1. Point FireFox to http://localhost/Blue # not yet tested in other browsers

1. See docs in the &#9432; menu

### License

   Copyright 2018, 2019 Liam Breck  
   Published at https://github.com/networkimprov/mnm-hammer

   This Source Code Form is subject to the terms of the Mozilla Public  
   License, v. 2.0. If a copy of the MPL was not distributed with this  
   file, You can obtain one at http://mozilla.org/MPL/2.0/

