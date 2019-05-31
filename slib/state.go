// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "fmt"
   "encoding/json"
   "os"
   "strings"
   "sync"
)

var sSortDefault = tSummarySort{Cc:"Who", Atc:"Date", Upload:"Date", Form:"Date"}
var sSvcTabsDefault = []string{"All","Unread","Todo","FFT"}
var sThreadTabsDefault = []string{"Open","All"}

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
   aState := &ClientState{Hpos: -1,
                          Thread: make(map[string]*tThreadState),
                          SvcTabs: tTabs{Terms:[]string{}},
                          historyMax: GetConfigService(iSvc).HistoryLen,
                          svc: iSvc, filePath: fileState(iClientId, iSvc)}
   aFd, err := os.Open(aState.filePath)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      err = os.Symlink("new_state", aState.filePath)
      if err == nil {
         err = syncDir(kStateDir + iClientId)
      }
      if err != nil && !os.IsExist(err) { quit(err) }
   } else {
      err = json.NewDecoder(aFd).Decode(aState)
      aFd.Close()
      if err != nil { quit(err) }
   }
   return aState
}

type ClientState struct {
   sync.RWMutex
   svc string
   filePath string
   historyMax int
   Hpos int // indexes History
   History []string // thread id
   Thread map[string]*tThreadState // key thread id
   SvcTabs tTabs
   UploadSort, FormSort string `json:",omitempty"`
}

type tThreadState struct {
   CcSort, AtcSort string `json:",omitempty"`
   Open tOpenState
   Tabs tTabs
   Discard bool
   Refs int
}

type tTabs struct {
   Pos int
   PosFor int8
   Terms []string
}

const ( ePosForDefault=iota; ePosForPinned; ePosForTerms; ePosForEnd )

