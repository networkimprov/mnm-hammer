// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package slib

import (
   "io"
   "encoding/json"
   "os"
   "sort"
)


type tOhi map[string]string // key uid, value date

type tOhiEl struct {
   Uid string
   Date string
}

type tForOhi []struct { Id string }

func _listOhi(iMap tOhi) []tOhiEl {
   aList := make([]tOhiEl, len(iMap))
   a := 0
   for aList[a].Uid, aList[a].Date = range iMap { a++ }
   sort.Slice(aList, func(cA, cB int) bool { return aList[cA].Date > aList[cB].Date })
   return aList
}

func GetFromOhi(iSvc string) []tOhiEl {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   if aSvc.fromOhi == nil {
      return []tOhiEl{}
   }
   return _listOhi(aSvc.fromOhi)
}

func setFromOhi(iSvc string, iHead *Header) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aSvc.fromOhi = tOhi{}
   aDate := dateRFC3339()
   for _, aUid := range iHead.Ohi {
      aSvc.fromOhi[aUid] = aDate
   }
}

func updateFromOhi(iSvc string, iHead *Header) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   if iHead.Status == 1 {
      aSvc.fromOhi[iHead.From] = dateRFC3339()
   } else {
      delete(aSvc.fromOhi, iHead.From)
   }
}

func dropFromOhi(iSvc string) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aSvc.fromOhi = nil
}

func GetIdxOhi(iSvc string) []tOhiEl {
   var aMap tOhi
   aSvc := getService(iSvc)
   aSvc.RLock()
   err := readJsonFile(&aMap, ohiFile(iSvc))
   aSvc.RUnlock()
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      return []tOhiEl{}
   }
   return _listOhi(aMap)
}

func SendAllOhi(iW io.Writer, iSvc string, iId string) error {
   var err error
   aMap := tOhi{}
   aSvc := getService(iSvc)
   aSvc.RLock()
   err = readJsonFile(&aMap, ohiFile(iSvc))
   aSvc.RUnlock()
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aFor := make(tForOhi, len(aMap))
   a := 0
   for aFor[a].Id, _ = range aMap { a++ }
   if a == 0 {
      return nil
   }
   aHead, err := json.Marshal(Msg{"Op":4, "Id":iId, "For":aFor, "Type":"add"})
   if err != nil { quit(err) }
   err = sendHeaders(iW, aHead, nil)
   return err
}

func editOhi(iSvc string, iUpdt *Update) *SendRecord {
   var err error
   aUid := iUpdt.Ohi.Uid
   if aUid == "" {
      aUid = lookupAdrsbk(iSvc, []string{iUpdt.Ohi.Alias})[0].Id //todo drop this
      if aUid == "" { quit(tError("missing Uid")) }
   }
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aMap := tOhi{}
   err = readJsonFile(&aMap, ohiFile(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   var aOp string
   if iUpdt.Op == "ohi_add" {
      aOp = "+"
      aMap[aUid] = dateRFC3339()
   } else {
      aOp = "-"
      delete(aMap, aUid)
   }
   err = storeFile(ohiFile(iSvc), aMap)
   if err != nil { quit(err) }
   return &SendRecord{Id: string(eSrecOhi) + aOp + makeLocalId(aUid)}
}

func sendEditOhi(iW io.Writer, iSvc string, iQid, iId string) error {
   var err error
   aId := parseLocalId(iQid)
   aFor := tForOhi{{Id:aId.ohi()[1:]}}
   aType := "add"; if aId.ohi()[0] == '-' { aType = "drop" }
   aHead, err := json.Marshal(Msg{"Op":4, "Id":iId, "For":aFor, "Type":aType})
   if err != nil { quit(err) }
   err = sendHeaders(iW, aHead, nil)
   return err
}


