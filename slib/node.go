// Copyright 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "sync/atomic"
   "fmt"
   "net/http"
   "io"
   "io/ioutil"
   "encoding/json"
   "os"
   "crypto/rand"
   "sort"
   "strconv"
   "strings"
   "archive/tar"
   "time"
   "net/url"
)

const kNodeFlagUpload = ".../"
var kNodeListen = []string{"/l"}
var kNodeStart  = []string{"/l", "/v"}

var sNodeSyncPeriod = time.Duration(2 * time.Minute)
var sNodePin = ""

type tToNode struct {
   Addr string
   Pin string
   Xfer int64
   client *http.Client
   busy int32
}

func (o *tToNode) isLocalhost() bool {
   return len(o.Addr) >= 9 && (o.Addr[:9] == "localhost" || o.Addr[:9] == "127.0.0.1") &&
          (len(o.Addr) == 9 || o.Addr[9] == ':')
}

func (o *tToNode) url(iSvc string) string {
   return "http://"+ o.Addr +"/n/"+ url.PathEscape(iSvc) +"?"+ url.QueryEscape(o.Pin)
}

type tPathInode struct {
   path string
   inode uint64
   size int64
   modtime time.Time
}

type tNodeAddr struct { Addr, Pin string }

func GetPinNode(iAddr string) tNodeAddr {
   aP := sNodePin; if aP != "" { aP = aP[:3] +" "+ aP[3:6] +" "+ aP[6:] }
   return tNodeAddr{iAddr, aP}
}

func ListenNode() []string {
   if sNodePin == "" {
      aBuf := make([]byte, 3)
      _, err := rand.Read(aBuf)
      if err != nil { quit(err) }
      sNodePin = fmt.Sprintf("%03d%03d%03d", aBuf[0], aBuf[1], aBuf[2])
      //todo timer to clear sNodePin
   } else {
      sNodePin = ""
   }
   return kNodeListen
}

func CheckPinNode(iPin string) bool {
   //todo limit number of checks
   return sNodePin != "" && strings.ReplaceAll(iPin, " ", "") == sNodePin
}

func StartNode(iSvc string) []string {
   //todo disable listen
   _, err := os.Stat(fileTemp(iSvc))
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      if getService(iSvc) == nil {
         fmt.Fprintf(os.Stderr, "StartNode %s: %s not found\n", iSvc, fileTemp(iSvc))
         return nil
      }
      return kNodeListen
   }
   err = addNodeService(iSvc, fileTemp(iSvc))
   if err != nil {
      fmt.Fprintf(os.Stderr, "StartNode %s: %v\n", iSvc, err)
      return kNodeListen
   }
   return kNodeStart
}

