// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package main

import (
   "runtime/debug"
   "flag"
   "fmt"
   "net/http"
   "io"
   "encoding/json"
   "mime/multipart"
   "net"
   "os"
   "path"
   pSl "github.com/networkimprov/mnm-hammer/slib"
   pWs "github.com/gorilla/websocket"
   "math/rand"
   "strconv"
   "strings"
   "sync"
   "text/template"
   "time"
   "crypto/tls"
   "net/url"
)

// production releases: 1+ . 0  . 0+
// preview releases:    pp . 1+ . pp (kVersionA & C from prior production release)
// example sequence:    1.0.0, 1.0.1, 1.0.2, 1.1.2, 1.2.2, 2.0.0, 2.1.0, 2.0.1, 2.2.1
const kVersionA, kVersionB, kVersionC = 0, 10, 0
const kVersionDate = "(unreleased)" // yyyy.mm.dd

const kDialRetryDelayMax = 6 * 60
const kIdleTimeFraction = 10
const kPulsePeriod time.Duration = 115 * time.Second
const kMsgHeaderMinLen = int64(len(`{"op":1}`))
const kMsgHeaderMaxLen = int64(1 << 16)
const kFirstOhiId = "first_ohi"

const (
   eOpTmtpRev = iota
   eOpRegister; eOpLogin
   eOpUserEdit; eOpOhiEdit;
   eOpGroupInvite; eOpGroupEdit
   eOpPost; eOpPostNotify; eOpPing
   eOpAck
   eOpPulse; eOpQuit
   eOpEnd
)

var sHttpSrvr = http.Server{Addr: ":http"}
var sServicesDoor sync.RWMutex
var sServices = make(map[string]tService)
var sServiceTmpl *template.Template
var sNetAddr string

func init() {
   flag.StringVar(&sHttpSrvr.Addr, "http", sHttpSrvr.Addr, "[host]:port of http server")
}

func main() {
   aVersionQuit := flag.Bool("version", false, "print version and quit")
   flag.Parse() // may os.Exit(2)
   if sTestCrash == "" && sTestVerify == "" {
      fmt.Printf("mnm-hammer tmtp client v%d.%d.%d %s\n", kVersionA, kVersionB, kVersionC, kVersionDate)
   }
   if *aVersionQuit {
      os.Exit(0)
   }
   os.Exit(mainResult())
}

func mainResult() int {
   // return 2 reserved for use by Go internals
   var err error
   defer func() { if err != nil { fmt.Fprintf(os.Stderr, "mainResult: %v\n", err) } }()

   sServices["local"] = tService{ccs: newClientConns()}

   if sTestHost != "" && sHttpSrvr.Addr == ":http" {
      sHttpSrvr.Addr = ":8123"
   }
   if sHttpSrvr.Addr[0] == ':' {
      sNetAddr = _getNetAddress()
      if sHttpSrvr.Addr != ":http" {
         sNetAddr += sHttpSrvr.Addr
      }
   } else {
      sNetAddr = sHttpSrvr.Addr
   }
   aLsn, err := net.Listen("tcp", sHttpSrvr.Addr)
   if err != nil { return 1 }

   if sTestHost != "" {
      if aRes := test(); aRes >= 0 {
         return aRes
      }
   } else {
      _, err = os.Stat("web/service.html")
      if err != nil {
         err = os.Chdir(path.Dir(os.Args[0]))
         if err != nil { return 1 }
      }
      pSl.Init(StartService, MsgToSelf, crashTest)
   }

   sServiceTmpl, err = template.New("service.html").Delims(`<%`,`%>`).ParseFiles("web/service.html")
   if err != nil { return 1 }

   http.HandleFunc("/"  , runService)
   http.HandleFunc("/a/", runAbout)
   http.HandleFunc("/l/", runNodeListen)
   http.HandleFunc("/n/", runNodeRecv)
   http.HandleFunc("/t/", runGlobal)
   http.HandleFunc("/f/", runGlobal)
   http.HandleFunc("/v/", runGlobal)
   http.HandleFunc("/g/", runTag)
   http.HandleFunc("/s/", runWebsocket)
   http.HandleFunc("/5/", runWebsocket) // test clients
   http.HandleFunc("/w/", runFile)
   http.HandleFunc("/favicon.ico", runFavicon)

   err = sHttpSrvr.Serve(aLsn)
   if err != http.ErrServerClosed { return 1 }
   err = nil
   return 0
}

