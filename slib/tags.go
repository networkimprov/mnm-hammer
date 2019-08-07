// Copyright 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "io"
   "os"
   "sync"
)

var sTagDefault = []string{"Todo"}

type tGlobalTag struct { // implements GlobalSet
   Map map[string]string // key Name, value Id
   sync.RWMutex
}
var Tag = &tGlobalTag{Map: make(map[string]string)}

func initTag() {
   err := resolveTmpFile(kTagFile +".tmp")
   if err != nil { quit(err) }
   err = readJsonFile(&Tag.Map, kTagFile)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      err = os.Symlink("empty", kTagFile)
      if err == nil {
         err = syncDir(kStorageDir)
      }
      if err != nil && !os.IsExist(err) { quit(err) }
   }
}

func (o *tGlobalTag) GetIdx() interface{} {
   type tTagEl struct { Id, Name string }
   aList := make([]tTagEl, 0, len(sTagDefault) + len(o.Map))
   for a := range sTagDefault {
      aList = append(aList, tTagEl{Name:sTagDefault[a], Id:sTagDefault[a]})
   }
   for aK, aV := range o.Map {
      aList = append(aList, tTagEl{Name:aK, Id:aV})
   }
   return aList
}

func (o *tGlobalTag) GetPath(iId string) string { return "" }

func (o *tGlobalTag) Add(iId, _ string, _ io.Reader) error {
   o.Lock(); defer o.Unlock()
   _, ok := o.Map[iId]
   if ok {
      quit(tError("tag already exists: "+ iId))
   }
   o.Map[iId] = dateRFC3339() //todo more robust unique id
   err := storeFile(kTagFile, o.Map)
   if err != nil { quit(err) }
   return nil
}

func (o *tGlobalTag) Drop(iId string) error {
   return nil
}

func (o *tGlobalTag) getId(iName string) string {
   for a := range sTagDefault {
      if sTagDefault[a] == iName {
         return iName
      }
   }
   o.RLock(); defer o.RUnlock()
   aVal, _ := o.Map[iName]
   return aVal
}