func MakeNode(iR io.Reader) error {
   aTf := tar.NewReader(iR)
   aHead, err := aTf.Next()
   if err != nil { return err }
   if aHead.Typeflag != tar.TypeDir {
      return tError("account type not directory")
   }
   aSvc := aHead.Name
   if !checkNameService(aSvc) || aSvc == "EOT" {
      return tError("account name invalid: "+ aSvc)
   }
   if getService(aSvc) != nil {
      return tError("account already exists: "+ aSvc)
   }
   aTemp := aSvc +".tmp"
   err = os.Mkdir(dirSvc(aTemp), 0700)
   if err != nil {
      if !os.IsExist(err) { quit(err) }
      return tError("account in progress: "+ aSvc)
   }
   defer os.RemoveAll(dirSvc(aTemp))
   err = os.Rename(fileTemp(aSvc), dirSvc(aTemp) + "to-remove")
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
   } else {
      err = os.RemoveAll(dirSvc(aTemp) + "to-remove")
      if err != nil { quit(err) }
      fmt.Printf("MakeNode: removed incomplete instance of %s\n", aSvc)
   }
   makeTreeService(aTemp)
   fmt.Printf("MakeNode: replicating %s\n", aSvc)

   for {
      aHead, err = aTf.Next()
      if err != nil { return err }
      if aHead.Name == "EOT" {
         aHead, err = aTf.Next()
         if err != io.EOF {
            return tError("got record after EOT: "+ aHead.Name)
         }
         break
      }
      var aName string
      aName, err =_checkSvcPath(aHead.Name)
      if err != nil { return err }
      if aHead.Typeflag == tar.TypeLink {
         var aLink string
         aLink, err =_checkSvcPath(aHead.Linkname)
         if err != nil { return err }
         err = os.Link(dirSvc(aTemp) + aLink, dirSvc(aTemp) + aName)
         if err != nil {
            if !os.IsNotExist(err) && !os.IsExist(err) { quit(err) }
            return err
         }
      } else if aHead.Typeflag == tar.TypeDir {
         err = os.Mkdir(dirSvc(aTemp) + aName, os.FileMode(aHead.Mode))
         if err != nil {
            if !os.IsNotExist(err) && !os.IsExist(err) { quit(err) }
            return err
         }
      } else if aHead.Typeflag == tar.TypeReg {
         err = os.Remove(dirSvc(aTemp) + aName) // delete placeholder
         if err != nil && !os.IsNotExist(err) { quit(err) }
         var aFd *os.File
         aFd, err = os.OpenFile(dirSvc(aTemp) + aName, os.O_WRONLY|os.O_CREATE|os.O_EXCL,
                                os.FileMode(aHead.Mode))
         if err != nil {
            if !os.IsNotExist(err) { quit(err) }
            return err
         }
         func() {
            var cLen, cLenDecode int64
            if aName == "tag" {
               cTagset := tTagset{}
               err = json.NewDecoder(&tReadCounter{aTf, &cLenDecode}).Decode(&cTagset)
               if err != nil { return }
               cTagset, err = fixConflictTag(cTagset, aSvc)
               if err != nil { return }
               err = json.NewEncoder(aFd).Encode(cTagset)
               if err != nil { quit(err) }
            }
            cLen, err = io.Copy(aFd, aTf)
            if err == nil && cLen + cLenDecode != aHead.Size {
               err = tError("size mismatch for "+ aName)
            }
         }()
         if err != nil {
            aFd.Close()
            return err //todo only network errors
         }
         err = aFd.Sync()
         if err != nil { quit(err) }
         aFd.Close()
         err = os.Chtimes(dirSvc(aTemp) + aName, aHead.ModTime, aHead.ModTime)
         if err != nil { quit(err) }
      } else {
         return tError("unexpected typeflag: "+ string(aHead.Typeflag))
      }
   }
   aDir, err := readDirNames(dirTemp(aTemp))
   if err != nil { quit(err) }
   for a := range aDir {
      err = os.Remove(dirTemp(aTemp) + aDir[a])
      if err != nil { quit(err) }
   }
   err = syncTree(dirSvc(aTemp))
   if err != nil { quit(err) }
   err = os.Rename(dirSvc(aTemp), fileTemp(aSvc))
   if err != nil { quit(err) }
   return nil
}

func _checkSvcPath(iPath string) (string, error) {
   aMax := -1
   switch {
   case strings.HasPrefix(iPath, "form/"):   aMax = 2
   case strings.HasPrefix(iPath, "attach/"): aMax = 3
   }
   aPath := strings.SplitN(iPath, "/", aMax)
   for a := range aPath {
      if aPath[a] == ".." || aPath[a] == "." || aPath[a] == "" || IsReservedFile(aPath[a]) {
         return "", tError("disallowed path element: "+ aPath[a])
      }
      if !(a == len(aPath)-1 && (aPath[0] == "attach" || aPath[0] == "form")) &&
         aPath[a] != escapeFile(aPath[a]) {
         return "", tError("invalid character in "+ aPath[a])
      }
   }
   if aPath[0] == "attach" || aPath[0] == "form" {
      aFile := aPath[len(aPath)-1]
      return iPath[:len(iPath) - len(aFile)] + escapeFile(aFile), nil
   }
   return iPath, nil
}

