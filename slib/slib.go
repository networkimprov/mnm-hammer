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
   "strings"
   "sync"
)

const kStorageDir = "store/"
const kServiceDir = kStorageDir + "svc/"
const UploadDir   = kStorageDir + "upload/"

var sServicesDoor sync.RWMutex
var sServices = make(map[string]*tService)
var sServiceStartFn func(string)

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
   Service *tService
}

type SendRecord struct {
   Head Msg
   Data []byte
   Files []string
}

type Msg map[string]interface{}


func Init(iFn func(string)) {
   var err error
   for _, aDir := range [...]string{UploadDir, kServiceDir} {
      err = os.MkdirAll(aDir, 0700)
      if err != nil { panic(err) }
   }

   var aFd *os.File
   aFd, err = os.Open(kServiceDir)
   if err != nil { panic(err) }
   aSvcs, err := aFd.Readdirnames(0)
   aFd.Close()
   if err != nil { panic(err) }

   for _, aSvc := range aSvcs {
      if strings.HasSuffix(aSvc, ".tmp") {
         err = os.RemoveAll(svcDir(aSvc))
         if err != nil { panic(err) }
         continue
      }
      completePending(aSvc)
      err = os.Rename(cfgFile(aSvc) + ".tmp", cfgFile(aSvc))
      if err != nil {
         if os.IsNotExist(err) {
            // ok
         } else if os.IsExist(err) {
            err = os.Remove(cfgFile(aSvc) + ".tmp")
            if err != nil { panic(err) }
         } else { panic(err) }
      }
      aService := &tService{}
      aFd, err = os.Open(cfgFile(aSvc))
      if err != nil { panic(err) }
      err = json.NewDecoder(aFd).Decode(aService)
      aFd.Close()
      if err != nil { panic(err) }
      sServices[aSvc] = aService
      sState[aSvc] = &tState{}
   }
   if sServices["test"] == nil {
      err = addService(&tService{Name:"test", Addr:"localhost:8888", LoginPeriod:30})
      if err != nil { panic(err) }
   }
   sServiceStartFn = iFn
}

func GetServices() (aS []string) {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   for aK, _ := range sServices {
      aS = append(aS, aK)
   }
   return aS
}

func GetData(iSvc string) *tService {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   return sServices[iSvc]
}

type tService struct {
   Name string
   Description string
   LoginPeriod int // seconds
   Addr string // for Dial()
   Uid string
   Node string
}

func svcDir   (iSvc string) string { return kServiceDir + iSvc + "/"        }
func tempDir  (iSvc string) string { return kServiceDir + iSvc + "/temp/"   }
func threadDir(iSvc string) string { return kServiceDir + iSvc + "/thread/" }
func cfgFile  (iSvc string) string { return kServiceDir + iSvc + "/config"  }

func addService(iService *tService) error {
   var err error
   if len(iService.Name) < 5 || strings.HasSuffix(iService.Name, ".tmp") {
      return tError(fmt.Sprintf("AddService: name %s not valid", iService.Name))
   }
   sServicesDoor.Lock(); defer sServicesDoor.Unlock()
   if sServices[iService.Name] != nil {
      return tError(fmt.Sprintf("AddService: name %s already exists", iService.Name))
   }
   aTemp := iService.Name + ".tmp"
   defer os.RemoveAll(svcDir(aTemp))
   for _, aDir := range [...]string{tempDir(aTemp), threadDir(aTemp)} {
      err = os.MkdirAll(aDir, 0700)
      if err != nil { return err }
   }
   aFd, err := os.OpenFile(cfgFile(aTemp), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { return err }
   defer aFd.Close()
   err = json.NewEncoder(aFd).Encode(iService)
   if err != nil { return err }
   err = aFd.Sync()
   if err != nil { return err }

   err = syncDir(svcDir(aTemp))
   if err != nil { return err }
   err = os.Rename(svcDir(aTemp), svcDir(iService.Name))
   if err != nil { return err }

   sServices[iService.Name] = iService
   sState[iService.Name] = &tState{}
   if sServiceStartFn != nil {
      sServiceStartFn(iService.Name)
   }
   return nil
}

func updateService(iService *tService) error {
   var err error
   sServicesDoor.Lock(); defer sServicesDoor.Unlock()
   if sServices[iService.Name] == nil {
      return tError(fmt.Sprintf("UpdateService: %s not found", iService.Name))
   }
   aTemp := cfgFile(iService.Name) + ".tmp"
   defer os.Remove(aTemp)
   aFd, err := os.OpenFile(aTemp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { return err }
   defer aFd.Close()
   err = json.NewEncoder(aFd).Encode(iService)
   if err != nil { return err }
   err = aFd.Sync()
   if err != nil { return err }

   err = syncDir(svcDir(iService.Name))
   if err != nil { return err }
   err = os.Remove(cfgFile(iService.Name))
   if err != nil { return err }
   err = os.Rename(aTemp, cfgFile(iService.Name))
   if err != nil {
      fmt.Fprintf(os.Stderr, "UpdateService %s: transaction failed: %s\n", iService.Name, err.Error())
      os.Exit(1) // skip deferred Remove(aTemp)
   }

   sServices[iService.Name] = iService
   return nil
}

func GetQueue(iSvc string) ([]*SendRecord, error) {
   aSvc := GetData(iSvc)
   if aSvc == nil {
      return nil, tError(fmt.Sprintf("getqueue: service %s not found", iSvc))
   }
   return nil, nil
}

func HandleMsg(iSvc string, iHead *Header, iData []byte, iR io.Reader) Msg {
   switch iHead.Op {
   case "registered":
      aNewSvc := *GetData(iSvc)
      aNewSvc.Uid = iHead.Uid
      aNewSvc.Node = iHead.NodeId
      err := updateService(&aNewSvc)
      if err != nil { return Msg{"op":iHead.Op, "err":err.Error()} }
   case "delivery":
      storeReceived(iSvc, iHead, iData, iR)
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
   case "service_add":
      err := addService(iUpdt.Service)
      if err != nil {
         return Msg{"op":iUpdt.Op, "err":err.Error()}, nil
      }
      return Msg{"op":iUpdt.Op, "etc":"add service"}, nil
   case "service_update":
      err := updateService(iUpdt.Service)
      if err != nil {
         return Msg{"op":iUpdt.Op, "err":err.Error()}, nil
      }
      return Msg{"op":iUpdt.Op, "etc":"update service"}, nil
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

func Upload(iId string, iR io.Reader, iLen int64) error {
   aFd, err := os.OpenFile(UploadDir+iId, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
   if err != nil { return err }
   defer aFd.Close()
   _, err = io.CopyN(aFd, iR, iLen)
   if err != nil { return err }
   err = aFd.Sync()
   if err != nil { return err }
   err = syncDir(UploadDir)
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
