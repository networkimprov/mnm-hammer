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
   "sort"
   "strings"
   "sync"
   "time"
   "net/url"
)

const kServiceNameMin = 2
const kServiceHistoryMax = 128

type tGlobalService struct{}
var Service tGlobalService

var sServicesDoor sync.RWMutex
var sServices = make(map[string]*tService)
var sServiceStartFn func(string)
var sMsgToSelfFn func(string, *Header)

type tSvcConfig struct {
   Name string
   Description string `json:",omitempty"`
   HistoryLen int
   LoginPeriod int // seconds
   Addr string // for tls.Dial()
   Verify bool // for tls.Config
   Alias string
   Uid string
   Node string `json:",omitempty"`
   NodeSet []tNode
   Error string `json:",omitempty"` // from "registered" message
}

type tNode struct {
   Name string
   Status byte
   Local bool    `json:",omitempty"`
   Qid string    `json:",omitempty"`
   NodeId string `json:",omitempty"`
}

const ( eNodePending = 'p'; eNodeSent = 's'; eNodeAllowed = 'l'
        eNodeReady = 'r'; eNodeActive = 'a'; eNodeDefunct = 'd' ) // reserve 0xFF

func initServices(iSs func(string), iMts func(string, *Header)) {
   sServiceStartFn, sMsgToSelfFn = iSs, iMts
   var err error
   aSvcs, err := readDirNames(kServiceDir)
   if err != nil { quit(err) }

   os.Remove(kStorageDir + "tags") //todo remove in 0.8
   for _, aSvc := range aSvcs {
      aSvc, err = url.QueryUnescape(aSvc)
      if err != nil { quit(err) }
      if strings.HasSuffix(aSvc, ".tmp") {
         err = os.RemoveAll(dirSvc(aSvc))
         if err != nil { quit(err) }
         continue
      }
      //makeTreeService(aSvc) // for development, update tree
      err = os.Symlink("empty", fileTag(aSvc)) //todo drop in 0.8
      if err != nil && !os.IsExist(err) { quit(err) }
      sServices[aSvc] = _openService(aSvc)
      initSyncNode(aSvc)
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
         } else if strings.HasPrefix(aTmp, "synclog") {
            // no action
         } else if strings.HasSuffix(aTmp, ".tmp") {
            // could be a valid attachment or forward from thread transaction
            defer os.Remove(dirTemp(aSvc) + aTmp)
         } else if strings.HasPrefix(aTmp, "syncupdt_") {
            completeUpdtNode(aSvc, aTmp)
         } else if strings.HasPrefix(aTmp, "syncack_") {
            dropSyncNode(aSvc, aTmp[8+1:], aTmp[8:], "complete")
         } else {
            completeThread(aSvc, aTmp)
         }
      }
   }
}

func _openService(iSvc string) *tService {
   aService := _newService(nil)
   aSvcFiles := [...]struct { name string; cache interface{}; reqd bool }{
      {fileCfg  (iSvc), &aService.config, true },
      {fileSendq(iSvc), &aService.sendQ,  false},
      {fileTab  (iSvc), &aService.tabs,   false},
      {fileNotc (iSvc), &aService.notice, false},
      {filePing (iSvc), nil,              false},
      {fileOhi  (iSvc), nil,              false},
      {fileTag  (iSvc), &tTagset{},       false}, // last for initTag()
   }
   for a := range aSvcFiles {
      err := resolveTmpFile(aSvcFiles[a].name + ".tmp")
      if err != nil { quit(err) }
      if aSvcFiles[a].cache != nil {
         err = readJsonFile(aSvcFiles[a].cache, aSvcFiles[a].name)
         if err != nil && (aSvcFiles[a].reqd || !os.IsNotExist(err)) { quit(err) }
      }
   }
   initTag(iSvc, * aSvcFiles[len(aSvcFiles)-1].cache.(*tTagset))
   aService.index = openIndexSearch(&aService.config)
   if len(aService.config.NodeSet) == 0 { //todo drop in 0.8
      aService.config.NodeSet = []tNode{{Name:"first", Status:eNodeActive, Local:true}}
      err := storeFile(fileCfg(iSvc), &aService.config)
      if err != nil { quit(err) }
   }
   return aService
}

func startAllService() {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   for aK := range sServices {
      sServiceStartFn(aK)
   }
}

