// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "sort"
)

type tQueueEl struct {
  Srec SendRecord
  Date string
}

func GetQueue(iSvc string, iPostFn func(...*SendRecord)) []*SendRecord {
   // assume we're called once during synchronous Init()
   aSvc := getService(iSvc)
   aSvc.sendQPost = iPostFn // do not call during Init()
   aSort := append([]*tQueueEl{}, aSvc.sendQ...)
   sort.Slice(aSort, func(cA, cB int) bool { return aSort[cA].Date < aSort[cB].Date })
   aQ := make([]*SendRecord, len(aSort))
   for a := range aSort {
      aQ[a] = &aSort[a].Srec
   }
   return aQ
}

func hasQueue(iSvc string, iType byte, iId string) bool {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aId := string(iType) + iId
   aEl := sort.Search(len(aSvc.sendQ), func(c int) bool { return aSvc.sendQ[c].Srec.Id >= aId })
   return aEl < len(aSvc.sendQ) && aSvc.sendQ[aEl].Srec.Id == aId
}

func addQueue(iSvc string, iType byte, iId string) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aId := string(iType) + iId
   aEl := sort.Search(len(aSvc.sendQ), func(c int) bool { return aSvc.sendQ[c].Srec.Id >= aId })
   if aEl < len(aSvc.sendQ) && aSvc.sendQ[aEl].Srec.Id == aId {
      return
   }
   aSvc.sendQ = append(aSvc.sendQ, &tQueueEl{})
   if aEl < len(aSvc.sendQ) {
      copy(aSvc.sendQ[aEl+1:], aSvc.sendQ[aEl:])
   }
   aSvc.sendQ[aEl].Srec = SendRecord{aId}
   aSvc.sendQ[aEl].Date = dateRFC3339()
   err := storeFile(fileSendq(iSvc), aSvc.sendQ)
   if err != nil { quit(err) }
   if aSvc.sendQPost != nil {
      aSvc.sendQPost(&aSvc.sendQ[aEl].Srec)
   }
}

func addListQueue(iSvc string, iType byte, iIds []string, iNoPost string) []*SendRecord {
   if len(iIds) == 0 {
      return nil
   }
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   sort.Strings(iIds)
   aNewQ := make([]*tQueueEl, len(aSvc.sendQ) + len(iIds))
   aDate := dateRFC3339()
   aRecs := make([]*SendRecord, len(iIds))
   var a, aPrevN, aN int
   for a = range iIds {
      aId := string(iType) + iIds[a]
      aN = sort.Search(len(aSvc.sendQ), func(c int) bool { return aSvc.sendQ[c].Srec.Id >= aId })
      if aN < len(aSvc.sendQ) && aSvc.sendQ[aN].Srec.Id == aId {
         return nil
      }
      copy(aNewQ[aPrevN+a:], aSvc.sendQ[aPrevN:aN])
      aPrevN = aN
      aNewQ[aN+a] = &tQueueEl{Srec:SendRecord{aId}, Date:aDate}
      aRecs[a] = &aNewQ[aN+a].Srec
   }
   copy(aNewQ[aN+a+1:], aSvc.sendQ[aN:])
   aSvc.sendQ = aNewQ
   err := storeFile(fileSendq(iSvc), aSvc.sendQ)
   if err != nil { quit(err) }
   if iNoPost != "" {
      return aRecs
   }
   if aSvc.sendQPost != nil {
      aSvc.sendQPost(aRecs...)
   }
   return nil
}

func dropQueue(iSvc string, iId string) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aEl := sort.Search(len(aSvc.sendQ), func(c int) bool { return aSvc.sendQ[c].Srec.Id >= iId })
   if aEl == len(aSvc.sendQ) || aSvc.sendQ[aEl].Srec.Id != iId {
      return
   }
   aSvc.sendQ = aSvc.sendQ[:aEl + copy(aSvc.sendQ[aEl:], aSvc.sendQ[aEl+1:])]
   err := storeFile(fileSendq(iSvc), aSvc.sendQ)
   if err != nil { quit(err) }
}

