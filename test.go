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
   "net/http"
   "io"
   "io/ioutil"
   "encoding/json"
   "net"
   "os"
   "sort"
   "strconv"
   "strings"
   "sync"
   "time"
   "net/url"
   "unicode/utf16"
   pWs "github.com/gorilla/websocket"
   pSl "github.com/networkimprov/mnm-hammer/slib"
)

const kTestDateF = "0102150405"
var kTestBase32 = base32.NewEncoding("%+123456789BCDFGHJKLMNPQRSTVWXYZ")

// inputs, set via command line flags
var sTestHost string // used by main.go
var sTestCrash, sTestVerify string
var sTestCrashOp string
var sTestCrashSrc, sTestCrashDst string
var sTestOrderSrc, sTestOrderDst uint64

var sTestWebAddr string
var sTestNodePin string
var sTestOrderN = map[string]*uint64{} // key SvcId, value atomic
var sTestNow = time.Now().Truncate(time.Second)
var sTestDate = sTestNow.Format(" "+kTestDateF)
var sTestDateGid = time.Now().Format(":"+kTestDateF+".000")
var sTestExit = false

type tTestClient struct {
   CountUtf8 []tTestSteppedRead
   Formspec map[string]interface{} // one for all clients
   Version string

   Name string
   SvcId string
   Cfg struct {
      Name, Alias string
      Addr string // for internal use; json value ignored
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
      Poll int
      Result map[string]interface{}
      Name string
      Client *struct { Name, SvcId string }
   }
}

type tTestSteppedRead struct {
   In []tTestQuoted
   Out []string
   pos int
}

type tTestQuoted string

func (o *tTestQuoted) UnmarshalJSON(iBuf []byte) error {
   iBuf = bytes.ReplaceAll(iBuf, []byte{'\\','\\'}, []byte{'\\'})
   aS, err := strconv.Unquote(string(iBuf))
   if err != nil { return err }
   *o = tTestQuoted(aS)
   return nil
}

func (o *tTestSteppedRead) inSum() int64 {
   var aLen int
   for a := range o.In { aLen += len(o.In[a]) }
   return int64(aLen)
}

func (o *tTestSteppedRead) Read(iBuf []byte) (int, error) {
   if o.pos >= len(o.In) {
      return 0, io.EOF
   }
   aStep := o.In[o.pos]
   o.pos++
   if len(iBuf) < len(aStep) {
      quit(tError("buffer too small"))
   }
   return copy(iBuf, aStep), nil
}

type tTestContext struct {
   svcId string
   lastId tTestLastId
   web *http.Client
   path, pathSoc string
   wg sync.WaitGroup
}

type tTestLastId map[string]*tTestAnyId // key is service op

type tTestAnyId []struct {
   Id, Qid, Uid string
}

func init() {
   flag.StringVar(&sTestHost, "test", sTestHost,
                  "run test sequence using named service host:port")
   flag.StringVar(&sTestCrash, "crash", sTestCrash,
                  "exit transaction at dir:service:order:op[:sender:order], or setup dir with 'init'")
   flag.StringVar(&sTestVerify, "verify", sTestVerify,
                  "resume after crash and check result for dir:service:order:count")
}

func crashTest(iSvc string, iOp string) {
   if iSvc != sTestCrashDst || iOp != sTestCrashOp ||
      atomic.LoadUint64(sTestOrderN[sTestCrashSrc]) < sTestOrderSrc ||
      atomic.LoadUint64(sTestOrderN[sTestCrashDst]) > sTestOrderDst {
      //if sTestCrash != "" { fmt.Printf("crash  --  %s %s\n", iSvc, iOp) }
      return
   }
   fmt.Printf("crash test %s %s\n", iSvc, iOp)
   err := sHttpSrvr.Close()
   if err != nil { quit(err) }
   time.Sleep(15 * time.Second)
   quit(tError("failed to exit"))
}

