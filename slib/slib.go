// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package slib

import (
   "runtime/debug"
   "hash/crc32"
   "fmt"
   "io"
   "encoding/json"
   "os"
   "path"
   "strings"
   "sync"
   "time"
)

const kStorageDir = "store/"
const kServiceDir = kStorageDir + "svc/"
const kStateDir   = kStorageDir + "state/"
const UploadDir   = kStorageDir + "upload/"
const kUploadTmp  = UploadDir   + "temp/"
const kFormDir    = kStorageDir + "form/"
const kFormRegDir = kStorageDir + "reg-cache/"

var sServicesDoor sync.RWMutex
var sServices = make(map[string]*tService)
var sServiceStartFn func(string)

var sCrc32c = crc32.MakeTable(crc32.Castagnoli)


type Header struct {
   Op string
   Error string
   Id, MsgId string
   Uid, NodeId string
   Info string
   From string
   Posted string
   DataLen, DataHead int64
   SubHead tHeader2
}

type tHeader2 struct {
   ThreadId string
   For []tHeaderFor
   Subject string
   Attach []tHeader2Attach `json:",omitempty"`
   isSaved bool
}

type tHeader2Attach struct {
   Name string
   Size int64 `json:",omitempty"`
   Ffn string `json:",omitempty"`
}

func (o *tHeader2) setWrite(iThreadId string, i *Update, iSvc string) {
   o.ThreadId = iThreadId
   o.For = i.Thread.For
   o.Subject = i.Thread.Subject
   o.Attach = savedAttach(iSvc, i)
   o.isSaved = true
}

func (o *tHeader2) setStore(iThreadId string) {
   o.ThreadId = iThreadId
   o.Attach = sentAttach(o.Attach)
   o.isSaved = true
}

func (o *Header) Check() bool {
   return true
}

func (o *Header) CheckSub() bool {
   return true
}

type tHeaderFor struct { Id string; Type int8 }

type Update struct {
   Op string
   Thread *struct {
      Id string
      For []tHeaderFor
      Subject string
      Data string
      Attach []struct{ Name, Ffn string }
      FormFill map[string]string
      New bool
   }
   Navigate *struct {
      History int
   }
   Tab *struct {
      Type int8
      Term string
      PosFor int8
      Pos int
   }
   Service *tService
}

type SendRecord struct { SaveId string }

type Msg map[string]interface{}


func Init(iFn func(string)) {
   for _, aDir := range [...]string{kUploadTmp, kServiceDir, kStateDir, kFormDir} {
      err := os.MkdirAll(aDir, 0700)
      if err != nil { quit(err) }
   }
   initUpload()
   initForms()
   initStates()
   initServices(iFn)
}

func initServices(iFn func(string)) {
   var err error
   aSvcs, err := readDirNames(kServiceDir)
   if err != nil { quit(err) }

   for _, aSvc := range aSvcs {
      if strings.HasSuffix(aSvc, ".tmp") {
         err = os.RemoveAll(svcDir(aSvc))
         if err != nil { quit(err) }
         continue
      }
      mktreeService(aSvc)
      completePending(aSvc)
      aService := &tService{}
      var aFd *os.File
      aFd, err = os.Open(cfgFile(aSvc))
      if err != nil { quit(err) }
      err = json.NewDecoder(aFd).Decode(aService)
      aFd.Close()
      if err != nil { quit(err) }
      sServices[aSvc] = aService
   }
   if sServices["test"] == nil {
      err = addService(&tService{Name:"test", Addr:"localhost:8888", LoginPeriod:30})
      if err != nil { quit(err) }
   }
   sServiceStartFn = iFn
}

