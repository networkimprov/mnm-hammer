// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "sync/atomic"
   "runtime/debug"
   "hash/crc32"
   "fmt"
   "io"
   "io/ioutil"
   "encoding/json"
   "os"
   "path"
   pBleve "github.com/blevesearch/bleve"
   "strings"
   "sync"
   "time"
   "net/url"
)

const kStorageDir = "store/"
const kServiceDir = kStorageDir + "svc/"
const kStateDir   = kStorageDir + "state/"
const kUploadDir  = kStorageDir + "upload/"
const kUploadTmp  = kUploadDir  + "temp/"
const kFormDir    = kStorageDir + "form/"
const kFormRegDir = kStorageDir + "reg-cache/"
const kTempDir    = kStorageDir + "temp/"

func fileState(iCli, iSvc string) string { return kStateDir + iCli +"/"+ escapeFile(iSvc) }

func fileUpload(iFil string) string { return kUploadDir + escapeFile(iFil) }
func fileUptmp (iFil string) string { return kUploadTmp + escapeFile(iFil) }

func fileFormReg(iFfn string) string { return kFormRegDir + escapeFile(iFfn) }

func fileTemp(iFil string) string { return kTempDir + escapeFile(iFil) }

func dirSvc(iSvc string) string { return kServiceDir + escapeFile(iSvc) + "/" }

// node.go uses some of these literals
func dirTemp  (iSvc string) string { return dirSvc(iSvc) + "temp/" }
func dirThread(iSvc string) string { return dirSvc(iSvc) + "thread/" }
func dirAttach(iSvc string) string { return dirSvc(iSvc) + "attach/" }
func dirForm  (iSvc string) string { return dirSvc(iSvc) + "form/" }
func fileCfg  (iSvc string) string { return dirSvc(iSvc) + "config" }
func filePing (iSvc string) string { return dirSvc(iSvc) + "ping-draft" }
func fileAdrs (iSvc string) string { return dirSvc(iSvc) + "adrsbk" }
func fileOhi  (iSvc string) string { return dirSvc(iSvc) + "ohi" }
func fileTag  (iSvc string) string { return dirSvc(iSvc) + "tag" }
func fileTab  (iSvc string) string { return dirSvc(iSvc) + "tabs" }
func fileSendq(iSvc string) string { return dirSvc(iSvc) + "sendq" }
func fileNotc (iSvc string) string { return dirSvc(iSvc) + "notice" }
func fileIndex(iSvc string) string { return dirSvc(iSvc) + "index.bleve" }

func fileDraft(iSvc, iTid, iLms string) string { return dirThread(iSvc) + iTid +"_"+ iLms }
func fileFwd  (iSvc, iTid       string) string { return dirThread(iSvc) + iTid + "_forward" }

func fileAtc(iSvc, iSub, iMid, iFil string) string { return dirAttach(iSvc) + iSub +"/"+
                                                            iMid +"_"+ escapeFile(iFil) }
func fileFfn(iSvc, iSub             string) string { return dirAttach(iSvc) + iSub + "/ffnindex" }

func fileForm(iSvc, iFft string) string { return dirForm(iSvc) + escapeFile(iFft) }

// these have either ".tmp" or a decimal string appended
func ftmpSr(iSvc, iTid, iMid string) string { return dirTemp(iSvc) +"sr_"+ iTid +"_"+ iMid +"__" }
func ftmpSc(iSvc, iTid, iMid string) string { return dirTemp(iSvc) +"sc_"+ iTid +"_"+ iMid +"__" }
func ftmpSs(iSvc, iTid, iMid,
                        iLms string) string { return dirTemp(iSvc) +"ss_"+ iTid +"_"+ iMid +"_"+ iLms +"_" }
func ftmpSd(iSvc, iTid, iLms string) string { return dirTemp(iSvc) +"ws_"+ iTid +"__"+ iLms +"_" }
func ftmpDd(iSvc, iTid, iLms string) string { return dirTemp(iSvc) +"ds_"+ iTid +"__"+ iLms +"_" }
func ftmpFr(iSvc, iTid       string) string { return dirTemp(iSvc) +"fr_"+ iTid +"_"+ iTid +"__" }
func ftmpFn(iSvc, iTid       string) string { return dirTemp(iSvc) +"fn_"+ iTid +"___" }
func ftmpFs(iSvc, iTid, iLms string) string { return dirTemp(iSvc) +"fs_"+ iTid +"__"+ iLms +"_" }
func ftmpTc(iSvc, iTid, iLms string) string { return dirTemp(iSvc) +"nr_"+ iTid +"__"+ iLms +"_" }