func (tGlobalService) GetIdx() interface{} {
   type tSvcEl struct { Name string; NoticeN, UnreadN int }
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   aS := make([]tSvcEl, 0, len(sServices))
   for aK, aV := range sServices {
      aV.RLock()
      aN := -1
      for aN = 0; aN < len(aV.notice) && aV.notice[aN].Seen != 0; aN++ {}
      aS = append(aS, tSvcEl{Name:aK, NoticeN: len(aV.notice) - aN, UnreadN: aV.unreadCount})
      aV.RUnlock()
   }
   sort.Slice(aS, func(cA, cB int) bool { return aS[cA].Name < aS[cB].Name })
   return aS
}

func (tGlobalService) GetPath(string) string {
   return ""
}

func (tGlobalService) Drop(iName string) error {
   return tError("Drop not supported")
}

func (tGlobalService) Add(iName, iDup string, iR io.Reader) error {
   var err error
   aCfg := tSvcConfig{HistoryLen:kServiceHistoryMax,
                      NodeSet: []tNode{{Name:"first", Status:eNodeActive, Local:true}}}
   err = json.NewDecoder(iR).Decode(&aCfg)
   if err != nil { return err } // todo only network errors

   if iDup != "" {
      return tError("duplicate disallowed")
   }
   if iName != aCfg.Name || !checkNameService(iName) {
      return tError("name not valid: "+ iName)
   }
   if aCfg.Addr[0] != '+' && aCfg.Addr[0] != '=' {
      return tError("address missing +/= prefix")
   }
   aCfg.Verify = aCfg.Addr[0] == '+'
   aCfg.Addr = aCfg.Addr[1:]

   sServicesDoor.Lock()
   if sServices[iName] != nil {
      sServicesDoor.Unlock()
      return tError("account already exists: "+ iName)
   }
   aTemp := iName + ".tmp"
   err = os.Mkdir(dirSvc(aTemp), 0700)
   if err != nil {
      if !os.IsExist(err) { quit(err) }
      sServicesDoor.Unlock()
      return tError("account in progress: "+ iName)
   }
   makeTreeService(aTemp)
   err = writeJsonFile(fileCfg(aTemp), &aCfg)
   if err != nil { quit(err) }
   err = syncDir(dirSvc(aTemp))
   if err != nil { quit(err) }
   err = os.Rename(dirSvc(aTemp), dirSvc(iName))
   if err != nil { quit(err) }
   sServices[iName] = _newService(&aCfg)
   sServicesDoor.Unlock()

   if sServiceStartFn != nil {
      sServiceStartFn(iName)
   }
   return nil
}

func addNodeService(iName, iPath string) error {
   sServicesDoor.Lock()
   if sServices[iName] != nil {
      err := os.Rename(iPath, dirSvc(iName +".tmp"))
      if err != nil {
         if !os.IsNotExist(err) { quit(err) }
      } else {
         err = os.RemoveAll(dirSvc(iName +".tmp"))
         if err != nil { quit(err) }
      }
      sServicesDoor.Unlock()
      return tError("account already exists: "+ iName)
   }
   err := os.Rename(iPath, dirSvc(iName))
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      sServicesDoor.Unlock()
      return tError("path does not exist: "+ iPath)
   }
   sServices[iName] = _openService(iName)
   sServicesDoor.Unlock()
   sServiceStartFn(iName)
   return nil
}

func getService(iSvc string) *tService {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   return sServices[iSvc]
}

func _editConfig(iSvc string, iFn func(*tSvcConfig)error) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   err := iFn(&aSvc.config)
   if err == nil {
      err = storeFile(fileCfg(iSvc), &aSvc.config)
      if err != nil { quit(err) }
   }
}

func _findNode(iSvc string, iName string) *tNode {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   for a := range aSvc.config.NodeSet {
      if aSvc.config.NodeSet[a].Name == iName {
         aNode := aSvc.config.NodeSet[a]
         return &aNode
      }
   }
   return nil
}

func _addNode(iSvc string, iNode *tNode) {
   _editConfig(iSvc, func(cCfg *tSvcConfig) error {
      for a := range cCfg.NodeSet {
         if cCfg.NodeSet[a].Name == iNode.Name {
            quit(tError("node name already exists: "+ iNode.Name))
         }
      }
      cCfg.NodeSet = append(cCfg.NodeSet, *iNode)
      if iNode.Status != eNodeActive {
         return nil
      }
      a := len(cCfg.NodeSet) - 1
      if a > 0 && (cCfg.NodeSet[a-1].Status == eNodePending || cCfg.NodeSet[a-1].Status == eNodeSent) {
         cCfg.NodeSet[a], cCfg.NodeSet[a-1] = cCfg.NodeSet[a-1], cCfg.NodeSet[a]
      }
      return nil
   })
}

