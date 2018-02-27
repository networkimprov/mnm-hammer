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

func LogoutService(iSvc string) interface{} {
   dropFromOhi(iSvc)
   return []string{"of"}
}

func HandleTmtpService(iSvc string, iHead *Header, iR io.Reader) (
                       aFn func(*ClientState)interface{}) {
   var err error
   var aResult []string
   fAll := func(c *ClientState) interface{} { return aResult }
   fErr := func(c *ClientState) interface{} { return tMsgError{iHead.Op, err.Error()} }

   switch iHead.Op {
   case "registered":
      aNewSvc := *GetDataService(iSvc)
      aNewSvc.Uid = iHead.Uid
      aNewSvc.Node = iHead.NodeId
      err = _updateService(&aNewSvc)
      if err != nil { return fErr }
      aFn, aResult = fAll, []string{"sl"}
   case "info":
      setFromOhi(iSvc, iHead)
      aFn, aResult = fAll, []string{"of"}
   case "ohi":
      updateFromOhi(iSvc, iHead)
      aFn, aResult = fAll, []string{"of"}
   case "ping":
      err = storeReceivedAdrsbk(iSvc, iHead, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: ping error %s\n", iSvc, err.Error())
         return fErr
      }
      aFn, aResult = fAll, []string{"pf", "pt"}
   case "invite":
      err = storeReceivedAdrsbk(iSvc, iHead, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: invite error %s\n", iSvc, err.Error())
         return fErr
      }
      aFn, aResult = fAll, []string{"pf", "pt", "if"}
   case "member":
      if iHead.Act == "join" {
         groupJoinedAdrsbk(iSvc, iHead)
         aFn, aResult = fAll, []string{"it"}
      }
   case "delivery":
      err = storeReceivedThread(iSvc, iHead, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: delivery error %s\n", iSvc, err.Error())
         return fErr
      }
      if iHead.SubHead.ThreadId == "" {
         aFn, aResult = fAll, []string{"pt", "tl"}
      } else {
         aFn = func(c *ClientState) interface{} {
            if c.getThread() == iHead.SubHead.ThreadId { c.openMsg(iHead.Id, true); return aResult }
            return aResult[:1]
         }
         aResult = []string{"pt", "al", "ml", "mo", "mn", iHead.Id} //todo drop mo
      }
   case "ack":
      if iHead.Error != "" {
         err = tError("ack: " + iHead.Error)
         return fErr
      }
      if iHead.Id == "t_22" { break } //todo temp
      aId := parseSaveId(iHead.Id[1:])
      switch iHead.Id[0] {
      case eSrecPing:
         storeSentAdrsbk(iSvc, aId.ping(), iHead.Posted)
         aFn, aResult = fAll, []string{"ps", "pt", "pf", "it", "gl"}
      case eSrecAccept:
         acceptInviteAdrsbk(iSvc, aId.ping(), iHead.Posted)
         aFn, aResult = fAll, []string{"if", "gl"}
      case eSrecThread:
         iHead.Id = iHead.Id[1:]
         storeSentThread(iSvc, iHead)
         if aId.tid() == "" {
            aFn = func(c *ClientState) interface{} {
               c.renameThread(iHead.Id, iHead.MsgId)
               if c.getThread() == iHead.MsgId { return aResult }
               return aResult[:2]
            }
         } else {
            aFn = func(c *ClientState) interface{} {
               c.renameMsg(aId.tid(), iHead.Id, iHead.MsgId)
               if c.getThread() == aId.tid() { return aResult[1:] }
               return aResult[1:2]
            }
         }
         aResult = []string{"tl", "pf", "al", "ml", "mo", "mn", iHead.MsgId} //todo drop mo
      }
   default:
      err = tError("unknown tmtp op")
      return fErr
   }
   return aFn
}