func completePending(iSvc string) {
   err := resolveTmpFile(cfgFile(iSvc) + ".tmp")
   if err != nil { quit(err) }

   aTmps, err := readDirNames(tempDir(iSvc))
   if err != nil { quit(err) }

   for _, aTmp := range aTmps {
      if strings.HasSuffix(aTmp, ".tmp") {
         defer os.Remove(tempDir(iSvc) + aTmp)
      } else if strings.HasSuffix(aTmp, ".atc") {
         // ok
      } else {
         completeThread(iSvc, aTmp)
      }
   }
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

func getUriService(iSvc string) string {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   aSvc := sServices[iSvc]
   return aSvc.Addr +"/"+ aSvc.Uid +"/"
}

type tService struct {
   Name string
   Description string
   LoginPeriod int // seconds
   Addr string // for Dial()
   Uid string
   Node string
}

func mktreeService(iSvc string) {
   for _, aDir := range [...]string{tempDir(iSvc), threadDir(iSvc), attachDir(iSvc), formDir(iSvc)} {
      err := os.MkdirAll(aDir, 0700)
      if err != nil { quit(err) }
   }
}

func svcDir   (iSvc string) string { return kServiceDir + iSvc + "/"        }
func tempDir  (iSvc string) string { return kServiceDir + iSvc + "/temp/"   }
func threadDir(iSvc string) string { return kServiceDir + iSvc + "/thread/" }
func attachDir(iSvc string) string { return kServiceDir + iSvc + "/attach/" }
func formDir  (iSvc string) string { return kServiceDir + iSvc + "/form/"   }
func cfgFile  (iSvc string) string { return kServiceDir + iSvc + "/config"  }

func addService(iService *tService) error {
   var err error
   if len(iService.Name) < 4 || strings.HasSuffix(iService.Name, ".tmp") {
      return tError(fmt.Sprintf("AddService: name %s not valid", iService.Name))
   }
   sServicesDoor.Lock(); defer sServicesDoor.Unlock()
   if sServices[iService.Name] != nil {
      return tError(fmt.Sprintf("AddService: name %s already exists", iService.Name))
   }
   aTemp := iService.Name + ".tmp"
   defer os.RemoveAll(svcDir(aTemp))
   mktreeService(aTemp)
   err = writeJsonFile(cfgFile(aTemp), iService)
   if err != nil { quit(err) }

   err = syncDir(svcDir(aTemp))
   if err != nil { quit(err) }
   err = os.Rename(svcDir(aTemp), svcDir(iService.Name))
   if err != nil { quit(err) }

   sServices[iService.Name] = iService
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
   err = storeFile(cfgFile(iService.Name), iService)
   if err != nil { quit(err) }

   sServices[iService.Name] = iService
   return nil
}

func GetQueue(iSvc string) ([]*SendRecord, error) {
   return nil, nil
}

func HandleMsg(iSvc string, iHead *Header, iData []byte, iR io.Reader) (
               aMsg Msg, aFn func(*ClientState)) {
   aMsg = Msg{"op":iHead.Op}
   switch iHead.Op {
   case "registered":
      aNewSvc := *GetData(iSvc)
      aNewSvc.Uid = iHead.Uid
      aNewSvc.Node = iHead.NodeId
      err := updateService(&aNewSvc)
      if err != nil { aMsg["err"] = err.Error() }
   case "delivery":
      err := storeReceived(iSvc, iHead, iData, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleMsg %s: delivery error %s\n", iSvc, err.Error())
         return nil, nil
      }
      if iHead.SubHead.ThreadId == "" { // temp
         aFn = func(c *ClientState) { c.addThread(iHead.Id, iHead.Id) }
      } else {
         aFn = func(c *ClientState) {
            if c.getThread() == iHead.SubHead.ThreadId { c.openMsg(iHead.Id, true) }
         }
      }
      aMsg["id"] = iHead.Id
   case "ack":
      if iHead.Id == "_22" { break }
      aMsg["msgid"] = iHead.MsgId
      if iHead.Error != "" {
         aMsg["err"] = iHead.Error
         break
      }
      storeSaved(iSvc, iHead)
      if iHead.Id[0] == '_' {
         aFn = func(c *ClientState) { c.renameThread(iHead.Id, iHead.MsgId) }
      } else {
         aTid := parseSaveId(iHead.Id).tid()
         aFn = func(c *ClientState) { c.renameMsg(aTid, iHead.Id, iHead.MsgId) }
      }
   }
   return aMsg, aFn
}

func HandleUpdt(iSvc string, iState *ClientState, iUpdt *Update) (
                aMsg Msg, aSrec *SendRecord, aFn func(*ClientState)) {
   aMsg = Msg{"op":iUpdt.Op}
   switch iUpdt.Op {
   case "service_add":
      err := addService(iUpdt.Service)
      if err != nil {
         aMsg["err"] = err.Error()
      }
   case "service_update":
      err := updateService(iUpdt.Service)
      if err != nil {
         aMsg["err"] = err.Error()
      }
   case "thread_ohi":
      aTid := iState.getThread()
      if len(aTid) > 0 && aTid[0] == '_' { break }
      aFd, err := os.OpenFile(threadDir(iSvc) + "_22", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
      if err != nil { quit(err) }
      aData := []byte("ohi there")
      aHead := Header{DataLen:int64(len(aData)), SubHead:
               tHeader2{ThreadId:aTid, isSaved:true, For:
               []tHeaderFor{{Id:GetData(iSvc).Uid, Type:1}}, Attach:
               []tHeader2Attach{{Name:"upload/trial"},
                  {Name:"r:abc", Size:80, Ffn:"localhost:8888/5X8SZWGW7MLR+4GNB1LF+P8YGXCZF4BN/abc"},
                  {Name:"form/trial", Ffn:"form-reg.github.io/cat/trial"} }}}
      aForm := map[string]string{"abc":
         `{"nr":1, "so":"s", "bd":true, "or":{ "anr":[[1,2],[1,2]], "aso":["s","s","s"] }}`}
      writeMsgTemp(aFd, &aHead, aData, nil, []tIndexEl{{}}, 0)
      writeFormFillAttach(aFd, &aHead.SubHead, aForm, &tIndexEl{})
      aFd.Close()
      os.Mkdir(attachSub(iSvc, "_22"), 0700)
      os.Link(UploadDir + "trial", attachSub(iSvc, "_22") + "22_u:trial")
      os.Link(kFormDir  + "trial", attachSub(iSvc, "_22") + "22_f:trial")
      aSrec = &SendRecord{SaveId: "_22"}
   case "thread_set":
      aLastId := loadThread(iSvc, iUpdt.Thread.Id)
      iState.addThread(iUpdt.Thread.Id, aLastId)
   case "thread_save":
      if iUpdt.Thread.Id == "" {
         aTid := ""; if !iUpdt.Thread.New { aTid = iState.getThread() }
         iUpdt.Thread.Id = makeSaveId(aTid)
      }
      writeSaved(iSvc, iUpdt)
      if iUpdt.Thread.New {
         iState.addThread(iUpdt.Thread.Id, iUpdt.Thread.Id)
      } else {
         iState.openMsg(iUpdt.Thread.Id, true)
      }
      aMsg["id"] = iUpdt.Thread.Id
   case "thread_discard":
      deleteSaved(iSvc, iUpdt)
      if iUpdt.Thread.Id[0] == '_' {
         aTid := iState.getThread()
         aFn = func(c *ClientState) { c.discardThread(aTid) }
      } else {
         aFn = func(c *ClientState) { c.openMsg(iUpdt.Thread.Id, false) }
      }
   case "thread_send":
      if iUpdt.Thread.Id == "" { break }
      err := validateSaved(iSvc, iUpdt)
      if err != nil {
         aMsg["err"] = err.Error()
      } else {
         aSrec = &SendRecord{SaveId:iUpdt.Thread.Id}
      }
   case "thread_close":
      iState.openMsg(iUpdt.Thread.Id, false)
   case "history":
      iState.goThread(iUpdt.Navigate.History)
   case "tab_add":
      iState.addTab(iUpdt.Tab.Type, iUpdt.Tab.Term)
   case "tab_drop":
      iState.dropTab(iUpdt.Tab.Type)
   case "tab_select":
      iState.setTab(iUpdt.Tab.Type, iUpdt.Tab.PosFor, iUpdt.Tab.Pos)
   default:
      aMsg["err"] = "unknown op"
   }
   return aMsg, aSrec, aFn
}

func readDirNames(iPath string) ([]string, error) {
   aFd, err := os.Open(iPath)
   if err != nil { return nil, err }
   aList, err := aFd.Readdirnames(0)
   aFd.Close()
   return aList, err
}

func storeFile(iPath string, iData interface{}) error {
   aTemp := iPath + ".tmp"
   defer os.Remove(aTemp)
   err := writeJsonFile(aTemp, iData)
   if err != nil { return err }
   err = syncDir(path.Dir(iPath))
   if err != nil { return err }
   err = os.Remove(iPath)
   if err != nil && !os.IsNotExist(err) { return err }
   err = os.Rename(aTemp, iPath)
   if err != nil {
      fmt.Fprintf(os.Stderr, "transaction failed...")
      quit(err)
   }
   return nil
}

func writeJsonFile(iPath string, iData interface{}) error {
   aFd, err := os.OpenFile(iPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { return err }
   defer aFd.Close()
   err = json.NewEncoder(aFd).Encode(iData)
   if err != nil { return err }
   err = aFd.Sync()
   return err
}

func resolveTmpFile(iPath string) error {
   return renameRemove(iPath, iPath[:len(iPath)-4])
}

func renameRemove(iA, iB string) error {
   err := os.Rename(iA, iB)
   if err != nil {
      if os.IsNotExist(err) {
         err = nil
      } else if os.IsExist(err) {
         err = os.Remove(iA)
      }
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

func dateRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

func quit(err error) {
   fmt.Fprintf(os.Stderr, "quit after %s\n", err.Error())
   debug.PrintStack()
   os.Exit(3)
}

type tError string
func (o tError) Error() string { return string(o) }
