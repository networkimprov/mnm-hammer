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

   mnm.ConfigUpdt = function(iObj) { // with addr, verify
      _wsSend({op:'config_update', config:iObj})
   };

   mnm.OhiAdd = function(iAliasTo, iUid) {
      _wsSend({op:'ohi_add', ohi:{alias:iAliasTo, uid:iUid}})
   };
   mnm.OhiDrop = function(iAliasTo, iUid) {
      _wsSend({op:'ohi_drop', ohi:{alias:iAliasTo, uid:iUid}})
   };

   mnm.PingSave = function(iObj) { // with alias, to, text, gid
      _wsSend({op:'ping_save', ping:iObj})
   };
   mnm.PingDiscard = function(iObj) { // with to, gid
      _wsSend({op:'ping_discard', ping:iObj})
   };
   mnm.PingSend = function(i) {
      _wsSend({op:'ping_send', ping:{qid:i}})
   };
   mnm.InviteAccept = function(i) {
      _wsSend({op:'accept_send', accept:{qid:i}})
   };
   mnm.AdrsbkSearch = function(iType, iTerm) {
      _wsSend({op:'adrsbk_search', adrsbk:{type:iType, term:iTerm}})
   };

   mnm.NoticeOpen = function(iSvc) {
      _xhr('nlo', iSvc)
   };
   mnm.NoticeSeen = function(iMsgId) {
      _wsSend({op:'notice_seen', notice:{msgid:iMsgId}})
   };

   mnm.NavigateThread = function(i) {
      _wsSend({op:'navigate_thread', navigate:{threadId:i}})
   };
   mnm.NavigateHistory = function(i) {
      _wsSend({op:'navigate_history', navigate:{history:i}})
   };
   mnm.NavigateLink = function(i) {
      var aPair = i.substr(i.indexOf('#')+1).split('&');
      _wsSend({op:'navigate_link', navigate:{threadId:aPair[0], msgId:aPair[1] || aPair[0]}})
   };

   mnm.ThreadNew = function(iObj) { // with alias, (cc), (data), (attach), (formFill)
      iObj.new = 1;
      _wsSend({op:'thread_save', thread:iObj})
   };
   mnm.ThreadReply = function(iObj) { // with alias, (data), (attach), (formFill)
      iObj.new = 2;
      _wsSend({op:'thread_save', thread:iObj})
   };
   mnm.ThreadSave = function(iObj) { // with id, alias, (cc), (data), (attach), (formFill)
      delete iObj.new // just in case
      _wsSend({op:'thread_save', thread:iObj})
   };
   mnm.ThreadRecv = function() {
      _wsSend({op:'thread_recvtest', thread:{}})
   };
   mnm.ThreadOpen = function(iId) {
      _xhr('mn', iId, true);
   };
   mnm.ThreadClose = function(iId) {
      _wsSend({op:'thread_close', thread:{id:iId}})
   };
   mnm.ThreadSend = function(iId) {
      _wsSend({op:'thread_send', thread:{id:iId}})
   };
   mnm.ThreadDiscard = function(iId) {
      _wsSend({op:'thread_discard', thread:{id:iId}})
   };

   mnm.ForwardSave = function(iId, iCc) {
      _wsSend({op:'forward_save', forward:{threadId:iId, cc:iCc}})
   };
   mnm.ForwardSend = function(iId, iQid) {
      _wsSend({op:'forward_send', forward:{threadId:iId, qid:iQid}})
   };

   mnm.TabAdd = function(iObj) { // with type, term
      _wsSend({op:'tab_add', tab:iObj})
   };
   mnm.TabSelect = function(iObj) { // with type, posfor, pos
      _wsSend({op:'tab_select', tab:iObj})
   };
   mnm.TabPin = function(iType) {
      _wsSend({op:'tab_pin', tab:{type:iType}})
   };
   mnm.TabDrop = function(iType) {
      _wsSend({op:'tab_drop', tab:{type:iType}})
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
         mnm.Log(iForm.action +' '+ aXhr.responseText);
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
         mnm.Log('ws '+ iEvent.data);
         if (iEvent.data.charAt(0) !== '[')
            return;
         var aObj = JSON.parse(iEvent.data);
         for (var a=0; a < aObj.length; ++a) {
            if (aObj[a] === '_n') {
               mnm.Render('nameset', null, aObj.slice(a+1));
               break;
            }
            if (aObj[a] === '_t')
               mnm.ThreadChange();
            else if (aObj[a] === '_e')
               mnm.Log('ws '+ aObj[++a]);
            else if (aObj[a] === 'mn' || aObj[a] === 'an' || aObj[a] === 'fn')
               _xhr(aObj[a], aObj[++a]);
            else
               _xhr(aObj[a]);
         }
      };
      sWs.onclose = function(iEvent) { mnm.Log('ws closed') };
      sWs.onerror = function(iEvent) { mnm.Log('ws error: ' + iEvent.data) };
   };

   function _xhr(i, iId, iOpen) {
      ++sXhrPending;
      var aXhr = new XMLHttpRequest();
      aXhr.onload = function() {
         --sXhrPending;
         if (aXhr.status !== 200) {
            mnm.Log(i +' '+ aXhr.responseText);
            return;
         }
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
               if (iOpen)
                  _wsSend({op:'thread_open', thread:
                              {id:aHead.Id, threadid:aHead.SubHead.ThreadId || aHead.Id}});
               return;
            }
            aMap[aHead.Id] = aHead;
         }
         mnm.Render(i, aXhr.responseText, aMap);
      };
      if (i === 'nlo') {
         aXhr.open('GET', '/'+ encodeURIComponent(iId) +'?nl');
      } else {
         var aN = iId ? encodeURIComponent(iId) : '';
         aXhr.open('GET', i.charAt(0) === '/' ? i+'/'+aN : '?'+i+(aN && '='+aN));
      }
      aXhr.send();
   }

   function _wsSend(i) {
      if (sWs.readyState !== 1) {
         mnm.Log('ws op failed on closed socket');
      } else if (sXhrPending > 0) {
         setTimeout(_wsSend, 5, i);
         mnm.Log('ws op deferred for pending xhr');
      } else {
         sWs.send(JSON.stringify(i));
      }
   }

}).call(this);

