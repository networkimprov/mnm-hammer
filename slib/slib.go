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
   "strings"
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

func svcDir   (iSvc string) string { return kServiceDir + iSvc + "/"        }
func tempDir  (iSvc string) string { return kServiceDir + iSvc + "/temp/"   }
func threadDir(iSvc string) string { return kServiceDir + iSvc + "/thread/" }
func attachDir(iSvc string) string { return kServiceDir + iSvc + "/attach/" }
func formDir  (iSvc string) string { return kServiceDir + iSvc + "/form/"   }
func cfgFile  (iSvc string) string { return kServiceDir + iSvc + "/config"  }
func pingFile (iSvc string) string { return kServiceDir + iSvc + "/ping-draft" }
func adrsFile (iSvc string) string { return kServiceDir + iSvc + "/adrsbk"  }
func ohiFile  (iSvc string) string { return kServiceDir + iSvc + "/ohi"     }
func tabFile  (iSvc string) string { return kServiceDir + iSvc + "/tabs"    }
func sendqFile(iSvc string) string { return kServiceDir + iSvc + "/sendq"   }

func attachSub(iSvc, iSub string) string { return attachDir(iSvc) + iSub + "/" }
func attachFfn(iSvc, iSub string) string { return attachSub(iSvc, iSub) + "ffnindex" }

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
   cfg tCfgService
   sendQ []tQueueEl
   fromOhi tOhi
   tabs []string
   doors map[string]tDoor // shared by *Thread & *FilledForm
   // ohiFile(svc), not cached
}

type tDoor interface {
   Lock(); Unlock()
   RLock(); RUnlock()
}

type Header struct {
   Op string
   Error string
   Id, MsgId string
   Uid, NodeId string
   Info string
   Ohi []string
   From string
   Posted string
   To string
   Gid string
   Alias string
   Act string
   Status int8
   DataLen, DataHead int64
   SubHead tHeader2
}

type tHeader2 struct {
   Alias string
   Cc []string
   ThreadId string
   Subject string
   Attach []tHeader2Attach `json:",omitempty"`
   For []tHeaderFor `json:",omitempty"` // copied to outgoing Header.For
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
   o.Cc = i.Thread.Cc
   o.For = lookupAdrsbk(iSvc, o.Cc)
   o.Subject = i.Thread.Subject
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
   Thread *struct {
      Id string
      Alias string
      Cc []string
      Subject string
      Data string
      Attach []tHeader2Attach
      FormFill map[string]string
      New int8
      ThreadId string //todo move to new struct
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
   Ohi *struct {
      Alias string
      Uid string
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
   Service *tCfgService
   Test *struct {
      Poll time.Duration // for use by test.go
      Request []string
   }
}

type SendRecord struct {
   Id string // Id[0] is one of eSrec*
}

const eSrecThread, eSrecPing, eSrecOhi, eSrecAccept byte = 't', 'p', 'o', 'a'

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

func sendHeaders(iW io.Writer, iHead, iSub []byte) error {
   var err error
   aLen := []byte(fmt.Sprintf("%04x", len(iHead)))
   if len(aLen) > 4 { quit(tError("header too long")) }
   _, err = iW.Write(aLen)
   if err != nil { return err }
   _, err = iW.Write(iHead)
   if iSub != nil {
      if err != nil { return err }
      _, err = iW.Write(iSub)
   }
   //fmt.Printf("sendHeaders: %s%s%s\n", aLen, iHead, iSub)
   return err
}

func makeLocalId(iTid string) string {
   return fmt.Sprintf("%s_%012x", iTid, time.Now().UTC().UnixNano() / 1e6) // milliseconds
}

func parseLocalId(i string) tLocalId { return strings.SplitN(i, "_", 2) }

type tLocalId []string

func (o tLocalId) tidSet(i string) { o[0] = i }
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