func _updateNode(iSvc string, iNode *tNode) {
   _editConfig(iSvc, func(cCfg *tSvcConfig) error {
      a := -1
      for a = 0; a < len(cCfg.NodeSet); a++ {
         if cCfg.NodeSet[a].Name == iNode.Name { break }
      }
      cCfg.NodeSet[a] = *iNode
      if iNode.Status == 0xFF {
         cCfg.NodeSet = cCfg.NodeSet[:a + copy(cCfg.NodeSet[a:], cCfg.NodeSet[a+1:])]
      }
      return nil
   })
}

func _dropNode(iSvc string, iNode *tNode) {
   iNode.Status = 0xFF
   _updateNode(iSvc, iNode)
}

func GetConfigService(iSvc string) *tSvcConfig {
   if iSvc == "local" {
      return &tSvcConfig{Name:"local"}
   }
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aCfg := aSvc.config
   aCfg.NodeSet = nil
   return &aCfg
}

func GetCfService(iSvc string) interface{} {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aCfg := aSvc.config
   aCfg.Node = ""
   aCfg.NodeSet = append([]tNode{}, aCfg.NodeSet...)
   aV := "="; if aCfg.Verify { aV = "+" }
   aCfg.Addr = aV + aCfg.Addr
   return &aCfg
}

func makeNodeConfigService(iSvc string, iNode *tNode) *tSvcConfig {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aCfg := aSvc.config
   aCfg.Node = iNode.NodeId
   aCfg.NodeSet = append([]tNode{}, aCfg.NodeSet...)
   for a := range aCfg.NodeSet {
      aCfg.NodeSet[a].Local = aCfg.NodeSet[a].Name == iNode.Name
      if aCfg.NodeSet[a].Local {
         aCfg.NodeSet[a].Status = eNodeActive
         aCfg.NodeSet[a].NodeId = ""
      }
   }
   return &aCfg
}

func getUriService(iSvc string) string {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   return aSvc.config.Addr +"/"+ aSvc.config.Alias +"/"
}

func _initUnreadCount(iSvc string) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock() //todo handle with atomics?
   if aSvc.unreadCount >= 0 {
      return
   }
   aSvc.unreadCount = countUnreadSearch(iSvc)
}

func incrUnreadService(iSvc string) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   if aSvc.unreadCount >= 0 {
      aSvc.unreadCount++
   }
}

func decrUnreadService(iSvc string) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   if aSvc.unreadCount > 0 {
      aSvc.unreadCount--
   } else if aSvc.unreadCount == 0 {
      quit(tError("cannot decrement zero"))
   }
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

func checkNameService(iName string) bool {
   iName = strings.ToLower(iName)
   return !(len(iName) < kServiceNameMin || strings.HasSuffix(iName, ".tmp") ||
            isReservedFile(iName) || iName == ".." || iName == "favicon.ico" )
}

func _newService(iCfg *tSvcConfig) *tService {
   aSvc := &tService{tabs: []tTermEl{}, unreadCount: -1, doors: make(map[string]tDoor)}
   if iCfg != nil {
      aSvc.config = *iCfg
      aSvc.index = openIndexSearch(iCfg)
   }
   return aSvc
}

func makeTreeService(iSvc string) {
   var err error
   for _, aDir := range [...]string{dirTemp(iSvc), dirThread(iSvc), dirAttach(iSvc), dirForm(iSvc)} {
      err = os.MkdirAll(aDir, 0700)
      if err != nil { quit(err) }
   }
   for _, aFile := range [...]string{filePing(iSvc), fileOhi(iSvc), fileTab(iSvc), fileSendq(iSvc),
                                     fileNotc(iSvc), fileTag(iSvc)} {
      err = os.Symlink("empty", aFile)
      if err != nil && !os.IsExist(err) { quit(err) }
   }
}

func getTabsService(iSvc string) []tTermEl {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   return append([]tTermEl{}, aSvc.tabs...)
}

func addTabService(iSvc string, iTerm *tTermEl) int {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   for a := range aSvc.tabs {
      if aSvc.tabs[a].Term == iTerm.Term && aSvc.tabs[a].Label == iTerm.Label {
         return a
      }
   }
   aSvc.tabs = append(aSvc.tabs, *iTerm)
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

func sendAliasService(iW io.Writer, iSvc string, iQid, iId string) error {
   aHead, err := json.Marshal(Msg{"Op":3, "Id":iId, "Newalias":iQid})
   if err != nil { quit(err) }
   err = writeHeaders(iW, aHead, nil)
   return err
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
   case eSrecAlias:  aFn = sendAliasService
   case eSrecNode:   aFn = sendUserEditNode
   case eSrecSync:   aFn = sendSyncNode
   default:
      quit(tError("unknown op " + iSrec.Id[:1]))
   }
   err := aFn(iW, iSvc, iSrec.Id[1:], iSrec.Id)
   if err != nil && err.Error() == "already sent" {
      dropQueue(iSvc, iSrec.Id)
   }
   return err
}

