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
   "sort"
   "strings"
   "time"

   pBleve     "github.com/blevesearch/bleve"
   pBquery    "github.com/blevesearch/bleve/search/query"
   pBscorch   "github.com/blevesearch/bleve/index/scorch"
   pBsearch   "github.com/blevesearch/bleve/search"
)

var sSearchIndexRev = []byte("0.6b")

type tSearchEl struct {
   Id string
   Subject string
   OrigDate, LastDate string
   OrigAuthor, LastAuthor string
   Unread bool `json:",omitempty"`
}

type tSearchDoc struct {
   id string
   Subject tStrings
   Author tStrings // excludes self
   OrigAuthor, LastAuthor string
   OrigDate, LastDate string
   Unread bool
   Body string
   bodyStream io.Reader
}

func (*tSearchDoc) Type() string { return "thread" }

type tStrings []string

func (o *tStrings) addUnique(i string) {
   for a := range *o {
      if (*o)[a] == i {
         return
      }
   }
   *o = append(*o, i)
}

type tIndexer interface {
   Index(string, interface{}) error
}

var sResultFields = []string{"*"} //todo list fields?

func WriteResultSearch(iW io.Writer, iSvc string, iState *ClientState) error {
   var err error
   aTabType, aTabVal := iState.getSvcTab()
   if aTabType == ePosForDefault && aTabVal == "FFT" {
      var aDir []os.FileInfo
      aDir, err = ioutil.ReadDir(dirForm(iSvc))
      if err != nil { quit(err) }
      aList := make([]tSearchEl, 0, len(aDir))
      for _, aFi := range aDir {
         aList = append(aList, tSearchEl{LastDate: aFi.ModTime().UTC().Format(time.RFC3339),
                                         Id: strings.Replace(aFi.Name(), "@", "/", -1)})
      }
      err = json.NewEncoder(iW).Encode(aList)
      return err
   }
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
   var aQ pBquery.Query
   if aTabType != ePosForDefault {
      aQ = pBleve.NewMatchPhraseQuery(aTabVal)
   } else if aTabVal == "All" {
      aQ = pBleve.NewMatchAllQuery()
   } else if aTabVal == "Unread" {
      aQb := pBleve.NewBoolFieldQuery(true)
      aQb.SetField("Unread")
      aQ = aQb
   } else if aTabVal == "Todo" {
      aQ = pBleve.NewMatchNoneQuery()
   }
   aBi := getService(iSvc).index
   aSr := pBleve.NewSearchRequest(aQ)
   aSr.Fields = sResultFields
   aSet, err := aBi.Search(aSr)
   if err != nil { quit(err) }
   aList := make([]tSearchEl, 0, len(aSet.Hits))
   for _, aHit := range aSet.Hits {
      aSubject := _i2slice(aHit.Fields["Subject"])
      //aAuthor  := _i2slice(aHit.Fields["Author"])
      aList = append(aList, tSearchEl{Id:         aHit.ID,
                                      Subject:    aSubject[len(aSubject)-1].(string),
                                      OrigDate:   aHit.Fields["OrigDate"].(string),
                                      LastDate:   aHit.Fields["LastDate"].(string),
                                      OrigAuthor: aHit.Fields["OrigAuthor"].(string),
                                      LastAuthor: aHit.Fields["LastAuthor"].(string),
                                      Unread:     aHit.Fields["Unread"].(bool)})
   }
   sort.Slice(aList, func(cA, cB int) bool { return aList[cA].LastDate > aList[cB].LastDate })
   err = json.NewEncoder(iW).Encode(aList)
   return err
}

func _i2slice(i interface{}) []interface{} { // bleve stores string for input []string{s}
   switch aV := i.(type) {
   case []interface{}: return aV
   default:            return []interface{}{aV}
   }
}

type tTermSites pBsearch.TermLocationMap

var sTermSitesEmpty = tTermSites{}
var sResultFieldsMsg = []string{"Body"}

func messageSearch(iSvc string, iTid string, iTerm string) tTermSites {
   aBi := getService(iSvc).index
   aQ := pBleve.NewConjunctionQuery(pBleve.NewDocIDQuery([]string{iTid}),
                                    pBleve.NewMatchPhraseQuery(iTerm))
   aSr := pBleve.NewSearchRequest(aQ)
   aSr.Fields = sResultFieldsMsg
   aSet, err := aBi.Search(aSr)
   if err != nil { quit(err) }
   if len(aSet.Hits) > 1 {
      quit(fmt.Errorf("search result got %d hits; expected 1", len(aSet.Hits)))
   } else if len(aSet.Hits) == 0 {
      return sTermSitesEmpty
   }
   return tTermSites(aSet.Hits[0].Locations["Body"])
}

