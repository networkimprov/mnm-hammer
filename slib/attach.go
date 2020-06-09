// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "fmt"
   "io"
   "encoding/json"
   "os"
   "sort"
   "strings"
   "time"
   "net/url"
)

const kSuffixRecv = "_recv"
const kSuffixSent = "_sent"


type tFfnIndex map[string]string

func GetIdxAttach(iSvc string, iState *ClientState) interface{} {
   type tAttachEl struct { Id, File, MsgId, Date, Who string; Size int64 }

   aId := iState.getThread()
   if aId == "" {
      return []tAttachEl{}
   }
   var aIdx []tIndexElCore
   var aDir []os.FileInfo
   err := getAttachThread(iSvc, aId, &aIdx, &aDir)
   if err != nil {
      return []tAttachEl{}
   }
   aSend := make([]tAttachEl, 0, len(aDir))
   for _, aFi := range aDir {
      if aFi.Name() == "ffnindex" { continue }
      var aFile string
      aFile, err = url.QueryUnescape(aFi.Name())
      if err != nil { quit(err) }
      aPair := strings.SplitN(aFile, "_", 2)
      aEl := tAttachEl{Id: aFile, Size: aFi.Size(),
                       MsgId: aPair[0], File: aPair[1][2:], // omit x: tag
                       Date: aFi.ModTime().UTC().Format(time.RFC3339)}
      if aId[0] == '_' {
         aEl.MsgId = aId
      } else if len(aPair[0]) == 12 { //todo codify
         aEl.MsgId = aId + "_" + aPair[0]
      }
      aN := -1
      for aN = 0; aN < len(aIdx) && aIdx[aN].Id != aEl.MsgId; aN++ {}
      if aN >= len(aIdx) {
         quit(tError("index missing attachment msgid "+ aEl.MsgId))
      }
      aEl.Who = aIdx[aN].Alias
      aSend = append(aSend, aEl)
   }
   sort.Slice(aSend, func(cA, cB int)bool { return aSend[cA].Date > aSend[cB].Date })
   return aSend
}

func GetPathAttach(iSvc string, iState *ClientState, iFile string) string {
   aDel := strings.IndexRune(iFile, '_')
   return fileAtc(iSvc, iState.getThread(), iFile[:aDel], iFile[aDel+1:])
}

func sizeDraftAttach(iSvc string, iSubHead *tHeader2, iId tLocalId) int64 {
   aTid := iId.tid(); if aTid == "" { aTid = "_" + iId.lms() }
   var aTotal int64
   for a, aFile := range iSubHead.Attach {
      if _isFormFill(aFile.Name) {
         iSubHead.Attach[a].FfKey = ""
      } else {
         aFi, err := os.Lstat(fileAtc(iSvc, aTid, iId.lms(), aFile.Name))
         if err != nil { quit(err) }
         iSubHead.Attach[a].Size = aFi.Size()
      }
      aTotal += iSubHead.Attach[a].Size
   }
   return aTotal
}

func writeDraftAttach(iW io.Writer, iSvc string, iSubHead *tHeader2, iId tLocalId, iFd *os.File) error {
   var err error
   aTid := iId.tid(); if aTid == "" { aTid = "_" + iId.lms() }
   for _, aFile := range iSubHead.Attach {
      aXd := iFd
      if !_isFormFill(aFile.Name) {
         aXd, err = os.Open(fileAtc(iSvc, aTid, iId.lms(), aFile.Name))
         if err != nil { quit(err) }
         defer aXd.Close()
         var aFi os.FileInfo
         aFi, err = aXd.Stat()
         if err != nil { quit(err) }
         if aFi.Size() != aFile.Size { quit(tError("file size mismatch")) }
      }
      _, err = io.CopyN(iW, aXd, aFile.Size)
      if err != nil { return err } //todo only return net errors
   }
   return nil
}

