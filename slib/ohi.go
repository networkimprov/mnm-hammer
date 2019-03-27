// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "io"
   "encoding/json"
   "os"
   "sort"
)


type tOhi map[string]tOhiEl // key uid

type tOhiEl struct {
   Date string
   Uid string `json:",omitempty"`
   Alias string
}

type tForOhi []struct { Id string }

func _listOhi(iSvc string, iMap tOhi) []tOhiEl {
   aList := make([]tOhiEl, 0, len(iMap))
   for aK, aV := range iMap {
      aV.Uid = aK
      if aV.Alias == "" {
         aV.Alias = lookupUidAdrsbk(iSvc, aK) //todo temporary
      }
      aList = append(aList, aV)
   }
   sort.Slice(aList, func(cA, cB int) bool { return aList[cA].Date > aList[cB].Date })
   return aList
}

func GetFromOhi(iSvc string) []tOhiEl {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   if aSvc.fromOhi == nil {
      return nil
   }
   return _listOhi(iSvc, aSvc.fromOhi)
}

func setFromOhi(iSvc string, iHead *Header) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aSvc.fromOhi = tOhi{}
   aDate := dateRFC3339()
   for _, aUid := range iHead.Ohi {
      aSvc.fromOhi[aUid] = tOhiEl{Date:aDate}
   }
}

func updateFromOhi(iSvc string, iHead *Header) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   if iHead.Status == 1 {
      aSvc.fromOhi[iHead.From] = tOhiEl{Date:dateRFC3339()}
   } else {
      delete(aSvc.fromOhi, iHead.From)
   }
}

func dropFromOhi(iSvc string) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aSvc.fromOhi = nil
}

func GetToOhi(iSvc string) []tOhiEl {
   var aMap tOhi
   aSvc := getService(iSvc)
   aSvc.RLock()
   err := readJsonFile(&aMap, fileOhi(iSvc))
   aSvc.RUnlock()
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      return []tOhiEl{}
   }
   return _listOhi(iSvc, aMap)
}

func SendAllOhi(iW io.Writer, iSvc string, iId string) error {
   var err error
   aMap := tOhi{}
   aSvc := getService(iSvc)
   aSvc.RLock()
   err = readJsonFile(&aMap, fileOhi(iSvc))
   aSvc.RUnlock()
   if err != nil && !os.IsNotExist(err) { quit(err) }
   if len(aMap) == 0 {
      return nil
   }
   aFor := make(tForOhi, len(aMap))
   a := 0
   for aFor[a].Id, _ = range aMap { a++ }
   aHead, err := json.Marshal(Msg{"Op":4, "Id":iId, "For":aFor, "Type":"add"})
   if err != nil { quit(err) }
   err = writeHeaders(iW, aHead, nil)
   return err
}

func editOhi(iSvc string, iUpdt *Update) {
   var err error
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aMap := tOhi{}
   err = readJsonFile(&aMap, fileOhi(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   var aOp string
   if iUpdt.Op == "ohi_add" {
      aOp = "+"
      aMap[iUpdt.Ohi.Uid] = tOhiEl{Alias:iUpdt.Ohi.Alias, Date:dateRFC3339()}
   } else {
      aOp = "-"
      delete(aMap, iUpdt.Ohi.Uid)
   }
   err = storeFile(fileOhi(iSvc), aMap)
   if err != nil { quit(err) }
   if aSvc.sendQPost != nil {
      aSvc.sendQPost(&SendRecord{Id: string(eSrecOhi) + aOp + makeLocalId(iUpdt.Ohi.Uid)})
   }
}

func sendEditOhi(iW io.Writer, iSvc string, iQid, iId string) error {
   var err error
   aId := parseLocalId(iQid)
   aFor := tForOhi{{Id:aId.ohi()[1:]}}
   aType := "add"; if aId.ohi()[0] == '-' { aType = "drop" }
   aHead, err := json.Marshal(Msg{"Op":4, "Id":iId, "For":aFor, "Type":aType})
   if err != nil { quit(err) }
   err = writeHeaders(iW, aHead, nil)
   return err
}


