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

   pBkeyword  "github.com/blevesearch/bleve/analysis/analyzer/keyword"
   pBleve     "github.com/blevesearch/bleve"
   pBquery    "github.com/blevesearch/bleve/search/query"
   pBscorch   "github.com/blevesearch/bleve/index/scorch"
   pBsearch   "github.com/blevesearch/bleve/search"
)

var kSearchIndexRev = []byte("0.8")

type tSearchEl struct {
   Id string
   Count uint32
   Subject string
   SubjectWas string `json:",omitempty"`
   OrigCc []string
   OrigDate, LastDate string
   OrigAuthor, LastAuthor string
   Unread bool `json:",omitempty"`
}

type tSearchDoc struct {
   id string
   Count uint32
   Subject tStrings
   Author tStrings // excludes self
   Tag tStrings
   OrigCc tStrings
   OrigAuthor, LastAuthor string
   OrigDate, LastDate string
   LastSubjectN int // ref to Subject item
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

var kResultFields = []string{"*"} //todo list fields?

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
      err = writeTableFilledForm(iW, iSvc, aFfn)
      if err != nil { return err }
      _, err = iW.Write([]byte{'}'})
      return err
   }
   var aQ pBquery.Query
   if aTabType != ePosForDefault || aTabVal[0] == '#' {
      aQ = _makeWordsQuery(aTabVal)
   } else if aTabVal == "All" {
      aQ = pBleve.NewMatchAllQuery()
   } else if aTabVal == "Unread" {
      aQb := pBleve.NewBoolFieldQuery(true)
      aQb.SetField("Unread")
      aQ = aQb
   }
   if aQ == nil {
      _, err = iW.Write([]byte{'[',']'})
      return err
   }
   aBi := getService(iSvc).index
   aSr := pBleve.NewSearchRequestOptions(aQ, 1024, 0, false)
   aSr.Fields = kResultFields
   aSet, err := aBi.Search(aSr)
   if err != nil { quit(err) }
   aList := make([]tSearchEl, 0, len(aSet.Hits))
   for _, aHit := range aSet.Hits {
      aSubject := _i2slice(aHit.Fields["Subject"])
      //aAuthor  := _i2slice(aHit.Fields["Author"])
      //aTag     := _i2slice(aHit.Fields["Tag"])
      aOrigCc  := _i2slice(aHit.Fields["OrigCc"])
      aOrigCcSet := make([]string, len(aOrigCc))
      for a := range aOrigCc { aOrigCcSet[a] = aOrigCc[a].(string) }
      aLastSubjectN := int(aHit.Fields["LastSubjectN"].(float64))
      aList = append(aList, tSearchEl{Id:         aHit.ID,
                                      Count:      uint32(aHit.Fields["Count"].(float64)),
                                      Subject:    aSubject[aLastSubjectN].(string),
                                      OrigCc:     aOrigCcSet,
                                      OrigDate:   aHit.Fields["OrigDate"].(string),
                                      LastDate:   aHit.Fields["LastDate"].(string),
                                      OrigAuthor: aHit.Fields["OrigAuthor"].(string),
                                      LastAuthor: aHit.Fields["LastAuthor"].(string),
                                      Unread:     aHit.Fields["Unread"].(bool)})
      if aLastSubjectN != 0 {
         aList[len(aList)-1].SubjectWas = aSubject[0].(string)
      }
   }
   sort.Slice(aList, func(cA, cB int) bool { return aList[cA].LastDate > aList[cB].LastDate })
   err = json.NewEncoder(iW).Encode(aList)
   return err
}

func _makeWordsQuery(iWords string) pBquery.Query {
   aWordSet := strings.Fields(iWords)
   if len(aWordSet) == 0 {
      return nil
   }
   aLast := aWordSet[len(aWordSet)-1]
   if aWordSet[0][0] == '=' || aLast[len(aLast)-1] == '=' {
      return pBleve.NewMatchPhraseQuery(iWords)
   }
   if aWordSet[0][0] == '#' || aLast[len(aLast)-1] == '#' {
      if aWordSet[0][0] == '#' {
         iWords = iWords[strings.IndexByte(iWords, '#') + 1:]
      } else {
         iWords = iWords[:strings.LastIndexByte(iWords, '#')]
      }
      aTag := GetIdTag(iWords)
      if aTag == "" {
         return nil
      }
      return pBquery.NewPhraseQuery([]string{aTag}, "Tag") 
   }
   aOpSet := make([]byte, len(aWordSet))
   for a := len(aWordSet)-1; a >= 0; a-- {
      for len(aWordSet[a]) > 0 && (aWordSet[a][0] == '+' || aWordSet[a][0] == '-') {
         aOpSet[a] = aWordSet[a][0]
         aWordSet[a] = aWordSet[a][1:]
      }
      if len(aWordSet[a]) == 0 {
         aWordSet = aWordSet[:a + copy(aWordSet[a:], aWordSet[a+1:])]
         if a < len(aOpSet)-1 { // assign op to next word unless it has one
            aA := a; if aOpSet[a+1] == 0 { aA++ }
            aOpSet = aOpSet[:aA + copy(aOpSet[aA:], aOpSet[aA+1:])]
         } else {
            aOpSet = aOpSet[:a]
         }
      }
   }
   if len(aWordSet) == 1 && aOpSet[0] != '-' {
      return pBquery.NewMatchQuery(aWordSet[0])
   }
   aMust := make([]pBquery.Query, 0, len(aWordSet))
   aNot  := make([]pBquery.Query, 0, len(aWordSet))
   aShld := make([]pBquery.Query, 0, len(aWordSet))
   for a := range aWordSet {
      aRef := &aShld
      switch aOpSet[a] {
      case '+': aRef = &aMust
      case '-': aRef = &aNot
      }
      *aRef = append(*aRef, pBquery.NewMatchQuery(aWordSet[a]))
   }
   return pBquery.NewBooleanQuery(aMust, aShld, aNot)
}

