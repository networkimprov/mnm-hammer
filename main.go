// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package main

//todo
// secure link to clients
// handle client msgs
//   addnode, dropnode
//   addservice
//   addalias, dropalias
//   groupinvite, etc
//   addohi, dropohi
//   savemsg, sendmsg
//   addform, revform
//   getservices, getthreads, getmsgs, getopenmsgs
// track form subscribers (clients with form in msg-editor) to send update on rev

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
   pSl "mnm-hammer/slib"
   pWs "github.com/gorilla/websocket-1.2.0"
   "strconv"
   "strings"
   "sync"
   "html/template"
   "time"
   "crypto/tls"
   "net/url"
)

const kVersionA, kVersionB, kVersionC = 0, 0, 0
const kVersionDate = "(unreleased)"
const kIdleTimeFraction = 10
const kMsgHeaderMinLen = int64(len(`{"op":1}`))
const kMsgHeaderMaxLen = int64(1 << 16)
const kFirstOhiId = "first_ohi"

const (
   eOpTmtpRev = iota
   eOpRegister; eOpLogin
   eOpUserEdit; eOpOhiEdit;
   eOpGroupInvite; eOpGroupEdit
   eOpPost; eOpPing
   eOpAck; eOpQuit
   eOpEnd
)

var sHttpSrvr = http.Server{Addr: ":80"}
var sServicesDoor sync.RWMutex
var sServices = make(map[string]tService)
var sServiceTmpl *template.Template


func main() {
   flag.StringVar(&sHttpSrvr.Addr, "http", sHttpSrvr.Addr, "[host]:port of http server")
   flag.Parse() // may os.Exit(2)
   os.Exit(mainResult())
}

func mainResult() int {
   // return 2 reserved for use by Go internals
   var err error

   fmt.Printf("mnm-hammer tmtp client v%d.%d.%d %s\n", kVersionA, kVersionB, kVersionC, kVersionDate)

   sServices["local"] = tService{ccs: newClientConns()}

   if sTestHost != "" {
      test()
   } else {
      _, err = os.Stat("web/service.html")
      if err != nil {
         err = os.Chdir(path.Dir(os.Args[0]))
         if err != nil {
            fmt.Fprintf(os.Stderr, "chdir: %s\n", err.Error())
            return 1
         }
      }
      pSl.Init(startService)
   }

   sServiceTmpl, err = template.New("service.html").Delims("[{","}]").ParseFiles("web/service.html")
   if err != nil {
      fmt.Fprintf(os.Stderr, "template parse error %s\n", err.Error())
      return 1
   }

   http.HandleFunc("/"  , runService)
   http.HandleFunc("/a/", runAbout)
   http.HandleFunc("/t/", runGlobal)
   http.HandleFunc("/f/", runGlobal)
   http.HandleFunc("/v/", runGlobal)
   http.HandleFunc("/s/", runWebsocket)
   http.HandleFunc("/web/", runFile)
   err = sHttpSrvr.ListenAndServe()
   fmt.Fprintf(os.Stderr, "http server stopped %s\n", err.Error())

   return 0
}

type tService struct {
   queue *tQueue
   ccs *tClientConns
}

func startService(iSvcId string) {
   sServicesDoor.Lock(); defer sServicesDoor.Unlock()
   if sServices[iSvcId].ccs != nil {
      panic(fmt.Sprintf("startService %s: already started", iSvcId))
   }
   sServices[iSvcId] = tService{queue: newQueue(iSvcId), ccs: newClientConns()}
   go runTmtpRecv(iSvcId)
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
         cC.WriteMessage(pWs.TextMessage, aJson)
      })
   }
}

type tQueue struct {
   sync.Once // to start runTmtpSend
   once func() // input to .Do()
   service string // service name
   connSrc chan net.Conn // synchronize writes to server
   in chan *pSl.SendRecord // message queue input
   out chan *pSl.SendRecord // message queue output
   buf []*pSl.SendRecord // message queue
   ack chan string // ack queue
   wakeup chan bool // reconnect a periodic service
}

func newQueue(iSvcId string) *tQueue {
   aRecs, err := pSl.GetQueueService(iSvcId)
   if err != nil {
      fmt.Fprintf(os.Stderr, "newqueue %s failure: %s\n", iSvcId, err.Error())
      return nil
   }
   var aQ *tQueue
   aQ = &tQueue{
      once: func(){ go runElasticChan(aQ); go runTmtpSend(aQ) },
      service: iSvcId,
      connSrc: make(chan net.Conn, 1),
      in: make(chan *pSl.SendRecord),
      out: make(chan *pSl.SendRecord),
      buf: aRecs,
      ack: make(chan string, 2), //todo larger buffer?
      wakeup: make(chan bool),
   }
   if len(aRecs) > 0 {
      aQ.Do(aQ.once)
   }
   return aQ
}

