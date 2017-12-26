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

var sSvcAdrsbkDoor sync.RWMutex
var sSvcAdrsbk = make(map[string]tAdrsbk) // key service

type tAdrsbk struct {
   pingToIdx   map[string]tAdrsbkLog // key alias
   pingFromIdx map[string]tAdrsbkLog // key uid
   aliasIdx    map[string]string     // key alias, value uid //todo replace with btree
}

type tAdrsbkLog []*tAdrsbkEl

type tAdrsbkEl struct {
   Type int8
   Date string
   Text_Tid string                         // Tid if eAbMsg*
   Alias string        `json:",omitempty"` // not eAbMsgTo
   Uid string          `json:",omitempty"` // not eAbPingSaved, eAbPingTo
   MyAlias string      `json:",omitempty"` // not eAbMsg*
   MsgId string        `json:",omitempty"` // not eAbPingSaved, eAbPingTo
   Response *tAdrsbkEl `json:",omitempty"` // not stored
}

const ( eAbPingSaved int8 = iota; eAbPingQueued; eAbPingTo; eAbPingFrom; eAbMsgTo; eAbMsgFrom )


func _getAliasIdx(iSvc string) map[string]string {
   sSvcAdrsbkDoor.RLock(); defer sSvcAdrsbkDoor.RUnlock()
   return sSvcAdrsbk[iSvc].aliasIdx
}

