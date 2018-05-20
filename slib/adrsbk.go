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
   "io"
   "io/ioutil"
   "encoding/json"
   "os"
   "path"
   "sort"
   "strconv"
   "strings"
   "sync"
)

const kUidUnknown = "\x00unknown"

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
   Type int8
   Date string
   Text string         `json:",omitempty"`
   Alias string        `json:",omitempty"`
   Uid string          `json:",omitempty"`
   MyAlias string      `json:",omitempty"`
   MsgId string        `json:",omitempty"`
   Tid string          `json:",omitempty"`
   Gid string          `json:",omitempty"`
   Response *tAdrsbkEl `json:",omitempty"` // not stored
   Qid string          `json:",omitempty"`
   Queued bool         `json:",omitempty"` // not stored
}

const (
   _ int8 = iota
   eAbPingDraft     // Type, Date, Text, Alias,      MyAlias,                 Qid
   eAbNone //todo remove
   eAbPingTo        // Type, Date, Text, Alias,      MyAlias
   eAbPingFrom      // Type, Date, Text, Alias, Uid, MyAlias, MsgId
   eAbMsgTo         // Type, Date,              Uid,          MsgId, Tid
   eAbMsgFrom       // Type, Date,       Alias, Uid,          MsgId, Tid
   eAbInviteTo      // Type, Date, Text, Alias,      MyAlias,            Gid
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
   aSvc := &GetService(iSvc).adrsbk
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
   err := readJsonFile(&aLog, adrsFile(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   for a, _ := range aLog {
      switch aLog[a].Type {
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
      case eAbMsgTo:
         _respondLog(aSvc.pingFromIdx[aLog[a].Uid], &aLog[a])
      case eAbMsgFrom:
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
   if iLog != nil {
      iEl.Response = iLog[0].Response
   }
   return append(iLog, iEl)
}

func _respondLog(iLog tAdrsbkLog, iEl *tAdrsbkEl) bool {
   if iLog == nil || iLog[0].Response != nil {
      return false
   }
   for a, _ := range iLog {
      iLog[a].Response = iEl
   }
   return true
}

func GetGroupAdrsbk(iSvc string) []tGroupEl {
   aSvc := _loadAdrsbk(iSvc)
   aSvc.RLock()
   aList := make([]tGroupEl, len(aSvc.groupIdx))
   a := 0
   for _, aList[a] = range aSvc.groupIdx { a++ }
   aSvc.RUnlock()
   sort.Slice(aList, func(cA, cB int) bool { return aList[cA].Date > aList[cB].Date })
   return aList
}

func GetReceivedAdrsbk(iSvc string) []tAdrsbkEl {
   aSvc := _loadAdrsbk(iSvc)
   return _listLogs(aSvc, aSvc.pingFromIdx)
}

func GetSentAdrsbk(iSvc string) []tAdrsbkEl {
   aSvc := _loadAdrsbk(iSvc)
   return _listLogs(aSvc, aSvc.pingToIdx)
}

func GetInviteFromAdrsbk(iSvc string) []tAdrsbkEl {
   aSvc := _loadAdrsbk(iSvc)
   aLog := _listLogs(aSvc, aSvc.inviteFromIdx)
   for a, _ := range aLog {
      aLog[a].Queued = queueHasService(iSvc, eSrecAccept, aLog[a].Qid)
   }
   return aLog
}

func GetInviteToAdrsbk(iSvc string) []tAdrsbkEl {
   aSvc := _loadAdrsbk(iSvc)
   return _listLogs(aSvc, aSvc.inviteToIdx)
}

func _listLogs(iSvc *tAdrsbk, iIdx map[string]tAdrsbkLog) []tAdrsbkEl {
   iSvc.RLock()
   aLog := make([]tAdrsbkEl, 0, len(iIdx))
   for _, aSet := range iIdx {
      for _, aEl := range aSet {
         aLog = append(aLog, *aEl)
      }
   }
   iSvc.RUnlock()
   sort.Slice(aLog, func(cA, cB int) bool { return aLog[cA].Date > aLog[cB].Date })
   return aLog
}

func lookupAdrsbk(iSvc string, iAlias []string) []tHeaderFor {
   aSvc :=  _loadAdrsbk(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aFor := make([]tHeaderFor, len(iAlias))
   for a, _ := range iAlias {
      aUid := aSvc.aliasIdx[iAlias[a]]
      if aUid != "" && aUid != kUidUnknown {
         aType := eForUser; if aUid == iAlias[a] { aType = eForGroupExcl }
         aFor[a] = tHeaderFor{Id:aUid, Type:aType}
      }
   }
   return aFor
}

func storeReceivedAdrsbk(iSvc string, iHead *Header, iR io.Reader) error {
   var err error
   aSvc := _loadAdrsbk(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aLog := aSvc.pingFromIdx[iHead.From]
   for a, _ := range aLog {
      if aLog[a].MsgId == iHead.Id {
         fmt.Fprintf(os.Stderr, "storeReceivedAdrsbk %s: ping %s already stored\n", iSvc, iHead.Id)
         _, err = io.CopyN(ioutil.Discard, iR, iHead.DataLen)
         return err
      }
   }
   aUid := aSvc.aliasIdx[iHead.SubHead.Alias]
   if aUid != "" && aUid != kUidUnknown && aUid != iHead.From {
      fmt.Fprintf(os.Stderr, "storeReceivedAdrsbk %s: ping from %s blocked\n", iSvc, iHead.From)
      _, err = io.CopyN(ioutil.Discard, iR, iHead.DataLen)
      return err
   }
   aBuf := make([]byte, iHead.DataLen)
   _, err = iR.Read(aBuf)
   if err != nil { return err }
   aType := eAbPingFrom; if iHead.Op == "invite" { aType = eAbInviteFrom }
   aEl := tAdrsbkEl{Type:aType, Date:iHead.Posted, Gid:iHead.Gid, Text:string(aBuf),
                    Alias:iHead.SubHead.Alias, Uid:iHead.From, MyAlias:iHead.To, MsgId:iHead.Id}
   if aEl.Type == eAbInviteFrom {
      aEl.Qid = makeLocalId(iHead.Gid)
      aEl2 := aEl
      aSvc.inviteFromIdx[iHead.Gid] = _appendLog(aSvc.inviteFromIdx[iHead.Gid], &aEl2)
   }
   aSvc.aliasIdx[aEl.Alias] = aEl.Uid
   _respondLog(aSvc.pingToIdx[aEl.Alias], &aEl)
   aSvc.pingFromIdx[iHead.From] = _appendLog(aLog, &aEl)
   _storeAdrsbk(iSvc, []tAdrsbkEl{aEl}, false)
   return nil
}

func storeSentAdrsbk(iSvc string, iKey string, iDate string) {
   aSvc := _loadAdrsbk(iSvc)
   var aMap map[string]*tAdrsbkEl
   aSvc.draftDoor.RLock()
   err := readJsonFile(&aMap, pingFile(iSvc))
   aSvc.draftDoor.RUnlock()
   if err != nil { quit(err) }
   aEl := aMap[iKey]
   if aEl == nil {
      fmt.Fprintf(os.Stderr, "storeSentAdrsbk %s: draft ping was cleared %s\n", iSvc, iKey)
      return
   }
   aSvc.Lock(); defer aSvc.Unlock()
   aLog := aSvc.pingToIdx[aEl.Alias]
   aEl.Type = eAbPingTo; if aEl.Gid != "" { aEl.Type = eAbInviteTo }
   aEl.Date = iDate
   if aEl.Type == eAbInviteTo {
      aEl2 := *aEl
      aSvc.inviteToIdx[iKey] = _appendLog(aSvc.inviteToIdx[iKey], &aEl2)
      if aSvc.groupIdx[aEl2.Gid].Gid == "" {
         aSvc.groupIdx[aEl2.Gid] = tGroupEl{Gid:aEl2.Gid, Date:aEl2.Date, Admin:true}
         aSvc.aliasIdx[aEl2.Gid] = aEl2.Gid
      }
   }
   aUid := aSvc.aliasIdx[aEl.Alias]
   if aUid == "" {
      aSvc.aliasIdx[aEl.Alias] = kUidUnknown
   } else if aUid != kUidUnknown {
      _respondLog(aSvc.pingFromIdx[aUid], aEl)
   }
   aSvc.pingToIdx[aEl.Alias] = _appendLog(aLog, aEl)
   _storeAdrsbk(iSvc, []tAdrsbkEl{*aEl}, true)
}

func resolveReceivedAdrsbk(iSvc string, iDate string, iFor []tHeaderFor, iTid, iMsgId string) {
   aSvc := _loadAdrsbk(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   var aEls []tAdrsbkEl
   for a, _ := range iFor {
      aEl := tAdrsbkEl{Type:eAbMsgTo, Date:iDate, Tid:iTid, MsgId:iMsgId, Uid:iFor[a].Id}
      if _respondLog(aSvc.pingFromIdx[iFor[a].Id], &aEl) {
         aEls = append(aEls, aEl)
      }
   }
   if len(aEls) > 0 {
      _storeAdrsbk(iSvc, aEls, false)
   }
}

func resolveSentAdrsbk(iSvc string, iDate string, iFrom, iAlias string, iTid, iMsgId string) {
   if iAlias == "" {
      return
   }
   aSvc := _loadAdrsbk(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aUid := aSvc.aliasIdx[iAlias]
   if aUid != kUidUnknown && aUid != iFrom {
      return
   }
   aEl := tAdrsbkEl{Type:eAbMsgFrom, Date:iDate, Tid:iTid, MsgId:iMsgId,
                    Uid:iFrom, Alias:iAlias}
   if _respondLog(aSvc.pingToIdx[iAlias], &aEl) {
      aSvc.aliasIdx[iAlias] = iFrom
      _storeAdrsbk(iSvc, []tAdrsbkEl{aEl}, false)
   }
}

func acceptInviteAdrsbk(iSvc string, iGid string, iDate string) {
   aSvc := _loadAdrsbk(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aEl := tAdrsbkEl{Type:eAbMsgAccept, Date:iDate, Gid:iGid}
   if _respondLog(aSvc.inviteFromIdx[iGid], &aEl) {
      aSvc.groupIdx[iGid] = tGroupEl{Gid:iGid, Date:aEl.Date}
      aSvc.aliasIdx[iGid] = iGid
      _storeAdrsbk(iSvc, []tAdrsbkEl{aEl}, false)
   }
}

func groupJoinedAdrsbk(iSvc string, iHead *Header) {
   aSvc := _loadAdrsbk(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aEl := tAdrsbkEl{Type:eAbMsgJoin, Date:iHead.Posted, Gid:iHead.Gid, Alias:iHead.Alias}
   if _respondLog(aSvc.inviteToIdx[aEl.Alias + "\x00" + aEl.Gid], &aEl) {
      _storeAdrsbk(iSvc, []tAdrsbkEl{aEl}, false)
   }
}

func _storeAdrsbk(iSvc string, iEls []tAdrsbkEl, iSent bool) {
   var err error
   aFi, err := os.Lstat(adrsFile(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aPos := int64(2); if err == nil { aPos = aFi.Size() }
   aTempOk := tempDir(iSvc) + fmt.Sprintf("adrsbk_%d_", aPos)
   if iSent {
      aTempOk += "sent"
   }
   aTemp := aTempOk + ".tmp"

   for a, _ := range iEls {
      iEls[a].Response = nil
   }
   err = writeJsonFile(aTemp, iEls)
   if err != nil { quit(err) }
   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(tempDir(iSvc))
   if err != nil { quit(err) }
   _completeAdrsbk(iSvc, path.Base(aTempOk), iEls)
}

func _completeAdrsbk(iSvc string, iTmp string, iEls []tAdrsbkEl) {
   var err error
   aRec := strings.SplitN(iTmp, "_", 3)
   if aRec[2] == "sent" {
      deleteDraftAdrsbk(iSvc, iEls[0].Alias, iEls[0].Gid) // when sent, len(iEls)==1
   }
   aFd, err := os.OpenFile(adrsFile(iSvc), os.O_WRONLY|os.O_CREATE, 0600)
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
      err = syncDir(svcDir(iSvc))
      if err != nil { quit(err) }
   }
   err = os.Remove(tempDir(iSvc) + iTmp)
   if err != nil { quit(err) }
}

func completeAdrsbk(iSvc string, iTmp string) {
   if strings.HasSuffix(iTmp, ".tmp") {
      os.Remove(tempDir(iSvc) + iTmp)
      return
   }
   fmt.Println("complete " + iTmp)
   var aEls []tAdrsbkEl
   err := readJsonFile(&aEls, tempDir(iSvc) + iTmp)
   if err != nil { quit(err) }
   _completeAdrsbk(iSvc, iTmp, aEls)
}

func GetDraftAdrsbk(iSvc string) tAdrsbkLog {
   aDoor := &GetService(iSvc).adrsbk.draftDoor
   var aMap map[string]*tAdrsbkEl
   aDoor.RLock()
   err := readJsonFile(&aMap, pingFile(iSvc))
   aDoor.RUnlock()
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      return tAdrsbkLog{}
   }
   aList := make(tAdrsbkLog, len(aMap))
   a := 0
   for _, aList[a] = range aMap {
      aList[a].Queued = queueHasService(iSvc, eSrecPing, aList[a].Qid)
      a++
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
   err = sendHeaders(iW, aHead, nil)
   return err
}

func sendDraftAdrsbk(iW io.Writer, iSvc string, iQid, iId string) error {
   aDoor := &GetService(iSvc).adrsbk.draftDoor
   var err error
   var aMap map[string]*tAdrsbkEl
   aDoor.RLock()
   err = readJsonFile(&aMap, pingFile(iSvc))
   aDoor.RUnlock()
   if err != nil { quit(err) }
   aId := parseLocalId(iQid)
   aEl := aMap[aId.ping()]
   if aEl == nil {
      fmt.Fprintf(os.Stderr, "sendDraftAdrsbk %s: ping draft was cleared %s\n", iSvc, iQid)
      return tError("already sent")
   }
   aSubh, err := json.Marshal(Msg{"Alias":aEl.MyAlias}) //todo drop when ping takes from:
   if err != nil { quit(err) }
   aData := []byte(aEl.Text)
   aMsg := Msg{"Op":8, "Id":iId, "To":aEl.Alias, "From":aEl.MyAlias,
               "DataHead":len(aSubh), "DataLen": len(aSubh) + len(aData)}
   if aEl.Gid != "" {
      aMsg["Op"] = 5
      aMsg["Gid"] = aEl.Gid
   }
   aHead, err := json.Marshal(aMsg)
   if err != nil { quit(err) }
   err = sendHeaders(iW, aHead, aSubh)
   if err != nil { return err }
   _, err = iW.Write(aData)
   return err
}

func storeDraftAdrsbk(iSvc string, iUpdt *Update) {
   aDoor := &GetService(iSvc).adrsbk.draftDoor
   aDoor.Lock(); defer aDoor.Unlock()
   var err error
   aMap := make(map[string]*tAdrsbkEl)
   err = readJsonFile(&aMap, pingFile(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aKey := iUpdt.Ping.To + "\x00" + iUpdt.Ping.Gid
   aMap[aKey] = &tAdrsbkEl{Type:eAbPingDraft, Date:dateRFC3339(), Text:iUpdt.Ping.Text,
                           Alias:iUpdt.Ping.To, MyAlias:iUpdt.Ping.Alias, Gid:iUpdt.Ping.Gid,
                           Qid:makeLocalId(aKey)}
   err = storeFile(pingFile(iSvc), aMap)
   if err != nil { quit(err) }
}

func deleteDraftAdrsbk(iSvc string, iAlias, iGid string) {
   aDoor := &GetService(iSvc).adrsbk.draftDoor
   aDoor.Lock(); defer aDoor.Unlock()
   var err error
   var aMap map[string]*tAdrsbkEl
   err = readJsonFile(&aMap, pingFile(iSvc))
   if err != nil { quit(err) }
   aKey := iAlias + "\x00" + iGid
   if aMap[aKey] == nil {
      return
   }
   delete(aMap, aKey)
   err = storeFile(pingFile(iSvc), aMap)
   if err != nil { quit(err) }
}

