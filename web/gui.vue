<!DOCTYPE html>
<html><head>
   <title>[{.Title}] - mnm</title>
   <link rel="icon" href="/web/favicon.png"/>

   <meta charset="utf-8">
   <meta name="viewport" content="width=device-width, initial-scale=1">

   <link  href="/web/uikit-30.min.css" rel="stylesheet"/>
   <script src="/web/uikit-30.min.js"></script>
   <script src="/web/uikit-icons-30.min.js"></script>

   <script src="/web/vue-25.js"></script>
   <script src="/web/markdown-it-84.js"></script>
   <script src="/web/luxon-13.js"></script>
   <link  href="/web/vue-formgen-22.css" rel="stylesheet"/>
   <script src="/web/vue-formgen-22.js"></script>

   <link  href="/web/service.css" rel="stylesheet"/>
   <script src="/web/socket.js"></script>
   <script>
      mnm._mdi = markdownit();
      mnm._adrsbkmenu = null;
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
         ps:[], pt:[], pf:[], it:[], if:[], gl:[], ot:[], of:null,
      // per thread
         cl:[[],[]], al:[], ao:{}, ml:[], mo:{}, // ao populated by an requests
         toSave:{}, // populated locally
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
   </script>
</head><body>
<base target="_blank">

<div id="app"></div>

<input id="toclipboard" style="display:none">

<script type="text/x-template" id="mnm-main">
<div uk-grid class="uk-grid-small">

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
         <mnm-cc @ccadd="ccAdd" @ccdrop="ccDrop"
                 ref="cl" :tid="ml.length ? ml[ml.length-1].Id : 'none'"/>
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
                  <div @keypress="keyAction('pv_'+aMsg.Id, $event)">
                     <div style="position:relative; padding:1px;">
                        <button @click="mnm.ThreadSend(aMsg.Id)"
                                title="Send draft"
                                class="btn-icon btn-alignt"><span uk-icon="forward"></span></button>
                        <div style="height:100%; position:absolute; left:13em; right:42px; top:0;">
                           <mnm-draftmenu :list="mo[aMsg.Id].SubHead.Attach"
                                          :msgid="aMsg.Id" :drop="atcDrop"
                                          :getname="atcGetName" :getkey="atcGetKey"
                                          :style="{float:'right'}"/>
                        </div>
                     </div>
                     <div style="float:right; margin-top:-1.7em;">
                        <span uk-icon="push"      :id="'t'+aMsg.Id" class="dropdown-icon"></span
                       ><span uk-icon="file-edit" :id="'f'+aMsg.Id" class="dropdown-icon"></span>
                        <span :id="'pv_'+aMsg.Id"></span>
                        <div uk-dropdown="mode:click; pos:right-top" class="uk-width-2-5"
                             style="overflow:auto; max-height:75vh;
                                    border-top:1em solid white; border-bottom:1em solid white;">
                           <mnm-markdown @formfill="ffAdd(aMsg.Id, arguments[0], arguments[1])"
                                         @toggle="atcToggleFf(aMsg.Id, arguments[0], arguments[1])"
                                         :src=     "(toSave[aMsg.Id] || mo[aMsg.Id]).msg_data"
                                         :formfill="(toSave[aMsg.Id] || mo[aMsg.Id]).form_fill"
                                         :atchasff="atcHasFf" :msgid="aMsg.Id"/></div>
                     </div>
                     <input @input="subjAdd(aMsg.Id, $event.target.value)"
                            :value="(toSave[aMsg.Id] || mo[aMsg.Id].SubHead).Subject"
                            placeholder="Subject" type="text" style="width:100%">
                     <mnm-textresize @input.native="textAdd(aMsg.Id, $event.target.value)"
                                     :src="(toSave[aMsg.Id] || mo[aMsg.Id]).msg_data"
                                     placeholder="Ctrl-J to Preview" style="width:100%"/>
                  </div>
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
      <mnm-adrsbkmenu/>
   </div>
</div>