func GetCnNode(iSvc string) interface{} {
   type tCn struct{ Addr, Pin string; Xfer int64 }
   aSvc := getService(iSvc)
   return &tCn{aSvc.toNode.Addr, aSvc.toNode.Pin, aSvc.toNode.Xfer} //todo possible race?
}

func createNode(iSvc string, iUpdt *Update, iNode *tNode) error {
   if iNode.Status != eNodePending && iNode.Status != eNodeAllowed {
      return tError("node status: "+ string(iNode.Status))
   }
   aTn := tToNode{Addr: iUpdt.Node.Addr, Pin: iUpdt.Node.Pin, client: &http.Client{}}
   aRsp, err := aTn.client.Get(aTn.url(""))
   if err != nil { return err }
   _, err = io.Copy(ioutil.Discard, aRsp.Body)
   if err != nil { return err }
   aRsp.Body.Close()
   if aRsp.StatusCode != http.StatusOK {
      return tError("pin not accepted")
   }
   aSvc := getService(iSvc)
   if !atomic.CompareAndSwapInt32(&aSvc.toNode.busy, 0, 1) {
      return tError("node creation in progress")
   }
   aSvc.toNode.Addr, aSvc.toNode.Pin, aSvc.toNode.client = aTn.Addr, aTn.Pin, aTn.client
   aSvc.toNode.Xfer = 0
   if iNode.Status == eNodePending {
      addQueue(iSvc, eSrecNode, iNode.Qid)
   } else {
      sMsgToSelfFn(iSvc, &Header{Op:"_node", NewNode:iNode.Name, NodeId:iNode.NodeId})
   }
   return nil
}

func sendUserEditNode(iW io.Writer, iSvc string, iQid, iId string) error {
   aId := parseLocalId(iQid)
   aHead, err := json.Marshal(Msg{"Op":3, "Id":iId, "Newnode":aId.info()})
   if err != nil { quit(err) }
   err = writeHeaders(iW, aHead, nil)
   return err
}

func replicateNode(iSvc string, iNode *tNode) error {
   aSvc := getService(iSvc)
   if !atomic.CompareAndSwapInt32(&aSvc.toNode.busy, 1, 2) {
      return tError("toNode unidentified or replicating")
   }
   defer atomic.StoreInt32(&aSvc.toNode.busy, 0)
   aSvc.updt.Lock(); defer aSvc.updt.Unlock()
   var err error
   defer func() { if err != nil { aSvc.toNode.client = nil } }()
   aBody, aTar := io.Pipe()
   go _runTar(iSvc, aTar, iNode, &aSvc.toNode)
   aRsp, err := aSvc.toNode.client.Post(aSvc.toNode.url(""), "application/x-tar", aBody)
   if err != nil { return err }
   defer aRsp.Body.Close()
   aBuf := make([]byte, 128)
   aLen, err := aRsp.Body.Read(aBuf)
   if err != nil {
      if err != io.EOF { return err }
      err = nil // don't trigger defer
   }
   if aRsp.StatusCode != http.StatusOK || aLen > 0 {
      err = tError(aRsp.Status +" "+ string(aBuf[:aLen])) // trigger defer
      return err
   }
   return nil
}

