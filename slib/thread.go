// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package slib

import (
   "bytes"
   "fmt"
   "io"
   "io/ioutil"
   "encoding/json"
   "os"
   "path"
   "sort"
   "strconv"
   "strings"
   "time"
)

func GetListThread(iSvc string, iState *ClientState) interface{} {
   var err error
   if iState.SvcTabs.PosFor == ePosForTerms &&
      strings.HasPrefix(iState.SvcTabs.Terms[iState.SvcTabs.Pos], "ffn:") {
      aResult := struct{ Ffn string; Table []Msg }{Ffn: iState.SvcTabs.Terms[iState.SvcTabs.Pos][4:]}
      err = readJsonFile(&aResult.Table, GetPathFilledForm(iSvc, aResult.Ffn))
      if err != nil {
         fmt.Fprintf(os.Stderr, "GetListThread: %s\n", err.Error())
         return []string{}
      }
      return aResult
   }
   if iState.SvcTabs.PosFor != ePosForDefault {
      return []string{}
   }
   var aDir []os.FileInfo
   if iState.SvcTabs.Pos == 3 {
      aDir, err = ioutil.ReadDir(formDir(iSvc))
      if err != nil { quit(err) }
   } else {
      aDir, err = ioutil.ReadDir(threadDir(iSvc))
      if err != nil { quit(err) }
      sort.Slice(aDir, func(cA, cB int) bool { return aDir[cA].ModTime().After(aDir[cB].ModTime()) })
   }
   aList := make([]struct{Id string; Date string}, len(aDir))
   aI := 0
   for a, _ := range aDir {
      aList[aI].Date = aDir[a].ModTime().UTC().Format(time.RFC3339)
      if iState.SvcTabs.Pos == 3 {
         aList[aI].Id = strings.Replace(aDir[a].Name(), "@", "/", -1)
         aI++
      } else if aDir[a].Name() != "_22" && !strings.ContainsRune(aDir[a].Name()[1:], '_') {
         aList[aI].Id = aDir[a].Name()
         aI++
      }
   }
   return aList[:aI]
}

type tIndexEl struct {
   Id string
   Offset int64
   Size int64
   From string
   Date string
   Subject string
   Checksum uint32
}

func _makeIndexEl(iHead *Header, iPos int64) tIndexEl {
   return tIndexEl{Id:iHead.Id, From:iHead.From, Date:iHead.Posted, Offset:iPos,
                   Subject:iHead.SubHead.Subject}
}

func GetIdxThread(iSvc string, iState *ClientState) interface{} {
   aTid := iState.getThread()
   if aTid == "" {
      return []tIndexEl{}
   }
   aFd, err := os.Open(threadDir(iSvc) + aTid)
   if err != nil { quit(err) }
   defer aFd.Close()
   var aIdx []struct{ Id, From, Date, Subject string; Size int64 }
   _ = _readIndex(aFd, &aIdx)
   for a1, a2 := 0, len(aIdx)-1; a1 < a2; a1, a2 = a1+1, a2-1 {
      aIdx[a1], aIdx[a2] = aIdx[a2], aIdx[a1]
   }
   return aIdx
}

func WriteMessagesThread(iW io.Writer, iSvc string, iState *ClientState, iId string) error {
   if iState.getThread() == "" { return nil }
   if iId != "" {
      iState.openMsg(iId, true)
   }
   aFd, err := os.Open(threadDir(iSvc) + iState.getThread())
   if err != nil { quit(err) }
   defer aFd.Close()
   var aIdx []tIndexEl
   _ = _readIndex(aFd, &aIdx)
   for a, _ := range aIdx {
      if iId != "" && aIdx[a].Id == iId || iId == "" && iState.isOpen(aIdx[a].Id) {
         var aRd, aXd *os.File
         if aIdx[a].Offset >= 0 {
            _, err = aFd.Seek(aIdx[a].Offset, io.SeekStart)
            if err != nil { quit(err) }
            aXd = aFd
         } else {
            aRd, err = os.Open(threadDir(iSvc) + aIdx[a].Id)
            if err != nil { quit(err) }
            defer aRd.Close()
            aXd = aRd
         }
         _, err = io.CopyN(iW, aXd, aIdx[a].Size)
         if err != nil { return err } //todo only return network errors
         if iId != "" {
            break
         }
      }
   }
   return nil
}

