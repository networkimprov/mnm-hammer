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
   "strconv"
   "strings"
   "sync"
)

type tIndexEl struct {
   Id string
   Offset, Size int64
   From string
   Alias string
   Date string
   Subject string
   Checksum uint32
   Seen string // mutable
}

const eSeenClear, eSeenLocal string = "!", "."

func _setupIndexEl(iEl *tIndexEl, iHead *Header, iPos int64) *tIndexEl {
   iEl.Id, iEl.From, iEl.Date, iEl.Offset, iEl.Subject, iEl.Alias =
      iHead.Id, iHead.From, iHead.Posted, iPos, iHead.SubHead.Subject, iHead.SubHead.Alias
   return iEl
}

func GetIdxThread(iSvc string, iState *ClientState) interface{} {
   aTid := iState.getThread()
   if aTid == "" { return []tIndexEl{} }

   var aIdx []struct{ Id, From, Alias, Date, Subject, Seen string
                      Queued bool }
   func() {
      cDoor := _getThreadDoor(iSvc, aTid)
      cDoor.RLock(); defer cDoor.RUnlock()
      if cDoor.renamed { return }

      cFd, err := os.Open(threadDir(iSvc) + aTid)
      if err != nil { quit(err) }
      defer cFd.Close()
      _ = _readIndex(cFd, &aIdx)
   }()
   for a, _ := range aIdx {
      if aIdx[a].From == "" {
         aIdx[a].Queued = hasQueue(iSvc, eSrecThread, aIdx[a].Id)
      }
   }
   for a1, a2 := 0, len(aIdx)-1; a1 < a2; a1, a2 = a1+1, a2-1 {
      aIdx[a1], aIdx[a2] = aIdx[a2], aIdx[a1]
   }
   return aIdx
}