func (o *tQueue) postMsg(iRec *pSl.SendRecord) {
   o.Do(o.once)
   o.in <- iRec
}

func (o *tQueue) postAck(iId string) {
   aMsg := pSl.Msg{"Op":eOpAck, "Id":iId, "Type":"ok"}
   aConn := <-o.connSrc
   _, err := aConn.Write(packMsg(tMsg(aMsg), nil))
   if err != nil { panic(err) }
   o.connSrc <- aConn
}

func runTmtpSend(o *tQueue) {
   aSrec := <-o.out
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
            aSrec = <-o.out
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
         aSrec = <-o.out
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
   var err error
   var aConn net.Conn
   var aJson []byte

   for {
      aCfg := pSl.GetDataService(iSvcId)

      if aCfg.LoginPeriod > 0 && aCfg.Uid != "" {
         // add +/- 0-20% to aCfg.LoginPeriod
         aPercent := aCfg.LoginPeriod / 5
         aRand := time.Now().Nanosecond() % (aPercent * 2 + 1) - aPercent
         aTmr := time.NewTimer(time.Duration(aCfg.LoginPeriod + aRand) * time.Second)
         select {
         case <-aTmr.C:
         case <-aSvc.queue.wakeup:
            aTmr.Stop()
         }
      }

      for {
         aCfg = pSl.GetDataService(iSvcId)
         aCfgTls := tls.Config{InsecureSkipVerify: !aCfg.Verify}
         aDlr := net.Dialer{Timeout: 3 * time.Second}
         aConn, err = tls.DialWithDialer(&aDlr, "tcp", aCfg.Addr, &aCfgTls)
         if err == nil { break }
         aSvc.ccs.Range(func(c *tWsConn) {
            c.WriteJSON(err.Error())
         })
         fmt.Fprintf(os.Stderr, "runTmtpRecv %s: %s\n", iSvcId, err.Error())
         time.Sleep(time.Duration(5000 + time.Now().Nanosecond() % 1000 * 5) * time.Millisecond)
      }

      aMsg := tMsg{"Op":eOpTmtpRev, "Id":"1"}
      aConn.Write(packMsg(aMsg, nil))
      if aCfg.Uid == "" {
         aMsg = tMsg{"Op":eOpRegister, "NewAlias":aCfg.Alias, "NewNode":"x"}
      } else {
         aMsg = tMsg{"Op":eOpLogin, "Uid":aCfg.Uid, "Node":aCfg.Node}
      }
      aConn.Write(packMsg(aMsg, nil))

      _readLink(iSvcId, aConn, time.Duration(aCfg.LoginPeriod / kIdleTimeFraction) * time.Second)
      aConn.Close()

      aJson, err = json.Marshal(pSl.LogoutService(iSvcId))
      if err != nil { panic(err) }
      aSvc.ccs.Range(func(c *tWsConn) {
         c.WriteMessage(pWs.TextMessage, aJson)
      })
   }
}

