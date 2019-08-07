// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "fmt"
   "io"
   "encoding/json"
   "os"
   pBleve "github.com/blevesearch/bleve"
   "strings"
   "sync"
   "net/url"
)

const kServiceNameMin = 2

type tGlobalService struct{}
var Service tGlobalService

var sServicesDoor sync.RWMutex
var sServices = make(map[string]*tService)
var sServiceStartFn func(string)

type tSvcConfig struct {
   Name string
   Description string
   HistoryLen int
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
      aSvc, err = url.QueryUnescape(aSvc)
      if err != nil { quit(err) }
      if strings.HasSuffix(aSvc, ".tmp") {
         err = os.RemoveAll(dirSvc(aSvc))
         if err != nil { quit(err) }
         continue
      }
      _makeTree(aSvc)
      aService := _newService(nil, nil)
      aSvcFiles := [...]struct { name string; cache interface{}; reqd bool }{
         {fileCfg  (aSvc), &aService.config, true },
         {fileSendq(aSvc), &aService.sendQ,  false},
         {fileTab  (aSvc), &aService.tabs,   false},
         {fileNotc (aSvc), &aService.notice, false},
         {filePing (aSvc), nil,              false},
         {fileOhi  (aSvc), nil,              false},
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
      aService.index = openIndexSearch(aSvc)
      var aTmps []string
      aTmps, err = readDirNames(dirTemp(aSvc))
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
         } else if strings.HasPrefix(aTmp, "forward_") {
            err = renameRemove(dirTemp(aSvc) + aTmp, fileFwd(aSvc, aTmp[8:]))
            if err != nil { quit(err) }
         } else if strings.HasPrefix(aTmp, "ffnindex_") {
            err = renameRemove(dirTemp(aSvc) + aTmp, fileFfn(aSvc, aTmp[9:]))
            if err != nil { quit(err) }
         } else if strings.HasSuffix(aTmp, ".tmp") {
            // could be a valid attachment or forward from thread transaction
            defer os.Remove(dirTemp(aSvc) + aTmp)
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
   aS := make([]string, 0, len(sServices))
   for aK := range sServices {
      aS = append(aS, aK)
   }
   return aS
}

func (tGlobalService) GetPath(string) string {
   return ""
}

func (tGlobalService) Add(iName, iDup string, iR io.Reader) error {
   var err error
   aCfg := tSvcConfig{HistoryLen:128}
   err = json.NewDecoder(iR).Decode(&aCfg)
   if err != nil { return err } // todo only network errors

   if iDup != "" {
      return tError("duplicate disallowed")
   }
   if iName != aCfg.Name || len(iName) < kServiceNameMin || strings.HasSuffix(iName, ".tmp") {
      return tError("name not valid: " + iName)
   }
   if aCfg.Addr[0] != '+' && aCfg.Addr[0] != '=' {
      return tError("address missing +/= prefix")
   }
   aCfg.Verify = aCfg.Addr[0] == '+'
   aCfg.Addr = aCfg.Addr[1:]

   sServicesDoor.Lock()
   if sServices[iName] != nil {
      sServicesDoor.Unlock()
      return tError("name already exists: " + iName)
   }
   aTemp := iName + ".tmp"
   _makeTree(aTemp)
   err = writeJsonFile(fileCfg(aTemp), &aCfg)
   if err != nil { quit(err) }
   err = syncDir(dirSvc(aTemp))
   if err != nil { quit(err) }
   err = os.Rename(dirSvc(aTemp), dirSvc(iName))
   if err != nil { quit(err) }
   sServices[iName] = _newService(&aCfg, openIndexSearch(iName))
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
   if iSvc == "local" {
      return &tSvcConfig{Name:"local"}
   }
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aCfg := aSvc.config
   return &aCfg
}

func GetCfService(iSvc string) interface{} {
   aCfg := GetConfigService(iSvc)
   aV := "="; if aCfg.Verify { aV = "+" }
   aCfg.Addr = aV + aCfg.Addr
   return aCfg
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

func _newService(iCfg *tSvcConfig, iBi pBleve.Index) *tService {
   aSvc := &tService{tabs: []string{}, doors: make(map[string]tDoor), index: iBi}
   if iCfg != nil { aSvc.config = *iCfg }
   return aSvc
}

func _makeTree(iSvc string) {
   var err error
   for _, aDir := range [...]string{dirTemp(iSvc), dirThread(iSvc), dirAttach(iSvc), dirForm(iSvc)} {
      err = os.MkdirAll(aDir, 0700)
      if err != nil { quit(err) }
   }
   for _, aFile := range [...]string{filePing(iSvc), fileOhi(iSvc), fileTab(iSvc), fileSendq(iSvc),
                                     fileNotc(iSvc)} {
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
   err = storeFile(fileCfg(iCfg.Name), iCfg)
   if err != nil { quit(err) }
   return nil
}

func getTabsService(iSvc string) []string {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   return append([]string{}, aSvc.tabs...)
}

func addTabService(iSvc string, iTerm string) int {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aSvc.tabs = append(aSvc.tabs, iTerm)
   err := storeFile(fileTab(iSvc), aSvc.tabs)
   if err != nil { quit(err) }
   return len(aSvc.tabs)-1
}

func dropTabService(iSvc string, iPos int) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aSvc.tabs = aSvc.tabs[:iPos + copy(aSvc.tabs[iPos:], aSvc.tabs[iPos+1:])]
   err := storeFile(fileTab(iSvc), aSvc.tabs)
   if err != nil { quit(err) }
}

func SendService(iW io.Writer, iSvc string, iSrec *SendRecord) error {
   var aFn func(io.Writer, string, string, string) error
   switch iSrec.Id[0] {
   case eSrecOhi:    aFn = sendEditOhi
   case eSrecPing:   aFn = sendDraftAdrsbk
   case eSrecAccept: aFn = sendJoinGroupAdrsbk
   case eSrecThread: aFn = sendDraftThread
   case eSrecFwd:    aFn = sendFwdDraftThread
   case eSrecCfm:    aFn = sendFwdConfirmThread
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
      aFn, aResult = fAll, []string{"nl", "pf", "pt"}
   case "member":
      if iHead.Act == "join" {
         groupJoinedAdrsbk(iSvc, iHead)
         aFn, aResult = fAll, []string{"pt"}
      }
   case "delivery":
      aGot := "thread"
      if iHead.Notify > 0 {
         err = storeFwdReceivedThread(iSvc, iHead, iR)
      } else {
         aGot, err = storeReceivedThread(iSvc, iHead, iR)
      }
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: delivery error %s\n", iSvc, err.Error())
         return fErr
      }
      if aGot == "thread" {
         aFn, aResult = fAll, []string{"pt", "pf", "tl"}
      } else if aGot == "msg" {
         aFn = func(c *ClientState) interface{} {
            if c.getThread() == iHead.SubHead.ThreadId { return aResult }
            return aResult[:3]
         }
         aResult = []string{"pt", "pf", "tl", "al", "ml"}
      }
   case "notify":
      err = storeFwdNotifyThread(iSvc, iHead, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: delivery error %s\n", iSvc, err.Error())
         return fErr
      }
      aFn = func(c *ClientState) interface{} {
         if c.getThread() == iHead.SubHead.ThreadId { return aResult }
         return aResult[:2]
      }
      aResult = []string{"pt", "pf", "cl"}
   case "ack":
      aQid := iHead.Id
      iHead.Id = iHead.Id[1:]
      aId := parseLocalId(iHead.Id)
      switch aQid[0] {
      case eSrecPing:
         if iHead.Error != "" {
            aFn, aResult = fAll, []string{"ps", "_e", iHead.Error}
            dropQueue(iSvc, aQid)
            break
         }
         storeSentAdrsbk(iSvc, aId.ping(), iHead.Posted, aQid)
         aFn, aResult = fAll, []string{"ps", "pt", "pf", "gl"}
      case eSrecAccept:
         if iHead.Error != "" {
            aFn, aResult = fAll, []string{"pf", "_e", iHead.Error}
            dropQueue(iSvc, aQid)
            break
         }
         acceptInviteAdrsbk(iSvc, aId.gid(), iHead.Posted, aQid)
         aFn, aResult = fAll, []string{"pf", "gl"}
      case eSrecThread:
         if iHead.Error != "" {
            aTid := aId.tid(); if aTid == "" { aTid = iHead.Id }
            aFn = func(c *ClientState) interface{} {
               if c.getThread() == aTid { return aResult }
               return aResult[1:]
            }
            aResult = []string{"ml", "_e", iHead.Error}
            dropQueue(iSvc, aQid)
            break
         }
         storeSentThread(iSvc, iHead, aQid)
         if aId.tid() == "" {
            aFn = func(c *ClientState) interface{} {
               c.renameThread(iHead.Id, iHead.MsgId)
               if c.getThread() == iHead.MsgId { return aResult }
               return aResult[1:3]
            }
         } else {
            aFn = func(c *ClientState) interface{} {
               c.renameMsg(aId.tid(), iHead.Id, iHead.MsgId)
               if c.getThread() == aId.tid() { return aResult[1:] }
               return aResult[1:3]
            }
         }
         aResult = []string{"cs", "tl", "pf", "cl", "al", "ml", "mn", iHead.MsgId}
      case eSrecFwd:
         if iHead.Error != "" {
            aFn = func(c *ClientState) interface{} {
               if c.getThread() == aId.tid() { return aResult }
               return aResult[1:]
            }
            aResult = []string{"cl", "_e", iHead.Error}
            dropQueue(iSvc, aQid)
            break
         }
         storeFwdSentThread(iSvc, iHead, aQid)
         aFn = func(c *ClientState) interface{} {
            if c.getThread() == aId.tid() { return aResult }
            return aResult[:1]
         }
         aResult = []string{"pf", "cl"}
      case eSrecCfm:
         if iHead.Error != "" {
            aFn, aResult = fAll, []string{"_e", iHead.Error}
         }
         dropQueue(iSvc, aQid)
      case eSrecOhi:
         if iHead.Error != "" {
            aFn, aResult = fAll, []string{"_e", iHead.Error}
         }
         // not queued
      default:
         quit(tError("bad SendRecord " + aQid))
      }
   default:
      err = tError("unknown tmtp op")
      return fErr
   }
   return aFn
}

func HandleUpdtService(iSvc string, iState *ClientState, iUpdt *Update) (
                       aFn func(*ClientState)interface{}) {
   var err error
   var aResult []string
   fAll := func(c *ClientState) interface{} { return aResult }
   fOne := func(c *ClientState) interface{} { if c == iState { return aResult }; return nil }
   fErr := func(c *ClientState) interface{} { if c == iState {
                                            return iUpdt.Op +" "+ err.Error() }; return nil }

   if iSvc == "local" && iUpdt.Op != "open" {
      err = tError("not supported")
      return fErr
   }

   switch iUpdt.Op {
   case "open":
      if iSvc == "local" {
         aFn, aResult = fOne, []string{"/v", "/t", "/f", "/g"}
      } else {
         aFn, aResult = fOne, []string{"of", "ot", "ps", "pt", "pf", "gl",
                                       "cf", "nl", "tl", "cs", "cl", "al", "_t", "ml", "mo",
                                       "/v", "/t", "/f", "/g"}
      }
   case "config_update":
      aNewCfg := GetConfigService(iSvc)
      if iUpdt.Config.HistoryLen >= 4 && iUpdt.Config.HistoryLen <= 1024 {
         aNewCfg.HistoryLen = iUpdt.Config.HistoryLen
         iState.setHistoryMax(aNewCfg.HistoryLen)
      }
      if iUpdt.Config.Addr != "" && (iUpdt.Config.Addr[0] == '+' || iUpdt.Config.Addr[0] == '=') {
         aNewCfg.Verify = iUpdt.Config.Addr[0] == '+'
         aNewCfg.Addr = iUpdt.Config.Addr[1:]
      }
      if iUpdt.Config.LoginPeriod >= 0 { aNewCfg.LoginPeriod = iUpdt.Config.LoginPeriod }
      err = _updateConfig(aNewCfg)
      if err != nil { return fErr }
      aFn, aResult = fAll, []string{"cf"}
   case "ohi_add", "ohi_drop":
      editOhi(iSvc, iUpdt)
      aFn, aResult = fAll, []string{"ot"}
   case "ping_save":
      storeDraftAdrsbk(iSvc, iUpdt)
      aFn, aResult = fAll, []string{"ps"}
   case "ping_discard":
      deleteDraftAdrsbk(iSvc, iUpdt.Ping.To, iUpdt.Ping.Gid)
      aFn, aResult = fAll, []string{"ps"}
   case "ping_send":
      addQueue(iSvc, eSrecPing, iUpdt.Ping.Qid)
      aFn, aResult = fAll, []string{"ps"}
   case "accept_send":
      addQueue(iSvc, eSrecAccept, iUpdt.Accept.Qid)
      aFn, aResult = fAll, []string{"pf"}
   case "adrsbk_search":
      if iUpdt.Adrsbk.Term == "" {
         err = tError("search term missing")
         return fErr
      }
      aResult = searchAdrsbk(iSvc, iUpdt)
      aFn, aResult = fOne, append([]string{"_n"}, aResult...)
   case "notice_seen":
      err = setLastSeenNotice(iSvc, iUpdt)
      if err != nil { return fErr }
      aFn, aResult = fAll, []string{"nl"}
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
         aResult = []string{"tl", "cs", "cl", "al", "_t", "ml", "mo"}
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
         aResult = []string{"cl", "al", "mn", iUpdt.Thread.Id}
         if iUpdt.Thread.Id[0] != '_' {
            aResult = aResult[1:]
         }
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
         aResult = []string{"tl", "cs", "cl", "al", "_t", "ml", "mo"}
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
      if err != nil { return fErr }
      aTid := iState.getThread()
      aFn = func(c *ClientState) interface{} {
         if c.getThread() == aTid { return aResult }
         return nil
      }
      aResult = []string{"ml"}
      addQueue(iSvc, eSrecThread, iUpdt.Thread.Id)
   case "thread_open":
      if iUpdt.Touch.ThreadId != iState.getThread() {
         err = tError("thread id out of sync")
         return fErr
      }
      iState.openMsg(iUpdt.Touch.MsgId, true)
      aChg := touchThread(iSvc, iUpdt)
      if !aChg { break }
      aFn = func(c *ClientState) interface{} {
         if c.getThread() == iUpdt.Touch.ThreadId { return aResult }
         return aResult[:1]
      }
      aResult = []string{"tl", "ml"}
   case "thread_close":
      iState.openMsg(iUpdt.Touch.MsgId, false)
      // no result
   case "thread_tag":
      iUpdt.Touch.ThreadId = iState.getThread()
      touchThread(iSvc, iUpdt)
      aFn = func(c *ClientState) interface{} {
         if c.getThread() == iUpdt.Touch.ThreadId {
            _, cTabVal := c.getSvcTab()
            if cTabVal[0] == '#' && Tag.getId(cTabVal[1:]) == iUpdt.Touch.TagId { return aResult }
            return aResult[1:]
         }
         return nil
      }
      aResult = []string{"tl", "ml"}
   case "forward_save":
      storeFwdDraftThread(iSvc, iUpdt)
      aFn = func(c *ClientState) interface{} {
         if c.getThread() == iUpdt.Forward.ThreadId { return aResult }
         return nil
      }
      aResult = []string{"cl"}
   case "forward_send":
      aFn = func(c *ClientState) interface{} {
         if c.getThread() == iUpdt.Forward.ThreadId { return aResult }
         return nil
      }
      aResult = []string{"cl"}
      addQueue(iSvc, eSrecFwd, iUpdt.Forward.Qid)
   case "navigate_thread":
      iState.addThread(iUpdt.Navigate.ThreadId)
      aFn, aResult = fOne, []string{"cs", "cl", "al", "_t", "ml", "mo"}
   case "navigate_history":
      iState.goThread(iUpdt.Navigate.History)
      aFn, aResult = fOne, []string{"cs", "cl", "al", "_t", "ml", "mo"}
   case "navigate_link":
      _, err = os.Lstat(dirThread(iSvc) + iUpdt.Navigate.ThreadId)
      if err != nil { return fErr }
      aDiff := iUpdt.Navigate.ThreadId != iState.getThread()
      iState.goLink(iUpdt.Navigate.ThreadId, iUpdt.Navigate.MsgId)
      aFn = fOne
      aResult = []string{"cs", "mo"}; if aDiff { aResult = []string{"cs", "cl", "al", "_t", "ml", "mo"} }
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
   case "sort_select":
      iState.setSort(iUpdt.Sort.Type, iUpdt.Sort.Field)
      aFn, aResult = fOne, []string{"cs"}
   case "test":
      if len(iUpdt.Test.Request) > 0 {
         if iUpdt.Test.ThreadId != "" {
            iState.addThread(iUpdt.Test.ThreadId)
         }
         aFn, aResult = fOne, iUpdt.Test.Request
      } else if iUpdt.Test.Notice != nil {
         aSvc := getService(iSvc)
         aSvc.Lock(); defer aSvc.Unlock()
         aSvc.notice = iUpdt.Test.Notice
      }
   default:
      err = tError("unknown op")
      return fErr
   }
   return aFn
}

// only for testing
func WipeDataService(iSvc string) error {
   aCfgTmp := kStorageDir +"svc-"+ url.QueryEscape(iSvc) +"-config"
   var err error
   err = os.Rename(fileCfg(iSvc), aCfgTmp)
   if err != nil { return err }
   err = os.RemoveAll(dirSvc(iSvc))
   if err != nil { return err }
   err = os.Mkdir(dirSvc(iSvc), 0700)
   if err != nil { return err }
   err = os.Rename(aCfgTmp, fileCfg(iSvc))
   return err
}
