// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "hash/crc32"
   "fmt"
   "io"
   "encoding/json"
   "os"
   "path"
   "sort"
   "strconv"
   "strings"
   "sync"
)

const kCcNoteMaxLen = 1024

type tIndexEl struct {
   tIndexElCore
   Offset, Size int64
   Checksum uint32
}

type tIndexElCore struct {
   Id string
   From string
   Alias string
   Date string
   Subject string
   Seen string // mutable
   Tags []string `json:",omitempty"` // mutable
   ForwardBy string `json:",omitempty"` // mutable
}

const eSeenClear, eSeenLocal string = "!", "."

func _setupIndexEl(iEl *tIndexEl, iHead *Header, iPos int64) *tIndexEl {
   iEl.Id, iEl.From, iEl.Date, iEl.Offset, iEl.Subject, iEl.Alias =
      iHead.Id, iHead.From, iHead.Posted, iPos, iHead.SubHead.Subject, iHead.SubHead.Alias
   return iEl
}

type tCcEl struct {
   tCcElCore
   Checksum uint32 `json:",omitempty"`
}

type tCcElCore struct {
   Who, By string
   WhoUid, ByUid string
   Date string
   Note string
   Subscribe bool
}

type tFwdEl struct {
   Id string
   Cc []tCcEl
}

func GetIdxThread(iSvc string, iState *ClientState) interface{} {
   aIdx := []struct{ tIndexElCore; Queued bool }{}
   aTid := iState.getThread()
   if aTid == "" { return aIdx }
   func() {
      cDoor := _getThreadDoor(iSvc, aTid)
      cDoor.RLock(); defer cDoor.RUnlock()
      if cDoor.renamed { return }

      cFd, err := os.Open(dirThread(iSvc) + aTid)
      if err != nil {
         if !os.IsNotExist(err) { quit(err) }
         return
      }
      _readIndex(cFd, &aIdx, nil)
      cFd.Close()
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

func GetCcThread(iSvc string, iState *ClientState) interface{} {
   type tCcElFwd struct {
      tCcElCore
      Queued bool
      Qid string `json:",omitempty"`
   }
   const ( eFwd = iota; eCc )
   aCc := [2][]tCcElFwd{{},{}}
   aTid := iState.getThread()
   if aTid == "" { return aCc }

   fReadCc := func() bool {
      cDoor := _getThreadDoor(iSvc, aTid)
      cDoor.RLock(); defer cDoor.RUnlock()
      if cDoor.renamed { return false }

      cFd, err := os.Open(dirThread(iSvc) + aTid)
      if err != nil {
         if !os.IsNotExist(err) { quit(err) }
         return false
      }
      _readCc(cFd, &aCc[eCc])
      cFd.Close()
      return true
   }
   if !fReadCc() { return aCc }

   aDoor := _getThreadDoor(iSvc, aTid + "_forward")
   aDoor.RLock()
   aFwd := _getFwd(iSvc, aTid, "")
   aDoor.RUnlock()
   for a := range aFwd {
      aN := eFwd; if hasQueue(iSvc, eSrecFwd, aFwd[a].Id) { aN = eCc }
      aQid := ""; if aN == eFwd { aQid = aFwd[a].Id }
      for a1 := range aFwd[a].Cc {
         aCc[aN] = append(aCc[aN], tCcElFwd{tCcElCore:aFwd[a].Cc[a1].tCcElCore, Queued:aN==eCc, Qid:aQid})
      }
   }

   sort.Slice(aCc[eFwd], func(cA, cB int) bool { return aCc[eFwd][cA].Who < aCc[eFwd][cB].Who })
   return aCc
}

func WriteMessagesThread(iW io.Writer, iSvc string, iState *ClientState, iId string) error {
   aTid := iState.getThread()
   if aTid == "" { return nil }

   aBufHead := []byte{0,0,0,0}
   var aTag, aRe string
   var aBodyTotal int64
   var aFound tTermSites
   if iId == "" {
      aTabType, aTabVal := iState.getThreadTab()
      if aTabType != ePosForTerms {
         // no-op
      } else if aTabVal[0] == '&' {
         iId = aTabVal[1:]
      } else if aTabVal[0] == ':' {
         aRe = aTabVal[1:]
      } else if aTabVal[0] == '#' {
         aTag = GetIdTag(aTabVal[1:])
         if aTag == "" {
            return tError("tag not found")
         }
      } else {
         aFound = messageSearch(iSvc, aTid, aTabVal)
      }
   }
   aDoor := _getThreadDoor(iSvc, aTid)
   aDoor.RLock(); defer aDoor.RUnlock()
   if aDoor.renamed { return tError("thread name changed") }

   aFd, err := os.Open(dirThread(iSvc) + aTid)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      return tError("thread not found")
   }
   defer aFd.Close()
   var aIdx []tIndexEl
   _readIndex(aFd, &aIdx, nil)
   for a := range aIdx {
      if iId != "" {
         if aIdx[a].Id != iId { continue }
      } else if aRe != "" {
         aN := 0; if aIdx[a].Subject != "" { aN = a }
         if aIdx[aN].Subject != aRe { continue }
      } else if aTag != "" {
         a1 := -1
         for a1 = 0; a1 < len(aIdx[a].Tags); a1++ {
            if aIdx[a].Tags[a1] == aTag { break }
         }
         if a1 >= len(aIdx[a].Tags) { continue }
      } else if aFound != nil {
         if aIdx[a].Offset < 0 { continue } //todo add draft support
         _, err = aFd.Seek(aIdx[a].Offset, io.SeekStart)
         if err != nil { quit(err) }
         _, err = aFd.Read(aBufHead)
         if err != nil { quit(err) }
         aUi, _ := strconv.ParseUint(string(aBufHead), 16, 0)
         aBodyTotal += aIdx[a].Size - int64(4+aUi+1)
         if !aFound.hasTermBefore(aBodyTotal) { continue }
      } else if !iState.isOpen(aIdx[a].Id) { continue }
      var aXd *os.File
      if aIdx[a].Offset >= 0 {
         _, err = aFd.Seek(aIdx[a].Offset, io.SeekStart)
         if err != nil { quit(err) }
         aXd = aFd
      } else {
         aXd, err = os.Open(dirThread(iSvc) + aIdx[a].Id)
         if err != nil { quit(err) }
         defer aXd.Close()
      }
      _, err = io.CopyN(iW, aXd, aIdx[a].Size)
      if err != nil {
         return err //todo only return network errors
      }
      if iId != "" { break }
   }
   return nil
}

func sendDraftThread(iW io.Writer, iSvc string, iDraftId, iId string) error {
   aFd, err := os.Open(dirThread(iSvc) + iDraftId)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      fmt.Fprintf(os.Stderr, "sendDraftThread %s: draft file was cleared %s\n", iSvc, iDraftId)
      return tError("already sent")
   }
   defer aFd.Close()

   aId := parseLocalId(iDraftId)
   aMh := _readMsgHead(aFd)
   aCc := aMh.SubHead.Cc
   if aCc == nil {
      aDoor := _getThreadDoor(iSvc, aId.tid())
      aDoor.RLock()
      var aOfd *os.File
      aOfd, err = os.Open(dirThread(iSvc) + aId.tid())
      if err != nil { quit(err) }
      _readCc(aOfd, &aCc)
      aOfd.Close(); aDoor.RUnlock()
   }

   aAttachLen := sizeDraftAttach(iSvc, &aMh.SubHead, aId) // revs subhead
   aBuf1, err := json.Marshal(aMh.SubHead)
   if err != nil { quit(err) }
   aUid := GetConfigService(iSvc).Uid
   aFor := make([]tHeaderFor, 0, len(aCc)-1)
   for a := range aCc {
      if aCc[a].WhoUid == aUid { continue }
      aType := eForUser; if aCc[a].WhoUid == aCc[a].Who { aType = eForGroupExcl }
      aFor = append(aFor, tHeaderFor{Id:aCc[a].WhoUid, Type:aType})
   }
   aHead := Msg{"Op":7, "Id":iId, "For":aFor,
                "DataHead": len(aBuf1), "DataLen": int64(len(aBuf1)) + aMh.Len + aAttachLen }
   aBuf0, err := json.Marshal(aHead)
   if err != nil { quit(err) }

   err = writeHeaders(iW, aBuf0, aBuf1)
   if err != nil { return err }
   _, err = io.CopyN(iW, aFd, aMh.Len) //todo only return network errors
   if err != nil { return err }
   err = writeDraftAttach(iW, iSvc, &aMh.SubHead, aId, aFd)
   return err
}