func sendSavedThread(iW io.Writer, iSvc string, iSaveId, iId string) error {
   aFd, err := os.Open(threadDir(iSvc) + iSaveId)
   if err != nil { quit(err) }
   defer aFd.Close()

   aJson := _parseHeader(aFd)
   if len(aJson.SubHead.For) == 0 { quit(tError("missing to field")) }

   aId := parseSaveId(iSaveId)
   aAttachLen := sizeSavedAttach(iSvc, &aJson.SubHead, aId) // revs subhead
   aBuf1, err := json.Marshal(aJson.SubHead)
   if err != nil { quit(err) }
   aHead := Msg{"Op":7, "Id":iId, "For":aJson.SubHead.For,
                "DataHead": len(aBuf1), "DataLen": int64(len(aBuf1)) + aJson.Len + aAttachLen }
   aBuf0, err := json.Marshal(aHead)
   if err != nil { quit(err) }
   err = sendHeaders(iW, aBuf0, aBuf1)
   if err != nil { return err }
   _, err = io.CopyN(iW, aFd, aJson.Len) //todo only return network errors
   if err != nil { return err }
   err = sendSavedAttach(iW, iSvc, &aJson.SubHead, aId, aFd)
   return err
}

//todo return open-msg map
func loadThread(iSvc string, iId string) string {
   aFd, err := os.Open(threadDir(iSvc) + iId)
   if err != nil { quit(err) }
   defer aFd.Close()
   var aIdx []tIndexEl
   _ = _readIndex(aFd, &aIdx)
   return aIdx[len(aIdx)-1].Id
}

func storeReceivedThread(iSvc string, iHead *Header, iR io.Reader) error {
   var err error
   aThreadId := iHead.SubHead.ThreadId; if aThreadId == "" { aThreadId = iHead.Id }
   aOrig := threadDir(iSvc) + aThreadId
   aTempOk := tempDir(iSvc) + aThreadId + "_" + iHead.Id + "_sr__"
   aTemp := aTempOk + ".tmp"

   fConsume := func() error {
      _, err = io.CopyN(ioutil.Discard, iR, iHead.DataLen)
      return err
   }

   if iHead.SubHead.ThreadId == "" {
      _, err = os.Lstat(aOrig)
      if err == nil {
         fmt.Fprintf(os.Stderr, "storeReceivedThread %s: thread %s already stored\n", iSvc, iHead.Id)
         return fConsume()
      }
   } else if iHead.SubHead.ThreadId[0] == '_' {
      fmt.Fprintf(os.Stderr, "storeReceivedThread %s: invalid thread id %s\n", iSvc, iHead.SubHead.ThreadId)
      return fConsume()
   }
   var aTd, aFd *os.File
   var aIdx []tIndexEl = []tIndexEl{{}}
   var aPos int64
   var aCopyLen int64
   aEl := 0
   if aThreadId != iHead.Id {
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil {
         fmt.Fprintf(os.Stderr, "storeReceivedThread %s: thread %s not found\n", iSvc, aThreadId)
         return fConsume()
      }
      defer aFd.Close()
      aPos = _readIndex(aFd, &aIdx)
      aEl = len(aIdx)
      aIdx = append(aIdx, tIndexEl{})
      for a, _ := range aIdx {
         if aIdx[a].Id == iHead.Id {
            fmt.Fprintf(os.Stderr, "storeReceivedThread %s: msg %s already stored\n", iSvc, iHead.Id)
            return fConsume()
         }
         if aIdx[a].Id > iHead.Id {
            aCopyLen = aPos - aIdx[a].Offset
            aPos = aIdx[a].Offset
            aEl = a
            copy(aIdx[a+1:], aIdx[a:])
            _, err = aFd.Seek(aPos, io.SeekStart)
            if err != nil { quit(err) }
            break
         }
      }
   }
   aIdx[aEl] = _makeIndexEl(iHead, aPos)
   aTempOk += fmt.Sprint(aPos)

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   err = _writeMsgTemp(aTd, iHead, iR, &aIdx[aEl])
   if err == nil {
      err = tempReceivedAttach(iSvc, iHead, iR)
   }
   if err != nil {
      os.Remove(aTemp)
      return err
   }
   if aCopyLen > 0 {
      _, err = io.CopyN(aTd, aFd, aCopyLen)
      if err != nil { quit(err) }
      _, err = aFd.Seek(aPos, io.SeekStart)
      if err != nil { quit(err) }
   }
   _writeIndex(aTd, aIdx)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { quit(err) }
   _completeStoreReceived(iSvc, path.Base(aTempOk), _makeHeadSaved(iHead), aFd, aTd)
   return nil
}

