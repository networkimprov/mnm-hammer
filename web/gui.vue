<!DOCTYPE html>
<!--
   Copyright 2018, 2019 Liam Breck
   Published at https://github.com/networkimprov/mnm-hammer

   This Source Code Form is subject to the terms of the Mozilla Public
   License, v. 2.0. If a copy of the MPL was not distributed with this
   file, You can obtain one at http://mozilla.org/MPL/2.0/
-->
<html><head>
   <title>[{.Title}] - mnm</title>
   <link rel="icon" href="/w/favicon.png"/>

   <meta charset="utf-8">
   <meta name="viewport" content="width=device-width, initial-scale=1">

   <link  href="/w/uikit-30.min.css" rel="stylesheet"/>
   <script src="/w/uikit-30.min.js"></script>
   <script src="/w/uikit-icons-30.min.js"></script>

   <script src="/w/vue-25.js"></script>
   <script src="/w/markdown-it-84.js"></script>
   <script src="/w/luxon-111.js"></script>
   <link  href="/w/vue-formgen-23.css" rel="stylesheet"/>
   <script src="/w/vue-formgen-23.js"></script>

   <link  href="/w/service.css" rel="stylesheet"/>
   <script src="/w/socket.js"></script>

   <!-- generated id attributes require 'x[y]_' prefix -->
</head><body>
<base target="_blank">

<div id="app"></div>

<input id="toclipboard" style="display:none">

<div id="app-quit" style="display:none">
   <div class="app-alert">
      <p>The mnm app has quit.</p>
      When it's back up, reload this tab.<br>
      <button onclick="location.reload()"
              title="Reload this tab"
              class="btn-icon"><span uk-icon="refresh"></span></button>
   </div></div>

<script type="text/x-template" id="mnm-main">
<div uk-grid class="uk-grid-small">

<mnm-adrsbkmenu ref="adrsbkmenu"/>

<div class="uk-width-2-5">
   <div class="uk-clearfix">
      <span style="padding-left:0.5em; display:inline-block">
         {{ msgTitle }}
         <span v-show="msgSubjects.length > 1"
               class="dropdown-icon">&nbsp;&#x25BD;&nbsp;</span>
      </span>
      <mnm-subject v-if="msgSubjects.length > 1"
                   :list="msgSubjects"/>
      <div class="uk-float-right">
         <span uk-icon="social" class="dropdown-icon">{{cl[1].length}}</span>
         <mnm-cc ref="cl"
                 :tid="ml.length ? ml[ml.length-1].Id : 'none'"/>
         <span uk-icon="location" class="dropdown-icon">{{al.length || '&nbsp;&nbsp;'}}</span>
         <mnm-attach ref="al"/>
         &nbsp;
         <button @click="mnm.ThreadNew({alias:cf.Alias, cc:[]})"
                 title="New thread draft"
                 class="btn-icon"><span uk-icon="pencil"></span></button>
         <button onclick="this.blur(); mnm.NavigateHistory(-1)"
                 :disabled="!cs.History || !cs.History.Prev"
                 class="uk-button uk-button-link">
            <span uk-icon="icon:arrow-left; ratio:1.6"></span></button>
         <button onclick="this.blur(); mnm.NavigateHistory( 1)"
                 :disabled="!cs.History || !cs.History.Next"
                 class="uk-button uk-button-link">
            <span uk-icon="icon:arrow-right; ratio:1.6"></span></button>
      </div>
   </div>
   <div uk-grid class="uk-grid-collapse">
      <mnm-tabs class="uk-width-expand"
                :set="msgTabset" :state="cs.ThreadTabs"/>
      <input @keyup.enter="tabSearch($event.target.value, cs.ThreadTabs)"
             :placeholder="' \u2315'" type="text"
             class="uk-width-1-6 search-box">
   </div>
   <div uk-height-viewport="offset-top:true; offset-bottom:true"
        class="firefox-minheight-fix uk-overflow-auto message-list">
      <ul class="uk-list uk-list-divider">
         <li v-for="aMsg in ml" :key="aMsg.Id"
             :class="{'message-edit': aMsg.From === '' && !aMsg.Queued}" style="margin:0">
            <span @click="msgToggle(aMsg.Id)"
                  class="message-title"
                  :class="{'message-title-edit': aMsg.From === '' && !aMsg.Queued,
                           'message-title-seen': aMsg.Seen !== ''}">
               <mnm-date :iso="aMsg.Date" ymd="md" hms="hm"/>
               <b>{{ aMsg.Alias || aMsg.From }}
                  <span v-if="aMsg.ForwardBy"
                        :title="'Forward by: '+aMsg.ForwardBy">
                     {{/failed$/.test(aMsg.ForwardBy) ? '[possibly forged]' : '[unverified]'}}</span>
               </b>
            </span>
            <div v-if="aMsg.Queued"
                 title="Awaiting link to server"
                 style="float:right; font-weight:bold"><span uk-icon="bolt"></span></div>
            <template v-if="aMsg.Id in mo">
               <div v-if="!('msg_data' in mo[aMsg.Id])"
                    class="uk-text-center"><span uk-icon="future"><!-- todo hourglass --></span></div>
               <template v-else-if="aMsg.From === '' && !aMsg.Queued">
                  <button @click="mnm.ThreadDiscard(aMsg.Id)"
                          title="Discard draft"
                          class="btn-iconred btn-floatr"><span uk-icon="trash"></span></button>
                  <mnm-draft :msgid="aMsg.Id"/>
               </template>
               <template v-else>
                  <div v-if="!aMsg.Queued"
                       style="float:right">
                     <a @click.prevent="mnm._toClipboard('#'+ cs.Thread +'&'+ aMsg.Id)"
                        title="Copy reference to clipboard"
                        :href="'#'+ cs.Thread +'&'+ aMsg.Id"><span uk-icon="link"></span></a>
                     <button @click="mnm.ThreadReply(getReplyTemplate(aMsg))"
                             title="New reply draft"
                             class="btn-icon"><span uk-icon="comment"></span></button>
                  </div>
                  <div v-if="aMsg.Subject">
                     Subject: {{ aMsg.Subject }}</div>
                  <div v-if="mo[aMsg.Id].SubHead.Attach">
                     Attached ({{ mo[aMsg.Id].SubHead.Attach.length }}):
                     <template v-for="aAtc in mo[aMsg.Id].SubHead.Attach">
                        <span v-if="'ForwardBy' in aMsg && !/failed$/.test(aMsg.ForwardBy)"
                              :title="'Awaiting receipt from '+ aMsg.ForwardBy">
                              class="">{{ aAtc.Name.substr(2) }}</span>
                        <span v-else-if="aAtc.Name.charAt(0) === 'r'"
                              @click="tabSearch('ffn:'+ aAtc.Ffn +
                                           (aMsg.From === cf.Uid ? '_sent' : '_recv'), cs.SvcTabs)"
                              class="uk-link">{{ aAtc.Name.substr(2) }}</span>
                        <template v-else>
                           <a :href="'?ad=' + encodeURIComponent(aMsg.Id +'_'+ aAtc.Name)">
                              <span uk-icon="download"></span></a
                          ><a :href="'?an=' + encodeURIComponent(aMsg.Id +'_'+ aAtc.Name)"
                              target="mnm_atc_[{.Title}]">{{ aAtc.Name.substr(2) }}</a>
                        </template>
                        &#x25CA;
                     </template>
                  </div>
                  <br>
                  <div v-if="!mo[aMsg.Id].msg_data">
                     <p><span uk-icon="comment"></span></p></div>
                  <mnm-markdown v-else
                                :src="mo[aMsg.Id].msg_data" :msgid="aMsg.Id"
                                :formreply="aMsg.Queued ? 'Q' : getReplyTemplate(aMsg)"/>
               </template>
            </template>
         </li></ul>
   </div>
</div>