//todo return open-msg map
func loadThread(iSvc string, iId string) string {
   aDoor := _getThreadDoor(iSvc, iId)
   aDoor.RLock(); defer aDoor.RUnlock()
   if aDoor.renamed { return "" }

   aFd, err := os.Open(dirThread(iSvc) + iId)
   if err != nil { quit(err) }
   defer aFd.Close()
   var aIdx []tIndexEl
   _ = _readIndex(aFd, &aIdx, nil)
   for a := len(aIdx)-1; a >= 0; a-- {
      if aIdx[a].Seen != "" {
         return aIdx[a].Id
      }
   }
   return aIdx[0].Id
}

func storeReceivedThread(iSvc string, iHead *Header, iR io.Reader) (string, error) {
   var err error
   aThreadId := iHead.SubHead.ThreadId; if aThreadId == "" { aThreadId = iHead.Id }
   aMsgId := iHead.Id
   aOrig := dirThread(iSvc) + aThreadId
   aTempOk := ftmpSr(iSvc, aThreadId, aMsgId)
   aTemp := aTempOk + ".tmp"

   if iHead.SubHead.ThreadId == "" {
      if iHead.SubHead.ConfirmId != "" {
         fmt.Fprintf(os.Stderr, "storeReceivedThread %s: missing thread id\n", iSvc)
         return "", discardTmtp(iHead, iR)
      }
      _, err = os.Lstat(aOrig)
      if err == nil {
         fmt.Fprintf(os.Stderr, "storeReceivedThread %s: thread %s already stored\n", iSvc, aThreadId)
         return "", discardTmtp(iHead, iR)
      }
   } else if iHead.SubHead.ThreadId[0] == '_' {
      fmt.Fprintf(os.Stderr, "storeReceivedThread %s: invalid thread id %s\n", iSvc, aThreadId)
      return "", discardTmtp(iHead, iR)
   }

   var aTd, aFd *os.File
   aIdx, aCc := []tIndexEl{{}}, []tCcEl{}
   var aPos, aCopyLen int64
   aEl := tIndexEl{tIndexElCore:tIndexElCore{Seen:eSeenClear}}
   aNewCc := iHead.SubHead.Cc; if aThreadId != aMsgId { aNewCc = nil }
   aCid := iHead.SubHead.ConfirmId

   if iHead.From == GetConfigService(iSvc).Uid {
      aEl.Seen = eSeenLocal
   }
   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   fClean := func() {
      cEr := os.Remove(aTemp)
      if cEr != nil { quit(cEr) }
      removeReceivedAttach(iSvc, iHead)
   }
   if aCid != "" {
      aTempOk = ftmpSc(iSvc, aThreadId, aCid)
      aHead := *iHead; iHead = &aHead
      iHead.Id = aCid
      iHead.Posted = iHead.SubHead.ConfirmPosted
   }
   iHead.SubHead.Cc = nil
   iHead.SubHead.ConfirmId = ""
   iHead.SubHead.ConfirmPosted = ""
   iHead.SubHead.ThreadId = aThreadId
   aMh, err := _writeMsg(aTd, iHead, iR, &aEl)
   if err == nil {
      err = tempReceivedAttach(iSvc, iHead, iR)
   }
   if err != nil {
      fClean()
      return "", err
   }
   if aThreadId == aMsgId {
      if aNewCc != nil { //todo handle invalid/missing SubHead.Cc
         aCc = aNewCc
         _revCc(aCc, iHead)
      }
      aIdx[0] = *_setupIndexEl(&aEl, iHead, aPos)
   } else {
      aDoor := _getThreadDoor(iSvc, aThreadId)
      aDoor.Lock(); defer aDoor.Unlock()
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil {
         fmt.Fprintf(os.Stderr, "storeReceivedThread %s: thread %s not found\n", iSvc, aThreadId)
         fClean()
         return "", nil
      }
      defer aFd.Close()
      aPos = _readIndex(aFd, &aIdx, &aCc)
      aIdxN := len(aIdx)
      if aCid != "" {
         for aIdxN = range aIdx {
            if aIdx[aIdxN].Id == aCid { break }
         }
         if aIdx[aIdxN].Id != aCid || aIdx[aIdxN].ForwardBy == "" {
            aMsg := "already confirmed"; if aIdx[aIdxN].Id != aCid { aMsg = "not found" }
            fmt.Fprintf(os.Stderr, "storeReceivedThread %s: confirm id %s %s\n", iSvc, aCid, aMsg)
            fClean()
            return "", nil
         }
         if aIdx[aIdxN].Size == aEl.Size && aIdx[aIdxN].Checksum == aEl.Checksum {
            aIdx[aIdxN].ForwardBy = ""
         } else {
            aIdx[aIdxN].ForwardBy += ", confirm failed"
         }
      } else {
         aIdx = append(aIdx, tIndexEl{})
         for a, _ := range aIdx {
            if aIdx[a].Id <  aMsgId || aIdx[a].Offset < 0 { continue }
            if aIdx[a].Id == aMsgId {
               fmt.Fprintf(os.Stderr, "storeReceivedThread %s: msg %s already stored\n", iSvc, aMsgId)
               fClean()
               return "", nil
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
               aIdx[a].Offset += aEl.Size
            }
         }
         aIdx[aIdxN] = *_setupIndexEl(&aEl, iHead, aPos)
      }
   }
   aTempOk += fmt.Sprint(aPos)

   _writeIndex(aTd, aIdx, aCc)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }
   if aCid != "" {
      _completeStoreConfirm(iSvc, path.Base(aTempOk), aFd, aTd, aMh, aIdx)
   } else {
      _completeStoreReceived(iSvc, path.Base(aTempOk), aFd, aTd, aMh, aNewCc)
   }

   aKind := "msg"; if aThreadId == aMsgId { aKind = "thread" }
   return aKind, nil
}

func _completeStoreConfirm(iSvc string, iTmp string, iFd, iTd *os.File, iHead *tMsgHead, iIdx []tIndexEl) {
   sCrashFn(iSvc, "store-confirm-thread")

   aRec := _parseFtmp(iTmp)
   aTempOk := dirTemp(iSvc) + iTmp
   var err error

   storeReceivedAttach(iSvc, &iHead.SubHead, aRec)

   _readIndex(iTd, nil, nil) // don't copy message
   err = iFd.Truncate(aRec.pos())
   if err != nil { quit(err) }
   _, err = io.Copy(iFd, iTd) // iFd has correct pos from caller
   if err != nil { quit(err) }
   err = iFd.Sync()
   if err != nil { quit(err) }
   _updateSearchDoc(iSvc, nil, aRec.tid(), iFd, nil)

   var aEl *tIndexEl
   for a := range iIdx {
      aEl = &iIdx[a]
      if iIdx[a].Id == aRec.mid() { break }
   }
   if aEl.ForwardBy != "" {
      fmt.Fprintf(os.Stderr, "_completeStoreConfirm %s: saved confirm mismatch %s_%s\n",
                             iSvc, aRec.tid(), aRec.mid())
      err = renameRemove(aTempOk, fileDraft(iSvc, aRec.tid(), aRec.mid())) //todo store diff
   } else {
      err = os.Remove(aTempOk)
   }
   if err != nil { quit(err) }
}

