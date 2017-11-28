// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

package slib

import (
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
   sort.Slice(aDir, func (cA, cB int) bool { return aDir[cA].ModTime().After(aDir[cB].ModTime()) })

   for _, aFi := range aDir {
      aFn := aFi.Name()
      if strings.HasSuffix(aFn, ".tmp") {
         err = os.Remove(kFormDir + aFn)
         if err != nil { quit(err) }
         continue
      } else if strings.HasSuffix(aFn, ".tok") {
         err = resolveTmpFile(kFormDir + aFn)
         if err != nil { quit(err) }
         aFn = aFn[:len(aFn)-4]
      }
      aName, aRev := _parseFileName(aFn)
      _insertBlank(aName, aRev, aFi.ModTime().UTC().Format(time.RFC3339))
   }
}

func GetIdxBlankForm() []interface{} {
   sBlankFormsDoor.RLock(); defer sBlankFormsDoor.RUnlock()
   aList := make([]interface{}, len(sBlankForms))
   a := 0
   for _, aFm := range sBlankForms {
      aList[a] = aFm
      a++
   }
   sort.Slice(aList, func(cA, cB int) bool {
      return aList[cA].(*tBlankForm).Revs[0].Date > aList[cB].(*tBlankForm).Revs[0].Date
   })
   return aList
}

func GetPathBlankForm(iFileName string) string {
   return kFormDir + iFileName
}

