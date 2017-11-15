// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package slib

import (
   "io"
   "os"
   "sort"
   "time"
)

func initUpload() {
   aFiles, err := readDirNames(kUploadTmp)
   if err != nil { quit(err) }
   for _, aFn := range aFiles {
      err = renameRemove(kUploadTmp + aFn, UploadDir + aFn)
      if err != nil { quit(err) }
   }
}

type tUploadEl struct { Name string; Size int64; Date string }

func GetIdxUpload() []tUploadEl {
   var err error
   aDir, err := readDirNames(UploadDir)
   if err != nil { quit(err) }
   aList := make([]tUploadEl, len(aDir)-1) // omit temp/
   var a int
   for _, aFn := range aDir {
      if aFn == "temp" { continue }
      var aFi os.FileInfo
      aFi, err = os.Lstat(UploadDir + aFn)
      if err != nil && !os.IsNotExist(err) { quit(err) }
      if err == nil {
         aList[a].Size = aFi.Size()
         aList[a].Date = aFi.ModTime().UTC().Format(time.RFC3339)
      } else {
         aList[a].Date = " dropped" // sorts to top
      }
      aList[a].Name = aFn
      a++
   }
   sort.Slice(aList, func(cA, cB int) bool { return aList[cA].Date > aList[cB].Date })
   return aList
}

func AddUpload(iId string, iR io.Reader, iLen int64) error {
   if iId == "" { quit(tError("missing filename")) }
   aOrig := UploadDir + iId
   aTemp := kUploadTmp + iId
   err := os.Symlink("upload_aborted", aOrig)
   if err != nil {
      if !os.IsExist(err) { quit(err) }
   } else {
      err = syncDir(UploadDir)
      if err != nil { quit(err) }
   }
   aFd, err := os.OpenFile(aTemp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aFd.Close()
   _, err = io.CopyN(aFd, iR, iLen)
   if err != nil { return err } //todo only return network errors
   err = aFd.Sync()
   if err != nil { quit(err) }
   err = syncDir(kUploadTmp)
   if err != nil { quit(err) }
   err = os.Remove(aOrig)
   if err != nil { quit(err) }
   err = os.Rename(aTemp, aOrig)
   if err != nil { quit(err) }
   return nil
}

func DropUpload(iId string) bool {
   if iId == "" { quit(tError("missing filename")) }
   err := os.Remove(UploadDir + iId)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   return err == nil
}

func MakeMsgUpload() Msg { return Msg{"op":"upload"} }


