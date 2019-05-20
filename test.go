// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package main

import (
   "sync/atomic"
   "encoding/base32"
   "bytes"
   "flag"
   "fmt"
   "encoding/json"
   "os"
   pSl "github.com/networkimprov/mnm-hammer/slib"
   "sort"
   "strconv"
   "strings"
   "sync"
   "time"
)

const kTestDateF = "0102150405"
var sTestBase32 = base32.NewEncoding("%+123456789BCDFGHJKLMNPQRSTVWXYZ")

var sTestHost = "" // used by main.go
var sTestCrash = ""
var sTestVerify = ""
var sTestCrashOp = ""
var sTestCrashSvc = ""
var sTestOrderCrash uint64
var sTestOrderN uint64

var sTestOrderLast uint64    // atomic
var sTestOrderPolling uint32 // atomic
var sTestState []tTestStateEl // used by main.go
var sTestNow = time.Now().Truncate(time.Second)
var sTestDate = sTestNow.Format(" "+kTestDateF)
var sTestDateGid = time.Now().Format(":"+kTestDateF+".000")
var sTestExit = false

type tTestStateEl struct {
   svcId, name string
   state *pSl.ClientState
}

type tTestClient struct {
   Version string
   Name string
   SvcId string
   Cfg struct {
      Name, Addr, Alias string
   }
   Files []struct {
      Name, Data string
   }
   Forms []struct {
      Name string
      Ffn string         `json:"ffn,omitempty"`
      Fields interface{} `json:"fields,omitempty"`
   }
   Orders []struct {
      Updt pSl.Update
      Result map[string]interface{}
   }
}

type tTestContext struct {
   svcId string
   lastId tTestLastId
   state *pSl.ClientState
   wg sync.WaitGroup
}

type tTestLastId map[string]*tTestAnyId // key is service op

type tTestAnyId []struct {
   Id, Qid, Uid, File string
}

func init() {
   flag.StringVar(&sTestHost, "test", sTestHost,
                  "run test sequence using named service host:port")
   flag.StringVar(&sTestCrash, "crash", sTestCrash,
                  "exit transaction at dir:service:orderIdx:op, or setup & print dir with 'init'")
   flag.StringVar(&sTestVerify, "verify", sTestVerify,
                  "resume after crash and check result for dir:service:orderIdx:count")
}

func crashTest(iSvc string, iOp string) {
   if iSvc != sTestCrashSvc || iOp != sTestCrashOp {
      //if sTestCrash != "" { fmt.Printf("crash  --  %s %s\n", iSvc, iOp) }
      return
   }
   // when called via pSl.HandleTmtpService(), wait for _runTestClient() to reach a .Test.Poll
   a := 0
   for atomic.LoadUint64(&sTestOrderLast) < sTestOrderCrash &&
       atomic.LoadUint32(&sTestOrderPolling) == 0 {
      time.Sleep(15 * time.Millisecond)
      a++
   }
   if a > 0 { fmt.Printf("crash wait %d for %s %d %s\n", a, iSvc, sTestOrderCrash, iOp) }
   if atomic.LoadUint64(&sTestOrderLast) != sTestOrderCrash {
      return
   }
   fmt.Printf("crash test %s %d %s\n", iSvc, sTestOrderCrash, iOp)
   err := sHttpSrvr.Close()
   if err != nil { quit(err) }
   time.Sleep(15 * time.Second)
   quit(tError("failed to exit"))
}