func test() int {
   pSl.SetSyncPeriodNode(1 * time.Second)
   sTestWebAddr = sHttpSrvr.Addr; if sTestWebAddr[0] == ':' { sTestWebAddr = "localhost"+ sTestWebAddr }
   aDir := "test-run/" + sTestDate[1:]
   var err error
   var aClients []tTestClient

   aFd, err := os.Open("test-in.json")
   if err == nil {
      defer aFd.Close()
      err = json.NewDecoder(aFd).Decode(&aClients)
   }
   if err != nil {
      fmt.Fprintf(os.Stderr, "%v\n", err)
      return 33
   }

   aAbout := getAbout()
   for a := range aClients {
      if aClients[a].Version != "" && aClients[a].Version != aAbout.Version {
         fmt.Fprintf(os.Stderr, "test-in expects v%s, app is v%s\n", aClients[a].Version, aAbout.Version)
         return 33
      }
      sTestOrderN[aClients[a].SvcId] = new(uint64)
      aResultLib := make(map[string]map[string]interface{})
      for a1 := range aClients[a].Orders {
         aOrder := &aClients[a].Orders[a1]
         for aK, aV := range aOrder.Result {
            if aPrior, _ := aV.(string); aPrior != "" {
               aOrder.Result[aK] = aResultLib[aPrior][aK]
            }
         }
         if aOrder.Name != "" {
            if aResultLib[aOrder.Name] != nil {
               fmt.Fprintf(os.Stderr, "order name %s appears more than once\n", aOrder.Name)
               return 33
            }
            aResultLib[aOrder.Name] = aOrder.Result
         }
      }
      aResultLib = nil
   }

   if !_algorithms(&aClients[0]) {
      return 33
   }

   if sTestVerify != "" {
      aDir, err = _setupTestVerify(aClients) // triggers receipt of msgs pending from -crash run
      if err != nil {
         fmt.Fprintf(os.Stderr, "invalid -verify parameter '%s': %v\n", sTestVerify, err)
         return 33
      }
      time.Sleep(600 * time.Millisecond) // handle msgs pending from -crash run
      for a := range aClients {
         if aClients[a].SvcId != sTestCrashDst { continue }
         go func(c int) {
            _runTestClient(&aClients[c], nil)
            err = sHttpSrvr.Close()
            if err != nil { quit(err) }
         }(a)
         return -1
      }
      quit(tError("SvcId not found: "+ sTestCrashDst))
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
         sTestHost = "" // reenable logging
      }
      if sTestExit {
         err = sHttpSrvr.Close()
         if err != nil { quit(err) }
      }
   }()
   return -1
}

func _algorithms(iClient *tTestClient) bool {
   aSet := iClient.CountUtf8
   aBuf := bytes.Buffer{}
   for a := range aSet {
      aUtf8 := pSl.NewCountUtf8(&aSet[a], aSet[a].inSum())
      _, err := io.Copy(&aBuf, aUtf8)
      if err != nil { quit(err) }
      aOut := strings.Join(aSet[a].Out, "")
      aOutLen := int64(len(utf16.Encode([]rune(aOut))))
      if string([]rune(aBuf.String())) != aOut || aUtf8.Utf16Len() != aOutLen {
         fmt.Fprintf(os.Stderr, "CountUtf8 mismatch\n  expect %d %s\n  got    %d %s\n",
                                aOutLen, aOut, aUtf8.Utf16Len(), aBuf.String())
         return false
      }
      aBuf.Reset()
   }
   return true
}