func ErrorService(iErr error) []string { return []string{"_e", iErr.Error()} }

func LogoutService(iSvc string) []string {
   dropFromOhi(iSvc)
   return []string{"of"}
}

func HandleTmtpService(iSvc string, iHead *Header, iR io.Reader) (
                       aFn func(*ClientState)[]string, aToAll []string) {
   var err error
   var aResult []string
   fAll := func(c *ClientState) []string { return aResult }
   fErr := func(c *ClientState) []string { return []string{"_e", iHead.Op +" "+ err.Error()} }

   switch iHead.Op {
   case "tmtprev":
      //todo
   case "registered":
      var aAlias, aUid string
      _editConfig(iSvc, func(cCfg *tSvcConfig) error {
         cCfg.Uid, cCfg.Node = iHead.Uid, iHead.NodeId
         if iHead.Error != "" {
            cCfg.Alias = ""
            cCfg.Error = iHead.Error
         }
         aAlias, aUid = cCfg.Alias, cCfg.Uid
         return nil
      })
      if iHead.Error != "" {
         aFn, aResult = fAll, []string{"cf", "_e", iHead.Error}
         break
      }
      storeSelfAdrsbk(iSvc, aAlias, aUid) //todo check for this on init
      aFn, aResult = fAll, []string{"cf"}
   case "login":
      fmt.Printf("HandleTmtpService %s: login %s\n", iSvc, iHead.Node)
   case "info":
      setFromOhi(iSvc, iHead)
      aFn, aResult = fAll, []string{"of"}
   case "user":
      if iHead.NewAlias != "" {
         _editConfig(iSvc, func(cCfg *tSvcConfig) error {
            if cCfg.Alias != "" {
               err = tError("unexpected newalias: "+ iHead.NewAlias)
               fmt.Fprintf(os.Stderr, "HandleTmtpService %s: %s\n", iSvc, err)
               return err
            }
            cCfg.Error = ""
            cCfg.Alias = iHead.NewAlias //todo support multiple aliases
            return nil
         })
         if err != nil { return fErr, nil }
         aFn, aResult = fAll, []string{"cf"}
         break
      }
      aNd := _findNode(iSvc, iHead.NewNode)
      if aNd != nil {
         if aNd.Status != eNodePending && aNd.Status != eNodeSent {
            fmt.Fprintf(os.Stderr, "HandleTmtpService %s: user node %s already '%c'\n",
                                   iSvc, iHead.NewNode, aNd.Status)
            break
         }
         go func(cQid, cNewNode, cNodeId string) { // start "_node" after ack received
            for c := 70; hasQueue(iSvc, eSrecNode, cQid); c += c/2 {
               if c > 2000 { c = 2000 }
               time.Sleep(time.Duration(c) * time.Millisecond)
               //todo break on dropped service connection
            }
            sMsgToSelfFn(iSvc, &Header{Op:"_node", NewNode:cNewNode, NodeId:cNodeId})
         }(aNd.Qid, iHead.NewNode, iHead.NodeId)
         aNd.Status, aNd.NodeId, aNd.Qid = eNodeAllowed, iHead.NodeId, ""
         _updateNode(iSvc, aNd)
      } else {
         _addNode(iSvc, &tNode{Name:iHead.NewNode, Status:eNodeActive})
      }
      aFn, aResult = fAll, []string{"cf"}
   case "_node": // via sMsgToSelfFn
      aNd := _findNode(iSvc, iHead.NewNode)
      if aNd == nil {
         quit(tError("_node message for unknown Newnode: "+ iHead.NewNode))
      }
      err = replicateNode(iSvc, aNd)
      if err != nil { return fErr, nil }
      aNd.Status = eNodeReady
      _updateNode(iSvc, aNd)
      err = completeNode(iSvc, nil, aNd)
      if err != nil { return fErr, nil }
      aNd.Status, aNd.NodeId = eNodeActive, ""
      _updateNode(iSvc, aNd)
      aFn, aResult = fAll, []string{"cf", "cn"}
   case "ohi":
      updateFromOhi(iSvc, iHead)
      aFn, aResult = fAll, []string{"of"}
   case "ohiedit":
      updateOhi(iSvc, iHead)
      aFn, aResult = fAll, []string{"ot"}
   case "ping":
      err = storeReceivedAdrsbk(iSvc, iHead, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: ping error %s\n", iSvc, err.Error())
         return fErr, nil
      }
      aFn, aResult = fAll, []string{"pf", "pt"}
      aToAll = []string{"/v"}
   case "invite":
      err = storeReceivedAdrsbk(iSvc, iHead, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: invite error %s\n", iSvc, err.Error())
         return fErr, nil
      }
      aFn, aResult = fAll, []string{"pf", "pt"}
      aToAll = []string{"/v"}
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
         return fErr, nil
      }
      if aGot == "thread" {
         aFn, aResult = fAll, []string{"pt", "pf", "tl", "/v"}
      } else if aGot == "msg" {
         aFn = func(c *ClientState) []string {
            if c.getThread() == iHead.SubHead.ThreadId { return aResult }
            return aResult[:4]
         }
         aResult = []string{"pt", "pf", "tl", "/v", "al", "ml"}
      }
   case "notify":
      err = storeFwdNotifyThread(iSvc, iHead, iR)
      if err != nil {
         fmt.Fprintf(os.Stderr, "HandleTmtpService %s: delivery error %s\n", iSvc, err.Error())
         return fErr, nil
      }
      aFn = func(c *ClientState) []string {
         if c.getThread() == iHead.SubHead.ThreadId { return aResult }
         return aResult[:2]
      }
      aResult = []string{"pt", "pf", "cl"}
   case "ack":
      aQid := iHead.Id
      iHead.Id = iHead.Id[1:]
      aId := parseLocalId(iHead.Id)
      switch aQid[0] {
      case eSrecAlias:
         if iHead.Error != "" {
            _editConfig(iSvc, func(cCfg *tSvcConfig) error {
               cCfg.Error = iHead.Error
               return nil
            })
            aFn, aResult = fAll, []string{"cf"}
         }
         dropQueue(iSvc, aQid)
      case eSrecNode:
         if iHead.Error != "" {
            aFn, aResult = fAll, []string{"_e", iHead.Error}
         } else {
            aNd := _findNode(iSvc, aId.info())
            if aNd == nil {
               quit(tError("ack for unknown node: "+ aId.info()))
            }
            if aNd.Status == eNodePending {
               aNd.Status = eNodeSent
               _updateNode(iSvc, aNd)
               aFn, aResult = fAll, []string{"cf"}
            }
         }
         dropQueue(iSvc, aQid)
      case eSrecSync:
         if iHead.Error != "" {
            aFn, aResult = fAll, []string{"_e", iHead.Error}
            dropQueue(iSvc, aQid)
            break
         }
         dropSyncNode(iSvc, iHead.Id, aQid, "")
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
         } else {
            aFn, aResult = fAll, []string{"pf", "gl"}
         }
         dropQueue(iSvc, aQid)
      case eSrecThread:
         if iHead.Error != "" {
            aTid := aId.tid(); if aTid == "" { aTid = iHead.Id }
            aFn = func(c *ClientState) []string {
               if c.getThread() == aTid { return aResult }
               return aResult[1:]
            }
            aResult = []string{"ml", "_e", iHead.Error}
            dropQueue(iSvc, aQid)
            break
         }
         storeSentThread(iSvc, iHead, aQid)
         if aId.tid() == "" {
            aFn = func(c *ClientState) []string {
               c.renameThread(iHead.Id, iHead.MsgId)
               if c.getThread() == iHead.MsgId { return aResult }
               return aResult[1:3]
            }
         } else {
            aFn = func(c *ClientState) []string {
               c.renameMsg(aId.tid(), iHead.Id, iHead.MsgId)
               if c.getThread() == aId.tid() { return aResult }
               return aResult[1:3]
            }
         }
         aResult = []string{"cs", "tl", "pf", "cl", "al", "ml", "mn", iHead.MsgId}
      case eSrecFwd:
         if iHead.Error != "" {
            aFn = func(c *ClientState) []string {
               if c.getThread() == aId.tid() { return aResult }
               return aResult[1:]
            }
            aResult = []string{"cl", "_e", iHead.Error}
            dropQueue(iSvc, aQid)
            break
         }
         storeFwdSentThread(iSvc, iHead, aQid)
         aFn = func(c *ClientState) []string {
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
         dropQueue(iSvc, aQid)
      default:
         quit(tError("bad SendRecord " + aQid))
      }
   default:
      err = tError("unknown tmtp op")
      return fErr, nil
   }
   return aFn, aToAll
}