<div class="uk-width-1-2">
   <span v-for="aCmp in draftRefs" :key="aCmp.msgid">
      <mnm-files @attach="aCmp.atcAdd(arguments[0])"
                 :toggle="'#t_'+aCmp.msgid" pos="right-top"/>
      <mnm-forms @attach="aCmp.atcAdd(arguments[0])"
                 :toggle="'#f_'+aCmp.msgid" pos="right-top"/>
   </span>
   <div class="uk-clearfix">
      <span class="uk-text-large">
         <span uk-icon="world"></span>
         [{.Title}]
      </span>
      <div class="uk-float-right">
         <span uk-icon="bell" class="dropdown-icon" style="font-weight:bold">{{nlNotSeen}}</span>
         <mnm-notice offset="2" pos="bottom-right"/>
         <span uk-icon="users" class="dropdown-icon">&nbsp;</span>
         <mnm-adrsbk/>
         <span @mousedown="ohiFrom = !ohiFrom" class="dropdown-icon">&nbsp;o/</span>
         &nbsp;
         <span uk-icon="push" class="dropdown-icon">&nbsp;</span>
         <mnm-files ref="t" pos="bottom-right"/>
         <span uk-icon="file-edit" class="dropdown-icon">&nbsp;</span>
         <mnm-forms ref="f" pos="bottom-right"/>
         &nbsp;
      </div>
   </div>
   <div uk-grid class="uk-grid-collapse">
      <ul uk-tab class="uk-width-expand"><li style="display:none"></li>
         <li v-for="(aTerm, aI) in cs.SvcTabs.Default"
             :class="{'uk-active': cs.SvcTabs.PosFor === 0 && cs.SvcTabs.Pos === aI}">
            <a @click.prevent="mnm.TabSelect({type:cs.SvcTabs.Type, posfor:0, pos:aI})" href="#">
               {{ aTerm }}</a>
         </li></ul>
      <input @keyup.enter="tabSearch($event.target.value, cs.SvcTabs)"
             :placeholder="' \u2315'" type="text"
             class="uk-width-1-2 search-box">
   </div>
   <mnm-tabs v-if="cs.SvcTabs.Pinned.length || cs.SvcTabs.Terms.length"
             :set="svcTabset" :state="cs.SvcTabs"/>
   <div class="uk-position-relative"><!-- context for ohi card -->
      <div uk-height-viewport="offset-top:true" class="firefox-minheight-fix uk-overflow-auto"
           :class="{'uk-background-muted':ffn}">
         <div v-if="!cf.Uid">
            <br>Welcome to mnm. See docs in the <span uk-icon="info"></span> menu at top right.</div>
         <template v-if="ffn">
            <table class="uk-table uk-table-small uk-table-hover uk-text-small">
               <tr>
                  <th v-for="(a, aKey) in ffnCol"
                      v-if="aKey.charAt(0) !== '$' || aKey === '$msgid'"
                      style="position:sticky; top:0" class="uk-background-muted">
                     {{ aKey === '$msgid' ? 'source' : aKey }}</th>
               </tr>
               <tr v-for="aRow in tl">
                  <td v-for="(a, aKey) in ffnCol"
                      v-if="aKey.charAt(0) !== '$' || aKey === '$msgid'">
                     <a v-if="aKey === '$msgid'"
                        onclick="mnm.NavigateLink(this.href); return false"
                        :href="'#'+ aRow.$threadid +'&'+ aRow.$msgid"><span uk-icon="mail"></span></a>
                     <table v-else-if="aRow[aKey] instanceof Object"
                            class="uk-table">
                        <tr>
                           <th v-for="(a, aSubKey) in aRow[aKey]"
                               style="padding:0 0.5em">{{aSubKey}}</th>
                        </tr><tr>
                           <td v-for="aSubRow in aRow[aKey]"
                               style="padding:0 0.5em">{{aSubRow}}</td>
                        </tr></table>
                     <template v-else>
                        {{aRow[aKey]}}</template>
                  </td>
               </tr>
            </table></template>
         <template v-else-if="cs.SvcTabs.PosFor === 0 && cs.SvcTabs.Default[cs.SvcTabs.Pos] === 'FFT'">
            <div v-for="aRow in tl" :key="aRow.Id"
                 @click="tabSearch('ffn:'+aRow.Id, cs.SvcTabs)"
                 uk-grid class="uk-grid-small thread-row">
               <div class="uk-width-auto" style="padding:0">
                  <mnm-date :iso="aRow.LastDate" ymd="md"/></div>
               <div class="uk-width-expand">{{aRow.Id}}</div>
               <!--todo more fields-->
            </div></template>
         <template v-else>
            <div v-for="aRow in tl" :key="aRow.Id"
                 @click="mnm.NavigateThread(aRow.Id)"
                 uk-grid class="uk-grid-small thread-row"
                 :style="{'background-color': aRow.Id === cs.Thread ? 'wheat' : null}"><!--todo class thread-row-thread-->
               <div class="uk-width-auto" style="padding:0">
                  <mnm-date :iso="aRow.LastDate" ymd="md"/></div>
               <div class="uk-width-1-6 overxhide">{{aRow.LastAuthor}}</div>
               <div class="uk-width-expand overxhide"
                    :title="aRow.Id">{{aRow.Subject}}</div>
               <div class="uk-width-auto">
                  <mnm-date :iso="aRow.OrigDate"/></div>
               <div class="uk-width-1-6 overxhide">{{aRow.OrigAuthor}}</div>
            </div></template>
         <div style="margin-top:1em">
            <div onclick="this.nextSibling.style.display = (this.nextSibling.style.display === 'none' ? 'block' : 'none')"
                 style="float:right; margin-right:1em; font-size:75%;">
               <span class="uk-link">+/- mo</span></div
           ><div style="display:none">{{JSON.stringify(mo)}}</div><br>
            <div onclick="this.nextSibling.style.display = (this.nextSibling.style.display === 'none' ? 'block' : 'none')"
                 style="float:right; margin-right:1em; font-size:75%;">
               <span class="uk-link">+/- log</span></div
           ><div style="display:none" id="log"></div>
         </div>
      </div>
      <div v-show="ohiFrom"
           class="uk-card uk-card-secondary uk-text-small uk-border-rounded"
           style="padding:8px; position:absolute; bottom:10px; right:10px">
         <div v-if="!of"
              class="uk-text-danger">offline</div>
         <template v-else>
            <div v-show="of.length === 0"
                 class="uk-text-warning">no o/</div>
            <ul class="uk-list uk-text-success" style="margin:0">
               <li v-for="aUser in of" :key="aUser.Uid">
                  {{aUser.Alias}}</li>
            </ul>
         </template>
      </div>
   </div>
</div>

<div class="uk-width-expand service-panel">
   <div class="uk-clearfix uk-light">
      <span uk-icon="plus-circle" class="dropdown-icon"></span>
      <mnm-svcadd/>
      <div style="float:right; margin:0 1em 1em 0">
         <!--todo span uk-icon="cog" class="dropdown-icon">&nbsp;</span>
         <div uk-dropdown="mode:click; offset:2; pos:bottom-right" class="uk-width-1-5">
            <div class="uk-text-right uk-text-small">SETTINGS</div></div -->
         <span uk-icon="info" class="dropdown-icon">&nbsp;</span>
         <div uk-dropdown="mode:click; offset:2; pos:bottom-right"
              class="uk-width-3-5" style="height:75vh; padding:0.8em">
            <iframe src="/w/docs.html" style="width:100%; height:100%"></iframe></div>
      </div>
   </div>
   <div uk-height-viewport="offset-top:true" class="firefox-minheight-fix uk-overflow-auto uk-light">
      <ul class="uk-list uk-list-divider">
         <li v-for="aSvc in v" :key="aSvc">
            <template v-if="aSvc === '[{.Title}]'">
               <span style="visibility:hidden">1</span
              ><span uk-icon="settings" class="dropdown-icon">&nbsp;</span>
               <mnm-svccfg/>
               {{aSvc}}
            </template>
            <template v-else>
               <span uk-icon="bell" :id="'n_'+aSvc" class="dropdown-icon">0{{aSvc.todo}} </span>
               <a :href="'/'+encodeURIComponent(aSvc)" :target="'mnm_'+aSvc">{{aSvc}}</a>
            </template>
         </li></ul>
   </div>
   <!--todo create notice menus dynamically-->
   <mnm-notice v-for="aSvc in v" :key="aSvc"
               :svc="aSvc" offset="-4" pos="left-top" :toggle="'#n_'+aSvc"
               @beforeshow.native="mnm.NoticeOpen(aSvc)"/>
</div>

</div>
</script>

<script type="text/x-template" id="mnm-date">
   <span :title="title">{{text}}</span>
