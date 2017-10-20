## mnm-hammer

_Mnm is Not Mail_

You choose the websites you visit; now choose who can send you mail.
See [Why mnm?](https://github.com/networkimprov/mnm/blob/master/Rationale.md)

mnm-hammer is a rich correspondence client for TMTP networks.  
For the mnm message-relay server, see https://github.com/networkimprov/mnm

### Status

A first release is in progress.

### Quick start

1. go get github.com/networkimprov/mnm

1. go get github.com/networkimprov/mnm-hammer

1. cd $GOPATH/src/github.com/networkimprov/mnm

1. go build mnm

1. cp mnm.conf mnm.config

1. ./mnm # starts server on localhost:8888

1. cd $GOPATH/src/github.com/networkimprov/mnm-hammer

1. go build mnm-hammer

1. ./mnm-hammer # starts server on localhost:80

1. browse to http://localhost/test
