// Copyright 2017, 2019 Liam Breck
// Published at https://github.com/networkimprov/mnm-hammer
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/

;var mnm = {};

(function() {
   var sUrl = 'ws://'+ location.host +'/s/'+ location.pathname.split('/')[1];
   var sTouchSeen = 's'.charCodeAt(0);
   var sTouchTag = 't'.charCodeAt(0);
   var sTouchUntag = 'u'.charCodeAt(0);
   var sWs = {};
   var sXhrPending = 0;
   var sNotice = '';

   // caller implements these
   mnm.Log =
   mnm.Err =
   mnm.Quit =
   mnm.Render =
   mnm.HasMoId =
   mnm.ThreadChange = null;

   mnm.SiteAdd = function(iAddr) {
      _wsSend({op:'site_add', site:{addr:iAddr}})
   };
   mnm.SiteDrop = function() {
      _wsSend({op:'site_drop', site:{}})
   };

   mnm.ConfigUpdt = function(iObj) { // with addr, verify
      _wsSend({op:'config_update', config:iObj})
   };

   mnm.OhiAdd = function(iAliasTo, iUid) {
      _wsSend({op:'ohi_add', ohi:{alias:iAliasTo, uid:iUid}})
   };
   mnm.OhiDrop = function(iUid) {
      _wsSend({op:'ohi_drop', ohi:{uid:iUid}})
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
      sNotice = iSvc;
      _xhr('nlo', iSvc)
   };
   mnm.NoticeClose = function() {
      sNotice = ''
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
   mnm.NavigateLink = function(iLabel, iHref) {
      if (iLabel.length > 32) // also limited by ClientState.goLink()
         iLabel = iLabel.substring(0, 32) +'...';
      var aPair = iHref.substring(iHref.indexOf('#')+1).split('&');
      _wsSend({op:'navigate_link', navigate:{label:iLabel, threadId:aPair[0], msgId:aPair[1] || aPair[0]}})
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
   mnm.ThreadSend = function(iId) {
      _wsSend({op:'thread_send', thread:{id:iId}})
   };
   mnm.ThreadDiscard = function(iId) {
      _wsSend({op:'thread_discard', thread:{id:iId}})
   };

   mnm.ThreadOpen = function(iId) {
      _xhr('mn', iId, null, true) // sends thread_open from onload
   };
   mnm.ThreadClose = function(iId) {
      _wsSend({op:'thread_close', touch:{msgid:iId}})
   };
   mnm.ThreadTag = function(iId, iTag) {
      _wsSend({op:'thread_tag', touch:{msgid:iId, act:sTouchTag, tagid:iTag}})
   };
   mnm.ThreadUntag = function(iId, iTag) {
      _wsSend({op:'thread_tag', touch:{msgid:iId, act:sTouchUntag, tagid:iTag}})
   };

   mnm.ForwardSave = function(iId, iCc) {
      _wsSend({op:'forward_save', forward:{threadId:iId, cc:iCc}})
   };
   mnm.ForwardSend = function(iId, iQid) {
      _wsSend({op:'forward_send', forward:{threadId:iId, qid:iQid}})
   };

   mnm.TagAdd = function(iName) {
      _wsSend({op:'tag_add', tag:{name:iName}})
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

   mnm.SortSelect = function(iType, iField) {
      _wsSend({op:'sort_select', sort:{type:iType, field:iField}})
   };

   mnm.NodeAdd = function(iAddr, iPin, iNewnode) {
      _wsSend({op:'node_add', node:{addr:iAddr, pin:iPin, newnode:iNewnode}})
   };

   mnm.FileForm = function(iId, iCb) {
      _xhr('/ft', iId, iCb);
   };
   mnm.FileBlob = function(iId, iCb) {
      _xhr('/tb', iId, iCb);
   };

   mnm.AttachForm = function(iId, iCb) {
      _xhr('ant', iId, iCb);
   };
   mnm.AttachBlob = function(iId, iCb) {
      _xhr('anb', iId, iCb);
   };

   mnm.Upload = function(iForm, iCb) {
      if (iForm.method.toLowerCase() !== 'post' || !iForm.action)
         throw new Error('mnm.Upload: requires method=POST and valid action');
      var aXhr = new XMLHttpRequest();
      aXhr.onload = function() {
         mnm.Log('post '+ iForm.action +' '+ aXhr.responseText);
         if (aXhr.status !== 200)
            mnm.Err(aXhr.statusText +' '+ aXhr.responseText);
         else if (iCb)
            iCb();
      };
      aXhr.open('POST', iForm.action);
      aXhr.send(new FormData(iForm));
   };

   mnm.Connect = function() {
      sWs = new WebSocket(sUrl);
      sWs.onopen = function() {
         sWs.send(JSON.stringify({op:'open'}));
      };
      sWs.onmessage = function(iEvent, iMs) {
         if (sXhrPending > 0) {
            setTimeout(sWs.onmessage, 6, iEvent, iMs || Date.now());
            mnm.Log('ws message deferred for pending xhr');
            return;
         }
         if (iMs) //todo verify that deferred msgs are handled in order
            mnm.Log('ws handle deferred from '+ iMs);
         mnm.Log('ws '+ iEvent.data);

         var aObj = JSON.parse(iEvent.data);
         for (var a=0; a < aObj.length; ++a) {
            if (aObj[a] === '_n') {
               mnm.Render('nameset', null, aObj.slice(a+1));
               break;
            }
            switch (aObj[a]) {
            case '_t': case '_T':  mnm.ThreadChange(aObj[a] === '_T'); break;
            case '_e':             mnm.Err(aObj[++a]);                 break;
            case 'mn': case 'an':  _xhr(aObj[a], aObj[++a]);           break;
            case '_m':
               var aOld = aObj[++a], aNew = aObj[++a];
               if (mnm.HasMoId(aOld === '' ? aNew : aOld))
                  _xhr('mn', aNew);
               break;
            default:
               _xhr(aObj[a]);
               if (aObj[a] === '/v' && sNotice)
                  _xhr('nlo', sNotice);
            }
         }
      };
      sWs.onclose = function(iEvent) {
         mnm.Log('ws closed');
         mnm.Quit();
      };
      sWs.onerror = function(iEvent) {
         mnm.Log('ws error: ' + iEvent.data);
         mnm.Err(iEvent.data);
      };
   };

   function _xhr(i, iId, iCb, iOpen) {
      ++sXhrPending;
      var aXhr = new XMLHttpRequest();
      if (iCb) {
         aXhr.responseType = i[2] === 'b' ? 'blob' : '';
         i = i.slice(0, 2);
      } else if (i === 'mo' || i === 'mn') {
         aXhr.responseType = 'arraybuffer';
      }
      aXhr.onload = function() {
         --sXhrPending;
         if (aXhr.status !== 200) {
            var aTxt = (iId ? iId +' ' : '') +
                       (aXhr.responseType === 'arraybuffer' ? _decode(aXhr) :
                        aXhr.responseType === 'blob' ? '[blob]' : aXhr.responseText);
            mnm.Log('get '+ i +' '+ aTxt);
            mnm.Err(aXhr.statusText +' '+ aTxt);
            return;
         }
         if (i !== 'mo' && i !== 'mn') {
            if (iCb)
               iCb(aXhr.response, iId);
            else
               mnm.Render(i, aXhr.responseText, iId);
            return;
         }
         var aMap = {};
         for (var a=0; a < aXhr.response.byteLength; ++a) {
            var aHeadLen = parseInt(_decode(aXhr, a, 4), 16);
            var aHead = JSON.parse(_decode(aXhr, a+4, aHeadLen));
            var aMsgLen = 'Size' in aHead ? aHead.Size : aHead.Len; // .Size appears in v0.8.0
            aHead.msg_data = _decode(aXhr, a+4+aHeadLen+1, aMsgLen);
            a += 4 + aHeadLen + 1 + aMsgLen;
            if (aHead.From === 'self' && aHead.SubHead.Attach) {
               aHead.form_fill = null;
               var aFormFill = {};
               var aAtc = aHead.SubHead.Attach;
               for (var aA=0; aA < aAtc.length; ++aA) {
                  if (!/^r:/.test(aAtc[aA].Name))
                     continue;
                  aFormFill[aAtc[aA].FfKey] = _decode(aXhr, a, aAtc[aA].Size);
                  a += aAtc[aA].Size;
                  aHead.form_fill = aFormFill;
               }
            }
            if (i === 'mn') {
               mnm.Render(i, null, aHead);
               if (iOpen)
                  _wsSend({op:'thread_open', touch:{act:sTouchSeen, msgid:aHead.Id,
                                                    threadid:aHead.SubHead.ThreadId || aHead.Id}});
               return;
            }
            aMap[aHead.Id] = aHead;
         }
         mnm.Render(i, null, aMap);
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
      } else {
         sWs.send(JSON.stringify(i));
      }
   }

   var sUtf8 = new TextDecoder();
   function _decode(iXhr, iPos, iLen) {
      return sUtf8.decode(new Uint8Array(iXhr.response, iPos, iLen));
   }

}).call(this);