func WriteMessagesThread(iW io.Writer, iSvc string, iState *ClientState, iId string) error {
   aTid := iState.getThread()
   if aTid == "" { return nil }

   aDoor := _getThreadDoor(iSvc, aTid)
   aDoor.RLock(); defer aDoor.RUnlock()
   if aDoor.renamed { return tError("thread name changed") }

   aFd, err := os.Open(threadDir(iSvc) + aTid)
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

func sendDraftThread(iW io.Writer, iSvc string, iDraftId, iId string) error {
   aFd, err := os.Open(threadDir(iSvc) + iDraftId)
   if err != nil {
      if os.IsNotExist(err) {
         fmt.Fprintf(os.Stderr, "sendDraftThread %s: draft file was cleared %s\n", iSvc, iDraftId)
         return tError("already sent")
      }
      quit(err)
   }
   defer aFd.Close()

   aDh := _readDraftHead(aFd)
   if len(aDh.SubHead.For) == 0 { quit(tError("missing to field")) }

   aId := parseLocalId(iDraftId)
   aAttachLen := sizeDraftAttach(iSvc, &aDh.SubHead, aId) // revs subhead
   aBuf1, err := json.Marshal(aDh.SubHead)
   if err != nil { quit(err) }
   aHead := Msg{"Op":7, "Id":iId, "For":aDh.SubHead.For,
                "DataHead": len(aBuf1), "DataLen": int64(len(aBuf1)) + aDh.Len + aAttachLen }
   aBuf0, err := json.Marshal(aHead)
   if err != nil { quit(err) }
   err = sendHeaders(iW, aBuf0, aBuf1)
   if err != nil { return err }
   _, err = io.CopyN(iW, aFd, aDh.Len) //todo only return network errors
   if err != nil { return err }
   err = sendDraftAttach(iW, iSvc, &aDh.SubHead, aId, aFd)
   return err
}

//todo return open-msg map
func loadThread(iSvc string, iId string) string {
   aDoor := _getThreadDoor(iSvc, iId)
   aDoor.RLock(); defer aDoor.RUnlock()
   if aDoor.renamed { return "" }

   aFd, err := os.Open(threadDir(iSvc) + iId)
   if err != nil { quit(err) }
   defer aFd.Close()
   var aIdx []tIndexEl
   _ = _readIndex(aFd, &aIdx)
   for a := len(aIdx)-1; a >= 0; a-- {
      if aIdx[a].Seen != "" {
         return aIdx[a].Id
      }
   }
   return aIdx[0].Id
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
   aIdx := []tIndexEl{{}}
   aIdxN := 0
   var aPos, aCopyLen int64
   aEl := tIndexEl{Seen:eSeenClear}

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   err = _writeMsgTemp(aTd, iHead, iR, &aEl)
   if err == nil {
      err = tempReceivedAttach(iSvc, iHead, iR)
   }
   if err != nil {
      os.Remove(aTemp)
      return err
   }
   if aThreadId != iHead.Id {
      aDoor := _getThreadDoor(iSvc, aThreadId)
      aDoor.Lock(); defer aDoor.Unlock()
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil {
         fmt.Fprintf(os.Stderr, "storeReceivedThread %s: thread %s not found\n", iSvc, aThreadId)
         os.Remove(aTemp)
         return nil
      }
      defer aFd.Close()
      aPos = _readIndex(aFd, &aIdx)
      aIdxN = len(aIdx)
      aIdx = append(aIdx, tIndexEl{})
      for a, _ := range aIdx {
         if aIdx[a].Id <  iHead.Id { continue }
         if aIdx[a].Id == iHead.Id {
            fmt.Fprintf(os.Stderr, "storeReceivedThread %s: msg %s already stored\n", iSvc, iHead.Id)
            os.Remove(aTemp)
            return nil
         }
         if aCopyLen == 0 {
            aCopyLen = aPos - aIdx[a].Offset
            aPos = aIdx[a].Offset
            aIdxN = a
            copy(aIdx[a+1:], aIdx[a:])
            _, err = aFd.Seek(aPos, io.SeekStart)
            if err != nil { quit(err) }
            _, err = io.CopyN(aTd, aFd, aCopyLen)
            if err != nil { quit(err) }
            _, err = aFd.Seek(aPos, io.SeekStart)
            if err != nil { quit(err) }
         } else {
            if aIdx[a].Offset >= 0 {
               aIdx[a].Offset += aIdx[aIdxN].Size
            }
         }
      }
   }
   aIdx[aIdxN] = *_setupIndexEl(&aEl, iHead, aPos)
   aTempOk += fmt.Sprint(aPos)

   _writeIndex(aTd, aIdx)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { quit(err) }
   _completeStoreReceived(iSvc, path.Base(aTempOk), _makeDraftHead(iHead), aFd, aTd)
   return nil
}

func _completeStoreReceived(iSvc string, iTmp string, iHead *tDraftHead, iFd, iTd *os.File) {
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

func seenReceivedThread(iSvc string, iUpdt *Update) {
   aOrig := threadDir(iSvc) + iUpdt.Thread.ThreadId
   aTempOk := tempDir(iSvc) + iUpdt.Thread.ThreadId + "__nr__"
   aTemp := aTempOk + ".tmp"
   var err error

   aDoor := _getThreadDoor(iSvc, iUpdt.Thread.ThreadId)
   aDoor.Lock(); defer aDoor.Unlock()
   if aDoor.renamed { return }

   var aTd, aFd *os.File
   aIdx := []tIndexEl{}
   aIdxN := -1
   var aPos int64

   aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
   if err != nil { quit(err) }
   defer aFd.Close()
   aPos = _readIndex(aFd, &aIdx)
   for aIdxN, _ = range aIdx {
      if aIdx[aIdxN].Id == iUpdt.Thread.Id { break }
   }
   if aIdx[aIdxN].Seen != "" {
      return
   }
   aIdx[aIdxN].Seen = dateRFC3339()
   aTempOk += fmt.Sprint(aPos)

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   _writeIndex(aTd, aIdx)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { quit(err) }
   _completeSeenReceived(iSvc, path.Base(aTempOk), aFd, aTd)
}

func _completeSeenReceived(iSvc string, iTmp string, iFd, iTd *os.File) {
   var err error
   aTempOk := tempDir(iSvc) + iTmp

   _, err = io.Copy(iFd, iTd) // iFd has correct pos from _readIndex
   if err != nil { quit(err) }
   err = iFd.Sync()
   if err != nil { quit(err) }
   err = os.Remove(aTempOk)
   if err != nil { quit(err) }
}

func storeSentThread(iSvc string, iHead *Header) {
   var err error
   aId := parseLocalId(iHead.Id)
   if aId.tid() == "" {
      aId.tidSet(iHead.MsgId)
   }
   aDraft := threadDir(iSvc) + iHead.Id
   aOrig := threadDir(iSvc) + aId.tid()
   aTempOk := tempDir(iSvc) + aId.tid() + "_" + iHead.MsgId + "_ss_" + aId.lms() + "_"
   aTemp := aTempOk + ".tmp"

   aTid := iHead.Id; if aId.tid() != iHead.MsgId { aTid = aId.tid() }
   aDoor := _getThreadDoor(iSvc, aTid)
   aDoor.Lock(); defer aDoor.Unlock()
   if aDoor.renamed { quit(tError("unexpected rename")) }
   if aId.tid() == iHead.MsgId {
      aDoor.renamed = true
   }

   aSd, err := os.Open(aDraft)
   if err != nil {
      if os.IsNotExist(err) {
         fmt.Fprintf(os.Stderr, "storeSentThread %s: draft file was cleared %s\n", iSvc, iHead.Id)
         return
      }
      quit(err)
   }
   defer aSd.Close()
   aDh := _readDraftHead(aSd)
   aHead := Header{Id:iHead.MsgId, From:GetConfigService(iSvc).Uid, Posted:iHead.Posted,
                   DataLen:aDh.Len, SubHead:aDh.SubHead}
   aHead.SubHead.setupSent(aId.tid())

   var aTd, aFd *os.File
   aIdx := []tIndexEl{}
   var aPos int64
   aEl := tIndexEl{}

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
   aIdx = append(aIdx, *_setupIndexEl(&aEl, &aHead, aPos))
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
   _completeStoreSent(iSvc, path.Base(aTempOk), _makeDraftHead(&aHead), aFd, aTd)
}

func _completeStoreSent(iSvc string, iTmp string, iHead *tDraftHead, iFd, iTd *os.File) {
   aRec := _parseTempOk(iTmp)

   resolveReceivedAdrsbk(iSvc, iHead.Posted, iHead.SubHead.For, aRec.tid(), aRec.mid())
   storeSentAttach(iSvc, &iHead.SubHead, aRec)

   aTid := ""; if aRec.tid() != aRec.mid() { aTid = aRec.tid() }
   err := os.Remove(threadDir(iSvc) + aTid + "_" + aRec.lms())
   if err != nil && !os.IsNotExist(err) { quit(err) }

   _completeStoreReceived(iSvc, iTmp, &tDraftHead{}, iFd, iTd)
}

func validateDraftThread(iSvc string, iUpdt *Update) error {
   aId := parseLocalId(iUpdt.Thread.Id)
   aFd, err := os.Open(threadDir(iSvc) + aId.tid() + "_" + aId.lms())
   if err != nil { quit(err) }
   defer aFd.Close()
   aDh := _readDraftHead(aFd)
   if len(aDh.SubHead.For) == 0 {
      return tError(fmt.Sprintf("%s to-list empty", iUpdt.Thread.Id))
   }
   for a, aHf := range aDh.SubHead.For {
      if aHf.Id == "" {
         return tError("alias unknown: " + aDh.SubHead.Cc[a])
      }
   }
   if aDh.SubHead.Subject == "" && aId.tid() == "" {
      return tError("subject missing")
   }
   _, err = aFd.Seek(aDh.Len, io.SeekCurrent)
   if err != nil { quit(err) }
   err = validateDraftAttach(iSvc, &aDh.SubHead, aId, aFd)
   return err
}

func storeDraftThread(iSvc string, iUpdt *Update) {
   aId := parseLocalId(iUpdt.Thread.Id)
   aOrig := threadDir(iSvc) + aId.tid()
   aTempOk := tempDir(iSvc) + aId.tid() + "__ws_" + aId.lms() + "_"
   aTemp := aTempOk + ".tmp"
   aData := bytes.NewBufferString(iUpdt.Thread.Data)
   var err error

   aTid := aId.tid(); if aTid == "" { aTid = "_" + aId.lms() }
   aDoor := _getThreadDoor(iSvc, aTid)
   aDoor.Lock(); defer aDoor.Unlock()
   if aDoor.renamed { quit(tError("unexpected rename")) }

   var aTd, aFd *os.File
   aIdx := []tIndexEl{}
   aIdxN := -1
   var aPos int64
   aEl := tIndexEl{Id:iUpdt.Thread.Id, Date:dateRFC3339(), Subject:iUpdt.Thread.Subject, Offset:-1}

   if aId.tid() != "" {
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      aPos = _readIndex(aFd, &aIdx)
      for a, _ := range aIdx {
         if aIdx[a].Id == iUpdt.Thread.Id {
            aIdx[a] = aEl
            aIdxN = a
            break
         }
      }
   }
   if aIdxN == -1 {
      aIdx = append(aIdx, aEl)
      aIdxN = len(aIdx)-1
   }
   aTempOk += fmt.Sprint(aPos)

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   aHead := Header{Id:iUpdt.Thread.Id, From:"self", Posted:"draft", DataLen:int64(aData.Len())}
   aHead.SubHead.setupDraft(aId.tid(), iUpdt, iSvc)
   _writeMsgTemp(aTd, &aHead, aData, &aIdx[aIdxN]) //todo stream from client
   writeFormFillAttach(aTd, &aHead.SubHead, iUpdt.Thread.FormFill, &aIdx[aIdxN])
   _writeIndex(aTd, aIdx)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { quit(err) }
   _completeStoreDraft(iSvc, path.Base(aTempOk), _makeDraftHead(&aHead), aFd, aTd)
}

func _completeStoreDraft(iSvc string, iTmp string, iHead *tDraftHead, iFd, iTd *os.File) {
   var err error
   aRec := _parseTempOk(iTmp)
   aDraft := threadDir(iSvc) + aRec.tid() + "_" + aRec.lms()
   aTempOk := tempDir(iSvc) + iTmp

   var aSubHeadOld *tHeader2
   aSd, err := os.Open(aDraft)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
   } else {
      aSubHeadOld = &_readDraftHead(aSd).SubHead
      aSd.Close()
   }
   updateDraftAttach(iSvc, aSubHeadOld, &iHead.SubHead, aRec)

   err = os.Remove(aDraft)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   if aRec.op() == "ws" {
      err = os.Link(aTempOk, aDraft)
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

func deleteDraftThread(iSvc string, iUpdt *Update) {
   aId := parseLocalId(iUpdt.Thread.Id)
   aOrig := threadDir(iSvc) + aId.tid()
   aTempOk := tempDir(iSvc) + aId.tid() + "__ds_" + aId.lms() + "_"
   aTemp := aTempOk + ".tmp"
   var err error

   aTid := aId.tid(); if aTid == "" { aTid = "_" + aId.lms() }
   aDoor := _getThreadDoor(iSvc, aTid)
   aDoor.Lock(); defer aDoor.Unlock()
   if aDoor.renamed { quit(tError("unexpected rename")) }

   var aTd, aFd *os.File
   aIdx := []tIndexEl{}
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
      if aIdx[a].Id != iUpdt.Thread.Id {
         return
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
   _completeDeleteDraft(iSvc, path.Base(aTempOk), aFd, aTd)
}

func _completeDeleteDraft(iSvc string, iTmp string, iFd, iTd *os.File) {
   _completeStoreDraft(iSvc, iTmp, &tDraftHead{}, iFd, iTd)
}

type tDraftHead struct {
   Len int64
   Posted string
   From string
   SubHead tHeader2
}

func _makeDraftHead(iHead *Header) *tDraftHead {
   return &tDraftHead{Posted:iHead.Posted, From:iHead.From, SubHead:iHead.SubHead}
}

func _readDraftHead(iFd *os.File) *tDraftHead {
   var aHead tDraftHead
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
func (o tComplete) lms() string { return o[3] } // local id milliseconds

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
   if iEl.Seen == eSeenClear {
      iEl.Seen = ""
   } else {
      iEl.Seen = eSeenLocal
   }
   iEl.Checksum = aCw.sum // excludes final '\n'
   iEl.Size, err = iTd.Seek(0, io.SeekCurrent)
   if err != nil { quit(err) }
   return nil
}

type tThreadDoor struct {
   sync.RWMutex
   renamed bool //todo provide new thread id here?
}

func _getThreadDoor(iSvc string, iTid string) *tThreadDoor {
   return getDoorService(iSvc, iTid, func()tDoor{ return &tThreadDoor{} }).(*tThreadDoor)
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
   fGetHead := func() *tDraftHead {
      cJson := _readDraftHead(aTd)
      aTd.Seek(0, io.SeekStart)
      return cJson
   }
   switch aRec.op() {
   case "sr":
      _completeStoreReceived(iSvc, iTempOk, fGetHead(), aFd, aTd)
   case "nr":
      _completeSeenReceived(iSvc, iTempOk, aFd, aTd)
   case "ss":
      _completeStoreSent(iSvc, iTempOk, fGetHead(), aFd, aTd)
   case "ws":
      _completeStoreDraft(iSvc, iTempOk, fGetHead(), aFd, aTd)
   case "ds":
      _completeDeleteDraft(iSvc, iTempOk, aFd, aTd)
   default:
      fmt.Fprintf(os.Stderr, "completeThread: unexpected op %s%s\n", tempDir(iSvc), iTempOk)
   }
}
