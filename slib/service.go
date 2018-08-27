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
   "encoding/json"
   "os"
   "strings"
   "sync"
)

type tGlobalService struct{}
var Service tGlobalService

var sServicesDoor sync.RWMutex
var sServices = make(map[string]*tService)
var sServiceStartFn func(string)

type tSvcConfig struct {
   Name string
   Description string
   LoginPeriod int // seconds
   Addr string // for tls.Dial()
   Verify bool // for tls.Config
   Uid string
   Alias string
   Node string
   Error string `json:",omitempty"` // from "registered" message
}

func initServices(iFn func(string)) {
   sServiceStartFn = iFn
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
      aService := _newService(nil)
      aSvcFiles := [...]struct { name string; cache interface{}; reqd bool }{
         {cfgFile  (aSvc), &aService.config, true },
         {sendqFile(aSvc), &aService.sendQ,  false},
         {tabFile  (aSvc), &aService.tabs,   false},
         {notcFile (aSvc), &aService.notice, false},
         {pingFile (aSvc), nil,              false},
         {ohiFile  (aSvc), nil,              false},
      }
      for a := range aSvcFiles {
         err = resolveTmpFile(aSvcFiles[a].name + ".tmp")
         if err != nil { quit(err) }
         if aSvcFiles[a].cache != nil {
            err = readJsonFile(aSvcFiles[a].cache, aSvcFiles[a].name)
            if err != nil && (aSvcFiles[a].reqd || !os.IsNotExist(err)) { quit(err) }
         }
      }
      sServices[aSvc] = aService
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
         } else if strings.HasPrefix(aTmp, "ffnindex_") {
            renameRemove(tempDir(aSvc) + aTmp, attachFfn(aSvc, aTmp[9:]))
         } else if strings.HasSuffix(aTmp, ".tmp") {
            // could be a valid attachment from thread transaction
            defer os.Remove(tempDir(aSvc) + aTmp)
         } else {
            completeThread(aSvc, aTmp)
         }
      }
   }
}

func startAllService() {
   for _, aSvc := range Service.GetIdx().([]string) {
      sServiceStartFn(aSvc)
   }
}

func (tGlobalService) GetIdx() interface{} {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   aS := make([]string, len(sServices))
   a := 0
   for aS[a], _ = range sServices { a++ }
   return aS
}

func (tGlobalService) GetPath(string) string {
   return ""
}

func (tGlobalService) Add(iName, iDup string, iR io.Reader) error {
   var err error
   var aCfg tSvcConfig
   err = json.NewDecoder(iR).Decode(&aCfg)
   if err != nil { return err } // todo only network errors

   if iDup != "" {
      return tError("duplicate disallowed")
   }
   if iName != aCfg.Name || len(iName) < 4 || strings.HasSuffix(iName, ".tmp") {
      return tError("name not valid: " + iName)
   }
   sServicesDoor.Lock()
   if sServices[iName] != nil {
      sServicesDoor.Unlock()
      return tError("name already exists: " + iName)
   }
   sServices[iName] = _newService(&aCfg)
   aTemp := iName + ".tmp"
   _makeTree(aTemp)
   err = writeJsonFile(cfgFile(aTemp), &aCfg)
   if err != nil { quit(err) }
   err = syncDir(svcDir(aTemp))
   if err != nil { quit(err) }
   err = os.Rename(svcDir(aTemp), svcDir(iName))
   if err != nil { quit(err) }
   sServicesDoor.Unlock()

   if sServiceStartFn != nil {
      sServiceStartFn(iName)
   }
   return nil
}

func (tGlobalService) Drop(iName string) error {
   return tError("Drop not supported")
}

func getService(iSvc string) *tService {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   return sServices[iSvc]
}

func GetConfigService(iSvc string) *tSvcConfig {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aCfg := aSvc.config
   return &aCfg
}