func _completeStoreReceived(iSvc string, iTmp string, iFd, iTd *os.File, iHead *tMsgHead, iCc []tCcEl) {
   sCrashFn(iSvc, "store-received-thread")

   var err error
   aRec := _parseFtmp(iTmp)
   aTempOk := dirTemp(iSvc) + iTmp

   resolveSentAdrsbk    (iSvc, iHead.Posted, iCc, aRec.tid())
   resolveReceivedAdrsbk(iSvc, iHead.Posted, iCc, aRec.tid())
   storeReceivedAttach(iSvc, &iHead.SubHead, aRec)

   if aRec.tid() == aRec.mid() {
      err = os.Link(aTempOk, dirThread(iSvc) + aRec.tid())
      if err != nil && !os.IsExist(err) { quit(err) }
      err = syncDir(dirThread(iSvc))
      if err != nil { quit(err) }
      iFd, err = os.Open(dirThread(iSvc) + aRec.tid())
      if err != nil { quit(err) }
      defer iFd.Close()
   } else {
      _, err = io.Copy(iFd, iTd) // iFd has correct pos from _readIndex
      if err != nil { quit(err) }
      err = iFd.Sync()
      if err != nil { quit(err) }
   }
   _updateSearchDoc(iSvc, nil, aRec.tid(), iFd, nil)
   err = os.Remove(aTempOk)
   if err != nil { quit(err) }
}

func touchThread(iSvc string, iUpdt *Update) bool {
   aOrig := dirThread(iSvc) + iUpdt.Touch.ThreadId
   aTempOk := ftmpTc(iSvc, iUpdt.Touch.ThreadId, "")
   if iUpdt.Touch.ThreadId[0] == '_' {
      aId := parseLocalId(iUpdt.Touch.ThreadId)
      aTempOk = ftmpTc(iSvc, aId.tid(), aId.lms())
   }
   aTemp := aTempOk + ".tmp"

   aDoor := _getThreadDoor(iSvc, iUpdt.Touch.ThreadId)
   aDoor.Lock(); defer aDoor.Unlock()
   if aDoor.renamed { return false }

   var err error
   var aTd, aFd *os.File
   aIdx, aCc := []tIndexEl{}, []tCcEl{}
   aIdxN := -1
   var aPos int64

   aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      fmt.Printf("touchThread %s: threadid not found %s\n", iSvc, iUpdt.Touch.ThreadId)
      return false
   }
   defer aFd.Close()
   aPos = _readIndex(aFd, &aIdx, &aCc)
   for aIdxN = 0; aIdxN < len(aIdx); aIdxN++ {
      if aIdx[aIdxN].Id == iUpdt.Touch.MsgId { break }
   }
   if aIdxN == len(aIdx) {
      fmt.Printf("touchThread %s: msgid not found %s\n", iSvc, iUpdt.Touch.MsgId)
      return false
   }
   switch iUpdt.Touch.Act {
   case 's':
      if aIdx[aIdxN].Seen != "" {
         return false
      }
      aIdx[aIdxN].Seen = dateRFC3339()
   case 't':
      for _, aTag := range aIdx[aIdxN].Tags {
         if aTag == iUpdt.Touch.TagId {
            return false
         }
      }
      aIdx[aIdxN].Tags = append(aIdx[aIdxN].Tags, iUpdt.Touch.TagId)
   case 'u':
      a := -1
      for a = 0; a < len(aIdx[aIdxN].Tags); a++ {
         if aIdx[aIdxN].Tags[a] == iUpdt.Touch.TagId { break }
      }
      if a >= len(aIdx[aIdxN].Tags) {
         return false
      }
      aIdx[aIdxN].Tags = aIdx[aIdxN].Tags[:a + copy(aIdx[aIdxN].Tags[a:], aIdx[aIdxN].Tags[a+1:])]
   default:
      quit(tError("unknown Update.Touch.Act: "+ string(iUpdt.Touch.Act)))
   }
   aTempOk += fmt.Sprint(aPos)

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   _writeIndex(aTd, aIdx, aCc)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }
   _completeTouch(iSvc, path.Base(aTempOk), aFd, aTd)
   return true
}

func _completeTouch(iSvc string, iTmp string, iFd, iTd *os.File) {
   sCrashFn(iSvc, "touch-thread")

   aRec := _parseFtmp(iTmp)
   aTid := aRec.tid(); if aTid == "" { aTid = "_"+ aRec.lms() }
   aTempOk := dirTemp(iSvc) + iTmp

   var err error
   err = iFd.Truncate(aRec.pos())
   if err != nil { quit(err) }
   _, err = io.Copy(iFd, iTd) // iFd has correct pos from _readIndex
   if err != nil { quit(err) }
   err = iFd.Sync()
   if err != nil { quit(err) }
   _updateSearchDoc(iSvc, nil, aTid, iFd, nil) //todo _updateUnread()
   err = os.Remove(aTempOk)
   if err != nil { quit(err) }
}

func storeSentThread(iSvc string, iHead *Header, iQid string) {
   var err error
   aId := parseLocalId(iHead.Id)
   if iQid != _makeQid(eSrecThread, aId.tid(), aId.lms()) {
      quit(tError("Qid mismatch"))
   }
   aTid := aId.tid(); if aTid == "" { aTid = iHead.MsgId }
   aDraft := dirThread(iSvc) + iHead.Id
   aOrig := dirThread(iSvc) + aTid
   aTempOk := ftmpSs(iSvc, aTid, iHead.MsgId, aId.lms())
   aTemp := aTempOk + ".tmp"

   aSd, err := os.Open(aDraft)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      fmt.Fprintf(os.Stderr, "storeSentThread %s: draft file was cleared %s\n", iSvc, iHead.Id)
      return
   }
   defer aSd.Close()

   aDoorId := aTid; if aTid == iHead.MsgId { aDoorId = iHead.Id }
   aDoor := _getThreadDoor(iSvc, aDoorId)
   aDoor.Lock(); defer aDoor.Unlock()
   if aDoor.renamed { quit(tError("unexpected rename")) }
   if aTid == iHead.MsgId {
      aDoor.renamed = true
   }

   var aTd, aFd *os.File
   aIdx, aCc := []tIndexEl{}, []tCcEl{}
   var aPos int64
   aEl := tIndexEl{}

   if aTid == iHead.MsgId {
      _readIndex(aSd, &aIdx, nil)
      aEl.Tags = aIdx[0].Tags
      aIdx = aIdx[:0]
      _, err = aSd.Seek(0, io.SeekStart)
      if err != nil { quit(err) }
   }
   aMh := _readMsgHead(aSd)
   aHeadCc := aMh.SubHead.Cc
   aMh.SubHead.Cc = nil

   if aTid == iHead.MsgId {
      aCc = aHeadCc
      _revCc(aCc, iHead)
   } else {
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      aPos = _readIndex(aFd, &aIdx, &aCc)
      a := -1
      for a, _ = range aIdx {
         if aIdx[a].Id == iHead.Id { break }
      }
      aEl.Tags = aIdx[a].Tags
      aIdx = aIdx[:a + copy(aIdx[a:], aIdx[a+1:])]
   }
   aHead := Header{Id:iHead.MsgId, From:GetConfigService(iSvc).Uid, Posted:iHead.Posted,
                   DataLen:aMh.Len, SubHead:&aMh.SubHead}
   aHead.SubHead.setupSent(aTid)
   sizeDraftAttach(iSvc, aHead.SubHead, aId)
   aIdx = append(aIdx, *_setupIndexEl(&aEl, &aHead, aPos))
   aTempOk += fmt.Sprint(aPos)

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   aMh, err = _writeMsg(aTd, &aHead, aSd, &aIdx[len(aIdx)-1])
   if err != nil { quit(err) }
   _writeIndex(aTd, aIdx, aCc)
   tempSentAttach(iSvc, &aHead, aSd)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }
   _completeStoreSent(iSvc, path.Base(aTempOk), aFd, aTd, aMh, aHeadCc)
}

func _completeStoreSent(iSvc string, iTmp string, iFd, iTd *os.File, iHead *tMsgHead, iCc []tCcEl) {
   sCrashFn(iSvc, "store-sent-thread")

   aRec := _parseFtmp(iTmp)

   resolveReceivedAdrsbk(iSvc, iHead.Posted, iCc, aRec.tid())
   storeSentAttach(iSvc, &iHead.SubHead, aRec)

   aTid := ""; if aRec.tid() != aRec.mid() { aTid = aRec.tid() }
   err := os.Remove(fileDraft(iSvc, aTid, aRec.lms()))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   if aTid == "" {
      deleteThreadSearch(iSvc, "_"+ aRec.lms())
   }
   dropQueue(iSvc, _makeQid(eSrecThread, aTid, aRec.lms()))

   _completeStoreReceived(iSvc, iTmp, iFd, iTd, &tMsgHead{}, nil)
}

