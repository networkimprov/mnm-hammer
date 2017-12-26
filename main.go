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
   "fmt"
   gws "github.com/gorilla/websocket-1.2.0"
   "net/http"
   "io"
   "encoding/json"
   "mime/multipart"
   "net"
   "os"
   "mnm-hammer/slib"
   "strconv"
   "strings"
   "sync"
   "html/template"
   "time"
   "net/url"
)

const kVersionA, kVersionB, kVersionC = 0, 0, 0
const kVersionDate = "(unreleased)"
const kIdleTimeFraction = 10
const kMsgHeaderMinLen = int64(len(`{"op":1}`))
const kMsgHeaderMaxLen = int64(1 << 16)

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


func main() { os.Exit(mainResult()) }

func mainResult() int {
   // return 2 reserved for use by Go internals
   var err error

   fmt.Printf("mnm-hammer tmtp client v%d.%d.%d %s\n", kVersionA, kVersionB, kVersionC, kVersionDate)

   if len(os.Args) == 2 {
      sHttpSrvr.Addr = os.Args[1]
   }

   slib.Init(startService)
   slib.Test()

   sServiceTmpl, err = template.ParseFiles("web/service.html")
   if err != nil {
      fmt.Fprintf(os.Stderr, "template parse error %s\n", err.Error())
      return 1
   }

   for _, aName := range slib.GetIdxService() {
      startService(aName)
   }

   http.HandleFunc("/"  , runService)
   http.HandleFunc("/t/", runPost)
   http.HandleFunc("/f/", runPost)
   http.HandleFunc("/s/", runWs)
   http.HandleFunc("/web/", runFile)
   err = sHttpSrvr.ListenAndServe()
   fmt.Fprintf(os.Stderr, "http server stopped %s\n", err.Error())

   return 0
}

type tService struct {
   queue *tQueue
   ccs *tClientConns
}

func startService(iSvc string) {
   sServicesDoor.Lock(); defer sServicesDoor.Unlock()
   if sServices[iSvc].queue != nil {
      panic(fmt.Sprintf("startService %s: already started", iSvc))
   }
   sServices[iSvc] = tService{queue: newQueue(iSvc), ccs: newClientConns()}
   go runLink(iSvc)
}

func getService(iSvc string) tService {
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   return sServices[iSvc]
}

func toAllClients(iMsg slib.Msg) {
   aJson, err := json.Marshal(iMsg)
   if err != nil { panic(err) }
   sServicesDoor.RLock(); defer sServicesDoor.RUnlock()
   for _, aV := range sServices {
      aV.ccs.Range(func(cC *tWsConn) {
         cC.WriteMessage(gws.TextMessage, aJson)
      })
   }
}

type tQueue struct {
   sync.Once // to start runQueue
   once func() // input to .Do()
   service string // service name
   connSrc chan net.Conn // synchronize writes to server
   in chan *slib.SendRecord // message queue input
   out chan *slib.SendRecord // message queue output
   buf []*slib.SendRecord // message queue
   ack chan string // ack queue
   wakeup chan bool // reconnect a periodic service
}

func newQueue(iSvc string) *tQueue {
   aRecs, err := slib.GetQueueService(iSvc)
   if err != nil {
      fmt.Fprintf(os.Stderr, "newqueue %s failure: %s\n", iSvc, err.Error())
      return nil
   }
   var aQ *tQueue
   aQ = &tQueue{
      once: func(){ go runElasticChan(aQ); go runQueue(aQ) },
      service: iSvc,
      connSrc: make(chan net.Conn, 1),
      in: make(chan *slib.SendRecord),
      out: make(chan *slib.SendRecord),
      buf: aRecs,
      ack: make(chan string, 2), //todo larger buffer?
      wakeup: make(chan bool),
   }
   if len(aRecs) > 0 {
      aQ.Do(aQ.once)
   }
   return aQ
}

func (o *tQueue) postMsg(iRec *slib.SendRecord) {
   o.Do(o.once)
   o.in <- iRec
}

func (o *tQueue) postAck(iId string) {
   aMsg := slib.Msg{"Op":eOpAck, "Id":iId, "Type":"ok"}
   aConn := <-o.connSrc
   _, err := aConn.Write(packMsg(tMsg(aMsg), nil))
   if err != nil { panic(err) }
   o.connSrc <- aConn
}