func _completeStoreReceived(iSvc string, iTmp string, iHead *tHeadSaved, iFd, iTd *os.File) {
   var err error
   aRec := _parseTempOk(iTmp)
   aTempOk := tempDir(iSvc) + iTmp

   resolveSentAdrsbk(iSvc, iHead.Posted, iHead.From, iHead.SubHead.Alias, aRec.tid(), aRec.mid())
   storeReceivedAttach(iSvc, &iHead.SubHead, aRec)

   if aRec.tid() == aRec.mid() {
      err = os.Link(aTempOk, threadDir(iSvc) + aRec.tid())
      if err != nil && !os.IsExist(err) { quit(err) }
      err = syncDir(threadDir(iSvc))
      if err != nil { quit(err) }
   } else {
      _, err = io.Copy(iFd, iTd) // iFd has correct pos from _readIndex
      if err != nil { quit(err) }
      err = iFd.Sync()
      if err != nil { quit(err) }
   }
   err = os.Remove(aTempOk)
   if err != nil { quit(err) }
}

func storeSentThread(iSvc string, iHead *Header) {
   var err error
   aId := parseSaveId(iHead.Id)
   if aId.tid() == "" {
      aId.tidSet(iHead.MsgId)
   }
   aSave := threadDir(iSvc) + iHead.Id
   aOrig := threadDir(iSvc) + aId.tid()
   aTempOk := tempDir(iSvc) + aId.tid() + "_" + iHead.MsgId + "_ss_" + aId.sid() + "_"
   aTemp := aTempOk + ".tmp"

   aSd, err := os.Open(aSave)
   if err != nil {
      if os.IsNotExist(err) {
         fmt.Fprintf(os.Stderr, "storeSentThread %s: saved file was cleared %s\n", iSvc, iHead.Id)
         return
      }
      quit(err)
   }
   defer aSd.Close()
   aJson := _parseHeader(aSd)
   aHead := Header{Id:iHead.MsgId, From:GetDataService(iSvc).Uid, Posted:iHead.Posted,
                   DataLen:aJson.Len, SubHead:aJson.SubHead}
   aHead.SubHead.setStore(aId.tid())

   var aIdx []tIndexEl
   var aTd, aFd *os.File
   var aPos int64
   if aId.tid() != iHead.MsgId {
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      aPos = _readIndex(aFd, &aIdx)
      a := -1
      for a, _ = range aIdx {
         if aIdx[a].Id == iHead.Id { break }
      }
      aIdx = aIdx[:a + copy(aIdx[a:], aIdx[a+1:])]
   }
   aIdx = append(aIdx, _makeIndexEl(&aHead, aPos))
   aTempOk += fmt.Sprint(aPos)

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   _writeMsgTemp(aTd, &aHead, aSd, &aIdx[len(aIdx)-1])
   _writeIndex(aTd, aIdx)
   tempSentAttach(iSvc, &aHead, aSd)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { quit(err) }
   _completeStoreSent(iSvc, path.Base(aTempOk), _makeHeadSaved(&aHead), aFd, aTd)
}

