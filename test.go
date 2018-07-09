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
   "bytes"
   "flag"
   "fmt"
   "encoding/json"
   "os"
   pSl "mnm-hammer/slib"
   "strconv"
   "strings"
   "sync"
   "time"
)

var sTestHost = "" // used by main.go
var sTestState []tTestStateEl // used by main.go
var sTestDate = dateRFC3339()

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
   fmt.Printf("start test pass\n")

   var err error
   var aClients []tTestClient

   aFd, err := os.Open("test-in.json")
   if err != nil { quit(err) }
   defer aFd.Close()
   err = json.NewDecoder(aFd).Decode(&aClients)
   if err != nil { quit(err) }

   aDir := "test-run/" + sTestDate
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
      sTestState = append(sTestState, tTestStateEl{aTc.SvcId, aTc.Name, nil})
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
      for aN := range aTc.Files {
         _, err = aBuf.WriteString(aTc.Files[aN].Data)
         if err != nil { quit(err) }
         err = pSl.Upload.Add(aTc.Files[aN].Name, "", &aBuf)
         if err != nil { goto ReturnErr }
         err = pSl.Upload.Add(aTc.Files[aN].Name, "Dup", nil)
         if err != nil { goto ReturnErr }
         err = pSl.Upload.Drop(aTc.Files[aN].Name+".Dup")
         if err != nil { goto ReturnErr }
      }
      for aN := range aTc.Forms { // expects .spec file first
         err = aEnc.Encode(aTc.Forms[aN])
         if err != nil { quit(err) }
         err = pSl.BlankForm.Add(aTc.Forms[aN].Name, "", &aBuf)
         if err != nil { goto ReturnErr }
         aPair := strings.SplitN(aTc.Forms[aN].Name, ".", 2)
         if len(aPair) == 1 || aPair[1] != "spec" {
            err = pSl.BlankForm.Add(aTc.Forms[aN].Name, "Dup", nil)
            if err != nil { goto ReturnErr }
            err = pSl.BlankForm.Drop(aPair[0]+".Dup")
            if err != nil { goto ReturnErr }
         }
      }
      continue
      ReturnErr:
         fmt.Fprintf(os.Stderr, "%s %s\n  %s\nend test pass\n", aTc.Name, aTc.SvcId, err.Error())
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
   }()
}

func _runTestClient(iTc *tTestClient, iWg *sync.WaitGroup) {
   defer iWg.Done()
   aSvc := getService(iTc.SvcId)
   if aSvc.ccs == nil {
      fmt.Fprintf(os.Stderr, "%s %s  client quit\n  service unknown\n", iTc.Name, iTc.SvcId)
      return
   }
   aCtx := tTestContext{
      svcId: iTc.SvcId,
      lastId: tTestLastId{"tl":{}, "al":{}, "ml":{}, "mn":{}, "ps":{}, "if":{}, "ot":{}},
      state: pSl.OpenState(iTc.Name, iTc.SvcId),
   }
   for a := range sTestState {
      if sTestState[a].name == iTc.Name { sTestState[a].state = aCtx.state }
   }

   for a := range iTc.Orders {
      aUpdt := &iTc.Orders[a].Updt
      aPrefix := fmt.Sprintf("%s %s %s", iTc.Name, iTc.SvcId, aUpdt.Op)
      if !_prepUpdt(aUpdt, aCtx.lastId, aPrefix) {
         continue
      }
      aFn, aSrec := pSl.HandleUpdtService(iTc.SvcId, aCtx.state, aUpdt)
      if aSrec != nil {
         aSvc.queue.postMsg(aSrec)
      }
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
            fmt.Fprintf(os.Stderr, "%s update error\n  %s\n", aPrefix, aMsg.(string))
            continue
         }
      }
      if iTc.Orders[a].Result == nil {
         iTc.Orders[a].Result = make(map[string]interface{})
      }
      for aK, aV := range iTc.Orders[a].Result {
         aN := 0
         for ; aN < len(aOps) && aOps[aN] != aK; aN++ {}
         if aN == len(aOps) {
            fmt.Fprintf(os.Stderr, "%s missing %s\n  expect %v\n", aPrefix, aK, aV)
         }
      }
      aSum := (*int32)(nil); if aUpdt.Op == "test" { aSum = new(int32) }
      for {
         for aN := 0; aN < len(aOps); aN++ {
            aOp, aId := aOps[aN], ""
            if aOp == "_n" {
               _verifyNameList(aOps[1+aN:], iTc.Orders[a].Result[aOp], aPrefix +" "+ aOp)
               break
            }
            if aOp == "_t" { continue }
            if aOp == "mn" || aOp == "an" || aOp == "fn" {
               aN++
               aId = aOps[aN]
            }
            aCtx.wg.Add(1)
            go _runTestService(&aCtx, aOp, aId, iTc.Orders[a].Result[aOp], aPrefix, aSum)
         }
         aCtx.wg.Wait()
         if aSum == nil || *aSum == int32(len(aOps)) { break }
         time.Sleep(aUpdt.Test.Poll * time.Millisecond)
         *aSum = 0
      }
      if len(*aCtx.lastId["mn"]) > 0 {
         *aCtx.lastId["ml"] = *aCtx.lastId["mn"]
         *aCtx.lastId["mn"] = tTestAnyId{}
      }
   }
}

