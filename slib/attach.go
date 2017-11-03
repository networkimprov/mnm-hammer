// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package slib

import (
   "fmt"
   "io"
   "net"
   "os"
   "path"
   "strings"
   "syscall"
)

type tAttachEl struct {
   Name, MsgId, File string
   Size int64
}

func GetIdxAttach(iSvc string, iState *ClientState) []tAttachEl {
   aId := iState.getThread()
   aList, err := readDirNames(attachSub(iSvc, aId))
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      return []tAttachEl{}
   }
   aSend := make([]tAttachEl, len(aList))
   for a, aFn := range aList {
      aSend[a].File = aFn
      aPair := strings.SplitN(aFn, "_", 2)
      if aId[0] == '_' {
         aSend[a].MsgId = aId
      } else if len(aPair[0]) == 12 { //todo codify
         aSend[a].MsgId = aId + "_" + aPair[0]
      } else {
         aSend[a].MsgId = aPair[0]
      }
      aSend[a].Name = aPair[1]
      var aFi os.FileInfo
      aFi, err = os.Lstat(attachSub(iSvc, aId) + aFn)
      if err != nil { quit(err) }
      aSend[a].Size = aFi.Size()
   }
   return aSend
}

func GetPathAttach(iSvc string, iState *ClientState, iFile string) string {
   return attachSub(iSvc, iState.getThread()) + iFile
}

func attachSub(iSvc, iSub string) string { return attachDir(iSvc) + iSub + "/" }

func sizeSavedAttach(iSvc string, iSubHead *tHeader2, iId tSaveId) int64 {
   aTid := iId.tid(); if aTid == "" { aTid = "_" + iId.sid() }
   aPrefix := attachSub(iSvc, aTid) + iId.sid() + "_"
   var aTotal int64

   for a, aFile := range iSubHead.Attach {
      aFi, err := os.Lstat(aPrefix + path.Base(aFile.Name))
      if err != nil { quit(err) }
      iSubHead.Attach[a].Size = aFi.Size()
      iSubHead.Attach[a].Name = path.Base(aFile.Name)
      aTotal += aFi.Size()
   }
   return aTotal
}

func sendSavedAttach(iConn net.Conn, iSvc string, iSubHead *tHeader2, iId tSaveId) error {
   aTid := iId.tid(); if aTid == "" { aTid = "_" + iId.sid() }
   aPrefix := attachSub(iSvc, aTid) + iId.sid() + "_"
   for _, aFile := range iSubHead.Attach {
      aFd, err := os.Open(aPrefix + path.Base(aFile.Name))
      if err != nil { quit(err) }
      _, err = io.Copy(iConn, aFd)
      aFd.Close()
      if err != nil { return err } //todo only return net errors
   }
   return nil
}