func _getNetAddress() string {
   aLink, err := net.Dial("udp", "1.1.1.1:11") // doesn't cause network activity
   if err != nil {
      fmt.Fprintf(os.Stderr, "_getNetAddress: %v\n", err)
      return "Unavailable"
   }
   defer aLink.Close()
   return aLink.LocalAddr().(*net.UDPAddr).IP.String()
}

type tService struct {
   queue *tQueue
   ccs *tClientConns
   toSelf chan *pSl.Header
}

func StartService(iSvcId string) {
   sServicesDoor.Lock(); defer sServicesDoor.Unlock()
   if sServices[iSvcId].ccs != nil {
      panic(fmt.Sprintf("startService %s: already started", iSvcId))
   }
   sServices[iSvcId] = tService{queue: newQueue(iSvcId),
                                ccs: newClientConns(),
                                toSelf: make(chan *pSl.Header, 1)} //todo larger buffer?
   go runTmtpRecv(iSvcId)
}

func MsgToSelf(iSvcId string, iHead *pSl.Header) {
   getService(iSvcId).toSelf <- iHead
}

func getService(iSvcId string) tService {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   return sServices[iSvcId]
}

func toAllClients(iMsg interface{}) {
   aJson, err := json.Marshal(iMsg)
   if err != nil { panic(err) }
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   for _, aV := range sServices {
      aV.ccs.Range(func(cC *tWsConn) {
         if !cC.test {
            cC.WriteMessage(pWs.TextMessage, aJson)
         }
      })
   }
}

type tQueue struct {
   service string // service name
   connSrc chan net.Conn // synchronize writes to server
   in chan *pSl.SendRecord // message queue input
   out chan *pSl.SendRecord // message queue output
   buf []*pSl.SendRecord // message queue
   ack chan string // ack queue
   wakeup chan bool // reconnect a periodic service
}

func newQueue(iSvcId string) *tQueue {
   var aQ *tQueue
   var aSo sync.Once
   fChan := func(){ go runElasticChan(aQ) }
   aRecs := pSl.GetQueue(iSvcId, func(cR ...*pSl.SendRecord){
      aSo.Do(fChan)
      for _, c := range cR {
         aQ.in <- c
      }
   })
   aQ = &tQueue{
      service: iSvcId,
      connSrc: make(chan net.Conn, 1),
      in: make(chan *pSl.SendRecord),
      out: make(chan *pSl.SendRecord),
      buf: aRecs,
      ack: make(chan string, 2), //todo larger buffer?
      wakeup: make(chan bool),
   }
   go runTmtpSend(aQ)
   if len(aRecs) > 0 {
      aSo.Do(fChan)
   }
   return aQ
}

func (o *tQueue) postAck(iId string) {
   aMsg := pSl.Msg{"Op":eOpAck, "Id":iId, "Type":"ok"}
   aConn := <-o.connSrc
   _, err := aConn.Write(packMsg(tMsg(aMsg), nil))
   if err != nil {
      fmt.Fprintf(os.Stderr, "postAck %s: %s\n", o.service, err)
      //todo maybe aConn.SetDeadline(time.Now()) if pending read doesn't fail
   }
   o.connSrc <- aConn
}

func (o *tQueue) _waitForSrec() *pSl.SendRecord {
   aTmr := time.NewTimer(kPulsePeriod)
   for {
      select {
      case aSrec := <-o.out:
         aTmr.Stop()
         return aSrec
      case <-aTmr.C:
         select {
         case aConn := <-o.connSrc:
            _, err := aConn.Write(packMsg(tMsg{"Op":eOpPulse}, nil))
            if err != nil {
               fmt.Fprintf(os.Stderr, "_waitForSrec %s: %s\n", o.service, err)
               //todo maybe aConn.SetDeadline(time.Now()) if pending read doesn't fail
            }
            o.connSrc <- aConn
         default:
         }
         aTmr.Reset(kPulsePeriod)
      }
   }
}

func runTmtpSend(o *tQueue) {
   aSrec := o._waitForSrec()
   for {
      var aConn net.Conn
      select {
      case aConn = <-o.connSrc:
      case o.wakeup <- true:
         aConn = <-o.connSrc
      }
      err := pSl.SendService(aConn, o.service, aSrec)
      o.connSrc <- aConn
      if err != nil { //todo retry transient error
         if err.Error() == "already sent" {
            aSrec = o._waitForSrec()
         } else {
            fmt.Fprintf(os.Stderr, "runTmtpSend %s: send error %s\n", o.service, err.Error())
            time.Sleep(5 * time.Millisecond)
         }
         continue
      }
      aTmr := time.NewTimer(15 * time.Second)
   WaitForAck:
      select {
      case aMsgId := <-o.ack:
         if aMsgId != aSrec.Id {
            fmt.Fprintf(os.Stderr, "runTmtpSend %s: got ack for %s, expected %s\n",
                        o.service, aMsgId, aSrec.Id)
            goto WaitForAck
         }
         aTmr.Stop()
         aSrec = o._waitForSrec()
      case <-aTmr.C:
         fmt.Fprintf(os.Stderr, "runTmtpSend %s: timeout awaiting ack\n", o.service)
      }
   }
}

