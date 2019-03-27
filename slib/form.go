// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "bytes"
   "fmt"
   "io"
   "io/ioutil"
   "encoding/json"
   "os"
   "path"
   "sort"
   "strconv"
   "strings"
   "sync"
   "time"
)

type tGlobalBlankForm struct{} // implements GlobalSet
var BlankForm tGlobalBlankForm

var sBlankFormsDoor sync.RWMutex
var sBlankForms = make(map[string]*tBlankForm)

type tBlankForm struct {
   Name string
   Spec bool
   Revs []tBlankFormRev
}

type tBlankFormRev struct { Id string; Date string }

func initForms() {
   var err error
   aDir, err := ioutil.ReadDir(kFormDir)
   if err != nil { quit(err) }
   sort.Slice(aDir, func (cA, cB int) bool { return aDir[cA].ModTime().Before(aDir[cB].ModTime()) })

   for _, aFi := range aDir {
      aFn := aFi.Name()
      if strings.HasSuffix(aFn, ".tmp") {
         err = os.Remove(kFormDir + aFn)
         if err != nil { quit(err) }
         continue
      } else if strings.HasSuffix(aFn, ".tok") {
         aFn = aFn[:len(aFn)-4]
         err = os.Remove(kFormDir + aFn)
         if err != nil && !os.IsNotExist(err) { quit(err) }
         err = os.Rename(kFormDir + aFn + ".tok", kFormDir + aFn)
         if err != nil { quit(err) }
      }
      aName, aRev := _parseFileName(aFn)
      _insertBlank(aName, aRev, aFi.ModTime().UTC().Format(time.RFC3339))
   }
}

func (tGlobalBlankForm) GetIdx() interface{} {
   sBlankFormsDoor.RLock(); defer sBlankFormsDoor.RUnlock()
   aList := make([]*tBlankForm, 0, len(sBlankForms))
   for _, aV := range sBlankForms {
      aList = append(aList, aV)
   }
   return aList
}

func (tGlobalBlankForm) GetPath(iFileName string) string {
   return kFormDir + iFileName
}