func _loadAdrsbk(iSvc string) tAdrsbk {
   sSvcAdrsbkDoor.Lock()
   aSvc := sSvcAdrsbk[iSvc]
   if aSvc.aliasIdx != nil {
      sSvcAdrsbkDoor.Unlock()
      return aSvc
   }
   aSvc.pingToIdx   = make(map[string]tAdrsbkLog)
   aSvc.pingFromIdx = make(map[string]tAdrsbkLog)
   aSvc.aliasIdx    = make(map[string]string)
   sSvcAdrsbk[iSvc] = aSvc
   sSvcAdrsbkDoor.Unlock()

   var aLog []tAdrsbkEl
   err := readJsonFile(&aLog, adrsFile(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   for a, _ := range aLog {
      switch aLog[a].Type {
      case eAbPingTo:
         aUid := aSvc.aliasIdx[aLog[a].Alias]
         if aUid == "" {
            aSvc.aliasIdx[aLog[a].Alias] = "unknown"
         } else if aUid != "unknown" {
            _respondLog(aSvc.pingFromIdx[aUid], &aLog[a])
         }
         aUserLog := aSvc.pingToIdx[aLog[a].Alias]
         aSvc.pingToIdx[aLog[a].Alias] = _appendLog(aUserLog, &aLog[a])
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

func GetReceivedAdrsbk(iSvc string) tAdrsbkLog {
   return _listPings(_loadAdrsbk(iSvc).pingFromIdx)
}

func GetSentAdrsbk(iSvc string) tAdrsbkLog {
   return _listPings(_loadAdrsbk(iSvc).pingToIdx)
}

func _listPings(iIdx map[string]tAdrsbkLog) tAdrsbkLog {
   aLog := tAdrsbkLog{}
   for _, aSet := range iIdx {
      for _, aEl := range aSet {
         aLog = append(aLog, aEl)
      }
   }
   sort.Slice(aLog, func(cA, cB int) bool { return aLog[cA].Date > aLog[cB].Date })
   return aLog
}

func lookupAdrsbk(iSvc string, iAlias []string) []tHeaderFor {
   aSvc :=  _loadAdrsbk(iSvc)
   aFor := make([]tHeaderFor, len(iAlias))
   for a, _ := range iAlias {
      aUid := aSvc.aliasIdx[iAlias[a]]
      if aUid != "" && aUid != "unknown" {
         aFor[a] = tHeaderFor{Id:aUid, Type:eForUser}
      }
   }
   return aFor
}

func storeReceivedAdrsbk(iSvc string, iHead *Header, iR io.Reader) error {
   var err error
   aSvc := _loadAdrsbk(iSvc)
   aLog := aSvc.pingFromIdx[iHead.From]
   for a, _ := range aLog {
      if aLog[a].MsgId == iHead.Id {
         fmt.Fprintf(os.Stderr, "storeReceivedAdrsbk %s: ping %s already stored\n", iSvc, iHead.Id)
         _, err = io.CopyN(ioutil.Discard, iR, iHead.DataLen)
         return err
      }
   }
   aUid := aSvc.aliasIdx[iHead.SubHead.Alias]
   if aUid != "" && aUid != "unknown" && aUid != iHead.From {
      fmt.Fprintf(os.Stderr, "storeReceivedAdrsbk %s: ping from %s blocked\n", iSvc, iHead.From)
      _, err = io.CopyN(ioutil.Discard, iR, iHead.DataLen)
      return err
   }
   aBuf := make([]byte, iHead.DataLen)
   _, err = iR.Read(aBuf)
   if err != nil { return err }
   aEl := tAdrsbkEl{Type:eAbPingFrom, Date:dateRFC3339(), Text_Tid:string(aBuf),
                    Alias:iHead.SubHead.Alias, Uid:iHead.From, MyAlias:iHead.To, MsgId:iHead.Id}
   aSvc.aliasIdx[aEl.Alias] = aEl.Uid
   _respondLog(aSvc.pingToIdx[aEl.Alias], &aEl)
   aSvc.pingFromIdx[iHead.From] = _appendLog(aLog, &aEl)
   _storeAdrsbk(iSvc, []tAdrsbkEl{aEl}, false)
   return nil
}

func storeSentAdrsbk(iSvc string, iAlias string) {
   var err error
   var aMap map[string]*tAdrsbkEl
   err = readJsonFile(&aMap, pingFile(iSvc))
   if err != nil { quit(err) }
   aEl := aMap[iAlias]
   aSvc := _loadAdrsbk(iSvc)
   aLog := aSvc.pingToIdx[iAlias]
   aEl.Type = eAbPingTo
   aEl.Date = dateRFC3339()
   aUid := aSvc.aliasIdx[iAlias]
   if aUid == "" {
      aSvc.aliasIdx[iAlias] = "unknown"
   } else if aUid != "unknown" {
      _respondLog(aSvc.pingFromIdx[aUid], aEl)
   }
   aSvc.pingToIdx[iAlias] = _appendLog(aLog, aEl)
   _storeAdrsbk(iSvc, []tAdrsbkEl{*aEl}, true)
}

func resolveReceivedAdrsbk(iSvc string, iFor []tHeaderFor, iTid, iMsgId string) {
   aSvc := _loadAdrsbk(iSvc)
   var aEls []tAdrsbkEl
   for a, _ := range iFor {
      aEl := tAdrsbkEl{Type:eAbMsgTo, Date:dateRFC3339(), Text_Tid:iTid, MsgId:iMsgId, Uid:iFor[a].Id}
      if _respondLog(aSvc.pingFromIdx[iFor[a].Id], &aEl) {
         aEls = append(aEls, aEl)
      }
   }
   if len(aEls) > 0 {
      _storeAdrsbk(iSvc, aEls, false)
   }
}

func resolveSentAdrsbk(iSvc string, iFrom, iAlias string, iTid, iMsgId string) {
   if iAlias == "" {
      return
   }
   aSvc := _loadAdrsbk(iSvc)
   aUid := aSvc.aliasIdx[iAlias]
   if aUid != "unknown" && aUid != iFrom {
      return
   }
   aEl := tAdrsbkEl{Type:eAbMsgFrom, Date:dateRFC3339(), Text_Tid:iTid, MsgId:iMsgId,
                    Uid:iFrom, Alias:iAlias}
   aSvc.aliasIdx[iAlias] = iFrom
   if _respondLog(aSvc.pingToIdx[iAlias], &aEl) {
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
      deleteSavedAdrsbk(iSvc, iEls[0].Alias) // when sent, len(iEls)==1
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

func GetSavedAdrsbk(iSvc string) tAdrsbkLog {
   var aMap map[string]*tAdrsbkEl
   err := readJsonFile(&aMap, pingFile(iSvc))
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      return tAdrsbkLog{}
   }
   aList := make(tAdrsbkLog, len(aMap))
   a := 0
   for _, aList[a] = range aMap { a++ }
   sort.Slice(aList, func(cA, cB int) bool { return aList[cA].Date > aList[cB].Date })
   return aList
}

//todo update .Type on queue for send

func sendSavedAdrsbk(iW io.Writer, iSvc string, iSaveId, iId string) error {
   var err error
   var aMap map[string]*tAdrsbkEl
   err = readJsonFile(&aMap, pingFile(iSvc))
   if err != nil { quit(err) }
   aId := parseSaveId(iSaveId)
   aEl := aMap[aId.alias()]
   aSubh, err := json.Marshal(Msg{"Alias":aEl.MyAlias})
   if err != nil { quit(err) }
   aData := []byte(aEl.Text_Tid)
   aHead, err := json.Marshal(Msg{"Op":8, "Id":iId, "To":aEl.Alias,
                                  "DataHead":len(aSubh), "DataLen": len(aSubh) + len(aData)})
   if err != nil { quit(err) }
   aLen := []byte(fmt.Sprintf("%04x", len(aHead)))
   if len(aLen) > 4 { quit(tError(fmt.Sprintf("header too long: %s %s", iSvc, iSaveId))) }

   _, err = iW.Write(aLen)
   if err != nil { return err }
   _, err = iW.Write(aHead)
   if err != nil { return err }
   _, err = iW.Write(aSubh)
   if err != nil { return err }
   _, err = iW.Write(aData)
   return err
}

func storeSavedAdrsbk(iSvc string, iUpdt *Update) {
   var err error
   aMap := make(map[string]*tAdrsbkEl)
   err = readJsonFile(&aMap, pingFile(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aMap[iUpdt.Ping.To] = &tAdrsbkEl{Type:eAbPingSaved, Date:dateRFC3339(), Text_Tid:iUpdt.Ping.Text,
                                    Alias:iUpdt.Ping.To, MyAlias:iUpdt.Ping.Alias}
   err = storeFile(pingFile(iSvc), aMap)
   if err != nil { quit(err) }
}

func deleteSavedAdrsbk(iSvc string, iAlias string) {
   var err error
   var aMap map[string]*tAdrsbkEl
   err = readJsonFile(&aMap, pingFile(iSvc))
   if err != nil { quit(err) }
   if aMap[iAlias] == nil {
      return
   }
   delete(aMap, iAlias)
   err = storeFile(pingFile(iSvc), aMap)
   if err != nil { quit(err) }
}