func _runTar(iSvc string, iW *io.PipeWriter, iNode *tNode, iToNode *tToNode) {
   // output not suitable for tar utilities
   var err error
   defer func() { if err != nil { fmt.Fprintf(os.Stderr, "_runTar %s: %v\n", iSvc, err) } }()
   aSfx := ""; if iToNode.isLocalhost() { aSfx = "."+ iNode.Name }
   aTf := tar.NewWriter(iW)

   aHead := tar.Header{Name: iSvc+aSfx, Typeflag: tar.TypeDir, Mode: 0700}
   err = aTf.WriteHeader(&aHead)
   if err != nil { return }

   fPut := func(cPath string) error {
      cFd, err := os.Open(cPath)
      if err != nil { quit(err) }
      defer cFd.Close()
      var cLen int64
      if aHead.Typeflag == '_' {
         cDl := newDraftlessThread(cFd)
         aHead.Typeflag = tar.TypeReg
         aHead.Size = cDl.size()
         err = aTf.WriteHeader(&aHead)
         if err != nil { return err }
         cLen, err = cDl.copy(aTf)
      } else {
         err = aTf.WriteHeader(&aHead)
         if err != nil { return err }
         cLen, err = io.Copy(aTf, cFd)
      }
      if err != nil { return err } //todo only network error
      if cLen != aHead.Size {
         quit(tError("size mismatch"))
      }
      iToNode.Xfer += aHead.Size
      return nil
   }
   var fSub func(string)error
   fSub = func(cName string) error {
      cHeadType := byte(tar.TypeReg)
      cDir, err := readDirFis(dirSvc(iSvc) + cName)
      if err != nil { quit(err) }
      sort.Slice(cDir, func(ccA, ccB int)bool { return cDir[ccA].Name() > cDir[ccB].Name() })
      for _, cFi := range cDir {
         cPath := cName +"/"+ cFi.Name()
         if cFi.IsDir() {
            if cName == "attach" && cFi.Name()[0] == '_' { continue }
            // doesn't omit attach/sub/ with only draft-owned files
            aHead = tar.Header{Name: cPath, Typeflag: tar.TypeDir, Mode: int64(cFi.Mode())}
            err = aTf.WriteHeader(&aHead)
            if err != nil { return err }
            if cName == "attach" { continue }
            err = fSub(cPath)
         } else {
            if cName == "thread" { //todo revert this when thread draft sync added
               cPos := strings.IndexByte(cFi.Name(), '_')
               if cPos > 0 {
                  cHeadType = '_'
               }
               if cPos >= 0 { continue }
            }
            cPathHead := cPath
            if strings.HasPrefix(cPath, "form/") {
               cPathHead = unescapeFile(cPath)
            }
            aHead = tar.Header{Name: cPathHead, Size: cFi.Size(),
                               ModTime: cFi.ModTime(), Typeflag: cHeadType, Mode: int64(cFi.Mode())}
            err = fPut(dirSvc(iSvc) + cPath)
            cHeadType = tar.TypeReg
         }
         if err != nil { return err }
      }
      return nil
   }

   aDir, err := readDirFis(dirSvc(iSvc))
   if err != nil { quit(err) }
   for _, aFi := range aDir {
      if aFi.Name() == "temp" || aFi.Name() == "sendq" ||
         aFi.Name() == "ping-draft" || aFi.Name() == "index.bleve" { continue }
      if aFi.IsDir() {
         err = fSub(aFi.Name())
      } else if aFi.Name() == "config" {
         aCfg := makeNodeConfigService(iSvc, iNode) //todo in Marshal()
         aCfg.Name += aSfx
         var aBuf []byte
         aBuf, err = json.Marshal(aCfg)
         if err != nil { quit(err) }
         aHead = tar.Header{Name: aFi.Name(), Size: int64(len(aBuf)),
                            ModTime: time.Now(), Typeflag: tar.TypeReg, Mode: 0600}
         err = aTf.WriteHeader(&aHead)
         if err != nil { return }
         _, err = aTf.Write(aBuf)
      } else {
         _, err = os.Stat(dirSvc(iSvc) + aFi.Name())
         if err != nil {
            if !os.IsNotExist(err) { quit(err) }
            continue // placeholder symlink
         }
         aHead = tar.Header{Name: aFi.Name(), Size: aFi.Size(),
                            ModTime: aFi.ModTime(), Typeflag: tar.TypeReg, Mode: int64(aFi.Mode())}
         err = fPut(dirSvc(iSvc) + aFi.Name())
      }
      if err != nil { return }
   }
   aAtc := _getAttachInodes(iSvc)
   var aIno uint64
   for a := range aAtc {
      if strings.HasPrefix(aAtc[a].path, kNodeFlagUpload) { continue }
      if aAtc[a].path[0] == '_' || strings.IndexByte(aAtc[a].path[17:], '_') == 12 { continue }
      if aAtc[a].inode != aIno {
         aIno = aAtc[a].inode
         aHead = tar.Header{Name: fmt.Sprintf("temp/%d", aAtc[a].inode), Size: aAtc[a].size,
                            ModTime: aAtc[a].modtime, Typeflag: tar.TypeReg, Mode: 0600}
         err = fPut(dirAttach(iSvc) + aAtc[a].path)
         if err != nil { return }
      }
      aHead = tar.Header{Name: "attach/"+ unescapeFile(aAtc[a].path),
                         Linkname: fmt.Sprintf("temp/%d", aAtc[a].inode), Typeflag: tar.TypeLink}
      err = aTf.WriteHeader(&aHead)
      if err != nil { return }
   }

   aHead = tar.Header{Name: "EOT", Typeflag: tar.TypeDir}
   err = aTf.WriteHeader(&aHead)
   if err != nil { return }
   err = aTf.Close()
   if err != nil { return }
   err = iW.Close()
}

