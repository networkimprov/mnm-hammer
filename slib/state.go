// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package slib

import (
   "fmt"
   "encoding/json"
   "os"
   "strings"
   "sync"
)

const kHistoryLen = 4

var sStateDoor sync.Mutex
var sStates = make(map[string]bool) // key client id

func initStates() {
   var err error
   aClients, err := readDirNames(kStateDir)
   if err != nil { quit(err) }

   for _, aDir := range aClients {
      var aStates []string
      aStates, err = readDirNames(kStateDir + aDir)
      if err != nil { quit(err) }

      for _, aFile := range aStates {
         if strings.HasSuffix(aFile, ".tmp") {
            err = resolveTmpFile(kStateDir + aDir + "/" + aFile)
            if err != nil { quit(err) }
         }
      }
   }
}

func OpenState(iClientId, iSvc string) *ClientState {
   var err error
   sStateDoor.Lock()
   if !sStates[iClientId] {
      err = os.MkdirAll(kStateDir + iClientId, 0700)
      if err != nil { quit(err) }
      err = syncDir(kStateDir)
      if err != nil { quit(err) }
      sStates[iClientId] = true
   }
   sStateDoor.Unlock()
   aState := &ClientState{Hpos: -1, Thread: make(map[string]*tThreadState),
                          filePath: kStateDir + iClientId + "/" + iSvc}
   var aFd *os.File
   aFd, err = os.Open(aState.filePath)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
   } else {
      err = json.NewDecoder(aFd).Decode(aState)
      aFd.Close()
      if err != nil { quit(err) }
   }
   return aState
}

type ClientState struct {
   sync.RWMutex
   filePath string
   Hpos int // indexes History
   History []string // thread id
   Thread map[string]*tThreadState // key thread id
   SvcTabs tTabs
}

type tThreadState struct {
   Open tOpenState
   Tabs tTabs
   Discard bool
   Refs int
}

const ( eTabThread=iota; eTabService )

const ( ePosForDefault=iota; ePosForTerms; ePosForPinned; ePosForEnd )

type tTabs struct {
   Pos int
   PosFor int8
   Terms []string
}

type tOpenState map[string]bool // key msg id

func (o tOpenState) MarshalJSON() ([]byte, error) {
   aBuf := []byte{'{'}
   for aK, aV := range o {
      if aV { aBuf = append(aBuf, (`"`+aK+`":true,`)...) }
   }
   if len(aBuf) > 1 {
      aBuf = aBuf[:len(aBuf)-1]
   }
   return append(aBuf, '}'), nil
}

func (o *ClientState) GetSummary() Msg {
   o.RLock(); defer o.RUnlock()
   aS := Msg{"Thread":"none"}
   if o.Hpos >= 0 {
      aS["Thread"] = o.History[o.Hpos]
      aS["ThreadTabs"] = o.Thread[o.History[o.Hpos]].Tabs
      aH := 0
      if o.Hpos == 0 {
         aH = -1
      } else if o.Hpos == len(o.History)-1 {
         aH = 1
      }
      aS["History"] = aH
   }
   aS["SvcTabs"] = o.SvcTabs
   return aS
}

func (o *ClientState) getThread() string {
   o.RLock(); defer o.RUnlock()
   if o.Hpos < 0 {
      return ""
   }
   return o.History[o.Hpos]
}

func (o *ClientState) isOpen(iMsgId string) bool {
   o.RLock(); defer o.RUnlock()
   aT := o.Thread[o.History[o.Hpos]]
   return aT.Tabs.PosFor == ePosForDefault &&
         (aT.Tabs.Pos == 1 || aT.Tabs.Pos == 0 && aT.Open[iMsgId])
}

func (o *ClientState) addThread(iId, iLastMsgId string) {
   o.Lock(); defer o.Unlock()
   if o.Thread[iId] == nil {
      o.Thread[iId] = &tThreadState{Open: tOpenState{iLastMsgId:true}}
   } else if iId == o.History[o.Hpos] {
      fmt.Fprintf(os.Stderr, "addThread: ignored attempt to readd %s\n", iId)
      return
   }
   o.Thread[iId].Refs++
   fDropRef := func(cPos int) {
      cT := o.Thread[o.History[cPos]]
      cT.Refs--
      if cT.Refs == 0 {
         delete(o.Thread, o.History[cPos])
      }
   }
   if o.Hpos >= 2 && iId == o.History[o.Hpos-1] && o.History[o.Hpos] == o.History[o.Hpos-2] {
      o.Hpos-- // prevent ABAB repetition
   } else {
      o.Hpos++
   }
   if o.Hpos > len(o.History)-1 {
      o.History = append(o.History, iId)
      if o.Hpos >= kHistoryLen {
         fDropRef(0)
         o.History = o.History[1:]
         o.Hpos--
      }
   } else {
      for a := o.Hpos; a < len(o.History); a++ {
         fDropRef(a)
      }
      o.History[o.Hpos] = iId
      o.History = o.History[:o.Hpos+1]
   }
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}