func _completeStoreSent(iSvc string, iTmp string, iHead *tHeadSaved, iFd, iTd *os.File) {
   aRec := _parseTempOk(iTmp)

   resolveReceivedAdrsbk(iSvc, iHead.Posted, iHead.SubHead.For, aRec.tid(), aRec.mid())
   storeSentAttach(iSvc, &iHead.SubHead, aRec)

   aTid := ""; if aRec.tid() != aRec.mid() { aTid = aRec.tid() }
   err := os.Remove(threadDir(iSvc) + aTid + "_" + aRec.sid())
   if err != nil && !os.IsNotExist(err) { quit(err) }

   _completeStoreReceived(iSvc, iTmp, &tHeadSaved{}, iFd, iTd)
}

func validateSavedThread(iSvc string, iUpdt *Update) error {
   aId := parseSaveId(iUpdt.Thread.Id)
   aFd, err := os.Open(threadDir(iSvc) + aId.tid() + "_" + aId.sid())
   if err != nil { quit(err) }
   defer aFd.Close()
   aJson := _parseHeader(aFd)
   if len(aJson.SubHead.For) == 0 {
      return tError(fmt.Sprintf("%s to-list empty", iUpdt.Thread.Id))
   }
   for a, aHf := range aJson.SubHead.For {
      if aHf.Id == "" {
         return tError("alias unknown: " + aJson.SubHead.Cc[a])
      }
   }
   _, err = aFd.Seek(aJson.Len, io.SeekCurrent)
   if err != nil { quit(err) }
   err = validateSavedAttach(iSvc, &aJson.SubHead, aId, aFd)
   return err
}

func storeSavedThread(iSvc string, iUpdt *Update) {
   aId := parseSaveId(iUpdt.Thread.Id)
   aOrig := threadDir(iSvc) + aId.tid()
   aTempOk := tempDir(iSvc) + aId.tid() + "__ws_" + aId.sid() + "_"
   aTemp := aTempOk + ".tmp"
   aData := bytes.NewBufferString(iUpdt.Thread.Data)
   var err error

   var aIdx []tIndexEl
   var aTd, aFd *os.File
   var aPos int64
   aEldata := tIndexEl{Id:iUpdt.Thread.Id, Date:dateRFC3339(), Subject:iUpdt.Thread.Subject, Offset:-1}
   aEl := -1
   if aId.tid() != "" {
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      aPos = _readIndex(aFd, &aIdx)
      for a, _ := range aIdx {
         if aIdx[a].Id == iUpdt.Thread.Id {
            aIdx[a] = aEldata
            aEl = a
            break
         }
      }
   }
   if aEl == -1 {
      aIdx = append(aIdx, aEldata)
      aEl = len(aIdx)-1
   }
   aTempOk += fmt.Sprint(aPos)

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   aHead := Header{Id:iUpdt.Thread.Id, From:"self", Posted:"draft", DataLen:int64(aData.Len())}
   aHead.SubHead.setWrite(aId.tid(), iUpdt, iSvc)
   _writeMsgTemp(aTd, &aHead, aData, &aIdx[aEl]) //todo stream from client
   writeFormFillAttach(aTd, &aHead.SubHead, iUpdt.Thread.FormFill, &aIdx[aEl])
   _writeIndex(aTd, aIdx)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { quit(err) }
   _completeStoreSaved(iSvc, path.Base(aTempOk), _makeHeadSaved(&aHead), aFd, aTd)
}