func test() int {
   aDir := "test-run/" + sTestDate[1:]
   var err error
   var aClients []tTestClient

   aFd, err := os.Open("test-in.json")
   if err != nil { quit(err) }
   defer aFd.Close()
   err = json.NewDecoder(aFd).Decode(&aClients)
   if err != nil { quit(err) }

   aAbout := getAbout()
   for a := range aClients {
      if aClients[a].Version != "" && aClients[a].Version != aAbout.Version {
         fmt.Fprintf(os.Stderr, "test-in expects v%s, app is v%s\n", aClients[a].Version, aAbout.Version)
         return 33
      }
   }

   if sTestVerify != "" {
      aDir, err = _setupTestVerify(aClients) // triggers receipt of msgs pending from -crash run
      if err != nil {
         fmt.Fprintf(os.Stderr, "invalid -verify parameter '%s': %v\n", sTestVerify, err)
         return 33
      }
      time.Sleep(600 * time.Millisecond) // handle msgs pending from -crash run
      var aTc *tTestClient
      for a := range aClients {
         sTestState = append(sTestState, tTestStateEl{aClients[a].SvcId, aClients[a].Name,
                                                      pSl.OpenState(aClients[a].Name, aClients[a].SvcId)})
         if aClients[a].SvcId == sTestCrashSvc {
            aTc = &aClients[a]
         }
      }
      _runTestClient(aTc, nil)
      return 0
   }
   if sTestCrash == "init" {
      if !_setupTestDir(aDir, aClients) {
         return 33
      }
      fmt.Printf("%s\n", aDir)
      return 0
   }
   if sTestCrash != "" {
      aDir, err = _setupTestCrash(aClients)
      if err != nil {
         fmt.Fprintf(os.Stderr, "invalid -crash parameter '%s': %v\n", sTestCrash, err)
         return 33
      }
   } else {
      fmt.Printf("start test pass in %s\n", aDir)
      if !_setupTestDir(aDir, aClients) {
         fmt.Printf("end test pass. http on %s\n", sHttpSrvr.Addr)
         return -1
      }
   }
   for a := range aClients {
      sTestState = append(sTestState, tTestStateEl{aClients[a].SvcId, aClients[a].Name,
                                                   pSl.OpenState(aClients[a].Name, aClients[a].SvcId)})
   }
   var aWg sync.WaitGroup
   for a := range aClients {
      aWg.Add(1)
      go _runTestClient(&aClients[a], &aWg)
   }
   go func() {
      aWg.Wait()
      if sTestCrash != "" {
         fmt.Printf("crash not triggered\n")
         sTestExit = true
      } else {
         fmt.Printf("end test pass. http on %s\n", sHttpSrvr.Addr)
      }
      if sTestExit {
         err = sHttpSrvr.Close()
         if err != nil { quit(err) }
      }
   }()
   return -1
}

func _setupTestDir(iDir string, iClients []tTestClient) bool {
   var err error

   err = os.MkdirAll(iDir, 0700)
   if err != nil { quit(err) }
   err = os.Chdir(iDir)
   if err != nil { quit(err) }
   err = os.Symlink("../../web", "web")
   if err != nil { quit(err) }
   err = os.Symlink("../../formspec", "formspec")
   if err != nil { quit(err) }

   pSl.Init(startService, crashTest)

   var aTc *tTestClient
   var aBuf bytes.Buffer
   aEnc := json.NewEncoder(&aBuf)
   for a := range iClients {
      aTc = &iClients[a]
      if aTc.Cfg.Name != "" {
         aTc.Cfg.Addr = sTestHost
         aTc.Cfg.Alias += sTestDate
         err = aEnc.Encode(aTc.Cfg)
         if err != nil { quit(err) }
         err = pSl.Service.Add(aTc.SvcId, "", &aBuf)
         if err != nil { goto ReturnErr }
      }
      for a1 := range aTc.Files {
         _, err = aBuf.WriteString(aTc.Files[a1].Data)
         if err != nil { quit(err) }
         err = pSl.Upload.Add(aTc.Files[a1].Name, "", &aBuf)
         if err != nil { goto ReturnErr }
         err = pSl.Upload.Add(aTc.Files[a1].Name, "Dup", nil)
         if err != nil { goto ReturnErr }
         err = pSl.Upload.Drop(aTc.Files[a1].Name+".Dup")
         if err != nil { goto ReturnErr }
      }
      for a1 := range aTc.Forms { // expects .spec file first
         err = aEnc.Encode(aTc.Forms[a1])
         if err != nil { quit(err) }
         err = pSl.BlankForm.Add(aTc.Forms[a1].Name, "", &aBuf)
         if err != nil { goto ReturnErr }
         aPair := strings.SplitN(aTc.Forms[a1].Name, ".", 2)
         if len(aPair) == 1 || aPair[1] != "spec" {
            err = pSl.BlankForm.Add(aTc.Forms[a1].Name, "Dup", nil)
            if err != nil { goto ReturnErr }
            err = pSl.BlankForm.Drop(aPair[0]+".Dup")
            if err != nil { goto ReturnErr }
         }
      }
      if aTc.Cfg.Name == "" { continue }
      for {
         aCfg := pSl.GetConfigService(aTc.SvcId)
         if aCfg.Error != "" {
            err = tError(aCfg.Error)
            goto ReturnErr
         }
         if aCfg.Uid != "" { break }
         time.Sleep(50 * time.Millisecond)
      }
   }
   return true
   ReturnErr:
      fmt.Fprintf(os.Stderr, "%s %s %s\n", aTc.Name, aTc.SvcId, err)
      return false
}

