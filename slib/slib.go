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
   "os"
   "sync"
)

const kStorageDir = "store/"
const kServiceDir = kStorageDir + "svc/"
const UploadDir   = kStorageDir + "upload/"

var sServicesDoor sync.RWMutex
var sServices = make(map[string]*tService)


type Header struct {
   Op string
   Id string
   Uid, NodeId string
   Info string
   From string
   DataLen, DataHead int64
   SubHead struct {
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

func tempDir(iSvc string) string { return kServiceDir + iSvc + "/temp/" }

func GetData(iSvc string) *tService {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   return sServices[iSvc]
}

func AddService(iSvc, iAddr string, iPeriod int) (*tService, error) {
   err := os.MkdirAll(tempDir(iSvc), 0700)
   if err != nil { return nil, err }
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

func HandleMsg(iSvc string, iHead *Header) Msg {
   aSvc := GetData(iSvc)
   if iHead.Op == "registered" {
      aSvc.Uid = iHead.Uid
      aSvc.Node = iHead.NodeId
   }
   return Msg{"op":iHead.Op, "id":iHead.Id}
}

func HandleUpdt(iSvc string, iUpdt *Update) (Msg, *SendRecord) {
   aSrec := &SendRecord{Head: Msg{"Op":7, "Id":"22", "DataLen":3,
                        "For":[]Msg{{"Id":GetData(iSvc).Uid, "Type":1}} }, Data:[]byte("ohi")}
   return Msg{"op":iUpdt.Op, "etc":"posting msg"}, aSrec
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