func tempReceivedAttach(iSvc string, iHead *Header, iR io.Reader) error {
   var err error
   aMtime, err := time.Parse(time.RFC3339, iHead.Posted)
   if err != nil {
      fmt.Fprintf(os.Stderr, "tempReceivedAttach %s: %s %s\n", iSvc, iHead.Posted, err.Error())
   }
   aDoSync := false
   for _, aFile := range iHead.SubHead.Attach {
      aDoSync = true
      if _isFormFill(aFile.Name) {
         aTid := iHead.SubHead.ThreadId; if aTid == "" { aTid = iHead.Id }
         err = tempFilledForm(iSvc, aTid, iHead.Id, kSuffixRecv, &aFile, iR)
         if err != nil {
            return err
         }
         continue
      }
      aPath := ftmpAtc(iSvc, iHead.Id, aFile.Name)
      var aFd *os.File
      aFd, err = os.OpenFile(aPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      _, err = io.CopyN(aFd, iR, aFile.Size)
      if err != nil {
         return err //todo only network errors
      }
      if !aMtime.IsZero() {
         err = os.Chtimes(aPath, time.Now(), aMtime)
         if err != nil { quit(err) }
      }
      err = aFd.Sync()
      if err != nil { quit(err) }
   }
   if aDoSync {
      err = syncDir(dirTemp(iSvc))
      if err != nil { quit(err) }
   }
   return nil
}

func removeReceivedAttach(iSvc string, iHead *Header) {
   var err error
   for _, aFile := range iHead.SubHead.Attach {
      if _isFormFill(aFile.Name) {
         removeTempFilledForm(iSvc, iHead.Id, &aFile)
      } else {
         err = os.Remove(ftmpAtc(iSvc, iHead.Id, aFile.Name))
         if err != nil && !os.IsNotExist(err) { quit(err) }
      }
   }
}

func storeReceivedAttach(iSvc string, iSubHead *tHeader2, iRec tComplete) {
   if iSubHead == nil || len(iSubHead.Attach) == 0 {
      return
   }
   var err error
   err = os.Mkdir(dirAttach(iSvc) + iRec.tid(), 0700)
   if err != nil {
      if !os.IsExist(err) { quit(err) }
   } else {
      err = syncDir(dirAttach(iSvc))
      if err != nil { quit(err) }
   }
   aDoSync, aDoFfn := false, false
   for _, aFile := range iSubHead.Attach {
      if _isFormFill(aFile.Name) { continue }
      aDoSync = true
      aDoFfn = aDoFfn || _isForm(aFile.Name)
      err = renameRemove(ftmpAtc(iSvc, iRec.mid(), aFile.Name),
                         fileAtc(iSvc, iRec.tid(), iRec.mid(), aFile.Name))
      if err != nil { quit(err) }
   }
   if aDoSync {
      var aFfnIdx tFfnIndex
      if aDoFfn { aFfnIdx = _loadFfnIndex(iSvc, iRec) }
      err = syncDir(dirAttach(iSvc) + iRec.tid())
      if err != nil { quit(err) }
      if aDoFfn { _updateFfnIndex(iSvc, iRec, aFfnIdx, iSubHead) }
   }
   _storeFormAttach(iSvc, iSubHead, iRec)
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
      err = syncDir(dirTemp(iSvc))
      if err != nil { quit(err) }
   }
}

func storeSentAttach(iSvc string, iSubHead *tHeader2, iRec tComplete) {
   if len(iSubHead.Attach) == 0 {
      return
   }
   var err error
   if iRec.tid() == iRec.mid() {
      err = os.Rename(dirAttach(iSvc) +"_"+ iRec.lms(), dirAttach(iSvc) + iRec.tid())
      if err != nil && !os.IsNotExist(err) { quit(err) }
      err = syncDir(dirAttach(iSvc))
      if err != nil { quit(err) }
   }
   aDoSync, aDoFfn := false, false
   for _, aFile := range iSubHead.Attach {
      if _isFormFill(aFile.Name) { continue }
      aDoSync = true
      aDoFfn = aDoFfn || _isForm(aFile.Name)
      err = renameRemove(fileAtc(iSvc, iRec.tid(), iRec.lms(), aFile.Name),
                         fileAtc(iSvc, iRec.tid(), iRec.mid(), aFile.Name))
      if err != nil { quit(err) }
   }
   if aDoSync {
      var aFfnIdx tFfnIndex
      if aDoFfn { aFfnIdx = _loadFfnIndex(iSvc, iRec) }
      err = syncDir(dirAttach(iSvc) + iRec.tid())
      if err != nil { quit(err) }
      if aDoFfn { _updateFfnIndex(iSvc, iRec, aFfnIdx, iSubHead) }
   }
   _storeFormAttach(iSvc, iSubHead, iRec)
}