func HandleSyncService(iSvc string, iHead *Header, iR io.Reader,
                       iNotify func(func(*ClientState)[]string, []string)) {
   if iHead.From != GetConfigService(iSvc).Uid {
      fmt.Fprintf(os.Stderr, "HandleSyncService %s: message from foreign uid %s\n", iSvc, iHead.From)
      _ = discardTmtp(iHead, iR)
      return
   }
   aBuf := make([]byte, iHead.DataLen)
   aLen, err := iR.Read(aBuf)
   if err != nil || aLen != len(aBuf) {
      return
   }
   var aUpdates []Update
   err = json.Unmarshal(aBuf, &aUpdates)
   if err != nil {
      fmt.Fprintf(os.Stderr, "HandleSyncService %s: %v\n", iSvc, err)
      return
   }
   if len(aUpdates) == 0 {
      fmt.Fprintf(os.Stderr, "HandleSyncService %s: missing updates\n", iSvc)
   } else {
      aCs := ClientState{History:[]string{""}}
      for a := range aUpdates {
         aCs.History[0] = aUpdates[a].LogThreadId
         if aUpdates[a].LogOp != "" {
            aUpdates[a].Op = aUpdates[a].LogOp
         }
         aUpdates[a].log = eLogNone
         iNotify(HandleUpdtService(iSvc, &aCs, &aUpdates[a]))
      }
   }
}