func ftmpFwdS(iSvc, iTid string) string { return dirTemp(iSvc) + iTid +"_fwd.tmp" }
func ftmpFwdD(iSvc, iTid string) string { return dirTemp(iSvc) +"forward_"+ iTid }

func ftmpAtc(iSvc, iMid, iFil string) string { return dirTemp(iSvc) +
                                                      iMid +"_"+ escapeFile(iFil) +"_atc.tmp" }

func ftmpFfn   (iSvc, iTid       string) string { return dirTemp(iSvc) +"ffnindex_"+ iTid }
func ftmpAdrsbk(iSvc, iPos, iQid string) string { return dirTemp(iSvc) +"adrsbk_"+ iPos +"_"+
                                                         escapeFile(iQid) }

func ftmpSyncLog (iSvc       string) string { return dirTemp(iSvc) +"synclog" }
func ftmpSyncLogQ(iSvc, iQid string) string { return dirTemp(iSvc) +"synclog_"+ iQid }
// this may have ".tmp" appended
func ftmpSyncAck (iSvc, iId  string) string { return dirTemp(iSvc) +"syncack_"+ iId }
// this has either ".tmp" or a decimal string appended
func ftmpSyncUpdt(iSvc, iCid string) string { return dirTemp(iSvc) +"syncupdt_"+ iCid +"_" }

var kCrc32c = crc32.MakeTable(crc32.Castagnoli)

var sCrashFn func(string, string)
var sLocalId = time.Now().UnixNano() / 1e6 // milliseconds

type GlobalSet interface {
   Add(string, string, io.Reader) error
   Drop(string) error
   GetIdx() interface{}
   GetPath(string) string
}

type tService struct {
   updt sync.RWMutex // protect entire service during node replication
   nodeUpdt sync.Mutex // protect node sync log
   adrsbk tAdrsbk
   index pBleve.Index
   toNode tToNode
   sync.RWMutex // protects the following
   config tSvcConfig
   siteData struct { Name string }
   sendQ []*tQueueEl
   sendQPost func(...*SendRecord)
   notice []tNoticeEl
   fromOhi tOhi
   tabs []tTermEl
   unreadCount int
   doors map[string]tDoor // shared by *Thread & *FilledForm
   // fileOhi(svc), not cached
}

type tDoor interface {
   Lock(); Unlock()
   RLock(); RUnlock()
}

type Header struct {
   Op string
   Error string
   Name string
   Auth byte
   AuthBy [][2]string
   Id, MsgId, PostId string
   Uid, NodeId string
   NewNode, Node string
   Info string
   Ohi []string
   From string
   Posted string
   To string
   Gid string
   NewAlias, Alias string
   Act string
   Status int8
   Notify uint16
   For tForOhi
   Type string
   DataLen, DataHead int64
   SubHead *tHeader2
}

type tHeader2 struct {
   Alias string
   ThreadId string
   Subject string
   Attach []tHeader2Attach `json:",omitempty"`
   Cc []tCcEl `json:",omitempty"`
   ConfirmId string `json:",omitempty"`
   ConfirmPosted string `json:",omitempty"`
   NodeSync bool `json:",omitempty"`
   noAttachSize bool
}

type tHeader2Attach struct {
   Name string
   Size int64 `json:",omitempty"`
   Ffn string `json:",omitempty"`
   FfKey string `json:",omitempty"` // only in draft
   IsNew bool   `json:",omitempty"` // only in draft
   AllowAnyData bool `json:",omitempty"` // for testing
}

func (o *tHeader2) setupDraft(iThreadId string, i *Update, iSvc string) {
   o.ThreadId = iThreadId
   o.Alias = i.Thread.Alias
   o.Subject = i.Thread.Subject
   o.Cc = i.Thread.Cc
   o.Attach = setupDraftAttach(iSvc, iThreadId, i)
   o.noAttachSize = true
}

