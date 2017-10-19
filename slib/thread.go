// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package slib

import (
   "hash/crc32"
   "fmt"
   "io"
   "encoding/json"
   "os"
   "path"
   "strconv"
   "strings"
   "time"
)

type tIndexEl struct {
   Id string
   Offset int64
   Size int64
   From string
   Date string
   Subject string
   Checksum uint32
}

func makeIndexEl(iHead *Header, iPos int64) tIndexEl {
   return tIndexEl{Id:iHead.Id, From:iHead.From, Date:iHead.Posted, Offset:iPos,
                   Subject:iHead.SubHead.Subject}
}

var sState = make(map[string]*tState)
type tState struct { // temp
   thread string
   msgs map[string]bool
}

func GetMsgIdx(iSvc string, iClientId string) []tIndexEl {
   if sState[iSvc].thread == "" { return nil }
   aFd, err := os.Open(threadDir(iSvc) + sState[iSvc].thread)
   if err != nil { panic(err) }
   defer aFd.Close()
   var aIdx []tIndexEl
   _ = readIndex(aFd, &aIdx)
   return aIdx
}

func WriteOpenMsgs(iW io.Writer, iSvc string, iClientId string, iId string) {
   if sState[iSvc].thread == "" { return }
   if iId != "" {
      sState[iSvc].msgs[iId] = true
   }
   aFd, err := os.Open(threadDir(iSvc) + sState[iSvc].thread)
   if err != nil { panic(err) }
   defer aFd.Close()
   var aIdx []tIndexEl
   _ = readIndex(aFd, &aIdx)
   for a, _ := range aIdx {
      if iId != "" && aIdx[a].Id == iId || iId == "" && sState[iSvc].msgs[aIdx[a].Id] {
         var aRd, aXd *os.File
         if aIdx[a].Offset >= 0 {
            _, err = aFd.Seek(aIdx[a].Offset, io.SeekStart)
            if err != nil { panic(err) }
            aXd = aFd
         } else {
            aRd, err = os.Open(threadDir(iSvc) + aIdx[a].Id)
            if err != nil { panic(err) }
            defer aRd.Close()
            aXd = aRd
         }
         _, err = io.CopyN(iW, aXd, aIdx[a].Size)
         if err != nil { panic(err) }
         if iId != "" {
            break
         }
      }
   }
}