func AddBlankForm(iFileName, iDupeRev string, iR io.Reader) error {
   var err error
   aName, aRev := _parseFileName(iFileName)
   if iDupeRev != "" { aRev = iDupeRev }
   if aRev == "tmp" || aRev == "tok" || strings.ContainsRune(aRev, '.') ||
      aRev == "spec" && iDupeRev != "" {
      return tError("invalid form name suffix")
   }
   aPath := kFormDir + aName
   if aRev != "" { aPath += "." + aRev }
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
   if aRev != "" {
      aBf := sBlankForms[aName]
      if aBf == nil {
         return tError("cannot add form rev/spec without original")
      }
      if aRev != "spec" && !aBf.Spec {
         return tError("cannot add form rev for original with no spec")
      }
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

func DropBlankForm(iFileName string) bool {
   aName, aRev := _parseFileName(iFileName)
   aPath := kFormDir + aName
   if aRev != "" { aPath += "." + aRev }

   sBlankFormsDoor.Lock(); defer sBlankFormsDoor.Unlock()
   aBf := sBlankForms[aName]
   if aBf == nil {
      return false
   }
   var a int
   for a, _ = range aBf.Revs {
      if aBf.Revs[a].Id == aRev { break }
   }
   if aBf.Revs[a].Id != aRev {
      return false
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
   return true
}

func MakeMsgBlankForm() Msg { return Msg{"op":"blank_form"} }

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
      return "local"
   }
   var aData map[string]interface{}
   err := readJsonFile(&aData, kFormDir + aName + ".spec")
   if err != nil {
      if os.IsNotExist(err) { quit(err) }
      return "#" + err.Error()
   }
   aFfn, _ := aData["ffn"].(string)
   if aFfn == "" {
      return "local"
   }
   return aFfn
}

func _parseFileName(i string) (string, string) {
   aPair := strings.SplitN(i, ".", 2)
   if len(aPair) == 2 {
      return aPair[0], aPair[1]
   }
   return aPair[0], ""
}

func readJsonFile(iForm interface{}, iPath string) error {
   aFd, err := os.Open(iPath)
   if err != nil {
      if !os.IsNotExist(err) { quit(err) }
      return err
   }
   defer aFd.Close()
   err = json.NewDecoder(aFd).Decode(iForm)
   if err != nil && err != io.ErrUnexpectedEOF {
      if _, ok := err.(*json.SyntaxError); !ok { quit(err) }
   }
   return err
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

func GetPathFilledForm(iSvc string, iFfn string) string {
   return formDir(iSvc) + _ffnFileName(iSvc, iFfn) // suffix appended by client
}

func GetRecordFilledForm(iSvc string, iFfn, iMsgId string) Msg {
   aFd, err := os.Open(formDir(iSvc) + _ffnFileName(iSvc, iFfn))
   if err != nil { quit(err) }
   defer aFd.Close()
   aData := []Msg{}
   err = json.NewDecoder(aFd).Decode(aData)
   if err != nil { quit(err) }
   for _, aV := range aData {
      if aV["msgid"].(string) == iMsgId {
         return aV
      }
   }
   return nil
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
   if aResult != nil { return tError(aResult) }
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
              iData []byte, iR io.Reader) error {
   var err error
   aFmTbl := _ffnFileName(iSvc, iFile.Ffn)
   aFn := tempDir(iSvc) + iMsgId + "_" + aFmTbl + ".tmp"
   aFd, err := os.OpenFile(aFn, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }
   defer aFd.Close()

   var aFi os.FileInfo
   aFi, err = os.Lstat(formDir(iSvc) + aFmTbl + iSuffix)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aPos := int64(2); if err == nil { aPos = aFi.Size() }
   _, err = aFd.Write([]byte(fmt.Sprintf("%016x%016x", aPos, aPos))) // 2 copies for safety
   if err != nil { quit(err) }

   aCw := tCrcWriter{}
   aTee := io.MultiWriter(aFd, &aCw)
   aSize := iFile.Size - 1 // omit closing '}'
   aLen := int64(len(iData)); if aLen > aSize { aLen = aSize }
   _, err = aTee.Write(iData[:aLen])
   if err != nil { quit(err) }
   _, err = io.CopyN(aTee, iR, aSize - aLen)
   if err != nil {
      os.Remove(aFn)
      return err //todo only return network error
   }
   _, err = aTee.Write([]byte(fmt.Sprintf(`,"threadid":"%s","msgid":"%s"`, iThreadId, iMsgId)))
   if err != nil { quit(err) }
   aCw.Write([]byte{'}'}) // include closing '}' in checksum
   if iFile.Size > int64(len(iData)) { iR.Read([]byte{0}) }
   _, err = aFd.Write([]byte(fmt.Sprintf(`,"checksum":%d}`, aCw.sum)))
   if err != nil { quit(err) }

   err = aFd.Sync()
   if err != nil { quit(err) }
   return nil
}

func storeFilledForm(iSvc string, iMsgId string, iSuffix string, iFile *tHeader2Attach) bool {
   var err error
   aFmTbl := _ffnFileName(iSvc, iFile.Ffn)
   aFn := tempDir(iSvc) + iMsgId + "_" + aFmTbl + ".tmp"
   aTd, err := os.Open(aFn)
   if err != nil { quit(err) }
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
   aPath := formDir(iSvc) + aFmTbl + iSuffix
   _, err = os.Lstat(aPath)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aDoSync := err != nil
   aFd, err := os.OpenFile(aPath, os.O_WRONLY|os.O_CREATE, 0600)
   if err != nil { quit(err) }
   if aPos[0] == 2 {
      _, err = aFd.Write([]byte{'[','\n'})
   } else {
      _, err = aFd.Seek(int64(aPos[0])-1, io.SeekStart)
      if err != nil { quit(err) }
      _, err = aFd.Write([]byte{',','\n','\n'})
   }
   if err != nil { quit(err) }
   _, err = io.Copy(aFd, aTd)
   if err != nil { quit(err) }
   _, err = aFd.Write([]byte{']'})
   if err != nil { quit(err) }
   err = aFd.Sync()
   if err != nil { quit(err) }
   aFd.Close()
   aTd.Close()
   return aDoSync
}

func _ffnFileName(iSvc, iFfn string) string {
   aUri := getUriService(iSvc)
   if strings.HasPrefix(iFfn, aUri) {
      return iFfn[len(aUri):]
   } else {
      return strings.Replace(iFfn, "/", "%", -1)
   }
}

