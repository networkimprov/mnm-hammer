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
   "encoding/json"
   "os"
   "strconv"
)


func GetPathForm(iSvc string, iFormId string) string {
   return formDir(iSvc) + iFormId
}

func GetRecordForm(iSvc string, iFormId, iMsgId string) Msg {
   aFd, err := os.Open(formDir(iSvc) + iFormId)
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

func tempForm(iSvc string, iThreadId, iMsgId string, iSuffix string, iFile *tHeader2Attach,
              iData []byte, iR io.Reader) error {
   var err error
   var aFd *os.File
   aFn := tempDir(iSvc) + iMsgId + "_" + iFile.Name[10:] + ".tmp"
   aFd, err = os.OpenFile(aFn, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
   if err != nil { quit(err) }

   var aFi os.FileInfo
   aFi, err = os.Lstat(formDir(iSvc) + iFile.Name[10:] + iSuffix)
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
   aFd.Close()
   return nil
}

func storeForm(iSvc string, iMsgId string, iSuffix string, iFile *tHeader2Attach) bool {
   var err error
   var aFd, aTd *os.File
   aFn := tempDir(iSvc) + iMsgId + "_" + iFile.Name[10:] + ".tmp"
   aTd, err = os.Open(aFn)
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
   aFtable := formDir(iSvc) + iFile.Name[10:] + iSuffix
   _, err = os.Lstat(aFtable)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aDoSync := err != nil
   aFd, err = os.OpenFile(aFtable, os.O_WRONLY|os.O_CREATE, 0600)
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