func runElasticChan(o *tQueue) {
   var aS *pSl.SendRecord
   var ok bool
   for {
      // buf needs a value to let select multiplex consumer & producer
      if len(o.buf) == 0 {
         aS, ok = <-o.in
         if !ok { goto Closed }
         o.buf = append(o.buf, aS)
      }

      select {
      case aS, ok = <-o.in:
         if !ok { goto Closed }
         o.buf = append(o.buf, aS)
         if len(o.buf) % 100 == 0 {
            fmt.Fprintf(os.Stderr, "runelasticchan %s buf len %d\n", o.service, len(o.buf))
         }
      case o.out <- o.buf[0]:
         o.buf = o.buf[1:]
      }
   }

Closed:
   for _, aS = range o.buf {
      o.out <- aS
   }
   close(o.out)
}

func runTmtpRecv(iSvcId string) {
   aSvc := getService(iSvcId)
   aRng := rand.New(rand.NewSource(time.Now().UnixNano()))
   aDlr := net.Dialer{Timeout: 20*time.Second, KeepAlive: 5*time.Second} //todo drop keepalive
   var err error
   var aConn net.Conn

   for {
      aCfg := pSl.GetConfigService(iSvcId)

      if aCfg.LoginPeriod > 0 && aCfg.Uid != "" {
         // add +/- 0-20% to aCfg.LoginPeriod
         aPercent := aCfg.LoginPeriod / 5
         aRand := aRng.Intn(aPercent * 2 + 1) - aPercent
         aTmr := time.NewTimer(time.Duration(aCfg.LoginPeriod + aRand) * time.Second)
         select {
         case <-aTmr.C:
         case <-aSvc.queue.wakeup:
            aTmr.Stop()
         }
      }

      for aWait := 4; true; aWait *= 2 {
         aCfg = pSl.GetConfigService(iSvcId)
         aCfgTls := tls.Config{InsecureSkipVerify: !aCfg.Verify}
         aConn, err = tls.DialWithDialer(&aDlr, "tcp", aCfg.Addr, &aCfgTls)
         if err == nil {
            break
         }
         aSvc.ccs.Range(func(c *tWsConn) {
            if !c.test {
               c.WriteJSON(pSl.ErrorService(err))
            }
         })
         fmt.Fprintf(os.Stderr, "runTmtpRecv %s: %s\n", iSvcId, err.Error())
         if aWait > kDialRetryDelayMax { aWait = kDialRetryDelayMax }
         time.Sleep(time.Duration(aWait * 1000 + aRng.Intn(1000) * aWait / 2) * time.Millisecond)
      }

      aMsg := tMsg{"Op":eOpTmtpRev, "Id":"1"}
      aConn.Write(packMsg(aMsg, nil))
      if aCfg.Uid == "" {
         aMsg = tMsg{"Op":eOpRegister, "NewAlias":aCfg.Alias, "NewNode":"x"}
      } else {
         aMsg = tMsg{"Op":eOpLogin, "Uid":aCfg.Uid, "Node":aCfg.Node}
      }
      aConn.Write(packMsg(aMsg, nil)) // on error, assume aConn.Read() fails

      err = _readLink(iSvcId, aConn, time.Duration(aCfg.LoginPeriod / kIdleTimeFraction) * time.Second)
      aConn.Close()

      aLogoutMsg := pSl.LogoutService(iSvcId)
      aSvc.ccs.Range(func(c *tWsConn) {
         if !c.test {
            c.WriteJSON(aLogoutMsg)
         }
      })
      if err != nil {
         fmt.Fprintf(os.Stderr, "runTmtpRecv %s: %s\n", iSvcId, err)
         time.Sleep(2 * time.Minute) // don't barrage server if error not transient
      }
   }
}

