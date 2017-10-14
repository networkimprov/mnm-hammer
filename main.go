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
   "net"
   "os"
   "mnm-hammer/slib"
   "strconv"
   "strings"
   "sync"
   "html/template"
   "time"
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

   slib.Init(startService)
   slib.Test()

   sServiceTmpl, err = template.ParseFiles("web/service.html")
   if err != nil {
      fmt.Fprintf(os.Stderr, "template parse error %s\n", err.Error())
      return 1
   }

   for _, aName := range slib.GetServices() {
      startService(aName)
   }

   http.HandleFunc("/"  , runService)
   http.HandleFunc("/t/", runUpload)
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
   aRecs, err := slib.GetQueue(iSvc)
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
      _, err := aConn.Write(packMsg(tMsg(aSrec.Head), aSrec.Data))
      if err != nil { panic(err) }
      for _, aFn := range aSrec.Files {
         aFd, err := os.Open(aFn)
         if err != nil { panic(err) }
         _, err = io.Copy(aConn, aFd)
         if err != nil { panic(err) }
         aFd.Close()
      }
      o.connSrc <- aConn
      aTmr := time.NewTimer(15 * time.Second)
   WaitForAck:
      select {
      case aMsgId := <-o.ack:
         if aMsgId != aSrec.Head["Id"] {
            fmt.Fprintf(os.Stderr, "runqueue %s: got ack for %s, expected %s\n", o.service, aMsgId, aSrec.Head["Id"])
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
      aSvc := slib.GetData(iName)

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
         fmt.Fprintf(os.Stderr, "runservice %s: %s\n", iName, err.Error())
         return //todo fix transient error
      }

      aMsg := tMsg{"Op":eOpTmtpRev, "Id":"1"}
      aConn.Write(packMsg(aMsg, nil))
      if aSvc.Uid == "" {
         aMsg = tMsg{"Op":eOpRegister, "NewAlias":"_", "NewNode":"x"}
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
            fmt.Fprintf(os.Stderr, "runservice %s: server close\n", iName)
            break
         } else if err.(net.Error).Timeout() {
            select {
            case <-aQ.connSrc:
               // if runQueue is awaiting ack, we will miss it and retry
               fmt.Printf("runservice %s: idle timeout\n", iName)
               return
            default:
               aQ.connSrc <- <-aQ.connSrc // wait for send to finish
               iIdleMax += 15 * time.Second // allow time for ack
               continue
            }
         } else {
            fmt.Fprintf(os.Stderr, "runservice %s: net error %s\n", iName, err.Error())
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
            fmt.Fprintf(os.Stderr, "runservice %s: invalid header length\n", iName)
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
            fmt.Fprintf(os.Stderr, "runservice %s: invalid header\n", iName)
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
            fmt.Fprintf(os.Stderr, "runservice %s: invalid header\n", iName)
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
         if aHead.Op == "ack" {
            select {
            case aQ.ack <- aHead.Id:
            default:
               fmt.Fprintf(os.Stderr, "runservice %s: ack channel blocked\n", iName)
            }
         }
         aMsg := slib.HandleMsg(iName, aHead, aData, iConn)
         if aMsg == nil {
            break
         }
         if aHead.From != "" {
            aQ.postAck(aHead.Id)
         }
         aJson, _ := json.Marshal(aMsg)
         getService(iName).ccs.Range(func(cC *tWsConn) {
            cC.WriteMessage(gws.TextMessage, aJson)
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

func runService(iResp http.ResponseWriter, iReq *http.Request) {
   var err error
   // url is "/service[?op]"
   aSvc := iReq.URL.Path[1:]; if aSvc == "" { aSvc = "local" }
   aClientId, _ := iReq.Cookie("clientid")

   if slib.GetData(aSvc) == nil {
      iResp.WriteHeader(http.StatusNotFound)
      iResp.Write([]byte("service not found: "+aSvc))
      return
   }

   aOp_Id := strings.SplitN(iReq.URL.RawQuery, "=", 2)
   switch aOp_Id[0] {
   case "": // service template
      if aClientId == nil {
         aClientId = &http.Cookie{Name: "clientid", Value: fmt.Sprint(time.Now().UTC().UnixNano())}
         http.SetCookie(iResp, aClientId)
      }
      err := sServiceTmpl.Execute(iResp, tMsg{"Title":aSvc})
      if err != nil {
         iResp.WriteHeader(http.StatusInternalServerError)
         iResp.Write([]byte("error sending template: "+err.Error()))
      }
   case "s": // service list
      aSvcs := slib.GetServices()
      err = json.NewEncoder(iResp).Encode(aSvcs)
      if err != nil { panic(err) }
   case "t": // thread list
      iResp.Write([]byte("threads "+aSvc))
   case "m": // msg list
      aIdx := slib.GetMsgIdx(aSvc, aClientId.Value)
      err = json.NewEncoder(iResp).Encode(aIdx)
      if err != nil { panic(err) }
   case "o": // open msgs
      slib.WriteOpenMsgs(iResp, aSvc, aClientId.Value, "")
   case "p": // open single msg
      if len(aOp_Id) < 2 { break }
      slib.WriteOpenMsgs(iResp, aSvc, aClientId.Value, aOp_Id[1])
   default:
      iResp.WriteHeader(http.StatusNotFound)
      iResp.Write([]byte("unknown op " + aOp_Id[0]))
   }
   fmt.Printf("svc %s op %s id %s\n", aSvc, aOp_Id[0], aClientId.Value)
}

func runUpload(iResp http.ResponseWriter, iReq *http.Request) {
   aId := iReq.URL.Path[3:]
   if aId == "" {
      iResp.WriteHeader(http.StatusNotAcceptable)
      iResp.Write([]byte("requires /t/temp_id"))
      return
   }
   if iReq.Method == "POST" {
      aF, aHead, err := iReq.FormFile("filename")
      if err != nil {
         iResp.WriteHeader(http.StatusNotAcceptable)
         iResp.Write([]byte("formfile error: " + err.Error()))
         return
      }
      defer aF.Close()
      err = slib.Upload(aId, aF, aHead.Size)
      if err != nil {
         iResp.WriteHeader(http.StatusInternalServerError)
         iResp.Write([]byte("recvFile error: " + err.Error()))
         return
      }
      iResp.Write(packMsg(tMsg{"op:":"ack", "id":aId}, nil))
   } else {
      http.ServeFile(iResp, iReq, slib.UploadDir + aId)
   }
}

var sWsInit = gws.Upgrader {
  ReadBufferSize:  1024,
  WriteBufferSize: 1024,
}

func runWs(iResp http.ResponseWriter, iReq *http.Request) {
   aSvc := iReq.URL.Path[3:]; if aSvc == "" { aSvc = "local" }
   if slib.GetData(aSvc) == nil {
      iResp.WriteHeader(http.StatusNotFound)
      iResp.Write([]byte("service not found: "+aSvc))
      return
   }

   aClientId, _ := iReq.Cookie("clientid")
   aSock, err := sWsInit.Upgrade(iResp, iReq, nil)
   if err != nil { panic(err) }
   aClients := getService(aSvc).ccs
   aCc := aClients.Get(aClientId.Value)
   if aCc != nil {
      aCc.WriteMessage(gws.TextMessage, []byte("new connection from same client"))
      aCc.conn.Close()
   }
   aClients.Set(aClientId.Value, &tWsConn{conn: aSock})

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
      aCmsg, aSrec := slib.HandleUpdt(aSvc, &aUpdate)

      aJson, err = json.Marshal(aCmsg)
      if err != nil { panic(err) }
      aClients.Range(func(cC *tWsConn) {
         err = cC.WriteMessage(gws.TextMessage, aJson)
         if err != nil {
            fmt.Fprintf(os.Stderr, "runws %s: writemsg: %s\n", aSvc, err.Error())
         }
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
}

func (o *tWsConn) WriteMessage(iT int, iB []byte) error {
   o.Lock(); defer o.Unlock()
   return o.conn.WriteMessage(iT, iB)
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