func _getAttachInodes(iSvc string) []tPathInode {
   aDir, err := readDirNames(dirAttach(iSvc))
   if err != nil { quit(err) }
   aList := make([]tPathInode, 0, 2*len(aDir)) //todo find average attachments/thread
   for a := range aDir {
      var aSub []os.FileInfo
      aSub, err = readDirFis(dirAttach(iSvc) + aDir[a])
      if err != nil { quit(err) }
      for _, aFi := range aSub {
         var aId uint64
         aId, err = getInode(dirAttach(iSvc) + aDir[a], aFi)
         if err != nil { quit(err) }
         aList = append(aList, tPathInode{aDir[a] +"/"+ aFi.Name(), aId, aFi.Size(), aFi.ModTime()})
      }
   }
   sort.Slice(aList, func(cA, cB int)bool { return aList[cA].inode < aList[cB].inode })

   aDirUp, err := readDirFis(kUploadDir)
   if err != nil { quit(err) }
   for _, aFi := range aDirUp {
      var aId uint64
      aId, err = getInode(kUploadDir, aFi)
      if err != nil { quit(err) }
      aPos := sort.Search(len(aList), func(c int)bool { return aList[c].inode >= aId })
      if aPos < len(aList) && aList[aPos].inode == aId {
         aList = append(aList, tPathInode{})
         copy(aList[aPos+1:], aList[aPos:])
         aList[aPos] = tPathInode{kNodeFlagUpload + aFi.Name(), aId, aFi.Size(), aFi.ModTime()}
      }
   }
   return aList
}

func completeNode(iSvc string, iUpdt *Update, iNode *tNode) error {
   aSvc := getService(iSvc)
   if !atomic.CompareAndSwapInt32(&aSvc.toNode.busy, 0, 2) {
      return tError("toNode unidentified or busy")
   }
   defer atomic.StoreInt32(&aSvc.toNode.busy, 0)
   if iUpdt != nil {
      aSvc.toNode.client = &http.Client{}
      aSvc.toNode.Addr, aSvc.toNode.Pin = iUpdt.Node.Addr, iUpdt.Node.Pin
   }
   defer func() { aSvc.toNode.client = nil }()
   aSfx := ""; if aSvc.toNode.isLocalhost() { aSfx = "."+ iNode.Name }
   aRsp, err := aSvc.toNode.client.Get(aSvc.toNode.url(iSvc+aSfx))
   if err != nil { return err }
   _, err = io.Copy(ioutil.Discard, aRsp.Body)
   if err != nil { return err }
   aRsp.Body.Close()
   if aRsp.StatusCode != http.StatusOK {
      return tError("status: "+ aRsp.Status)
   }
   return nil
}