func _readLink(iSvcId string, iConn net.Conn, iIdleMax time.Duration) error {
   aSvc := getService(iSvcId)
   aBuf := make([]byte, kMsgHeaderMaxLen+4) //todo start smaller, realloc as needed
   aReadFlag := make(chan bool)
   aLogin := false
   var aHead *pSl.Header
   var aPos, aHeadEnd, aHeadStart int64 = 0, 0, 4
   var aLen int
   var err error

   fErr := func(cS string) error {
      if aLogin { <-aSvc.queue.connSrc }
      return tError(cS)
   }
   fNotify := func(cFn func(*pSl.ClientState)[]string, cToAll []string) {
      if cToAll != nil {
         toAllClients(cToAll)
      }
      if cFn != nil {
         aSvc.ccs.Range(func(cC *tWsConn) {
            cMsg := cFn(cC.state)
            if !cC.test && cMsg != nil {
               cC.WriteJSON(cMsg)
            }
         })
      }
   }

   go func() { //todo drop this if net.Conn.Read() can be interrupted
      for <-aReadFlag { // wait for handler
         if iIdleMax > 0 {
            iConn.SetReadDeadline(time.Now().Add(iIdleMax))
         }
         aLen, err = iConn.Read(aBuf[aPos:])
         aReadFlag <- true
      }
   }()
   defer func() { aReadFlag <- false }()

   for {
      aReadFlag <- true // signal to reader
   WaitForMsg:
      select {
      case aHd := <-aSvc.toSelf:
         fNotify(pSl.HandleTmtpService(iSvcId, aHd, nil))
         goto WaitForMsg
      case <-aReadFlag:
      }
      if err != nil {
         //todo if recoverable continue
         if err == io.EOF {
            return fErr("server close")
         } else if err.(net.Error).Timeout() {
            select {
            case <-aSvc.queue.connSrc:
               // if runTmtpSend is awaiting ack, we will miss it and retry
               fmt.Printf("_readLink %s: idle timeout\n", iSvcId)
            default:
               if aLogin {
                  aSvc.queue.connSrc <- <-aSvc.queue.connSrc // wait for send to finish
                  iIdleMax = 15 * time.Second // allow time for ack
                  continue
               }
            }
            return nil
         } else {
            return fErr(err.Error())
         }
      }
      aPos += int64(aLen)
   Parse:
      if aPos < kMsgHeaderMinLen+4 {
         continue
      }
      if aHeadEnd == 0 {
         aUi,_ := strconv.ParseUint(string(aBuf[:4]), 16, 0)
         aHeadEnd = int64(aUi)+4
         if aHeadEnd-4 < kMsgHeaderMinLen {
            return fErr("invalid header length")
         }
      }
      if aHeadEnd > aPos {
         continue
      }
      if aHeadStart == 4 {
         aHead = &pSl.Header{Op:""}
         err = json.Unmarshal(aBuf[4:aHeadEnd], aHead)
         if err != nil || !aHead.Check() {
            return fErr("invalid header")
         }
         aHeadStart = aHeadEnd
         aHeadEnd += aHead.DataHead
         aHead.DataLen -= aHead.DataHead
         if aHeadEnd > aPos {
            continue
         }
      }
      if aHeadEnd > aHeadStart {
         err = json.Unmarshal(aBuf[aHeadStart:aHeadEnd], &aHead.SubHead)
         if err != nil || !aHead.CheckSub() {
            return fErr("invalid subheader")
         }
      }
      aData := aBuf[aHeadEnd:aHeadEnd] // ref aBuf even if DataLen==0
      if aPos > aHeadEnd && aHead.DataLen > 0 {
         aEnd := aHeadEnd + aHead.DataLen; if aPos < aEnd { aEnd = aPos }
         aData = aBuf[aHeadEnd:aEnd]
      }
      if aHead.Op == "ack" && aHead.Id == kFirstOhiId {
         // no-op
      } else {
         if aHead.Op == "info" && aHead.Info == "login ok" {
            pSl.SendAllOhi(iConn, iSvcId, kFirstOhiId)
            aLogin = true
            aSvc.queue.connSrc <- iConn
         } else if aHead.Op == "ack" {
            select {
            case aSvc.queue.ack <- aHead.Id:
            default:
               fmt.Fprintf(os.Stderr, "_readLink %s: ack channel blocked\n", iSvcId)
            }
         }
         if aHead.SubHead == nil || !aHead.SubHead.NodeSync {
            fNotify(pSl.HandleTmtpService(iSvcId, aHead, &tTmtpInput{aData, iConn}))
         } else {
            pSl.HandleSyncService(iSvcId, aHead, &tTmtpInput{aData, iConn}, fNotify)
         }
         if aHead.From != "" && aHead.Id != "" {
            aSvc.queue.postAck(aHead.Id)
         }
      }
      if aPos > aHeadEnd + aHead.DataLen {
         aPos = int64(copy(aBuf, aBuf[aHeadEnd + aHead.DataLen : aPos]))
         aHeadEnd, aHeadStart = 0, 4
         goto Parse
      }
      aPos, aHeadEnd, aHeadStart = 0, 0, 4
   }
}