func HandleUpdtService(iSvc string, iState *ClientState, iUpdt *Update) (
                       aFn func(*ClientState)interface{}, aSrec *SendRecord) {
   var err error
   var aResult []string
   fAll := func(c *ClientState) interface{} { return aResult }
   fOne := func(c *ClientState) interface{} { if c == iState { return aResult }; return nil }
   fErr := func(c *ClientState) interface{} { if c == iState {
                                      return tMsgError{iUpdt.Op, err.Error()} }; return nil }

   switch iUpdt.Op {
   case "open":
      aFn, aResult = fOne, []string{"sl", "of", "ot", "ps", "pt", "pf", "if", "it", "gl",
                                    "cf", "tl", "cs", "al", "ml", "mo", "/t", "/f"}
   case "service_add":
      err = _addService(iUpdt.Service)
      if err != nil { return fErr, nil }
      aFn, aResult = fAll, []string{"sl"}
   case "service_update":
      err = _updateService(iUpdt.Service)
      if err != nil { return fErr, nil }
      aFn, aResult = fAll, []string{"sl"}
   case "ohi_add", "ohi_drop":
      aSrec = editOhi(iSvc, iUpdt)
      aFn, aResult = fAll, []string{"ot"}
   case "ping_save":
      storeSavedAdrsbk(iSvc, iUpdt)
      aFn, aResult = fAll, []string{"ps"}
   case "ping_discard":
      deleteSavedAdrsbk(iSvc, iUpdt.Ping.To, iUpdt.Ping.Gid)
      aFn, aResult = fAll, []string{"ps"}
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
      aFn, aResult = fOne, []string{"cs", "al", "ml", "mo"}
   case "thread_save":
      const ( _ int8 = iota; eNewThread; eNewReply )
      if iUpdt.Thread.New > 0 {
         aTid := ""; if iUpdt.Thread.New == eNewReply { aTid = iState.getThread() }
         iUpdt.Thread.Id = makeSaveId(aTid)
      }
      storeSavedThread(iSvc, iUpdt)
      if iUpdt.Thread.New == eNewThread {
         iState.addThread(iUpdt.Thread.Id, iUpdt.Thread.Id)
         aFn = func(c *ClientState) interface{} {
            if c == iState { return aResult }
            return aResult[:1]
         }
      } else if iUpdt.Thread.New == eNewReply {
         iState.openMsg(iUpdt.Thread.Id, true)
         aTid := iState.getThread()
         aFn = func(c *ClientState) interface{} {
            if c.getThread() == aTid { return aResult[2:] }
            return nil
         }
      } else { // may update msg from a threadid other than iState.getThread()
         aFn = func(c *ClientState) interface{} {
            if c == iState { return []string{"al"} }
            if c.isOpen(iUpdt.Thread.Id) { return []string{"al", "mn", iUpdt.Thread.Id} }
            return nil
         }
      }
      aResult = []string{"tl", "cs", "al", "ml", "mo"}
   case "thread_discard":
      deleteSavedThread(iSvc, iUpdt)
      aTid := iState.getThread()
      if iUpdt.Thread.Id[0] == '_' {
         aFn = func(c *ClientState) interface{} {
            defer c.discardThread(aTid)
            if c.getThread() == aTid { return aResult }
            return aResult[:1]
         }
      } else {
         aFn = func(c *ClientState) interface{} {
            c.openMsg(iUpdt.Thread.Id, false)
            if c.getThread() == aTid { return aResult [2:] }
            return nil
         }
      }
      aResult = []string{"tl", "cs", "al", "ml", "mo"} //todo drop mo
   case "thread_send":
      if iUpdt.Thread.Id == "" { break }
      err = validateSavedThread(iSvc, iUpdt)
      if err != nil { return fErr, nil }
      aSrec = &SendRecord{id: string(eSrecThread) + iUpdt.Thread.Id}
   case "thread_close":
      iState.openMsg(iUpdt.Thread.Id, false)
      aFn, aResult = fOne, []string{"mo"} //todo drop
   case "history":
      iState.goThread(iUpdt.Navigate.History)
      aFn, aResult = fOne, []string{"cs", "al", "ml", "mo"}
   case "tab_add":
      iState.addTab(iUpdt.Tab.Type, iUpdt.Tab.Term)
      aAlt := "tl"; if iUpdt.Tab.Type == eTabThread { aAlt = "mo" }
      aFn, aResult = fOne, []string{"cs", aAlt}
   case "tab_pin":
      iState.pinTab(iUpdt.Tab.Type)
      aFn, aResult = fAll, []string{"cs"}
   case "tab_drop":
      iState.dropTab(iUpdt.Tab.Type)
      aAlt := "tl"; if iUpdt.Tab.Type == eTabThread { aAlt = "mo" }
      aFn = fAll; if iUpdt.Tab.Type == eTabThread { aFn = fOne } //todo eTabService && !pinned = fOne
      aResult = []string{"cs", aAlt}
   case "tab_select":
      iState.setTab(iUpdt.Tab.Type, iUpdt.Tab.PosFor, iUpdt.Tab.Pos)
      aAlt := "tl"; if iUpdt.Tab.Type == eTabThread { aAlt = "mo" }
      aFn, aResult = fOne, []string{"cs", aAlt}
   default:
      err = tError("unknown op")
      return fErr, nil
   }
   return aFn, aSrec
}

type tMsgError struct { Op, Err string }