func validateDraftThread(iSvc string, iUpdt *Update) error {
   aId := parseLocalId(iUpdt.Thread.Id)
   aFd, err := os.Open(fileDraft(iSvc, aId.tid(), aId.lms()))
   if err != nil { quit(err) }
   defer aFd.Close()
   aMh := _readMsgHead(aFd)
   if aMh.SubHead.Subject == "" && aId.tid() == "" {
      return tError("subject missing")
   }
   _, err = aFd.Seek(aMh.Len, io.SeekCurrent)
   if err != nil { quit(err) }
   err = validateDraftAttach(iSvc, &aMh.SubHead, aId, aFd)
   return err
}

func storeDraftThread(iSvc string, iUpdt *Update) {
   aId := parseLocalId(iUpdt.Thread.Id)
   aDraft := fileDraft(iSvc, aId.tid(), aId.lms())
   aOrig := dirThread(iSvc) + aId.tid()
   aTempOk := ftmpSd(iSvc, aId.tid(), aId.lms())
   aTemp := aTempOk + ".tmp"
   aData := strings.NewReader(iUpdt.Thread.Data)
   var err error

   aTid := aId.tid(); if aTid == "" { aTid = "_" + aId.lms() }
   aDoor := _getThreadDoor(iSvc, aTid)
   aDoor.Lock(); defer aDoor.Unlock()
   if aDoor.renamed { quit(tError("unexpected rename")) }

   var aTd, aFd *os.File
   aIdx, aCc := []tIndexEl{}, []tCcEl{}
   aIdxN := -1
   var aPos int64
   aEl := tIndexEl{Offset:-1, tIndexElCore:
                   tIndexElCore{Id:iUpdt.Thread.Id, Date:dateRFC3339(), Subject:iUpdt.Thread.Subject}}

   if aId.tid() == "" {
      aCc = _updateCc(iSvc, iUpdt.Thread.Cc, false)
      iUpdt.Thread.Cc = aCc
      var aSd *os.File
      aSd, err = os.Open(aDraft)
      if err != nil {
         if !os.IsNotExist(err) { quit(err) }
      } else {
         _readIndex(aSd, &aIdx, nil)
         aEl.Tags = aIdx[0].Tags
         aIdx = aIdx[:0]
         aSd.Close()
      }
   } else {
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      aPos = _readIndex(aFd, &aIdx, &aCc)
      for a, _ := range aIdx {
         if aIdx[a].Id == iUpdt.Thread.Id {
            aEl.Tags = aIdx[a].Tags
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
   aHead := Header{Id:iUpdt.Thread.Id, From:"self", Posted:"draft", DataLen:int64(aData.Len()),
                   SubHead:&tHeader2{}}
   aHead.SubHead.setupDraft(aId.tid(), iUpdt, iSvc)
   aMh, err := _writeMsg(aTd, &aHead, aData, &aIdx[aIdxN]) //todo stream from client
   if err != nil { quit(err) }
   writeFormFillAttach(aTd, aHead.SubHead, iUpdt.Thread.FormFill, &aIdx[aIdxN])
   _writeIndex(aTd, aIdx, aCc)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }
   _completeStoreDraft(iSvc, path.Base(aTempOk), aFd, aTd, aMh)
}

func _completeStoreDraft(iSvc string, iTmp string, iFd, iTd *os.File, iHead *tMsgHead) {
   sCrashFn(iSvc, "store-draft-thread")

   var err error
   aRec := _parseFtmp(iTmp)
   aDraft := fileDraft(iSvc, aRec.tid(), aRec.lms())
   aTempOk := dirTemp(iSvc) + iTmp

   var aSubHeadOld *tHeader2
   aSd, err := os.Open(aDraft)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
   } else {
      aSubHeadOld = &_readMsgHead(aSd).SubHead
      aSd.Close()
   }
   updateDraftAttach(iSvc, aSubHeadOld, &iHead.SubHead, aRec)

   err = os.Remove(aDraft)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   if aRec.op() == "ws" {
      err = os.Link(aTempOk, aDraft)
      if err != nil { quit(err) }
   }
   err = syncDir(dirThread(iSvc))
   if err != nil { quit(err) }

   if aRec.tid() != "" {
      _ = _readIndex(iTd, nil, nil)
      err = iFd.Truncate(aRec.pos())
      if err != nil { quit(err) }
      _, err = io.Copy(iFd, iTd)
      if err != nil { quit(err) }
      err = iFd.Sync()
      if err != nil { quit(err) }
   } else {
      iFd = iTd
   }
   aTid := aRec.tid(); if aTid == "" { aTid = "_" + aRec.lms() }
   if aRec.op() == "ws" || aRec.tid() != "" {
      _updateSearchDoc(iSvc, nil, aTid, iFd, nil)
   } else {
      deleteThreadSearch(iSvc, aTid)
   }
   err = os.Remove(aTempOk)
   if err != nil { quit(err) }
}

func deleteDraftThread(iSvc string, iUpdt *Update) {
   aId := parseLocalId(iUpdt.Thread.Id)
   aOrig := dirThread(iSvc) + aId.tid()
   aTempOk := ftmpDd(iSvc, aId.tid(), aId.lms())
   aTemp := aTempOk + ".tmp"
   var err error

   aTid := aId.tid(); if aTid == "" { aTid = "_" + aId.lms() }
   aDoor := _getThreadDoor(iSvc, aTid)
   aDoor.Lock(); defer aDoor.Unlock()
   if aDoor.renamed { quit(tError("unexpected rename")) }

   var aTd, aFd *os.File
   aIdx, aCc := []tIndexEl{}, []tCcEl{}
   var aPos int64

   if aId.tid() != "" {
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      aPos = _readIndex(aFd, &aIdx, &aCc)
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
   _writeIndex(aTd, aIdx, aCc)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }
   _completeDeleteDraft(iSvc, path.Base(aTempOk), aFd, aTd)
}

func _completeDeleteDraft(iSvc string, iTmp string, iFd, iTd *os.File) {
   sCrashFn(iSvc, "delete-draft-thread")

   _completeStoreDraft(iSvc, iTmp, iFd, iTd, &tMsgHead{})
}

func sendFwdConfirmThread(iW io.Writer, iSvc string, iDraftId, iId string) error {
   const ( eTid = iota; eMid; eDate; eByUid )
   aRec := strings.SplitN(iDraftId, "_", eByUid+1)

   aDoor := _getThreadDoor(iSvc, aRec[eTid])
   aDoor.RLock(); defer aDoor.RUnlock()

   aFd, err := os.Open(dirThread(iSvc) + aRec[eTid])
   if err != nil { quit(err) }
   defer aFd.Close()

   var aIdx []tIndexEl
   var aCc []tCcEl
   _readIndex(aFd, &aIdx, &aCc)

   aFor := make([]tHeaderFor, 0, len(aCc))
   for a := range aCc {
      if aCc[a].Date != aRec[eDate] || aCc[a].ByUid != aRec[eByUid] { continue }
      aType := eForUser; if aCc[a].WhoUid == aCc[a].Who { aType = eForGroupExcl }
      aFor = append(aFor, tHeaderFor{Id:aCc[a].WhoUid, Type:aType})
   }

   var aEl *tIndexEl
   for a := range aIdx {
      aEl = &aIdx[a]
      if aEl.Id == aRec[eMid] { break }
   }
   _, err = aFd.Seek(aEl.Offset, io.SeekStart)
   if err != nil { quit(err) }
   aMh := _readMsgHead(aFd)
   aMh.SubHead.ConfirmId = aEl.Id
   aMh.SubHead.ConfirmPosted = aEl.Date
   aBufSub, err := json.Marshal(aMh.SubHead)
   if err != nil { quit(err) }

   aHead := Msg{"Op":7, "Id":iId, "For":aFor, "DataHead":len(aBufSub),
                "DataLen": int64(len(aBufSub)) + aMh.Len + totalAttach(&aMh.SubHead)}
   aBufHead, err := json.Marshal(aHead)
   if err != nil { quit(err) }

   err = writeHeaders(iW, aBufHead, aBufSub)
   if err != nil { return err }
   _, err = io.CopyN(iW, aFd, aMh.Len)
   if err != nil { return err }
   err = writeStoredAttach(iW, iSvc, &aMh.SubHead)
   return err
}

func sendFwdDraftThread(iW io.Writer, iSvc string, iDraftId, iId string) error {
   aId := parseLocalId(iDraftId)
   aUid := GetConfigService(iSvc).Uid
   var aCc []tCcEl

   fCcToFor := func() []tHeaderFor {
      cFor := make([]tHeaderFor, 0, len(aCc))
      for c := range aCc {
         if aCc[c].WhoUid == aUid { continue }
         cType := eForUser; if aCc[c].WhoUid == aCc[c].Who { cType = eForGroupExcl }
         cFor = append(cFor, tHeaderFor{Id:aCc[c].WhoUid, Type:cType})
      }
      return cFor
   }

   aDoor := _getThreadDoor(iSvc, aId.tid() + "_forward")
   aDoor.RLock()
   aFwd := _getFwd(iSvc, aId.tid(), "exist")
   aDoor.RUnlock()
   for a := range aFwd {
      if aFwd[a].Id == iDraftId {
         aCc = aFwd[a].Cc
         break
      }
   }
   if aCc == nil {
      fmt.Fprintf(os.Stderr, "sendFwdDraftThread %s: forward entry was cleared %s\n", iSvc, iDraftId)
      return tError("already sent")
   }
   aFor := fCcToFor()
   aBufNote, err := json.Marshal(&tHeader2{ThreadId:aId.tid(), Cc:aCc})
   if err != nil { quit(err) }
   aBufSubh, err := json.Marshal(&tHeader2{ThreadId:aId.tid()})
   if err != nil { quit(err) }

   aDoor = _getThreadDoor(iSvc, aId.tid())
   aDoor.RLock(); defer aDoor.RUnlock()

   aFd, err := os.Open(dirThread(iSvc) + aId.tid())
   if err != nil { quit(err) }
   defer aFd.Close()

   var aIdx []tIndexEl
   aLenMsg := _readIndex(aFd, &aIdx, &aCc)
   _, err = aFd.Seek(0, io.SeekStart)
   if err != nil { quit(err) }
   aForNote := fCcToFor()
   aBufCc, err := json.Marshal(aCc)
   if err != nil { quit(err) }
   for a := len(aIdx)-1; a >= 0; a-- {
      aIdx[a].Seen = ""
      aIdx[a].Tags = nil
      if aIdx[a].Offset < 0 {
         aIdx = aIdx[:a + copy(aIdx[a:], aIdx[a+1:])]
      }
   }
   aBufIdx, err := json.Marshal(aIdx)
   if err != nil { quit(err) }
   aStrTail := fmt.Sprintf("%08x%08x", len(aBufIdx), len(aBufCc))
   aHead := Msg{"Op":8, "Id":iId, "For":aFor, "ForNotSelf":true, "DataHead":len(aBufSubh),
                "NoteFor":aForNote, "NoteLen":len(aBufNote), "NoteHead":len(aBufNote),
                "DataLen": aLenMsg + int64(len(aBufIdx) + len(aBufCc) + len(aStrTail) +
                           len(aBufNote) + len(aBufSubh))}
   aBufHead, err := json.Marshal(aHead)
   if err != nil { quit(err) }

   err = writeHeaders(iW, aBufHead, append(aBufNote, aBufSubh...))
   if err != nil { return err }
   _, err = io.CopyN(iW, aFd, aLenMsg)
   if err != nil { return err }
   _, err = iW.Write(aBufIdx)
   if err != nil { return err }
   _, err = iW.Write(aBufCc)
   if err != nil { return err }
   _, err = io.WriteString(iW, aStrTail)
   return err
}

func storeFwdReceivedThread(iSvc string, iHead *Header, iR io.Reader) error {
   aOrig := dirThread(iSvc) + iHead.SubHead.ThreadId
   aTempOk := ftmpFr(iSvc, iHead.SubHead.ThreadId)
   aTemp := aTempOk + ".tmp"
   var err error

   if iHead.SubHead.ThreadId == "" {
      fmt.Fprintf(os.Stderr, "storeFwdReceivedThread %s: missing thread id\n", iSvc)
      return discardTmtp(iHead, iR)
   }
   if iHead.SubHead.ThreadId[0] == '_' {
      fmt.Fprintf(os.Stderr, "storeFwdReceivedThread %s: invalid thread id %s\n",
                             iSvc, iHead.SubHead.ThreadId)
      return discardTmtp(iHead, iR)
   }
   _, err = os.Lstat(aOrig)
   if err == nil {
      fmt.Fprintf(os.Stderr, "storeFwdReceivedThread %s: thread %s already stored\n",
                             iSvc, iHead.SubHead.ThreadId)
      return discardTmtp(iHead, iR)
   }

   aTd, err := os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   _, err = io.CopyN(aTd, iR, iHead.DataLen)
   if err != nil {     //todo network errors only
      os.Remove(aTemp)
      return err
   }

   aIdx, aCc := []tIndexEl{}, []tCcEl{}
   _ = _readIndex(aTd, &aIdx, &aCc)
   for a := range aIdx {
      aIdx[a].ForwardBy = iHead.From
   }
   _writeIndex(aTd, aIdx, aCc)

   aTempOk += "0"
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }
   _completeStoreFwdReceived(iSvc, path.Base(aTempOk), aTd)
   return nil
}