func (o *tTabs) copy() *tTabs {
   return &tTabs{Pos:o.Pos, PosFor:o.PosFor, Terms:append([]string{}, o.Terms...)}
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

type tSummary struct {
   Sort tSummarySort
   Thread string
   ThreadTabs *tSummaryTabs `json:",omitempty"`
   History struct{ Prev, Next bool }
   SvcTabs tSummaryTabs
}

type tSummarySort struct {
   Cc     string `json:"cl"`
   Atc    string `json:"al"`
   Upload string `json:"t"`
   Form   string `json:"f"`
}

type tSummaryTabs struct {
   tTabs
   Default []string
   Pinned  *[]string `json:",omitempty"`
   Type int8
}

const ( eTabThread=iota; eTabService )

func (o *ClientState) GetSummary() interface{} {
   aPinned := getTabsService(o.svc)

   o.RLock(); defer o.RUnlock()
   aS := &tSummary{ Sort: sSortDefault, Thread: "none",
                    SvcTabs: tSummaryTabs{ Type: eTabService, Default: sSvcTabsDefault,
                                           tTabs: *o.SvcTabs.copy(), Pinned: &aPinned }}
   if o.UploadSort != "" { aS.Sort.Upload = o.UploadSort }
   if o.FormSort   != "" { aS.Sort.Form   = o.FormSort }

   if o.Hpos >= 0 {
      aTs := o.Thread[o.History[o.Hpos]]
      if aTs.CcSort  != "" { aS.Sort.Cc  = aTs.CcSort }
      if aTs.AtcSort != "" { aS.Sort.Atc = aTs.AtcSort }
      aS.Thread = o.History[o.Hpos]
      aS.ThreadTabs = &tSummaryTabs{ Type: eTabThread, Default: sThreadTabsDefault,
                                     tTabs: *aTs.Tabs.copy() }
      aS.History.Prev = o.Hpos > 0
      aS.History.Next = o.Hpos < len(o.History)-1
   }
   return aS
}

func (o *ClientState) setHistoryMax(iLen int) {
   o.RLock(); defer o.RUnlock()
   o.historyMax = iLen
   // if max < len(o.History), o.History shrinks on back+add
}

func (o *ClientState) getThread() string {
   o.RLock(); defer o.RUnlock()
   if o.Hpos < 0 {
      return ""
   }
   return o.History[o.Hpos]
}

func (o *ClientState) getSvcTab() (int8, string) {
   o.RLock(); defer o.RUnlock()
   var aSet []string
   switch o.SvcTabs.PosFor {
   case ePosForDefault: aSet = sSvcTabsDefault
   case ePosForPinned:  aSet = getTabsService(o.svc)
   case ePosForTerms:   aSet = o.SvcTabs.Terms
   }
   return o.SvcTabs.PosFor, aSet[o.SvcTabs.Pos]
}

func (o *ClientState) isOpen(iMsgId string) bool {
   o.RLock(); defer o.RUnlock()
   aT := o.Thread[o.History[o.Hpos]]
   if aT.Tabs.PosFor == ePosForDefault {
      return aT.Tabs.Pos == 1 || aT.Tabs.Pos == 0 && aT.Open[iMsgId]
   } else if aT.Tabs.Terms[aT.Tabs.Pos][0] == '&' {
      return aT.Tabs.Terms[aT.Tabs.Pos][1:] == iMsgId
   }
   return false
}

func (o *ClientState) addThread(iId string) {
   o.Lock(); defer o.Unlock()
   o._addThread(iId)
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}

func (o *ClientState) _addThread(iId string) {
   if o.Thread[iId] == nil {
      aMsgId := iId; if iId[0] != '_' { aMsgId = loadThread(o.svc, iId) }
      aOpen := tOpenState{}; if aMsgId != "" { aOpen[aMsgId] = true }
      o.Thread[iId] = &tThreadState{Open: aOpen, Tabs: tTabs{Terms:[]string{}}}
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
   if o.Hpos >= len(o.History) {
      o.History = append(o.History, iId)
   } else {
      for a := o.Hpos; a < len(o.History); a++ {
         fDropRef(a)
      }
      o.History[o.Hpos] = iId
      o.History = o.History[:o.Hpos+1]
   }
   if o.Hpos >= o.historyMax {
      fDropRef(0)
      o.History = o.History[1:]
      o.Hpos--
   }
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
   if aT == nil || !aT.Open[iMsgId] {
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
   aT.Discard = true
   if iId == o.History[len(o.History)-1] {
      aT.Refs--
      if aT.Refs == 0 {
         delete(o.Thread, iId)
      }
      if o.Hpos == len(o.History)-1 {
         o.Hpos--
      }
      o.History = o.History[:len(o.History)-1]
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

func (o *ClientState) pinTab(iType int8) {
   if iType == eTabThread { quit(tError("pinTab: cannot pin thread tab")) }
   o.Lock(); defer o.Unlock()
   aTabs := &o.SvcTabs
   if aTabs.PosFor != ePosForTerms { quit(tError("pinTab: not ePosForTerms")) }

   aOrig := aTabs.Pos
   aTabs.Pos = addTabService(o.svc, aTabs.Terms[aOrig])
   aTabs.PosFor = ePosForPinned
   if len(aTabs.Terms) == 0 { quit(tError("pinTab: no terms to pin")) }
   aTabs.Terms = aTabs.Terms[:aOrig + copy(aTabs.Terms[aOrig:], aTabs.Terms[aOrig+1:])]
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}

func (o *ClientState) dropTab(iType int8) {
   o.Lock(); defer o.Unlock()
   aTabs := &o.SvcTabs; if iType == eTabThread { aTabs = &o.Thread[o.History[o.Hpos]].Tabs }
   if aTabs.PosFor == ePosForDefault { quit(tError("dropTab: cannot drop ePosForDefault")) }

   aOrig := aTabs.Pos
   aFor := aTabs.PosFor
   aTabs.Pos = 0
   aTabs.PosFor = ePosForDefault
   if aFor == ePosForTerms {
      if len(aTabs.Terms) == 0 { quit(tError("dropTab: no terms to drop")) }
      aTabs.Terms = aTabs.Terms[:aOrig + copy(aTabs.Terms[aOrig:], aTabs.Terms[aOrig+1:])]
   }
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
   if aFor == ePosForPinned {
      dropTabService(o.svc, aOrig)
   }
}

func (o *ClientState) setSort(iType string, iField string) {
   o.Lock(); defer o.Unlock()
   switch iType {
   case "t":  o.UploadSort                        = iField
   case "f":  o.FormSort                          = iField
   case "cl": o.Thread[o.History[o.Hpos]].CcSort  = iField
   case "al": o.Thread[o.History[o.Hpos]].AtcSort = iField
   default:
      quit(tError("setSort got unknown type: "+ iType))
   }
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}

func (o *ClientState) goLink(iThreadId, iMsgId string) {
   o.Lock(); defer o.Unlock()
   if o.Hpos < 0 || o.History[o.Hpos] != iThreadId {
      o._addThread(iThreadId)
   }
   aTabs := &o.Thread[o.History[o.Hpos]].Tabs
   aTabs.PosFor = ePosForTerms
   for aTabs.Pos = 0; aTabs.Pos < len(aTabs.Terms); aTabs.Pos++ {
      if aTabs.Terms[aTabs.Pos] == "&" + iMsgId { break }
   }
   if aTabs.Pos == len(aTabs.Terms) {
      aTabs.Terms = append(aTabs.Terms, "&" + iMsgId)
   }
   err := storeFile(o.filePath, o)
   if err != nil { quit(err) }
}
