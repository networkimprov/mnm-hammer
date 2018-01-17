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
   "os"
   "strings"
)

var sServiceStartFn func(string)

type tCfgService struct {
   Name string
   Description string
   LoginPeriod int // seconds
   Addr string // for Dial()
   Uid string
   Alias string
   Node string
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
      _makeTree(aSvc)
      for _, aFile := range [...]string{cfgFile(aSvc), pingFile(aSvc), ohiFile(aSvc),
                                        tabFile(aSvc)} {
         err = resolveTmpFile(aFile + ".tmp")
         if err != nil { quit(err) }
      }
      var aTmps []string
      aTmps, err = readDirNames(tempDir(aSvc))
      if err != nil { quit(err) }
      for _, aTmp := range aTmps {
         // some adrsbk ops stem from thread ops; complete them first
         if strings.HasPrefix(aTmp, "adrsbk_") {
            completeAdrsbk(aSvc, aTmp)
         }
      }
      for _, aTmp := range aTmps {
         if strings.HasPrefix(aTmp, "adrsbk_") {
            // handled above
         } else if strings.HasSuffix(aTmp, ".tmp") {
            defer os.Remove(tempDir(aSvc) + aTmp)
         } else {
            completeThread(aSvc, aTmp)
         }
      }
      aService := &tService{tabs:[]string{}}
      err = readJsonFile(&aService.cfg, cfgFile(aSvc))
      if err != nil { quit(err) }
      err = readJsonFile(&aService.tabs, tabFile(aSvc))
      if err != nil && !os.IsNotExist(err) { quit(err) }
      sServices[aSvc] = aService
   }
   if sServices["test"] == nil {
      err = _addService(&tCfgService{Name:"test", Addr:"localhost:8888", Alias:"_", LoginPeriod:30})
      if err != nil { quit(err) }
   }
   sServiceStartFn = iFn
}

func GetIdxService() []string {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   aS := make([]string, len(sServices))
   a := 0
   for aS[a], _ = range sServices { a++ }
   return aS
}

func GetDataService(iSvc string) *tCfgService {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   aSvc := sServices[iSvc]
   if aSvc == nil {
      return nil
   }
   return &aSvc.cfg
}

func getUriService(iSvc string) string {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   aSvc := sServices[iSvc]
   return aSvc.cfg.Addr +"/"+ aSvc.cfg.Uid +"/"
}

func _makeTree(iSvc string) {
   var err error
   for _, aDir := range [...]string{tempDir(iSvc), threadDir(iSvc), attachDir(iSvc), formDir(iSvc)} {
      err = os.MkdirAll(aDir, 0700)
      if err != nil { quit(err) }
   }
   for _, aFile := range [...]string{pingFile(iSvc), ohiFile(iSvc), tabFile(iSvc)} {
      err = os.Symlink("empty", aFile)
      if err != nil && !os.IsExist(err) { quit(err) }
   }
}

func _addService(iService *tCfgService) error {
   var err error
   if len(iService.Name) < 4 || strings.HasSuffix(iService.Name, ".tmp") {
      return tError(fmt.Sprintf("name %s not valid", iService.Name))
   }
   sServicesDoor.Lock(); defer sServicesDoor.Unlock()
   if sServices[iService.Name] != nil {
      return tError(fmt.Sprintf("name %s already exists", iService.Name))
   }
   aTemp := iService.Name + ".tmp"
   _makeTree(aTemp)
   err = writeJsonFile(cfgFile(aTemp), iService)
   if err != nil { quit(err) }

   err = syncDir(svcDir(aTemp))
   if err != nil { quit(err) }
   err = os.Rename(svcDir(aTemp), svcDir(iService.Name))
   if err != nil { quit(err) }

   sServices[iService.Name] = &tService{cfg: *iService}
   if sServiceStartFn != nil {
      sServiceStartFn(iService.Name)
   }
   return nil
}

func _updateService(iService *tCfgService) error {
   var err error
   sServicesDoor.Lock(); defer sServicesDoor.Unlock()
   aSvc := sServices[iService.Name]
   if aSvc == nil {
      return tError(iService.Name + " not found")
   }
   err = storeFile(cfgFile(iService.Name), iService)
   if err != nil { quit(err) }

   aSvc.cfg = *iService
   return nil
}