func _setupTestCrash(iClients []tTestClient) (_ string, err error) {
   aArg := []string{"","","",""}
   copy(aArg, strings.Split(sTestCrash, ":"))
   if !strings.HasPrefix(aArg[0], "test-run/") {
      return "", tError("invalid dir")
   }
   if aArg[3] == "" {
      return "", tError("missing op")
   }
   sTestOrderCrash, err = strconv.ParseUint(aArg[2], 10, 64)
   if err != nil { return }
   sTestNow, err = time.Parse(kTestDateF, aArg[0][9:]) // omit "test-run/"
   if err != nil { return }
   sTestNow = sTestNow.AddDate(time.Now().Year(), 0, 0)
   sTestDate = sTestNow.Format(" "+kTestDateF)
   sTestCrashSvc = aArg[1]
   sTestCrashOp = aArg[3]

   err = os.Chdir(aArg[0])
   if err != nil { return }
   err = os.RemoveAll("store/state")
   if err != nil { return }
   for a := range iClients {
      if iClients[a].Cfg.Name == "" { continue }
      err = pSl.WipeDataService(iClients[a].SvcId)
      if err != nil { return }
   }
   pSl.Init(startService, crashTest)
   if sServices[aArg[1]].queue == nil {
      return "", tError("invalid service")
   }
   return aArg[0], nil
}

func _setupTestVerify(iClients []tTestClient) (_ string, err error) {
   aArg := []string{"","","",""}
   copy(aArg, strings.Split(sTestVerify, ":"))
   if !strings.HasPrefix(aArg[0], "test-run/") {
      return "", tError("invalid dir")
   }
   sTestOrderCrash, err = strconv.ParseUint(aArg[2], 10, 64)
   if err != nil { return }
   sTestOrderN, err = strconv.ParseUint(aArg[3], 10, 64)
   if err != nil { return }
   sTestNow, err = time.Parse(kTestDateF, aArg[0][9:]) // omit "test-run/"
   if err != nil { return }
   sTestNow = sTestNow.AddDate(time.Now().Year(), 0, 0)
   sTestDate = sTestNow.Format(" "+kTestDateF)
   sTestCrashSvc = aArg[1]

   err = os.Chdir(aArg[0])
   if err != nil { return }
   pSl.Init(startService, crashTest)
   if sServices[aArg[1]].queue == nil {
      return "", tError("invalid service")
   }
   var aTc *tTestClient
   for a := range iClients {
      if iClients[a].SvcId == aArg[1] {
         aTc = &iClients[a]
         break
      }
   }
   if sTestOrderN == 0 || sTestOrderCrash+sTestOrderN > uint64(len(aTc.Orders)) {
      return "", tError("invalid order range")
   }
   aTc.Orders = aTc.Orders[sTestOrderCrash : sTestOrderCrash+sTestOrderN]
   aOrder := &aTc.Orders[0]
   if aOrder.Updt.Op != "test" {
      aOrder.Updt.Op = "test"
      aOrder.Updt.Test = &pSl.UpdateTest{Request:make([]string, 0, len(aOrder.Result))}
      for aK := range aOrder.Result {
         aOrder.Updt.Test.Request = append(aOrder.Updt.Test.Request, aK)
      }
   }
   for a := range aTc.Orders {
      if aTc.Orders[a].Updt.Op != "test" {
         return "", tError("Updt.Op not 'test': "+ aTc.Orders[a].Updt.Op)
      }
      aTc.Orders[a].Updt.Test.Poll = 0
   }
   return aArg[0], nil
}

