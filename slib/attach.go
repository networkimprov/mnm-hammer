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
   "encoding/json"
   "net"
   "os"
   "path"
   "strings"
   "syscall"
)

const kSuffixRecv = "_recv"
const kSuffixSent = "_sent"


type tAttachEl struct {
   Name, MsgId, File string
   Size int64
}

func GetIdxAttach(iSvc string, iState *ClientState) []tAttachEl {
   aId := iState.getThread()
   if aId == "" {
      return []tAttachEl{}
   }
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
      aSend[a].Name = aPair[1][2:] // omit x: tag
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

func sizeSavedAttach(iSvc string, iSubHead *tHeader2, iId tSaveId) int64 {
   aTid := iId.tid(); if aTid == "" { aTid = "_" + iId.sid() }
   aPrefix := attachSub(iSvc, aTid) + iId.sid() + "_"
   var aTotal int64

   for a, aFile := range iSubHead.Attach {
      if aFile.Size == 0 {
         aFi, err := os.Lstat(aPrefix + _pathToTag(aFile.Name))
         if err != nil { quit(err) }
         iSubHead.Attach[a].Size = aFi.Size()
         iSubHead.Attach[a].Name = _pathToTag(aFile.Name)
      }
      aTotal += iSubHead.Attach[a].Size
   }
   return aTotal
}

func sendSavedAttach(iConn net.Conn, iSvc string, iSubHead *tHeader2, iId tSaveId, iFd *os.File) error {
   var err error
   aTid := iId.tid(); if aTid == "" { aTid = "_" + iId.sid() }
   aPrefix := attachSub(iSvc, aTid) + iId.sid() + "_"
   for _, aFile := range iSubHead.Attach {
      var aXd, aFd *os.File = iFd, nil
      var aFi os.FileInfo
      if !_isFormFill(aFile.Name) {
         aFd, err = os.Open(aPrefix + aFile.Name)
         if err != nil { quit(err) }
         defer aFd.Close()
         aFi, err = aFd.Stat()
         if err != nil { quit(err) }
         if aFi.Size() != aFile.Size { quit(tError("file size mismatch")) }
         aXd = aFd
      }
      _, err = io.CopyN(iConn, aXd, aFile.Size)
      if err != nil { return err } //todo only return net errors
   }
   return nil
}

func tempReceivedAttach(iSvc string, iHead *Header, iR io.Reader) error {
   var err error
   aDoSync := false
   aPath := make([]string, len(iHead.SubHead.Attach))
   for a, aFile := range iHead.SubHead.Attach {
      aDoSync = true
      aPath[a] = tempDir(iSvc) + iHead.Id + "_" + aFile.Name + ".tmp" //todo escape '/' in .Name
      if _isFormFill(aFile.Name) {
         aTid := iHead.SubHead.ThreadId; if aTid == "" { aTid = iHead.Id }
         err = tempFilledForm(iSvc, aTid, iHead.Id, kSuffixRecv, &aFile, iR)
         if err != nil {
            break
         }
         continue
      }
      var aFd *os.File
      aFd, err = os.OpenFile(aPath[a], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      _, err = io.CopyN(aFd, iR, aFile.Size)
      if err != nil {
         break
      }
      err = aFd.Sync()
      if err != nil { quit(err) }
   }
   if err != nil {
      for _, aP := range aPath {
         if aP != "" { os.Remove(aP) }
      }
      return err
   }
   if aDoSync {
      err = syncDir(tempDir(iSvc))
      if err != nil { quit(err) }
   }
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
   aDoSync := false
   for _, aFile := range iSubHead.Attach {
      if _isFormFill(aFile.Name) { continue }
      aDoSync = true
      err = renameRemove(tempDir(iSvc) + iRec.mid() + "_" + aFile.Name + ".tmp",
                         attachSub(iSvc, iRec.tid()) + iRec.mid() + "_" + aFile.Name)
      if err != nil { quit(err) }
   }
   if aDoSync {
      err = syncDir(attachSub(iSvc, iRec.tid()))
      if err != nil { quit(err) }
   }
   _storeFormAttach(iSvc, iSubHead, iRec)
}

func sentAttach(i []tHeader2Attach) []tHeader2Attach {
   for a, _ := range i {
      if _isFormFill(i[a].Name) { continue }
      i[a].Name = _pathToTag(i[a].Name)
   }
   return i
}

func tempSentAttach(iSvc string, iHead *Header, iSd *os.File) {
   var err error
   aDoSync := false
   for _, aFile := range iHead.SubHead.Attach {
      if !_isFormFill(aFile.Name) { continue }
      aDoSync = true
      err = tempFilledForm(iSvc, iHead.SubHead.ThreadId, iHead.Id, kSuffixSent, &aFile, iSd)
      if err != nil { quit(err) }
   }
   if aDoSync {
      err = syncDir(tempDir(iSvc))
      if err != nil { quit(err) }
   }
}

func storeSentAttach(iSvc string, iSubHead *tHeader2, iRec tComplete) {
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
   aDoSync := false
   for _, aFile := range iSubHead.Attach {
      if _isFormFill(aFile.Name) { continue }
      aDoSync = true
      err = renameRemove(attachSub(iSvc, iRec.tid()) + iRec.sid() + "_" + aFile.Name,
                         attachSub(iSvc, iRec.tid()) + iRec.mid() + "_" + aFile.Name)
      if err != nil { quit(err) }
   }
   if aDoSync {
      err = syncDir(attachSub(iSvc, iRec.tid()))
      if err != nil { quit(err) }
   }
   _storeFormAttach(iSvc, iSubHead, iRec)
}

func _storeFormAttach(iSvc string, iSubHead *tHeader2, iRec tComplete) {
   aSuffix := kSuffixRecv; if iRec.sid() != "" { aSuffix = kSuffixSent }
   aDoSync := false
   for _, aFile := range iSubHead.Attach {
      if !_isFormFill(aFile.Name) { continue }
      aOk := storeFilledForm(iSvc, iRec.mid(), aSuffix, &aFile)
      aDoSync = aDoSync || aOk
   }
   if aDoSync {
      err := syncDir(formDir(iSvc))
      if err != nil { quit(err) }
   }
}

func validateSavedAttach(iSvc string, iSubHead *tHeader2, iId tSaveId, iFd *os.File) error {
   var err error
   aTid := iId.tid(); if aTid == "" { aTid = "_" + iId.sid() }
   for _, aFile := range iSubHead.Attach {
      if _isFormFill(aFile.Name) {
         aBuf := make([]byte, aFile.Size)
         _, err = iFd.Read(aBuf)
         if err != nil { quit(err) }
         err = validateFilledForm(iSvc, aBuf, aFile.Ffn)
         if err != nil { return err }
         continue
      }
      if strings.HasPrefix(aFile.Name, "form/") && aFile.Ffn[0] == '#' {
         return tError(aFile.Ffn[1:])
      }
      _, err = os.Lstat(attachSub(iSvc, aTid) + iId.sid() + "_" + _pathToTag(aFile.Name))
      if err != nil {
         if !os.IsNotExist(err) { quit(err) }
         return tError(fmt.Sprintf("%s missing %s", aTid, aFile.Name))
      }
   }
   return nil
}

func savedAttach(iSvc string, i *Update) []tHeader2Attach {
   aAtc := make([]tHeader2Attach, len(i.Thread.Attach))
   for a, aObj := range i.Thread.Attach {
      aAtc[a].Name = aObj.Name
      if strings.HasPrefix(aObj.Name, "form_fill/") {
         aAtc[a].Name = "r:" + aObj.Name[10:]
         aAtc[a].Size = int64(len(i.Thread.FormFill[aObj.Name[10:]]))
         aAtc[a].Ffn = aObj.Ffn
      } else if strings.HasPrefix(aObj.Name, "form/") {
         aAtc[a].Ffn = readFfnBlankForm(aObj.Name[5:])
         if aAtc[a].Ffn == "local" {
            aAtc[a].Ffn = getUriService(iSvc) + aObj.Name[5:]
         }
      }
   }
   return aAtc
}

func writeFormFillAttach(iFd *os.File, iSubHead *tHeader2, iMap map[string]string, iEl *tIndexEl) {
   var err error
   aCw := tCrcWriter{sum:iEl.Checksum}
   aTee := io.MultiWriter(iFd, &aCw)
   aSize := iEl.Size

   for _, aFile := range iSubHead.Attach {
      if !_isFormFill(aFile.Name) { continue }
      if iEl.Size == aSize {
         _, err = iFd.Seek(-1, io.SeekCurrent) // overwrite '\n'
         if err != nil { quit(err) }
      }
      aS := []byte(iMap[aFile.Name[2:]])
      if int64(len(aS)) != aFile.Size || aFile.Size <= 0 {
         quit(tError("empty or mis-sized " + aFile.Name))
      }
      err = json.Unmarshal(aS, &struct{}{})
      if err != nil { quit(err) }
      _, err = aTee.Write(aS)
      if err != nil { quit(err) }
      iEl.Size += aFile.Size
      iEl.Checksum = aCw.sum
   }
   if iEl.Size > aSize {
      _, err = iFd.Write([]byte{'\n'})
      if err != nil { quit(err) }
   }
}

func updateSavedAttach(iSvc string, iSubHeadOld, iSubHeadNew *tHeader2, iRec tComplete) {
   var err error
   aHasOld := iSubHeadOld != nil && len(iSubHeadOld.Attach) > 0
   aHasNew := iSubHeadNew != nil && len(iSubHeadNew.Attach) > 0
   aTid := iRec.tid(); if aTid == "" { aTid = "_" + iRec.sid() }

   if aHasOld {
      for _, aFile := range iSubHeadOld.Attach {
         if _isFormFill(aFile.Name) { continue }
         err = os.Remove(attachSub(iSvc, aTid) + iRec.sid() + "_" + _pathToTag(aFile.Name))
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
         if _isFormFill(aFile.Name) { continue }
         err = os.Link(kStorageDir + aFile.Name,
                       attachSub(iSvc, aTid) + iRec.sid() + "_" + _pathToTag(aFile.Name))
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
   if iSubHead.isSaved { return 0 }
   var aLen int64
   for _, aFile := range iSubHead.Attach { aLen += aFile.Size }
   return aLen
}

func _isFormFill(iName string) bool {
   return strings.HasPrefix(iName, "r:")
}

func _pathToTag(i string) string {
   return i[:1] + ":" + path.Base(i)
}

