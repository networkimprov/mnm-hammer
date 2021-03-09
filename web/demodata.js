// Copyright 2021 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

;(function() {
   var sConnect = mnm.Connect;
   mnm.Connect = function() {
      sConnect();
      if (location.hash === '#demodata')
         setTimeout(_logData, 1000);
   };

   function _logData() {
      var aLog = '';
      var aDataV = mnm._data.v.slice();
      for (var a = aDataV.length-1; a >= 0; --a)
         if (aDataV[a].Name.startsWith('Alt'))
            aDataV.splice(a, 1);
      if (location.pathname === '/') {
         aLog += '{\n';
         var aGlobal = ['/v', '/t', '/f', '/g', '/l'];
         aGlobal.forEach(function(c) {
            aLog += '   "'+ c +'":'+ JSON.stringify(c === '/v' ? aDataV : mnm._data[c[1]]) +',\n';
         });
         aLog += '   "S": {\n';
         fSvc(0);
      } else {
         mnm._data.cs.History.Prev = mnm._data.cs.History.Next = false;
         var aSset = ['cs', 'cf', 'cn', 'tl', 'fl', 'ps', 'pt', 'pf', 'gl', 'ot', 'of'];
         var aTset = ['cl', 'al', 'ml', 'mo'];
         var aAdrsbk = [];
         aLog += '      '+ JSON.stringify(mnm._data.cf.Name) +':{\n';
         aSset.forEach(function(c) {
            aLog += '         "'+ c +'":'+ JSON.stringify(mnm._data[c]) +',\n';
         });
         mnm.NoticeOpen(mnm._data.cf.Name);
         setTimeout(function() {
            aLog += '         "nl":'+ JSON.stringify(mnm._data.nlo) +',\n';
            aLog += '         "T": {\n';
            fThread(0);
         }, 100);
         function fThread(cN) {
            if (cN < mnm._data.tl.length) {
               var cT = mnm._data.tl[cN];
               mnm.NavigateThread(cT.Id);
               setTimeout(function() {
                  aLog += '            '+ JSON.stringify(cT.Id) +':{\n';
                  aTset.forEach(function(c) {
                     aLog += '               "'+ c +'":'+ JSON.stringify(mnm._data[c]) +',\n';
                  });
                  aLog = aLog.slice(0, -2) +'\n            },\n';
                  fThread(cN+1);
               }, 500);
            } else {
               mnm._data.tl.forEach(function() { mnm.NavigateHistory(-1) });
               if (mnm._data.tl.length)
                  aLog = aLog.slice(0, -2) +'\n';
               aLog += '         },\n';
               aLog += '         "F": {\n';
               fFft(0);
            }
         }
         function fFft(cN) {
            if (cN < mnm._data.fl.length) {
               var cF = mnm._data.fl[cN];
               mnm.TabAdd({type:1, term:'ffn:'+ cF.Id});
               setTimeout(function() {
                  aLog += '            '+ JSON.stringify(cF.Id) +':'+ JSON.stringify(mnm._data.tl) +',\n';
                  mnm.TabDrop(1);
                  fFft(cN+1);
               }, 500);
            } else {
               if (mnm._data.fl.length)
                  aLog = aLog.slice(0, -2) +'\n';
               aLog += '         },\n';
               fMatch('a');
            }
         }
         function fMatch(cLtr) {
            if (cLtr <= 'z') {
               mnm.AdrsbkSearch(3, cLtr);
               setTimeout(function() {
                  for (var c in mnm._data.adrsbkmenuId)
                     if (c !== mnm._data.cf.Alias)
                        aAdrsbk.push([c, mnm._data.adrsbkmenuId[c]]);
                  fMatch(String.fromCharCode(cLtr.charCodeAt(0)+1));
               }, 50);
            } else {
               aAdrsbk = aAdrsbk.filter(function(cEl, cN) {
                  return aAdrsbk.findIndex(function(c) { return c[0] === cEl[0] }) === cN;
               });
               aLog += '         "A": '+ JSON.stringify(aAdrsbk) +'\n';
               aLog += '      },\n';
               fSvc(1 + aDataV.findIndex(function(c) { return c.Name === mnm._data.cf.Name }));
            }
         }
      }
      function fSvc(cN) {
         if (cN === aDataV.length) {
            if (aDataV.length)
               aLog = aLog.slice(0, -2) +'\n';
            aLog += '   }}';
         }
         sessionStorage.setItem('demodata', cN === 0 ? aLog : sessionStorage.getItem('demodata') + aLog);
         if (cN < aDataV.length)
            setTimeout(function() { location.pathname = aDataV[cN].Name }, 500);
         else
            console.log(sessionStorage.getItem('demodata'));
      }
   }

}).call(this);

