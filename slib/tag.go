// Copyright 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "os"
   "sort"
   "strings"
   "sync"
)

type tTagset map[string]string // key name, value id

var sTags = tTagset{"Todo":"Todo"}
var sTagCopies = map[string]bool{"Todo\x00":true} // key id+"\x00"+svcid
var sTagsDoor sync.RWMutex

func initTag(iSvc string, iMap tTagset) {
   sTagsDoor.Lock(); defer sTagsDoor.Unlock()
   for aK, aV := range iMap {
      if aId, ok := sTags[aK]; ok && aId != aV {
         quit(tError("tag duplicated: "+ aK))
      }
      sTags[aK] = aV
      sTagCopies[aV +"\x00"+ iSvc] = true
   }
}

func GetIdxTag() interface{} {
   sTagsDoor.RLock(); defer sTagsDoor.RUnlock()
   type tTagEl struct { Id, Name string }
   aList := make([]tTagEl, 0, len(sTags))
   for aK, aV := range sTags {
      aList = append(aList, tTagEl{Name:aK, Id:aV})
   }
   sort.Slice(aList, func(cA, cB int) bool { return strings.ToLower(aList[cA].Name) <
                                                    strings.ToLower(aList[cB].Name) })
   return aList
}

func GetIdTag(iName string) string {
   sTagsDoor.RLock(); defer sTagsDoor.RUnlock()
   aVal, _ := sTags[iName]
   return aVal
}

func addTag(iSvc string, iName string) error {
   aVal := dateRFC3339() //todo more robust unique id
   sTagsDoor.Lock(); defer sTagsDoor.Unlock()
   if _, ok := sTags[iName]; ok {
      return tError("tag already exists: "+ iName)
   }
   sTagCopies[aVal +"\x00"+ iSvc] = true
   sTags[iName] = aVal
   aTagsSvc := tTagset{}
   err := readJsonFile(&aTagsSvc, fileTag(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aTagsSvc[iName] = aVal
   err = storeFile(fileTag(iSvc), aTagsSvc)
   if err != nil { quit(err) }
   return nil
}

func copyTag(iSvc string, iIds []string) {
   sTagsDoor.Lock(); defer sTagsDoor.Unlock()
   var aTagsSvc tTagset
   var err error
   for a := range iIds {
      if sTagCopies[iIds[a] +"\x00"+ iSvc] || sTagCopies[iIds[a] +"\x00"] { continue }
      if aTagsSvc == nil {
         aTagsSvc = tTagset{}
         err = readJsonFile(&aTagsSvc, fileTag(iSvc))
         if err != nil && !os.IsNotExist(err) { quit(err) }
      }
      var aName, aId string
      for aName, aId = range sTags {
         if aId == iIds[a] { break }
      }
      if aId != iIds[a] {
         quit(tError("id not found in sTags: "+ iIds[a]))
      }
      sTagCopies[iIds[a] +"\x00"+ iSvc] = true
      aTagsSvc[aName] = iIds[a]
   }
   if aTagsSvc != nil {
      err = storeFile(fileTag(iSvc), aTagsSvc)
      if err != nil { quit(err) }
   }
}

func hasConflictTag(iSvc string) (string, bool) {
   aTagsSvc := tTagset{}
   err := readJsonFile(&aTagsSvc, fileTag(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   sTagsDoor.RLock(); defer sTagsDoor.RUnlock()
   for aK, aV := range aTagsSvc {
      if aId, ok := sTags[aK]; ok && aId != aV {
         return aK, true
      }
   }
   return "", false
}