func _completeStoreFwdReceived(iSvc string, iTmp string, iTd *os.File) {
   sCrashFn(iSvc, "store-fwd-received-thread")

   aRec := _parseFtmp(iTmp)
   aTempOk := dirTemp(iSvc) + iTmp

   err := os.Link(aTempOk, dirThread(iSvc) + aRec.tid())
   if err != nil && !os.IsExist(err) { quit(err) }
   err = syncDir(dirThread(iSvc))
   if err != nil { quit(err) }
   _updateSearchDoc(iSvc, nil, aRec.tid(), iTd, nil)
   err = os.Remove(aTempOk)
   if err != nil { quit(err) }
}

func storeFwdNotifyThread(iSvc string, iHead *Header, iR io.Reader) error {
   aOrig := dirThread(iSvc) + iHead.SubHead.ThreadId
   aTempOk := ftmpFn(iSvc, iHead.SubHead.ThreadId)
   aTemp := aTempOk + ".tmp"
   var err error

   if iHead.DataLen > 0 {
      fmt.Fprintf(os.Stderr, "storeFwdNotifyThread %s: datalen too long, postid %s\n", iSvc, iHead.PostId)
      return discardTmtp(iHead, iR)
   }
   if iHead.SubHead.ThreadId == "" {
      fmt.Fprintf(os.Stderr, "storeFwdNotifyThread %s: threadid missing, postid %s\n", iSvc, iHead.PostId)
      return nil
   }

   var aTd, aFd *os.File
   aCc := []tCcEl{}
   var aPos, aLenIdx int64

   aDoor := _getThreadDoor(iSvc, iHead.SubHead.ThreadId)
   aDoor.Lock(); defer aDoor.Unlock()

   aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      fmt.Fprintf(os.Stderr, "storeFwdNotifyThread %s: threadid %s not found, postid %s\n",
                             iSvc, iHead.SubHead.ThreadId, iHead.PostId)
      return nil
   }
   defer aFd.Close()
   aPos, aLenIdx = _readCc(aFd, &aCc)
   _, err = aFd.Seek(aPos, io.SeekStart)
   if err != nil { quit(err) }

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   _revCc(iHead.SubHead.Cc, iHead)
   aCc = append(aCc, iHead.SubHead.Cc...)
   _writeCc(aTd, aCc, aLenIdx)

   aTempOk += fmt.Sprint(aPos)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }
   _completeStoreFwdNotify(iSvc, path.Base(aTempOk), aFd, aTd, aCc)
   return nil
}

