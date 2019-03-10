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
   "io/ioutil"
   "os"
   "strings"
   "time"
)

type tGlobalUpload struct{} // implements GlobalSet
var Upload tGlobalUpload

func initUpload() {
   aFiles, err := readDirNames(kUploadTmp)
   if err != nil { quit(err) }
   for _, aFn := range aFiles {
      err = renameRemove(kUploadTmp + aFn, kUploadDir + aFn)
      if err != nil { quit(err) }
   }
}

type tUploadEl struct {
   Name string
   Size int64
   Date string
}

func (tGlobalUpload) GetIdx() interface{} {
   aDir, err := ioutil.ReadDir(kUploadDir)
   if err != nil { quit(err) }
   aList := make([]tUploadEl, 0, len(aDir)-1) // omit temp/
   for _, aFi := range aDir {
      if aFi.Name() == "temp" { continue }
      aList = append(aList, tUploadEl{Name:aFi.Name(), Size:aFi.Size(),
                                      Date:aFi.ModTime().UTC().Format(time.RFC3339)})
   }
   return aList
}

func (tGlobalUpload) GetPath(iId string) string {
   return kUploadDir + iId
}

func (tGlobalUpload) Add(iId, iDup string, iR io.Reader) error {
   if iId == "" || strings.ContainsRune(iId, '/') {
      return tError("missing or invalid filename")
   }
   aOrig := kUploadDir + iId
   aTemp := kUploadTmp + iId
   if iDup != "" {
      if strings.ContainsRune(iDup, '/') {
         return tError("invalid dup revname")
      }
      aOrig += "." + iDup
      aTemp += "." + iDup
   }
   err := os.Symlink("upload_aborted", aOrig)
   if err != nil {
      if !os.IsExist(err) { quit(err) }
   } else {
      err = syncDir(kUploadDir)
      if err != nil { quit(err) }
   }
   if iDup != "" {
      var aDfd *os.File
      aDfd, err = os.Open(kUploadDir + iId)
      if err != nil {
         if !os.IsNotExist(err) { quit(err) }
         return err
      }
      defer aDfd.Close()
      iR = aDfd
   }
   aFd, err := os.OpenFile(aTemp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aFd.Close()
   _, err = io.Copy(aFd, iR)
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

func (tGlobalUpload) Drop(iId string) error {
   if iId == "" || strings.ContainsRune(iId, '/') {
      return tError("missing or invalid filename")
   }
   err := os.Remove(kUploadDir + iId)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   return err
}

