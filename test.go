// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

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

var sTestHost = "" // used by main.go
var sTestState []tTestStateEl // used by main.go
var sTestNow = time.Now().Truncate(time.Second)
var sTestDate = sTestNow.Format(" 0102150405")
var sTestBase32 = base32.NewEncoding("%+123456789BCDFGHJKLMNPQRSTVWXYZ")
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
   flag.StringVar(&sTestHost, "test", sTestHost, "run test sequence using named service host:port")
}

func test() {
   aDir := "test-run/" + sTestDate[1:]
   fmt.Printf("start test pass in %s\n", aDir)

   var err error
   var aClients []tTestClient

   aFd, err := os.Open("test-in.json")
   if err != nil { quit(err) }
   defer aFd.Close()
   err = json.NewDecoder(aFd).Decode(&aClients)
   if err != nil { quit(err) }

   for a := range aClients {
      sTestState = append(sTestState, tTestStateEl{aClients[a].SvcId, aClients[a].Name, nil})
   }

   err = os.MkdirAll(aDir, 0700)
   if err != nil { quit(err) }
   err = os.Chdir(aDir)
   if err != nil { quit(err) }
   err = os.Symlink("../../web", "web")
   if err != nil { quit(err) }
   err = os.Symlink("../../formspec", "formspec")
   if err != nil { quit(err) }

   aAbout := getAbout()
   pSl.Init(startService)

   var aBuf bytes.Buffer
   aEnc := json.NewEncoder(&aBuf)
   for a := range aClients {
      aTc := &aClients[a]
      if aTc.Version != "" && aTc.Version != aAbout.Version {
         err = tError("version expect " + aTc.Version + ", got " + aAbout.Version)
         goto ReturnErr
      }
      if aTc.Cfg.Name != "" {
         aTc.Cfg.Addr = sTestHost
         aTc.Cfg.Alias += sTestDate
         err = aEnc.Encode(aTc.Cfg)
         if err != nil { quit(err) }
         err = pSl.Service.Add(aTc.SvcId, "", &aBuf)
         if err != nil { goto ReturnErr }
      }
      sTestState[a].state = pSl.OpenState(aTc.Name, aTc.SvcId)
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
      continue
      ReturnErr:
         fmt.Fprintf(os.Stderr, "%s %s %s\nend test pass\n", aTc.Name, aTc.SvcId, err)
         return
   }
   var aWg sync.WaitGroup
   for a := range aClients {
      aWg.Add(1)
      go _runTestClient(&aClients[a], &aWg)
   }
   go func() {
      aWg.Wait()
      fmt.Printf("end test pass\n")
      if sTestExit {
         err = sHttpSrvr.Close()
         if err != nil { quit(err) }
      }
   }()
}

func _runTestClient(iTc *tTestClient, iWg *sync.WaitGroup) {
   defer iWg.Done()
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
   for iTc.SvcId != "local" && pSl.GetConfigService(iTc.SvcId).Uid == "" {
      time.Sleep(100 * time.Millisecond)
   }

   for a := range iTc.Orders {
      aUpdt := &iTc.Orders[a].Updt
      aPrefix := fmt.Sprintf("%s %s %s", iTc.Name, iTc.SvcId, aUpdt.Op)
      if !_prepUpdt(aUpdt, &aCtx, aPrefix) {
         continue
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
      aSum := (*int32)(nil); if aUpdt.Test != nil && aUpdt.Test.Poll > 0 { aSum = new(int32) }
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
         if aSum == nil || *aSum == int32(len(aOps)) {
            break
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