</script><script>
   Vue.component('mnm-date', {
      template: '#mnm-date',
      props: {iso:String, ymd:String, hms:String},
      computed: {
         local: function() { return luxon.DateTime.fromISO(this.iso) },
         text: function() {
            var aDate = this.local.toString();
            var aD = aDate.substring(this.ymd === 'md' ? 5 : 0, 10);
            if (aD.charAt(0) === '0')
               aD = '\u2007' + aD.substr(1);
            if (!this.hms)
               return aD;
            return aD +' '+ aDate.substring(11, this.hms === 'hm' ? 16 : 19);
         },
         title: function() {
            return this.local.toLocaleString(luxon.DateTime.DATETIME_FULL_WITH_SECONDS);
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-subject">
   <div uk-dropdown="mode:click; offset:2" style="padding:0 0.5em 0.5em">
      <template v-for="aSubject in list">
         <a onclick="mnm.NavigateLink(this.href); return false"
            :href="'#'+ mnm._data.cs.Thread +'&'+ aSubject.msgId">
            {{ aSubject.name }}</a> <br>
      </template></div>
</script><script>
   Vue.component('mnm-subject', {
      template: '#mnm-subject',
      props: {list:Array},
      computed: { mnm: function() { return mnm } },
   });
</script>

<script type="text/x-template" id="mnm-cc">
   <div uk-dropdown="mode:click; offset:2" class="uk-width-1-3 dropdown-scroll">
      <button @click="mnm.ForwardSend(tid, mnm._data.cl[0][0].Qid)"
              :style="{visibility: ccSet ? 'hidden' : 'visible'}"
              :disabled="ccSet || !mnm._data.cl[ccSet].length"
              title="Forward thread to new recipients"
              style="float:left; margin:0 0.5em 1em 0"
              class="btn-icon"><span uk-icon="forward"></span></button>
      <input v-model="note"
             placeholder="Note" maxlength="1024" type="text"
             style="width:calc(100% - 4em)">
      <div style="position:relative; padding:1px;">
         <mnm-adrsbkinput @keyup.enter.native="addUser"
                          :type="3"
                          placeholder="To"
                          style="width:calc(50% - 2em)"/>
         <div style="height:100%; position:absolute; right:2em; top:0; width:calc(50% - 2.5em)">
            <mnm-draftmenu ref="menu" :list="menu" @drop="dropUser"/></div>
      </div>
      <ul uk-tab>
         <li v-for="aKey in ['Who','By','Date']"
             :class="{'uk-active': aKey === mnm._data.sort.cl}">
            <a @click.prevent="listSort(aKey)" href="#">{{aKey}}</a>
         </li></ul>
      <ul class="uk-list uk-list-divider dropdown-scroll-list">
         <li v-for="aUser in mnm._data.cl[1]" :key="aUser.WhoUid">
            <div style="float:left; width:40%">
               <span :title="aUser.Note">{{aUser.Who}}</span>
               <span v-show="aUser.Queued"
                     title="Awaiting link to server"
                     uk-icon="bolt"></span>
            </div>
            {{aUser.By}}
            <div style="float:right"><mnm-date :iso="ccSet ? now().toISO() : aUser.Date"/></div>
         </li></ul>
   </div>
</script><script>
   Vue.component('mnm-cc', {
      template: '#mnm-cc',
      props: {tid:String},
      data: function() { return {note:'', lastMenu:[]} },
      watch: {
         tid: function() { this.note = this.ccSet ? 'initial recipient' : '' },
      },
      computed: {
         ccSet: function() { return this.tid.charAt(0) === '_' ? 1 : 0 },
         menu: function() {
            var aCc = mnm._data.cl[this.ccSet];
            function fChanged(c) {
               var aLen = 0;
               for (var a=0; a < aCc.length; ++a)
                  if (aCc[a].WhoUid !== aCc[a].ByUid) {
                     if (c.lastMenu.indexOf(aCc[a].Who) < 0)
                        return true;
                     ++aLen;
                  }
               return aLen !== c.lastMenu.length;
            }
            var aMenu = fChanged(this) ? [] : this.lastMenu;
            for (var a=0, aN=0; a < aCc.length; ++a)
               if (aCc[a].WhoUid !== aCc[a].ByUid)
                  aMenu[aN++] = aCc[a].Who;
            if ('menu' in this.$refs)
               this.$refs.menu.$forceUpdate();
            return this.lastMenu = aMenu;
         },
         mnm: function() { return mnm }
      },
      methods: {
         now: function() { return luxon.DateTime.local() },
         addUser: function(iEvt) {
            var aAlias = iEvt.target.value;
            if (!(aAlias && aAlias in mnm._adrsbkmenuId))
               return;
            iEvt.target.value = '';
            var aCc = mnm._data.cl[this.ccSet].slice();
            var aPrev = aCc.findIndex(function(c) { return c.Who === aAlias });
            var aEl = aPrev >= 0 ? aCc.splice(aPrev, 1)[0]
                                 : {Who:aAlias, WhoUid:mnm._adrsbkmenuId[aAlias]};
            aEl.Note = this.note;
            aCc.unshift(aEl);
            if (this.ccSet)
               mnm._data.draftRefs[this.tid].save(aCc, null);
            else
               mnm.ForwardSave(this.tid, aCc);
         },
         dropUser: function(iItem) {
            var aCc = mnm._data.cl[this.ccSet];
            for (var a=0; a <= iItem; ++a)
               if (aCc[a].WhoUid === aCc[a].ByUid)
                  ++iItem;
            aCc = aCc.slice(0, iItem).concat(aCc.slice(iItem+1));
            if (this.ccSet)
               mnm._data.draftRefs[this.tid].save(aCc, null);
            else
               mnm.ForwardSave(this.tid, aCc);
         },
         listSort: function(i) { return mnm._listSort('cl', i) },
      },
   });
</script>

<script type="text/x-template" id="mnm-attach">
   <div uk-dropdown="mode:click; offset:2" class="uk-width-1-3 dropdown-scroll">
      <ul uk-tab>
         <li v-for="aKey in ['Date','Name','Size']"
             :class="{'uk-active': aKey === mnm._data.sort.al}">
            <a @click.prevent="listSort(aKey)" href="#">{{aKey}}</a>
         </li></ul>
      <ul class="uk-list uk-list-divider dropdown-scroll-list">
         <li v-for="aFile in mnm._data.al" :key="aFile.File">
            <a v-if="aFile.MsgId.charAt(0) !== '_'"
               onclick="mnm.NavigateLink(this.href); return false"
               :href="'#'+ mnm._data.cs.Thread +'&'+ aFile.MsgId"><span uk-icon="mail"></span></a>
            <span v-else
                  uk-icon="mail" style="visibility:hidden"></span>
            <mnm-date :iso="aFile.Date" ymd="md" hms="hm"/>
            &nbsp;
            <button v-if="false"
                    :title="aFile.File.charAt(17) === 'u' ? 'Copy to attachable files'
                                                          : 'Copy to blank forms'"
                    class="btn-icon">
               <span :uk-icon="aFile.File.charAt(17) === 'u' ? 'push' : 'file-edit'"></span></button>
            &nbsp;
            <a @click.prevent="mnm._toClipboard(aFile.File)"
               title="Copy reference to clipboard"
               :href="'#@'+ aFile.File"><span uk-icon="link"></span></a>
            <a :href="'?ad=' + encodeURIComponent(aFile.File)">
               <span uk-icon="download"></span></a>
            <a :href="'?an=' + encodeURIComponent(aFile.File)" target="mnm_atc_[{.Title}]">
               {{aFile.Name}}</a>
            <div class="uk-float-right">{{aFile.Size}}</div>
         </li></ul>
   </div>
</script><script>
   Vue.component('mnm-attach', {
      template: '#mnm-attach',
      computed: { mnm: function() { return mnm } },
      methods: { listSort: function(i) { return mnm._listSort('al', i) } },
   });
</script>

<script type="text/x-template" id="mnm-draft">
   <div @keydown="keyAction('pv_'+msgid, $event)">
      <div style="position:relative; padding:1px;">
         <button @click="send"
                 title="Send draft"
                 class="btn-icon btn-alignt"><span uk-icon="forward"></span></button>
         <div style="height:100%; position:absolute; left:13em; right:42px; top:0;">
            <mnm-draftmenu @drop="atcDrop"
                           :list="mnm._data.mo[msgid].SubHead.Attach"
                           :getname="atcGetName" :getkey="atcGetKey"
                           :style="{float:'right'}"/>
         </div>
      </div>
      <div style="float:right; margin-top:-1.7em;">
         <span uk-icon="push"      :id="'t_'+msgid" class="dropdown-icon"></span
        ><span uk-icon="file-edit" :id="'f_'+msgid" class="dropdown-icon"></span>
         <span :id="'pv_'+msgid"></span>
         <div uk-dropdown="mode:click; pos:right-top"
              class="uk-width-2-5 message-edit"
              style="overflow:auto; max-height:75vh; padding: 0.5em 1em;
                     border-width: 1em 0; border-color:transparent; border-style:solid;"
              onwheel="return mnm._canScroll(this, event.deltaY)">
            <div v-show="!(mnm._data.toSave[msgid] || mnm._data.mo[msgid]).msg_data">
               <p><span uk-icon="comment"></span></p></div>
            <mnm-markdown v-show="(mnm._data.toSave[msgid] || mnm._data.mo[msgid]).msg_data"
                          @formfill="ffAdd"
                          @toggle="atcToggleFf"
                          :src=     "(mnm._data.toSave[msgid] || mnm._data.mo[msgid]).msg_data"
                          :formfill="(mnm._data.toSave[msgid] || mnm._data.mo[msgid]).form_fill"
                          :atchasff="atcHasFf" :msgid="msgid"/>
         </div>
      </div>
      <input @input="subjAdd"
             @click.stop="clickPreview('pv_'+msgid)"
             :value="(mnm._data.toSave[msgid] || mnm._data.mo[msgid].SubHead).Subject"
             :placeholder="'Subject'+ (mnm._data.ml.length > 1 ? '' : ' (req.)')" type="text"
             class="width100">
      <mnm-textresize @input.native="textAdd"
                      @click.native.stop="clickPreview('pv_'+msgid)"
                      :src="(mnm._data.toSave[msgid] || mnm._data.mo[msgid]).msg_data"
                      placeholder="Ctrl-J to Preview"
                      class="width100"/>
   </div>
</script><script>
   Vue.component('mnm-draft', {
      template: '#mnm-draft',
      props: {msgid:String},
      computed: { mnm: function() { return mnm } },
      created: function() { Vue.set(mnm._data.draftRefs, this.msgid, this) }, // $refs not reactive
      beforeDestroy: function() { Vue.delete(mnm._data.draftRefs, this.msgid) },
      methods: {
         keyAction: function(iId, iEvent) {
            if (iEvent.ctrlKey && iEvent.key === 'j')
               mnm._lastPreview = iId;
         },
         clickPreview: function(iId) {
            document.getElementById(iId).nextElementSibling.click();
         },
         getToSave: function(iNoTimer) {
            var aMo = mnm._data.mo[this.msgid];
            if (!(this.msgid in mnm._data.toSave))
               Vue.set(mnm._data.toSave, this.msgid,
                       {timer:null, form_fill:aMo.form_fill,       ffUpdt:{},
                                    msg_data: aMo.msg_data,        mdUpdt:false,
                                    Subject:  aMo.SubHead.Subject, suUpdt:false});
            var aToSave = mnm._data.toSave[this.msgid];
            if (!iNoTimer && !aToSave.timer)
               aToSave.timer = setTimeout(fDing, 2000, this);
            return aToSave;
            function fDing(that) {
               aToSave.timer = null;
               that.save(null, null, aToSave, aMo);
            }
         },
         save: function(iCc, iAttach, iToSave, iMo) {
            if (!iToSave) iToSave = this.getToSave(true);
            if (!iMo)     iMo = mnm._data.mo[this.msgid];
            if (iToSave.timer) {
               clearTimeout(iToSave.timer);
               iToSave.timer = null;
            }
            mnm.ThreadSave({
               Id:       this.msgid,
               Alias:               iMo.SubHead.Alias,
               Attach:   iAttach || iMo.SubHead.Attach,
               Cc:       iCc     || mnm._data.cl[1],
               FormFill: iToSave.form_fill,
               Data:     iToSave.msg_data,
               Subject:  iToSave.Subject,
            });
            iToSave.suUpdt = iToSave.mdUpdt = false;
            for (var a in iToSave.ffUpdt)
               if (iToSave.ffUpdt[a] === 'save')
                  iToSave.ffUpdt[a] = null;
         },
         send: function() {
            var aToSave = mnm._data.toSave[this.msgid];
            if (aToSave && aToSave.timer)
               this.save(null, null, aToSave, null);
            mnm.ThreadSend(this.msgid);
         },
         atcGetName: function(iEl) { return iEl.Name },
         atcGetKey:  function(iEl) { return iEl.FfKey || iEl.Name },
         atcAdd: function(iPath) {
            var aAtc = mnm._data.mo[this.msgid].SubHead.Attach;
            aAtc = aAtc ? aAtc.slice() : [];
            var aPrefix = /^upload/.test(iPath) ? 'u:' : /^form_fill/.test(iPath) ? 'r:' : 'f:';
            var aStoredName = iPath.replace(/^[^/]*\//, aPrefix);
            var aPrev = aAtc.findIndex(function(c) { return c.Name === aStoredName });
            if (aPrev >= 0)
               aAtc.splice(aPrev, 1);
            aAtc.unshift({Name:iPath});
            this.save(null, aAtc);
         },
         atcDrop: function(iN) {
            var aAtc = mnm._data.mo[this.msgid].SubHead.Attach;
            this.save(null, aAtc.slice(0, iN).concat(aAtc.slice(iN+1)));
         },
         atcHasFf: function(iId, iFfKey) { // called by formview
            var aAtc = mnm._data.mo[iId].SubHead.Attach;
            return !! (aAtc && aAtc.find(function(c) { return c.FfKey === iFfKey }));
         },
         atcToggleFf: function(iFfKey, iPath) {
            var aAtc = mnm._data.mo[this.msgid].SubHead.Attach || [];
            var aN = aAtc.findIndex(function(c) { return c.FfKey === iFfKey });
            this.ffAdd(iFfKey, null, aN < 0);
            this.save(null, aN < 0 ? [ {Name:iPath, FfKey:iFfKey} ].concat(aAtc)
                                   : aAtc.slice(0, aN).concat(aAtc.slice(aN+1)));
         },
         ffAdd: function(iFfKey, iText, iToggle) {
            var aToSave = this.getToSave(!iText);
            if (!iText) {
               if (aToSave.form_fill && iFfKey in aToSave.form_fill) {
                  aToSave.ffUpdt[iFfKey] = iToggle ? 'save'
                                                   : aToSave.form_fill[iFfKey] !== '{}' ? 'keep' : null;
                  return;
               }
               iText = '{}';
            } else {
               iToggle = this.atcHasFf(this.msgid, iFfKey);
            }
            if (!aToSave.form_fill)
               aToSave.form_fill = {};
            Vue.set(aToSave.form_fill, iFfKey, iText);
            aToSave.ffUpdt[iFfKey] = iToggle ? 'save'
                                             : iText !== '{}' ? 'keep' : null;
         },
         textAdd: function(iEvent) {
            var aToSave = this.getToSave(false);
            aToSave.msg_data = iEvent.target.value;
            aToSave.mdUpdt = true;
         },
         subjAdd: function(iEvent) {
            var aToSave = this.getToSave(false);
            aToSave.Subject = iEvent.target.value;
            aToSave.suUpdt = true;
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-adrsbkinput">
   <div @click="menu.selectId($el.firstChild, $event.target.id)"
        class="adrsbkinput">
      <input @focus="menu.placeEl($el, type, $event.target.value)"
             @blur ="menu.hideEl()"
             @input="menu.search(type, $event.target.value)"
             @keydown.down ="menu.selectItem($event.target,  1)"
             @keydown.up   ="menu.selectItem($event.target, -1)"
             @keydown.esc  ="menu.selectNone($event.target)"
             @keyup.enter  ="menu.clear()"
             v-bind="$attrs" type="text"
             class="width100">
      <!--menu appended here-->
   </div>
</script><script>
   Vue.component('mnm-adrsbkinput', {
      template: '#mnm-adrsbkinput',
      props: {type:Number},
      inheritAttrs: false,
      computed: {
         menu: function() { return this.$root.$refs.adrsbkmenu },
      },
   });
</script>

<script type="text/x-template" id="mnm-adrsbkmenu">
   <div style="position:relative; display:none">
      <div v-show="query"
           @mousedown.prevent=""
           style="position:absolute; min-width:100%" class="adrsbkmenu">
         <div v-show="!list.length"
              class="adrsbkmenu-none">[ no result ]</div>
         <div v-for="(aName, aI) in list"
              :id="'am_'+ aI"
              :title="'Use \u2191\u2193 keys to select'"
              :class="{'adrsbkmenu-select': aI === select}">{{aName}}</div>
      </div></div>
</script><script>
   Vue.component('mnm-adrsbkmenu', {
      template: '#mnm-adrsbkmenu',
      data: function() { return { query:'', list:[], select:-1 } },
      methods: {
         clear: function() {
            this.query = '';
            this.select = -1;
            this.list = [];
         },
         placeEl: function(iParent, iType, iQuery) {
            if (iParent.lastChild !== this.$el) {
               this.clear();
               this.query = iQuery;
               if (iQuery)
                  mnm.AdrsbkSearch(iType, iQuery);
               iParent.appendChild(this.$el);
            }
            this.$nextTick(function() {
               this.$el.style.display = 'block';
            });
         },
         hideEl: function() {
            this.$el.style.display = 'none';
         },
         search: function(iType, iQuery) {
            this.select = -1;
            this.query = iQuery;
            if (iQuery)
               mnm.AdrsbkSearch(iType, iQuery);
            else
               this.list = [];
         },
         results: function(iList) {
            this.list = iList;
         },
         selectNone: function(iInput) {
            this.select = -1;
            iInput.value = this.query;
         },
         selectId: function(iInput, iId) {
            if (!iId)
               return;
            this.select = parseInt(iId.substring(3), 10);
            iInput.value = this.list[this.select];
         },
         selectItem: function(iInput, iDirection) {
            if (this.select === -1 && iDirection === -1)
               this.select = this.list.length;
            this.select += iDirection;
            if (this.select === this.list.length || this.select === -1)
               this.selectNone(iInput);
            else
               iInput.value = this.list[this.select];
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-draftmenu">
   <div v-show="list && list.length > 0"
        @click="draftMenu"
        class="draftmenu-hiddn draftmenu">
      <span v-for="(aEl, aI) in list" :key="getkey ? getkey(aEl) : aEl">
         {{getname ? getname(aEl) : aEl}}
         <div v-if="aI === 0 && list.length > 1"
              class="draftmenu-v">&#x25BD;</div>
         <div @click="$emit('drop', aI)"
              class="draftmenu-x">&times;</div>
         <br>
      </span>
   </div>
</script><script>
   Vue.component('mnm-draftmenu', {
      template: '#mnm-draftmenu',
      props: {list:Array, getname:Function, getkey:Function},
      watch: {
         list: function() {
            // show menu if changed by any client
            this.draftMenu(null);
         },
      },
      methods: {
         draftMenu: function(iE) {
            if (this.$el.className === 'draftmenu-hiddn draftmenu') {
               this.$el.className = 'draftmenu-shown draftmenu';
               var aMenu = this.$el;
               document.addEventListener('click',
                  function() { aMenu.className = 'draftmenu-hiddn draftmenu' }, {once:true});
            }
            if (iE)
               iE.stopPropagation();
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-textresize">
   <textarea @input="resize" :value="src" class="text-resize"></textarea>
</script><script>
   Vue.component('mnm-textresize', {
      template: '#mnm-textresize',
      props: {src:String},
      mounted: function() { this.resize() },
      updated: function() { this.resize() },
      methods: {
         resize: function() {
            this.$el.style.height = 'auto';
            this.$el.style.height = this.$el.scrollHeight+4 + 'px';
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-markdown">
   <div class="message" v-html="mdi.render(src, env)"></div>
</script><script>
   Vue.component('mnm-markdown', {
      template: '#mnm-markdown',
      props: {src:String, msgid:String, formfill:Object, formreply:[Object,String], atchasff:Function},
      computed: {
         mdi: function() { return mnm._mdi },
      },
      watch: {
         formfill: { deep: true, handler:
            function(iMap) {
               for (var a in this.env.fillMap)
                  if (!iMap || !(a in iMap))
                     Vue.delete(this.env.fillMap, a);
               for (var a in iMap)
                  Vue.set(this.env.fillMap, a, iMap[a]);
            }
         },
      },
      created: function() {
         this.formDefBad = {fields:[ {type:"label",label:"file not found or invalid"} ]};
         this.env = { thisVal:this.formreply ? this.msgid : this.msgid.substr(-12),
                      fillMap:{}, parent:this, formview:null };
         if (this.formfill)
            for (var a in this.formfill)
               Vue.set(this.env.fillMap, a, this.formfill[a]);
      },
      mounted:       function() { if (this.env.formview) this.env.formview.remount() },
      updated:       function() { if (this.env.formview) this.env.formview.remount() },
      beforeDestroy: function() { if (this.env.formview) this.env.formview.destroy() },
   });
</script>

<script type="text/x-template" id="mnm-formview">
   <div :id="'fv_'+ parent.msgid +'_'+ file">
      <div class="uk-clearfix">
         <label v-if="!parent.formreply">
            <input type="checkbox" @click="onFillAttach" :checked="atcHasFf"
                   :disabled="formDef === parent.formDefBad">
            attach fill</label>
         <button @click="startReply"
                 :disabled="!parent.formreply || parent.formreply === 'Q'"
                 title="New reply draft with form below"
                 class="btn-icon btn-floatr"><span uk-icon="commenting"></span></button>
      </div>
      <plugin-vfg @model-updated="onInput"
                  :schema="formDef" :model="formState"
                  :options="{fieldIdPrefix: 'fv_'+ parent.msgid +'_'+ file +'_'}"/>
   </div>
</script><script>
   Vue.component('mnm-formview', {
      template: '#mnm-formview',
      props: {file:String, fillMap:Object, parent:Object},
      computed: {
         formState: function() {
            return JSON.parse(this.fillMap[this.file] || '{}');
         },
         formDef: function() {
            if (!(this.file in mnm._data.ao))
               return {};
            try {
               var aDef = JSON.parse(mnm._data.ao[this.file]);
            } catch(e) {
               return this.parent.formDefBad;
            }
            if (this.parent.formreply) { //todo update if VFG adds .disabled at top level
               if ('fields' in aDef)
                  aDef = {groups:[aDef]};
               for (var a in aDef.groups)
                  for (var a1 in aDef.groups[a].fields)
                     aDef.groups[a].fields[a1].disabled = true;
            }
            return aDef;
         },
         atcHasFf: function() {
            return this.parent.atchasff(this.parent.msgid, this.file);
         },
      },
      methods: {
         onInput: function(iVal, iField) {
            if (iVal !== true && !_.isFinite(iVal) && _.isEmpty(iVal)) //todo replace
               delete this.formState[iField];
            this.parent.$emit('formfill', this.file, JSON.stringify(this.formState));
         },
         onFillAttach: function(iEvent) {
            this.parent.$emit('toggle', this.file, this.fill_name());
         },
         startReply: function() {
            var aReply = JSON.parse(JSON.stringify(this.parent.formreply));
            aReply.data += '![?]('+ this.file +')';
            aReply.attach = [ {Name:this.fill_name(), FfKey:this.file, Ffn:''} ];
            aReply.formFill = {};
            aReply.formFill[this.file] = '{}';
            mnm.ThreadReply(aReply);
         },
         fill_name: function() {
            return 'form_fill/'+ this.file.substr(this.file.indexOf('_')+3);
         },
      },
      components: { 'plugin-vfg': VueFormGenerator.component },
   });

   mnm._FormViews = function(iEnv) {
      this.env = iEnv;
      this.comp = {};
   };
   mnm._FormViews.prototype.make = function(iKey) {
      if (iKey in this.comp) {
         if (this.comp[iKey][0].formDef === this.env.parent.formDefBad)
            mnm.AttachOpen(iKey);
      } else {
         mnm.AttachOpen(iKey);
         this.comp[iKey] = [ new (Vue.component('mnm-formview'))({
            propsData: { file:iKey, fillMap:this.env.fillMap, parent:this.env.parent },
         }), null ];
      }
      return 'fv_'+ this.env.parent.msgid +'_'+ iKey;
   };
   mnm._FormViews.prototype.remount = function() {
      for (var a in this.comp) {
         var aEl = document.getElementById('fv_'+ this.env.parent.msgid +'_'+ a);
         if (!aEl) {
            this.comp[a][0].$destroy();
            delete this.comp[a];
            continue;
         }
         if (aEl.tagName === 'DIV')
            continue;
         if (this.comp[a][1])
            aEl.parentNode.replaceChild(this.comp[a][1], aEl);
         else
            this.comp[a][1] = this.comp[a][0].$mount(aEl).$el;
      }
   };
   mnm._FormViews.prototype.destroy = function() {
      for (var a in this.comp)
         this.comp[a][0].$destroy();
   };
</script>

<script type="text/x-template" id="mnm-files">
   <div uk-dropdown="mode:click; offset:2" :toggle="toggle" class="uk-width-1-3 dropdown-scroll">
      <form :action="'/t/+' + encodeURIComponent(upname)"
            method="POST" enctype="multipart/form-data"
            onsubmit="mnm.Upload(this); this.reset(); return false;"
            class="dropdown-scroll-item">
         <div class="uk-float-right uk-text-small">ATTACHABLE FILES</div>
         <input @input="vis = !!(upname = $event.target.value.substr(12))" type="file"
                name="filename" required>
         <div :style="{visibility: vis ? 'visible' : 'hidden'}" style="margin-top:0.5em">
            <input v-model="upname"
                   placeholder="Alt Name" type="text"
                   style="width:60%">
            <button @click="vis = false" type="submit"
                    :disabled="!upname"
                    title="Copy to attachable files"
                    class="btn-icon"><span uk-icon="push"></span></button>
            <button @click="vis = false" type="reset"
                    class="btn-iconx">&times;</button>
         </div>
      </form>
      <ul uk-tab style="margin-top:0">
         <li v-for="aKey in ['Date','Name']"
             :class="{'uk-active': aKey === mnm._data.sort.t}">
            <a @click.prevent="listSort(aKey)" href="#">{{aKey}}</a>
         </li></ul>
      <ul class="uk-list uk-list-divider dropdown-scroll-list">
         <li v-for="aFile in mnm._data.t" :key="aFile.Name">
            <mnm-date :iso="aFile.Date" hms="hm"/>
            <button v-if="toggle"
                    @click="$emit('attach', 'upload/'+aFile.Name)"
                    title="Attach file"
                    class="btn-icon"><span uk-icon="location"></span></button>
            <a :href="'/t/!' + encodeURIComponent(aFile.Name)">
               <span uk-icon="download">&nbsp;</span></a>
            <a :href="'/t/' + encodeURIComponent(aFile.Name)" target="mnm_atc_[{.Title}]">
               {{aFile.Name}}</a>
            <div class="uk-float-right">
               {{aFile.Size}}
               <form v-if="!toggle"
                     :action="'/t/-' + encodeURIComponent(aFile.Name)" method="POST"
                     onsubmit="mnm.Upload(this); return false;"
                     style="display:inline!important">
                  <button title="Erase file"
                          class="btn-iconred"><span uk-icon="trash"></span></button>
               </form>
            </div>
         </li></ul>
   </div>
</script><script>
   Vue.component('mnm-files', {
      template: '#mnm-files',
      props: {toggle:String},
      data: function() { return {upname:'', vis:false} },
      computed: { mnm: function() { return mnm } },
      methods: { listSort: function(i) { return mnm._listSort('t', i) } },
   });
</script>

<script type="text/x-template" id="mnm-forms">
   <div uk-dropdown="mode:click; offset:2" :toggle="toggle" class="uk-width-1-3 dropdown-scroll"
        @hidden="revClose" @click="revClose">
      <form :action="'/f/+' + encodeURIComponent(upname)"
            method="POST" enctype="multipart/form-data"
            onsubmit="mnm.Upload(this); this.reset(); return false;"
            class="dropdown-scroll-item">
         <div class="uk-float-right uk-text-small">BLANK FORMS</div>
         <input type="hidden" name="filename"
                value='{"fields":[ {"label":"Untitled","model":"s","type":"input","inputType":"text"} ]}'>
         <input v-model="upname"
                placeholder="New Type" type="text"
                style="width:60%">
         <button @click="upname = ''"
                 :disabled="!validName(upname.split('.'))"
                 title="New form"
                 class="btn-icon"><span uk-icon="pencil"></span></button>
      </form>
      <ul uk-tab style="margin-top:0">
         <li v-for="aKey in ['Date','Name']"
             :class="{'uk-active': aKey === mnm._data.sort.f}">
            <a @click.prevent="listSort(aKey)" href="#">{{aKey}}</a>
         </li></ul>
      <div style="position:relative"><!--context for rev card-->
         <ul class="uk-list uk-list-divider dropdown-scroll-list">
            <template v-for="aSet in mnm._data.f">
            <li v-for="aFile in aSet.Revs" :key="aSet.Name+'.'+aFile.Id">
               <mnm-date :iso="aFile.Date" hms="hm"/>
               <button v-if="toggle"
                       @click="$emit('attach', 'form/'+aSet.Name+'.'+aFile.Id)"
                       title="Attach form"
                       class="btn-icon"><span uk-icon="location"></span></button>
               <a @click.stop.prevent="revOpen(aSet.Name,aFile.Id,$event.currentTarget)"
                  :ref="aSet.Name+'.'+aFile.Id" href="#">
                  <span uk-icon="triangle-left">&nbsp;</span>{{aSet.Name}}.{{aFile.Id}}</a>
               <form v-if="!toggle"
                     :action="'/f/-' + encodeURIComponent(aSet.Name+'.'+aFile.Id)" method="POST"
                     onsubmit="mnm.Upload(this); return false;"
                     style="float:right">
                  <button @click="revDelete(aSet.Name,aFile.Id)"
                          title="Erase form"
                          class="btn-iconred"><span uk-icon="trash"></span></button>
               </form>
            </li></template></ul>
         <div v-show="setName"
              class="uk-card uk-card-default uk-card-small uk-card-body uk-width-1-1"
              style="position:absolute" :style="{top:editTop, right:editRight}"
              @click.stop>
            <div class="uk-text-right uk-text-small">
               {{(setName+'.'+fileId).toUpperCase()}}</div>
            <div v-show="!mnm._data.fo"
                 class="uk-text-center" style="padding:0.5em">
               <span uk-icon="future"></span></div>
            <div v-show="mnm._data.fo"
                 class="pane-clip" style="margin-top:-1.5em">
               <span v-if="!toggle"
                     @click="showCode"
                     class="uk-link"><tt>{...}</tt></span>
               &nbsp;
               <div style="font-size:smaller; text-align:right">&nbsp;{{parseError}}</div>
               <div class="pane-slider" :class="{'pane-slider-rhs':codeShow}">
                  <div class="pane-scroller" style="min-height:1px">
                     <plugin-vfg :schema="formDef" :model="{}" :options="{}"/></div>
                  <div class="pane-scroller">
                     <mnm-textresize @input.native="editCode"
                                     :src="mnm._data.fo"
                                     ref="code"
                                     class="width100"/></div>
               </div>
            </div>
            <form action="/f/?" method="POST" enctype="multipart/form-data">
               <input ref="save" name="filename" value="" type="hidden"></form>
            <form :action="'/f/*' + encodeURIComponent(setName+'.'+fileId) +
                              '+' + encodeURIComponent(dupname)" method="POST"
                  onsubmit="mnm.Upload(this); return false;">
               <input v-model="dupname"
                      placeholder="New Revision" type="text"
                      style="width:60%">
               <button @click="dupShow = dupname"
                       :disabled="!validName([].concat(setName,dupname.split('.')))"
                       title="Duplicate form"
                       class="btn-icon"><span uk-icon="copy"></span></button>
            </form>
         </div>
      </div>
   </div>
</script><script>
   Vue.component('mnm-forms', {
      template: '#mnm-forms',
      props: {toggle:String},
      data: function() {
         return {upname:'', dupname:'', setName:'', fileId:'', codeShow:false, dupShow:'',
                 editTop:'', editRight:'', formDef:null, parseError:'', toSave:{}};
      },
      computed: {
         mnm: function() { return mnm },
      },
      methods: {
         validName: function(iPair) {
            if (iPair[0] === '' || iPair.length > 2)
               return false;
            if (iPair.length === 1)
               iPair.push('original');
            else if (iPair[1] === '')
               iPair[1] = 'original';
            var aLst = mnm._data.f;
            for (var aF=0; aF < aLst.length; ++aF) {
               if (aLst[aF].Name === iPair[0]) {
                  for (var aR=0; aR < aLst[aF].Revs.length; ++aR)
                     if (aLst[aF].Revs[aR].Id === iPair[1])
                        return false;
                  return true;
               }
            }
            return true;
         },
         listSort: function(i) {
            mnm._data.sort.f = i;
            mnm._data.f.sort(function(cA, cB) {
               if (i === 'Name')
                  return cA.Name < cB.Name ? -1 : 1;
               if (i === 'Date')
                  return cA.Revs[0].Date > cB.Revs[0].Date ? -1 : 1;
               return 0;
            });
         },
         revOpen: function(iSet, iRev, iEl) {
            var aKey = iSet+'.'+iRev;
            if (aKey in this.toSave) {
               mnm._data.fo = this.toSave[aKey].data;
            } else {
               mnm._data.fo = '';
               mnm.FormOpen(aKey);
            }
            this.setName = iSet;
            this.fileId = iRev;
            this.editTop = iEl.offsetTop + 'px';
            var aParentWidth = iEl.parentNode.parentNode.parentNode.offsetWidth;
            this.editRight = (aParentWidth - iEl.offsetLeft) +'px';
            this.codeShow = false;
            this.dupname = '';
         },
         revClose: function() {
            this.setName = this.fileId = '';
         },
         revDelete: function(iSet, iRev) {
            var aKey = iSet+'.'+iRev;
            if (aKey in this.toSave) {
               clearTimeout(this.toSave[aKey].timer);
               delete this.toSave[aKey];
            }
         },
         showCode: function() {
            this.codeShow = !this.codeShow;
            if (this.codeShow)
               this.$refs.code.$el.focus();
         },
         editCode: function(iEvt) {
            mnm._data.fo = iEvt.target.value || '{}';
            this.$nextTick(function() {
               if (this.parseError !== '')
                  return;
               var aKey = this.setName+'.'+this.fileId;
               if (!(aKey in this.toSave))
                  this.toSave[aKey] = {timer:setTimeout(fDing, 2000, this), data:''};
               this.toSave[aKey].data = mnm._data.fo;
               function fDing(that) {
                  that.$refs.save.value = that.toSave[aKey].data;
                  that.$refs.save.form.action = '/f/+' + encodeURIComponent(aKey);
                  mnm.Upload(that.$refs.save.form); // assume save.value can be changed after this
                  delete that.toSave[aKey];
               }
            });
         },
      },
      watch: {
         '$root.f': function() {
            if (!this.dupShow)
               return;
            this.$nextTick(function() {
               var aEl = this.$refs[this.setName+'.'+this.dupShow];
               if (aEl) {
                  this.revOpen(this.setName, this.dupShow, aEl[0]);
                  this.dupShow = '';
               }
            });
         },
         '$root.fo': function() { //todo fires 3x, per alert(); fix?
            try {
               this.formDef = JSON.parse(mnm._data.fo);
               this.parseError = '';
            } catch(aErr) {
               this.formDef = {fields:[ {type:"label", label:"code incorrect"} ]};
               this.parseError = aErr.message.slice(12, -17);
            }
         },
      },
      components: { 'plugin-vfg': VueFormGenerator.component },
   });
</script>

<script type="text/x-template" id="mnm-notice">
   <div uk-dropdown="mode:click" :toggle="toggle" class="uk-width-1-4 dropdown-scroll">
      <button v-if="!svc"
              @click="mnm.NoticeSeen(nl[0].MsgId)"
              :disabled="!nl.length || nl[0].Seen > 0"
              title="Mark all as seen"
              style="margin-right:1em"
              class="btn-icon btn-floatr"><span uk-icon="check"></span></button>
      <div style="min-height:2em; font-style:oblique;">
         <span v-for="aType in [['i', 'invites']]"
               @click="$data[aType[0]] = !$data[aType[0]]"
               class="uk-link" style="margin-right:0.5em">
            <span :style="{visibility: $data[aType[0]] ? 'visible' : 'hidden'}">&bull; </span>
            {{ aType[1] }}
         </span>
      </div>
      <ul class="uk-list dropdown-scroll-list notice">
         <li v-if="!nl.length"
             style="text-align:center">No notices yet</li>
         <li v-for="aNote in nl" :key="aNote.MsgId"
             v-show="$data[aNote.Type]"
             @click="$set(aNote, 'open', !aNote.open)"
             :class="{'notice-seen':aNote.Seen, 'notice-hasblurb':aNote.Blurb}">
            <div style="float:left; font-style:oblique">{{aNote.Type}}</div>
            <div style="margin-left:1em">
               <div style="float:right"><mnm-date :iso="aNote.Date" ymd="md" hms="hm"/></div>
               {{aNote.Alias}}
               <template v-if="aNote.Gid">
                   - {{aNote.Gid}}</template>
               <span v-show="aNote.Blurb && !aNote.open">. . .</span>
               <div v-show="aNote.open">{{aNote.Blurb}}</div>
            </div>
         </li></ul>
   </div>
</script><script>
   Vue.component('mnm-notice', {
      template: '#mnm-notice',
      props: {svc:String, toggle:String},
      data: function() { return { i:true } },
      computed: {
         nl: function() { return this.svc ? mnm._data.nlo : mnm._data.nl },
         mnm: function() { return mnm },
      },
   });
</script>

<script type="text/x-template" id="mnm-adrsbk">
   <div uk-dropdown="mode:click; offset:2; pos:bottom-right" class="uk-width-2-5 dropdown-scroll">
      <ul uk-tab class="uk-child-width-expand" style="margin-top:0; margin-right:20px"
          @click.prevent>
         <li><a href="#">{{mnm._data.pf.length || null}} invites </a></li>
         <li class="uk-active"
            ><a href="#">{{mnm._data.ps.length || null}} drafts  </a></li>
         <li><a href="#">{{mnm._data.pt.length || null}} sent    </a></li>
         <li><a href="#">{{mnm._data.gl.length || null}} groups  </a></li>
         <li><a href="#">{{mnm._data.ot.length || null}} ohi to  </a></li>
      </ul>
      <ul class="uk-switcher dropdown-scroll-list">
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th> <th>From</th> <th>Group</th> <th>Msg</th> <th>Response</th></tr>
               <tr v-for="aPing in mnm._data.pf">
                  <td><mnm-date :iso="aPing.Date" ymd="md"/></td>
                  <td>{{aPing.Alias}}</td>
                  <td>{{aPing.Gid}}
                     <span v-if="aPing.Queued"
                           title="Awaiting link to server"
                           uk-icon="bolt"></span>
                     <button v-else-if="aPing.Qid"
                             @click="mnm.InviteAccept(aPing.Qid)"
                             title="Accept group invite"
                             class="btn-icon"><span uk-icon="forward"></span></button>
                  </td>
                  <td>{{aPing.Text}}</td>
                  <td><mnm-pingresponse :ping="aPing"/></td>
               </tr></table></li>
         <li>
            <form onsubmit="return false"
                  style="width:70%; margin: 0 auto; display:table">
               <input v-model="draft.to"
                      placeholder="To ([{.aliasMin}]+ characters)" type="text"
                      style="width:calc(50% - 1.5em)">
               <div style="width:calc(50% - 1.5em); display:inline-block; vertical-align:top">
                  <input v-model="draft.gid"
                         placeholder="Group (opt. [{.aliasMin}]+)" type="text"
                         class="width100">
                  <br>
                  <mnm-adrsbkinput @keyup.enter.native="setGid($event.target)"
                                   @keydown.enter.native.prevent=""
                                   :type="2"
                                   placeholder="Search groups"
                                   class="width100"/>
               </div>
               <button @click="startPing()"
                       :disabled="!validDraft"
                       title="New draft invitation"
                       class="btn-icon"><span uk-icon="pencil"></span></button>
            </form>
            <table class="uk-table uk-table-small" style="margin:0">
               <tr><th>To / (Group)</th> <th></th> <th>Message</th> <th></th></tr>
               <tr v-for="a in mnm._data.ps" :key="rowId(a)">
                  <td>{{a.Alias}}<br>{{a.Gid && '('+a.Gid+')'}}</td>
                  <td><span v-if="a.Queued"
                            title="Awaiting link to server"
                            uk-icon="bolt"></span>
                      <button v-else
                              @click="mnm.PingSend(a.Qid)"
                              title="Send invitation"
                              class="btn-icon"><span uk-icon="forward"></span></button></td>
                  <td><textarea @input="timer(a, $event.target.value)"
                                :disabled="a.Queued"
                                cols="40" rows="3" maxlength="120"
                                >{{toSave[rowId(a)] || a.Text}}</textarea></td>
                  <td><button v-if="!a.Queued"
                              @click="mnm.PingDiscard({to:a.Alias, gid:a.Gid})"
                              title="Discard draft"
                              class="btn-iconred"><span uk-icon="trash"></span></button></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th> <th>To</th> <th>Group</th> <th>Msg</th> <th>Response</th></tr>
               <tr v-for="aPing in mnm._data.pt">
                  <td><mnm-date :iso="aPing.Date" ymd="md"/></td>
                  <td>{{aPing.Alias}}</td>
                  <td>{{aPing.Gid}}</td>
                  <td>{{aPing.Text}}</td>
                  <td><mnm-pingresponse :ping="aPing"/></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th> <th>Group</th></tr>
               <tr v-for="a in mnm._data.gl">
                  <td><mnm-date :iso="a.Date"/></td>
                  <td>{{a.Gid}}
                     <span v-if="a.Admin"
                           class="uk-badge">A</span></td>
               </tr></table></li>
         <li>
            <form onsubmit="this.reset(); return false;"
                  style="width:70%; margin: 0 auto; display:table">
               <mnm-adrsbkinput @keyup.enter.native="setOhiTo($event.target)"
                                @keydown.enter.native.prevent
                                :type="1"
                                placeholder="To" name="resets" autocomplete="off"
                                style="width:50%"/>
               <button onclick="mnm.OhiAdd(this.innerText.substring(3), this.value)"
                       disabled
                       title="Notify contact when you're online"
                       style="width:calc(50% - 0.5em)"
                       class="btn-icontxt">o/</button>
            </form>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th> <th>To</th> <th></th></tr>
               <tr v-for="aOhi in mnm._data.ot">
                  <td><mnm-date :iso="aOhi.Date"/></td>
                  <td>{{aOhi.Alias}}</td>
                  <td><button @click="mnm.OhiDrop(aOhi.Uid)"
                              title="Stop notifying contact"
                              class="btn-iconred"><span uk-icon="trash"></span></button></td>
               </tr></table></li>
      </ul>
   </div>
</script><script>
   Vue.component('mnm-adrsbk', {
      template: '#mnm-adrsbk',
      data: function() { return {draft:{to:'', gid:''}, toSave:{}} },
      computed: {
         mnm: function() { return mnm },
         validDraft: function() {
            if (this.draft.to.length < [{.aliasMin}] ||
                this.draft.gid && this.draft.gid.length < [{.aliasMin}])
               return false;
            for (var a=0; a < mnm._data.ps.length; ++a)
               if (mnm._data.ps[a].Alias ===  this.draft.to &&
                   mnm._data.ps[a].Gid   === (this.draft.gid || undefined))
                  return false;
            return true;
         },
      },
      methods: {
         rowId: function(iRec) { return iRec.Alias +'\0'+ (iRec.Gid || '') },
         setGid: function(iInput) {
            if (iInput.value) {
               this.draft.gid = iInput.value;
               iInput.value = '';
            }
            iInput.form.elements[1].focus();
         },
         startPing: function() {
            mnm.PingSave({alias:mnm._data.cf.Alias, to:this.draft.to, gid:this.draft.gid});
            this.draft.to = this.draft.gid = '';
         },
         timer: function(iRec, iText) {
            var aKey = this.rowId(iRec);
            if (!(aKey in this.toSave)) {
               this.toSave[aKey] = {alias:iRec.MyAlias, to:iRec.Alias, gid:iRec.Gid};
               setTimeout(fDing, 2000, this.toSave);
            }
            this.toSave[aKey].text = iText;
            function fDing(cMap) {
               mnm.PingSave(cMap[aKey]);
               delete cMap[aKey];
            }
         },
         setOhiTo: function(iInput) {
            var aOk = iInput.value && iInput.value in mnm._adrsbkmenuId;
            iInput.form.elements[1].disabled = !aOk;
            iInput.form.elements[1].innerText = aOk ? 'o/ '+ iInput.value : 'o/';
            iInput.form.elements[1].value = aOk ? mnm._adrsbkmenuId[iInput.value] : '';
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-pingresponse">
   <div class="pingresponse">
      <div v-if="ping.Response">
         <a v-if="ping.Response.Tid"
            onclick="mnm.NavigateLink(this.href); return false"
            :href="'#'+ ping.Response.Tid"><span uk-icon="mail"></span></a>
         <span v-else
               title="Responded by invite"
               uk-icon="rss"></span>
         <mnm-date :iso="ping.Response.Date" ymd="md"/>
      </div>
      <div v-if="ping.ResponseInvt">
         <span v-if="ping.ResponseInvt.Type === 9"
               title="Accepted invitation"
               uk-icon="check"></span>
         <span v-else-if="ping.ResponseInvt.Type === 10"
               title="Recipient joined group"
               uk-icon="plus-circle"></span>
         <mnm-date :iso="ping.ResponseInvt.Date" ymd="md"/>
      </div>
   </div>
</script><script>
   Vue.component('mnm-pingresponse', {
      template: '#mnm-pingresponse',
      props: {ping:Object},
   });
</script>

<script type="text/x-template" id="mnm-svcadd">
   <div uk-dropdown="mode:click; offset:2; pos:bottom-right" class="uk-width-1-5"
        @hidden="verify = !(addr = name = alias = lpin = loginperiod = null)">
      <div class="uk-float-right uk-text-small">ADD ACCOUNT</div>
      <form :action="'/v/+' + encodeURIComponent(name)"
            method="POST" enctype="multipart/form-data"
            onsubmit="mnm.Upload(this); return false;">
         <input type="hidden" name="filename" :value="JSON.stringify($data)">
         <button :disabled="!(name  && name.length  >= [{.serviceMin}] &&
                              alias && alias.length >= [{.aliasMin}] &&
                              addr && !isNaN(loginperiod))"
                 title="Register new account at service"
                 class="btn-icon"><span uk-icon="forward"></span></button>
         <input v-model="name"
                placeholder="Title ([{.serviceMin}]+ characters)" type="text"
                class="width100">
         <input v-model="alias"
                placeholder="Alias ([{.aliasMin}]+ characters)"   type="text"
                class="width100">
         <input v-model="addr"
                placeholder="Net Address (host:port)"             type="text"
                class="width100">
         <!--todo input v-model="lpin"
                @input="loginperiod = mnm._stringToSeconds($event.target.value)"
                placeholder="(Pd days:hh:mm:ss)"                  size="19" type="text">
         <div v-show="loginperiod"
              style="float:right">{{loginperiod}} sec</div -->
         <br>
         <label><input v-model="verify" type="checkbox"> Verify host (TLS certificate)</label>
      </form>
   </div>
</script><script>
   Vue.component('mnm-svcadd', {
      template: '#mnm-svcadd',
      data: function() { return {addr:null, name:null, alias:null, lpin:null, loginperiod:null,
                                 verify:true} },
      computed: { mnm: function() { return mnm } },
   });
</script>

<script type="text/x-template" id="mnm-svccfg">
   <div uk-dropdown="mode:click; offset:-4; pos:left-top" class="uk-width-1-5">
      <div class="uk-float-right uk-text-small">SETTINGS</div>
      <form onsubmit="return false">
         <button @click="sendUpdate"
                 :disabled="!(addr || verify || historylen >= 0 || loginperiod >= 0)
                            || isNaN(historylen) || isNaN(loginperiod)"
                 title="Update settings"
                 class="btn-icon"><span uk-icon="forward"></span></button>
         <table class="svccfg">
            <tr><td>Thread History </td><td>{{mnm._data.cf.HistoryLen}}<br>
               <input v-model="hlin"
                      @input="historylen = parseInt($event.target.value || '-1')"
                      placeholder="4 to 1024" type="text"
                      class="width100"></td></tr>
            <tr><td>Net Address    </td><td>{{mnm._data.cf.Addr  }}<br>
               <input v-model="addr"
                      placeholder="New host:port" type="text"
                      class="width100"></td></tr>
            <!--todo tr><td>Login Period   </td><td>{{mnm._secondsToString(mnm._data.cf.LoginPeriod)}}<br>
               <input v-model="lpin"
                      @input="loginperiod = toSeconds($event.target.value)"
                      placeholder="New days:hh:mm:ss" size="25" type="text"></td></tr -->
            <tr><td>Verify<br>host </td><td>{{mnm._data.cf.Verify}}<br>
               <label><input v-model="verify" type="checkbox"><tt>Toggle</tt></label></td></tr>
            <tr><td>Title          </td><td>{{mnm._data.cf.Name  }}</td></tr>
            <tr><td>Alias          </td><td>{{mnm._data.cf.Alias ||
                                              mnm._data.cf.Error }}</td></tr>
            <tr><td>Uid            </td><td>{{mnm._data.cf.Uid   }}</td></tr>
         </table>
      </form>
   </div>
</script><script>
   Vue.component('mnm-svccfg', {
      template: '#mnm-svccfg',
      data: function() { return {hlin:null, addr:null, lpin:null, verify:false,
                                 historylen:-1, loginperiod:-1} },
      computed: { mnm: function() { return mnm } },
      methods: {
         toSeconds: function(i) {
            var a = mnm._stringToSeconds(i);
            return a === null ? -1 : a;
         },
         sendUpdate: function() {
            mnm.ConfigUpdt(this.$data);
            this.hlin = this.addr = this.lpin = null;
            this.verify = false;
            this.historylen = this.loginperiod = -1;
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-tabs">
   <ul uk-tab style="margin-top:0; margin-bottom:0;"><li style="display:none"></li>
      <template v-for="(aTabs, aI) in set">
         <li v-for="(aTerm, aJ) in aTabs"
             :class="{'uk-active': aI === state.PosFor && aJ === state.Pos}">
            <a @click.prevent="mnm.TabSelect({type:state.Type, posfor:aI, pos:aJ})" href="#">
               {{ aTerm }}
               <span v-if="aI > 0 && aI === state.PosFor && aJ === state.Pos"
                     @click.prevent.stop="mnm.TabDrop(state.Type)">&times;</span>
               <span v-else-if="aI > 0"
                     style="visibility:hidden">&times;</span>
            </a></li></template></ul>
</script><script>
   Vue.component('mnm-tabs', {
      template: '#mnm-tabs',
      props: {set:Array, state:Object},
      computed: { mnm: function() { return mnm } }
   });
</script>

<script>
;(function() {
   var sChange = 0;
   var sTemp = {ml:null, mo:null};

   mnm._mdi = markdownit();
   mnm._adrsbkmenuId = null;
   mnm._lastPreview = '';
   mnm._data = {
   // global
      v:[], t:[], f:[], fo:'', nlo:[], // fo populated by f requests
   // per client
      cs:{SvcTabs:{Default:[], Pinned:[], Terms:[]}, ThreadTabs:{Terms:[]}},
      sort:{cl:'Who', al:'Date', t:'Date', f:'Date'}, //todo move to cs
      ohiFrom:true, //todo move to cs
   // per service
      cf:{}, nl:[], tl:[], ffn:'', // ffn derived from tl
      ps:[], pt:[], pf:[], gl:[], ot:[], of:null,
   // per thread
      cl:[[],[]], al:[], ao:{}, ml:[], mo:{}, // ao populated by an requests
      toSave:{}, draftRefs:{}, // populated locally
   };

   var sApp = new Vue({
      template: '#mnm-main',
      data: mnm._data,
      methods: {
         tabSearch: function(iText, iState) {
            if (iText.length === 0)
               return;
            if (iState.Pinned)
               for (var a=0; a < iState.Pinned.length; ++a)
                  if (iState.Pinned[a] === iText)
                     return;
            for (var a=0; a < iState.Terms.length; ++a)
               if (iState.Terms[a] === iText)
                  return;
            mnm.TabAdd({type:iState.Type, term:iText});
         },
         msgToggle: function(iId) {
            if (!(iId in mnm._data.mo)) {
               Vue.set(mnm._data.mo, iId, {});
               mnm.ThreadOpen(iId);
            } else {
               mnm.ThreadClose(iId);
               Vue.delete(mnm._data.mo, iId);
            }
         },
         getReplyTemplate: function(iIdxEl) {
            return {alias: mnm._data.cf.Alias, data: '',
                    subject: iIdxEl === mnm._data.ml[mnm._data.ml.length-1] ? '' : iIdxEl.Subject};
         },
      },
      computed: {
         mnm:       function() { return mnm },
         svcTabset: function() {
            return [[], mnm._data.cs.SvcTabs.Pinned, mnm._data.cs.SvcTabs.Terms];
         },
         msgTabset: function() {
            var aT = mnm._data.cs.ThreadTabs;
            return aT ? [aT.Default, [], aT.Terms] : [];
         },
         msgTitle: function() {
            for (var a=0; a < mnm._data.ml.length; ++a) {
               var aM = mnm._data.ml[a];
               if (aM.From !== '')
                  return aM.Subject || this.msgSubjects[this.msgSubjects.length-1].name;
            }
            return a === 1 ? this.msgSubjects[0].name : '';
         },
         msgSubjects: function() {
            var aList = [];
            for (var a = mnm._data.ml.length-1; a >= 0; --a) {
               var aM = mnm._data.ml[a];
               if (aList.length === 0 || (aM.From !== '' && aM.Subject !== '' &&
                                          !aList.find(function(c){ return c.name === aM.Subject })))
                  aList.unshift({msgId:aM.Id, name: aM.Subject ||
                     '\u25b8'+ (aM.From === '' ? 'Untitled Draft' : 'Subject Missing') +'\u25c2'});
            }
            return aList;
         },
         nlNotSeen: function() {
            for (var aN=0; aN < mnm._data.nl.length && !mnm._data.nl[aN].Seen; ++aN) {}
            return aN || null;
         },
         ffnCol: function() {
            if (!mnm._data.ffn) return {};
            var aSet = {};
            for (var a=0; a < mnm._data.tl.length; ++a)
               for (var aKey in mnm._data.tl[a])
                  aSet[aKey] = true;
            return aSet;
         },
      },
   });

   var sUrlStart = /^[A-Za-z][A-Za-z0-9+.\-]*:/;
   mnm._mdi.renderer.rules.link_open = function(iTokens, iIdx, iOptions, iEnv, iSelf) {
      var aHref = iTokens[iIdx].attrs[iTokens[iIdx].attrIndex('href')];
      if (aHref[1].charAt(0) === '#') {
         iTokens[iIdx].attrs.push(['onclick','mnm.NavigateLink(this.href);return false']);
      } else if (!sUrlStart.test(aHref[1])) {
         var aParam = aHref[1].replace(/^this_/, iEnv.thisVal+'_');
         aHref[1] = '?an=' + encodeURIComponent(aParam);
      }
      return iSelf.renderToken(iTokens, iIdx, iOptions);
   };

   var sMdiRenderImg = mnm._mdi.renderer.rules.image;
   mnm._mdi.renderer.rules.image = function(iTokens, iIdx, iOptions, iEnv, iSelf) {
      var aAlt = iSelf.renderInlineAsText(iTokens[iIdx].children, iOptions, iEnv);
      var aSrc = iTokens[iIdx].attrs[iTokens[iIdx].attrIndex('src')];
      var aParam = aSrc[1].replace(/^this_/, iEnv.thisVal+'_');
      if (aAlt.charAt(0) === '?') {
         if (!iEnv.formview)
            iEnv.formview = new mnm._FormViews(iEnv);
         var aId = iEnv.formview.make(aParam);
         return '<component'+ iSelf.renderAttrs({attrs:[['id',aId]]}) +'></component>';
      }
      aSrc[1] = '?an=' + encodeURIComponent(aParam);
      return sMdiRenderImg(iTokens, iIdx, iOptions, iEnv, iSelf);
   };

   mnm._listSort = function(iName, iKey) {
      var aTmp;
      var aList = iName === 'cl' ? mnm._data.cl[1] : mnm._data[iName];
      mnm._data.sort[iName] = iKey;
      aList.sort(function(cA, cB) {
         if (iKey === 'Date')
            aTmp = cA, cA = cB, cB = aTmp;
         if (cA[iKey] <= cB[iKey])
            return -1;
         return 1;
      });
   };

   mnm._toClipboard = function(iRef) {
      var aEl = document.getElementById('toclipboard');
      aEl.value = iRef;
      aEl.style.display = 'inline';
      aEl.select();
      document.execCommand('copy');
      aEl.style.display = 'none';
   };

   mnm._stringToSeconds = function(iStr) {
      var aNum = iStr.split(':', 5);
      if (aNum.length > 4)
         return NaN;
      var aSec = null;
      if (aNum.length > 0 && aNum[0] !== '') aSec += aNum[0]*24*60*60;
      if (aNum.length > 1 && aNum[1] !== '') aSec += aNum[1]*60*60;
      if (aNum.length > 2 && aNum[2] !== '') aSec += aNum[2]*60;
      if (aNum.length > 3 && aNum[3] !== '') aSec += aNum[3]*1;
      return aSec;
   };

   mnm._secondsToString = function(iNum) {
      return luxon.Duration.fromMillis(iNum * 1000).toFormat('d:hh:mm:ss');
   };

   mnm._canScroll = function(iEl, iDeltaY) {
      if (iDeltaY < 0)
         return iEl.scrollTop > 0;
      if (iDeltaY > 0)
         return iEl.scrollTop < iEl.scrollHeight - iEl.clientHeight;
      return false;
   };

   mnm.Log = function(i) {
      var aLog = document.getElementById('log').innerText;
      document.getElementById('log').innerText = (i.substr(-1) === '\n' ? i : i+'\n')+aLog;
   };

   mnm.Quit = function() {
      document.body.click(); // close dropdowns
      document.getElementById('app-quit').style.display = 'block';
   };

   mnm.Render = function(i, iData, iEtc) {
      if (i.charAt(0) === '/')
         i = i.substr(1);

      if (sChange && (i === 'ml' || i === 'mo')) {
         sTemp[i] = iEtc || JSON.parse(iData);
         if (++sChange === 2)
            return;
         sChange = 0;
         mnm._data.ml = sTemp.ml;
         iEtc         = sTemp.mo;
         i = 'mo';
      }

      switch (i) {
      case 'cs': case 'cf': case 'nl': case 'cl': case 'al': case 'ml':
      case 'ps': case 'pt': case 'pf': case 'gl': case 'ot': case 'of':
      case 't' : case 'f' : case 'v' : case 'nlo':
         if (i === 'f' && iEtc) {
            mnm._data.fo = iData;
         } else {
            mnm._data[i] = JSON.parse(iData);
            if (i === 'cl' || i === 'al' || i === 't' || i === 'f')
               sApp.$refs[i].listSort(mnm._data.sort[i]);
            else if (i === 'v')
               mnm._data.v.sort();
         }
         break;
      case 'tl':
         var aData = JSON.parse(iData);
         if ('Ffn' in aData) {
            mnm._data.tl = aData.Table;
            mnm._data.ffn = aData.Ffn;
         } else {
            mnm._data.tl = aData;
            mnm._data.ffn = '';
         }
         break;
      case 'an':
         Vue.set(mnm._data.ao, iEtc, iData)
         break;
      case 'mo':
         for (var aK in mnm._data.mo)
            if (!(aK in iEtc))
               Vue.delete(mnm._data.mo, aK);
         for (var aK in mnm._data.toSave)
            if (!(aK in iEtc))
               Vue.delete(mnm._data.toSave, aK);
         for (var aK in iEtc)
            if (!(aK in mnm._data.mo))
               Vue.set(mnm._data.mo, aK, iEtc[aK]);
         break;
      case 'mn':
         // avoid opening Attach menu if not changed
         var aOrig = mnm._data.mo[iEtc.Id] && mnm._data.mo[iEtc.Id].SubHead;
         if (aOrig) {
            if (!fDiff('Attach')) iEtc.SubHead.Attach = aOrig.Attach;
         }
         Vue.set(mnm._data.mo, iEtc.Id, iEtc); //todo set ml Date
         function fDiff(c) {
            if ( aOrig[c]         === iEtc.SubHead[c])         return false;
            if (!aOrig[c]         || !iEtc.SubHead[c])         return true;
            if ( aOrig[c].length  !== iEtc.SubHead[c].length)  return true;
            if ( aOrig[c].length  === 0)                       return false;
            if ( aOrig[c][0].Name === iEtc.SubHead[c][0].Name) return false; //todo full comparison?
            return true;
         }
         var aOrig = mnm._data.toSave[iEtc.Id];
         if (aOrig) {
            if (!aOrig.suUpdt)
               aOrig.Subject = iEtc.SubHead.Subject;
            if (!aOrig.mdUpdt)
               aOrig.msg_data = iEtc.msg_data;
            for (var a in aOrig.ffUpdt) {
               if (aOrig.ffUpdt[a] === 'keep' && iEtc.form_fill && a in iEtc.form_fill) {
                  aOrig.ffUpdt[a] = null;
               } else if (aOrig.ffUpdt[a] === 'save' || aOrig.ffUpdt[a] === 'keep') {
                  if (!iEtc.form_fill)
                     iEtc.form_fill = {};
                  iEtc.form_fill[a] = aOrig.form_fill[a];
               }
            }
            aOrig.form_fill = iEtc.form_fill;
         }
         break;
      case 'nameset':
         mnm._adrsbkmenuId = {};
         var aList = [];
         for (var a=0; a < iEtc.length; a+=2) {
            mnm._adrsbkmenuId[iEtc[a]] = iEtc[a+1];
            aList.push(iEtc[a]);
         }
         sApp.$refs.adrsbkmenu.results(aList);
         break;
      }
   };

   mnm.ThreadChange = function() {
      sChange = 1;
      mnm._lastPreview = '';
   };

   window.addEventListener('keydown', function(iEvent) {
      if (iEvent.ctrlKey && iEvent.key === 'j') {
         iEvent.preventDefault();
         if (mnm._lastPreview) {
            var aEl = document.getElementById(mnm._lastPreview);
            if (aEl) aEl.click();
         }
      }
   });

   sApp.$mount('#app');
   window.onload = mnm.Connect;

}).call(this);
</script>

</body></html>