func getUriService(iSvc string) string {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   return aSvc.config.Addr +"/"+ aSvc.config.Uid +"/"
}

func getDoorService(iSvc string, iId string, iMake func()tDoor) tDoor {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aDoor := aSvc.doors[iId]
   if aDoor == nil {
      aDoor = iMake()
      aSvc.doors[iId] = aDoor
   }
   return aDoor
}

func _newService(iCfg *tSvcConfig) *tService {
   aSvc := &tService{tabs: []string{}, doors: make(map[string]tDoor)}
   if iCfg != nil { aSvc.config = *iCfg }
   return aSvc
}

func _makeTree(iSvc string) {
   var err error
   for _, aDir := range [...]string{tempDir(iSvc), threadDir(iSvc), attachDir(iSvc), formDir(iSvc)} {
      err = os.MkdirAll(aDir, 0700)
      if err != nil { quit(err) }
   }
   for _, aFile := range [...]string{pingFile(iSvc), ohiFile(iSvc), tabFile(iSvc), sendqFile(iSvc),
                                     notcFile(iSvc)} {
      err = os.Symlink("empty", aFile)
      if err != nil && !os.IsExist(err) { quit(err) }
   }
}

func _updateConfig(iCfg *tSvcConfig) error {
   var err error
   aSvc := getService(iCfg.Name)
   if aSvc == nil {
      return tError(iCfg.Name + " not found")
   }
   aSvc.Lock(); defer aSvc.Unlock()
   aSvc.config = *iCfg
   err = storeFile(cfgFile(iCfg.Name), iCfg)
   if err != nil { quit(err) }
   return nil
}

func getTabsService(iSvc string) []string {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aList := make([]string, len(aSvc.tabs))
   copy(aList, aSvc.tabs)
   return aList
}

func addTabService(iSvc string, iTerm string) int {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aSvc.tabs = append(aSvc.tabs, iTerm)
   err := storeFile(tabFile(iSvc), aSvc.tabs)
   if err != nil { quit(err) }
   return len(aSvc.tabs)-1
}

func dropTabService(iSvc string, iPos int) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aSvc.tabs = aSvc.tabs[:iPos + copy(aSvc.tabs[iPos:], aSvc.tabs[iPos+1:])]
   err := storeFile(tabFile(iSvc), aSvc.tabs)
   if err != nil { quit(err) }
}

func SendService(iW io.Writer, iSvc string, iSrec *SendRecord) error {
   var aFn func(io.Writer, string, string, string) error
   switch iSrec.Id[0] {
   case eSrecOhi:    aFn = sendEditOhi
   case eSrecPing:   aFn = sendDraftAdrsbk
   case eSrecAccept: aFn = sendJoinGroupAdrsbk
   case eSrecThread: aFn = sendDraftThread
   default:
      quit(tError("unknown op " + iSrec.Id[:1]))
   }
   err := aFn(iW, iSvc, iSrec.Id[1:], iSrec.Id)
   if err != nil && err.Error() == "already sent" {
      dropQueue(iSvc, iSrec.Id)
   }
   return err
}

func LogoutService(iSvc string) []string {
   dropFromOhi(iSvc)
   return []string{"of"}
}