func storeReceived(iSvc string, iHead *Header, iData []byte, iR io.Reader) {
   var err error
   aThreadId := iHead.SubHead.ThreadId; if aThreadId == "" { aThreadId = iHead.Id }
   aOrig := threadDir(iSvc) + aThreadId
   aTempOk := tempDir(iSvc) + aThreadId + "_" + iHead.Id + "_sr__"
   aTemp := aTempOk + ".tmp"

   if iHead.SubHead.ThreadId == "" {
      _, err = os.Lstat(aOrig)
      if err == nil {
         fmt.Fprintf(os.Stderr, "storeReceived %s: thread %s already stored\n", iSvc, iHead.Id)
         return
      }
   }
   var aTd, aFd *os.File
   var aIdx []tIndexEl = []tIndexEl{{}}
   var aPos int64
   var aCopyLen int64
   aEl := 0
   if aThreadId != iHead.Id {
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil {
         fmt.Fprintf(os.Stderr, "storeReceived %s: thread %s not found\n", iSvc, aThreadId)
         return
      }
      defer aFd.Close()
      aPos = readIndex(aFd, &aIdx)
      aEl = len(aIdx)
      aIdx = append(aIdx, tIndexEl{})
      for a, _ := range aIdx {
         if aIdx[a].Id == iHead.Id {
            fmt.Fprintf(os.Stderr, "storeReceived %s: msg %s already stored\n", iSvc, iHead.Id)
            return
         }
         if aIdx[a].Id > iHead.Id {
            aCopyLen = aPos - aIdx[a].Offset
            aPos = aIdx[a].Offset
            aEl = a
            copy(aIdx[a+1:], aIdx[a:])
            _, err = aFd.Seek(aPos, io.SeekStart)
            if err != nil { panic(err) }
            break
         }
      }
   }
   aIdx[aEl] = makeIndexEl(iHead, aPos)
   aTempOk += fmt.Sprint(aPos)

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { panic(err) }
   defer aTd.Close()
   writeMsgTemp(aTd, iHead, iData, iR, aIdx, aEl)
   if aCopyLen > 0 {
      _, err = io.CopyN(aTd, aFd, aCopyLen)
      if err != nil { panic(err) }
      _, err = aFd.Seek(aPos, io.SeekStart)
      if err != nil { panic(err) }
   }
   writeIndex(aTd, aIdx)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { panic(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { panic(err) }
   completeStoreReceived(iSvc, path.Base(aTempOk), aFd, aTd)
}

func completeStoreReceived(iSvc string, iTmp string, iFd, iTd *os.File) {
   var err error
   aRec := parseTempOk(iTmp)
   aTempOk := tempDir(iSvc) + iTmp

   if aRec.tid() == aRec.mid() {
      err = os.Link(aTempOk, threadDir(iSvc) + aRec.tid())
      if err != nil { panic(err) }
      err = syncDir(threadDir(iSvc))
      if err != nil { panic(err) }
   } else {
      _, err = io.Copy(iFd, iTd) // iFd has correct pos from readIndex
      if err != nil { panic(err) }
      err = iFd.Sync()
      if err != nil { panic(err) }
   }
   err = os.Remove(aTempOk)
   if err != nil { panic(err) }
}

func storeSaved(iSvc string, iHead *Header) {
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
         fmt.Fprintf(os.Stderr, "storeSaved %s: saved file was cleared %s\n", iSvc, iHead.Id)
         return
      }
      panic(err)
   }
   defer aSd.Close()
   var aJson struct { Len int64 }
   parseHeader(aSd, &aJson)
   aHead := Header{Id:iHead.MsgId, From:GetData(iSvc).Uid, Posted:iHead.Posted, DataLen:aJson.Len,
                   SubHead:iHead.SubHead}

   var aIdx []tIndexEl
   var aTd, aFd *os.File
   var aPos int64
   if aId.tid() != iHead.MsgId {
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil { panic(err) }
      defer aFd.Close()
      aPos = readIndex(aFd, &aIdx)
      a := -1
      for a, _ = range aIdx {
         if aIdx[a].Id == iHead.Id { break }
      }
      aIdx = aIdx[:a + copy(aIdx[a:], aIdx[a+1:])]
   }
   aIdx = append(aIdx, makeIndexEl(&aHead, aPos))
   aTempOk += fmt.Sprint(aPos)

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { panic(err) }
   defer aTd.Close()
   writeMsgTemp(aTd, &aHead, nil, aSd, aIdx, len(aIdx)-1)
   writeIndex(aTd, aIdx)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { panic(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { panic(err) }
   completeStoreSaved(iSvc, path.Base(aTempOk), aFd, aTd)
}

func completeStoreSaved(iSvc string, iTmp string, iFd, iTd *os.File) {
   aRec := parseTempOk(iTmp)
   err := os.Remove(threadDir(iSvc) + aRec.tid() + "_" + aRec.sid())
   if err != nil && !os.IsNotExist(err) { panic(err) }
   completeStoreReceived(iSvc, iTmp, iFd, iTd)
}

func writeSaved(iSvc string, iUpdt *Update) {
   if iUpdt.Thread.Id == "" {
      iUpdt.Thread.Id = fmt.Sprintf("%s_%012x", sState[iSvc].thread, time.Now().UTC().UnixNano() / 1e6) // milliseconds
   }
   aId := parseSaveId(iUpdt.Thread.Id)
   aOrig := threadDir(iSvc) + aId.tid()
   aTempOk := tempDir(iSvc) + aId.tid() + "__ws_" + aId.sid() + "_"
   aTemp := aTempOk + ".tmp"
   aData := []byte(iUpdt.Thread.Data)
   var err error

   var aIdx []tIndexEl
   var aTd, aFd *os.File
   var aPos int64
   aEldata := tIndexEl{Id:iUpdt.Thread.Id, Date:dateRFC3339(), Subject:iUpdt.Thread.Subject, Offset:-1}
   aEl := -1
   if aId.tid() != "" {
      aFd, err = os.OpenFile(aOrig, os.O_RDWR, 0600)
      if err != nil { panic(err) }
      defer aFd.Close()
      aPos = readIndex(aFd, &aIdx)
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
   if err != nil { panic(err) }
   defer aTd.Close()
   aHead := Header{Id:iUpdt.Thread.Id, From:"self", Posted:"draft", DataLen:int64(len(aData))}
   writeMsgTemp(aTd, &aHead, aData, nil, aIdx, aEl) //todo stream from client
   writeIndex(aTd, aIdx)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { panic(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { panic(err) }
   completeWriteSaved(iSvc, path.Base(aTempOk), aFd, aTd)
}

func completeWriteSaved(iSvc string, iTmp string, iFd, iTd *os.File) {
   var err error
   aRec := parseTempOk(iTmp)
   aSave := threadDir(iSvc) + aRec.tid() + "_" + aRec.sid()
   aTempOk := tempDir(iSvc) + iTmp

   err = os.Remove(aSave)
   if err != nil && !os.IsNotExist(err) { panic(err) }
   if aRec.op() == "ws" {
      err = os.Link(aTempOk, aSave)
      if err != nil { panic(err) }
   }
   err = syncDir(threadDir(iSvc))
   if err != nil { panic(err) }

   if aRec.tid() != "" {
      _ = readIndex(iTd, nil)
      err = iFd.Truncate(aRec.pos())
      if err != nil { panic(err) }
      _, err = io.Copy(iFd, iTd)
      if err != nil { panic(err) }
      err = iFd.Sync()
      if err != nil { panic(err) }
   }
   err = os.Remove(aTempOk)
   if err != nil { panic(err) }
}

func deleteSaved(iSvc string, iUpdt *Update) {
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
      if err != nil { panic(err) }
      defer aFd.Close()
      aPos = readIndex(aFd, &aIdx)
      a := -1
      for a, _ = range aIdx {
         if aIdx[a].Id == iUpdt.Thread.Id { break }
      }
      aIdx = aIdx[:a + copy(aIdx[a:], aIdx[a+1:])]
   }
   aTempOk += fmt.Sprint(aPos)

   aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { panic(err) }
   defer aTd.Close()
   writeIndex(aTd, aIdx)
   err = os.Rename(aTemp, aTempOk)
   if err != nil { panic(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { panic(err) }
   completeDeleteSaved(iSvc, path.Base(aTempOk), aFd, aTd)
}

func completeDeleteSaved(iSvc string, iTmp string, iFd, iTd *os.File) {
   completeWriteSaved(iSvc, iTmp, iFd, iTd)
}

func dateRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

func parseHeader(iFd *os.File, iHead interface{}) {
   aBuf := make([]byte, 65536)
   _, err := iFd.Read(aBuf[:4])
   if err != nil { panic(err) }
   aUi, _ := strconv.ParseUint(string(aBuf[:4]), 16, 0)
   _, err = iFd.Read(aBuf[:aUi+1])
   if err != nil { panic(err) }
   err = json.Unmarshal(aBuf[:aUi], iHead)
   if err != nil { panic(err) }
}

type tSaveId []string
func parseSaveId(i string) tSaveId { return strings.SplitN(i, "_", 2) }

func (o tSaveId) tidSet(i string) { o[0] = i }
func (o tSaveId) tid() string { return o[0] }
func (o tSaveId) sid() string { return o[1] }

type tComplete []string
func parseTempOk(i string) tComplete { return strings.SplitN(i, "_", 5) }

func (o tComplete) tid() string { return o[0] } // thread id
func (o tComplete) mid() string { return o[1] } // message id
func (o tComplete)  op() string { return o[2] } // transaction type
func (o tComplete) sid() string { return o[3] } // saved id

func (o tComplete) pos() int64 { // thread offset to index
   aPos, err := strconv.ParseInt(o[4], 10, 64)
   if err != nil { panic(err) }
   return aPos
}

func readIndex(iFd *os.File, iIdx *[]tIndexEl) int64 {
   _, err := iFd.Seek(-8, io.SeekEnd)
   if err != nil { panic(err) }
   aBuf := make([]byte, 8)
   _, err = iFd.Read(aBuf)
   if err != nil { panic(err) }
   aIdxLen, err := strconv.ParseUint(string(aBuf), 16, 0)
   if err != nil { panic(err) }
   aPos, err := iFd.Seek(-8 - int64(aIdxLen), io.SeekEnd)
   if err != nil { panic(err) }
   if iIdx == nil {
      return aPos
   }
   aBuf = make([]byte, aIdxLen)
   _, err = iFd.Read(aBuf) //todo ensure all read
   if err != nil { panic(err) }
   err = json.Unmarshal(aBuf, iIdx)
   if err != nil { panic(err) }
   aPos, err = iFd.Seek(-8 - int64(aIdxLen), io.SeekEnd)
   if err != nil { panic(err) }
   return aPos
}

func writeIndex(iTd *os.File, iIdx []tIndexEl) {
   aBuf, err := json.Marshal(iIdx)
   if err != nil { panic(err) }
   _, err = iTd.Write(append(aBuf, fmt.Sprintf("%08x", len(aBuf))...))
   if err != nil { panic(err) }
   err = iTd.Sync()
   if err != nil { panic(err) }
   _, err = iTd.Seek(0, io.SeekStart)
   if err != nil { panic(err) }
}

type tCrcWriter struct { sum uint32 }

func (o *tCrcWriter) Write(i []byte) (int, error) {
   o.sum = crc32.Update(o.sum, sCrc32c, i)
   return len(i), nil
}

func writeMsgTemp(iTd *os.File, iHead *Header, iData []byte, iR io.Reader,
                  iIdx []tIndexEl, iEl int) {
   var err error
   var aCw tCrcWriter
   aTee := io.MultiWriter(iTd, &aCw)
   aBuf, err := json.Marshal(Msg{"Id":iHead.Id, "From":iHead.From, "Posted":iHead.Posted,
                                 "Len":iHead.DataLen})
   if err != nil { panic(err) }
   aLen, err := aTee.Write([]byte(fmt.Sprintf("%04x", len(aBuf))))
   if err != nil { panic(err) }
   if aLen != 4 { panic("json input too long") }
   _, err = aTee.Write(append(aBuf, '\n'))
   if err != nil { panic(err) }
   if iHead.DataLen > 0 {
      _, err = aTee.Write(iData)
      if err != nil { panic(err) }
      if iR != nil {
         _, err = io.CopyN(aTee, iR, iHead.DataLen - int64(len(iData)))
         if err != nil { panic(err) }
      }
   }
   _, err = aTee.Write([]byte{'\n'})
   if err != nil { panic(err) }
   iIdx[iEl].Checksum = aCw.sum
   iIdx[iEl].Size, err = iTd.Seek(0, io.SeekCurrent)
   if err != nil { panic(err) }
}

func completePending(iSvc string) {
   aTmps, err := readDirNames(tempDir(iSvc))
   if err != nil { panic(err) }

   for _, aTmp := range aTmps {
      if strings.HasSuffix(aTmp, ".tmp") {
         err = os.Remove(tempDir(iSvc) + aTmp)
         if err != nil { panic(err) }
      } else {
         aRec := parseTempOk(aTmp)
         if len(aRec) != 5 {
            fmt.Fprintf(os.Stderr, "completePending: unexpected file %s%s\n", tempDir(iSvc), aTmp)
            continue
         }
         var aFd, aTd *os.File
         if aRec.tid() != "" && aRec.tid() != aRec.mid() {
            aFd, err = os.OpenFile(threadDir(iSvc)+aRec.tid(), os.O_RDWR, 0600)
            if err != nil { panic(err) }
            defer aFd.Close()
            _, err = aFd.Seek(aRec.pos(), io.SeekStart)
            if err != nil { panic(err) }
         }
         aTd, err = os.Open(tempDir(iSvc)+aTmp)
         if err != nil { panic(err) }
         defer aTd.Close()
         switch aRec.op() {
         case "sr":
            completeStoreReceived(iSvc, aTmp, aFd, aTd)
         case "ss":
            completeStoreSaved(iSvc, aTmp, aFd, aTd)
         case "ws":
            completeWriteSaved(iSvc, aTmp, aFd, aTd)
         case "ds":
            completeDeleteSaved(iSvc, aTmp, aFd, aTd)
         default:
            fmt.Fprintf(os.Stderr, "completePending: unexpected op %s%s\n", tempDir(iSvc), aTmp)
         }
      }
   }
}