func _runTestClient(iTc *tTestClient, iWg *sync.WaitGroup) {
   if iWg != nil {
      defer iWg.Done()
   }
   aSvc := getService(iTc.SvcId)
   if aSvc.ccs == nil {
      fmt.Fprintf(os.Stderr, "%s %s client quit; service unknown\n", iTc.Name, iTc.SvcId)
      return
   }
   aCtx := tTestContext{
      svcId: iTc.SvcId,
      lastId: tTestLastId{"tl":{}, "al":{}, "ml":{}, "cl":{}, "mn":{}, "ps":{}, "pf":{}},
   }
   for a := range sTestState {
      if sTestState[a].name == iTc.Name { aCtx.state = sTestState[a].state }
   }

   for a := range iTc.Orders {
      aUpdt := &iTc.Orders[a].Updt
      aPrefix := fmt.Sprintf("%s %s %s", iTc.Name, iTc.SvcId, aUpdt.Op)
      if !_prepUpdt(aUpdt, &aCtx, aPrefix) {
         continue
      }
      if iTc.SvcId == sTestCrashSvc {
         atomic.StoreUint64(&sTestOrderLast, uint64(a))
      }
      aFn := pSl.HandleUpdtService(iTc.SvcId, aCtx.state, aUpdt)
      var aOps []string
      if aFn != nil {
         aSvc.ccs.Range(func(cC *tWsConn) {
            cMsg := aFn(cC.state)
            if cMsg == nil { return }
            cC.WriteJSON(cMsg)
         })
         aMsg := aFn(aCtx.state)
         aOps, _ = aMsg.([]string)
         if aOps == nil {
            fmt.Fprintf(os.Stderr, "%s update error %s\n", aPrefix, aMsg.(string))
            continue
         }
      }
      if iTc.Orders[a].Result == nil {
         iTc.Orders[a].Result = make(map[string]interface{})
      }
      for aK, aV := range iTc.Orders[a].Result {
         a1 := 0
         for ; a1 < len(aOps) && aOps[a1] != aK; a1++ {}
         if a1 == len(aOps) {
            fmt.Fprintf(os.Stderr, "%s missing result\n  expect %s %v\n", aPrefix, aK, aV)
         }
      }
      var aSum *int32 = nil; if aUpdt.Test != nil && aUpdt.Test.Poll > 0 { aSum = new(int32) }
      for aTryN := 1; true; aTryN++ {
         for a1 := 0; a1 < len(aOps); a1++ {
            aOp, aId := aOps[a1], ""
            if aOp == "_n" {
               _verifyNameList(aOps[1+a1:], iTc.Orders[a].Result[aOp], aPrefix +" "+ aOp)
               break
            }
            if aOp == "_t" { continue }
            if aOp == "mn" || aOp == "an" || aOp == "fn" {
               a1++
               aId = aOps[a1]
            }
            aCtx.wg.Add(1)
            go _runTestService(&aCtx, aOp, aId, iTc.Orders[a].Result[aOp], aPrefix, aSum, aTryN)
         }
         aCtx.wg.Wait()
         if aTryN == 5 || aSum == nil || *aSum == int32(len(aOps)) {
            if iTc.SvcId == sTestCrashSvc {
               atomic.StoreUint32(&sTestOrderPolling, 0)
            }
            break
         }
         if iTc.SvcId == sTestCrashSvc {
            atomic.StoreUint32(&sTestOrderPolling, 1)
         }
         time.Sleep(aUpdt.Test.Poll * time.Millisecond)
         *aSum = 0
      }
      if len(*aCtx.lastId["mn"]) > 0 {
         *aCtx.lastId["ml"] = *aCtx.lastId["mn"]
         *aCtx.lastId["mn"] = tTestAnyId{}
      }
   }
}