func _setupTestDir(iDir string, iClients []tTestClient) bool {
   var err error

   err = os.MkdirAll(iDir, 0700)
   if err != nil { quit(err) }
   err = os.Chdir(iDir)
   if err != nil { quit(err) }
   err = os.Symlink("../../web", "web")
   if err != nil { quit(err) }

   pSl.Init(StartService, MsgToSelf, crashTest)
   pSl.ListenNode()
   aPin := pSl.GetPinNode(sNetAddr)
   sTestNodePin = aPin.Pin

   var aTc *tTestClient
   var aBuf bytes.Buffer
   aEnc := json.NewEncoder(&aBuf)
   for a := range iClients {
      aTc = &iClients[a]
      if aTc.Formspec != nil { //todo download spec
         var aFd *os.File
         aFd, err = os.Create("formspec")
         if err != nil { quit(err) }
         err = json.NewEncoder(aFd).Encode(aTc.Formspec)
         if err != nil { quit(err) }
         err = aFd.Close()
         if err != nil { quit(err) }
      }
      if aTc.Cfg.Name != "" {
         aTc.Cfg.Addr = "=" + sTestHost
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
   const ( eDir = iota; eDst; eDstOrder; eOp; eSrc; eSrcOrder; eArgLen )
   aArg := make([]string, eArgLen)
   copy(aArg, strings.Split(sTestCrash, ":"))
   if !strings.HasPrefix(aArg[eDir], "test-run/") {
      return "", tError("invalid dir")
   }
   if aArg[eOp] == "" {
      return "", tError("missing op")
   }
   sTestCrashDst = aArg[eDst]
   _, sTestOrderDst, err = _findOrder(iClients, aArg[eDst], aArg[eDstOrder])
   if err != nil { return }
   sTestCrashSrc, sTestOrderSrc = sTestCrashDst, sTestOrderDst
   // may need aArg[eCount], sTestOrderDst += aCount - 1
   if aArg[eSrc] != "" {
      sTestCrashSrc = aArg[eSrc]
      _, sTestOrderSrc, err = _findOrder(iClients, aArg[eSrc], aArg[eSrcOrder])
      if err != nil { return }
   }
   sTestNow, err = time.Parse(kTestDateF, aArg[eDir][9:]) // omit "test-run/"
   if err != nil { return }
   sTestNow = sTestNow.AddDate(time.Now().Year(), 0, 0)
   sTestDate = sTestNow.Format(" "+kTestDateF)
   sTestCrashOp = aArg[eOp]

   err = os.Chdir(aArg[eDir])
   if err != nil { return }
   err = os.RemoveAll("store/state")
   if err != nil { return }
   for a := range iClients {
      if iClients[a].Cfg.Name == "" { continue }
      err = pSl.WipeDataService(iClients[a].SvcId)
      if err != nil { return }
   }
   pSl.Init(StartService, MsgToSelf, crashTest)
   if sServices[sTestCrashDst].queue == nil || sServices[sTestCrashSrc].queue == nil {
      return "", tError("invalid service")
   }
   return aArg[eDir], nil
}

func _setupTestVerify(iClients []tTestClient) (_ string, err error) {
   const ( eDir = iota; eSvc; eOrder; eCount; eArgLen )
   aArg := make([]string, eArgLen)
   copy(aArg, strings.Split(sTestVerify, ":"))
   if !strings.HasPrefix(aArg[eDir], "test-run/") {
      return "", tError("invalid dir")
   }
   aTc, aOrderN, err := _findOrder(iClients, aArg[eSvc], aArg[eOrder])
   if err != nil { return }
   aOrderLen, err := strconv.ParseUint(aArg[eCount], 10, 64)
   if err != nil { return }
   sTestNow, err = time.Parse(kTestDateF, aArg[eDir][9:]) // omit "test-run/"
   if err != nil { return }
   sTestNow = sTestNow.AddDate(time.Now().Year(), 0, 0)
   sTestDate = sTestNow.Format(" "+kTestDateF)
   sTestCrashDst = aArg[eSvc]

   err = os.Chdir(aArg[eDir])
   if err != nil { return }
   pSl.Init(StartService, MsgToSelf, crashTest)
   if sServices[aArg[eSvc]].queue == nil {
      return "", tError("invalid service")
   }
   if aOrderLen == 0 || aOrderN+aOrderLen > uint64(len(aTc.Orders)) {
      return "", tError("invalid order range")
   }
   aTc.Orders = aTc.Orders[aOrderN : aOrderN+aOrderLen]
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
      aTc.Orders[a].Poll = 0
   }
   return aArg[eDir], nil
}

func _findOrder(iClients []tTestClient, iSvc string, iOrder string) (*tTestClient, uint64, error) {
   for a := range iClients {
      if iClients[a].SvcId != iSvc { continue }
      for a1 := range iClients[a].Orders {
         if iClients[a].Orders[a1].Name != iOrder { continue }
         return &iClients[a], uint64(a1), nil
      }
   }
   return nil, 1e6, tError("order not found")
}

type tJar []*http.Cookie
func (o tJar) SetCookies(_ *url.URL, i []*http.Cookie) { copy(o, i) }
func (o tJar) Cookies(*url.URL) []*http.Cookie { return o }

func _runTestClient(iTc *tTestClient, iWg *sync.WaitGroup) {
   if iWg != nil {
      defer iWg.Done()
   }
   aCtx := tTestContext{
      svcId: iTc.SvcId,
      lastId: tTestLastId{"tl":{}, "al":{}, "ml":{}, "cl":{}, "mn":{}, "ps":{}, "pf":{}},
      web: &http.Client{Jar: tJar([]*http.Cookie{{Name: "clientid", Value: iTc.Name}})},
      path: "http://"+ sTestWebAddr +"/"+ url.PathEscape(iTc.SvcId),
      pathSoc: "ws://"+ sTestWebAddr +"/5/"+ url.PathEscape(iTc.SvcId),
   }
   aRsp, err := aCtx.web.Get(aCtx.path)
   if err != nil { quit(err) }
   _, err = io.Copy(ioutil.Discard, aRsp.Body)
   if err != nil { quit(err) }
   aRsp.Body.Close()
   if aRsp.StatusCode != http.StatusOK {
      quit(tError("status "+ aRsp.Status +" for "+ aCtx.path))
   }
   aReq := http.Request{Header:http.Header{}}
   aReq.AddCookie(aCtx.web.Jar.Cookies(nil)[0])
   aSoc, _, err := pWs.DefaultDialer.Dial(aCtx.pathSoc, aReq.Header)
   if err != nil { quit(err) }
   defer aSoc.Close()
   var aBuf []byte

   for a := range iTc.Orders {
      if sTestOrderN[iTc.SvcId] != nil {
         atomic.StoreUint64(sTestOrderN[iTc.SvcId], uint64(a))
      }
      if iTc.Orders[a].Client != nil {
         aCl := iTc.Orders[a].Client
         iTc.Orders[a].Client = nil
         aTc := tTestClient{Name:aCl.Name, SvcId:aCl.SvcId, Orders:iTc.Orders[a:a+1]}
         iWg.Add(1)
         go _runTestClient(&aTc, iWg)
         continue
      }
      aUpdt := &iTc.Orders[a].Updt
      aPrefix := fmt.Sprintf("%s %s %s", iTc.Name, iTc.SvcId, aUpdt.Op)
      if !_prepUpdt(aUpdt, &aCtx, aPrefix) {
         continue
      }
      aBuf, err = json.Marshal(aUpdt)
      if err != nil { quit(err) }
      err = aSoc.WriteMessage(pWs.TextMessage, aBuf)
      if err != nil { quit(err) }
      if len(iTc.Orders[a].Result) == 0 {
         continue
      }
      _, aBuf, err = aSoc.ReadMessage()
      if err != nil { quit(err) }
      var aOps []string
      err = json.Unmarshal(aBuf, &aOps)
      if err != nil { quit(err) }
      if aOps[0] == "_e" {
         fmt.Fprintf(os.Stderr, "%s update error %s\n", aPrefix, aOps[1])
         continue
      }
      for aK, aV := range iTc.Orders[a].Result {
         a1 := -1
         for a1 = 0; a1 < len(aOps) && !(aOps[a1] == aK || aK == "mn" && aOps[a1] == "_m"); a1++ {}
         if a1 == len(aOps) {
            fmt.Fprintf(os.Stderr, "%s missing result\n  expect %s %v\n", aPrefix, aK, aV)
         }
      }
      var aSum *int32 = nil; if iTc.Orders[a].Poll > 0 { aSum = new(int32) }
      for aTryN := 2 * iTc.Orders[a].Poll; true; aTryN-- {
         for a1 := 0; a1 < len(aOps); a1++ {
            aOp, aId := aOps[a1], ""
            if aOp == "_n" {
               _verifyNameList(aOps[1+a1:], iTc.Orders[a].Result[aOp], aPrefix +" "+ aOp)
               break
            }
            if aOp == "_t" || aOp == "_T" { continue }
            if aOp == "mn" || aOp == "an" {
               a1++
               aId = aOps[a1]
               if aSum != nil { atomic.AddInt32(aSum, 1) }
            } else if aOp == "_m" {
               a1 += 2
               aOp, aId = "mn", aOps[a1]
               if aSum != nil { atomic.AddInt32(aSum, 2) }
            }
            aCtx.wg.Add(1)
            go _runTestService(&aCtx, aOp, aId, iTc.Orders[a].Result[aOp], aPrefix, aSum, aTryN)
         }
         aCtx.wg.Wait()
         if aSum == nil || *aSum == int32(len(aOps)) || aTryN == 0 {
            break
         }
         time.Sleep(500 * time.Millisecond)
         *aSum = 0
      }
      if len(*aCtx.lastId["mn"]) > 0 {
         *aCtx.lastId["ml"] = *aCtx.lastId["mn"]
         *aCtx.lastId["mn"] = tTestAnyId{}
      }
   }
   err = aSoc.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
   if err != nil { quit(err) }
   for {
      _, aBuf, err = aSoc.ReadMessage()
      if err != nil {
         if !err.(net.Error).Timeout() { quit(err) }
         break
      }
      fmt.Printf("%s %s unprompted\n  got    %s\n", iTc.Name, iTc.SvcId, string(aBuf))
   }
   err = aSoc.WriteMessage(pWs.CloseMessage, pWs.FormatCloseMessage(pWs.CloseGoingAway, ""))
   if err != nil { quit(err) }
}

func _prepUpdt(iUpdt *pSl.Update, iCtx *tTestContext, iPrefix string) bool {
   var aApply string
   switch iUpdt.Op {
   case "config_update":
      if iUpdt.Config.Addr == "orig" {
         iUpdt.Config.Addr = "=" + sTestHost
      }
   case "node_add":
      if iUpdt.Node.Addr == "localhost" {
         if sHttpSrvr.Addr[0] == ':' {
            if sHttpSrvr.Addr != ":http" {
               iUpdt.Node.Addr += sHttpSrvr.Addr
            }
         } else {
            iUpdt.Node.Addr = sHttpSrvr.Addr
         }
      }
      if iUpdt.Node.Pin == "localpin" {
         iUpdt.Node.Pin = sTestNodePin
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
      for a := range iUpdt.Thread.Attach {
         if iUpdt.Thread.Attach[a].FfKey == "lastfile" {
            aFileId := (*iCtx.lastId["al"])[0].Id
            iUpdt.Thread.Attach[a].FfKey = aFileId
            iUpdt.Thread.FormFill[aFileId] = iUpdt.Thread.FormFill["lastfile"]
            delete(iUpdt.Thread.FormFill, "lastfile")
         }
      }
      fallthrough
   case "thread_send", "thread_discard":
      _applyLastId(&iUpdt.Thread.Id,         &aApply, iCtx.lastId, "ml")
   case "thread_open", "thread_close", "thread_tag":
      _applyLastId(&iUpdt.Touch.MsgId,       &aApply, iCtx.lastId, "ml")
      _applyLastId(&iUpdt.Touch.ThreadId,    &aApply, iCtx.lastId, "tl")
      if iUpdt.Touch.TagId != "" {
         iUpdt.Touch.TagId = pSl.GetIdTag(iUpdt.Touch.TagId)
      }
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
        "tag_add",
        "tab_add", "tab_pin", "tab_drop", "tab_select",
        "sort_select",
        "open":
      // nothing to do
   case "test":
      _applyLastId(&iUpdt.Test.ThreadId,     &aApply, iCtx.lastId, "tl")
      if len(iUpdt.Test.Request) >= 2 {
         if iUpdt.Test.Request[0] == "mn" { // assume Request[1] is valid
            _applyLastId(&iUpdt.Test.Request[1], &aApply, iCtx.lastId, "ml")
         }
      } else if iUpdt.Test.Notice != nil {
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
   case "tl", "ml", "al":
      *iField = aSet[a].Id
   case "cl", "ps":
      *iField = aSet[a].Qid
   case "pf":
      if *iField == "lastqid" {
         *iField = aSet[a].Qid
      } else {
         *iField = aSet[a].Uid
      }
   default:
      return
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
   defer func() { if err != nil { fmt.Fprintf(os.Stderr, "%s %s\n  expect %s %v\n",
                                                         iPrefix, err, iOp, iExpect) } }()

   aPath := iCtx.path
   if iOp[0] == '/' {
      aPath = aPath[:strings.LastIndexByte(aPath, '/')] + iOp +"/"
   } else {
      aPath += "?"+ iOp
      if iId != "" {
         aPath += "="+ url.QueryEscape(iId)
      }
   }
   aRsp, err := iCtx.web.Get(aPath)
   if err != nil { quit(err) }
   defer aRsp.Body.Close()
   var aResult bytes.Buffer
   _, err = io.Copy(&aResult, aRsp.Body)
   if err != nil { quit(err) }
   if aRsp.StatusCode != http.StatusOK {
      err = tError("status "+ aRsp.Status +" for "+ aPath)
      return
   }
   if iOp == "mn" {
      *iCtx.lastId[iOp] = tTestAnyId{{Id:iId}}
   } else if iOp == "cl" {
      aClPair := [2]tTestAnyId{}
      err = json.Unmarshal(aResult.Bytes(), &aClPair)
      if err != nil { return }
      *iCtx.lastId[iOp] = aClPair[0]
   } else if iOp == "tl" && aResult.Bytes()[0] == '{' {
      // nothing to do
   } else if iCtx.lastId[iOp] != nil {
      err = json.Unmarshal(aResult.Bytes(), iCtx.lastId[iOp])
      if err != nil { return }
   }
   if iExpect == nil {
      fmt.Fprintf(os.Stderr, "%s unexpected\n  got    %s %s\n",
                             iPrefix, iOp, aResult.Bytes())
      return
   }
   var aGot interface{}
   if iOp == "mo" || iOp == "mn" {
      aGot, err = _parseMessageStream(aResult.Bytes())
   } else {
      err = json.Unmarshal(aResult.Bytes(), &aGot)
      if iOp == "al" {
         aLid := *iCtx.lastId[iOp]
         sort.Slice(aLid, func(cA, cB int)bool { return aLid[cA].Id < aLid[cB].Id })
         aS := aGot.([]interface{})
         sort.Slice(aS, func(cA, cB int)bool { return aS[cA].(map[string]interface{})["Id"].(string) <
                                                      aS[cB].(map[string]interface{})["Id"].(string) })
      }
   }
   if err != nil { return }

   aName, aMis := _hasExpected(iOp, iExpect, aGot)
   if aName != "" {
      if iSum == nil || iTryN == 0 {
         aWhat := "mismatch"; if iSum != nil { aWhat = "polling" }
         fmt.Fprintf(os.Stderr, "%s %s\n  expect %v\n  got    %s %v\n",
                                iPrefix, aWhat, iExpect, aName, aMis)
      }
      return
   }
   if iSum != nil {
      atomic.AddInt32(iSum, 1)
   }
}

func _parseMessageStream(iBuf []byte) ([]interface{}, error) {
   type tHeader struct {
      From string
      SubHead struct { Attach []struct{ Name string; Size uint64 } }
   }
   aList := []interface{}{}
   aRune := []rune(string(iBuf)) // assume no utf16 surrogate pairs
   for a, aBufLen := uint64(0), uint64(len(aRune)); a < aBufLen; a++ {
      var aHead map[string]interface{}
      var aHeadType tHeader

      if a+4 > aBufLen { return nil, tError("head len missing") }
      aHeadLen, err := strconv.ParseUint(string(aRune[a : a+4]), 16, 0)
      if err != nil { return nil, err }

      if a+4+aHeadLen > aBufLen { return nil, tError("head len invalid") }
      aBuf := []byte(string(aRune[a+4 : a+4+aHeadLen]))
      err = json.Unmarshal(aBuf, &aHead)
      _   = json.Unmarshal(aBuf, &aHeadType)
      if err != nil { return nil, err }

      aMsgLen, ok := aHead["Len"].(float64)
      if !ok || a+4+aHeadLen+1+uint64(aMsgLen) > aBufLen { return nil, tError("msg len invalid") }
      aHead["msg_data"] = string(aRune[a+4+aHeadLen+1 : a+4+aHeadLen+1+uint64(aMsgLen)])

      a += 4 + aHeadLen + 1 + uint64(aMsgLen)
      aFfPos := a
      if aHeadType.From == "self" {
         for _, aAtc := range aHeadType.SubHead.Attach {
            if strings.HasPrefix(aAtc.Name, "r:") { a += aAtc.Size }
         }
      }
      if a > aFfPos {
         aHead["form_fill"] = string(aRune[aFfPos : a])
      }
      aList = append(aList, aHead)
   }
   return aList, nil
}

func _hasExpected(iName string, iExpect, iGot interface{}) (string, interface{}) {
   switch aExpect := iExpect.(type) {
   case string:
      if aExpect == "**" { break }
      if len(aExpect) >= 2 && aExpect[0] == '>' {
         aGot, ok := iGot.(float64)
         if ok {
            aF, err := strconv.ParseFloat(aExpect[1:], 64)
            if err != nil || aGot <= aF { return iName, iGot }
            break
         }
      }
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
         aBuf, err := kTestBase32.DecodeString(aGot)
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

