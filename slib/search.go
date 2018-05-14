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
   "io/ioutil"
   "encoding/json"
   "os"
   "sort"
   "strings"
   "time"
)

func WriteResultSearch(iW io.Writer, iSvc string, iState *ClientState) error {
   var err error
   if iState.SvcTabs.PosFor == ePosForTerms &&
      strings.HasPrefix(iState.SvcTabs.Terms[iState.SvcTabs.Pos], "ffn:") {
      aFfn := iState.SvcTabs.Terms[iState.SvcTabs.Pos][4:]
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
   if iState.SvcTabs.PosFor != ePosForDefault {
      _, err = iW.Write([]byte(`[]`))
      return err
   }
   var aDir []os.FileInfo
   if iState.SvcTabs.Pos == 3 {
      aDir, err = ioutil.ReadDir(formDir(iSvc))
      if err != nil { quit(err) }
   } else {
      aDir, err = ioutil.ReadDir(threadDir(iSvc))
      if err != nil { quit(err) }
      sort.Slice(aDir, func(cA, cB int) bool { return aDir[cA].ModTime().After(aDir[cB].ModTime()) })
   }
   aList := make([]struct{Id string; Date string}, len(aDir))
   aI := 0
   for a, _ := range aDir {
      aList[aI].Date = aDir[a].ModTime().UTC().Format(time.RFC3339)
      if iState.SvcTabs.Pos == 3 {
         aList[aI].Id = strings.Replace(aDir[a].Name(), "@", "/", -1)
         aI++
      } else if aDir[a].Name() != "_22" && !strings.ContainsRune(aDir[a].Name()[1:], '_') {
         aList[aI].Id = aDir[a].Name()
         aI++
      }
   }
   err = json.NewEncoder(iW).Encode(aList[:aI])
   return err
}