func _loadFfnIndex(iSvc string, iRec tComplete) tFfnIndex {
   // expects to be followed by syncDir(dirAttach(iSvc) + iRec.tid())
   var aIdx tFfnIndex
   aPath := fileFfn(iSvc, iRec.tid())
   err := readJsonFile(&aIdx, aPath)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      err = os.Symlink("placeholder", aPath)
      if err != nil && !os.IsExist(err) { quit(err) }
      aIdx = make(tFfnIndex)
   }
   return aIdx
}

func _updateFfnIndex(iSvc string, iRec tComplete, iIdx tFfnIndex, iSubHead *tHeader2) {
   for _, aFile := range iSubHead.Attach {
      if !_isForm(aFile.Name) { continue }
      iIdx[iRec.mid() + "_" + aFile.Name] = aFile.Ffn
   }
   var err error
   aTemp := ftmpFfn(iSvc, iRec.tid())
   aPath := fileFfn(iSvc, iRec.tid())
   err = writeJsonFile(aTemp, iIdx)
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }
   err = os.Remove(aPath)
   if err != nil { quit(err) }
   err = os.Rename(aTemp, aPath)
   if err != nil { quit(err) }
}

func _storeFormAttach(iSvc string, iSubHead *tHeader2, iRec tComplete) {
   aSuffix := kSuffixRecv; if iRec.lms() != "" { aSuffix = kSuffixSent }
   aDoSync := false
   for _, aFile := range iSubHead.Attach {
      if !_isFormFill(aFile.Name) { continue }
      aOk := storeFilledForm(iSvc, iRec.mid(), aSuffix, &aFile)
      aDoSync = aDoSync || aOk
   }
   if aDoSync {
      err := syncDir(dirForm(iSvc))
      if err != nil { quit(err) }
   }
   for _, aFile := range iSubHead.Attach {
      if !_isFormFill(aFile.Name) { continue }
      removeTempFilledForm(iSvc, iRec.mid(), &aFile)
   }
}

func validateDraftAttach(iSvc string, iSubHead *tHeader2, iId tLocalId, iFd *os.File) error {
   var err error
   aTid := iId.tid(); if aTid == "" { aTid = "_" + iId.lms() }
   for _, aFile := range iSubHead.Attach {
      if _isFormFill(aFile.Name) {
         if aFile.Ffn == "" {
            return tError(fmt.Sprintf("%s missing Ffn", aFile.Name))
         }
         aBuf := make([]byte, aFile.Size)
         _, err = iFd.Read(aBuf)
         if err != nil { quit(err) }
         err = validateFilledForm(iSvc, aBuf, aFile.Ffn)
         if err != nil { return err }
         continue
      }
      if _isForm(aFile.Name) && aFile.Ffn[0] == '#' {
         return tError(aFile.Ffn[1:])
      }
      _, err = os.Lstat(fileAtc(iSvc, aTid, iId.lms(), aFile.Name))
      if err != nil {
         if !os.IsNotExist(err) { quit(err) }
         return tError(fmt.Sprintf("%s missing %s", aTid, aFile.Name))
      }
   }
   return nil
}

func setupDraftAttach(iSvc string, iTid string, i *Update) []tHeader2Attach {
   aAtc := i.Thread.Attach
   for a, aFile := range aAtc {
      if strings.HasPrefix(aFile.Name, "form_fill/") {
         if aFile.FfKey[12:15] == "_f:" { //todo codify
            aAtc[a].Ffn = _lookupFfn(iSvc, aFile.FfKey[15:])
         } else {
            var aIdx tFfnIndex
            err := readJsonFile(&aIdx, fileFfn(iSvc, iTid))
            if err != nil && !os.IsNotExist(err) { quit(err) }
            if err == nil {
               aAtc[a].Ffn = aIdx[aFile.FfKey]
            }
         }
         aAtc[a].Name = "r:" + aFile.Name[10:]
      } else if strings.HasPrefix(aFile.Name, "form/") {
         aAtc[a].Ffn = _lookupFfn(iSvc, aFile.Name[5:])
         aAtc[a].Name = "f:" + aFile.Name[5:]
      } else if strings.HasPrefix(aFile.Name, "upload/") {
         aAtc[a].Name = "u:" + aFile.Name[7:]
      }
      if _isFormFill(aAtc[a].Name) {
         aAtc[a].Size = int64(len(i.Thread.FormFill[aFile.FfKey]))
      }
   }
   defer func(){ i.Thread.Attach = nil }()
   return i.Thread.Attach
}