func HandleUpdtService(iSvc string, iState *ClientState, iUpdt *Update) (
                       aFn func(*ClientState)[]string, aToAll []string) {
   var err error
   var aResult []string
   fAll := func(c *ClientState) []string { return aResult }
   fOne := func(c *ClientState) []string { if c != iState { return nil }; return aResult }
   fErr := func(c *ClientState) []string { if c != iState { return nil }
                                           return []string{"_e", iUpdt.Op +" "+ err.Error()} }

   if iUpdt.Op != "open" {
      if iSvc == "local" {
         err = tError("not supported")
         return fErr, nil
      }
      aSvc := getService(iSvc)
      aSvc.updt.RLock(); defer aSvc.updt.RUnlock() //todo use TryRLock()
   }

   switch iUpdt.Op {
   case "open":
      aResult = []string{"cf", "cn", "of", "ot", "ps", "pt", "pf", "gl",
                         "tl", "cs", "cl", "al", "_t", "ml", "mo",
                         "/v", "/t", "/f", "/g", "/l",
                         "_e", ""}
      aLen := len(aResult) - 2
      if iSvc == "local" {
         aFn, aResult = fOne, aResult[15:aLen]
      } else {
         //todo aToAll return []string{"/v"} to update .UnreadN everywhere? (also thread_open & delivery)
         _initUnreadCount(iSvc)
         aCfg := GetConfigService(iSvc)
         if aCfg.Error != "" {
            aLen += 2
            aResult[aLen-1] = aCfg.Error
         }
         aFn, aResult = fOne, aResult[:aLen]
      }
   case "config_update":
      if iUpdt.Config.Addr != "" && iUpdt.Config.Addr[0] != '+' && iUpdt.Config.Addr[0] != '=' {
         err = tError("address requires prefix + or =")
         return fErr, nil
      }
      if iUpdt.log == 0 && iUpdt.Config.Alias != "" {
         addQueue(iSvc, eSrecAlias, iUpdt.Config.Alias)
      }
      syncUpdtNode(iSvc, iUpdt, iState, func() error {
         _editConfig(iSvc, func(cCfg *tSvcConfig) error {
            if iUpdt.Config.Addr != "" {
               cCfg.Verify = iUpdt.Config.Addr[0] == '+'
               cCfg.Addr = iUpdt.Config.Addr[1:]
            }
            if iUpdt.Config.LoginPeriod >= 0 {
               cCfg.LoginPeriod = iUpdt.Config.LoginPeriod
            }
            if iUpdt.Config.HistoryLen >= 4 && iUpdt.Config.HistoryLen <= 1024 {
               cCfg.HistoryLen = iUpdt.Config.HistoryLen
               iState.setHistoryMax(cCfg.HistoryLen)
            }
            return nil
         })
         return nil
      })
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
         return fErr, nil
      }
      aResult = searchAdrsbk(iSvc, iUpdt)
      aFn, aResult = fOne, append([]string{"_n"}, aResult...)
   case "notice_seen":
      syncUpdtNode(iSvc, iUpdt, iState, func() error {
         err = setLastSeenNotice(iSvc, iUpdt)
         return err
      })
      if err != nil { return fErr, nil }
      aToAll = []string{"/v"}
   case "thread_save":
      const ( _ int8 = iota; eNewThread; eNewReply )
      if iUpdt.Thread.New > 0 {
         aTid := ""; if iUpdt.Thread.New == eNewReply { aTid = iState.getThread() }
         iUpdt.Thread.Id = makeLocalId(aTid)
      }
      storeDraftThread(iSvc, iUpdt)
      if iUpdt.Thread.New == eNewThread {
         iState.addThread(iUpdt.Thread.Id)
         aFn = func(c *ClientState) []string {
            if c == iState { return aResult }
            return aResult[:1]
         }
         aResult = []string{"tl", "cs", "al", "_T", "cl", "ml", "mo"}
      } else if iUpdt.Thread.New == eNewReply {
         iState.openMsg(iUpdt.Thread.Id, true, true)
         aTid := iState.getThread()
         aFn = func(c *ClientState) []string {
            if c.getThread() == aTid { return aResult }
            return nil
         }
         aResult = []string{"tl", "al", "ml", "mn", iUpdt.Thread.Id}
      } else { // may update msg from a threadid other than iState.getThread()
         aFn = func(c *ClientState) []string {
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
         aFn = func(c *ClientState) []string {
            defer c.discardThread(aTid)
            if c.getThread() == aTid { return aResult }
            return aResult[:1]
         }
         aResult = []string{"tl", "cs", "cl", "al", "_t", "ml", "mo"}
      } else {
         aFn = func(c *ClientState) []string {
            c.openMsg(iUpdt.Thread.Id, false, true)
            if c.getThread() == aTid { return aResult }
            return nil
         }
         aResult = []string{"tl", "al", "ml"}
      }
   case "thread_send":
      if iUpdt.Thread.Id == "" { break }
      err = validateDraftThread(iSvc, iUpdt)
      if err != nil { return fErr, nil }
      aTid := iState.getThread()
      aFn = func(c *ClientState) []string {
         if c.getThread() == aTid { return aResult }
         return nil
      }
      aResult = []string{"ml"}
      addQueue(iSvc, eSrecThread, iUpdt.Thread.Id)
   case "thread_open":
      if iUpdt.log == 0 && iUpdt.Touch.ThreadId != iState.getThread() {
         err = tError("thread id out of sync")
         return fErr, nil
      }
      aChg := false
      iUpdt.LogOp = "thread_seen_sync"
      syncUpdtNode(iSvc, iUpdt, iState, func() error {
         iState.openMsg(iUpdt.Touch.MsgId, true, false)
         aChg = touchThread(iSvc, iUpdt)
         if !aChg { return tError("") } // no sync
         return nil
      })
      if !aChg { break }
      aFn = func(c *ClientState) []string {
         if c.getThread() == iUpdt.Touch.ThreadId { return aResult }
         return aResult[:2]
      }
      aResult = []string{"tl", "/v", "ml"}
   case "thread_seen_sync":
      touchThread(iSvc, iUpdt)
      aFn = func(c *ClientState) []string {
         if c.getThread() == iUpdt.Touch.ThreadId { return aResult }
         return aResult[:2]
      }
      aResult = []string{"tl", "/v", "ml"}
   case "thread_close":
      iState.openMsg(iUpdt.Touch.MsgId, false, false)
      // no result
   case "thread_tag":
      if iUpdt.log == 0 {
         iUpdt.Touch.TagName = mustCopyTag(iSvc, iUpdt.Touch.TagId)
         iUpdt.Touch.ThreadId = iState.getThread()
      }
      syncUpdtNode(iSvc, iUpdt, iState, func() error {
         if iUpdt.Touch.TagName != "" {
            addTag(iSvc, iUpdt.Touch.TagName, iUpdt.Touch.TagId)
         }
         touchThread(iSvc, iUpdt)
         return nil
      })
      aFn = func(c *ClientState) []string {
         _, cTabVal := c.getSvcTab()
         if cTabVal[0] == '#' && GetIdTag(cTabVal[1:]) == iUpdt.Touch.TagId {
            if c.getThread() == iUpdt.Touch.ThreadId { return aResult }
            return aResult[:2]
         }
         if c.getThread() == iUpdt.Touch.ThreadId { return aResult[1:] }
         return aResult[1:2]
      }
      aResult = []string{"tl", "/g", "ml"}
   case "forward_save":
      storeFwdDraftThread(iSvc, iUpdt)
      aFn = func(c *ClientState) []string {
         if c.getThread() == iUpdt.Forward.ThreadId { return aResult }
         return nil
      }
      aResult = []string{"cl"}
   case "forward_send":
      aFn = func(c *ClientState) []string {
         if c.getThread() == iUpdt.Forward.ThreadId { return aResult }
         return nil
      }
      aResult = []string{"cl"}
      addQueue(iSvc, eSrecFwd, iUpdt.Forward.Qid)
   case "tag_add":
      if iUpdt.log == 0 {
         iUpdt.Tag.Id = makeIdTag()
      }
      syncUpdtNode(iSvc, iUpdt, iState, func() error {
         addTag(iSvc, iUpdt.Tag.Name, iUpdt.Tag.Id)
         return nil
      })
      if err != nil { return fErr, nil }
      aToAll = []string{"/g"}
   case "navigate_thread":
      iState.addThread(iUpdt.Navigate.ThreadId)
      aFn, aResult = fOne, []string{"cs", "cl", "al", "_t", "ml", "mo"}
   case "navigate_history":
      iState.goThread(iUpdt.Navigate.History)
      aFn, aResult = fOne, []string{"cs", "cl", "al", "_t", "ml", "mo"}
   case "navigate_link":
      _, err = os.Lstat(dirThread(iSvc) + iUpdt.Navigate.ThreadId)
      if err != nil { return fErr, nil }
      aDiff := iUpdt.Navigate.ThreadId != iState.getThread()
      iState.goLink(iUpdt.Navigate.Label, iUpdt.Navigate.ThreadId, iUpdt.Navigate.MsgId)
      aFn = fOne
      aResult = []string{"cs", "mo"}; if aDiff { aResult = []string{"cs", "cl", "al", "_t", "ml", "mo"} }
   case "tab_add":
      iState.addTab(iUpdt.Tab.Type, iUpdt.Tab.Term)
      aAlt := "tl"; if iUpdt.Tab.Type == eTabThread { aAlt = "mo" }
      aFn, aResult = fOne, []string{"cs", aAlt}
   case "tab_pin":
      if iUpdt.log == 0 {
         iUpdt.Tab.Term = iState.getTab(iUpdt.Tab.Type).Term
      }
      iUpdt.LogOp = "tab_pin_sync"
      syncUpdtNode(iSvc, iUpdt, iState, func() error {
         iState.pinTab(iUpdt.Tab.Type)
         return nil
      })
      aFn, aResult = fAll, []string{"cs"}
   case "tab_pin_sync":
      addTabService(iSvc, newTermEl(iUpdt.Tab.Term, ""))
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
   case "node_add":
      aNd := _findNode(iSvc, iUpdt.Node.Newnode)
      aIsNew := aNd == nil
      if aIsNew {
         aNd = &tNode{Name:iUpdt.Node.Newnode, Status:eNodePending, Qid:makeLocalId(iUpdt.Node.Newnode)}
         _addNode(iSvc, aNd)
      }
      if aNd.Status != eNodeReady {
         err = createNode(iSvc, iUpdt, aNd)
         if err != nil {
            if aIsNew {
               _dropNode(iSvc, aNd)
            }
            return fErr, nil
         }
      } else {
         err = completeNode(iSvc, iUpdt, aNd)
         if err != nil { return fErr, nil }
         aNd.Status, aNd.NodeId = eNodeActive, ""
         _updateNode(iSvc, aNd)
      }
      aFn, aResult = fAll, []string{"cf", "cn"}
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
      return fErr, nil
   }
   return aFn, aToAll
}

// only for testing
func WipeDataService(iSvc string) error {
   aCfgTmp := kStorageDir +"svc-"+ url.QueryEscape(iSvc) +"-config"
   err := os.Rename(fileCfg(iSvc), aCfgTmp)
   if err != nil { return err }
   err = os.RemoveAll(dirSvc(iSvc))
   if err != nil { quit(err) }
   makeTreeService(iSvc)
   err = os.Rename(aCfgTmp, fileCfg(iSvc))
   if err != nil { quit(err) }
   var aCfg tSvcConfig
   err = readJsonFile(&aCfg, fileCfg(iSvc))
   if err != nil { quit(err) }
   aCfg.HistoryLen = kServiceHistoryMax
   err = storeFile(fileCfg(iSvc), &aCfg)
   if err != nil { quit(err) }
   patchSelfAdrsbk(iSvc, aCfg.Alias, aCfg.Uid)
   return nil
}