<div class="uk-width-1-2">
   <span v-for="aMsg in ml" :key="aMsg.Id"
         v-if="mo[aMsg.Id] && mo[aMsg.Id].Posted === 'draft'">
      <mnm-files @attach="atcAdd(aMsg.Id, arguments[0])"
                 :toggle="'#t'+aMsg.Id" pos="right-top"/>
      <mnm-forms @attach="atcAdd(aMsg.Id, arguments[0])"
                 :toggle="'#f'+aMsg.Id" pos="right-top"/>
   </span>
   <div class="uk-clearfix">
      <span class="uk-text-large">
         <span uk-icon="world"></span>
         [{.Title}]
      </span>
      <div class="uk-float-right">
         <span uk-icon="reply" class="dropdown-icon" style="font-weight:bold">{{nlNotSeen}}</span>
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
         <template v-else>
            <div v-for="aRow in tl"
                 onclick="this.lastChild.click()"
                 uk-grid class="uk-grid-small" style="margin:0; padding:0.25em 0; cursor:pointer"
                 :style="{'background-color': aRow.Id === cs.Thread ? 'wheat' : 'inherit'}">
               <div class="uk-width-auto" style="padding:0">
                  <mnm-date :iso="aRow.Date" ymd="md"/></div>
               <div v-if="aRow.Id.indexOf('/') < 0"
                    class="uk-width-1-6">{{'Last Author'}}</div>
               <div class="uk-width-expand">
                  {{'Something'}} {{aRow.Id}}
               </div>
               <div class="uk-width-auto">
                  <mnm-date :iso="'2018-01-17T04:16:57Z'"/></div>
               <div v-if="aRow.Id.indexOf('/') < 0"
                    class="uk-width-1-6">{{'Orig Author'}}</div>
               <span v-if="aRow.Id.indexOf('/') >= 0"
                     @click="tabSearch('ffn:'+aRow.Id, cs.SvcTabs)"
                     style="padding:0"></span>
               <span v-else
                     @click="mnm.NavigateThread(aRow.Id)"
                     style="padding:0"></span>
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
         <div v-else-if="of.length === 0"
              class="uk-text-warning">no o/</div>
         <ul v-else
             class="uk-list uk-text-success" style="margin-bottom:0">
            <li v-for="aUser in of" :key="aUser.Uid">
               {{aUser.Uid}}</li>
         </ul>
      </div>
   </div>
</div>