func _completeStoreSaved(iSvc string, iTmp string, iHead *tHeadSaved, iFd, iTd *os.File) {
   var err error
   aRec := _parseTempOk(iTmp)
   aSave := threadDir(iSvc) + aRec.tid() + "_" + aRec.sid()
   aTempOk := tempDir(iSvc) + iTmp

   var aSubHeadOld *tHeader2
   aSd, err := os.Open(aSave)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
   } else {
      aSubHeadOld = &_parseHeader(aSd).SubHead
      aSd.Close()
   }
   updateSavedAttach(iSvc, aSubHeadOld, &iHead.SubHead, aRec)

   err = os.Remove(aSave)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   if aRec.op() == "ws" {
      err = os.Link(aTempOk, aSave)
      if err != nil { quit(err) }
   }
   err = syncDir(threadDir(iSvc))
   if err != nil { quit(err) }

   if aRec.tid() != "" {
      _ = _readIndex(iTd, nil)
      err = iFd.Truncate(aRec.pos())
      if err != nil { quit(err) }
      _, err = io.Copy(iFd, iTd)
      if err != nil { quit(err) }
      err = iFd.Sync()
      if err != nil { quit(err) }
   }
   err = os.Remove(aTempOk)
   if err != nil { quit(err) }
}

func deleteSavedThread(iSvc string, iUpdt *Update) {
   aId := parseSaveId(iUpdt.Thread.Id)
   aOrig := threadDir(iSvc) + aId.tid()
   aTempOk := tempDir(iSvc) + aId.tid() + "__ds_" + aId.sid() + "_"
   aTemp := aTempOk + ".tmp"
   var err error

   var aIdx []tIndexEl
   var aTd, aFd *os.File
   var aPos int64
   if aId.tid() != "" {
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      aPos = _readIndex(aFd, &aIdx)
      a := -1
      for a, _ = range aIdx {
         if aIdx[a].Id == iUpdt.Thread.Id { break }
      }
      aIdx = aIdx[:a + copy(aIdx[a:], aIdx[a+1:])]
   }
   aTempOk += fmt.Sprint(aPos)

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   _writeIndex(aTd, aIdx)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { quit(err) }
   _completeDeleteSaved(iSvc, path.Base(aTempOk), aFd, aTd)
}

func _completeDeleteSaved(iSvc string, iTmp string, iFd, iTd *os.File) {
   _completeStoreSaved(iSvc, iTmp, &tHeadSaved{}, iFd, iTd)
}

type tHeadSaved struct {
   Len int64
   Posted string
   From string
   SubHead tHeader2
}

func _makeHeadSaved(iHead *Header) *tHeadSaved {
   return &tHeadSaved{Posted:iHead.Posted, From:iHead.From, SubHead:iHead.SubHead}
}

func _parseHeader(iFd *os.File) *tHeadSaved {
   var aHead tHeadSaved
   aBuf := make([]byte, 65536)
   _, err := iFd.Read(aBuf[:4])
   if err != nil { quit(err) }
   aUi, _ := strconv.ParseUint(string(aBuf[:4]), 16, 0)
   _, err = iFd.Read(aBuf[:aUi])
   if err != nil { quit(err) }
   err = json.Unmarshal(aBuf[:aUi], &aHead)
   if err != nil { quit(err) }
   _, err = iFd.Seek(1, io.SeekCurrent) // consume newline
   if err != nil { quit(err) }
   return &aHead
}

func _parseTempOk(i string) tComplete { return strings.SplitN(i, "_", 5) }
type tComplete []string

func (o tComplete) tid() string { return o[0] } // thread id
func (o tComplete) mid() string { return o[1] } // message id
func (o tComplete)  op() string { return o[2] } // transaction type
func (o tComplete) sid() string { return o[3] } // saved id

func (o tComplete) pos() int64 { // thread offset to index
   aPos, err := strconv.ParseInt(o[4], 10, 64)
   if err != nil { quit(err) }
   return aPos
}

