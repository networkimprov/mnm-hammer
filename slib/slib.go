// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package slib

import (
   "runtime/debug"
   "hash/crc32"
   "fmt"
   "io"
   "encoding/json"
   "os"
   "path"
   "sync"
   "time"
)

const kStorageDir = "store/"
const kServiceDir = kStorageDir + "svc/"
const kStateDir   = kStorageDir + "state/"
const kUploadDir  = kStorageDir + "upload/"
const kUploadTmp  = kUploadDir  + "temp/"
const kFormDir    = kStorageDir + "form/"
const kFormRegDir = kStorageDir + "reg-cache/"

func dirSvc   (iSvc string) string { return kServiceDir + iSvc + "/" }
func dirTemp  (iSvc string) string { return kServiceDir + iSvc + "/temp/" }
func dirThread(iSvc string) string { return kServiceDir + iSvc + "/thread/" }
func dirAttach(iSvc string) string { return kServiceDir + iSvc + "/attach/" }
func dirForm  (iSvc string) string { return kServiceDir + iSvc + "/form/" }

func fileCfg  (iSvc string) string { return kServiceDir + iSvc + "/config" }
func filePing (iSvc string) string { return kServiceDir + iSvc + "/ping-draft" }
func fileAdrs (iSvc string) string { return kServiceDir + iSvc + "/adrsbk" }
func fileOhi  (iSvc string) string { return kServiceDir + iSvc + "/ohi" }
func fileTab  (iSvc string) string { return kServiceDir + iSvc + "/tabs" }
func fileSendq(iSvc string) string { return kServiceDir + iSvc + "/sendq" }
func fileNotc (iSvc string) string { return kServiceDir + iSvc + "/notice" }

func fileFwd  (iSvc, iTid string) string { return dirThread(iSvc) + iTid + "_forward" }

func subAttach(iSvc, iSub string) string { return dirAttach(iSvc) + iSub + "/" }
func fileFfn  (iSvc, iSub string) string { return subAttach(iSvc, iSub) + "ffnindex" }

// these have either ".tmp" or a decimal string appended
func ftmpSr(iSvc, iTid, iMid string) string { return dirTemp(iSvc) + iTid +"_"+ iMid +"_sr__" }
func ftmpSc(iSvc, iTid, iMid string) string { return dirTemp(iSvc) + iTid +"_"+ iMid +"_sc__" }
func ftmpNr(iSvc, iTid       string) string { return dirTemp(iSvc) + iTid +"__nr__" }
func ftmpSs(iSvc, iTid, iMid,
                        iLms string) string { return dirTemp(iSvc) + iTid +"_"+ iMid +"_ss_"+ iLms +"_" }
func ftmpSd(iSvc, iTid, iLms string) string { return dirTemp(iSvc) + iTid +"__ws_"+ iLms +"_" }
func ftmpDd(iSvc, iTid, iLms string) string { return dirTemp(iSvc) + iTid +"__ds_"+ iLms +"_" }
func ftmpFr(iSvc, iTid       string) string { return dirTemp(iSvc) + iTid +"_"+ iTid +"_fr__" }
func ftmpFn(iSvc, iTid       string) string { return dirTemp(iSvc) + iTid +"__fn__" }
func ftmpFs(iSvc, iTid, iLms string) string { return dirTemp(iSvc) + iTid +"__fs_"+ iLms +"_" }

func ftmpFwdS(iSvc, iTid string) string { return dirTemp(iSvc) + iTid +"_fwd.tmp" }
func ftmpFwdD(iSvc, iTid string) string { return dirTemp(iSvc) +"forward_"+ iTid }

func ftmpAttach(iSvc, iMid, iFile string) string { return dirTemp(iSvc) + iMid +"_"+ iFile +"_atc.tmp" }

func ftmpFfn   (iSvc, iTid string) string { return dirTemp(iSvc) +"ffnindex_"+ iTid }
func ftmpAdrsbk(iSvc, iPos string) string { return dirTemp(iSvc) +"adrsbk_"+   iPos }

var sCrc32c = crc32.MakeTable(crc32.Castagnoli)

type GlobalSet interface {
   Add(string, string, io.Reader) error
   Drop(string) error
   GetIdx() interface{}
   GetPath(string) string
}