func _prepUpdt(iUpdt *pSl.Update, iLastId tTestLastId, iPrefix string) bool {
   var aApply string
   switch iUpdt.Op {
   case "thread_save":
      if iUpdt.Thread.Alias != "" {
         iUpdt.Thread.Alias += sTestDate
      }
      for aN := range iUpdt.Thread.Cc {
         iUpdt.Thread.Cc[aN] += sTestDate
      }
      if iUpdt.Thread.FormFill != nil {
         aFf := iUpdt.Thread.FormFill["lastfile"]
         if aFf != "" {
            aFileId := (*iLastId["al"])[0].File
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
      _applyLastId(&iUpdt.Thread.Id,         &aApply, iLastId, "ml")
      _applyLastId(&iUpdt.Thread.ThreadId,   &aApply, iLastId, "ml")
   case "ping_save":
      iUpdt.Ping.Alias += sTestDate
      iUpdt.Ping.To    += sTestDate
      if iUpdt.Ping.Gid != "" {
         iUpdt.Ping.Gid += sTestDate
      }
   case "ping_send":
      _applyLastId(&iUpdt.Ping.Qid,          &aApply, iLastId, "ps")
   case "ping_discard":
      iUpdt.Ping.To += sTestDate
   case "accept_send":
      _applyLastId(&iUpdt.Accept.Qid,        &aApply, iLastId, "if")
   case "ohi_add":
      iUpdt.Ohi.Alias += sTestDate
   case "ohi_drop":
      _applyLastId(&iUpdt.Ohi.Uid,           &aApply, iLastId, "ot")
   case "navigate_thread":
      _applyLastId(&iUpdt.Navigate.ThreadId, &aApply, iLastId, "tl")
   case "navigate_link":
      _applyLastId(&iUpdt.Navigate.ThreadId, &aApply, iLastId, "ml")
      _applyLastId(&iUpdt.Navigate.MsgId,    &aApply, iLastId, "ml")
   case "navigate_history",
        "tab_add", "tab_pin", "tab_drop", "tab_select",
        "adrsbk_search",
        "open",
        "test":
      // nothing to do
   default:
      fmt.Fprintf(os.Stderr, "%s unknown op\n  update %s\n", iPrefix, iUpdt.Op)
      return false
   }
   if aApply != "" {
      fmt.Printf("%s applied %s\n", iPrefix, aApply)
   }
   return true
}

func _applyLastId(iField, iMsg *string, iLastId tTestLastId, iType string) {
   a := -1
   switch *iField {
   case "last", "lastfile": a = 0
   case "2ndlast":          a = 1
   default:                 return
   }
   aSet := *iLastId[iType]
   switch iType {
   case "tl", "ml": *iField = aSet[a].Id
   case "if", "ps": *iField = aSet[a].Qid
   case "ot":       *iField = aSet[a].Uid
   case "al":       *iField = aSet[a].File
   default:         return
   }
   aAmp := ""; if *iMsg != "" { aAmp = " & " }
   *iMsg += aAmp + *iField
}

func _verifyNameList(iList []string, iExpect interface{}, iPrefix string) {
   aGot := make([]interface{}, len(iList))
   for aI := range iList { aGot[aI] = iList[aI] }
   if iExpect == nil {
      fmt.Fprintf(os.Stderr, "%s unexpected\n  got    %v\n",
                             iPrefix, aGot)
   } else if !_hasExpected(iExpect, aGot) {
      fmt.Fprintf(os.Stderr, "%s mismatch\n  expect %v\n  got    %v\n",
                             iPrefix, iExpect, aGot)
   }
}

func _runTestService(iCtx *tTestContext, iOp, iId string, iExpect interface{},
                    iPrefix string, iSum *int32) {
   defer iCtx.wg.Done()
   var err error
   var aResult interface{}
   var aResp bytes.Buffer

   switch(iOp) {
   case "/t": aResult = pSl.Upload.GetIdx()
   case "/f": aResult = pSl.BlankForm.GetIdx()
   case "/v": aResult = pSl.Service.GetIdx()
   case "cs": aResult = iCtx.state.GetSummary()
   case "cf": aResult = pSl.GetDataService(iCtx.svcId)
   case "ps": aResult = pSl.GetDraftAdrsbk(iCtx.svcId)
   case "pt": aResult = pSl.GetSentAdrsbk(iCtx.svcId)
   case "pf": aResult = pSl.GetReceivedAdrsbk(iCtx.svcId)
   case "it": aResult = pSl.GetInviteToAdrsbk(iCtx.svcId)
   case "if": aResult = pSl.GetInviteFromAdrsbk(iCtx.svcId)
   case "gl": aResult = pSl.GetGroupAdrsbk(iCtx.svcId)
   case "of": aResult = pSl.GetFromOhi(iCtx.svcId)
   case "ot": aResult = pSl.GetIdxOhi(iCtx.svcId)
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
   } else if iCtx.lastId[iOp] != nil {
      err = json.Unmarshal(aResp.Bytes(), iCtx.lastId[iOp])
      if err != nil { goto ReturnErr }
   }
   if iExpect == nil {
      fmt.Fprintf(os.Stderr, "%s %s %s unexpected\n  got    %s\n",
                             iPrefix, iOp, iId, aResp.Bytes())
      return
   }
   if iOp == "mo" || iOp == "mn" {
      aResult, err = _parseMessageStream(aResp.Bytes())
   } else {
      aResult = nil
      err = json.Unmarshal(aResp.Bytes(), &aResult)
   }
   if err != nil { goto ReturnErr }

   if !_hasExpected(iExpect, aResult) {
      aWhat := "mismatch"; if iSum != nil { aWhat = "polling" }
      fmt.Fprintf(os.Stderr, "%s %s %s %s\n  expect %v\n  got    %v\n",
                             iPrefix, iOp, iId, aWhat, iExpect, aResult)
      return
   }
   if iSum != nil {
      atomic.AddInt32(iSum, 1)
   }
   return
   ReturnErr:
      fmt.Fprintf(os.Stderr, "%s %s %s %s\n  expect %v\n",
                             iPrefix, iOp, iId, err.Error(), iExpect)
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

func _hasExpected(iExpect, iGot interface{}) bool {
   switch aExpect := iExpect.(type) {
   case string:
      if aExpect == "**" { break }
      aGot, ok := iGot.(string)
      if !ok { return false }
      if aExpect == "*d" {
         _, err := time.Parse(time.RFC3339, aGot)
         if err != nil || aGot[:19]+"Z" < sTestDate { return false }
      } else if strings.HasSuffix(aExpect, "#td") {
         if aExpect[:len(aExpect)-3] + sTestDate != aGot { return false }
      } else if aExpect != "*" {
         if aExpect != aGot { return false }
      }
   case nil, bool, float64:
      if iExpect != iGot { return false }
   case []interface{}:
      aGot, _ := iGot.([]interface{})
      if aGot == nil || len(aExpect) != len(aGot) { return false }
      for a := range aExpect {
         if !_hasExpected(aExpect[a], aGot[a]) { return false }
      }
   case map[string]interface{}:
      aGot, _ := iGot.(map[string]interface{})
      if aGot == nil || len(aExpect) != len(aGot) { return false }
      for a := range aExpect {
         if !_hasExpected(aExpect[a], aGot[a]) { return false }
      }
   default:
      return false
   }
   return true
}

