// Copyright 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

// +build linux windows

package slib

import "os"

func readDirNames(iPath string) ([]string, error) {
   aFd, err := os.Open(iPath)
   if err != nil { return nil, err }
   defer aFd.Close()
   return aFd.Readdirnames(0)
}

func readDirFis(iPath string) ([]os.FileInfo, error) {
   aFd, err := os.Open(iPath)
   if err != nil { return nil, err }
   defer aFd.Close()
   return aFd.Readdir(0)
}
