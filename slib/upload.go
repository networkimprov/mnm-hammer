// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "io"
   "io/ioutil"
   "os"
   "time"
   "net/url"
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
      var aFile string
      aFile, err = url.QueryUnescape(aFi.Name())
      if err != nil { quit(err) }
      aList = append(aList, tUploadEl{Name:aFile, Size:aFi.Size(),
                                      Date:aFi.ModTime().UTC().Format(time.RFC3339)})
   }
   return aList
}

func (tGlobalUpload) GetPath(iId string) string {
   return fileUpload(iId)
}

func (tGlobalUpload) Add(iId, iDup string, iR io.Reader) error {
   if iId == "" {
      return tError("missing filename")
   }
   if iDup != "" && iDup[0] != '.' {
      iDup = "." + iDup
   }
   aOrig := fileUpload(iId + iDup)
   aTemp := fileUptmp(iId + iDup)
   err := os.Symlink("upload_aborted", aOrig)
   if err != nil {
      if !os.IsExist(err) { quit(err) }
   } else {
      err = syncDir(kUploadDir)
      if err != nil { quit(err) }
   }
   if iDup != "" {
      var aDfd *os.File
      aDfd, err = os.Open(fileUpload(iId))
      if err != nil {
         if !os.IsNotExist(err) { quit(err) }
         return err
      }
      defer aDfd.Close()
      iR = aDfd
   }
   err = writeStreamFile(aTemp, iR)
   if err != nil { return err }
   err = syncDir(kUploadTmp)
   if err != nil { quit(err) }
   err = os.Remove(aOrig)
   if err != nil { quit(err) }
   err = os.Rename(aTemp, aOrig)
   if err != nil { quit(err) }
   return nil
}

func (tGlobalUpload) Drop(iId string) error {
   if iId == "" {
      return tError("missing filename")
   }
   err := os.Remove(fileUpload(iId))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   return err
}