func _prepUpdt(iUpdt *pSl.Update, iCtx *tTestContext, iPrefix string) bool {
   var aApply string
   switch iUpdt.Op {
   case "config_update":
      if iUpdt.Config.Addr == "orig" {
         iUpdt.Config.Addr = sTestHost
      }
   case "thread_save":
      if iUpdt.Thread.Alias != "" {
         iUpdt.Thread.Alias += sTestDate
      }
      for a := range iUpdt.Thread.Cc {
         iUpdt.Thread.Cc[a].Who += sTestDate
         if iUpdt.Thread.Cc[a].WhoUid == "lookup" {
            iUpdt.Thread.Cc[a].WhoUid = pSl.LookupAdrsbk(iCtx.svcId, iUpdt.Thread.Cc[a].Who)
         }
      }
      if iUpdt.Thread.FormFill != nil {
         aFf := iUpdt.Thread.FormFill["lastfile"]
         if aFf != "" {
            aFileId := (*iCtx.lastId["al"])[0].File
            delete(iUpdt.Thread.FormFill, "lastfile")
            iUpdt.Thread.FormFill[aFileId] = aFf
            for a := range iUpdt.Thread.Attach {
               if iUpdt.Thread.Attach[a].FfKey == "lastfile" {
                  iUpdt.Thread.Attach[a].FfKey = aFileId
               }
            }
         }
      }
      fallthrough
   case "thread_send", "thread_discard",
        "thread_open", "thread_close":
      _applyLastId(&iUpdt.Thread.Id,         &aApply, iCtx.lastId, "ml")
      _applyLastId(&iUpdt.Thread.ThreadId,   &aApply, iCtx.lastId, "tl")
   case "forward_save":
      for a := range iUpdt.Forward.Cc {
         iUpdt.Forward.Cc[a].Who += sTestDate
         if iUpdt.Forward.Cc[a].WhoUid == "lookup" {
            iUpdt.Forward.Cc[a].WhoUid = pSl.LookupAdrsbk(iCtx.svcId, iUpdt.Forward.Cc[a].Who)
         }
      }
      fallthrough
   case "forward_send":
      _applyLastId(&iUpdt.Forward.ThreadId,  &aApply, iCtx.lastId, "tl")
      _applyLastId(&iUpdt.Forward.Qid,       &aApply, iCtx.lastId, "cl")
   case "adrsbk_search":
      if iUpdt.Adrsbk.Term == "td" {
         iUpdt.Adrsbk.Term = sTestDate[1:3]
      }
   case "ping_save":
      iUpdt.Ping.Alias += sTestDate
      iUpdt.Ping.To    += sTestDate
      if iUpdt.Ping.Gid != "" {
         iUpdt.Ping.Gid += sTestDate
         if sTestCrash != "" {
            iUpdt.Ping.Gid += sTestDateGid
         }
      }
   case "ping_send":
      _applyLastId(&iUpdt.Ping.Qid,          &aApply, iCtx.lastId, "ps")
   case "ping_discard":
      iUpdt.Ping.To += sTestDate
   case "accept_send":
      _applyLastId(&iUpdt.Accept.Qid,        &aApply, iCtx.lastId, "pf")
   case "ohi_add", "ohi_drop":
      iUpdt.Ohi.Alias += sTestDate
      _applyLastId(&iUpdt.Ohi.Uid,           &aApply, iCtx.lastId, "pf")
   case "navigate_thread":
      _applyLastId(&iUpdt.Navigate.ThreadId, &aApply, iCtx.lastId, "tl")
   case "navigate_link":
      _applyLastId(&iUpdt.Navigate.ThreadId, &aApply, iCtx.lastId, "ml")
      _applyLastId(&iUpdt.Navigate.MsgId,    &aApply, iCtx.lastId, "ml")
   case "navigate_history",
        "notice_seen",
        "tab_add", "tab_pin", "tab_drop", "tab_select",
        "open":
      // nothing to do
   case "test":
      _applyLastId(&iUpdt.Test.ThreadId,     &aApply, iCtx.lastId, "tl")
      if iUpdt.Test.Notice != nil {
         aNow := time.Now().UTC()
         for a := range iUpdt.Test.Notice {
            aN, err := strconv.Atoi(iUpdt.Test.Notice[a].Date)
            if err != nil {
               fmt.Fprintf(os.Stderr, "%s notice date %s\n", iPrefix, err)
               return false
            }
            iUpdt.Test.Notice[a].Date = aNow.AddDate(0,0,aN).Format(time.RFC3339)
         }
      }
   default:
      fmt.Fprintf(os.Stderr, "%s unknown update op %s\n", iPrefix, iUpdt.Op)
      return false
   }
   if aApply != "" {
      // fmt.Printf("%s applied %s\n", iPrefix, aApply)
   }
   return true
}