func runQueue(o *tQueue) {
   aSrec := <-o.out
   for {
      var aConn net.Conn
      select {
      case aConn = <-o.connSrc:
      case o.wakeup <- true:
         aConn = <-o.connSrc
      }
      err := aSrec.Write(aConn, o.service)
      o.connSrc <- aConn
      if err != nil { //todo retry transient error
         fmt.Fprintf(os.Stderr, "runQueue %s: send error %s\n", o.service, err.Error())
         time.Sleep(5 * time.Millisecond)
         continue
      }
      aTmr := time.NewTimer(15 * time.Second)
   WaitForAck:
      select {
      case aMsgId := <-o.ack:
         if aMsgId != aSrec.Id() {
            fmt.Fprintf(os.Stderr, "runqueue %s: got ack for %s, expected %s\n",
                        o.service, aMsgId, aSrec.Id())
            goto WaitForAck
         }
         aTmr.Stop()
         aSrec = <-o.out
      case <-aTmr.C:
         fmt.Fprintf(os.Stderr, "runqueue %s: timeout awaiting ack\n", o.service)
      }
   }
}

func runElasticChan(o *tQueue) {
   var aS *slib.SendRecord
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

func runLink(iName string) {
   for {
      aSvc := slib.GetDataService(iName)

      if aSvc.LoginPeriod > 0 && aSvc.Uid != "" {
         // add +/- 0-20% to aSvc.LoginPeriod
         aPercent := aSvc.LoginPeriod / 5
         aRand := time.Now().Nanosecond() % (aPercent * 2 + 1) - aPercent
         aTmr := time.NewTimer(time.Duration(aSvc.LoginPeriod + aRand) * time.Second)
         select {
         case <-aTmr.C:
         case <-getService(iName).queue.wakeup:
            aTmr.Stop()
         }
      }

      aConn, err := net.Dial("tcp", aSvc.Addr)
      if err != nil {
         fmt.Fprintf(os.Stderr, "runLink %s: %s\n", iName, err.Error())
         return //todo fix transient error
      }

      aMsg := tMsg{"Op":eOpTmtpRev, "Id":"1"}
      aConn.Write(packMsg(aMsg, nil))
      if aSvc.Uid == "" {
         aMsg = tMsg{"Op":eOpRegister, "NewAlias":aSvc.Alias, "NewNode":"x"}
      } else {
         aMsg = tMsg{"Op":eOpLogin, "Uid":aSvc.Uid, "Node":aSvc.Node}
      }
      aConn.Write(packMsg(aMsg, nil))

      _readLink(iName, aConn, time.Duration(aSvc.LoginPeriod / kIdleTimeFraction) * time.Second)
      aConn.Close()
   }
}

func _readLink(iName string, iConn net.Conn, iIdleMax time.Duration) {
   aQ := getService(iName).queue
   aBuf := make([]byte, kMsgHeaderMaxLen+4) //todo start smaller, realloc as needed
   var aPos, aHeadEnd, aHeadStart int64 = 0, 0, 4

   for {
      if iIdleMax > 0 {
         iConn.SetReadDeadline(time.Now().Add(iIdleMax))
      }
      aLen, err := iConn.Read(aBuf[aPos:])
      if err != nil {
         //todo if recoverable continue
         if err == io.EOF {
            fmt.Fprintf(os.Stderr, "_readLink %s: server close\n", iName)
            break
         } else if err.(net.Error).Timeout() {
            select {
            case <-aQ.connSrc:
               // if runQueue is awaiting ack, we will miss it and retry
               fmt.Printf("_readLink %s: idle timeout\n", iName)
               return
            default:
               aQ.connSrc <- <-aQ.connSrc // wait for send to finish
               iIdleMax += 15 * time.Second // allow time for ack
               continue
            }
         } else {
            fmt.Fprintf(os.Stderr, "_readLink %s: %s\n", iName, err.Error())
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
            fmt.Fprintf(os.Stderr, "_readLink %s: invalid header length\n", iName)
            break
         }
      }
      if aHeadEnd > aPos {
         continue
      }
      var aHead *slib.Header
      if aHeadStart == 4 {
         aHead = &slib.Header{Op:""}
         err = json.Unmarshal(aBuf[4:aHeadEnd], aHead)
         if err != nil || !aHead.Check() {
            fmt.Fprintf(os.Stderr, "_readLink %s: invalid header\n", iName)
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
            fmt.Fprintf(os.Stderr, "_readLink %s: invalid header\n", iName)
            break
         }
      }
      aData := aBuf[aHeadEnd:aHeadEnd] // ref aBuf even if DataLen==0
      if aPos > aHeadEnd && aHead.DataLen > 0 {
         aEnd := aHeadEnd + aHead.DataLen; if aPos < aEnd { aEnd = aPos }
         aData = aBuf[aHeadEnd:aEnd]
      }
      if aHead.Info == "login ok" {
         aQ.connSrc <- iConn
      } else {
         if aHead.Op == "ack" && aHead.Error == "" {
            select {
            case aQ.ack <- aHead.Id:
            default:
               fmt.Fprintf(os.Stderr, "_readLink %s: ack channel blocked\n", iName)
            }
         }
         aMsg, aFn := slib.HandleTmtpService(iName, aHead, &tTmtpInput{aData, iConn})
         if aMsg == nil {
            break
         }
         if aHead.From != "" {
            aQ.postAck(aHead.Id)
         }
         aJson, _ := json.Marshal(aMsg)
         getService(iName).ccs.Range(func(cC *tWsConn) {
            cC.WriteMessage(gws.TextMessage, aJson)
            if aFn != nil { aFn(cC.state) }
         })
      }
      if aPos > aHeadEnd + aHead.DataLen {
         aPos = int64(copy(aBuf, aBuf[aHeadEnd + aHead.DataLen : aPos]))
         aHeadEnd, aHeadStart = 0, 4
         goto Parse
      }
      aPos, aHeadEnd, aHeadStart = 0, 0, 4
   }
   <-aQ.connSrc
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
   var err error
   // url is "/service[?op]"
   aSvc := iReq.URL.Path[1:]; if aSvc == "" { aSvc = "local" }
   aClientId, _ := iReq.Cookie("clientid")

   if slib.GetDataService(aSvc) == nil {
      iResp.WriteHeader(http.StatusNotFound)
      iResp.Write([]byte("service not found: "+aSvc))
      return
   }

   var aState *slib.ClientState
   if iReq.URL.RawQuery != "" {
      //for getService(aSvc).ccs.Get(aClientId.Value) == nil {
      //   fmt.Printf("nsvc %s op %s id %s\n", aSvc, iReq.URL.RawQuery, aClientId.Value)
      //   time.Sleep(1 * time.Millisecond)
      //}
      aState = getService(aSvc).ccs.Get(aClientId.Value).state
   }
   aQuery, err := url.QueryUnescape(iReq.URL.RawQuery)
   if err != nil { aQuery = "query_error" }
   aOp_Id := strings.SplitN(aQuery, "=", 2)

   switch aOp_Id[0] {
   case "": // service template
      if aClientId == nil {
         aClientId = &http.Cookie{Name: "clientid", Value: fmt.Sprint(time.Now().UTC().UnixNano())}
         http.SetCookie(iResp, aClientId)
      }
      err = sServiceTmpl.Execute(iResp, tMsg{"Title":aSvc, "Addr":sHttpSrvr.Addr})
   case "c": // client state
      aMsg := aState.GetSummary()
      err = json.NewEncoder(iResp).Encode(aMsg)
   case "s": // service list
      aSvcs := slib.GetIdxService()
      err = json.NewEncoder(iResp).Encode(aSvcs)
   case "ps": // saved pings
      aList := slib.GetSavedAdrsbk(aSvc)
      err = json.NewEncoder(iResp).Encode(aList)
   case "pt": // sent pings
      aList := slib.GetSentAdrsbk(aSvc)
      err = json.NewEncoder(iResp).Encode(aList)
   case "pf": // received pings
      aList := slib.GetReceivedAdrsbk(aSvc)
      err = json.NewEncoder(iResp).Encode(aList)
   case "t": // thread list
      _, err = iResp.Write([]byte("threads "+aSvc))
   case "a": // attachment list
      if len(aOp_Id) > 1 {
         http.ServeFile(iResp, iReq, slib.GetPathAttach(aSvc, aState, aOp_Id[1]))
      } else {
         aIdx := slib.GetIdxAttach(aSvc, aState)
         err = json.NewEncoder(iResp).Encode(aIdx)
      }
   case "m": // msg list
      aIdx := slib.GetIdxThread(aSvc, aState)
      err = json.NewEncoder(iResp).Encode(aIdx)
   case "o": // open msgs
      err = slib.WriteMessagesThread(iResp, aSvc, aState, "")
   case "p": // open single msg
      if len(aOp_Id) < 2 { break }
      err = slib.WriteMessagesThread(iResp, aSvc, aState, aOp_Id[1])
   case "form":
      http.ServeFile(iResp, iReq, slib.GetPathFilledForm(aSvc, aOp_Id[1]))
   default:
      iResp.WriteHeader(http.StatusNotFound)
      iResp.Write([]byte("unknown op " + aOp_Id[0]))
   }
   fmt.Printf("svc %s op %s id %s\n", aSvc, aOp_Id[0], aClientId.Value)
   if err != nil {
      fmt.Fprintf(os.Stderr, "runService %s: op %s error %s\n", aSvc, aOp_Id[0], err.Error())
   }
}

type tPostSet struct {
   add func(string, string, io.Reader) error
   drop func(string) bool
   updt func() slib.Msg
   list func() []interface{}
   path func(string) string
}

var sUpload = tPostSet{
   add: slib.AddUpload,
   drop: slib.DropUpload,
   updt: slib.MakeMsgUpload,
   list: slib.GetIdxUpload,
   path: slib.GetPathUpload,
}

var sForm = tPostSet{
   add: slib.AddBlankForm,
   drop: slib.DropBlankForm,
   updt: slib.MakeMsgBlankForm,
   list: slib.GetIdxBlankForm,
   path: slib.GetPathBlankForm,
}

func runPost(iResp http.ResponseWriter, iReq *http.Request) {
   aSet := sUpload; if iReq.URL.Path[1] == 'f' { aSet = sForm }
   aId := iReq.URL.Path[3:]
   if iReq.Method == "POST" {
      fErr := func(cSt int, cMsg string) { iResp.WriteHeader(cSt); iResp.Write([]byte(cMsg)) }
      aStatus := "ok"
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
         err = aSet.add(aId[1:], "", aPart)
         if err != nil {
            fErr(http.StatusInternalServerError, "upload error: " + err.Error())
            return
         }
      } else if aId[0] == '*' {
         var err error
         aPrev_New := strings.SplitN(aId[1:], "+", 2)
         if len(aPrev_New) < 2 || aPrev_New[1] == "" {
            err = tError("missing + param")
         } else {
            err = aSet.add(aPrev_New[0], aPrev_New[1], nil)
         }
         if err != nil {
            fErr(http.StatusNotAcceptable, "duplicate error: " + err.Error())
            return
         }
      } else if aId[0] == '-' {
         if !aSet.drop(aId[1:]) {
            aStatus = "not found"
         }
      } else {
         fErr(http.StatusNotAcceptable, "missing +/- operator")
         return
      }
      toAllClients(aSet.updt())
      iResp.Write(packMsg(tMsg{"op:":"ack", "id":aId, "status":aStatus}, nil))
   } else if aId == "" {
      aIdx := aSet.list()
      err := json.NewEncoder(iResp).Encode(aIdx)
      if err != nil { fmt.Fprintf(os.Stderr, "runPost: %s\n", err.Error()) }
   } else {
      http.ServeFile(iResp, iReq, aSet.path(aId))
   }
}