type tTmtpInput struct {
   Buf []byte
   R io.Reader
}

func (o *tTmtpInput) Read(iOut []byte) (int, error) {
   if len(o.Buf) == 0 {
      return o.R.Read(iOut)
   }
   aLen := copy(iOut, o.Buf)
   o.Buf = o.Buf[aLen:]
   if aLen == len(iOut) {
      return aLen, nil
   }
   aLen2, err := o.R.Read(iOut[aLen:])
   return aLen+aLen2, err
}

var kStateOp = map[string]bool{
   "cs":true, "cl":true, "al":true, "ml":true, "tl":true, "mo":true, "mn":true, "an":true, "ad":true,
}

func runService(iResp http.ResponseWriter, iReq *http.Request) {
   // expects "/service[?op[=id]]"
   var err error
   aClientId, _ := iReq.Cookie("clientid")
   var aState *pSl.ClientState
   aSvcId := iReq.URL.Path[1:]; if aSvcId == "" { aSvcId = "local" }
   aOp_Id := []string{"er", ""}
   aQuery, err := url.QueryUnescape(iReq.URL.RawQuery)
   if err == nil {
      aSvc := getService(aSvcId)
      if aSvc.ccs == nil {
         err = tError("service not found")
      } else if len(aQuery) >= 2 && kStateOp[aQuery[:2]] {
         aCid := ""; if aClientId != nil { aCid = aClientId.Value }
         aCc := aSvc.ccs.Get(aCid)
         if aCc == nil {
            err = tError("no client connected to service")
         } else {
            aState = aCc.state
         }
      }
      if err == nil {
         copy(aOp_Id, strings.SplitN(aQuery, "=", 2))
      }
   }
   if sTestHost == "" {
      fmt.Printf("runService %s: %s %s\n", aSvcId, aOp_Id[0], aOp_Id[1])
   }
   iResp.Header().Set("Content-Type", "text/plain; charset=utf-8")
   var aResult interface{}

   switch aOp_Id[0] {
   case "": // service template
      if aClientId == nil {
         aClientId = &http.Cookie{Name: "clientid", SameSite: http.SameSiteLaxMode,
                                  Expires: time.Date(5678, 1, 2, 3, 4, 56, 78, time.UTC), //todo sooner?
                                  Value: fmt.Sprint(time.Now().UTC().UnixNano())}
         http.SetCookie(iResp, aClientId)
      } else if aClientId.SameSite != http.SameSiteLaxMode { // drop after 0.8
         aClientId.SameSite = http.SameSiteLaxMode
         aClientId.Expires = time.Date(5678, 1, 2, 3, 4, 56, 78, time.UTC)
         http.SetCookie(iResp, aClientId)
      }
      aAboutTag := getAbout().etag()
      iResp.Header().Set("Cache-Control", "private, max-age=0, no-cache")
      iResp.Header().Set("ETag", aAboutTag)
      if iReq.Header.Get("If-None-Match") == aAboutTag {
         iResp.WriteHeader(http.StatusNotModified)
         return
      }
      iResp.Header().Set("Content-Type", "text/html; charset=utf-8")
      aSvcIdJs := strings.ReplaceAll(template.JSEscapeString(aSvcId), `"`, `x22`) // avoid v-attr="'\"'"
      aParams := pSl.GetConstants(tMsg{"Title":aSvcId, "TitleJs":aSvcIdJs, "Addr":sHttpSrvr.Addr})
      err = sServiceTmpl.Execute(iResp, aParams)
   case "cs": aResult = aState.GetSummary()
   case "cf": aResult = pSl.GetCfService(aSvcId)
   case "cn": aResult = pSl.GetCnNode(aSvcId)
   case "nl": aResult = pSl.GetIdxNotice(aSvcId)
   case "fl": aResult = pSl.GetIdxFilledForm(aSvcId)
   case "ps": aResult = pSl.GetDraftAdrsbk(aSvcId)
   case "pt": aResult = pSl.GetSentAdrsbk(aSvcId)
   case "pf": aResult = pSl.GetReceivedAdrsbk(aSvcId)
   case "gl": aResult = pSl.GetGroupAdrsbk(aSvcId)
   case "of": aResult = pSl.GetFromOhi(aSvcId)
   case "ot": aResult = pSl.GetToOhi(aSvcId)
   case "cl": aResult = pSl.GetCcThread(aSvcId, aState)
   case "al": aResult = pSl.GetIdxAttach(aSvcId, aState)
   case "ml": aResult = pSl.GetIdxThread(aSvcId, aState)
   case "tl":
      err = pSl.WriteResultSearch(iResp, aSvcId, aState)
   case "mo":
      err = pSl.WriteMessagesThread(iResp, aSvcId, aState, "")
   case "mn":
      if aOp_Id[1] == "" {
         err = tError("missing Id")
         break
      }
      err = pSl.WriteMessagesThread(iResp, aSvcId, aState, aOp_Id[1])
   case "an", "ad":
      aDelim := strings.IndexByte(aOp_Id[1], '_')
      if aDelim < 0 || len(aOp_Id[1]) <= aDelim+3 {
         err = tError("invalid id")
         break
      }
      if aOp_Id[0] == "ad" {
         iResp.Header().Set("Content-Disposition",
                            "attachment; filename*=UTF-8''" + escapeFile(aOp_Id[1][aDelim+3:]))
      }
      iResp.Header().Del("Content-Type") // let ServeFile() infer type
      iResp.Header().Set("Cache-Control", "private, max-age=0, no-cache") //todo compare checksums
      http.ServeFile(iResp, iReq, pSl.GetPathAttach(aSvcId, aState, aOp_Id[1][:aDelim],
                                                                    aOp_Id[1][aDelim+1:]))
   default:
      if err == nil {
         err = tError("unknown op")
      }
   }
   if err != nil {
      fmt.Fprintf(os.Stderr, "runService %s: op %s %s\n", aSvcId, aOp_Id[0], err)
      iResp.WriteHeader(http.StatusNotAcceptable)
      aResult = err.Error()
   }
   if aResult != nil {
      err = json.NewEncoder(iResp).Encode(aResult)
      if err != nil {
         fmt.Fprintf(os.Stderr, "runService %s: %s\n", aSvcId, err)
      }
   }
}

