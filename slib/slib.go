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
   "sync"
)

const kStorageDir = "store/"
const kServiceDir = kStorageDir + "svc/"
const UploadDir   = kStorageDir + "upload/"

var sServicesDoor sync.RWMutex
var sServices = make(map[string]*tService)

var sCrc32c = crc32.MakeTable(crc32.Castagnoli)


type Header struct {
   Op string
   Id, MsgId string
   Uid, NodeId string
   Info string
   From string
   Posted string
   DataLen, DataHead int64
   SubHead struct {
      ThreadId string
      Subject string
   }
}

func (o *Header) Check() bool {
   return true
}

func (o *Header) CheckSub() bool {
   return true
}

type Update struct {
   Op string
   Thread *struct {
      Id string
      Subject string
      Data string
      New bool //temp
   }
}

type SendRecord struct {
   Head Msg
   Data []byte
   Files []string
}

type Msg map[string]interface{}


func Init() {
   err := os.MkdirAll(UploadDir, 0700)
   if err != nil { panic(err) }
   _, err = AddService("test", "localhost:8888", 30)
   if err != nil { panic(err) }
}

func GetServices() (aS []string) {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   for aK, _ := range sServices {
      aS = append(aS, aK)
   }
   return aS
}

type tService struct {
   Name string
   LoginPeriod int // seconds
   Addr string // for Dial()
   Uid string
   Node string
}

func tempDir  (iSvc string) string { return kServiceDir + iSvc + "/temp/"   }
func threadDir(iSvc string) string { return kServiceDir + iSvc + "/thread/" }

func GetData(iSvc string) *tService {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   return sServices[iSvc]
}

func AddService(iSvc, iAddr string, iPeriod int) (*tService, error) {
   var err error
   for _, aDir := range [...]string{tempDir(iSvc), threadDir(iSvc)} {
      err = os.MkdirAll(aDir, 0700)
      if err != nil {
         os.RemoveAll(kServiceDir + iSvc)
         return nil, err
      }
   }
   sServicesDoor.Lock(); defer sServicesDoor.Unlock()
   if sServices[iSvc] != nil {
      return nil, tError(fmt.Sprintf("AddService: name %s already exists", iSvc))
   }
   aSvc := &tService{Name: iSvc, Addr: iAddr, LoginPeriod: iPeriod}
   sServices[iSvc] = aSvc
   return aSvc, nil
}

func GetQueue(iSvc string) ([]*SendRecord, error) {
   aSvc := GetData(iSvc)
   if aSvc == nil {
      return nil, tError(fmt.Sprintf("getqueue: service %s not found", iSvc))
   }
   return nil, nil
}

func HandleMsg(iSvc string, iHead *Header, iData []byte, iR io.Reader) Msg {
   aSvc := GetData(iSvc)
   switch iHead.Op {
   case "registered":
      aSvc.Uid = iHead.Uid
      aSvc.Node = iHead.NodeId
   case "delivery":
      storeReceived(aSvc.Name, iHead, iData, iR)
      sState[iSvc].thread = iHead.SubHead.ThreadId; if sState[iSvc].thread == "" { sState[iSvc].thread = iHead.Id } //temp
      sState[iSvc].msgs = map[string]bool{iHead.Id:true}
   case "ack":
      if iHead.Id == "22" { break }
      storeSaved(iSvc, iHead)
      if iHead.Id[0] == '_' {
         sState[iSvc].thread = iHead.MsgId
      }
      delete(sState[iSvc].msgs, iHead.Id)
      sState[iSvc].msgs[iHead.MsgId] = true
      iHead.Id = iHead.MsgId
   }
   return Msg{"op":iHead.Op, "id":iHead.Id, "threadid":iHead.SubHead.ThreadId}
}

func HandleUpdt(iSvc string, iUpdt *Update) (Msg, *SendRecord) {
   switch iUpdt.Op {
   case "thread_ohi":
      aData, _ := json.Marshal(Msg{"ThreadId":sState[iSvc].thread})
      aHeadLen := len(aData)
      aData = append(aData, "ohi there"...)
      aSrec := &SendRecord{Head: Msg{"Op":7, "Id":"22", "DataLen":len(aData), "DataHead":aHeadLen,
                           "For":[]Msg{{"Id":GetData(iSvc).Uid, "Type":1}} }, Data:aData}
      return Msg{"op":iUpdt.Op, "etc":"posting msg"}, aSrec
   case "thread_save":
      if iUpdt.Thread.New {
         sState[iSvc].thread = ""
      }
      writeSaved(iSvc, iUpdt)
      if iUpdt.Thread.New {
         sState[iSvc].thread = iUpdt.Thread.Id
         sState[iSvc].msgs = map[string]bool{}
      }
      sState[iSvc].msgs[iUpdt.Thread.Id] = true
      return Msg{"op":iUpdt.Op, "etc":"save reply", "id":iUpdt.Thread.Id}, nil
   case "thread_discard":
      deleteSaved(iSvc, iUpdt)
      if iUpdt.Thread.Id[0] == '_' {
         sState[iSvc].thread = ""
      }
      delete(sState[iSvc].msgs, iUpdt.Thread.Id)
      return Msg{"op":iUpdt.Op, "etc":"discard"}, nil
   case "thread_send":
      if iUpdt.Thread.Id == "" {
         return Msg{"op":iUpdt.Op, "etc":"no op"}, nil
      }
      aSrec := &SendRecord{Head: Msg{"Op":7, "Id":iUpdt.Thread.Id, "DataLen":1,
                  "For":[]Msg{{"Id":"LG3KCJGZPVVNDPV6%JRK4H6FC6LS8P37", "Type":1}} }, Data:[]byte{'1'}}
      return Msg{"op":iUpdt.Op, "etc":"send reply"}, aSrec
   case "thread_close":
      sState[iSvc].msgs[iUpdt.Thread.Id] = false
      return Msg{"op":iUpdt.Op, "etc":"closed"}, nil
   }
   return Msg{"op":iUpdt.Op, "etc":"unknown op"}, nil
}

func RecvFile(iSvc, iId string, iData []byte, iStream io.Reader, iLen int64) error {
   if iSvc != "" && GetData(iSvc) == nil {
      return tError(fmt.Sprintf("recvfile: service %s not found", iSvc))
   }
   aDir := UploadDir; if iSvc != "" { aDir = tempDir(iSvc) }
   aFd, err := os.OpenFile(aDir+iId, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
   if err != nil { return err }
   defer aFd.Close()
   for aPos, aLen := 0,0; aPos < len(iData); aPos += aLen {
      aLen, err = aFd.Write(iData[aPos:])
      if err != nil && err != io.ErrShortWrite { return err }
   }
   _,err = io.CopyN(aFd, iStream, iLen - int64(len(iData)))
   if err != nil { return err }
   err = aFd.Sync()
   if err == nil && aDir == UploadDir {
      err = syncDir(UploadDir)
   }
   return err
}

func syncDir(iPath string) error {
   aFd, err := os.Open(iPath)
   if err != nil { return err }
   err = aFd.Sync()
   aFd.Close()
   return err
}

type tError string
func (o tError) Error() string { return string(o) }