func (o *tHeader2) setupSent(iThreadId string) {
   o.ThreadId = iThreadId
   o.noAttachSize = true
}

func (o *Header) Check() bool {
   return true
}

func (o *Header) CheckSub() bool {
   return true
}

type tHeaderFor struct { Id string; Type int8 }

const ( _ int8 = iota; eForUser; eForGroupAll; eForGroupExcl; eForSelf )

type Update struct {
   log int8 // values eLog*
   logPos int64
   LogThreadId string `json:",omitempty"`
   LogOp string `json:",omitempty"`
   Op string
   Config *struct {
      HistoryLen int
      Addr string
      Alias string
      LoginPeriod int
   } `json:",omitempty"`
   Thread *struct {
      Id string
      Alias string
      Cc []tCcEl
      Subject string
      Data string
      Attach []tHeader2Attach
      FormFill map[string]string
      New int8
   } `json:",omitempty"`
   Touch *UpdateTouch `json:",omitempty"`
   Forward *struct {
      ThreadId string
      Cc []tCcEl
      Qid string
   } `json:",omitempty"`
   Ping *struct {
      Alias string
      To string
      Text string
      Gid string
      Qid string
   } `json:",omitempty"`
   Accept *struct {
      Qid string
   } `json:",omitempty"`
   Adrsbk *struct {
      Type int8
      Term string
   } `json:",omitempty"`
   Ohi *struct {
      Alias string
      Uid string
   } `json:",omitempty"`
   Notice *struct {
      MsgId string
   } `json:",omitempty"`
   Tag *struct {
      Name string
      Id string `json:",omitempty"`
   } `json:",omitempty"`
   Navigate *struct {
      History int
      Label string
      ThreadId, MsgId string
   } `json:",omitempty"`
   Tab *struct {
      Type int8
      Term string
      PosFor int8
      Pos int
   } `json:",omitempty"`
   Sort *struct {
      Type string
      Field string
   } `json:",omitempty"`
   Node *struct {
      Addr string
      Pin string
      Newnode string
   } `json:",omitempty"`
   Test *UpdateTest `json:",omitempty"`
}

const ( _ int8 = iota; eLogRetry; eLogNone )

type UpdateTouch struct { //todo move into Update when draft sync'd
   ThreadId, MsgId string
   TagId string
   TagName string `json:",omitempty"`
   Act int8
}

type UpdateTest struct {
   Request []string
   Notice []tNoticeEl
}

type SendRecord struct {
   Id string // Id[0] is one of eSrec*
}

const (
   eSrecThread = 't'; eSrecFwd = 'f'; eSrecCfm = 'c'
   eSrecPing = 'p'; eSrecOhi = 'o'; eSrecAccept = 'a'
   eSrecAlias = 'l'; eSrecNode = 'n'; eSrecSync = 's'
)

type Msg map[string]interface{}


func Init(iStart func(string), iMts func(string, *Header), iCrash func(string, string)) {
   sCrashFn = iCrash
   for _, aDir := range [...]string{kUploadTmp, kServiceDir, kStateDir, kFormDir, kFormRegDir, kTempDir} {
      err := os.MkdirAll(aDir, 0700)
      if err != nil { quit(err) }
   }
   initUpload()
   initForms()
   initStates()
   initServices(iStart, iMts)
   startAllService()
}

func GetConstants(iMap map[string]interface{}) map[string]interface{} {
   // keys uncapitalized
   iMap["serviceMin"] = kServiceNameMin
   iMap["aliasMin"] = 8
   iMap["pingTextMax"] = kPingTextMax
   iMap["tabsStdService"] = kTabsStdService
   iMap["tabsStdThread"] = kTabsStdThread
   return iMap
}

// utilities follow

func discardTmtp(iHead *Header, iR io.Reader) error {
   _, err := io.CopyN(ioutil.Discard, iR, iHead.DataLen) // .DataHead already subtracted
   return err
}

func writeHeaders(iW io.Writer, iHead, iSub []byte) error {
   var err error
   aLen := []byte(fmt.Sprintf("%04x", len(iHead)))
   if len(aLen) > 4 { quit(tError("header too long")) }
   _, err = iW.Write(aLen)
   if err != nil { return err }
   _, err = iW.Write(iHead)
   if err == nil && iSub != nil {
      _, err = iW.Write(iSub)
   }
   return err
}

