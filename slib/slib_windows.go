// Copyright 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import "syscall"

const kENOTEMPTY = syscall.Errno(145) // missing in syscall

func init() {
   // see README.md for required patch to go/src/syscall/syscall_windows.go
   syscall.Open_FileShareDelete = true
}

// the NTFS journal logs file create, delete, rename
// hopefully that is equivalent to fsync() of a directory in unix

func syncDir(string) error { return nil }