func (tGlobalBlankForm) Add(iFileName, iDupeRev string, iR io.Reader) error {
   var err error
   aName, aRev := _parseFileName(iFileName)
   if iDupeRev != "" {
      if aRev == "" { iFileName = aName + ".original" }
      aRev = iDupeRev
   } else {
      if aRev == "" { aRev = "original" }
   }
   if strings.ContainsRune(iFileName, '/') || strings.ContainsRune(aRev, '.') ||
      aRev == "tmp" || aRev == "tok" ||
      iDupeRev == "original" || iDupeRev == "spec" {
      return tError("invalid form name")
   }
   aPath := kFormDir + aName + "." + aRev
   aTemp := aPath + ".tmp"
   aTempOk := aPath + ".tok"

   if iDupeRev != "" {
      var aDd *os.File
      aDd, err = os.Open(kFormDir + iFileName)
      if err != nil {
         if !os.IsNotExist(err) { quit(err) }
         return tError("dupe source not found")
      }
      defer aDd.Close()
      iR = aDd
   }

   sBlankFormsDoor.Lock(); defer sBlankFormsDoor.Unlock()
   aBf := sBlankForms[aName]
   if aBf != nil && aRev != aBf.Revs[0].Id && !aBf.Spec && aRev != "spec" {
      return tError("cannot add form rev for original with no spec")
   }
   _insertBlank(aName, aRev, dateRFC3339())

   aFd, err := os.OpenFile(aTemp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aFd.Close()
   _, err = io.Copy(aFd, iR)
   if err != nil { return err } //todo only network errors
   err = aFd.Sync()
   if err != nil { quit(err) }

   err = os.Rename(aTemp, aTempOk)
   if err != nil { quit(err) }
   err = syncDir(kFormDir)
   if err != nil { quit(err) }
   err = os.Remove(aPath)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   err = os.Rename(aTempOk, aPath)
   if err != nil { quit(err) }
   return nil
}

func (tGlobalBlankForm) Drop(iFileName string) error {
   aName, aRev := _parseFileName(iFileName)
   aPath := kFormDir + aName + "." + aRev

   sBlankFormsDoor.Lock(); defer sBlankFormsDoor.Unlock()
   aBf := sBlankForms[aName]
   if aBf == nil {
      return tError("form not found for "+iFileName)
   }
   var a int
   for a, _ = range aBf.Revs {
      if aBf.Revs[a].Id == aRev { break }
   }
   if aBf.Revs[a].Id != aRev {
      return tError("rev not found for "+iFileName)
   }
   if len(aBf.Revs) == 1 {
      delete(sBlankForms, aName)
   } else {
      aBf.Revs = aBf.Revs[:a + copy(aBf.Revs[a:], aBf.Revs[a+1:])]
      if aRev == "spec" {
         aBf.Spec = false
      }
   }
   err := os.Remove(aPath)
   if err != nil { quit(err) }
   return nil
}

func readFfnBlankForm(iFileName string) string {
   aName, aRev := _parseFileName(iFileName)
   sBlankFormsDoor.RLock(); defer sBlankFormsDoor.RUnlock()
   aBf := sBlankForms[aName]
   if aBf == nil {
      return "#form not found"
   }
   if aRev == "spec" {
      return ""
   }
   if !aBf.Spec {
      return "local/" + aName
   }
   var aJson struct { Ffn string }
   err := readJsonFile(&aJson, kFormDir + aName + ".spec")
   if err != nil {
      if os.IsNotExist(err) { quit(err) }
      return "#" + err.Error()
   }
   if aJson.Ffn == "" {
      return "local/" + aName
   }
   return aJson.Ffn
}

func _parseFileName(i string) (string, string) {
   aPair := strings.SplitN(i, ".", 2)
   if len(aPair) == 2 {
      return aPair[0], aPair[1]
   }
   return aPair[0], ""
}

func _insertBlank(iName, iRev string, iDate string) {
   aBf := sBlankForms[iName]
   if aBf == nil {
      aBf = &tBlankForm{ Name: iName, Revs: []tBlankFormRev{{Id: iRev, Date: iDate}} }
      sBlankForms[iName] = aBf
   } else {
      var a int
      for a, _ = range aBf.Revs {
         if aBf.Revs[a].Id == iRev { break }
      }
      if aBf.Revs[a].Id != iRev {
         a = len(aBf.Revs)
         aBf.Revs = append(aBf.Revs, tBlankFormRev{})
      }
      copy(aBf.Revs[1:], aBf.Revs[0:a])
      aBf.Revs[0] = tBlankFormRev{Id: iRev, Date: iDate}
   }
   if iRev == "spec" {
      aBf.Spec = true
   }
}

func WriteTableFilledForm(iW io.Writer, iSvc string, iFfn string) error {
   var err error
   aDoor := _getFormDoor(iSvc, iFfn)
   aDoor.RLock(); defer aDoor.RUnlock()
   aFd, err := os.Open(dirForm(iSvc) + _ffnFileName(iFfn))
   if err != nil { return err }
   defer aFd.Close()
   _, err = io.Copy(iW, aFd)
   return err
}

func writeRowFilledForm(iW io.Writer, iSvc string, iFfn, iMsgId string) (int64, error) {
   var err error
   aDoor := _getFormDoor(iSvc, iFfn)
   aDoor.RLock(); defer aDoor.RUnlock()
   aFd, err := os.Open(dirForm(iSvc) + _ffnFileName(iFfn))
   if err != nil { quit(err) }
   defer aFd.Close()

   aDc := json.NewDecoder(aFd)
   aDc.UseNumber()
   _, err = aDc.Token()
   if err != nil { quit(err) }

   var aRow Msg
   for aRow = nil; aDc.More(); aRow = nil {
      err = aDc.Decode(&aRow)
      if err != nil { quit(err) }
      if aRow["$msgid"].(string) == iMsgId { break }
   }
   if aRow == nil {
      quit(fmt.Errorf("%s formfill table %s lacks msgid %s\n", iSvc, iFfn, iMsgId))
   }
   aLen, err := aRow["$size"].(json.Number).Int64()
   if err != nil { quit(err) }
   aTxt := aRow["$text"]
   if aTxt != nil {
      _, err = io.WriteString(iW, aTxt.(string))
      return aLen, err
   }
   aPos, err := aRow["$offset"].(json.Number).Int64()
   if err != nil { quit(err) }

   _, err = aFd.Seek(aPos, io.SeekStart)
   if err != nil { quit(err) }
   _, err = io.CopyN(iW, aFd, aLen-1)
   if err == nil {
      _, err = iW.Write([]byte{'}'})
   }
   return aLen, err //todo only net errors
}

func validateFilledForm(iSvc string, iBuf []byte, iFfn string) error {
   var err error
   var aForm map[string]interface{}
   err = json.Unmarshal(iBuf, &aForm)
   if err != nil { return err }

   var aPath string
   aLocalUri := getUriService(iSvc)
   if strings.HasPrefix(iFfn, aLocalUri) {
      aPath = kFormDir + iFfn[len(aLocalUri):] + ".spec"
   } else {
      aPath = kFormRegDir + iFfn
      err = _retrieveSpec(iFfn)
      if err != nil { return err }
   }
   var aJson struct { Spec []tSpecEl; Ffn string }
   err = readJsonFile(&aJson, aPath)
   if err != nil && !os.IsNotExist(err) { return err }
   if aJson.Spec == nil { return nil } //todo indicate spec not found?

   var aResult []byte
   _validateObject(&aResult, "", aForm, aJson.Spec)
   if aResult != nil { return tError("form-fill " + string(aResult)) }
   return nil
}

type tSpecEl struct {
   Name, Type string
   Status string // required, optional, deprecated
   Array int // N-dimensional array of the specified type
   Spec []tSpecEl // for Type "object"
}

func _retrieveSpec(iFfn string) error {
   //todo download from registry
   aTd, err := os.Open("./formspec")
   if err != nil { quit(err) }
   err = os.MkdirAll(path.Dir(kFormRegDir + iFfn), 0700)
   if err != nil { quit(err) }
   aFd, err := os.OpenFile(kFormRegDir + iFfn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
   if err != nil { quit(err) }
   _, err = io.Copy(aFd, aTd)
   if err != nil { quit(err) }
   err = aFd.Sync()
   if err != nil { quit(err) }
   aFd.Close()
   aTd.Close()
   return nil
}

func _validateObject(iResult *[]byte, iParent string, iForm map[string]interface{}, iSpec []tSpecEl) {
   fAppend := func(c string) { *iResult = append(*iResult, iParent+c+"; "...) }
   for _, aEl := range iSpec {
      aField := iForm[aEl.Name]
      if aEl.Status == "required" {
         if aField == nil { fAppend(aEl.Name+" missing") }
      } else if aEl.Status == "deprecated" {
         if aField != nil { fAppend(aEl.Name+" deprecated") }
      }
      if !_validateType(iResult, iParent, aField, &aEl, aEl.Array) {
         aWant := aEl.Type; if aEl.Array > 0 { aWant = fmt.Sprint(aEl.Array)+"D array of "+aEl.Type }
         fAppend(aEl.Name+" must be "+aWant)
      }
      delete(iForm, aEl.Name)
   }
   for aK, _ := range iForm {
      fAppend(aK+" not defined in spec")
   }
}

func _validateType(iResult *[]byte, iParent string, iField interface{}, iEl *tSpecEl, iArray int) bool {
   switch iField.(type) {
   case bool:                   if iArray > 0 || iEl.Type != "bool"   { return false }
   case string:                 if iArray > 0 || iEl.Type != "string" { return false }
   case float64:                if iArray > 0 || iEl.Type != "number" { return false }
   case map[string]interface{}: if iArray > 0 || iEl.Type != "object" { return false }
      _validateObject(iResult, iParent+iEl.Name+".", iField.(map[string]interface{}), iEl.Spec)
   case []interface{}:          if iArray < 1          { return false }
      for _, aI := range iField.([]interface{}) {
         if !_validateType(iResult, iParent, aI, iEl, iArray-1) { return false }
      }
   }
   return true
}

func tempFilledForm(iSvc string, iThreadId, iMsgId string, iSuffix string, iFile *tHeader2Attach,
                    iR io.Reader) error {
   var err error
   aFn := ftmpAttach(iSvc, iMsgId, iFile.Name)
   aFd, err := os.OpenFile(aFn, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aFd.Close()

   var aFi os.FileInfo
   aFi, err = os.Lstat(dirForm(iSvc) + _ffnFileName(iFile.Ffn) + iSuffix)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aPos, aSep := int64(0), '['; if err == nil { aPos, aSep = aFi.Size() - 1, ',' }
   _, err = aFd.Write([]byte(fmt.Sprintf("%016x%016x%c\n\n", aPos, aPos, aSep))) // 2 copies for safety
   if err != nil { quit(err) }

   var aBuf bytes.Buffer
   var aCw tCrcWriter
   aTee := io.MultiWriter(aFd, &aBuf, &aCw)
   _, err = io.CopyN(aTee, iR, iFile.Size - 1) // omit closing '}'
   if err == nil {
      _, err = io.CopyN(&aBuf, iR, 1)
   }
   if err != nil {
      return err //todo only return network error
   }
   if aBuf.Bytes()[0] != '{' || json.Unmarshal(aBuf.Bytes(), &struct{}{}) != nil {
      fmt.Fprintf(os.Stderr, "tempFilledForm %s: received bad json for %s\n", iSvc, iFile.Ffn+iSuffix)
      var aJson []byte
      aJson, err = json.Marshal(aBuf.String())
      if err != nil { quit(err) }
      _, err = aFd.Seek(32, io.SeekStart)
      if err != nil { quit(err) }
      aCw = tCrcWriter{}
      aTee = io.MultiWriter(aFd, &aCw)
      _, err = aTee.Write([]byte(fmt.Sprintf(`{"$text":%s`, aJson)))
      if err != nil { quit(err) }
   }
   _, err = aTee.Write([]byte(fmt.Sprintf(`,"$threadid":"%s","$msgid":"%s","$offset":%d,"$size":%d`,
                                          iThreadId, iMsgId, aPos+3, iFile.Size)))
   if err != nil { quit(err) }
   aCw.Write([]byte{'}'}) // include closing '}' in checksum
   _, err = aFd.Write([]byte(fmt.Sprintf(`,"$checksum":%d}]`, aCw.sum)))
   if err != nil { quit(err) }

   err = aFd.Sync()
   if err != nil { quit(err) }
   return nil
}

func storeFilledForm(iSvc string, iMsgId string, iSuffix string, iFile *tHeader2Attach) bool {
   var err error
   aDoor := _getFormDoor(iSvc, iFile.Ffn + iSuffix)
   aDoor.Lock(); defer aDoor.Unlock()
   aFn := ftmpAttach(iSvc, iMsgId, iFile.Name)
   aTd, err := os.Open(aFn)
   if err != nil { quit(err) }
   defer aTd.Close()
   aBuf := make([]byte, 32)
   _, err = aTd.Read(aBuf)
   if err != nil { quit(err) }
   var aPos [2]uint64
   for a, _ := range aPos {
      aPos[a], err = strconv.ParseUint(string(aBuf[a*16:(a+1)*16]), 16, 64)
      if err != nil { quit(err) }
   }
   if aPos[0] != aPos[1] {
      quit(tError(fmt.Sprintf("position values do not match in %s", aFn)))
      //todo recovery instructions
   }
   aPath := dirForm(iSvc) + _ffnFileName(iFile.Ffn) + iSuffix
   _, err = os.Lstat(aPath)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aDoSync := err != nil
   aFd, err := os.OpenFile(aPath, os.O_WRONLY|os.O_CREATE, 0600)
   if err != nil { quit(err) }
   defer aFd.Close()
   if aPos[0] > 0 {
      _, err = aFd.Seek(int64(aPos[0]), io.SeekStart)
      if err != nil { quit(err) }
   }
   _, err = io.Copy(aFd, aTd)
   if err != nil { quit(err) }
   err = aFd.Sync()
   if err != nil { quit(err) }
   return aDoSync
}

func _getFormDoor(iSvc string, iFfn string) *sync.RWMutex {
   return getDoorService(iSvc, iFfn, func()tDoor{ return &sync.RWMutex{} }).(*sync.RWMutex)
}

func _ffnFileName(iFfn string) string {
   return strings.Replace(iFfn, "/", "@", -1)
}