func _applyLastId(iField, iMsg *string, iLastId tTestLastId, iType string) {
   a := -1
   switch *iField {
   case "last", "lastfile", "lastqid": a = 0
   case "2ndlast":                     a = 1
   default:                            return
   }
   aSet := *iLastId[iType]
   switch iType {
   case "tl", "ml": *iField = aSet[a].Id
   case "cl", "ps": *iField = aSet[a].Qid
   case "al":       *iField = aSet[a].File
   case "pf":       if *iField == "lastqid" { *iField = aSet[a].Qid
                    } else                  { *iField = aSet[a].Uid }
   default:         return
   }
   aAmp := ""; if *iMsg != "" { aAmp = " & " }
   *iMsg += aAmp + *iField
}

func _verifyNameList(iList []string, iExpect interface{}, iPrefix string) {
   aGot := make([]interface{}, len(iList))
   for a := range iList { aGot[a] = iList[a] }
   if iExpect == nil {
      fmt.Fprintf(os.Stderr, "%s unexpected\n  got    _n %v\n",
                             iPrefix, aGot)
   } else {
      aName, aMis := _hasExpected("_n", iExpect, aGot)
      if aName != "" {
         fmt.Fprintf(os.Stderr, "%s mismatch\n  expect %v\n  got    %s %v\n",
                                iPrefix, iExpect, aName, aMis)
      }
   }
}