func SetSyncPeriodNode(iPeriod time.Duration) { sNodeSyncPeriod = iPeriod }

func syncUpdtNode(iSvc string, iUpdt *Update, iState *ClientState, iFunc func()error) {
   if iUpdt.log == eLogNone {
      iFunc()
      return
   }
   aLog := ftmpSyncLog(iSvc)
   aTempOk := ftmpSyncUpdt(iSvc, iState.id)
   aTemp := aTempOk +".tmp"
   var aTd *os.File
   var err error
   var aPos int64

   aSvc := getService(iSvc)
   aSvc.nodeUpdt.Lock(); defer aSvc.nodeUpdt.Unlock()

   if iUpdt.log != eLogRetry {
      aFi, err := os.Stat(aLog)
      if err != nil {
         if !os.IsNotExist(err) { quit(err) }
      } else {
         aPos = aFi.Size(); if aPos > 0 { aPos-- }
      }
      aTempOk += fmt.Sprint(aPos)

      aTd, err = os.OpenFile(aTemp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
      if err != nil { quit(err) }
      defer aTd.Close()

      iUpdt.LogThreadId = iState.getThread()
      err = json.NewEncoder(aTd).Encode(iUpdt)
      if err != nil { quit(err) }
      err = aTd.Sync()
      if err != nil { quit(err) }
      _, err = aTd.Seek(0, io.SeekStart)
      if err != nil { quit(err) }

      err = os.Rename(aTemp, aTempOk)
      if err != nil { quit(err) }
      err = syncDir(dirTemp(iSvc))
      if err != nil { quit(err) }
   } else {
      aPos = iUpdt.logPos
      aTempOk += fmt.Sprint(aPos)
      aTd, err = os.Open(aTempOk)
      if err != nil { quit(err) }
      defer aTd.Close()
   }

   if iFunc() == nil {
      sCrashFn(iSvc, "sync-updt-node")
      var aFd *os.File
      aFd, err = os.OpenFile(aLog, os.O_WRONLY, 0600)
      if err != nil {
         if !os.IsNotExist(err) { quit(err) }
         err = os.Remove(aLog)
         if err != nil && !os.IsNotExist(err) { quit(err) }
         aLogQ := ftmpSyncLogQ(iSvc, makeLocalId("")[1:])
         err = os.Symlink(aLogQ[len(dirTemp(iSvc)):], aLog)
         if err != nil { quit(err) }
         aFd, err = os.OpenFile(aLogQ, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
         if err != nil { quit(err) }
         err = syncDir(dirTemp(iSvc))
         if err != nil { quit(err) }
//todo fmt.Println("## log  ", aLogQ, iSvc)
      }
      defer aFd.Close()

      aChar := byte('['); if aPos > 0 { aChar = ',' }
      _, err = aFd.Seek(aPos, io.SeekStart)
      if err != nil { quit(err) }
      _, err = aFd.Write([]byte{aChar, '\n'})
      if err != nil { quit(err) }
      _, err = io.Copy(aFd, aTd)
      if err != nil { quit(err) }
      _, err = aFd.Write([]byte{']'})
      if err != nil { quit(err) }
      err = aFd.Sync()
      if err != nil { quit(err) }

      if aPos == 0 {
         _startSyncTimer(iSvc)
      }
   }
   err = os.Remove(aTempOk)
   if err != nil { quit(err) }
}

func initSyncNode(iSvc string) {
   aFi, err := os.Stat(ftmpSyncLog(iSvc))
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
   } else if aFi.Size() > 0 {
      var aPath string
      aPath, err = os.Readlink(ftmpSyncLog(iSvc))
      if err != nil { quit(err) }
      if !hasQueue(iSvc, eSrecSync, aPath) {
         _startSyncTimer(iSvc)
      }
   }
}

func _startSyncTimer(iSvc string) {
   //todo P2P xfer to local nodes, with short delay
   // updates from nodes with conflicting inputs that cross on the relay can yield out-of-sync state
   aPath, err := os.Readlink(ftmpSyncLog(iSvc))
   if err != nil { quit(err) }
   _ = time.AfterFunc(sNodeSyncPeriod, func() {
      //todo don't queue if no link to service; retry timer
      addQueue(iSvc, eSrecSync, aPath)
   })
}

func sendSyncNode(iW io.Writer, iSvc string, iNodeQ, iId string) error {
   aSvc := getService(iSvc)
   aSvc.nodeUpdt.Lock()
   aPath, err := os.Readlink(ftmpSyncLog(iSvc))
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
   } else if aPath == iNodeQ {
      err = os.Remove(ftmpSyncLog(iSvc))
      if err != nil { quit(err) }
   }
   aSvc.nodeUpdt.Unlock()
//todo fmt.Println("## send ", iNodeQ, iSvc, " log", aPath, err)

   aFd, err := os.Open(dirTemp(iSvc) + iNodeQ)
   if err != nil { quit(err) }
   defer aFd.Close()
   aFi, err := aFd.Stat()
   if err != nil { quit(err) }

   aSubh, err := json.Marshal(tHeader2{NodeSync:true})
   if err != nil { quit(err) }
   aMsg := Msg{"Op":7, "Id":iId, "For":[]tHeaderFor{},
               "DataHead": len(aSubh), "DataLen": int64(len(aSubh)) + aFi.Size()}
   aHead, err := json.Marshal(aMsg)
   if err != nil { quit(err) }

   err = writeHeaders(iW, aHead, aSubh)
   if err != nil { return err }
   _, err = io.Copy(iW, aFd) //todo only return network errors
   return err
}

