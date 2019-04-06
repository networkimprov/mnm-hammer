// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package main

import (
   "fmt"
   "testing"
)

func TestCoverage(i *testing.T) {
   if sTestCrash == "" && sTestVerify == "" {
      fmt.Printf("code coverage for v%d.%d.%d %s\n", kVersionA, kVersionB, kVersionC, kVersionDate)
   }
   sTestExit = true
   if mainResult() != 0 {
      i.Fail()
   }
}

