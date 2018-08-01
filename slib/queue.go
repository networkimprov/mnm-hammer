// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package slib

import (
   "sort"
)

type tQueueEl struct {
  Srec SendRecord
  Date string
}

func GetQueue(iSvc string) ([]*SendRecord, error) {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aSort := make([]*tQueueEl, len(aSvc.sendQ))
   for a, _ := range aSvc.sendQ {
      aSort[a] = &aSvc.sendQ[a]
   }
   sort.Slice(aSort, func(cA, cB int) bool { return aSort[cA].Date < aSort[cB].Date })
   aQ := make([]*SendRecord, len(aSort))
   for a, _ := range aSort {
      aQ[a] = &aSort[a].Srec
   }
   return aQ, nil
}

func hasQueue(iSvc string, iType byte, iId string) bool {
   aSvc := getService(iSvc)
   aSvc.RLock(); defer aSvc.RUnlock()
   aId := string(iType) + iId
   aEl := sort.Search(len(aSvc.sendQ), func(c int) bool { return aSvc.sendQ[c].Srec.Id >= aId })
   return aEl < len(aSvc.sendQ) && aSvc.sendQ[aEl].Srec.Id == aId
}

func addQueue(iSvc string, iType byte, iId string) *SendRecord {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aId := string(iType) + iId
   aEl := sort.Search(len(aSvc.sendQ), func(c int) bool { return aSvc.sendQ[c].Srec.Id >= aId })
   if aEl < len(aSvc.sendQ) && aSvc.sendQ[aEl].Srec.Id == aId {
      return nil
   }
   aSvc.sendQ = append(aSvc.sendQ, tQueueEl{})
   if aEl < len(aSvc.sendQ) {
      copy(aSvc.sendQ[aEl+1:], aSvc.sendQ[aEl:])
   }
   aSvc.sendQ[aEl].Srec = SendRecord{aId}
   aSvc.sendQ[aEl].Date = dateRFC3339()
   err := storeFile(sendqFile(iSvc), aSvc.sendQ)
   if err != nil { quit(err) }
   return &aSvc.sendQ[aEl].Srec
}

func dropQueue(iSvc string, iId string) {
   aSvc := getService(iSvc)
   aSvc.Lock(); defer aSvc.Unlock()
   aEl := sort.Search(len(aSvc.sendQ), func(c int) bool { return aSvc.sendQ[c].Srec.Id >= iId })
   if aEl == len(aSvc.sendQ) || aSvc.sendQ[aEl].Srec.Id != iId {
      return
   }
   aSvc.sendQ = aSvc.sendQ[:aEl + copy(aSvc.sendQ[aEl:], aSvc.sendQ[aEl+1:])]
   err := storeFile(sendqFile(iSvc), aSvc.sendQ)
   if err != nil { quit(err) }
}