var sWsInit = gws.Upgrader {
  ReadBufferSize:  1024,
  WriteBufferSize: 1024,
}

func runWs(iResp http.ResponseWriter, iReq *http.Request) {
   aSvc := iReq.URL.Path[3:]; if aSvc == "" { aSvc = "local" }
   if slib.GetDataService(aSvc) == nil {
      iResp.WriteHeader(http.StatusNotFound)
      iResp.Write([]byte("service not found: "+aSvc))
      return
   }

   var aState *slib.ClientState
   aClientId, _ := iReq.Cookie("clientid")
   aClients := getService(aSvc).ccs
   aCc := aClients.Get(aClientId.Value)
   if aCc != nil {
      aCc.WriteMessage(gws.TextMessage, []byte("new connection from same client"))
      aCc.conn.Close()
      aState = aCc.state
   } else {
      aState = slib.OpenState(aClientId.Value, aSvc)
   }
   aSock, err := sWsInit.Upgrade(iResp, iReq, nil)
   if err != nil { panic(err) }
   aClients.Set(aClientId.Value, &tWsConn{conn: aSock, state: aState})

   aQ := getService(aSvc).queue
   for {
      _, aJson, err := aSock.ReadMessage()
      if err != nil {
         fmt.Fprintf(os.Stderr, "runws %s: readmsg: %s\n", aSvc, err.Error())
         if strings.HasSuffix(err.Error(), "use of closed network connection") {
            return // don't .Drop(aClientId.Value)
         }
         break
      }
      fmt.Printf("runws %s: msg %s\n", aSvc, string(aJson))

      var aUpdate slib.Update
      err = json.Unmarshal(aJson, &aUpdate)
      if err != nil { panic(err) }
      aCmsg, aSrec, aFn := slib.HandleUpdtService(aSvc, aState, &aUpdate)

      aJson, err = json.Marshal(aCmsg)
      if err != nil { panic(err) }
      aClients.Range(func(cC *tWsConn) {
         cC.WriteMessage(gws.TextMessage, aJson)
         if aFn != nil { aFn(cC.state) }
      })
      if aSrec != nil {
         aQ.postMsg(aSrec)
      }
   }
   aClients.Drop(aClientId.Value)
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
   sync.Mutex // protect conn.WriteMessage()
   conn *gws.Conn
   state *slib.ClientState
}

func (o *tWsConn) WriteMessage(iT int, iB []byte) {
   o.Lock(); defer o.Unlock()
   err := o.conn.WriteMessage(iT, iB)
   if err != nil {
      fmt.Fprintf(os.Stderr, "WriteMessage: %s\n", err.Error())
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

type tError string
func (o tError) Error() string { return string(o) }