func runAbout(iResp http.ResponseWriter, iReq *http.Request) {
   if sTestHost == "" {
      fmt.Printf("runAbout %s %s\n", iReq.Method, iReq.URL.Path)
   }
   err := json.NewEncoder(iResp).Encode(getAbout())
   if err != nil { fmt.Fprintf(os.Stderr, "runAbout: %s\n", err.Error()) }
}

type tAbout struct { Version, VersionDate, HttpAddr string }

func getAbout() *tAbout {
   return &tAbout{ fmt.Sprintf("%d.%d.%d", kVersionA, kVersionB, kVersionC),
                   kVersionDate, sHttpSrvr.Addr }
}

func (o *tAbout) etag() string { return o.Version +" "+ o.VersionDate }

func runNodeListen(iResp http.ResponseWriter, iReq *http.Request) {
   if sTestHost == "" {
      fmt.Printf("runNodeListen %s %s\n", iReq.Method, iReq.URL.Path)
   }
   if iReq.Method == "POST" {
      aToAll := pSl.ListenNode()
      toAllClients(aToAll)
   } else {
      err := json.NewEncoder(iResp).Encode(pSl.GetPinNode(sNetAddr))
      if err != nil { fmt.Fprintf(os.Stderr, "runListen: %v\n", err) }
   }
}

func runNodeRecv(iResp http.ResponseWriter, iReq *http.Request) {
   if sTestHost == "" {
      fmt.Printf("runNodeRecv %s %s?%s\n", iReq.Method, iReq.URL.Path, iReq.URL.RawQuery)
   }
   // network peer is a separate mnm app instance
   aQuery, err := url.QueryUnescape(iReq.URL.RawQuery)
   if err != nil || !pSl.CheckPinNode(aQuery) {
      iResp.WriteHeader(http.StatusForbidden)
      return
   }
   if iReq.Method == "GET" {
      if len(iReq.URL.Path) > 3 {
         aToAll := pSl.StartNode(iReq.URL.Path[3:])
         if aToAll == nil {
            iResp.WriteHeader(http.StatusNotFound)
            return
         }
         toAllClients(aToAll)
      }
   } else if iReq.Method == "POST" {
      err = pSl.MakeNode(iReq.Body)
      if err != nil {
         fmt.Fprintf(os.Stderr, "runNodeRecv: %v\n", err)
         iResp.WriteHeader(http.StatusNotAcceptable)
         iResp.Write([]byte(err.Error()))
      }
   } else {
      iResp.WriteHeader(http.StatusMethodNotAllowed)
   }
}

