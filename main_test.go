// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package main

import (
   "fmt"
   "testing"
)

func TestCoverage(i *testing.T) {
   fmt.Printf("code coverage for v%d.%d.%d %s\n", kVersionA, kVersionB, kVersionC, kVersionDate)
   sTestExit = true
   if mainResult() != 0 {
      i.Fail()
   }
}