func _runTestService(iCtx *tTestContext, iOp, iId string, iExpect interface{},
                     iPrefix string, iSum *int32, iTryN int) {
   defer iCtx.wg.Done()
   var err error
   var aResult, aMis interface{}
   var aResp bytes.Buffer
   var aName string

   switch(iOp) {
   case "/t": aResult = pSl.Upload.GetIdx()
   case "/f": aResult = pSl.BlankForm.GetIdx()
   case "/v": aResult = pSl.Service.GetIdx()
              sort.Strings(aResult.([]string))
   case "cs": aResult = iCtx.state.GetSummary()
   case "cf": aResult = pSl.GetConfigService(iCtx.svcId)
   case "nl": aResult = pSl.GetIdxNotice(iCtx.svcId)
   case "ps": aResult = pSl.GetDraftAdrsbk(iCtx.svcId)
   case "pt": aResult = pSl.GetSentAdrsbk(iCtx.svcId)
   case "pf": aResult = pSl.GetReceivedAdrsbk(iCtx.svcId)
   case "gl": aResult = pSl.GetGroupAdrsbk(iCtx.svcId)
   case "of": aResult = pSl.GetFromOhi(iCtx.svcId)
   case "ot": aResult = pSl.GetToOhi(iCtx.svcId)
   case "cl": aResult = pSl.GetCcThread(iCtx.svcId, iCtx.state)
   case "al": aResult = pSl.GetIdxAttach(iCtx.svcId, iCtx.state)
   case "ml": aResult = pSl.GetIdxThread(iCtx.svcId, iCtx.state)
   case "tl": err     = pSl.WriteResultSearch(&aResp, iCtx.svcId, iCtx.state)
   case "mo": err     = pSl.WriteMessagesThread(&aResp, iCtx.svcId, iCtx.state, "")
   case "mn": err     = pSl.WriteMessagesThread(&aResp, iCtx.svcId, iCtx.state, iId)
   case "fn": err     = pSl.WriteTableFilledForm(&aResp, iCtx.svcId, iId)
   case "an":
      var aFi os.FileInfo
      aFi, err = os.Lstat(pSl.GetPathAttach(iCtx.svcId, iCtx.state, iId))
      aResult = aFi.Size()
   default:
      err = tError("unknown op")
   }
   if err != nil { goto ReturnErr }
   if aResult != nil {
      err = json.NewEncoder(&aResp).Encode(aResult)
      if err != nil { goto ReturnErr }
   }
   if iOp == "mn" {
      *iCtx.lastId[iOp] = tTestAnyId{{Id:iId}}
   } else if iOp == "cl" {
      aClPair := [2]tTestAnyId{}
      err = json.Unmarshal(aResp.Bytes(), &aClPair)
      if err != nil { goto ReturnErr }
      *iCtx.lastId[iOp] = aClPair[0]
   } else if iOp == "tl" && aResp.Bytes()[0] == '{' {
      // nothing to do
   } else if iCtx.lastId[iOp] != nil {
      err = json.Unmarshal(aResp.Bytes(), iCtx.lastId[iOp])
      if err != nil { goto ReturnErr }
   }
   if iExpect == nil {
      fmt.Fprintf(os.Stderr, "%s unexpected\n  got    %s %s\n",
                             iPrefix, iOp, aResp.Bytes())
      return
   }
   if iOp == "mo" || iOp == "mn" {
      aResult, err = _parseMessageStream(aResp.Bytes())
   } else {
      aResult = nil
      err = json.Unmarshal(aResp.Bytes(), &aResult)
   }
   if err != nil { goto ReturnErr }

   aName, aMis = _hasExpected(iOp, iExpect, aResult)
   if aName != "" {
      if iSum == nil || iTryN % 2 == 0 {
         aWhat := "mismatch"; if iSum != nil { aWhat = "polling" }
         fmt.Fprintf(os.Stderr, "%s %s\n  expect %v\n  got    %s %v\n",
                                iPrefix, aWhat, iExpect, aName, aMis)
      }
      return
   }
   if iSum != nil {
      atomic.AddInt32(iSum, 1)
   }
   return
   ReturnErr:
      fmt.Fprintf(os.Stderr, "%s %s\n  expect %s %v\n",
                             iPrefix, err, iOp, iExpect)
}

func _parseMessageStream(iBuf []byte) ([]interface{}, error) {
   type tHeader struct {
      From string
      SubHead struct { Attach []struct{ Name string; Size uint64 } }
   }
   aList := make([]interface{}, 0)
   for a, aBufLen := uint64(0), uint64(len(iBuf)); a < aBufLen; a++ {
      var aHead map[string]interface{}
      var aHeadType tHeader

      if a+4 > aBufLen { return nil, tError("head len missing") }
      aHeadLen, err := strconv.ParseUint(string(iBuf[a : a+4]), 16, 0)
      if err != nil { return nil, err }

      if a+4+aHeadLen > aBufLen { return nil, tError("head len invalid") }
      err = json.Unmarshal(iBuf[a+4 : a+4+aHeadLen], &aHead)
      _   = json.Unmarshal(iBuf[a+4 : a+4+aHeadLen], &aHeadType)
      if err != nil { return nil, err }

      aMsgLen, ok := aHead["Len"].(float64)
      if !ok || a+4+aHeadLen+1+uint64(aMsgLen) > aBufLen { return nil, tError("msg len invalid") }
      aHead["msg_data"] = string(iBuf[a+4+aHeadLen+1 : a+4+aHeadLen+1+uint64(aMsgLen)])

      a += 4 + aHeadLen + 1 + uint64(aMsgLen)
      aFfPos := a
      if aHeadType.From == "self" {
         for _, aAtc := range aHeadType.SubHead.Attach {
            if strings.HasPrefix(aAtc.Name, "r:") { a += aAtc.Size }
         }
      }
      if a > aFfPos {
         aHead["form_fill"] = string(iBuf[aFfPos : a])
      }
      aList = append(aList, aHead)
   }
   return aList, nil
}

