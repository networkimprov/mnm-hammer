// Copyright 2017 Liam Breck
//
// This file is part of the "mnm" software. Anyone may redistribute mnm and/or modify
// it under the terms of the GNU Lesser General Public License version 3, as published
// by the Free Software Foundation. See www.gnu.org/licenses/
// Mnm is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See said License for details.

;var mnm = {};
(function() {
   var sWs = {};
   var sXhrPending = 0;

   // caller implements these
   mnm.Log = mnm.Render = mnm.ThreadChange = function(){};

   mnm.SvcAdd = function(iObj) { // with name, addr, alias, loginperiod
      _wsSend(JSON.stringify({op:'service_add', service:iObj}))
   };

   mnm.OhiAdd = function(iAliasTo) {
      _wsSend(JSON.stringify({op:'ohi_add', ohi:{alias:iAliasTo}}))
   };
   mnm.OhiDrop = function(iAliasTo, iUid) {
      _wsSend(JSON.stringify({op:'ohi_drop', ohi:{alias:iAliasTo, uid:iUid}}))
   };

   mnm.PingSave = function(iObj) { // with alias, to, text, gid
      _wsSend(JSON.stringify({op:'ping_save', ping:iObj}))
   };
   mnm.PingDiscard = function(iObj) { // with to, gid
      _wsSend(JSON.stringify({op:'ping_discard', ping:iObj}))
   };
   mnm.PingSend = function(iObj) { // with to, gid
      _wsSend(JSON.stringify({op:'ping_send', ping:iObj}))
   };
   mnm.InviteAccept = function(iGid) {
      _wsSend(JSON.stringify({op:'accept_send', accept:{gid:iGid}}))
   };

   mnm.NavigateThread = function(i) {
      _wsSend(JSON.stringify({op:'navigate_thread', navigate:{threadId:i}}))
   };
   mnm.NavigateHistory = function(i) {
      _wsSend(JSON.stringify({op:'navigate_history', navigate:{history:i}}))
   };
   mnm.NavigateLink = function(i) {
      var aPair = i.substr(i.indexOf('#')+1).split('&');
      _wsSend(JSON.stringify({op:'navigate_link', navigate:
                                 {threadId:aPair[0], msgId:aPair[1] || aPair[0]}}))
   };

   mnm.ThreadNew = function(iObj) { // with alias, (cc), (data), (attach), (formFill)
      iObj.new = 1;
      _wsSend(JSON.stringify({op:'thread_save', thread:iObj}))
   };
   mnm.ThreadReply = function(iObj) { // with alias, (cc), (data), (attach), (formFill)
      iObj.new = 2;
      _wsSend(JSON.stringify({op:'thread_save', thread:iObj}))
   };
   mnm.ThreadSave = function(iObj) { // with id, alias, (cc), (data), (attach), (formFill)
      delete iObj.new // just in case
      _wsSend(JSON.stringify({op:'thread_save', thread:iObj}))
   };
   mnm.ThreadRecv = function() {
      _wsSend(JSON.stringify({op:'thread_recvtest', thread:{}}))
   };
   mnm.ThreadOpen = function(iId) {
      _xhr('mn', iId);
   };
   mnm.ThreadClose = function(iId) {
      _wsSend(JSON.stringify({op:'thread_close', thread:{id:iId}}))
   };
   mnm.ThreadSend = function(iId) {
      _wsSend(JSON.stringify({op:'thread_send', thread:{id:iId}}))
   };
   mnm.ThreadDiscard = function(iId) {
      _wsSend(JSON.stringify({op:'thread_discard', thread:{id:iId}}))
   };

   mnm.TabAdd = function(iObj) { // with type, term
      _wsSend(JSON.stringify({op:'tab_add', tab:iObj}))
   };
   mnm.TabSelect = function(iObj) { // with type, posfor, pos
      _wsSend(JSON.stringify({op:'tab_select', tab:iObj}))
   };
   mnm.TabPin = function(iType) {
      _wsSend(JSON.stringify({op:'tab_pin', tab:{type:iType}}))
   };
   mnm.TabDrop = function(iType) {
      _wsSend(JSON.stringify({op:'tab_drop', tab:{type:iType}}))
   };

   mnm.FormOpen = function(iId) {
      _xhr('/f', iId);
   };
   mnm.AttachOpen = function(iId) {
      _xhr('an', iId);
   };

   mnm.Upload = function(iForm) {
      if (iForm.method.toLowerCase() !== 'post' || !iForm.action)
        throw 'mnm.Upload: requires method=POST and valid action'
      var aXhr = new XMLHttpRequest();
      aXhr.onload = function() {
         mnm.Log(aXhr.responseText);
      };
      aXhr.open('POST', iForm.action);
      aXhr.send(new FormData(iForm));
   };

   mnm.Connect = function() {
      var aSvc = window.location.pathname.split('/')[1];
      sWs = new WebSocket('ws://'+window.location.host+'/s/'+aSvc);
      sWs.onopen = function() {
         sWs.send(JSON.stringify({op:'open'}));
      };
      sWs.onmessage = function(iEvent) {
         mnm.Log(iEvent.data);
         var aObj = JSON.parse(iEvent.data);
         if (!(aObj instanceof Array))
            return;
         for (var a=0; a < aObj.length; ++a) {
            if (aObj[a] === 'ml')
               mnm.ThreadChange();
            if (aObj[a] === 'mn' || aObj[a] === 'an' || aObj[a] === 'fn')
               _xhr(aObj[a], aObj[++a]);
            else
               _xhr(aObj[a]);
         }
      };
      sWs.onclose = function(iEvent) { mnm.Log('closed') };
      sWs.onerror = function(iEvent) { mnm.Log('ws error: ' + iEvent.data) };
   };

   function _xhr(i, iId) {
      ++sXhrPending;
      var aXhr = new XMLHttpRequest();
      aXhr.onload = function() {
         --sXhrPending;
         if (i !== 'mo' && i !== 'mn') {
            mnm.Render(i, aXhr.responseText, iId);
            return;
         }
         var aMap = {};
         for (var a=0; a < aXhr.responseText.length; ++a) {
            var aHeadLen = parseInt(aXhr.responseText.substr(a, 4), 16);
            var aHead = JSON.parse(aXhr.responseText.substr(a+4, aHeadLen));
            aHead.msg_data = aXhr.responseText.substr(a+4+aHeadLen+1, aHead.Len);
            a += 4 + aHeadLen + 1 + aHead.Len;
            if (aHead.From === 'self' && aHead.SubHead.Attach) {
               aHead.form_fill = null;
               var aFormFill = {};
               var aAtc = aHead.SubHead.Attach;
               for (var aA=0; aA < aAtc.length; ++aA) {
                  if (!/^r:/.test(aAtc[aA].Name))
                     continue;
                  aFormFill[aAtc[aA].FfKey] = aXhr.responseText.substr(a, aAtc[aA].Size);
                  a += aAtc[aA].Size;
                  aHead.form_fill = aFormFill;
               }
            }
            if (i === 'mn') {
               mnm.Render(i, aXhr.responseText, aHead);
               return;
            }
            aMap[aHead.Id] = aHead;
         }
         mnm.Render(i, aXhr.responseText, aMap);
      };
      var aN = iId ? encodeURIComponent(iId) : '';
      aXhr.open('GET', i.charAt(0) === '/' ? i+'/'+aN : '?'+i+(aN && '='+aN));
      aXhr.send();
   }

   function _wsSend(i) {
      if (sWs.readyState !== 1) {
         mnm.Log('op failed on closed socket');
      } else if (sXhrPending > 0) {
         setTimeout(_wsSend, 5, i);
         mnm.Log('op deferred for pending xhr');
      } else {
         sWs.send(i);
      }
   }

}).call(this);