func addTabService(iSvc string, iTerm string) int {
   sServicesDoor.RLock()
   aSvc := sServices[iSvc]
   sServicesDoor.RUnlock()
   aSvc.tabs = append(aSvc.tabs, iTerm)
   err := storeFile(tabFile(iSvc), aSvc.tabs)
   if err != nil { quit(err) }
   return len(aSvc.tabs)-1
}

func dropTabService(iSvc string, iPos int) {
   sServicesDoor.RLock()
   aSvc := sServices[iSvc]
   sServicesDoor.RUnlock()
   aSvc.tabs = aSvc.tabs[:iPos + copy(aSvc.tabs[iPos:], aSvc.tabs[iPos+1:])]
   err := storeFile(tabFile(iSvc), aSvc.tabs)
   if err != nil { quit(err) }
}

func GetQueueService(iSvc string) ([]*SendRecord, error) {
   return nil, nil
}

func LogoutService(iSvc string) Msg {
   dropFromOhi(iSvc)
   return Msg{"op":"disconnect"}
}

func HandleTmtpService(iSvc string, iHead *Header, iR io.Reader) (
                aMsg Msg, aFn func(*ClientState)) {
   aMsg = Msg{"op":iHead.Op}
   switch iHead.Op {
   case "registered":
      aNewSvc := *GetDataService(iSvc)
      aNewSvc.Uid = iHead.Uid
      aNewSvc.Node = iHead.NodeId
      err := _updateService(&aNewSvc)
      if err != nil { aMsg["err"] = err.Error() }
   case "info":
      aMsg["op"] = "ohi"
      setFromOhi(iSvc, iHead)
   case "ohi":
      updateFromOhi(iSvc, iHead)
   case "ping":
      err := storeReceivedAdrsbk(iSvc, iHead, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: ping error %s\n", iSvc, err.Error())
         return nil, nil
      }
   case "invite":
      err := storeReceivedAdrsbk(iSvc, iHead, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: invite error %s\n", iSvc, err.Error())
         return nil, nil
      }
   case "member":
      if iHead.Act == "join" {
         groupJoinedAdrsbk(iSvc, iHead)
      }
   case "delivery":
      err := storeReceivedThread(iSvc, iHead, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: delivery error %s\n", iSvc, err.Error())
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
      if iHead.Id == "t_22" { break } //todo temp
      aMsg["msgid"] = iHead.MsgId
      if iHead.Error != "" {
         aMsg["err"] = iHead.Error
         break
      }
      aId := parseSaveId(iHead.Id[1:])
      switch iHead.Id[0] {
      case eSrecPing:
         storeSentAdrsbk(iSvc, aId.ping(), iHead.Posted)
      case eSrecAccept:
         acceptInviteAdrsbk(iSvc, aId.ping(), iHead.Posted)
      case eSrecThread:
         iHead.Id = iHead.Id[1:]
         storeSentThread(iSvc, iHead)
         if aId.tid() == "" {
            aFn = func(c *ClientState) { c.renameThread(iHead.Id, iHead.MsgId) }
         } else {
            aFn = func(c *ClientState) { c.renameMsg(aId.tid(), iHead.Id, iHead.MsgId) }
         }
      }
   }
   return aMsg, aFn
}

func HandleUpdtService(iSvc string, iState *ClientState, iUpdt *Update) (
                aMsg Msg, aSrec *SendRecord, aFn func(*ClientState)) {
   aMsg = Msg{"op":iUpdt.Op}
   switch iUpdt.Op {
   case "service_add":
      err := _addService(iUpdt.Service)
      if err != nil {
         aMsg["err"] = err.Error()
      }
   case "service_update":
      err := _updateService(iUpdt.Service)
      if err != nil {
         aMsg["err"] = err.Error()
      }
   case "ohi_add", "ohi_drop":
      aSrec = editOhi(iSvc, iUpdt)
   case "ping_save":
      storeSavedAdrsbk(iSvc, iUpdt)
   case "ping_discard":
      deleteSavedAdrsbk(iSvc, iUpdt.Ping.To, iUpdt.Ping.Gid)
   case "ping_send":
      aSrec = &SendRecord{id: string(eSrecPing) + makeSaveId(keySavedAdrsbk(iUpdt))}
   case "accept_send":
      aSrec = &SendRecord{id: string(eSrecAccept) + makeSaveId(iUpdt.Accept.Gid)}
   case "thread_recvtest":
      aTid := iState.getThread()
      if len(aTid) > 0 && aTid[0] == '_' { break }
      aFd, err := os.OpenFile(threadDir(iSvc) + "_22", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
      if err != nil { quit(err) }
      aData := bytes.NewBufferString("ohi there")
      aHead := Header{DataLen:int64(aData.Len()), SubHead:
               tHeader2{ThreadId:aTid, isSaved:true, For:
               []tHeaderFor{{Id:GetDataService(iSvc).Uid, Type:1}}, Attach:
               []tHeader2Attach{{Name:"upload/trial"},
                  {Name:"r:abc", Size:80, Ffn:"localhost:8888/5X8SZWGW7MLR+4GNB1LF+P8YGXCZF4BN/abc"},
                  {Name:"form/trial", Ffn:"form-reg.github.io/cat/trial"} }}}
      aForm := map[string]string{"abc":
         `{"nr":1, "so":"s", "bd":true, "or":{ "anr":[[1,2],[1,2]], "aso":["s","s","s"] }}`}
      _writeMsgTemp(aFd, &aHead, aData, &tIndexEl{})
      writeFormFillAttach(aFd, &aHead.SubHead, aForm, &tIndexEl{})
      aFd.Close()
      os.Mkdir(attachSub(iSvc, "_22"), 0700)
      os.Link(kUploadDir + "trial", attachSub(iSvc, "_22") + "22_u:trial")
      os.Link(kFormDir  + "trial", attachSub(iSvc, "_22") + "22_f:trial")
      aSrec = &SendRecord{id: string(eSrecThread) + "_22"}
   case "thread_set":
      aLastId := loadThread(iSvc, iUpdt.Thread.Id)
      iState.addThread(iUpdt.Thread.Id, aLastId)
   case "thread_save":
      if iUpdt.Thread.Id == "" {
         aTid := ""; if !iUpdt.Thread.New { aTid = iState.getThread() }
         iUpdt.Thread.Id = makeSaveId(aTid)
      }
      storeSavedThread(iSvc, iUpdt)
      if iUpdt.Thread.New {
         iState.addThread(iUpdt.Thread.Id, iUpdt.Thread.Id)
      } else {
         iState.openMsg(iUpdt.Thread.Id, true)
      }
      aMsg["id"] = iUpdt.Thread.Id
   case "thread_discard":
      deleteSavedThread(iSvc, iUpdt)
      if iUpdt.Thread.Id[0] == '_' {
         aTid := iState.getThread()
         aFn = func(c *ClientState) { c.discardThread(aTid) }
      } else {
         aFn = func(c *ClientState) { c.openMsg(iUpdt.Thread.Id, false) }
      }
   case "thread_send":
      if iUpdt.Thread.Id == "" { break }
      err := validateSavedThread(iSvc, iUpdt)
      if err != nil {
         aMsg["err"] = err.Error()
      } else {
         aSrec = &SendRecord{id: string(eSrecThread) + iUpdt.Thread.Id}
      }
   case "thread_close":
      iState.openMsg(iUpdt.Thread.Id, false)
   case "history":
      iState.goThread(iUpdt.Navigate.History)
   case "tab_add":
      iState.addTab(iUpdt.Tab.Type, iUpdt.Tab.Term)
   case "tab_pin":
      iState.pinTab(iUpdt.Tab.Type)
   case "tab_drop":
      iState.dropTab(iUpdt.Tab.Type)
   case "tab_select":
      iState.setTab(iUpdt.Tab.Type, iUpdt.Tab.PosFor, iUpdt.Tab.Pos)
   default:
      aMsg["err"] = "unknown op"
   }
   return aMsg, aSrec, aFn
}