func _completeStoreFwdNotify(iSvc string, iTmp string, iFd, iTd *os.File, iCc []tCcEl) {
   sCrashFn(iSvc, "store-fwd-notify-thread")

   aRec := _parseFtmp(iTmp)
   aUid := GetConfigService(iSvc).Uid
   var aCcRes []tCcEl

   a := len(iCc)-1
   if iCc[a].WhoUid == aUid { aCcRes = iCc }
   for aD, aB := iCc[a].Date, iCc[a].ByUid; a > 0 && iCc[a-1].Date == aD && iCc[a-1].ByUid == aB; a-- {
      if iCc[a-1].WhoUid == aUid { aCcRes = iCc }
   }
   if aCcRes == nil { aCcRes = iCc[a:] }
   iCc = iCc[a:]

   resolveSentAdrsbk    (iSvc, iCc[0].Date, aCcRes, aRec.tid())
   resolveReceivedAdrsbk(iSvc, iCc[0].Date, aCcRes, aRec.tid())

   _finishStoreFwd(iSvc, iTmp, iFd, iTd, iCc)
}

func storeFwdSentThread(iSvc string, iHead *Header, iQid string) {
   aId := parseLocalId(iHead.Id)
   if iQid != _makeQid(eSrecFwd, aId.tid(), aId.lms()) {
      quit(tError("Qid mismatch"))
   }
   aOrig := dirThread(iSvc) + aId.tid()
   aTempOk := ftmpFs(iSvc, aId.tid(), aId.lms())
   aTemp := aTempOk + ".tmp"
   aFwdTemp := ftmpFwdS(iSvc, aId.tid())
   var err error

   aDoor := _getThreadDoor(iSvc, aId.tid() + "_forward")
   aDoor.Lock(); defer aDoor.Unlock()

   aFwd := _getFwd(iSvc, aId.tid(), "")
   if len(aFwd) == 0 || aFwd[0].Id != iHead.Id {
      fmt.Fprintf(os.Stderr, "storeFwdSentThread %s: forward draft was cleared %s\n", iSvc, iHead.Id)
      return
   }
   err = writeJsonFile(aFwdTemp, aFwd[1:])
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }

   aDoor = _getThreadDoor(iSvc, aId.tid())
   aDoor.Lock(); defer aDoor.Unlock()

   var aTd, aFd *os.File
   aCc := []tCcEl{}
   var aPos, aLenIdx int64

   aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
   if err != nil { quit(err) }
   defer aFd.Close()
   aPos, aLenIdx = _readCc(aFd, &aCc)
   _, err = aFd.Seek(aPos, io.SeekStart)
   if err != nil { quit(err) }

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aTd.Close()
   _revCc(aFwd[0].Cc, iHead)
   aCc = append(aCc, aFwd[0].Cc...)
   _writeCc(aTd, aCc, aLenIdx)

   aTempOk += fmt.Sprint(aPos)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }
   _completeStoreFwdSent(iSvc, path.Base(aTempOk), aFd, aTd, aCc, len(aFwd)-1)
}

func _completeStoreFwdSent(iSvc string, iTmp string, iFd, iTd *os.File, iCc []tCcEl, iFwdN int) {
   sCrashFn(iSvc, "store-fwd-sent-thread")

   aRec := _parseFtmp(iTmp)
   aFwdOrig := fileFwd(iSvc, aRec.tid())
   aFwdTemp := ftmpFwdS(iSvc, aRec.tid())
   var err error

   a := len(iCc)-1
   for aD, aB := iCc[a].Date, iCc[a].ByUid; a > 0 && iCc[a-1].Date == aD && iCc[a-1].ByUid == aB; a-- {}
   iCc = iCc[a:]

   resolveReceivedAdrsbk(iSvc, iCc[0].Date, iCc, aRec.tid())

   if iFwdN >= 0 {
      err = os.Remove(aFwdOrig)
      if err != nil && !os.IsNotExist(err) { quit(err) }
      if iFwdN > 0 {
         err = os.Rename(aFwdTemp, aFwdOrig)
      } else {
         err = os.Remove(aFwdTemp)
      }
      if err != nil { quit(err) }
      err = syncDir(dirThread(iSvc))
      if err != nil { quit(err) }
   }
   dropQueue(iSvc, _makeQid(eSrecFwd, aRec.tid(), aRec.lms()))
   _finishStoreFwd(iSvc, iTmp, iFd, iTd, iCc)
}

func _finishStoreFwd(iSvc string, iTmp string, iFd, iTd *os.File, iCc []tCcEl) {
   aRec := _parseFtmp(iTmp)
   var err error

   _, err = io.Copy(iFd, iTd) // iFd has correct pos from caller
   if err != nil { quit(err) }
   err = iFd.Sync()
   if err != nil { quit(err) }

   aUid := GetConfigService(iSvc).Uid
   var aIds []string
   var aIdx []tIndexEl
   _readIndex(iFd, &aIdx, nil) //todo skip if aUid is in iCc?
   for a := range aIdx {
      if aIdx[a].From == aUid {
         aIds = append(aIds, aRec.tid() +"_"+ aIdx[a].Id +"_"+ iCc[0].Date +"_"+ iCc[0].ByUid)
      }
   }
   aPost := addListQueue(iSvc, eSrecCfm, aIds, "nopost")

   err = os.Remove(dirTemp(iSvc) + iTmp)
   if err != nil { quit(err) }

   if aPost != nil {
      aSvc := getService(iSvc)
      if aSvc.sendQPost != nil {
         aSvc.sendQPost(aPost...)
      }
   }
}

func storeFwdDraftThread(iSvc string, iUpdt *Update) {
   aFwdOrig := fileFwd(iSvc, iUpdt.Forward.ThreadId)
   aFwdTemp := ftmpFwdD(iSvc, iUpdt.Forward.ThreadId)

   if iUpdt.Forward.ThreadId[0] == '_' { quit(tError("cannot forward draft")) }

   aCcNew := iUpdt.Forward.Cc
   fCheckInput := func(cSet []tCcEl) {
      for c := len(aCcNew)-1; c >= 0; c-- {
         if aCcNew[c].Date != "" { continue }
         for c1 := range cSet {
            if cSet[c1].WhoUid == aCcNew[c].WhoUid {
               aCcNew = aCcNew[:c + copy(aCcNew[c:], aCcNew[c+1:])]
               break
            }
         }
      }
   }
   var aCcOrig []tCcEl
   aDoor := _getThreadDoor(iSvc, iUpdt.Forward.ThreadId)
   aDoor.RLock()
   aFd, err := os.Open(dirThread(iSvc) + iUpdt.Forward.ThreadId)
   if err != nil { quit(err) }
   _readCc(aFd, &aCcOrig)
   aFd.Close(); aDoor.RUnlock()
   fCheckInput(aCcOrig)

   aDoor = _getThreadDoor(iSvc, iUpdt.Forward.ThreadId + "_forward")
   aDoor.Lock(); defer aDoor.Unlock()

   aFwd := _getFwd(iSvc, iUpdt.Forward.ThreadId, "make")
   if len(aFwd) == 0 || hasQueue(iSvc, eSrecFwd, aFwd[len(aFwd)-1].Id) {
      aFwd = append(aFwd, tFwdEl{Id:makeLocalId(iUpdt.Forward.ThreadId)})
   }
   for a := 0; a < len(aFwd)-1; a++ {
      fCheckInput(aFwd[a].Cc)
   }
   if len(aFwd) == 1 && len(aCcNew) == 0 {
      err = os.Remove(aFwdOrig)
      return
   }
   aFwd[len(aFwd)-1].Cc = _updateCc(iSvc, aCcNew, true)
   err = writeJsonFile(aFwdTemp, aFwd)
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }
   err = os.Remove(aFwdOrig)
   if err != nil { quit(err) }
   err = os.Rename(aFwdTemp, aFwdOrig)
   if err != nil { quit(err) }
}

func _getFwd(iSvc string, iTid string, iOpt string) []tFwdEl {
   aPath := fileFwd(iSvc, iTid)
   if iOpt == "temp" {
      aPath = ftmpFwdS(iSvc, iTid)
      iOpt = ""
   }
   var aFwd []tFwdEl
   aFd, err := os.Open(aPath)
   if err != nil {
      if iOpt == "exist" || !os.IsNotExist(err) { quit(err) }
      if iOpt == "" {
         return aFwd
      }
      if iOpt != "make" { quit(tError("unknown option "+iOpt)) }
      _, err = os.Lstat(dirThread(iSvc) + iTid)
      if err != nil { quit(err) }
      err = os.Symlink("placeholder", aPath)
      if err == nil {
         err = syncDir(dirThread(iSvc))
      }
      if err != nil && !os.IsExist(err) { quit(err) }
   } else {
      err = json.NewDecoder(aFd).Decode(&aFwd)
      aFd.Close()
      if err != nil { quit(err) }
   }
   return aFwd
}