func _readLink(iSvcId string, iConn net.Conn, iIdleMax time.Duration) {
   aSvc := getService(iSvcId)
   aBuf := make([]byte, kMsgHeaderMaxLen+4) //todo start smaller, realloc as needed
   aLogin := false
   var aHead *pSl.Header
   var aPos, aHeadEnd, aHeadStart int64 = 0, 0, 4

   for {
      if iIdleMax > 0 {
         iConn.SetReadDeadline(time.Now().Add(iIdleMax))
      }
      aLen, err := iConn.Read(aBuf[aPos:])
      if err != nil {
         //todo if recoverable continue
         if err == io.EOF {
            fmt.Fprintf(os.Stderr, "_readLink %s: server close\n", iSvcId)
            break
         } else if err.(net.Error).Timeout() {
            select {
            case <-aSvc.queue.connSrc:
               // if runTmtpSend is awaiting ack, we will miss it and retry
               fmt.Printf("_readLink %s: idle timeout\n", iSvcId)
            default:
               if aLogin {
                  aSvc.queue.connSrc <- <-aSvc.queue.connSrc // wait for send to finish
                  iIdleMax += 15 * time.Second // allow time for ack
                  continue
               }
            }
            return
         } else {
            fmt.Fprintf(os.Stderr, "_readLink %s: %s\n", iSvcId, err.Error())
            break
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
            fmt.Fprintf(os.Stderr, "_readLink %s: invalid header length\n", iSvcId)
            break
         }
      }
      if aHeadEnd > aPos {
         continue
      }
      if aHeadStart == 4 {
         aHead = &pSl.Header{Op:""}
         err = json.Unmarshal(aBuf[4:aHeadEnd], aHead)
         if err != nil || !aHead.Check() {
            fmt.Fprintf(os.Stderr, "_readLink %s: invalid header\n", iSvcId)
            break
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
            fmt.Fprintf(os.Stderr, "_readLink %s: invalid header\n", iSvcId)
            break
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
         aFn := pSl.HandleTmtpService(iSvcId, aHead, &tTmtpInput{aData, iConn})
         if aHead.From != "" && aHead.Id != "" {
            aSvc.queue.postAck(aHead.Id)
         }
         if aFn != nil {
            for _, aStEl := range sTestState {
               if aStEl.svcId == iSvcId { aFn(aStEl.state) }
            }
            aSvc.ccs.Range(func(cC *tWsConn) {
               cMsg := aFn(cC.state)
               if cMsg == nil { return }
               cC.WriteJSON(cMsg)
            })
         }
      }
      if aPos > aHeadEnd + aHead.DataLen {
         aPos = int64(copy(aBuf, aBuf[aHeadEnd + aHead.DataLen : aPos]))
         aHeadEnd, aHeadStart = 0, 4
         goto Parse
      }
      aPos, aHeadEnd, aHeadStart = 0, 0, 4
   }
   if aLogin {
      <-aSvc.queue.connSrc
   }
}

type tTmtpInput struct { Buf []byte; R io.Reader }

func (o *tTmtpInput) Read(iOut []byte) (int, error) {
   aLen := 0
   if len(o.Buf) > 0 {
      aLen = copy(iOut, o.Buf)
      o.Buf = o.Buf[aLen:]
   }
   if aLen < len(iOut) {
      aLen2, err := o.R.Read(iOut[aLen:])
      if err != nil {
         return aLen+aLen2, err //todo only network errors
      }
   }
   return len(iOut), nil
}

func runService(iResp http.ResponseWriter, iReq *http.Request) {
   // expects "/service[?op[=id]]"
   var err error
   aClientId, _ := iReq.Cookie("clientid")
   aCid := ""; if aClientId != nil { aCid = aClientId.Value }
   var aState *pSl.ClientState
   aSvcId := iReq.URL.Path[1:]; if aSvcId == "" { aSvcId = "local" }
   aOp_Id := []string{"er", ""}
   aQuery, err := url.QueryUnescape(iReq.URL.RawQuery)
   if err == nil {
      aSvc := getService(aSvcId)
      if aSvc.ccs == nil {
         err = tError("service not found")
      } else if aQuery != "" {
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
   fmt.Printf("runService %s: op %s id %s\n", aSvcId, aOp_Id[0], aCid)
   var aResult interface{}

   switch aOp_Id[0] {
   case "": // service template
      if aClientId == nil {
         aClientId = &http.Cookie{Name: "clientid", Value: fmt.Sprint(time.Now().UTC().UnixNano())}
         http.SetCookie(iResp, aClientId)
      }
      err = sServiceTmpl.Execute(iResp, tMsg{"Title":aSvcId, "Addr":sHttpSrvr.Addr})
   case "cs": aResult = aState.GetSummary()
   case "cf": aResult = pSl.GetDataService(aSvcId)
   case "ps": aResult = pSl.GetDraftAdrsbk(aSvcId)
   case "pt": aResult = pSl.GetSentAdrsbk(aSvcId)
   case "pf": aResult = pSl.GetReceivedAdrsbk(aSvcId)
   case "it": aResult = pSl.GetInviteToAdrsbk(aSvcId)
   case "if": aResult = pSl.GetInviteFromAdrsbk(aSvcId)
   case "gl": aResult = pSl.GetGroupAdrsbk(aSvcId)
   case "of": aResult = pSl.GetFromOhi(aSvcId)
   case "ot": aResult = pSl.GetIdxOhi(aSvcId)
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
   case "an":
      iResp.Header().Set("Cache-Control", "private, max-age=0, no-cache") //todo compare checksums
      http.ServeFile(iResp, iReq, pSl.GetPathAttach(aSvcId, aState, aOp_Id[1]))
   case "fn":
      err = pSl.WriteTableFilledForm(iResp, aSvcId, aOp_Id[1])
   default:
      if err == nil { err = tError("unknown op") }
   }
   if err != nil {
      iResp.WriteHeader(http.StatusNotAcceptable)
      aResult = err.Error()
      fmt.Fprintf(os.Stderr, "runService %s: op %s %s\n", aSvcId, aOp_Id[0], err.Error())
   }
   if aResult != nil {
      err = json.NewEncoder(iResp).Encode(aResult)
      if err != nil { fmt.Fprintf(os.Stderr, "runService %s: %s\n", aSvcId, err.Error()) }
   }
}

func runAbout(iResp http.ResponseWriter, iReq *http.Request) {
   err := json.NewEncoder(iResp).Encode(getAbout())
   if err != nil { fmt.Fprintf(os.Stderr, "runAbout: %s\n", err.Error()) }
}

type tAbout struct { Version, VersionDate, HttpAddr string }

func getAbout() *tAbout {
   return &tAbout{ fmt.Sprintf("%d.%d.%d", kVersionA, kVersionB, kVersionC),
                   kVersionDate, sHttpSrvr.Addr }
}

func runGlobal(iResp http.ResponseWriter, iReq *http.Request) {
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
            fErr(http.StatusNotAcceptable, "form error: " + err.Error())
            return
         }
         defer aPart.Close()
         err = aSet.Add(aId[1:], "", aPart)
         if err != nil {
            fErr(http.StatusInternalServerError, "add error: " + err.Error())
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
            fErr(http.StatusNotAcceptable, "duplicate error: " + err.Error())
            return
         }
      } else if aId[0] == '-' {
         err := aSet.Drop(aId[1:])
         if err != nil {
            fErr(http.StatusNotAcceptable, "drop error: " + err.Error())
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
      aPath := aSet.GetPath(aId)
      if aPath == "" {
         fErr(http.StatusNotAcceptable, "not a file type")
         return
      }
      iResp.Header().Set("Cache-Control", "private, max-age=0, no-cache") //todo compare checksums
      http.ServeFile(iResp, iReq, aPath)
   }
}

var sWsInit = pWs.Upgrader {
  ReadBufferSize:  1024,
  WriteBufferSize: 1024,
}

func runWebsocket(iResp http.ResponseWriter, iReq *http.Request) {
   aSvcId := iReq.URL.Path[3:]; if aSvcId == "" { aSvcId = "local" }
   aSvc := getService(aSvcId)
   if aSvc.ccs == nil {
      iResp.WriteHeader(http.StatusNotFound)
      iResp.Write([]byte("service not found: "+aSvcId))
      return
   }

   var aState *pSl.ClientState
   aClientId, _ := iReq.Cookie("clientid")
   aCc := aSvc.ccs.Get(aClientId.Value)
   if aCc != nil {
      aCc.WriteMessage(pWs.TextMessage, []byte("new connection from same client"))
      aCc.conn.Close()
      aState = aCc.state
   } else {
      aState = pSl.OpenState(aClientId.Value, aSvcId)
   }
   aSock, err := sWsInit.Upgrade(iResp, iReq, nil)
   if err != nil { panic(err) }
   aSvc.ccs.Set(aClientId.Value, &tWsConn{conn: aSock, state: aState})

   for {
      _, aJson, err := aSock.ReadMessage()
      if err != nil {
         fmt.Fprintf(os.Stderr, "runWebsocket %s: readmsg: %s\n", aSvcId, err.Error())
         if strings.HasSuffix(err.Error(), "use of closed network connection") {
            return // don't .Drop(aClientId.Value)
         }
         break
      }
      fmt.Printf("runWebsocket %s: msg %s\n", aSvcId, string(aJson))

      var aUpdate pSl.Update
      err = json.Unmarshal(aJson, &aUpdate)
      if err != nil { panic(err) }
      aFn, aSrec := pSl.HandleUpdtService(aSvcId, aState, &aUpdate)

      if aFn != nil {
         aSvc.ccs.Range(func(cC *tWsConn) {
            cMsg := aFn(cC.state)
            if cMsg == nil { return }
            cC.WriteJSON(cMsg)
         })
      }
      if aSrec != nil {
         aSvc.queue.postMsg(aSrec)
      }
   }
   aSvc.ccs.Drop(aClientId.Value)
}

func runFile(iResp http.ResponseWriter, iReq *http.Request) {
   http.ServeFile(iResp, iReq, iReq.URL.Path[1:])
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

func dateRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

func quit(err error) {
   fmt.Fprintf(os.Stderr, "quit after %s\n", err.Error())
   debug.PrintStack()
   os.Exit(3)
}

type tError string
func (o tError) Error() string { return string(o) }

