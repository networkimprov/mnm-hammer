// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "fmt"
   "io"
   "encoding/json"
   "os"
   "path"
   "sort"
   "strconv"
   "strings"
   "sync"
)

const kPingTextMax = 120 //todo 140 when .DataHead dropped// UTF-16 units
const kUidUnknown = "\x00unknown"
var   kResponseNone = tAdrsbkEl{}

type tAdrsbk struct {
   sync.RWMutex
   pingToIdx     map[string]tAdrsbkLog // key alias
   pingFromIdx   map[string]tAdrsbkLog // key uid
   aliasIdx      map[string]string     // key alias, value uid //todo replace with btree
   inviteToIdx   map[string]tAdrsbkLog // key alias + gid
   inviteFromIdx map[string]tAdrsbkLog // key gid
   groupIdx      map[string]tGroupEl   // key gid
   draftDoor     sync.RWMutex
}

type tAdrsbkLog []*tAdrsbkEl

type tAdrsbkEl struct {
   Type int8           `json:",omitempty"`
   Date string         `json:",omitempty"`
   Text string         `json:",omitempty"`
   Alias string        `json:",omitempty"`
   Uid string          `json:",omitempty"`
   MyAlias string      `json:",omitempty"`
   MsgId string        `json:",omitempty"`
   Tid string          `json:",omitempty"`
   Gid string          `json:",omitempty"`
   Qid string          `json:",omitempty"`
   Response *tAdrsbkEl `json:",omitempty"` // not stored
}

const (
   _ int8 = iota
   eAbPingDraft     // Type, Date, Text, Alias,      MyAlias,                 Qid
   eAbSelf          // Type, Date,              Uid, MyAlias
   eAbPingTo        // Type, Date, Text, Alias,      MyAlias, MsgId //todo MsgId in storeSentAdrsbk()
   eAbPingFrom      // Type, Date, Text, Alias, Uid, MyAlias, MsgId
   eAbResolveFrom   // Type, Date,              Uid,                 Tid
   eAbResolveTo     // Type, Date,       Alias, Uid,                 Tid
   eAbInviteTo      // Type, Date, Text, Alias,      MyAlias, MsgId      Gid
   eAbInviteFrom    // Type, Date, Text, Alias, Uid, MyAlias, MsgId,     Gid, Qid
   eAbMsgAccept     // Type, Date,                                       Gid
   eAbMsgJoin       // Type, Date,       Alias, Uid,                     Gid
)

type tGroupEl struct {
   Gid string
   Date string
   Admin bool
}