func _updateCc(iSvc string, iCc []tCcEl, iOmitSelf bool) []tCcEl {
   aCfg := GetConfigService(iSvc)
   for a := range iCc {
      iOmitSelf = iOmitSelf || iCc[a].WhoUid == aCfg.Uid
      if iCc[a].Date != "" { continue }
      iCc[a].Date = dateRFC3339()
      iCc[a].ByUid = aCfg.Uid
      iCc[a].By = aCfg.Alias
      iCc[a].Subscribe = true
   }
   if !iOmitSelf {
      iCc = append([]tCcEl{{tCcElCore:tCcElCore{
                            Who: aCfg.Alias, WhoUid: aCfg.Uid,
                            By:  aCfg.Alias, ByUid:  aCfg.Uid,
                            Date: dateRFC3339(), Note: "author", Subscribe: true}}},
                   iCc...)
   }
   return iCc
}

func _revCc(iCc []tCcEl, iHead *Header) {
   var err error
   var aBuf []byte
   for a := range iCc {
      if len(iCc[a].Note) > kCcNoteMaxLen {
         iCc[a].Note = iCc[a].Note[:kCcNoteMaxLen-7] + "[trunc]"
      }
      iCc[a].Date = iHead.Posted
      aBuf, err = json.Marshal(iCc[a])
      if err != nil { quit(err) }
      iCc[a].Checksum = crc32.Checksum(aBuf, kCrc32c)
   }
}

/*func _updateUnread(iSvc string, iTid string, iFd *os.File) {
   var aIdx []tIndexEl
   _readIndex(iFd, &aIdx, nil)
   aUnread := false
   for a := range aIdx {
      aUnread = aUnread || aIdx[a].Seen == ""
   }
   updateUnreadSearch(iSvc, iTid, aUnread)
}*/

func _updateSearchDoc(iSvc string, iCfg *tSvcConfig, iTid string, iFd *os.File, iI tIndexer) {
   if iCfg == nil {
      iCfg = GetConfigService(iSvc)
   }
   aLastSubjectN, aHasDraft := -1, false
   var aIdx []tIndexEl
   _readIndex(iFd, &aIdx, nil)
   aDoc := &tSearchDoc{id: iTid, OrigDate: aIdx[0].Date, OrigAuthor: aIdx[0].Alias}
   for a := range aIdx {
      if aIdx[a].Alias != "" && aIdx[a].Alias != iCfg.Alias {
         aDoc.Author.addUnique(aIdx[a].Alias)
      }
      if aIdx[a].Subject != "" || a == 0 {
         aDoc.Subject.addUnique(aIdx[a].Subject)
      }
      if aIdx[a].From == "" || !aHasDraft {
         aLastSubjectN = a
         aHasDraft = aIdx[a].From == ""
      }
      for a1 := range aIdx[a].Tags {
         aDoc.Tag.addUnique(aIdx[a].Tags[a1])
      }
      if aIdx[a].From != "" || a == 0 {
         aDoc.LastDate, aDoc.LastAuthor = aIdx[a].Date, aIdx[a].Alias
      }
      aDoc.Unread = aDoc.Unread || aIdx[a].Seen == ""
   }
   aSubj := aIdx[aLastSubjectN].Subject; if aSubj == "" { aSubj = aIdx[0].Subject }
   for a := range aDoc.Subject {
      if aDoc.Subject[a] != aSubj { continue }
      aDoc.LastSubjectN = a
      break
   }
   aDoc.bodyStream = _newThreadStream(iSvc, aIdx, iFd)
   indexThreadSearch(iSvc, aDoc, iI)
}

type tThreadStream struct {
   bufHead []byte
   svc string
   idx []tIndexEl
   a int
   fd *os.File
   pos int64
   draft *tThreadStream
}

func _newThreadStream(iSvc string, iIdx []tIndexEl, iFd *os.File) *tThreadStream {
   o := &tThreadStream{svc: iSvc, idx: iIdx, fd: iFd, bufHead: make([]byte, 4)}
   _, err := iFd.Seek(0, io.SeekStart)
   if err != nil { quit(err) }
   if len(iIdx) == 1 && iIdx[0].Offset < 0 {
      iIdx[0].Offset = 0
   }
   for a := len(iIdx)-1; a >= 0; a-- { // move drafts to end to preserve file sequence
      if iIdx[a].Offset >= 0 { continue }
      aEl := iIdx[a]
      iIdx = iIdx[:a + copy(iIdx[a:], iIdx[a+1:])]
      o.idx[len(iIdx)] = aEl
   }
   return o
}

func (o *tThreadStream) Read(iBuf []byte) (int, error) { // for io.Reader
   if o.a >= len(o.idx) {
      return 0, io.EOF
   }
   var err error
   var aLen int
   if o.idx[o.a].Offset < 0 {
      if o.draft == nil {
         o.draft = &tThreadStream{bufHead: o.bufHead, idx: []tIndexEl{o.idx[o.a]}}
         o.draft.idx[0].Offset = 0
         o.draft.fd, err = os.Open(dirThread(o.svc) + o.idx[o.a].Id)
         if err != nil { quit(err) }
      }
      aLen, err = o.draft.Read(iBuf)
      if err != nil {
         if err != io.EOF { quit(err) }
         o.draft.fd.Close()
         o.draft = nil
         o.a++
         return 0, nil
      }
      return aLen, nil
   }
   if o.pos == o.idx[o.a].Offset {
      _, err = o.fd.Read(o.bufHead)
      if err != nil { quit(err) }
      aUi, _ := strconv.ParseUint(string(o.bufHead), 16, 0)
      o.pos, err = o.fd.Seek(int64(aUi)+1, io.SeekCurrent)
      if err != nil { quit(err) }
   }
   aMax := o.idx[o.a].Offset + o.idx[o.a].Size - o.pos
   if aMax <= int64(len(iBuf)) {
      iBuf = iBuf[:aMax]
      o.a++
   }
   aLen, err = o.fd.Read(iBuf)
   if err != nil { quit(err) }
   if aLen < len(iBuf) {
      quit(tError("read length short"))
   }
   o.pos += int64(aLen)
   return aLen, nil
}

type tMsgHead struct {
   Id string
   Len int64
   Posted string
   From string
   SubHead tHeader2
}