func HandleTmtpService(iSvc string, iHead *Header, iR io.Reader) (
                       aFn func(*ClientState)interface{}) {
   var err error
   var aResult []string
   fAll := func(c *ClientState) interface{} { return aResult }
   fErr := func(c *ClientState) interface{} { return iHead.Op +" "+ err.Error() }

   switch iHead.Op {
   case "tmtprev":
      //todo
   case "registered":
      aNewCfg := GetConfigService(iSvc)
      aNewCfg.Uid = iHead.Uid
      aNewCfg.Node = iHead.NodeId
      if iHead.Error != "" {
         aNewCfg.Alias = ""
         aNewCfg.Error = "["+ iHead.Error[len("AddAlias: alias "):] +"]"
      } else {
         storeSelfAdrsbk(iSvc, aNewCfg.Alias, aNewCfg.Uid)
      }
      err = _updateConfig(aNewCfg)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: %s %s\n", iSvc, iHead.Op, err.Error())
         return fErr
      }
      aFn, aResult = fAll, []string{"cf"}
   case "login":
      aFn, aResult = fAll, []string{"_e", "login by "+ iHead.Node}
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
      aFn, aResult = fAll, []string{"nl", "pf", "pt"}
   case "invite":
      err = storeReceivedAdrsbk(iSvc, iHead, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: invite error %s\n", iSvc, err.Error())
         return fErr
      }
      aFn, aResult = fAll, []string{"nl", "pf", "pt", "if"}
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
            if c.getThread() == iHead.SubHead.ThreadId { return aResult }
            return aResult[:1]
         }
         aResult = []string{"pt", "al", "ml"}
      }
   case "ack":
      if iHead.Id == "t_22" { break } //todo temp
      aQid := iHead.Id
      iHead.Id = iHead.Id[1:]
      aId := parseLocalId(iHead.Id)
      switch aQid[0] {
      case eSrecPing:
         if iHead.Error != "" {
            aFn, aResult = fAll, []string{"ps", "_e", iHead.Error}
            break
         }
         storeSentAdrsbk(iSvc, aId.ping(), iHead.Posted)
         aFn, aResult = fAll, []string{"ps", "pt", "pf", "it", "gl"}
      case eSrecAccept:
         if iHead.Error != "" {
            aFn, aResult = fAll, []string{"if", "_e", iHead.Error}
            break
         }
         acceptInviteAdrsbk(iSvc, aId.gid(), iHead.Posted)
         aFn, aResult = fAll, []string{"if", "gl"}
      case eSrecThread:
         if iHead.Error != "" {
            aTid := aId.tid(); if aTid == "" { aTid = iHead.Id }
            aFn = func(c *ClientState) interface{} {
               if c.getThread() == aTid { return aResult }
               return aResult[1:]
            }
            aResult = []string{"ml", "_e", iHead.Error}
            break
         }
         storeSentThread(iSvc, iHead)
         if aId.tid() == "" {
            aFn = func(c *ClientState) interface{} {
               c.renameThread(iHead.Id, iHead.MsgId)
               if c.getThread() == iHead.MsgId { return aResult }
               return aResult[1:3]
            }
         } else {
            aFn = func(c *ClientState) interface{} {
               c.renameMsg(aId.tid(), iHead.Id, iHead.MsgId)
               if c.getThread() == aId.tid() { return aResult[2:] }
               return aResult[2:3]
            }
         }
         aResult = []string{"cs", "tl", "pf", "al", "ml", "mn", iHead.MsgId}
      case eSrecOhi:
         if iHead.Error != "" {
            aFn, aResult = fAll, []string{"_e", iHead.Error}
         }
         return aFn // not queued
      default:
         quit(tError("bad SendRecord " + aQid))
      }
      dropQueue(iSvc, aQid)
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
                                            return iUpdt.Op +" "+ err.Error() }; return nil }

   if iSvc == "local" && iUpdt.Op != "open" {
      err = tError("not supported")
      return fErr, nil
   }

   switch iUpdt.Op {
   case "open":
      if iSvc == "local" {
         aFn, aResult = fOne, []string{"/v", "/t", "/f"}
      } else {
         aFn, aResult = fOne, []string{"of", "ot", "ps", "pt", "pf", "if", "it", "gl",
                                       "cf", "nl", "tl", "cs", "al", "_t", "ml", "mo",
                                       "/v", "/t", "/f"}
      }
   case "config_update":
      aNewCfg := GetConfigService(iSvc)
      if iUpdt.Config.Addr != "" { aNewCfg.Addr = iUpdt.Config.Addr }
      if iUpdt.Config.LoginPeriod >= 0 { aNewCfg.LoginPeriod = iUpdt.Config.LoginPeriod }
      if iUpdt.Config.Verify { aNewCfg.Verify = !aNewCfg.Verify }
      err = _updateConfig(aNewCfg)
      if err != nil { return fErr, nil }
      aFn, aResult = fAll, []string{"cf"}
   case "ohi_add", "ohi_drop":
      aSrec, err = editOhi(iSvc, iUpdt)
      if err != nil { return fErr, nil }
      aFn, aResult = fAll, []string{"ot"}
   case "ping_save":
      storeDraftAdrsbk(iSvc, iUpdt)
      aFn, aResult = fAll, []string{"ps"}
   case "ping_discard":
      deleteDraftAdrsbk(iSvc, iUpdt.Ping.To, iUpdt.Ping.Gid)
      aFn, aResult = fAll, []string{"ps"}
   case "ping_send":
      aSrec = addQueue(iSvc, eSrecPing, iUpdt.Ping.Qid)
      aFn, aResult = fAll, []string{"ps"}
   case "accept_send":
      aSrec = addQueue(iSvc, eSrecAccept, iUpdt.Accept.Qid)
      aFn, aResult = fAll, []string{"if"}
   case "adrsbk_search":
      if iUpdt.Adrsbk.Term == "" {
         err = tError("search term missing")
         return fErr, nil
      }
      aResult = searchAdrsbk(iSvc, iUpdt)
      aFn, aResult = fOne, append([]string{"_n"}, aResult...)
   case "notice_seen":
      err = setLastSeenNotice(iSvc, iUpdt)
      if err != nil { return fErr, nil }
      aFn, aResult = fAll, []string{"nl"}
   case "thread_recvtest":
      aTid := iState.getThread()
      if len(aTid) > 0 && aTid[0] == '_' { break }
      aFd, err := os.OpenFile(threadDir(iSvc) + "_22", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
      if err != nil { quit(err) }
      aData := bytes.NewBufferString("ohi there\n![?](this_f:trial.original)")
      aHead := Header{DataLen:int64(aData.Len()), SubHead:
               tHeader2{ThreadId:aTid, noAttachSize:true, For:
               []tHeaderFor{{Id:GetConfigService(iSvc).Uid, Type:1}}, Attach:
               []tHeader2Attach{{Name:"u:trial"},
                  {Name:"r:abc", Size:80, FfKey:"abc",
                   Ffn:"localhost:8888/5X8SZWGW7MLR+4GNB1LF+P8YGXCZF4BN/abc"},
                  {Name:"f:trial.original", Ffn:"form-reg.github.io/cat/trial"} }}}
      aForm := map[string]string{"abc":
         `{"nr":1, "so":"s", "bd":true, "or":{ "anr":[[1,2],[1,2]], "aso":["s","s","s"] }}`}
      _writeMsgTemp(aFd, &aHead, aData, &tIndexEl{})
      writeFormFillAttach(aFd, &aHead.SubHead, aForm, &tIndexEl{})
      aFd.Close()
      os.Mkdir(attachSub(iSvc, "_22"), 0700)
      os.Link(kUploadDir + "trial",          attachSub(iSvc, "_22") + "22_u:trial")
      os.Link(kFormDir   + "trial.original", attachSub(iSvc, "_22") + "22_f:trial.original")
      aSrec = addQueue(iSvc, eSrecThread, "_22")
   case "thread_save":
      const ( _ int8 = iota; eNewThread; eNewReply )
      if iUpdt.Thread.New > 0 {
         aTid := ""; if iUpdt.Thread.New == eNewReply { aTid = iState.getThread() }
         iUpdt.Thread.Id = makeLocalId(aTid)
      }
      storeDraftThread(iSvc, iUpdt)
      if iUpdt.Thread.New == eNewThread {
         iState.addThread(iUpdt.Thread.Id)
         aFn = func(c *ClientState) interface{} {
            if c == iState { return aResult }
            return aResult[:1]
         }
         aResult = []string{"tl", "cs", "al", "_t", "ml", "mo"}
      } else if iUpdt.Thread.New == eNewReply {
         iState.openMsg(iUpdt.Thread.Id, true)
         aTid := iState.getThread()
         aFn = func(c *ClientState) interface{} {
            if c.getThread() == aTid { return aResult }
            return nil
         }
         aResult = []string{"al", "ml", "mn", iUpdt.Thread.Id}
      } else { // may update msg from a threadid other than iState.getThread()
         aFn = func(c *ClientState) interface{} {
            if c.isOpen(iUpdt.Thread.Id) { return aResult }
            return nil
         }
         aResult = []string{"al", "mn", iUpdt.Thread.Id}
      }
   case "thread_discard":
      deleteDraftThread(iSvc, iUpdt)
      aTid := iState.getThread()
      if iUpdt.Thread.Id[0] == '_' {
         aFn = func(c *ClientState) interface{} {
            defer c.discardThread(aTid)
            if c.getThread() == aTid { return aResult }
            return aResult[:1]
         }
         aResult = []string{"tl", "cs", "al", "_t", "ml", "mo"}
      } else {
         aFn = func(c *ClientState) interface{} {
            c.openMsg(iUpdt.Thread.Id, false)
            if c.getThread() == aTid { return aResult }
            return nil
         }
         aResult = []string{"al", "ml"}
      }
   case "thread_send":
      if iUpdt.Thread.Id == "" { break }
      err = validateDraftThread(iSvc, iUpdt)
      if err != nil { return fErr, nil }
      aTid := iState.getThread()
      aFn = func(c *ClientState) interface{} {
         if c.getThread() == aTid { return aResult }
         return nil
      }
      aResult = []string{"ml"}
      aSrec = addQueue(iSvc, eSrecThread, iUpdt.Thread.Id)
   case "thread_open":
      if iUpdt.Thread.ThreadId != iState.getThread() {
         err = tError("thread id out of sync")
         return fErr, nil
      }
      iState.openMsg(iUpdt.Thread.Id, true)
      seenReceivedThread(iSvc, iUpdt)
      aFn = func(c *ClientState) interface{} {
         if c.getThread() == iUpdt.Thread.ThreadId { return aResult }
         return nil
      }
      aResult = []string{"ml"}
   case "thread_close":
      iState.openMsg(iUpdt.Thread.Id, false)
      // no result
   case "navigate_thread":
      iState.addThread(iUpdt.Navigate.ThreadId)
      aFn, aResult = fOne, []string{"cs", "al", "_t", "ml", "mo"}
   case "navigate_history":
      iState.goThread(iUpdt.Navigate.History)
      aFn, aResult = fOne, []string{"cs", "al", "_t", "ml", "mo"}
   case "navigate_link":
      _, err = os.Lstat(threadDir(iSvc) + iUpdt.Navigate.ThreadId)
      if err != nil { return fErr, nil }
      aDiff := iUpdt.Navigate.ThreadId != iState.getThread()
      iState.goLink(iUpdt.Navigate.ThreadId, iUpdt.Navigate.MsgId)
      aFn = fOne
      aResult = []string{"cs", "mo"}; if aDiff { aResult = []string{"cs", "al", "_t", "ml", "mo"} }
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
   case "test":
      if len(iUpdt.Test.Request) > 0 {
         aFn, aResult = fOne, iUpdt.Test.Request
      } else if iUpdt.Test.Notice != nil {
         aSvc := getService(iSvc)
         aSvc.Lock(); defer aSvc.Unlock()
         aSvc.notice = iUpdt.Test.Notice
      }
   default:
      err = tError("unknown op")
      return fErr, nil
   }
   return aFn, aSrec
}