func runGlobal(iResp http.ResponseWriter, iReq *http.Request) {
   if sTestHost == "" {
      fmt.Printf("runGlobal %s %s\n", iReq.Method, iReq.URL.Path)
   }
   var aSet pSl.GlobalSet
   switch iReq.URL.Path[1] {
   case 'f': aSet = pSl.BlankForm
   case 't': aSet = pSl.Upload
   case 'v': aSet = pSl.Service
   }
   aId := iReq.URL.Path[3:]
   fErr := func(cSt int, cMsg string) { iResp.WriteHeader(cSt); iResp.Write([]byte(cMsg)) }
   if iReq.Method == "POST" {
      if aId[0] == '+' {
         var aPart *multipart.Part
         aR, err := iReq.MultipartReader()
         if err == nil {
            aPart, err = aR.NextPart()
         }
         if err != nil {
            fErr(http.StatusNotAcceptable, "form: " + err.Error())
            return
         }
         defer aPart.Close()
         err = aSet.Add(aId[1:], "", aPart)
         if err != nil {
            fErr(http.StatusInternalServerError, "add: " + err.Error())
            return
         }
      } else if aId[0] == '*' {
         var err error
         aPrev_New := strings.SplitN(aId[1:], "+", 2)
         if len(aPrev_New) < 2 || aPrev_New[1] == "" {
            err = tError("missing + param")
         } else {
            err = aSet.Add(aPrev_New[0], aPrev_New[1], nil)
         }
         if err != nil {
            fErr(http.StatusNotAcceptable, "duplicate: " + err.Error())
            return
         }
      } else if aId[0] == '-' {
         err := aSet.Drop(aId[1:])
         if err != nil {
            fErr(http.StatusNotAcceptable, "drop: " + err.Error())
            return
         }
      } else {
         fErr(http.StatusNotAcceptable, "missing +/- operator")
         return
      }
      toAllClients([]string{iReq.URL.Path[:2]})
      iResp.Write([]byte(`"ok"`))
   } else if aId == "" {
      aIdx := aSet.GetIdx()
      err := json.NewEncoder(iResp).Encode(aIdx)
      if err != nil { fmt.Fprintf(os.Stderr, "runGlobal: %s\n", err.Error()) }
   } else {
      if aId[0] == '=' {
         aId = aId[1:]
         iResp.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+ escapeFile(aId))
      }
      aPath := aSet.GetPath(aId)
      if aPath == "" {
         fErr(http.StatusNotAcceptable, "get: not a file type: "+ iReq.URL.Path)
         return
      }
      iResp.Header().Set("Cache-Control", "private, max-age=0, no-cache") //todo compare checksums
      http.ServeFile(iResp, iReq, aPath)
   }
}

func runTag(iResp http.ResponseWriter, iReq *http.Request) {
   if sTestHost == "" {
      fmt.Printf("runTag %s %s\n", iReq.Method, iReq.URL.Path)
   }
   err := json.NewEncoder(iResp).Encode(pSl.GetIdxTag())
   if err != nil { fmt.Fprintf(os.Stderr, "runTag: %v\n", err) }
}

var kWsInit = pWs.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