func _hasExpected(iName string, iExpect, iGot interface{}) (string, interface{}) {
   switch aExpect := iExpect.(type) {
   case string:
      if aExpect == "**" { break }
      aGot, ok := iGot.(string)
      if !ok { return iName, iGot }
      if aExpect == "*d" {
         aT, err := time.Parse(time.RFC3339, aGot)
         if err != nil || aT.Before(sTestNow) { return iName, iGot }
      } else if aExpect == "*dyo" {
         aT, err := time.Parse(time.RFC3339, aGot)
         if err != nil || aT.Before(sTestNow.AddDate(-1,0,0)) { return iName, iGot }
      } else if strings.HasSuffix(aExpect, "#td") {
         if aExpect[:len(aExpect)-3] + sTestDate != aGot { return iName, iGot }
      } else if strings.HasSuffix(aExpect, "#tdg") {
         if !strings.HasPrefix(aGot, aExpect[:len(aExpect)-4] + sTestDate) { return iName, iGot }
      } else if aExpect == "*mid" {
         _, err := strconv.ParseUint(aGot, 16, 64)
         if err != nil || len(aGot) != 16 { return iName, iGot }
      } else if aExpect == "*midt" {
         _, err := strconv.ParseUint(aGot[1:], 16, 64)
         if err != nil || len(aGot) != 13 || aGot[0] != '_' { return iName, iGot }
      } else if aExpect == "*midm" {
         _, err := strconv.ParseUint(aGot[:16], 16, 64)
         if err == nil {
            _, err = strconv.ParseUint(aGot[17:], 16, 64)
         }
         if err != nil || len(aGot) != 29 || aGot[16] != '_' { return iName, iGot }
      } else if aExpect == "*uid" {
         aBuf, err := sTestBase32.DecodeString(aGot)
         if err != nil || len(aBuf) != 20 { return iName, iGot }
      } else if aExpect != "*" {
         if aExpect != aGot { return iName, iGot }
      }
   case nil, bool, float64:
      if iExpect != iGot { return iName, iGot }
   case []interface{}:
      aGot, _ := iGot.([]interface{})
      aDiff := len(aGot) - len(aExpect)
      if aDiff != 0 || aGot == nil { return fmt.Sprintf("%s%+d", iName, aDiff), iGot }
      var aName string
      for a := range aExpect {
         aName, iGot = _hasExpected(fmt.Sprintf("%s.%d", iName, a), aExpect[a], aGot[a])
         if aName != "" { return aName, iGot }
      }
   case map[string]interface{}:
      aGot, ok := iGot.(map[string]interface{})
      if !ok { return iName, iGot }
      for a := range aGot {
         if _, ok = aExpect[a]; !ok { return iName +"+"+ a, aGot[a] }
      }
      var aVal, aName string
      for a := range aExpect {
         if aVal, _ = aExpect[a].(string); aVal != "**" {
            if _, ok = aGot[a]; !ok { return iName +"-"+ a, aExpect[a] }
         }
         aName, iGot = _hasExpected(iName +"."+ a, aExpect[a], aGot[a])
         if aName != "" { return aName, iGot }
      }
   default:
      return iName, iGot
   }
   return "", nil
}

