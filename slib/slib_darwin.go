// Copyright 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import "os"

func readDirNames(iPath string) ([]string, error) {
   aFd, err := os.Open(iPath)
   if err != nil { return nil, err }
   defer aFd.Close()
   aList, err := aFd.Readdirnames(0)
   for a := range aList {
      if aList[a] != ".DS_Store" { continue }
      aList = aList[:a + copy(aList[a:], aList[a+1:])]
      break
   }
   return aList, err
}

func readDirFis(iPath string) ([]os.FileInfo, error) {
   aFd, err := os.Open(iPath)
   if err != nil { return nil, err }
   defer aFd.Close()
   aList, err := aFd.Readdir(0)
   for a := range aList {
      if aList[a].Name() != ".DS_Store" { continue }
      aList = aList[:a + copy(aList[a:], aList[a+1:])]
      break
   }
   return aList, err
}