func (o *ClientState) openMsg(iMsgId string, iBool bool) {
   o.Lock(); defer o.Unlock()
   o.Thread[o.History[o.Hpos]].Open[iMsgId] = iBool
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}

func (o *ClientState) goThread(i int) {
   o.Lock(); defer o.Unlock()
   aV := o.Hpos + i
   if aV < 0 || aV > len(o.History)-1 {
      return
   }
   o.Hpos = aV
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}

func (o *ClientState) renameThread(iId, iNewId string) {
   o.Lock(); defer o.Unlock()
   aT := o.Thread[iId]
   if aT == nil {
      return
   }
   o.Thread[iNewId] = aT
   delete(o.Thread, iId)
   for a, _ := range o.History {
      if o.History[a] == iId {
         o.History[a] = iNewId
      }
   }
   aT.Open[iNewId] = aT.Open[iId]
   delete(aT.Open, iId)
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}

func (o *ClientState) renameMsg(iThreadId, iMsgId, iNewId string) {
   o.Lock(); defer o.Unlock()
   aT := o.Thread[iThreadId]
   if aT == nil {
      return
   }
   aT.Open[iNewId] = aT.Open[iMsgId]
   delete(aT.Open, iMsgId)
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}

func (o *ClientState) discardThread(iId string) {
   o.Lock(); defer o.Unlock()
   aT := o.Thread[iId]
   if aT == nil {
      return
   }
   if iId == o.History[len(o.History)-1] {
      aT.Refs--
      if aT.Refs == 0 {
         delete(o.Thread, iId)
      }
      if o.Hpos == len(o.History)-1 {
         o.Hpos--
      }
      o.History = o.History[:len(o.History)-1]
   } else {
      aT.Discard = true
   }
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}

func (o *ClientState) addTab(iType int8, iTerm string) {
   o.Lock(); defer o.Unlock()
   aTabs := &o.SvcTabs; if iType == eTabThread { aTabs = &o.Thread[o.History[o.Hpos]].Tabs }
   aTabs.Pos = len(aTabs.Terms)
   aTabs.PosFor = ePosForTerms
   aTabs.Terms = append(aTabs.Terms, iTerm)
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}

func (o *ClientState) setTab(iType int8, iPosFor int8, iPos int) {
   o.Lock(); defer o.Unlock()
   aTabs := &o.SvcTabs; if iType == eTabThread { aTabs = &o.Thread[o.History[o.Hpos]].Tabs }
   if iPosFor < 0 || iPosFor >= ePosForEnd { quit(tError("setTab: iPosFor out of range")) }
   if iPos >= len(aTabs.Terms) && iPosFor == ePosForTerms ||
      iPos < 0 { quit(tError("setTab: iPos out of range")) }

   aTabs.PosFor = iPosFor
   aTabs.Pos = iPos
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}

func (o *ClientState) dropTab(iType int8) {
   o.Lock(); defer o.Unlock()
   aTabs := &o.SvcTabs; if iType == eTabThread { aTabs = &o.Thread[o.History[o.Hpos]].Tabs }
   if aTabs.PosFor != ePosForTerms { quit(tError("dropTab: not ePosForTerms")) }
   if len(aTabs.Terms) == 0 { quit(tError("dropTab: no terms to drop")) }

   if aTabs.Pos == len(aTabs.Terms)-1 {
      if aTabs.Pos == 0 {
         aTabs.PosFor = ePosForDefault
      } else {
         aTabs.Pos--
      }
   } else {
      copy(aTabs.Terms[aTabs.Pos:], aTabs.Terms[aTabs.Pos+1:])
   }
   if len(aTabs.Terms) == 1 {
      aTabs.Terms = nil
   } else {
      aTabs.Terms = aTabs.Terms[:len(aTabs.Terms)-1]
   }
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}