func dropSyncNode(iSvc string, iNodeQ, iQid string, iComplete string) {
   aTempOk := ftmpSyncAck(iSvc, iQid)
   aTemp := aTempOk +".tmp"
   var err error
   if iComplete == "" {
      var aFd *os.File
      aFd, err = os.OpenFile(aTemp, os.O_RDONLY|os.O_CREATE|os.O_EXCL, 0600)
      if err != nil { quit(err) }
      err = aFd.Sync() // not strictly required since no data
      if err != nil { quit(err) }
      aFd.Close()
      err = os.Rename(aTemp, aTempOk)
      if err != nil { quit(err) }
      err = syncDir(dirTemp(iSvc))
      if err != nil { quit(err) }
   } else {
      fmt.Printf("complete %s\n", aTempOk[len(dirTemp(iSvc)):])
   }
   sCrashFn(iSvc, "drop-sync-node")
   dropQueue(iSvc, iQid)
   err = os.Remove(dirTemp(iSvc) + iNodeQ)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   err = os.Remove(aTempOk)
   if err != nil { quit(err) }
}

func completeUpdtNode(iSvc string, iTmp string) {
   fmt.Printf("complete %s\n", iTmp)
   var err error
   aUpdt := Update{log:eLogRetry}
   aRec := strings.SplitN(iTmp, "_", 3)
   aUpdt.logPos, err = strconv.ParseInt(aRec[2], 10, 64)
   if err != nil { quit(err) }
   aCs := OpenState(aRec[1], iSvc)
   err = readJsonFile(&aUpdt, dirTemp(iSvc) + iTmp)
   if err != nil { quit(err) }
   aFunc, _ := HandleUpdtService(iSvc, aCs, &aUpdt) // must call syncUpdtNode()
   aFunc(aCs) //todo update other saved ClientStates
}