func _lookupFfn(iSvc string, iName string) string {
   aFfn := readFfnBlankForm(iName)
   if strings.HasPrefix(aFfn, "local/") {
      aFfn = getUriService(iSvc) + aFfn[6:]
   }
   return aFfn
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
      aS := []byte(iMap[aFile.FfKey])
      if len(aS) == 0 {
         quit(tError("empty " + aFile.FfKey))
      }
      err = json.Unmarshal(aS, &struct{}{})
      if err != nil { quit(err) }
      _, err = aTee.Write(aS)
      if err != nil { quit(err) }
      iEl.Size += int64(len(aS))
      iEl.Checksum = aCw.sum
   }
   if iEl.Size > aSize {
      _, err = iFd.Write([]byte{'\n'})
      if err != nil { quit(err) }
   }
}

func updateDraftAttach(iSvc string, iSubHeadOld, iSubHeadNew *tHeader2, iRec tComplete) {
   var err error
   aHasOld := iSubHeadOld != nil && len(iSubHeadOld.Attach) > 0
   aHasNew := iSubHeadNew != nil && len(iSubHeadNew.Attach) > 0
   aTid := iRec.tid(); if aTid == "" { aTid = "_" + iRec.lms() }

   if aHasOld {
      for _, aFile := range iSubHeadOld.Attach {
         if _isFormFill(aFile.Name) { continue }
         err = os.Remove(fileAtc(iSvc, aTid, iRec.lms(), aFile.Name))
         if err != nil && !os.IsNotExist(err) { quit(err) }
      }
      if !aHasNew {
         err = os.Remove(dirAttach(iSvc) + aTid)
         if err != nil {
            if !os.IsNotExist(err) && err.(*os.PathError).Err != kENOTEMPTY { quit(err) }
         } else {
            err = syncDir(dirAttach(iSvc))
            if err != nil { quit(err) }
            return
         }
      }
   }
   if aHasNew {
      err = os.Mkdir(dirAttach(iSvc) + aTid, 0700)
      if err != nil {
         if !os.IsExist(err) { quit(err) }
      } else {
         err = syncDir(dirAttach(iSvc))
         if err != nil { quit(err) }
      }
      for _, aFile := range iSubHeadNew.Attach {
         if _isFormFill(aFile.Name) { continue }
         aPath := kFormDir + aFile.Name[2:]
         if !_isForm(aFile.Name) {
            aPath = fileUpload(aFile.Name[2:])
         }
         err = os.Link(aPath, fileAtc(iSvc, aTid, iRec.lms(), aFile.Name))
         if err != nil {
            if !os.IsNotExist(err) { quit(err) }
            fmt.Fprintf(os.Stderr, "updateDraftAttach %s: %s missing\n", iSvc, aFile.Name) //todo inform user
         }
      }
   }
   if aHasOld || aHasNew {
      err = syncDir(dirAttach(iSvc) + aTid)
      if err != nil { quit(err) }
   }
}

func writeStoredAttach(iW io.Writer, iSvc string, iSubHead *tHeader2) error {
   var aLen int64
   var err error
   for _, aFile := range iSubHead.Attach {
      if _isFormFill(aFile.Name) {
         aLen, err = writeRowFilledForm(iW, iSvc, aFile.Ffn+kSuffixSent, iSubHead.ConfirmId)
      } else {
         var aFd *os.File
         aFd, err = os.Open(fileAtc(iSvc, iSubHead.ThreadId, iSubHead.ConfirmId, aFile.Name))
         if err != nil { quit(err) }
         aLen, err = io.Copy(iW, aFd)
         aFd.Close()
      }
      if err != nil {
         return err
      }
      if aLen != aFile.Size {
         quit(fmt.Errorf("size mismatch %d & %d for %s\n", aLen, aFile.Size,
                         fileAtc(iSvc, iSubHead.ThreadId, iSubHead.ConfirmId, aFile.Name)))
      }
   }
   return nil
}

func totalAttach(iSubHead *tHeader2) int64 {
   if iSubHead.noAttachSize { return 0 }
   var aLen int64
   for _, aFile := range iSubHead.Attach { aLen += aFile.Size }
   return aLen
}

func _isFormFill(iName string) bool {
   return strings.HasPrefix(iName, "r:")
}

func _isForm(iName string) bool {
   return strings.HasPrefix(iName, "f:")
}