func (o tTermSites) hasTermBefore(iPos int64) bool {
   aFound := false
   for aTerm, aLoc := range o {
      a := -1
      for a = 0; a < len(aLoc); a++ {
         aFound = aFound || aLoc[a].End < uint64(iPos)
         if aLoc[a].Start >= uint64(iPos) { break }
      }
      o[aTerm] = aLoc[a:]
   }
   return aFound
}

func indexThreadSearch(iSvc string, iDoc *tSearchDoc, iI tIndexer) {
   if iI == nil {
      iI = getService(iSvc).index.(tIndexer)
   }
   var err error
   aData, err := ioutil.ReadAll(iDoc.bodyStream)
   if err != nil { quit(err) }
   iDoc.Body = string(aData)
   err = iI.Index(iDoc.id, iDoc)
   if err != nil { quit(err) }
}

func updateUnreadSearch(iSvc string, iTid string, iUnread bool) {
   //todo store status with SetInternal()?
}

func deleteThreadSearch(iSvc string, iTid string) {
   aBi := getService(iSvc).index
   err := aBi.Delete(iTid)
   if err != nil && err != pBleve.ErrorEmptyID { quit(err) }
}

func openIndexSearch(iSvc string) pBleve.Index {
   aPath := fileIndex(iSvc)
   aTemp := aPath + ".tmp"
   err := os.RemoveAll(aTemp)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aBi, err := pBleve.Open(aPath)
   if err == nil {
      var aRev []byte
      aRev, err = aBi.GetInternal([]byte{'v'})
      if err != nil { quit(err) }
      if bytes.Compare(aRev, sSearchIndexRev) != 0 {
         err = aBi.Close()
         if err != nil { quit(err) }
         err = os.Rename(aPath, aTemp)
         if err != nil { quit(err) }
         aBi = openIndexSearch(iSvc)
      }
      return aBi
   }
   if err != pBleve.ErrorIndexPathDoesNotExist { quit(err) }
   pBleve.Config.DefaultIndexType = pBscorch.Name
   aIm := pBleve.NewIndexMapping()
   aIm.TypeField = "type"
   aIm.DefaultAnalyzer = "en"

   aFtext := pBleve.NewTextFieldMapping()
   aBtext := pBleve.NewTextFieldMapping()
   aBtext.Store = false
   aNtext := pBleve.NewTextFieldMapping()
   aNtext.Index = false
   aFbool := pBleve.NewBooleanFieldMapping()
   aThread := pBleve.NewDocumentMapping()
   aThread.AddFieldMappingsAt("Subject", aFtext)
   aThread.AddFieldMappingsAt("Author", aFtext)
   aThread.AddFieldMappingsAt("OrigDate", aNtext)
   aThread.AddFieldMappingsAt("LastDate", aNtext)
   aThread.AddFieldMappingsAt("OrigAuthor", aNtext)
   aThread.AddFieldMappingsAt("LastAuthor", aNtext)
   aThread.AddFieldMappingsAt("Unread", aFbool)
   aThread.AddFieldMappingsAt("Body", aBtext)
   aIm.AddDocumentMapping("thread", aThread)

   aBi, err = pBleve.New(aTemp, aIm)
   if err != nil { quit(err) }
   _reindex(iSvc, aBi)
   err = aBi.Close()
   if err != nil { quit(err) }
   err = syncDir(aTemp) // in case bleve doesn't do so
   if err != nil { quit(err) }
   err = os.Rename(aTemp, aPath)
   if err != nil { quit(err) }
   aBi, err = pBleve.Open(aPath)
   if err != nil { quit(err) }
   return aBi
}

func _reindex(iSvc string, iBi pBleve.Index) {
   aTx := iBi.NewBatch()
   aDir, err := readDirNames(dirThread(iSvc))
   if err != nil { quit(err) }
   if len(aDir) > 0 {
      fmt.Printf("Indexing %d threads...", len(aDir))
   }
   for _, aFn := range aDir {
      if strings.ContainsRune(aFn[1:], '_') { continue }
      var aFd *os.File
      aFd, err = os.Open(dirThread(iSvc) + aFn)
      if err != nil { quit(err) }
      _updateSearchDoc(iSvc, aFn, aFd, aTx)
      aFd.Close()
   }
   if len(aDir) > 0 {
      fmt.Printf(" done\n")
   }
   aTx.SetInternal([]byte{'v'}, sSearchIndexRev)
   err = iBi.Batch(aTx)
   if err != nil { quit(err) }
}