func _loadAdrsbk(iSvc string) *tAdrsbk {
   aSvc := &getService(iSvc).adrsbk
   aSvc.Lock(); defer aSvc.Unlock()
   if aSvc.aliasIdx != nil {
      return aSvc
   }
   aSvc.pingToIdx     = make(map[string]tAdrsbkLog)
   aSvc.pingFromIdx   = make(map[string]tAdrsbkLog)
   aSvc.aliasIdx      = make(map[string]string)
   aSvc.inviteToIdx   = make(map[string]tAdrsbkLog)
   aSvc.inviteFromIdx = make(map[string]tAdrsbkLog)
   aSvc.groupIdx      = make(map[string]tGroupEl)

   var aLog []tAdrsbkEl
   err := readJsonFile(&aLog, fileAdrs(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   for a, _ := range aLog {
      switch aLog[a].Type {
      case eAbSelf:
         aSvc.aliasIdx[aLog[a].MyAlias] = aLog[a].Uid
      case eAbInviteTo:
         aKey := aLog[a].Alias + "\x00" + aLog[a].Gid
         aEl := aLog[a]
         aUserLog := aSvc.inviteToIdx[aKey]
         aSvc.inviteToIdx[aKey] = _appendLog(aUserLog, &aEl)
         if aSvc.groupIdx[aEl.Gid].Gid == "" {
            aSvc.groupIdx[aEl.Gid] = tGroupEl{Gid:aEl.Gid, Date:aEl.Date, Admin:true}
            aSvc.aliasIdx[aEl.Gid] = aEl.Gid
         }
         fallthrough
      case eAbPingTo:
         aUid := aSvc.aliasIdx[aLog[a].Alias]
         if aUid == "" {
            aSvc.aliasIdx[aLog[a].Alias] = kUidUnknown
         } else if aUid != kUidUnknown {
            _respondLog(aSvc.pingFromIdx[aUid], &aLog[a])
         }
         aUserLog := aSvc.pingToIdx[aLog[a].Alias]
         aSvc.pingToIdx[aLog[a].Alias] = _appendLog(aUserLog, &aLog[a])
      case eAbInviteFrom:
         aEl := aLog[a]
         aUserLog := aSvc.inviteFromIdx[aLog[a].Gid]
         aSvc.inviteFromIdx[aLog[a].Gid] = _appendLog(aUserLog, &aEl)
         fallthrough
      case eAbPingFrom:
         aSvc.aliasIdx[aLog[a].Alias] = aLog[a].Uid
         _respondLog(aSvc.pingToIdx[aLog[a].Alias], &aLog[a])
         aUserLog := aSvc.pingFromIdx[aLog[a].Uid]
         aSvc.pingFromIdx[aLog[a].Uid] = _appendLog(aUserLog, &aLog[a])
      case eAbResolveFrom:
         _respondLog(aSvc.pingFromIdx[aLog[a].Uid], &aLog[a])
      case eAbResolveTo:
         aSvc.aliasIdx[aLog[a].Alias] = aLog[a].Uid
         _respondLog(aSvc.pingToIdx[aLog[a].Alias], &aLog[a])
      case eAbMsgAccept:
         aSvc.groupIdx[aLog[a].Gid] = tGroupEl{Gid:aLog[a].Gid, Date:aLog[a].Date}
         aSvc.aliasIdx[aLog[a].Gid] = aLog[a].Gid
         _respondLog(aSvc.inviteFromIdx[aLog[a].Gid], &aLog[a])
      case eAbMsgJoin:
         _respondLog(aSvc.inviteToIdx[aLog[a].Alias + "\x00" + aLog[a].Gid], &aLog[a])
      default:
         quit(tError(fmt.Sprintf("unexpected adrsbk type %d", aLog[a].Type)))
      }
   }
   return aSvc
}

func _appendLog(iLog tAdrsbkLog, iEl *tAdrsbkEl) tAdrsbkLog {
   if iLog != nil && iLog[0].Response != nil {
      iEl.Response = &kResponseNone
   }
   return append(iLog, iEl)
}

func _respondLog(iLog tAdrsbkLog, iEl *tAdrsbkEl) bool {
   if iLog == nil {
      return false
   }
   iEl.Response = &kResponseNone
   if iLog[0].Response != nil {
      return false
   }
   for a := range iLog {
      iLog[a].Response = iEl
   }
   return true
}

func GetGroupAdrsbk(iSvc string) []tGroupEl {
   aSvc := _loadAdrsbk(iSvc)
   aSvc.RLock()
   aList := make([]tGroupEl, 0, len(aSvc.groupIdx))
   for _, aV := range aSvc.groupIdx {
      aList = append(aList, aV)
   }
   aSvc.RUnlock()
   sort.Slice(aList, func(cA, cB int) bool { return aList[cA].Date > aList[cB].Date })
   return aList
}

func GetReceivedAdrsbk(iSvc string) interface{} {
   return _listLogs(iSvc, false)
}

func GetSentAdrsbk(iSvc string) interface{} {
   return _listLogs(iSvc, true)
}

func _listLogs(iSvc string, iTo bool) interface{} {
   type tAdrsbkElOut struct {
      // assume Response pointers are safe to read outside lock
      tAdrsbkEl
      ResponseInvt *tAdrsbkEl `json:",omitempty"`
      Queued bool             `json:",omitempty"`
   }
   aSvc := _loadAdrsbk(iSvc)
   aIdx := aSvc.pingFromIdx; if iTo { aIdx = aSvc.pingToIdx }
   aLog := make([]tAdrsbkElOut, 0, len(aIdx)) // min number of items
   var aEl *tAdrsbkEl
   var aOut tAdrsbkElOut
   var aInvt tAdrsbkLog
   var a int

   aSvc.RLock()
   for _, aSet := range aIdx {
      for _, aEl = range aSet {
         aOut = tAdrsbkElOut{tAdrsbkEl: *aEl}
         if aEl.Type == eAbInviteTo {
            aInvt = aSvc.inviteToIdx[aEl.Alias + "\x00" + aEl.Gid]
            aOut.ResponseInvt = aInvt[0].Response
         } else if aEl.Type == eAbInviteFrom {
            aInvt = aSvc.inviteFromIdx[aEl.Gid]
            aOut.ResponseInvt = aInvt[0].Response
            aOut.Qid = ""
            if aInvt[0].Response == nil {
               for a = range aInvt {
                  if hasQueue(iSvc, eSrecAccept, aInvt[a].Qid) {
                     aOut.Queued = true
                     break
                  }
               }
               if !aOut.Queued {
                  aOut.Qid = aInvt[0].Qid
               }
            }
         }
         aLog = append(aLog, aOut)
      }
   }
   aSvc.RUnlock()
   sort.Slice(aLog, func(cA, cB int) bool { return aLog[cA].Date > aLog[cB].Date })
   return aLog
}

//todo temporary until tmtp ohi includes alias
func lookupUidAdrsbk(iSvc string, iUid string) string {
   aSvc :=  _loadAdrsbk(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   for aAlias, aUid := range aSvc.aliasIdx {
      if aUid == iUid {
         return aAlias
      }
   }
   return "? "+ iUid[:5]
}

func LookupAdrsbk(iSvc string, iAlias string) string {
   aSvc :=  _loadAdrsbk(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aUid := aSvc.aliasIdx[iAlias]
   if aUid == "" || aUid == kUidUnknown {
      return ""
   }
   return aUid
}

type tSearchResult struct { name, id string }

func searchAdrsbk(iSvc string, iUpdt *Update) []string {
   aSvc := _loadAdrsbk(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aTerm := iUpdt.Adrsbk.Term
   var aResult []tSearchResult
   fMatch := func(cName, cId string) {
      if len(cName) < len(aTerm) {
         return
      }
      if strings.EqualFold(cName[:len(aTerm)], aTerm) {
         aResult = append(aResult, tSearchResult{cName, cId})
         return
      }
      for _, cPart := range strings.Fields(cName) { //todo better split logic; skip first item
         if len(cPart) >= len(aTerm) && strings.EqualFold(cPart[:len(aTerm)], aTerm) {
            aResult = append(aResult, tSearchResult{cName, cId})
            return
         }
      }
   }
   var aName, aUid string
   if iUpdt.Adrsbk.Type & 1 == 1 {
      for aName, aUid = range aSvc.aliasIdx {
         if aUid == kUidUnknown { continue }
         if _, ok := aSvc.groupIdx[aName]; !ok {
            fMatch(aName, aUid)
         }
      }
   }
   if iUpdt.Adrsbk.Type & 2 == 2 {
      for aName, _ = range aSvc.groupIdx {
         fMatch(aName, aName)
      }
   }
   sort.Slice(aResult, func(cA, cB int) bool { return aResult[cA].name < aResult[cB].name })
   aList := make([]string, 0, 2*len(aResult))
   for a := range aResult {
      aList = append(aList, aResult[a].name)
      aList = append(aList, aResult[a].id)
   }
   return aList
}

func storeSelfAdrsbk(iSvc string, iAlias string, iUid string) {
   aSvc := _loadAdrsbk(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aEl := tAdrsbkEl{Type:eAbSelf, Date:dateRFC3339(), MyAlias:iAlias, Uid:iUid}
   if aSvc.aliasIdx[aEl.MyAlias] != "" {
      fmt.Fprintf(os.Stderr, "storeSelfAdrsbk %s: MyAlias %s already stored\n", iSvc, aEl.MyAlias)
      return
   }
   aSvc.aliasIdx[aEl.MyAlias] = aEl.Uid
   _storeAdrsbk(iSvc, []tAdrsbkEl{aEl})
}

func patchSelfAdrsbk(iSvc string, iAlias string, iUid string) { // for WipeDataService()
   _storeAdrsbk(iSvc, []tAdrsbkEl{{Type:eAbSelf, Date:dateRFC3339(), MyAlias:iAlias, Uid:iUid}})
}

func storeReceivedAdrsbk(iSvc string, iHead *Header, iR io.Reader) error {
   aBuf := make([]byte, iHead.DataLen)
   _, err := iR.Read(aBuf)
   if err != nil {
      return err
   }
   aFromSelf := iHead.From == GetConfigService(iSvc).Uid
   aSvc := _loadAdrsbk(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aLog := aSvc.pingFromIdx[iHead.From]; if aFromSelf { aLog = aSvc.pingToIdx[iHead.To] }
   for a := range aLog {
      if aLog[a].MsgId == iHead.Id {
         fmt.Fprintf(os.Stderr, "storeReceivedAdrsbk %s: ping %s already stored\n", iSvc, iHead.Id)
         return nil
      }
   }
   if aFromSelf {
      aEl := tAdrsbkEl{Date:iHead.Posted, Gid:iHead.Gid, Text:string(aBuf),
                       Alias:iHead.To, MyAlias:iHead.Alias, MsgId:iHead.Id}
      _storeSentAdrsbk(iSvc, aSvc, &aEl, "")
      return nil
   }
   aUid := aSvc.aliasIdx[iHead.Alias]
   if aUid != "" && aUid != kUidUnknown && aUid != iHead.From {
      fmt.Fprintf(os.Stderr, "storeReceivedAdrsbk %s: blocked ping from %s aka %s\n",
                             iSvc, iHead.From, aUid)
      return nil
   }
   aEl := tAdrsbkEl{Date:iHead.Posted, Gid:iHead.Gid, Text:string(aBuf),
                    Alias:iHead.Alias, Uid:iHead.From, MyAlias:iHead.To, MsgId:iHead.Id}
   aEl.Type = eAbPingFrom; if iHead.Op == "invite" { aEl.Type = eAbInviteFrom }
   if aEl.Type == eAbInviteFrom {
      aEl.Qid = makeLocalId(iHead.Gid)
      aEl2 := aEl
      aSvc.inviteFromIdx[iHead.Gid] = _appendLog(aSvc.inviteFromIdx[iHead.Gid], &aEl2)
   }
   aSvc.aliasIdx[aEl.Alias] = aEl.Uid
   _respondLog(aSvc.pingToIdx[aEl.Alias], &aEl)
   aSvc.pingFromIdx[iHead.From] = _appendLog(aLog, &aEl)
   _storeAdrsbk(iSvc, []tAdrsbkEl{aEl})
   return nil
}

func storeSentAdrsbk(iSvc string, iKey string, iDate string, iQid string) {
   aSvc := _loadAdrsbk(iSvc)
   var aMap map[string]*tAdrsbkEl
   aSvc.draftDoor.RLock()
   err := readJsonFile(&aMap, filePing(iSvc))
   if err != nil { quit(err) }
   aSvc.draftDoor.RUnlock()
   aEl := aMap[iKey]
   if aEl == nil {
      fmt.Fprintf(os.Stderr, "storeSentAdrsbk %s: draft ping was cleared %s\n", iSvc, iKey)
      return
   }
   aSvc.Lock(); defer aSvc.Unlock()
   aEl.Date, aEl.Qid = iDate, ""
   _storeSentAdrsbk(iSvc, aSvc, aEl, iQid)
}

func _storeSentAdrsbk(iSvc string, iAbk *tAdrsbk, iEl *tAdrsbkEl, iQid string) {
   iEl.Type = eAbPingTo; if iEl.Gid != "" { iEl.Type = eAbInviteTo }
   if iEl.Type == eAbInviteTo {
      aEl2 := *iEl
      aKey := aEl2.Alias +"\x00"+ aEl2.Gid
      iAbk.inviteToIdx[aKey] = _appendLog(iAbk.inviteToIdx[aKey], &aEl2)
      if iAbk.groupIdx[aEl2.Gid].Gid == "" {
         iAbk.groupIdx[aEl2.Gid] = tGroupEl{Gid:aEl2.Gid, Date:aEl2.Date, Admin:true}
         iAbk.aliasIdx[aEl2.Gid] = aEl2.Gid
      }
   }
   aUid := iAbk.aliasIdx[iEl.Alias]
   if aUid == "" {
      iAbk.aliasIdx[iEl.Alias] = kUidUnknown
   } else if aUid != kUidUnknown {
      _respondLog(iAbk.pingFromIdx[aUid], iEl)
   }
   aLog := iAbk.pingToIdx[iEl.Alias]
   iAbk.pingToIdx[iEl.Alias] = _appendLog(aLog, iEl)
   _storeAdrsbkQid(iSvc, []tAdrsbkEl{*iEl}, iQid)
}

func resolveReceivedAdrsbk(iSvc string, iDate string, iCc []tCcEl, iTid string, iCcSelf *tCcEl) {
   if len(iCc) == 0 {
      return
   }
   aViaSelf := iCcSelf == nil
   aSvc := _loadAdrsbk(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aEls := make([]tAdrsbkEl, 0, 2*len(iCc))
   for a := range iCc {
      if aSvc.aliasIdx[iCc[a].Who] == "" {
         aSvc.aliasIdx[iCc[a].Who] = iCc[a].WhoUid
         if iCcSelf == nil {
            aCfg := GetConfigService(iSvc)
            for a1 := range iCc {
                if iCc[a1].WhoUid == aCfg.Uid {
                   iCcSelf = &iCc[a1]
                   break
                }
            }
         }
         aEl := tAdrsbkEl{Type:eAbPingFrom, Date:iDate, Text:"via ",
                          Alias:iCc[a].Who, Uid:iCc[a].WhoUid, MyAlias:iCcSelf.Who, MsgId:iTid}
         if aViaSelf { aEl.Text += iCcSelf.By } else { aEl.Text += iCc[a].By }
         _respondLog(aSvc.pingToIdx[iCc[a].Who], &aEl)
         aSvc.pingFromIdx[iCc[a].WhoUid] = _appendLog(aSvc.pingFromIdx[iCc[a].WhoUid], &aEl)
         aEls = append(aEls, aEl)
      }
      aEl := tAdrsbkEl{Type:eAbResolveFrom, Date:iDate, Tid:iTid, Uid:iCc[a].WhoUid}
      if _respondLog(aSvc.pingFromIdx[iCc[a].WhoUid], &aEl) {
         aEls = append(aEls, aEl)
      }
   }
   if len(aEls) > 0 {
      _storeAdrsbk(iSvc, aEls)
   }
}

func resolveSentAdrsbk(iSvc string, iDate string, iCc []tCcEl, iTid string) {
   if len(iCc) == 0 {
      return
   }
   aSvc := _loadAdrsbk(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aEls := make([]tAdrsbkEl, 0, len(iCc))
   for a := range iCc {
      aUid := aSvc.aliasIdx[iCc[a].Who]
      if aUid != kUidUnknown && aUid != iCc[a].WhoUid { continue }
      aEl := tAdrsbkEl{Type:eAbResolveTo, Date:iDate, Tid:iTid,
                       Uid:iCc[a].WhoUid, Alias:iCc[a].Who}
      if _respondLog(aSvc.pingToIdx[iCc[a].Who], &aEl) {
         aSvc.aliasIdx[iCc[a].Who] = iCc[a].WhoUid
         aEls = append(aEls, aEl)
      }
   }
   if len(aEls) > 0 {
      _storeAdrsbk(iSvc, aEls)
   }
}

func groupJoinedAdrsbk(iSvc string, iHead *Header) bool {
   aSvc := _loadAdrsbk(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   if iHead.From == GetConfigService(iSvc).Uid {
      aEl := tAdrsbkEl{Type:eAbMsgAccept, Date:iHead.Posted, Gid:iHead.Gid}
      if _respondLog(aSvc.inviteFromIdx[aEl.Gid], &aEl) {
         aSvc.groupIdx[aEl.Gid] = tGroupEl{Gid:aEl.Gid, Date:aEl.Date}
         aSvc.aliasIdx[aEl.Gid] = aEl.Gid
         _storeAdrsbk(iSvc, []tAdrsbkEl{aEl})
      }
      return true
   } else {
      aEl := tAdrsbkEl{Type:eAbMsgJoin, Date:iHead.Posted, Gid:iHead.Gid, Alias:iHead.Alias}
      if _respondLog(aSvc.inviteToIdx[aEl.Alias + "\x00" + aEl.Gid], &aEl) {
         _storeAdrsbk(iSvc, []tAdrsbkEl{aEl})
      }
      return false
   }
}

func _storeAdrsbk(iSvc string, iEls []tAdrsbkEl) { _storeAdrsbkQid(iSvc, iEls, "") }

func _storeAdrsbkQid(iSvc string, iEls []tAdrsbkEl, iQid string) {
   var err error
   aFi, err := os.Lstat(fileAdrs(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aPos := int64(2); if err == nil { aPos = aFi.Size() }
   aTempOk := ftmpAdrsbk(iSvc, fmt.Sprint(aPos), iQid)
   aTemp := aTempOk + ".tmp"

   for a, _ := range iEls {
      iEls[a].Response = nil
   }
   err = writeJsonFile(aTemp, iEls)
   if err != nil { quit(err) }
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(dirTemp(iSvc))
   if err != nil { quit(err) }
   _completeAdrsbk(iSvc, path.Base(aTempOk), iEls)
}

func _completeAdrsbk(iSvc string, iTmp string, iEls []tAdrsbkEl) {
   var err error
   switch iEls[0].Type {
   case eAbPingTo, eAbInviteTo:
      if iEls[0].MsgId == "" { //todo revise if MsgId added to storeSentAdrsbk()
         deleteDraftAdrsbk(iSvc, iEls[0].Alias, iEls[0].Gid)
      }
   case eAbPingFrom, eAbInviteFrom:
      addPingNotice(iSvc, iEls[0].MsgId, iEls[0].Alias, iEls[0].Gid, iEls[0].Text)
   }
   aRec := strings.SplitN(iTmp, "_", 3)
   aFd, err := os.OpenFile(fileAdrs(iSvc), os.O_WRONLY|os.O_CREATE, 0600)
   if err != nil { quit(err) }
   defer aFd.Close()
   aPos, err := strconv.ParseInt(aRec[1], 10, 64)
   if err != nil { quit(err) }
   if aPos != 2 {
      _, err = aFd.Seek(aPos-1, io.SeekStart)
      if err != nil { quit(err) }
   }
   aChar := byte('['); if aPos != 2 { aChar = ',' }
   aEnc := json.NewEncoder(aFd)
   for a, _ := range iEls {
      _, err = aFd.Write([]byte{aChar,'\n'})
      if err != nil { quit(err) }
      err = aEnc.Encode(iEls[a])
      if err != nil { quit(err) }
      aChar = ','
   }
   _, err = aFd.Write([]byte{']'})
   if err != nil { quit(err) }
   err = aFd.Sync()
   if err != nil { quit(err) }
   if aPos == 2 {
      err = syncDir(dirSvc(iSvc))
      if err != nil { quit(err) }
   }
   if aRec[2] != "" {
      aRec[2] = unescapeFile(aRec[2])
      dropQueue(iSvc, aRec[2])
   }
   err = os.Remove(dirTemp(iSvc) + iTmp)
   if err != nil { quit(err) }
}

func completeAdrsbk(iSvc string, iTmp string) {
   if strings.HasSuffix(iTmp, ".tmp") {
      os.Remove(dirTemp(iSvc) + iTmp)
      return
   }
   fmt.Println("complete " + iTmp)
   var aEls []tAdrsbkEl
   err := readJsonFile(&aEls, dirTemp(iSvc) + iTmp)
   if err != nil { quit(err) }
   _completeAdrsbk(iSvc, iTmp, aEls)
}

func GetDraftAdrsbk(iSvc string) interface{} {
   type tAdrsbkElOut struct {
      tAdrsbkEl
      Text string // hides tAdrsbkEl.Text
      Queued bool `json:",omitempty"`
   }
   var aMap map[string]*tAdrsbkElOut
   aDoor := &getService(iSvc).adrsbk.draftDoor
   aDoor.RLock()
   err := readJsonFile(&aMap, filePing(iSvc))
   aDoor.RUnlock()
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      return []*tAdrsbkElOut{}
   }
   aList := make([]*tAdrsbkElOut, 0, len(aMap))
   for _, aEl := range aMap {
      aEl.Queued = hasQueue(iSvc, eSrecPing, aEl.Qid)
      aList = append(aList, aEl)
   }
   sort.Slice(aList, func(cA, cB int) bool {
      if aList[cA].Alias == aList[cB].Alias {
         return aList[cA].Gid < aList[cB].Gid
      }
      return aList[cA].Alias < aList[cB].Alias
   })
   return aList
}

func sendJoinGroupAdrsbk(iW io.Writer, iSvc string, iQid, iId string) error {
   aId := parseLocalId(iQid)
   aSvc := _loadAdrsbk(iSvc)
   aSvc.RLock()
   _, ok := aSvc.groupIdx[aId.gid()]
   aSvc.RUnlock()
   if ok {
      fmt.Fprintf(os.Stderr, "sendJoinGroupAdrsbk %s: already joined group %s\n", iSvc, aId.gid())
      return tError("already sent")
   }
   var err error
   aHead, err := json.Marshal(Msg{"Op":6, "Id":iId, "Act":"join", "Gid":aId.gid()})
   if err != nil { quit(err) }
   err = writeHeaders(iW, aHead, nil)
   return err
}

func sendDraftAdrsbk(iW io.Writer, iSvc string, iQid, iId string) error {
   aDoor := &getService(iSvc).adrsbk.draftDoor
   var err error
   var aMap map[string]*tAdrsbkEl
   aDoor.RLock()
   err = readJsonFile(&aMap, filePing(iSvc))
   aDoor.RUnlock()
   if err != nil { quit(err) }
   aId := parseLocalId(iQid)
   aEl := aMap[aId.ping()]
   if aEl == nil {
      fmt.Fprintf(os.Stderr, "sendDraftAdrsbk %s: ping draft was cleared %s\n", iSvc, iQid)
      return tError("already sent")
   }
   aData := []byte(aEl.Text)
   aMsg := Msg{"Op":9, "Id":iId, "To":aEl.Alias, "From":aEl.MyAlias, "DataLen": len(aData)}
   if aEl.Gid != "" {
      aMsg["Op"] = 5
      aMsg["Gid"] = aEl.Gid
   }
   aHead, err := json.Marshal(aMsg)
   if err != nil { quit(err) }
   err = writeHeaders(iW, aHead, nil)
   if err != nil { return err }
   _, err = iW.Write(aData)
   return err
}

func storeDraftAdrsbk(iSvc string, iUpdt *Update) {
   aDoor := &getService(iSvc).adrsbk.draftDoor
   aDoor.Lock(); defer aDoor.Unlock()
   var err error
   aMap := make(map[string]*tAdrsbkEl)
   err = readJsonFile(&aMap, filePing(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aKey := iUpdt.Ping.To + "\x00" + iUpdt.Ping.Gid
   aMap[aKey] = &tAdrsbkEl{Type:eAbPingDraft, Date:dateRFC3339(), Text:iUpdt.Ping.Text,
                           Alias:iUpdt.Ping.To, MyAlias:iUpdt.Ping.Alias, Gid:iUpdt.Ping.Gid,
                           Qid:makeLocalId(aKey)}
   err = storeFile(filePing(iSvc), aMap)
   if err != nil { quit(err) }
}

func deleteDraftAdrsbk(iSvc string, iAlias, iGid string) {
   aDoor := &getService(iSvc).adrsbk.draftDoor
   aDoor.Lock(); defer aDoor.Unlock()
   var err error
   var aMap map[string]*tAdrsbkEl
   err = readJsonFile(&aMap, filePing(iSvc))
   if err != nil { quit(err) }
   aKey := iAlias + "\x00" + iGid
   if aMap[aKey] == nil {
      return
   }
   delete(aMap, aKey)
   err = storeFile(filePing(iSvc), aMap)
   if err != nil { quit(err) }
}