type tService struct {
   adrsbk tAdrsbk
   sync.RWMutex // protects the following
   config tSvcConfig
   sendQ []tQueueEl
   sendQPost func(...*SendRecord)
   notice []tNoticeEl
   fromOhi tOhi
   tabs []string
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
   Id, MsgId, PostId string
   Uid, NodeId string
   Node string
   Info string
   Ohi []string
   From string
   Posted string
   To string
   Gid string
   Alias string
   Act string
   Status int8
   Notify uint16
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
   noAttachSize bool
}

type tHeader2Attach struct {
   Name string
   Size int64 `json:",omitempty"`
   Ffn string `json:",omitempty"`
   FfKey string `json:",omitempty"` // only in draft
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
   Op string
   Config *struct {
      Addr string
      LoginPeriod int
      Verify bool
   }
   Thread *struct {
      Id string
      Alias string
      Cc []tCcEl
      Subject string
      Data string
      Attach []tHeader2Attach
      FormFill map[string]string
      New int8
      ThreadId string //todo move to new struct
   }
   Forward *struct {
      ThreadId string
      Cc []tCcEl
      Qid string
   }
   Ping *struct {
      Alias string
      To string
      Text string
      Gid string
      Qid string
   }
   Accept *struct {
      Qid string
   }
   Adrsbk *struct {
      Type int8
      Term string
   }
   Ohi *struct {
      Alias string
      Uid string
   }
   Notice *struct {
      MsgId string
   }
   Navigate *struct {
      History int
      ThreadId, MsgId string
   }
   Tab *struct {
      Type int8
      Term string
      PosFor int8
      Pos int
   }
   Test *struct {
      Poll time.Duration // for use by test.go
      Request []string
      ThreadId string
      Notice []tNoticeEl
   }
}

type SendRecord struct {
   Id string // Id[0] is one of eSrec*
}

const (
   eSrecThread = 't'; eSrecFwd = 'f'; eSrecCfm = 'c'
   eSrecPing = 'p'; eSrecOhi = 'o'; eSrecAccept = 'a'
)

type Msg map[string]interface{}


func Init(iFn func(string)) {
   for _, aDir := range [...]string{kUploadTmp, kServiceDir, kStateDir, kFormDir} {
      err := os.MkdirAll(aDir, 0700)
      if err != nil { quit(err) }
   }
   initUpload()
   initForms()
   initStates()
   initServices(iFn)
   startAllService()
}

// utilities follow

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
   return fmt.Sprintf("%s_%012x", iTid, time.Now().UnixNano() / 1e6) // milliseconds
}

type tLocalId []string

func parseLocalId(i string) tLocalId { return tLocalId{i[:len(i)-13], i[len(i)-12:]} }

func (o tLocalId)  ping() string { return o[0] }
func (o tLocalId)   gid() string { return o[0] }
func (o tLocalId)   ohi() string { return o[0] }
func (o tLocalId)   tid() string { return o[0] }
func (o tLocalId)   lms() string { return o[1] }

type tCrcWriter struct { sum uint32 }

func (o *tCrcWriter) Write(i []byte) (int, error) {
   o.sum = crc32.Update(o.sum, sCrc32c, i)
   return len(i), nil
}

func readDirNames(iPath string) ([]string, error) {
   aFd, err := os.Open(iPath)
   if err != nil { return nil, err }
   aList, err := aFd.Readdirnames(0)
   aFd.Close()
   return aList, err
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

func resolveTmpFile(iPath string) error {
   return renameRemove(iPath, iPath[:len(iPath)-4])
}

func renameRemove(iA, iB string) error {
   err := os.Rename(iA, iB)
   if err != nil {
      if os.IsNotExist(err) {
         err = nil
      } else if os.IsExist(err) {
         err = os.Remove(iA)
      }
   }
   return err
}

func syncDir(iPath string) error {
   aFd, err := os.Open(iPath)
   if err != nil { return err }
   err = aFd.Sync()
   aFd.Close()
   return err
}

func dateRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

func quit(err error) {
   fmt.Fprintf(os.Stderr, "quit after %s\n", err.Error())
   debug.PrintStack()
   os.Exit(3)
}

type tError string
func (o tError) Error() string { return string(o) }