func _readMsgHead(iFd *os.File) *tMsgHead {
   var aHead tMsgHead
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

func _readIndex(iFd *os.File, iIdx, iCc interface{}) int64 {
   aLenIdx, aLenCc := _readTail(iFd)
   aPos, err := iFd.Seek(-16 - aLenIdx - aLenCc, io.SeekEnd)
   if err != nil { quit(err) }
   if iIdx == nil {
      return aPos
   }
   aBuf := make([]byte, aLenIdx)
   _, err = iFd.Read(aBuf) //todo ensure all read
   if err != nil { quit(err) }
   err = json.Unmarshal(aBuf, iIdx)
   if err != nil { quit(err) }
   if iCc != nil {
      aBuf = make([]byte, aLenCc)
      _, err = iFd.Read(aBuf) //todo ensure all read
      if err != nil { quit(err) }
      err = json.Unmarshal(aBuf, iCc)
      if err != nil { quit(err) }
   }
   _, err = iFd.Seek(aPos, io.SeekStart)
   if err != nil { quit(err) }
   return aPos
}

func _readCc(iFd *os.File, iCc interface{}) (int64, int64) {
   aLenIdx, aLenCc := _readTail(iFd)
   aBuf := make([]byte, aLenCc)
   aPos, err := iFd.Seek(-16 - aLenCc, io.SeekEnd)
   if err != nil { quit(err) }
   _, err = iFd.Read(aBuf) //todo ensure all read
   if err != nil { quit(err) }
   err = json.Unmarshal(aBuf, iCc)
   if err != nil { quit(err) }
   return aPos, aLenIdx
}

func _readTail(iFd *os.File) (int64, int64) {
   aBuf := make([]byte, 16)
   _, err := iFd.Seek(-16, io.SeekEnd)
   if err != nil { quit(err) }
   _, err = iFd.Read(aBuf)
   if err != nil { quit(err) }
   aStr := string(aBuf)
   aLenIdx, err := strconv.ParseUint(aStr[:8], 16, 0)
   if err != nil { quit(err) }
   aLenCc,  err := strconv.ParseUint(aStr[8:], 16, 0)
   if err != nil { quit(err) }
   return int64(aLenIdx), int64(aLenCc)
}

func _writeIndex(iTd *os.File, iIdx []tIndexEl, iCc []tCcEl) {
   aBuf, err := json.Marshal(iIdx)
   if err != nil { quit(err) }
   _, err = iTd.Write(aBuf)
   if err != nil { quit(err) }
   _writeCc(iTd, iCc, int64(len(aBuf)))
}

func _writeCc(iTd *os.File, iCc []tCcEl, iLenIdx int64) {
   aBuf, err := json.Marshal(iCc)
   if err != nil { quit(err) }
   _, err = iTd.Write(append(aBuf, fmt.Sprintf("%08x%08x", iLenIdx, len(aBuf))...))
   if err != nil { quit(err) }
   err = iTd.Sync()
   if err != nil { quit(err) }
   _, err = iTd.Seek(0, io.SeekStart)
   if err != nil { quit(err) }
}

func _writeMsg(iTd *os.File, iHead *Header, iR io.Reader, iEl *tIndexEl) (*tMsgHead, error) {
   var err error
   var aCw tCrcWriter
   aTee := io.MultiWriter(iTd, &aCw)
   aSize := iHead.DataLen - totalAttach(iHead.SubHead)
   if aSize < 0 {
      return nil, tError("attachment size total exceeds DataLen")
   }
   aHead := tMsgHead{Id:iHead.Id, From:iHead.From, Posted:iHead.Posted, Len:aSize, SubHead:*iHead.SubHead}
   aBuf, err := json.Marshal(aHead)
   if err != nil { quit(err) }
   aLen, err := aTee.Write([]byte(fmt.Sprintf("%04x", len(aBuf))))
   if err != nil { quit(err) }
   if aLen != 4 { quit(tError("json input too long")) }
   _, err = aTee.Write(append(aBuf, '\n'))
   if err != nil { quit(err) }
   if aSize > 0 {
      _, err = io.CopyN(aTee, iR, aSize)
      if err != nil {
         return nil, err //todo only net errors
      }
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
   return &aHead, nil
}

type tDraftless struct {
   fd *os.File
   bufIdx, bufCc []byte
   pos int64
}

func newDraftlessThread(iFd *os.File) *tDraftless {
   o := tDraftless{fd:iFd}
   var aIdx []tIndexEl
   var aCc []tCcEl
   o.pos = _readIndex(o.fd, &aIdx, &aCc)
   aIdx2 := aIdx[:0]
   for a := range aIdx {
      if aIdx[a].Offset == -1 { continue }
      aIdx2 = append(aIdx2, aIdx[a])
   }
   var err error
   o.bufIdx, err = json.Marshal(aIdx2)
   if err != nil { quit(err) }
   o.bufCc, err = json.Marshal(aCc)
   if err != nil { quit(err) }
   return &o
}

func (o *tDraftless) size() int64 { return o.pos + int64(len(o.bufIdx) + len(o.bufCc)) + 16 }

func (o *tDraftless) copy(iW io.Writer) (int64, error) {
   _, err := o.fd.Seek(0, io.SeekStart)
   if err != nil { quit(err) }
   aLen1, err := io.CopyN(iW, o.fd, o.pos)
   if err != nil { return aLen1, err }
   aLen2, err := iW.Write(o.bufIdx)
   if err != nil { return aLen1 + int64(aLen2), err }
   aLen3, err := iW.Write(o.bufCc)
   if err != nil { return aLen1 + int64(aLen2 + aLen3), err }
   aLen4, err := iW.Write([]byte(fmt.Sprintf("%08x%08x", aLen2, aLen3)))
   return aLen1 + int64(aLen2 + aLen3 + aLen4), err
}

func _makeQid(iType byte, iTid, iLms string) string { return string(iType) + iTid +"_"+ iLms }

type tComplete []string

func _parseFtmp(i string) tComplete { return strings.SplitN(i, "_", 5) }

func (o tComplete)  op() string { return o[0] } // transaction type
func (o tComplete) tid() string { return o[1] } // thread id
func (o tComplete) mid() string { return o[2] } // message id
func (o tComplete) lms() string { return o[3] } // local id milliseconds

func (o tComplete) pos() int64 { // thread offset to index
   aPos, err := strconv.ParseInt(o[4], 10, 64)
   if err != nil { quit(err) }
   return aPos
}

type tThreadDoor struct {
   sync.RWMutex
   renamed bool //todo provide new thread id here?
}

func _getThreadDoor(iSvc string, iTid string) *tThreadDoor {
   return getDoorService(iSvc, iTid, func()tDoor{ return &tThreadDoor{} }).(*tThreadDoor)
}

func completeThread(iSvc string, iTempOk string) {
   aRec := _parseFtmp(iTempOk)
   if len(aRec) != 5 {
      fmt.Fprintf(os.Stderr, "completeThread: unexpected file %s%s\n", dirTemp(iSvc), iTempOk)
      return
   }
   var err error
   var aFd, aTd *os.File
   aTd, err = os.Open(dirTemp(iSvc) + iTempOk)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      fmt.Printf("complete %s already removed\n", iTempOk)
      return
   }
   defer aTd.Close()
   fmt.Printf("complete %s\n", iTempOk)
   if aRec.op() == "sc" || aRec.op() == "nr" || aRec.tid() != "" && aRec.tid() != aRec.mid() {
      aTid := aRec.tid(); if aTid == "" { aTid = "_"+ aRec.lms() }
      aFd, err = os.OpenFile(dirThread(iSvc) + aTid, os.O_RDWR, 0600)
      if err != nil { quit(err) }
      defer aFd.Close()
      _, err = aFd.Seek(aRec.pos(), io.SeekStart)
      if err != nil { quit(err) }
   }
   fMsgHead := func() *tMsgHead {
      cMh := _readMsgHead(aTd)
      aTd.Seek(0, io.SeekStart)
      return cMh
   }
   fIdx := func() []tIndexEl {
      var cIdx []tIndexEl
      _readIndex(aTd, &cIdx, nil)
      return cIdx
   }
   fCc := func(cFlag string) []tCcEl {
      var cCc []tCcEl
      if cFlag != "orig" || aRec.tid() == aRec.mid() {
         _readCc(aTd, &cCc)
         aTd.Seek(0, io.SeekStart)
      }
      return cCc
   }
   fFwdSent := func() int {
      cFwd := _getFwd(iSvc, aRec.tid(), "temp")
      if cFwd != nil {
         return len(cFwd)
      }
      return -1
   }
   switch aRec.op() {
   case "sc": _completeStoreConfirm    (iSvc, iTempOk, aFd, aTd, fMsgHead(), fIdx())
   case "sr": _completeStoreReceived   (iSvc, iTempOk, aFd, aTd, fMsgHead(), fCc("orig"))
   case "ss": _completeStoreSent       (iSvc, iTempOk, aFd, aTd, fMsgHead(), fCc("orig"))
   case "ws": _completeStoreDraft      (iSvc, iTempOk, aFd, aTd, fMsgHead())
   case "ds": _completeDeleteDraft     (iSvc, iTempOk, aFd, aTd)
   case "fr": _completeStoreFwdReceived(iSvc, iTempOk,      aTd)
   case "fn": _completeStoreFwdNotify  (iSvc, iTempOk, aFd, aTd, fCc("fwd"))
   case "fs": _completeStoreFwdSent    (iSvc, iTempOk, aFd, aTd, fCc("fwd"), fFwdSent())
   case "nr": _completeTouch           (iSvc, iTempOk, aFd, aTd)
   default:
      fmt.Fprintf(os.Stderr, "completeThread: unexpected op %s%s\n", dirTemp(iSvc), iTempOk)
   }
}
