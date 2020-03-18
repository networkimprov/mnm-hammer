// Copyright 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "fmt"
   "os"
   "sort"
   "strings"
   "sync"
)

var kNumSup = [...]rune{'\xb9','\xb2','\xb3','\u2074','\u2075','\u2076','\u2077','\u2078','\u2079'}

var sTags = tTagset{"Todo":"Todo"}
var sTagCopies = map[string]bool{"Todo\x00":true} // key id+"\x00"+svcid
var sTagsDoor sync.RWMutex

type tTagset map[string]string // key name, value id

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

func makeIdTag() string {
   return dateRFC3339() //todo more robust unique id
}

func mustCopyTag(iSvc string, iId string) string {
   sTagsDoor.RLock(); defer sTagsDoor.RUnlock()
   if sTagCopies[iId +"\x00"+ iSvc] || sTagCopies[iId +"\x00"] {
      return ""
   }
   for aK, aV := range sTags {
      if aV == iId {
         return aK
      }
   }
   quit(tError("id not found in sTags: "+ iId))
   return ""
}

func addTag(iSvc string, iName string, iId string) {
   sTagsDoor.Lock(); defer sTagsDoor.Unlock()
   if sTagCopies[iId +"\x00"+ iSvc] || sTagCopies[iId +"\x00"] {
      fmt.Fprintf(os.Stderr, "addTag %s: already have id for %s=%s\n", iSvc, iName, iId)
      return
   }
   iName, err := _altTagName(sTags, iName, iId, iSvc, "addTag "+ iSvc)
   if err != nil { quit(err) }
   sTagCopies[iId +"\x00"+ iSvc] = true
   sTags[iName] = iId
   aTagsSvc := tTagset{}
   err = readJsonFile(&aTagsSvc, fileTag(iSvc))
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aTagsSvc[iName] = iId
   err = storeFile(fileTag(iSvc), aTagsSvc)
   if err != nil { quit(err) }
}

func fixConflictTag(iTagsSvc tTagset, iSvcId string) (tTagset, error) {
   aTags, aTagsNew := tTagset{}, tTagset{}
   sTagsDoor.RLock()
   for aK, aV := range sTags {
      aTags[aK] = aV
   }
   sTagsDoor.RUnlock()
   var err error
   for aK, aV := range iTagsSvc {
      aK, err = _altTagName(aTags, aK, aV, iSvcId, "fixConflictTag "+ iSvcId)
      if err != nil {
         return nil, err
      }
      aTags[aK] = aV
      aTagsNew[aK] = aV
   }
   return aTagsNew, nil
}

func _altTagName(iTags tTagset, iName string, iId string, iExt, iPrefix string) (string, error) {
   if aV, ok := iTags[iName]; ok && aV != iId {
      var aK string
      for aK, aV = range iTags {
         if aV != iId { continue }
         fmt.Printf("%s: found %s for %s=%s\n", iPrefix, aK, iName, iId)
         //todo consider iName = aK
      }
      for _, aSup := range kNumSup {
         aK = iName +" ["+ iExt + string(aSup) +"]"
         if aV, ok = iTags[aK]; !ok || aV == iId {
            fmt.Printf("%s: set name %s for %s=%s\n", iPrefix, aK, iName, iId)
            iName = aK
            break
         }
      }
      if aK != iName {
         return iName, tError("can't find alternate name for tag: "+ iName)
      }
   }
   return iName, nil
}
