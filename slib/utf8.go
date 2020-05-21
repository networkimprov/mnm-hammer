// Copyright 2020 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

package slib

import (
   "io"
   "unicode/utf8"
)

type tCountUtf8 struct {
   r io.Reader
   expect int64    // does not constrain total read
   charLen int64
   last [3]byte
   lastN int
}

func NewCountUtf8(iR io.Reader, iExpect int64) *tCountUtf8 { return &tCountUtf8{r:iR, expect:iExpect} }

func (o *tCountUtf8) Utf16Len() int64 { return o.charLen } // for javascript

func (o *tCountUtf8) Read(iBuf []byte) (int, error) {
   aLen, err := o.r.Read(iBuf[o.lastN:]) // assume Read() following error returns error
   if o.lastN > 0 {
      copy(iBuf, o.last[:o.lastN]) // assume len(iBuf) >= o.lastN
      aLen += o.lastN
      o.lastN = 0
   }
   for a, aStep := 0, 0; a < aLen; a += aStep {
      var aR rune
      aR, aStep = utf8.DecodeRune(iBuf[a:aLen])
      if aR != utf8.RuneError {
         if aR >= 0x10000 {
            o.charLen++
         }
         o.lastN = 0
      } else {
         switch o.lastN {
         case 3:
            o.lastN--
            o.last[0], o.last[1] = o.last[1], o.last[2]
            fallthrough
         case 0, 1, 2:
            o.last[o.lastN] = iBuf[a]
            //todo reconsider: iBuf[a] = 0xFF // browser will display \uFFFD
         }
         o.lastN++
      }
      o.charLen++
   }
   if o.expect - int64(aLen) > 0 && err == nil {
      o.charLen -= int64(o.lastN)
      aLen -= o.lastN
   } else {
      o.lastN = 0
   }
   if aLen > 0 {
      o.expect -= int64(aLen)
      if o.expect < 0 {
         quit(tError("exceeded expected amount"))
      }
      err = nil
   }
   return aLen, err
}