func writeReceivedAttach(iSvc string, iHead *Header, iData []byte, iR io.Reader) error {
   var err error
   aWritten := iHead.DataLen - totalAttach(&iHead.SubHead)
   if aWritten >= int64(len(iData)) {
      iData = nil
   } else {
      iData = iData[aWritten:]
   }
   for _, aFile := range iHead.SubHead.Attach {
      aPath := tempDir(iSvc) + iHead.Id + "_" + aFile.Name //todo escape '/' in .Name
      var aFd *os.File
      aFd, err = os.OpenFile(aPath + ".tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      var aLen int64 = 0
      if len(iData) > 0 {
         aLen = int64(len(iData)); if aLen > aFile.Size { aLen = aFile.Size }
         _, err = aFd.Write(iData[:aLen])
         if err != nil { quit(err) }
         iData = iData[aLen:]
      }
      if aLen < aFile.Size {
         _, err = io.CopyN(aFd, iR, aFile.Size - aLen)
         if err != nil {
            os.Remove(aPath + ".tmp")
            return err //todo only return net errors
         }
      }
      err = aFd.Sync()
      if err != nil { quit(err) }
      err = os.Remove(aPath + ".atc")
      if err != nil && !os.IsNotExist(err) { quit(err) }
      err = os.Rename(aPath + ".tmp", aPath + ".atc")
      if err != nil { quit(err) }
   }
   err = syncDir(tempDir(iSvc))
   if err != nil { quit(err) }
   return nil
}

func storeReceivedAttach(iSvc string, iSubHead *tHeader2, iRec tComplete) {
   if iSubHead == nil || len(iSubHead.Attach) == 0 {
      return
   }
   var err error
   err = os.Mkdir(attachSub(iSvc, iRec.tid()), 0700)
   if err != nil {
      if !os.IsExist(err) { quit(err) }
   } else {
      err = syncDir(attachDir(iSvc))
      if err != nil { quit(err) }
   }
   for _, aFile := range iSubHead.Attach {
      err = renameRemove(tempDir(iSvc) + iRec.mid() + "_" + aFile.Name + ".atc",
                         attachSub(iSvc, iRec.tid()) + iRec.mid() + "_" + aFile.Name)
      if err != nil { quit(err) }
   }
   err = syncDir(attachSub(iSvc, iRec.tid()))
   if err != nil { quit(err) }
}

func storeSavedAttach(iSvc string, iSubHead *tHeader2, iRec tComplete) {
   if len(iSubHead.Attach) == 0 {
      return
   }
   var err error
   if iRec.tid() == iRec.mid() {
      err = os.Rename(attachSub(iSvc, "_" + iRec.sid()), attachSub(iSvc, iRec.tid()))
      if err != nil && !os.IsNotExist(err) { quit(err) }
      err = syncDir(attachDir(iSvc))
      if err != nil { quit(err) }
   }
   for _, aFile := range iSubHead.Attach {
      err = renameRemove(attachSub(iSvc, iRec.tid()) + iRec.sid() + "_" + path.Base(aFile.Name),
                         attachSub(iSvc, iRec.tid()) + iRec.mid() + "_" + path.Base(aFile.Name))
      if err != nil { quit(err) }
   }
   err = syncDir(attachSub(iSvc, iRec.tid()))
   if err != nil { quit(err) }
}

func validateSavedAttach(iSvc string, iSubHead *tHeader2, iId tSaveId) error {
   aTid := iId.tid(); if aTid == "" { aTid = "_" + iId.sid() }
   for _, aFile := range iSubHead.Attach {
      _, err := os.Lstat(attachSub(iSvc, aTid) + iId.sid() + "_" + path.Base(aFile.Name))
      if err != nil {
         return tError(fmt.Sprintf("%s missing %s", aTid, aFile.Name))
      }
   }
   return nil
}

func updateSavedAttach(iSvc string, iSubHeadOld, iSubHeadNew *tHeader2, iRec tComplete) {
   var err error
   aHasOld := iSubHeadOld != nil && len(iSubHeadOld.Attach) > 0
   aHasNew := iSubHeadNew != nil && len(iSubHeadNew.Attach) > 0
   aTid := iRec.tid(); if aTid == "" { aTid = "_" + iRec.sid() }

   if aHasOld {
      for _, aFile := range iSubHeadOld.Attach {
         err = os.Remove(attachSub(iSvc, aTid) + iRec.sid() + "_" + path.Base(aFile.Name))
         if err != nil && !os.IsNotExist(err) { quit(err) }
      }
      if !aHasNew {
         err = os.Remove(attachSub(iSvc, aTid))
         if err != nil {
            if !os.IsNotExist(err) && err.(*os.PathError).Err != syscall.ENOTEMPTY { quit(err) }
         } else {
            err = syncDir(attachDir(iSvc))
            if err != nil { quit(err) }
            return
         }
      }
   }
   if aHasNew {
      err = os.Mkdir(attachSub(iSvc, aTid), 0700)
      if err != nil {
         if !os.IsExist(err) { quit(err) }
      } else {
         err = syncDir(attachDir(iSvc))
         if err != nil { quit(err) }
      }
      for _, aFile := range iSubHeadNew.Attach {
         err = os.Link(kStorageDir + aFile.Name,
                       attachSub(iSvc, aTid) + iRec.sid() + "_" + path.Base(aFile.Name))
         if err != nil {
            if !os.IsNotExist(err) { quit(err) }
            fmt.Fprintf(os.Stderr, "updateSavedAttach %s: %s missing\n", iSvc, aFile.Name) //todo inform user
         }
      }
   }
   if aHasOld || aHasNew {
      err = syncDir(attachSub(iSvc, aTid))
      if err != nil { quit(err) }
   }
}

func totalAttach(iSubHead *tHeader2) int64 {
   var aLen int64
   for _, aFile := range iSubHead.Attach { aLen += aFile.Size }
   return aLen
}