<div class="uk-width-expand service-panel">
   <div class="uk-clearfix uk-light">
      <span uk-icon="plus-circle" class="dropdown-icon"></span>
      <mnm-svcadd/>
      <div style="float:right; margin:0 1em 1em 0">
         <span uk-icon="cog" class="dropdown-icon">&nbsp;</span>
         <div uk-dropdown="mode:click; offset:2; pos:bottom-right" class="uk-width-1-5">
            <div class="uk-text-right uk-text-small">SETTINGS</div></div>
         <span uk-icon="info" class="dropdown-icon">&nbsp;</span>
         <div uk-dropdown="mode:click; offset:2; pos:bottom-right"
              class="uk-width-3-5" style="height:75vh; padding:0.8em">
            <iframe src="/web/docs.html" style="width:100%; height:100%"></iframe></div>
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
               <span uk-icon="reply" :id="'n'+aSvc" class="dropdown-icon">0{{aSvc.todo}} </span>
               <a :href="'/'+encodeURIComponent(aSvc)" :target="'mnm_'+aSvc">{{aSvc}}</a>
            </template>
         </li></ul>
   </div>
   <!--todo create notice menus dynamically-->
   <mnm-notice v-for="aSvc in v" :key="aSvc"
               :svc="aSvc" offset="-4" pos="left-top" :toggle="'#n'+aSvc"
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
      <input v-model="note" placeholder="Note" size="57" maxlength="1024" type="text">
      <div style="position:relative; padding:1px;">
         <mnm-adrsbkinput @keyup.enter.native="addUser"
                          :type="3" placeholder="+To" size="25"/>
         <div style="height:100%; position:absolute; left:13em; right:0; top:0;">
            <mnm-draftmenu ref="menu" :list="menu" :drop="dropUser"/></div>
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
               this.$refs['menu'].$forceUpdate();
            return this.lastMenu = aMenu;
         },
         mnm: function() { return mnm }
      },
      methods: {
         now: function() { return luxon.DateTime.local() },
         addUser: function(iEvt) {
            this.$emit('ccadd', this.tid, this.ccSet, iEvt.target.value, this.note);
            iEvt.target.value = '';
         },
         dropUser: function(iJunk, iItem) {
            var aCc = mnm._data.cl[this.ccSet];
            for (var a=0; a <= iItem; ++a)
               if (aCc[a].WhoUid === aCc[a].ByUid)
                  ++iItem;
            this.$emit('ccdrop', this.tid, this.ccSet, iItem);
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

<script type="text/x-template" id="mnm-adrsbkinput">
   <div style="display:inline-block">
      <input @focus="menu.placeEl($el, type, $event.target.value)"
             @blur ="menu.hideEl()"
             @input="menu.search(type, $event.target.value)"
             @keypress.down ="menu.selectItem($event.target,  1)"
             @keypress.up   ="menu.selectItem($event.target, -1)"
             @keypress.esc  ="menu.selectNone($event.target)"
             @keyup.enter   ="menu.clear()"
             v-bind="$attrs" type="text">
      <!--menu appended here-->
   </div>
</script><script>
   Vue.component('mnm-adrsbkinput', {
      template: '#mnm-adrsbkinput',
      props: {type:Number},
      inheritAttrs: false,
      computed: { menu: function() { return mnm._adrsbkmenu } },
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
              :title="'Use \u2191\u2193 keys to select'"
              :class="{'adrsbkmenu-select': aI === select}">{{aName}}</div>
      </div></div>
</script><script>
   Vue.component('mnm-adrsbkmenu', {
      template: '#mnm-adrsbkmenu',
      data: function() { return { query:'', list:[], select:-1 } },
      created: function() { mnm._adrsbkmenu = this },
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
         <div @click="drop(msgid, aI)"
              class="draftmenu-x">&times;</div>
         <br>
      </span>
   </div>
</script><script>
   Vue.component('mnm-draftmenu', {
      template: '#mnm-draftmenu',
      props: {msgid:String, list:Array, drop:Function, getname:Function, getkey:Function},
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
   <textarea @input="resize" @click.stop :value="src" class="text-resize"></textarea>
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
   <div>
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
                  :schema="formDef" :model="formState" :options="{}"/>
   </div>
</script><script>
   Vue.component('mnm-formview', {
      template: '#mnm-formview',
      props: {file:String, fillMap:Object, parent:Object},
      data: function() {
         return { formState: JSON.parse(this.fillMap[this.file] || '{}') };
      },
      computed: {
         formDef: function() {
            try {
               return this.file in mnm._data.ao ? JSON.parse(mnm._data.ao[this.file]) : {};
            } catch(a) {
               return this.parent.formDefBad;
            }
         },
         atcHasFf: function() {
            return this.parent.atchasff(this.parent.msgid, this.file);
         },
      },
      methods: {
         onInput: function() {
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
      watch: {
         fillMap: { deep: true, handler:
            function() {
               this.formState = JSON.parse(this.fillMap[this.file]);
            }
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
         return;
      }
      mnm.AttachOpen(iKey);
      this.comp[iKey] = [ new (Vue.component('mnm-formview'))({
         propsData: { file:iKey, fillMap:this.env.fillMap, parent:this.env.parent },
      }), null ];
   };
   mnm._FormViews.prototype.remount = function() {
      for (var a in this.comp) {
         var aEl = document.getElementById(a);
         if (!aEl) continue;
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
            <input v-model="upname" type="text" size="40" placeholder="Alt Name">
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
         <input v-model="upname" type="text" size="40" placeholder="New Type">
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
               <a @click.stop.prevent="revOpen(aSet.Name,aFile.Id,$event.target)"
                  :id="'bf_'+aSet.Name+'.'+aFile.Id" href="#">
                  <span uk-icon="triangle-left">&nbsp;</span>{{aSet.Name}}.{{aFile.Id}}</a>
               <form v-if="!toggle"
                     :action="'/f/-' + encodeURIComponent(aSet.Name+'.'+aFile.Id)" method="POST"
                     onsubmit="mnm.Upload(this); return false;"
                     style="float:right">
                  <button title="Erase form"
                          class="btn-iconred"><span uk-icon="trash"></span></button>
               </form>
            </li></template></ul>
         <div v-show="setName"
              class="uk-card uk-card-default uk-card-small uk-card-body uk-width-1-1"
              style="position:absolute; right:20em;" :style="{top:revPos}" @click.stop>
            <div class="uk-text-right uk-text-small">
               {{(setName+'.'+fileId).toUpperCase()}}</div>
            <div v-if="!mnm._data.fo"
                 class="uk-text-center" style="padding:0.5em">
               <span uk-icon="future"></span></div>
            <form v-else
                  :action="'/f/+' + encodeURIComponent(setName+'.'+fileId)"
                  method="POST" enctype="multipart/form-data"
                  onsubmit="mnm.Upload(this); return false;"
                  style="margin-top:-1.5em" class="pane-clip">
               <span @click="codeShow = !codeShow"
                     class="uk-link">{...}</span>
               <button :disabled="!!parseError"
                       title="Save form"
                       class="btn-icon"><span uk-icon="file-edit"></span></button>
               <div style="font-size:smaller; text-align:right">&nbsp;{{parseError}}</div>
               <div class="pane-slider" :class="{'pane-slider-rhs':codeShow}">
                  <div class="pane-scroller" style="min-height:1px">
                     <plugin-vfg :schema="formDef" :model="{}" :options="{}"/></div>
                  <div class="pane-scroller">
                     <mnm-textresize @input.native="mnm._data.fo=$event.target.value"
                                     :src="mnm._data.fo"
                                     name="filename" style="width:100%"/></div>
               </div>
            </form>
            <form :action="'/f/*' + encodeURIComponent(setName+'.'+fileId) +
                              '+' + encodeURIComponent(dupname)" method="POST"
                  onsubmit="mnm.Upload(this); return false;">
               <input v-model="dupname" type="text" size="40" placeholder="New Revision">
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
         return {upname:'', dupname:'', setName:'', fileId:'', revPos:'', codeShow:false, dupShow:''};
      },
      computed: {
         mnm: function() { return mnm },
         formDef: function() {
            try { return JSON.parse(mnm._data.fo) }
            catch(a) { return {fields:[ {type:"label", label:"code incorrect"} ]} }
         },
         parseError: function() {
            try { JSON.parse(mnm._data.fo) }
            catch(aErr) { return aErr.message.slice(12,-17) }
            return '';
         },
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
            mnm.FormOpen(iSet+'.'+iRev);
            mnm._data.fo = this.dupname = '';
            this.setName = iSet;
            this.fileId = iRev;
            this.revPos = iEl.offsetTop + 'px';
            this.codeShow = false;
         },
         revClose: function() {
            this.setName = this.fileId = '';
         },
      },
      watch: {
         data: function() {
            if (!this.dupShow)
               return;
            this.$nextTick(function() {
               var aEl = document.getElementById('bf_'+this.setName+'.'+this.dupShow);
               if (aEl) {
                  this.revOpen(this.setName, this.dupShow, aEl);
                  this.dupShow = '';
               }
            });
         },
      },
      components: { 'plugin-vfg': VueFormGenerator.component },
   });
</script>

<script type="text/x-template" id="mnm-pingresponse">
   <span v-if="response.Type">
      <a v-if="response.Tid"
         onclick="mnm.NavigateLink(this.href); return false"
         :href="'#'+ response.Tid"><span uk-icon="mail"></span></a>
      <template v-else>
         ping</template>
      <mnm-date :iso="response.Date"/>
   </span>
</script><script>
   Vue.component('mnm-pingresponse', {
      template: '#mnm-pingresponse',
      props: {response:Object},
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
         <span v-for="aType in ['ping', 'invite']"
               @click="$data[aType] = !$data[aType]"
               class="uk-link" style="margin-right:0.5em">
            <span :style="{visibility: $data[aType] ? 'visible' : 'hidden'}">&bull; </span>
            {{ aType }}s</span>
      </div>
      <ul class="uk-list dropdown-scroll-list notice">
         <li v-if="!nl.length"
             style="text-align:center">No notices yet</li>
         <li v-for="aNote in nl" :key="aNote.MsgId"
             v-if="$data[aNote.Type]"
             @click="$set(aNote, 'open', !aNote.open)"
             :class="{'notice-seen':aNote.Seen, 'notice-hasblurb':aNote.Blurb}">
            <div style="float:left; font-style:oblique">{{aNote.Type.charAt(0)}}</div>
            <div style="margin-left:1em">
               <div style="float:right"><mnm-date :iso="aNote.Date" ymd="md" hms="hm"/></div>
               <template v-if="aNote.Type === 'invite'">
                  {{aNote.Alias}} - {{aNote.Gid}}</template>
               <template v-else-if="aNote.Type === 'ping'">
                  {{aNote.Alias}}</template>
               <span v-show="aNote.Blurb && !aNote.open">. . .</span>
               <div v-show="aNote.open">{{aNote.Blurb}}</div>
            </div>
         </li></ul>
   </div>
</script><script>
   Vue.component('mnm-notice', {
      template: '#mnm-notice',
      props: {svc:String, toggle:String},
      data: function() { return { ping:true, invite:true } },
      computed: {
         nl: function() { return this.svc ? mnm._data.nlo : mnm._data.nl },
         mnm: function() { return mnm },
      },
   });
</script>

<script type="text/x-template" id="mnm-adrsbk">
   <div uk-dropdown="mode:click; offset:2; pos:bottom-right" class="uk-width-2-5 dropdown-scroll">
      <ul uk-tab class="uk-child-width-expand" style="margin-top:0; margin-right:20px">
         <li v-for="aName in ['pings','invites','drafts','pinged','invited','groups','ohi to']">
            <a @click.prevent="" href="#" style="cursor:default">{{aName}}</a>
         </li></ul>
      <ul class="uk-switcher dropdown-scroll-list">
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>From</th><th>Message</th><th>Response</th></tr>
               <tr v-for="a in mnm._data.pf">
                  <td><mnm-date :iso="a.Date"/></td>
                  <td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"/></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>Group</th><th>From</th><th>Msg</th><th>Response</th></tr>
               <tr v-for="a in mnm._data.if">
                  <td><mnm-date :iso="a.Date"/></td>
                  <td>{{a.Gid}}
                     <span v-if="mnm._data.gl.find(function(c){return c.Gid === a.Gid})"
                           class="uk-badge">in</span>
                     <span v-else-if="a.Queued"
                           title="Awaiting link to server"
                           uk-icon="bolt"></span>
                     <button v-else
                             @click="mnm.InviteAccept(a.Qid)"
                             title="Accept group invite"
                             class="btn-icon"><span uk-icon="forward"></span></button>
                  </td>
                  <td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"/></td>
               </tr></table></li>
         <li>
            <form onsubmit="return false"
                  style="margin: 0 auto; display:table">
               <input v-model="draft.to" placeholder="To" size="25" type="text">
               <div style="display:inline-block; vertical-align:top">
                  <input v-model="draft.gid" placeholder="(Group)" size="25" type="text">
                  <br>
                  <mnm-adrsbkinput @keyup.enter.native="setGid($event.target)"
                                   @keypress.enter.native.prevent=""
                                   :type="2" placeholder="Search groups" size="25"/>
               </div>
               <button @click="startPing()"
                       :disabled="!validDraft"
                       title="New ping draft"
                       class="btn-icon"><span uk-icon="pencil"></span></button>
            </form>
            <table class="uk-table uk-table-small" style="margin:0">
               <tr><th>To / (Group)</th><th></th><th>Message</th><th></th></tr>
               <tr v-for="a in mnm._data.ps" :key="rowId(a)">
                  <td>{{a.Alias}}<br>{{a.Gid && '('+a.Gid+')'}}</td>
                  <td><span v-if="a.Queued"
                            title="Awaiting link to server"
                            uk-icon="bolt"></span>
                      <button v-else
                              @click="mnm.PingSend(a.Qid)"
                              title="Send ping draft"
                              class="btn-icon"><span uk-icon="forward"></span></button></td>
                  <td><textarea @input="timer(a, $event.target.value)"
                                :disabled="a.Queued"
                                cols="40" rows="3" maxlength="120"
                                >{{toSave[rowId(a)] || a.Text}}</textarea></td>
                  <td><button v-if="!a.Queued"
                              @click="mnm.PingDiscard({to:a.Alias, gid:a.Gid})"
                              title="Discard ping draft"
                              class="btn-iconred"><span uk-icon="trash"></span></button></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>To</th><th>Message</th><th>Response</th></tr>
               <tr v-for="a in mnm._data.pt">
                  <td><mnm-date :iso="a.Date"/></td>
                  <td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"/></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>Group</th><th>To</th><th>Message</th><th>Response</th></tr>
               <tr v-for="a in mnm._data.it">
                  <td><mnm-date :iso="a.Date"/></td>
                  <td>{{a.Gid}}</td><td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"/></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>Group</th></tr>
               <tr v-for="a in mnm._data.gl">
                  <td><mnm-date :iso="a.Date"/></td>
                  <td>{{a.Gid}}
                     <span v-if="a.Admin"
                           class="uk-badge">A</span></td>
               </tr></table></li>
         <li>
            <form onsubmit="this.reset(); return false;"
                  style="margin: 0 auto; display:table">
               <mnm-adrsbkinput oninput="this.form.elements[1].disabled = !this.value"
                                :type="1" placeholder="To" size="40"
                                name="resets" autocomplete="off"/>
               <button onclick="mnm.OhiAdd(null, mnm._adrsbkmenuId[this.form.elements[0].value])"
                       disabled
                       title="Notify contact when you're online"
                       class="btn-icontxt">o/</button>
            </form>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>To</th><th></th></tr>
               <tr v-for="a in mnm._data.ot">
                  <td><mnm-date :iso="a.Date"/></td>
                  <td>{{a.Uid /*todo alias*/}}</td>
                  <td><button @click="mnm.OhiDrop(null,a.Uid)"
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
            if (!this.draft.to)
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
      },
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
         <button :disabled="!(addr && name && alias && alias.length >= 8 && !isNaN(loginperiod))"
                 title="Register new account at service"
                 class="btn-icon"><span uk-icon="forward"></span></button>
         <input v-model="addr"  placeholder="Net Address (host:port)" size="33" type="text">
         <input v-model="name"  placeholder="Title"                   size="33" type="text">
         <input v-model="alias" placeholder="Alias (8+ characters)"   size="33" type="text">
         <input v-model="lpin"  placeholder="(Pd days:hh:mm:ss)"      size="19" type="text"
                @input="loginperiod = mnm._stringToSeconds($event.target.value)">
         <div v-show="loginperiod"
              style="float:right">{{loginperiod}} sec</div>
         <br>
         <label><input v-model="verify" type="checkbox"> Verify identity (TLS certificate)</label>
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
                 :disabled="!(addr || verify || loginperiod >= 0) || isNaN(loginperiod)"
                 title="Update settings"
                 class="btn-icon"><span uk-icon="forward"></span></button>
         <table class="svccfg">
            <tr><td>Net Address    </td><td>{{mnm._data.cf.Addr  }}<br>
               <input v-model="addr" placeholder="New host:port"     size="25" type="text"></td></tr>
            <tr><td>Login Period   </td><td>{{mnm._secondsToString(mnm._data.cf.LoginPeriod)}}<br>
               <input v-model="lpin" placeholder="New days:hh:mm:ss" size="25" type="text"
                      @input="loginperiod = toSeconds($event.target.value)"></td></tr>
            <tr><td>Verify identity</td><td>{{mnm._data.cf.Verify}}<br>
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
      data: function() { return {addr:null, lpin:null, loginperiod:-1, verify:false} },
      computed: { mnm: function() { return mnm } },
      methods: {
         toSeconds: function(i) {
            var a = mnm._stringToSeconds(i);
            return a === null ? -1 : a;
         },
         sendUpdate: function() {
            mnm.ConfigUpdt(this.$data);
            this.verify = !!(this.addr = this.lpin = null);
            this.loginperiod = -1;
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
         keyAction: function(iId, iEvent) {
            if (iEvent.ctrlKey && iEvent.key === 'j')
               mnm._lastPreview = iId;
         },
         getReplyTemplate: function(iIdxEl) {
            return {alias: mnm._data.cf.Alias, data: '',
                    subject: iIdxEl === mnm._data.ml[mnm._data.ml.length-1] ? '' : iIdxEl.Subject};
         },
         draft_tosave: function(iId, iNoTimer) {
            if (!(iId in mnm._data.toSave))
               Vue.set(mnm._data.toSave, iId, {timer:null,
                       form_fill:mnm._data.mo[iId].form_fill,       ffUpdt:false,
                       msg_data: mnm._data.mo[iId].msg_data,        mdUpdt:false,
                       Subject:  mnm._data.mo[iId].SubHead.Subject, suUpdt:false});
            if (!iNoTimer && !mnm._data.toSave[iId].timer)
               mnm._data.toSave[iId].timer =
                  setTimeout(fDing, 2000, this, mnm._data.toSave[iId], mnm._data.mo[iId]);
            return mnm._data.toSave[iId];
            function fDing(cThis, cToSave, cMo) {
               cToSave.timer = null;
               cThis.draft_save(iId, null, null, cToSave, cMo);
            }
         },
         draft_save: function(iId, iCc, iAttach, iToSave, iMo) {
            if (!iToSave) iToSave = this.draft_tosave(iId, true);
            if (!iMo)     iMo = mnm._data.mo[iId];
            if (iToSave.timer) {
               clearTimeout(iToSave.timer);
               iToSave.timer = null;
            }
            mnm.ThreadSave({
               Id:       iId,
               Cc:       iCc     || mnm._data.cl[1],
               Alias:               iMo.SubHead.Alias,
               Attach:   iAttach || iMo.SubHead.Attach,
               FormFill: iToSave.form_fill,
               Data:     iToSave.msg_data,
               Subject:  iToSave.Subject,
            });
            iToSave.suUpdt = iToSave.mdUpdt = iToSave.ffUpdt = false;
         },
         ccAdd: function(iTid, iCcSet, iAlias, iNote) {
            if (!iAlias.length || !(iAlias in mnm._adrsbkmenuId))
               return;
            var aCc = mnm._data.cl[iCcSet].slice();
            var aPrev = aCc.findIndex(function(c) { return c.Who === iAlias });
            var aEl = aPrev >= 0 ? aCc.splice(aPrev, 1)[0]
                                 : {Who:iAlias, WhoUid:mnm._adrsbkmenuId[iAlias]};
            aEl.Note = iNote;
            aCc.unshift(aEl);
            if (iCcSet)
               this.draft_save(iTid, aCc, null);
            else
               mnm.ForwardSave(iTid, aCc);
         },
         ccDrop:  function(iTid, iCcSet, iN) {
            var aCc = mnm._data.cl[iCcSet];
            aCc = aCc.slice(0, iN).concat(aCc.slice(iN+1));
            if (iCcSet)
               this.draft_save(iTid, aCc, null);
            else
               mnm.ForwardSave(iTid, aCc);
         },
         atcAdd: function(iId, iPath) {
            var aAtc = mnm._data.mo[iId].SubHead.Attach;
            aAtc = aAtc ? aAtc.slice() : [];
            var aPrefix = /^upload/.test(iPath) ? 'u:' : /^form_fill/.test(iPath) ? 'r:' : 'f:';
            var aStoredName = iPath.replace(/^[^/]*\//, aPrefix);
            var aPrev = aAtc.findIndex(function(c) { return c.Name === aStoredName });
            if (aPrev >= 0)
               aAtc.splice(aPrev, 1);
            aAtc.unshift({Name:iPath});
            this.draft_save(iId, null, aAtc);
         },
         atcDrop: function(iId, iN) {
            var aAtc = mnm._data.mo[iId].SubHead.Attach;
            this.draft_save(iId, null, aAtc.slice(0, iN).concat(aAtc.slice(iN+1)));
         },
         atcHasFf: function(iId, iFfKey) {
            var aAtc = mnm._data.mo[iId].SubHead.Attach;
            return !! (aAtc && aAtc.find(function(c) { return c.FfKey === iFfKey }));
         },
         atcToggleFf: function(iId, iFfKey, iPath) {
            var aAtc = mnm._data.mo[iId].SubHead.Attach || [];
            var aN = aAtc.findIndex(function(c) { return c.FfKey === iFfKey });
            if (aN < 0)
               this.ffAdd(iId, iFfKey);
            this.draft_save(iId, null, aN < 0
                                       ? [ {Name:iPath, FfKey:iFfKey} ].concat(aAtc)
                                       : aAtc.slice(0, aN).concat(aAtc.slice(aN+1)));
         },
         ffAdd: function(iId, iFfKey, iText) {
            var aToSave = this.draft_tosave(iId, !iText);
            if (!iText) {
               if (aToSave.form_fill && iFfKey in aToSave.form_fill)
                  return;
               iText = '{}';
            }
            if (!aToSave.ffUpdt)
               aToSave.ffUpdt = {};
            if (!aToSave.form_fill)
               aToSave.form_fill = {};
            Vue.set(aToSave.form_fill, iFfKey, iText);
            aToSave.ffUpdt[iFfKey] = true;
         },
         textAdd: function(iId, iText) {
            var aToSave = this.draft_tosave(iId, false);
            aToSave.msg_data = iText;
            aToSave.mdUpdt = true;
         },
         subjAdd: function(iId, iText) {
            var aToSave = this.draft_tosave(iId, false);
            aToSave.Subject = iText;
            aToSave.suUpdt = true;
         },
         atcGetName: function(iEl) { return iEl.Name },
         atcGetKey:  function(iEl) { return iEl.FfKey || iEl.Name },
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
         iEnv.formview.make(aParam);
         return '<component'+ iSelf.renderAttrs({attrs:[['id',aParam]]}) +'></component>';
      }
      aSrc[1] = '?an=' + encodeURIComponent(aParam);
      return sMdiRenderImg(iTokens, iIdx, iOptions, iEnv, iSelf);
   };

   mnm.Log = function(i) {
      var aLog = document.getElementById('log').innerText;
      document.getElementById('log').innerText = (i.substr(-1) === '\n' ? i : i+'\n')+aLog;
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
      case 'ps': case 'pt': case 'pf': case 'it': case 'if': case 'gl': case 'ot': case 'of':
      case 't' : case 'f' : case 'v' : case 'nlo':
         if (i === 'f' && iEtc) {
            mnm._data.fo = iData;
         } else {
            mnm._data[i] = JSON.parse(iData);
            if (i === 'cl' || i === 'al' || i === 't' || i === 'f')
               sApp.$refs[i].listSort(mnm._data.sort[i]);
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
            if (!aOrig.ffUpdt) {
               aOrig.form_fill = iEtc.form_fill;
            } else if (iEtc.form_fill) {
               for (var aK in iEtc.form_fill)
                  if (!aOrig.ffUpdt[aK])
                     Vue.set(aOrig.form_fill, aK, iEtc.form_fill[aK]);
            }
         }
         break;
      case 'nameset':
         mnm._adrsbkmenuId = {};
         var aList = [];
         for (var a=0; a < iEtc.length; a+=2) {
            mnm._adrsbkmenuId[iEtc[a]] = iEtc[a+1];
            aList.push(iEtc[a]);
         }
         mnm._adrsbkmenu.results(aList);
         break;
      }
   };

   mnm.ThreadChange = function() {
      sChange = 1;
      mnm._lastPreview = '';
   };

   window.addEventListener('keypress', function(iEvent) {
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