func _readIndex(iFd *os.File, iIdx interface{}) int64 {
   _, err := iFd.Seek(-8, io.SeekEnd)
   if err != nil { quit(err) }
   aBuf := make([]byte, 8)
   _, err = iFd.Read(aBuf)
   if err != nil { quit(err) }
   aIdxLen, err := strconv.ParseUint(string(aBuf), 16, 0)
   if err != nil { quit(err) }
   aPos, err := iFd.Seek(-8 - int64(aIdxLen), io.SeekEnd)
   if err != nil { quit(err) }
   if iIdx == nil {
      return aPos
   }
   aBuf = make([]byte, aIdxLen)
   _, err = iFd.Read(aBuf) //todo ensure all read
   if err != nil { quit(err) }
   err = json.Unmarshal(aBuf, iIdx)
   if err != nil { quit(err) }
   aPos, err = iFd.Seek(-8 - int64(aIdxLen), io.SeekEnd)
   if err != nil { quit(err) }
   return aPos
}

func _writeIndex(iTd *os.File, iIdx []tIndexEl) {
   aBuf, err := json.Marshal(iIdx)
   if err != nil { quit(err) }
   _, err = iTd.Write(append(aBuf, fmt.Sprintf("%08x", len(aBuf))...))
   if err != nil { quit(err) }
   err = iTd.Sync()
   if err != nil { quit(err) }
   _, err = iTd.Seek(0, io.SeekStart)
   if err != nil { quit(err) }
}

func _writeMsgTemp(iTd *os.File, iHead *Header, iR io.Reader, iEl *tIndexEl) error {
   var err error
   var aCw tCrcWriter
   aTee := io.MultiWriter(iTd, &aCw)
   aSize := iHead.DataLen - totalAttach(&iHead.SubHead)
   if aSize < 0 { return tError("attachment size total exceeds DataLen") }
   aBuf, err := json.Marshal(Msg{"Id":iHead.Id, "From":iHead.From, "Posted":iHead.Posted,
                                 "Len":aSize, "SubHead":iHead.SubHead})
   if err != nil { quit(err) }
   aLen, err := aTee.Write([]byte(fmt.Sprintf("%04x", len(aBuf))))
   if err != nil { quit(err) }
   if aLen != 4 { quit(tError("json input too long")) }
   _, err = aTee.Write(append(aBuf, '\n'))
   if err != nil { quit(err) }
   if aSize > 0 {
      _, err = io.CopyN(aTee, iR, aSize)
      if err != nil { return err }
   }
   _, err = iTd.Write([]byte{'\n'})
   if err != nil { quit(err) }
   iEl.Checksum = aCw.sum // excludes final '\n'
   iEl.Size, err = iTd.Seek(0, io.SeekCurrent)
   if err != nil { quit(err) }
   return nil
}

func completeThread(iSvc string, iTempOk string) {
   var err error
   aRec := _parseTempOk(iTempOk)
   if len(aRec) != 5 {
      fmt.Fprintf(os.Stderr, "completeThread: unexpected file %s%s\n", tempDir(iSvc), iTempOk)
      return
   }
   fmt.Printf("complete %s\n", iTempOk)
   var aFd, aTd *os.File
   if aRec.tid() != "" && aRec.tid() != aRec.mid() {
      aFd, err = os.OpenFile(threadDir(iSvc)+aRec.tid(), os.O_RDWR, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      _, err = aFd.Seek(aRec.pos(), io.SeekStart)
      if err != nil { quit(err) }
   }
   aTd, err = os.Open(tempDir(iSvc)+iTempOk)
   if err != nil { quit(err) }
   defer aTd.Close()
   fGetHead := func() *tHeadSaved {
      cJson := _parseHeader(aTd)
      aTd.Seek(0, io.SeekStart)
      return cJson
   }
   switch aRec.op() {
   case "sr":
      _completeStoreReceived(iSvc, iTempOk, fGetHead(), aFd, aTd)
   case "ss":
      _completeStoreSent(iSvc, iTempOk, fGetHead(), aFd, aTd)
   case "ws":
      _completeStoreSaved(iSvc, iTempOk, fGetHead(), aFd, aTd)
   case "ds":
      _completeDeleteSaved(iSvc, iTempOk, aFd, aTd)
   default:
      fmt.Fprintf(os.Stderr, "completeThread: unexpected op %s%s\n", tempDir(iSvc), iTempOk)
   }
}