func runWebsocket(iResp http.ResponseWriter, iReq *http.Request) {
   aSvcId := iReq.URL.Path[3:]; if aSvcId == "" { aSvcId = "local" }
   aSvc := getService(aSvcId)
   if aSvc.ccs == nil {
      fmt.Fprintf(os.Stderr, "runWebsocket %s: not found\n", aSvcId)
      iResp.WriteHeader(http.StatusNotFound)
      iResp.Write([]byte("service not found: "+aSvcId))
      return
   }

   var aState *pSl.ClientState
   aClientId, _ := iReq.Cookie("clientid")
   aCc := aSvc.ccs.Get(aClientId.Value)
   if aCc != nil {
      aCc.WriteJSON([]string{"_e", "new connection from same client"})
      aCc.conn.Close()
      aState = aCc.state
   } else {
      aState = pSl.OpenState(aClientId.Value, aSvcId)
   }
   aSock, err := kWsInit.Upgrade(iResp, iReq, nil)
   if err != nil { panic(err) }
   aSvc.ccs.Set(aClientId.Value, &tWsConn{conn: aSock, state: aState, test: iReq.URL.Path[1] == '5'})

   for {
      _, aJson, err := aSock.ReadMessage()
      if err != nil {
         if sTestHost == "" || !pWs.IsCloseError(err, pWs.CloseGoingAway) {
            fmt.Fprintf(os.Stderr, "runWebsocket %s: readmsg: %s\n", aSvcId, err.Error())
         }
         if strings.HasSuffix(err.Error(), "use of closed network connection") {
            return // don't .Drop(aClientId.Value)
         }
         break
      }
      if sTestHost == "" {
         fmt.Printf("runWebsocket %s: %s\n", aSvcId, string(aJson))
      }

      var aUpdate pSl.Update
      err = json.Unmarshal(aJson, &aUpdate)
      if err != nil { panic(err) }
      aFn, aToAll := pSl.HandleUpdtService(aSvcId, aState, &aUpdate)
      if aToAll != nil {
         toAllClients(aToAll)
      }
      if aFn != nil {
         aSvc.ccs.Range(func(cC *tWsConn) {
            cMsg := aFn(cC.state)
            if cMsg != nil {
               cC.WriteJSON(cMsg)
            }
         })
      }
   }
   aSvc.ccs.Drop(aClientId.Value)
}

func runFile(iResp http.ResponseWriter, iReq *http.Request) {
   aAboutTag := getAbout().etag()
   iResp.Header().Set("Cache-Control", "private, max-age=0, no-cache")
   iResp.Header().Set("ETag", aAboutTag)
   if iReq.Header.Get("If-None-Match") == aAboutTag {
      iResp.WriteHeader(http.StatusNotModified)
      return
   }
   http.ServeFile(iResp, iReq, "web"+ iReq.URL.Path[2:])
}

func runFavicon(iResp http.ResponseWriter, iReq *http.Request) {
   iResp.WriteHeader(http.StatusNotFound)
}


type tClientConns struct {
   sync.RWMutex
   m map[string]*tWsConn // key is client id
}

func newClientConns() *tClientConns {
   return &tClientConns{m: make(map[string]*tWsConn)}
}

func (o *tClientConns) Get(iClient string) *tWsConn {
   o.RLock(); defer o.RUnlock()
   return o.m[iClient]
}

func (o *tClientConns) Range(iFn func(*tWsConn)) {
   o.RLock(); defer o.RUnlock()
   for _, aV := range o.m { iFn(aV) }
}

func (o *tClientConns) Set(iClient string, iConn *tWsConn) {
   o.Lock(); defer o.Unlock()
   o.m[iClient] = iConn
}

func (o *tClientConns) Drop(iClient string) {
   o.Lock(); defer o.Unlock()
   delete(o.m, iClient)
}

type tWsConn struct {
   sync.Mutex // protect conn.WriteMessage/JSON()
   conn *pWs.Conn
   state *pSl.ClientState
   test bool
}

func (o *tWsConn) WriteMessage(iT int, iB []byte) {
   o.Lock(); defer o.Unlock()
   err := o.conn.WriteMessage(iT, iB)
   if err != nil {
      fmt.Fprintf(os.Stderr, "WriteMessage: %s\n", err.Error())
   }
}

func (o *tWsConn) WriteJSON(i interface{}) {
   o.Lock(); defer o.Unlock()
   err := o.conn.WriteJSON(i)
   if err != nil {
      fmt.Fprintf(os.Stderr, "WriteJSON: %s\n", err.Error())
   }
}

type tMsg map[string]interface{}

func packMsg(iJso tMsg, iData []byte) []byte {
   aHead, err := json.Marshal(iJso)
   if err != nil { panic(err) }
   aLen := fmt.Sprintf("%04x", len(aHead))
   if len(aLen) != 4 { panic("packmsg json input too long") }
   aBuf := make([]byte, 0, 4+len(aHead)+len(iData))
   aBuf = append(aBuf, aLen...)
   aBuf = append(aBuf, aHead...)
   aBuf = append(aBuf, iData...)
   return aBuf
}

func escapeFile(i string) string {
   if i == ".." || i == "." || pSl.IsReservedFile(i) {
      return i + url.QueryEscape("\u25a1")
   }
   return url.QueryEscape(i)
}

func dateRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

func quit(err error) {
   fmt.Fprintf(os.Stderr, "quit after %s\n", err.Error())
   debug.PrintStack()
   os.Exit(3)
}

type tError string
func (o tError) Error() string { return string(o) }