func makeLocalId(iTid string) string {
   return fmt.Sprintf("%s_%012x", iTid, atomic.AddInt64(&sLocalId, 1))
}

type tLocalId []string

func parseLocalId(i string) tLocalId { return tLocalId{i[:len(i)-13], i[len(i)-12:]} }

func (o tLocalId)  ping() string { return o[0] }
func (o tLocalId)   gid() string { return o[0] }
func (o tLocalId)   ohi() string { return o[0] }
func (o tLocalId)   tid() string { return o[0] }
func (o tLocalId)  info() string { return o[0] } //todo replace others with this?
func (o tLocalId)   lms() string { return o[1] }

type tCrcWriter struct { sum uint32 }

func (o *tCrcWriter) Write(i []byte) (int, error) {
   o.sum = crc32.Update(o.sum, kCrc32c, i)
   return len(i), nil
}

func (o *tCrcWriter) clear() { o.sum = 0 }

type tReadCounter struct {
   r io.Reader
   c *int64
}

func (o *tReadCounter) Read(iBuf []byte) (int, error) {
   aLen, err := o.r.Read(iBuf)
   *o.c += int64(aLen)
   return aLen, err
}

func storeFile(iPath string, iData interface{}) error {
   aTemp := iPath + ".tmp"
   defer os.Remove(aTemp)
   err := writeJsonFile(aTemp, iData)
   if err != nil { return err }
   err = syncDir(path.Dir(iPath))
   if err != nil { return err }
   err = os.Remove(iPath)
   if err != nil { return err }
   err = os.Rename(aTemp, iPath)
   if err != nil {
      fmt.Fprintf(os.Stderr, "transaction failed...")
      quit(err)
   }
   return nil
}

func readJsonFile(iObj interface{}, iPath string) error {
   aFd, err := os.Open(iPath)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      return err
   }
   defer aFd.Close()
   err = json.NewDecoder(aFd).Decode(iObj)
   if err != nil && err != io.ErrUnexpectedEOF {
      if _, ok := err.(*json.SyntaxError); !ok { quit(err) }
   }
   return err
}

func writeJsonFile(iPath string, iData interface{}) error {
   aFd, err := os.OpenFile(iPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { return err }
   defer aFd.Close()
   err = json.NewEncoder(aFd).Encode(iData)
   if err != nil { return err }
   err = aFd.Sync()
   return err
}

func writeStreamFile(iPath string, iSrc io.Reader) error {
   aFd, err := os.OpenFile(iPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aFd.Close()
   _, err = io.Copy(aFd, iSrc)
   if err != nil { //todo only return network errors
      os.Remove(iPath)
      return err
   }
   err = aFd.Sync()
   if err != nil { quit(err) }
   return nil
}

func resolveTmpFile(iPath string) error {
   return renameRemove(iPath, iPath[:len(iPath)-4])
}

func renameRemove(iA, iB string) error {
   err := os.Link(iA, iB)
   if err != nil {
      if os.IsNotExist(err) { return nil }
      if !os.IsExist(err) { return err }
   }
   err = os.Remove(iA)
   return err
}

func escapeFile(i string) string {
   if i == "" {
      return i
   } else if i[0] == '.' {
      return "%2E" + url.QueryEscape(i[1:])
   } else if IsReservedFile(i) {
      aEnd := len(i) - 1
      return fmt.Sprintf("%s%%%X", i[:aEnd], i[aEnd])
   }
   return url.QueryEscape(i)
}

func unescapeFile(i string) string {
   i, err := url.QueryUnescape(i)
   if err != nil { quit(err) }
   return i
}

func IsReservedFile(i string) bool {
   if len(i) < 3 || len(i) > 4 {
      return false
   }
   i = strings.ToLower(i)
   if len(i) == 3 {
      return i == "con" || i == "prn" || i == "aux" || i == "nul"
   }
   return (i[:3] == "com" || i[:3] == "lpt") && i[3] >= '1' && i[3] <= '9'
}

func dateRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

func quit(err error) {
   fmt.Fprintf(os.Stderr, "quit after %s\n", err.Error())
   debug.PrintStack()
   os.Exit(3)
}

type tError string
func (o tError) Error() string { return string(o) }
