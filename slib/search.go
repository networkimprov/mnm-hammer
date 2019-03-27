// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "io"
   "io/ioutil"
   "encoding/json"
   "os"
   "sort"
   "strings"
   "time"
)

func WriteResultSearch(iW io.Writer, iSvc string, iState *ClientState) error {
   var err error
   aTabType, aTabVal := iState.getSvcTab()
   if aTabType == ePosForTerms && strings.HasPrefix(aTabVal, "ffn:") {
      aFfn := aTabVal[4:]
      _, err = iW.Write([]byte(`{"Ffn":`))
      if err != nil { return err }
      err = json.NewEncoder(iW).Encode(aFfn)
      if err != nil { return err }
      _, err = iW.Write([]byte(`,"Table":`))
      if err != nil { return err }
      err = WriteTableFilledForm(iW, iSvc, aFfn)
      if err != nil { return err }
      _, err = iW.Write([]byte{'}'})
      return err
   }
   if aTabType != ePosForDefault {
      _, err = iW.Write([]byte(`[]`))
      return err
   }
   var aDir []os.FileInfo
   if aTabVal == "FFT" {
      aDir, err = ioutil.ReadDir(dirForm(iSvc))
      if err != nil { quit(err) }
   } else {
      aDir, err = ioutil.ReadDir(dirThread(iSvc))
      if err != nil { quit(err) }
      sort.Slice(aDir, func(cA, cB int) bool { return aDir[cA].ModTime().After(aDir[cB].ModTime()) })
   }
   type tSearchEl struct {
      Id string
      Subject string
      OrigDate, LastDate string
      OrigAuthor, LastAuthor string
   }
   aList := make([]tSearchEl, 0, len(aDir))

   var aIdx, aFidx []tIndexElCore
   fReadIndex := func(cTid string) []tIndexElCore {
      cDoor := _getThreadDoor(iSvc, cTid)
      cDoor.RLock(); defer cDoor.RUnlock()
      if cDoor.renamed { return nil }

      cFd, err := os.Open(dirThread(iSvc) + cTid)
      if err != nil { quit(err) }
      defer cFd.Close()
      _ = _readIndex(cFd, &aIdx, nil)
      return aIdx
   }
   for _, aFi := range aDir {
      if aTabVal == "FFT" {
         aList = append(aList, tSearchEl{LastDate: aFi.ModTime().UTC().Format(time.RFC3339),
                                         Id: strings.Replace(aFi.Name(), "@", "/", -1)})
      } else if !strings.ContainsRune(aFi.Name()[1:], '_') {
         aFidx = fReadIndex(aFi.Name())
         if aFidx != nil {
            aEl := tSearchEl{Id: aFi.Name(), OrigDate: aFidx[0].Date, OrigAuthor: aFidx[0].Alias}
            for a := range aFidx {
               if aFidx[a].Subject != "" {
                  aEl.Subject = aFidx[a].Subject
               }
               if aFidx[a].From != "" || a == 0 {
                  aEl.LastDate, aEl.LastAuthor = aFidx[a].Date, aFidx[a].Alias
               }
            }
            aList = append(aList, aEl)
         }
      }
   }
   err = json.NewEncoder(iW).Encode(aList)
   return err
}