func _i2slice(i interface{}) []interface{} { // bleve stores string for input []string{s}
   switch aV := i.(type) {
   case []interface{}: return aV
   case nil:           return nil
   default:            return []interface{}{aV}
   }
}

func countUnreadSearch(iSvc string) int {
   aQb := pBleve.NewBoolFieldQuery(true)
   aQb.SetField("Unread")
   aSr := pBleve.NewSearchRequestOptions(aQb, 0, 0, false)
   aBi := getService(iSvc).index
   aSet, err := aBi.Search(aSr)
   if err != nil { quit(err) }
   return int(aSet.Total)
}

type tTermSites pBsearch.TermLocationMap

var kTermSitesEmpty = tTermSites{}
var kResultFieldsMsg = []string{"Body"}

func messageSearch(iSvc string, iTid string, iTerm string) tTermSites {
   aBi := getService(iSvc).index
   aQ := pBleve.NewConjunctionQuery(pBleve.NewDocIDQuery([]string{iTid}),
                                    pBleve.NewMatchPhraseQuery(iTerm))
   aSr := pBleve.NewSearchRequest(aQ)
   aSr.Fields = kResultFieldsMsg
   aSet, err := aBi.Search(aSr)
   if err != nil { quit(err) }
   if len(aSet.Hits) > 1 {
      quit(fmt.Errorf("search result got %d hits; expected 1", len(aSet.Hits)))
   } else if len(aSet.Hits) == 0 {
      return kTermSitesEmpty
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

func openIndexSearch(iCfg *tSvcConfig) pBleve.Index {
   aPath := fileIndex(iCfg.Name)
   aTemp := aPath + ".tmp"
   err := os.RemoveAll(aTemp)
   if err != nil && !os.IsNotExist(err) { quit(err) }
   aBi, err := pBleve.Open(aPath)
   if err == nil {
      var aRev []byte
      aRev, err = aBi.GetInternal([]byte{'v'})
      if err != nil { quit(err) }
      if bytes.Compare(aRev, kSearchIndexRev) != 0 {
         err = aBi.Close()
         if err != nil { quit(err) }
         err = os.Rename(aPath, aTemp)
         if err != nil { quit(err) }
         aBi = openIndexSearch(iCfg)
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
   aKtext := pBleve.NewTextFieldMapping()
   aKtext.Analyzer = pBkeyword.Name
   aKtext.Store = false
   aNnumr := pBleve.NewNumericFieldMapping()
   aNnumr.Index = false
   aFbool := pBleve.NewBooleanFieldMapping()

   aThread := pBleve.NewDocumentMapping()
   aThread.AddFieldMappingsAt("Count", aNnumr)
   aThread.AddFieldMappingsAt("Subject", aFtext)
   aThread.AddFieldMappingsAt("Author", aFtext)
   aThread.AddFieldMappingsAt("Tag", aKtext)
   aThread.AddFieldMappingsAt("OrigCc", aNtext)
   aThread.AddFieldMappingsAt("OrigDate", aNtext)
   aThread.AddFieldMappingsAt("LastDate", aNtext)
   aThread.AddFieldMappingsAt("OrigAuthor", aNtext)
   aThread.AddFieldMappingsAt("LastAuthor", aNtext)
   aThread.AddFieldMappingsAt("LastSubjectN", aNnumr)
   aThread.AddFieldMappingsAt("Unread", aFbool)
   aThread.AddFieldMappingsAt("Body", aBtext)
   aIm.AddDocumentMapping("thread", aThread)

   aBi, err = pBleve.New(aTemp, aIm)
   if err != nil { quit(err) }
   _reindex(iCfg, aBi)
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

func _reindex(iCfg *tSvcConfig, iBi pBleve.Index) {
   aTx := iBi.NewBatch()
   aDir, err := readDirNames(dirThread(iCfg.Name))
   if err != nil { quit(err) }
   if len(aDir) > 0 {
      fmt.Printf("_reindex %s: Indexing %d threads...", iCfg.Name, len(aDir))
   }
   for _, aFn := range aDir {
      if strings.ContainsRune(aFn[1:], '_') || strings.HasSuffix(aFn, ".bak") { continue }
      var aFd *os.File
      aFd, err = os.Open(dirThread(iCfg.Name) + aFn)
      if err != nil { quit(err) }
      _updateSearchDoc(iCfg.Name, iCfg, aFn, aFd, aTx)
      aFd.Close()
   }
   if len(aDir) > 0 {
      fmt.Printf(" done\n")
   }
   aTx.SetInternal([]byte{'v'}, kSearchIndexRev)
   err = iBi.Batch(aTx)
   if err != nil { quit(err) }
}
