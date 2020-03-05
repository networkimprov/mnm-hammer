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
   aSvc := getService(iSvc)
   aSvc.RLock()
   aMap := tOhi{}
   err := readJsonFile(&aMap, fileOhi(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aSvc.RUnlock()
   a, aFor := 0, make(tForOhi, len(aMap))
   for aK, aV := range aMap {
      if aV.Date == "pending" { continue }
      aFor[a].Id = aK
      a++
   }
   if a == 0 {
      return nil
   }
   aHead, err := json.Marshal(Msg{"Op":4, "Id":iId, "For":aFor[:a], "Type":"init"})
   if err != nil { quit(err) }
   err = writeHeaders(iW, aHead, nil)
   return err
}

func editOhi(iSvc string, iUpdt *Update) {
   aOp := "-"; if iUpdt.Op == "ohi_add" { aOp = "+" }
   addQueue(iSvc, eSrecOhi, aOp + makeLocalId(iUpdt.Ohi.Uid)) // can cause race with updateOhi()?
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aMap := tOhi{}
   err := readJsonFile(&aMap, fileOhi(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aMap[iUpdt.Ohi.Uid] = tOhiEl{Alias:iUpdt.Ohi.Alias, Date:"pending"}
   err = storeFile(fileOhi(iSvc), aMap) // if addQueue works but this fails, updateOhi() will fix the map
   if err != nil { quit(err) }
}

func sendEditOhi(iW io.Writer, iSvc string, iQid, iId string) error {
   aId := parseLocalId(iQid)
   aFor := tForOhi{{Id:aId.ohi()[1:]}}
   aType := "add"; if aId.ohi()[0] == '-' { aType = "drop" }
   aHead, err := json.Marshal(Msg{"Op":4, "Id":iId, "For":aFor, "Type":aType})
   if err != nil { quit(err) }
   err = writeHeaders(iW, aHead, nil)
   return err
}

func updateOhi(iSvc string, iHead *Header) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aMap := tOhi{}
   err := readJsonFile(&aMap, fileOhi(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   for a := range iHead.For {
      if iHead.Type == "add" {
         aAlias := lookupUidAdrsbk(iSvc, iHead.For[a].Id) //todo temporary
         aMap[iHead.For[a].Id] = tOhiEl{Alias:aAlias, Date:iHead.Posted}
      } else {
         delete(aMap, iHead.For[a].Id)
      }
   }
   err = storeFile(fileOhi(iSvc), aMap)
   if err != nil { quit(err) }
}
