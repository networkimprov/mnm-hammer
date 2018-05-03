<!DOCTYPE html>
<html><head>
   <title>[{.Title}] - mnm</title>

   <meta charset="utf-8">
   <meta name="viewport" content="width=device-width, initial-scale=1">

   <link  href="/web/uikit-30.min.css" rel="stylesheet"/>
   <script src="/web/uikit-30.min.js"></script>
   <script src="/web/uikit-icons-30.min.js"></script>

   <script src="/web/vue-25.js"></script>
   <script src="/web/markdown-it-84.js"></script>

   <link  href="/web/vue-formgen-22.css" rel="stylesheet"/>
   <script src="/web/vue-formgen-22.js"></script>

   <link  href="/web/service.css" rel="stylesheet"/>
   <script src="/web/socket.js"></script>
   <script>
      mnm._mdi = markdownit();
      mnm._lastPreview = '';
      mnm._data = {
      // global
         sl:[], t:[], f:[], fo:'', // fo populated by f requests
      // per client
         cs:{SvcTabs:{Default:[], Pinned:[], Terms:[]}},
         sort:{al:'Size', t:'Date', f:'Date'}, //todo move to cs
         ohiFrom:true, //todo move to cs
      // per service
         cf:{}, tl:[], ffn:'', // ffn derived from tl
         ps:[], pt:[], pf:[], it:[], if:[], gl:[], ot:[], of:[],
      // per thread
         al:[], ao:{}, ml:[], mo:{}, // ao populated by an requests
         toSave:{}, // populated locally
      };
      mnm._listSort = function(iList, iKey) {
         var aTmp;
         mnm._data.sort[iList] = iKey;
         mnm._data[iList].sort(function(cA, cB) {
            if (iKey === 'Date')
               aTmp = cA, cA = cB, cB = aTmp;
            if (cA[iKey] <= cB[iKey])
               return -1;
            return 1;
         });
      };
      mnm._formatDate = function(iDate, iYmd, iHms) {
         var aD = iDate.substring(iYmd === 'md' ? 5 : 0, 10);
         if (aD.charAt(0) === '0')
            aD = '\u2007' + aD.substr(1);
         if (!iHms)
            return aD;
         return aD +' '+ iDate.substring(11, iHms === 'hm' ? 16 : 19);
      };
   </script>
</head><body>
<base target="_blank">

<div id="app" uk-grid class="uk-grid-small">

<div class="uk-width-2-5">
   <div class="uk-clearfix">
      <span style="padding-left:0.5em; display:inline-block">
         {{ msgTitle }}
         <span v-show="msgSubjects.length > 1"
               class="dropdown-icon">&nbsp;&#x25BD;&nbsp;</span>
      </span>
      <mnm-subject v-if="msgSubjects.length > 1"
                   :list="msgSubjects"></mnm-subject>
      <div class="uk-float-right">
         <button @click="mnm.ThreadRecv()"
                 class="btn-icon"><span uk-icon="cloud-download"></span></button>
         <span uk-icon="copy" class="dropdown-icon">{{al.length || '&nbsp;&nbsp;'}}</span>
         <mnm-attach ref="al"></mnm-attach>
         &nbsp;
         <button @click="mnm.ThreadNew({alias:cf.Alias, cc:[]})"
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
                :set="msgTabset" :state="cs.ThreadTabs"></mnm-tabs>
      <input @keyup.enter="tabSearch($event.target.value, cs.ThreadTabs)"
             placeholder=" &#x2315;" type="text"
             class="uk-width-1-6 search-box">
   </div>
   <div uk-height-viewport="offset-top:true; offset-bottom:true"
        class="firefox-minheight-fix uk-overflow-auto">
      <ul id="msg-panel" class="uk-list uk-list-divider message-list">
         <li v-for="aMsg in ml" :key="aMsg.Id"
             :class="{'message-edit': aMsg.From === ''}" style="margin:0">
            <span @click="msgToggle(aMsg.Id)"
                  class="message-title" :class="{'message-title-edit': aMsg.From === ''}">
               {{ fmtD(aMsg.Date,'md','hm') }}
               <b>{{ aMsg.Alias || aMsg.From }}</b>
            </span>
            <template v-if="aMsg.Id in mo">
               <div v-if="!('msg_data' in mo[aMsg.Id])"
                    class="uk-text-center"><span uk-icon="future"><!-- todo hourglass --></span></div>
               <template v-else-if="aMsg.From === ''">
                  <button @click="mnm.ThreadSend(aMsg.Id)"
                          class="btn-icon btn-alignt"><span uk-icon="forward"></span></button>
                  <button @click="mnm.ThreadDiscard(aMsg.Id)"
                          class="btn-iconred btn-floatr"><span uk-icon="trash"></span></button>
                  <div @keypress="keyAction('pv_'+aMsg.Id, $event)">
                     <div style="position:relative; padding:1px;">
                        <input @keyup.enter="ccAdd(aMsg.Id, $event.target)"
                               placeholder="+To" size="25" type="text">
                        <div style="height:100%; position:absolute; left:13em; right:2em; top:0;">
                           <mnm-draftmenu :list="mo[aMsg.Id].SubHead.Cc"
                                          :msgid="aMsg.Id" :drop="ccDrop"></mnm-draftmenu>
                           <mnm-draftmenu :list="mo[aMsg.Id].SubHead.Attach"
                                          :msgid="aMsg.Id" :drop="atcDrop"
                                          :getname="atcGetName" :getkey="atcGetKey"
                                          :style="{float:'right'}"></mnm-draftmenu>
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
                                         :atchasff="atcHasFf" :msgid="aMsg.Id"></mnm-markdown></div>
                     </div>
                     <input @input="subjAdd(aMsg.Id, $event.target.value)"
                            :value="(toSave[aMsg.Id] || mo[aMsg.Id].SubHead).Subject"
                            placeholder="Subject" type="text" style="width:100%">
                     <mnm-textresize @input.native="textAdd(aMsg.Id, $event.target.value)"
                                     :src="(toSave[aMsg.Id] || mo[aMsg.Id]).msg_data"
                                     placeholder="Ctrl-J to Preview" style="width:100%"></mnm-textresize>
                  </div>
               </template>
               <template v-else>
                  <button @click="mnm.ThreadReply(getReplyTemplate(aMsg))"
                          class="btn-icon btn-floatr"><span uk-icon="comment"></span></button>
                  <div v-if="mo[aMsg.Id].SubHead.For.length !== 1
                          || mo[aMsg.Id].SubHead.For[0].Id !== cf.Uid">
                     Cc:
                     <span v-for="(aTo, aI) in mo[aMsg.Id].SubHead.For"
                           v-if="aTo.Id !== cf.Uid"
                           style="margin-right:1em">
                        {{ mo[aMsg.Id].SubHead.Cc[aI] }}</span>
                  </div>
                  <div v-if="aMsg.Subject">
                     Subject: {{ aMsg.Subject }}</div>
                  <div v-if="mo[aMsg.Id].SubHead.Attach">
                     Attached ({{ mo[aMsg.Id].SubHead.Attach.length }}):
                     <template v-for="aAtc in mo[aMsg.Id].SubHead.Attach">
                        <template v-if="aAtc.Name.charAt(0) === 'r'">
                           <span @click="tabSearch('ffn:'+ aAtc.Ffn +
                                              (aMsg.From === cf.Uid ? '_sent' : '_recv'), cs.SvcTabs)"
                                 class="uk-link">{{ aAtc.Name.substr(2) }}</span></template>
                        <template v-else>
                           <a :href="'?an=' + encodeURIComponent(aMsg.Id +'_'+ aAtc.Name)"
                              target="mnm_atc_[{.Title}]">{{ aAtc.Name.substr(2) }}</a></template>
                        &#x25CA;
                     </template>
                  </div>
                  <br>
                  <div v-if="!mo[aMsg.Id].msg_data">
                     <p><span uk-icon="comment"></span></p></div>
                  <mnm-markdown v-else
                                :src="mo[aMsg.Id].msg_data" :msgid="aMsg.Id"
                                :formreply="getReplyTemplate(aMsg)"></mnm-markdown>
               </template>
            </template>
         </li></ul>
      <br/><div id="log"></div>
   </div>
</div>

<div class="uk-width-1-2">
   <span v-for="aMsg in ml" :key="aMsg.Id"
         v-if="mo[aMsg.Id] && mo[aMsg.Id].Posted === 'draft'">
      <mnm-files @attach="atcAdd(aMsg.Id, arguments[0])"
                 :toggle="'#t'+aMsg.Id" pos="right-top"></mnm-files>
      <mnm-forms @attach="atcAdd(aMsg.Id, arguments[0])"
                 :toggle="'#f'+aMsg.Id" pos="right-top"></mnm-forms>
   </span>
   <div class="uk-clearfix">
      <span class="uk-text-large">
         <span uk-icon="world"></span>
         [{.Title}]
      </span>
      <div class="uk-float-right">
         <span @mousedown="ohiFrom = !ohiFrom" class="dropdown-icon">&nbsp;o/</span>
         <span uk-icon="users" class="dropdown-icon">&nbsp;</span>
         <mnm-adrsbk></mnm-adrsbk>
         &nbsp;
         <span uk-icon="push" class="dropdown-icon">&nbsp;</span>
         <mnm-files ref="t" pos="bottom-right"></mnm-files>
         <span uk-icon="file-edit" class="dropdown-icon">&nbsp;</span>
         <mnm-forms ref="f" pos="bottom-right"></mnm-forms>
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
             placeholder=" &#x2315;" type="text"
             class="uk-width-1-2 search-box">
   </div>
   <mnm-tabs v-if="cs.SvcTabs.Pinned.length || cs.SvcTabs.Terms.length"
             :set="svcTabset" :state="cs.SvcTabs"></mnm-tabs>
   <div class="uk-position-relative"><!-- context for ohi card -->
      <div uk-height-viewport="offset-top:true" class="firefox-minheight-fix uk-overflow-auto"
           :class="{'uk-background-muted':ffn}">
         <template v-if="ffn">
            <table class="uk-table uk-table-small uk-table-hover uk-text-small">
               <tr>
                  <th v-for="(a, aKey) in ffnCol"
                      v-if="aKey !== 'checksum' && aKey !== 'threadid'"
                      style="position:sticky; top:0" class="uk-background-muted">
                     {{ aKey === 'msgid' ? 'source' : aKey }}</th>
               </tr>
               <tr v-for="aRow in tl">
                  <td v-for="(a, aKey) in ffnCol"
                      v-if="aKey !== 'checksum' && aKey !== 'threadid'">
                     <a v-if="aKey === 'msgid'"
                        onclick="mnm.NavigateLink(this.href); return false"
                        :href="'#'+ aRow.threadid +'&'+ aRow.msgid"><span uk-icon="mail"></span></a>
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
               <div class="uk-width-auto" style="padding:0">{{ fmtD(aRow.Date,'md') }}</div>
               <div v-if="aRow.Id.indexOf('/') < 0"
                    class="uk-width-1-6">{{'Last Author'}}</div>
               <div class="uk-width-expand">
                  {{'Something'}} {{aRow.Id}}
               </div>
               <div class="uk-width-auto">{{ fmtD('2018-01-17T04:16:57Z') }}</div>
               <div v-if="aRow.Id.indexOf('/') < 0"
                    class="uk-width-1-6">{{'Orig Author'}}</div>
               <span v-if="aRow.Id.indexOf('/') >= 0"
                     @click="tabSearch('ffn:'+aRow.Id, cs.SvcTabs)"
                     style="padding:0"></span>
               <span v-else
                     @click="mnm.NavigateThread(aRow.Id)"
                     style="padding:0"></span>
            </div></template>
         <br/>{{JSON.stringify(mo)}}
      </div>
      <div v-if="ohiFrom"
           class="uk-card uk-card-secondary uk-text-small uk-border-rounded"
           style="padding:8px; position:absolute; bottom:10px; right:10px">
         <div v-if="of.length === 0"
              class="uk-text-warning">no o/</div>
         <ul v-else
             class="uk-list uk-text-success" style="margin-bottom:0">
            <li v-for="aUser in of">
               {{aUser.Uid}}
            </li></ul>
      </div>
   </div>
</div>

<div class="uk-width-expand uk-light service-panel">
   <div class="uk-text-right" style="margin:0 1em 1em 0">
      <span uk-icon="plus-circle" class="dropdown-icon">&nbsp;</span>
      <mnm-svcadd></mnm-svcadd>
      <span uk-icon="cog" class="dropdown-icon">&nbsp;</span>
      <div uk-dropdown="mode:click; offset:2; pos:bottom-right" class="uk-width-1-5">
          <div class="uk-text-right uk-text-small">SETTINGS</div>
      </div>
   </div>
   <div uk-height-viewport="offset-top:true" class="firefox-minheight-fix uk-overflow-auto">
      <ul class="uk-list uk-list-divider">
         <li v-for="aSvc in sl" :key="aSvc">
            <template v-if="aSvc === '[{.Title}]'">
               <span style="visibility:hidden">1</span
              ><span uk-icon="settings" class="dropdown-icon">&nbsp;</span>
               <mnm-svccfg></mnm-svccfg>
               {{aSvc}}
            </template>
            <template v-else>
               <span uk-icon="reply" class="dropdown-icon">0{{aSvc.todo}} </span>
               <div uk-dropdown="mode:click; offset:-4; pos:left-top" class="uk-width-1-5">
                  <div class="uk-text-right uk-text-small">UPDATES</div>
               </div>
               <a :href="'/'+aSvc" :target="'mnm_'+aSvc">{{aSvc}}</a>
            </template>
         </li></ul>
   </div>
</div>

</div>

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
      props: ['list'],
      computed: { mnm: function() { return mnm } },
   });
</script>

<script type="text/x-template" id="mnm-attach">
   <div uk-dropdown="mode:click; offset:2" class="uk-width-1-3 dropdown-scroll">
      <ul uk-tab>
         <!-- todo Date -->
         <li v-for="aKey in ['Size','Name']"
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
            2018-01-17T04:16:57Z{{aFile.Date}} &nbsp;
            <button @click=""
                    class="btn-icon"><span uk-icon="push"></span></button>
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

<style>
.draft-menu {
   min-width: 4em;
   float: left;
   box-sizing: border-box;
   padding: 0 0.5em;
   margin-right: 1em;
   cursor: default;
   background: lightgray;
   font-family: Monaco, monospace;
   color: black;
}
.draft-shown {
   padding: 1px calc(0.5em + 1px);
   min-height: 100%;
   box-shadow: 0 5px 12px rgba(0,0,0,.15);
   filter: brightness(105%);
}
.draft-hiddn {
   border: 1px solid dimgray;
   height: 100%;
   overflow: hidden;
}
.menu-x {
   float: right; margin-left:  0.50em;
   font-family: initial;
}
.menu-v {
   float: right; margin-left: -0.75em;
   font-size: 80%;
   position: relative; bottom: -4px;
}
.draft-hiddn .menu-x { visibility: hidden; }
.draft-shown .menu-v { display: none; }
</style>

<script type="text/x-template" id="mnm-draftmenu">
   <div v-show="list && list.length > 0"
        @click="draftMenu" class="draft-hiddn draft-menu">
      <span v-for="(aEl, aI) in list" :key="getkey ? getkey(aEl) : aEl">
         {{getname ? getname(aEl) : aEl}}
         <div v-if="aI === 0 && list.length > 1"
              class="menu-v">&#x25BD;</div>
         <div @click="drop(msgid, aI)" class="menu-x">&times;</div>
         <br>
      </span>
   </div>
</script><script>
   Vue.component('mnm-draftmenu', {
      template: '#mnm-draftmenu',
      props: ['msgid', 'list', 'drop', 'getname', 'getkey'],
      watch: {
         list: function() {
            // show menu if changed by any client
            this.draftMenu(null);
         },
      },
      methods: {
         draftMenu: function(iE) {
            if (this.$el.className === 'draft-hiddn draft-menu') {
               this.$el.className = 'draft-shown draft-menu';
               var aMenu = this.$el;
               document.addEventListener('click',
                  function() { aMenu.className = 'draft-hiddn draft-menu' }, {once:true});
            }
            if (iE)
               iE.stopPropagation();
         },
      },
   });
</script>

<style>
.text-resize {
   box-sizing: border-box;
   resize: none;
   overflow-y: hidden;
}
</style>

<script type="text/x-template" id="mnm-textresize">
   <textarea @input="resize" @click.stop :value="src" class="text-resize"></textarea>
</script><script>
   Vue.component('mnm-textresize', {
      template: '#mnm-textresize',
      props: ['src'],
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
      props: ['src', 'msgid', 'formfill', 'formreply', 'atchasff'],
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
                 :disabled="!parent.formreply"
                 class="btn-icon btn-floatr"><span uk-icon="commenting"></span></button>
      </div>
      <plugin-vfg @model-updated="onInput"
                  :schema="formDef" :model="formState" :options="{}"></plugin-vfg>
   </div>
</script><script>
   Vue.component('mnm-formview', {
      template: '#mnm-formview',
      props: ['file', 'fillMap', 'parent'],
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
            {{aFile.Date}}
            <button v-if="toggle"
                    @click="$emit('attach', 'upload/'+aFile.Name)"
                    class="btn-icon"><span uk-icon="copy"></span></button>
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
                  <button class="btn-iconred"><span uk-icon="trash"></span></button>
               </form>
            </div>
         </li></ul>
   </div>
</script><script>
   Vue.component('mnm-files', {
      template: '#mnm-files',
      props: ['toggle'],
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
         <div class="uk-float-right uk-text-small">ATTACHABLE FORMS</div>
         <input type="hidden" name="filename" value='{}'>
         <input v-model="upname" type="text" size="40" placeholder="New Type">
         <button @click="upname = ''"
                 :disabled="!validName(upname.split('.'))"
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
               {{aFile.Date}}
               <button v-if="toggle"
                       @click="$emit('attach', 'form/'+aSet.Name+'.'+aFile.Id)"
                       class="btn-icon"><span uk-icon="copy"></span></button>
               <a @click.stop.prevent="revOpen(aSet.Name,aFile.Id,$event.target)"
                  :id="'bf_'+aSet.Name+'.'+aFile.Id" href="#">
                  <span uk-icon="triangle-left">&nbsp;</span>{{aSet.Name}}.{{aFile.Id}}</a>
               <form v-if="!toggle"
                     :action="'/f/-' + encodeURIComponent(aSet.Name+'.'+aFile.Id)" method="POST"
                     onsubmit="mnm.Upload(this); return false;"
                     style="float:right">
                  <button class="btn-iconred"><span uk-icon="trash"></span></button>
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
                       class="btn-icon"><span uk-icon="file-edit"></span></button>
               <div style="font-size:smaller; text-align:right">&nbsp;{{parseError}}</div>
               <div class="pane-slider" :class="{'pane-slider-rhs':codeShow}">
                  <div class="pane-scroller" style="min-height:1px">
                     <plugin-vfg :schema="formDef" :model="{}" :options="{}"></plugin-vfg></div>
                  <div class="pane-scroller">
                     <mnm-textresize @input.native="mnm._data.fo=$event.target.value"
                                     :src="mnm._data.fo"
                                     name="filename" style="width:100%"></mnm-textresize></div>
               </div>
            </form>
            <form :action="'/f/*' + encodeURIComponent(setName+'.'+fileId) +
                              '+' + encodeURIComponent(dupname)" method="POST"
                  onsubmit="mnm.Upload(this); return false;">
               <input v-model="dupname" type="text" size="40" placeholder="New Revision">
               <button @click="dupShow = dupname"
                       :disabled="!validName([].concat(setName,dupname.split('.')))"
                       class="btn-icon"><span uk-icon="copy"></span></button>
            </form>
         </div>
      </div>
   </div>
</script><script>
   Vue.component('mnm-forms', {
      template: '#mnm-forms',
      props: ['toggle'],
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
   <span v-if="response">
      <a v-if="response.Tid"
         onclick="mnm.NavigateLink(this.href); return false"
         :href="'#'+ response.Tid +'&'+ response.MsgId"><span uk-icon="mail"></span></a>
      <template v-else>
         ping</template>
      {{fmtD(response.Date)}}
   </span>
</script><script>
   Vue.component('mnm-pingresponse', {
      template: '#mnm-pingresponse',
      props: ['response'],
      methods: { fmtD: mnm._formatDate }
   });
</script>

<script type="text/x-template" id="mnm-adrsbk">
   <div uk-dropdown="mode:click; offset:2; pos:bottom-right" class="uk-width-2-5 dropdown-scroll">
      <ul uk-tab style="margin-top:0">
         <li v-for="aName in ['pings','invites','drafts','pinged','invited','groups','ohi to']">
            <a @click.prevent="" href="#" style="cursor:default">{{aName}}</a>
         </li></ul>
      <ul class="uk-switcher dropdown-scroll-list">
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>From</th><th>Message</th><th>Response</th></tr>
               <tr v-for="a in mnm._data.pf">
                  <td>{{fmtD(a.Date)}}</td><td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"></mnm-pingresponse></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>Group</th><th>From</th><th>Msg</th><th>Response</th></tr>
               <tr v-for="a in mnm._data.if">
                  <td>{{fmtD(a.Date)}}</td>
                  <td>{{a.Gid}}
                     <span v-if="mnm._data.gl.find(function(c){return c.Gid === a.Gid})"
                           class="uk-badge">in</span>
                     <button v-else
                             @click="mnm.InviteAccept(a.Gid)"
                             class="btn-icon"><span uk-icon="forward"></span></button>
                  </td>
                  <td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"></mnm-pingresponse></td>
               </tr></table></li>
         <li>
            <form onsubmit="return false"
                  style="text-align:center">
               <input v-model="draft.to" placeholder="To" size="25" type="text">
               <input v-model="draft.gid" placeholder="(Group)" size="25" type="text">
               <button @click="startPing()"
                       :disabled="!validDraft"
                       class="btn-icon"><span uk-icon="pencil"></span></button>
            </form>
            <table class="uk-table uk-table-small">
               <tr><th>To / (Group)</th><th></th><th>Message</th><th></th></tr>
               <tr v-for="a in mnm._data.ps" :key="rowId(a)">
                  <td>{{a.Alias}}<br>{{a.Gid && '('+a.Gid+')'}}</td>
                  <td><button @click="mnm.PingSend({to:a.Alias, gid:a.Gid})"
                              class="btn-icon"><span uk-icon="forward"></span></button></td>
                  <td><textarea cols="40" rows="3" maxlength="120"
                                @input="timer(a, $event.target.value)"
                     >{{toSave[rowId(a)] || a.Text}}</textarea></td>
                  <td><button @click="mnm.PingDiscard({to:a.Alias, gid:a.Gid})"
                              class="btn-iconred"><span uk-icon="trash"></span></button></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>To</th><th>Message</th><th>Response</th></tr>
               <tr v-for="a in mnm._data.pt">
                  <td>{{fmtD(a.Date)}}</td><td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"></mnm-pingresponse></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>Group</th><th>To</th><th>Message</th><th>Response</th></tr>
               <tr v-for="a in mnm._data.it">
                  <td>{{fmtD(a.Date)}}</td><td>{{a.Gid}}</td><td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"></mnm-pingresponse></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>Group</th></tr>
               <tr v-for="a in mnm._data.gl">
                  <td>{{fmtD(a.Date)}}</td>
                  <td>{{a.Gid}}
                     <span v-if="a.Admin"
                           class="uk-badge">A</span></td>
               </tr></table></li>
         <li>
            <form onsubmit="this.reset(); return false;"
                  style="text-align:center">
               <input oninput="this.nextElementSibling.disabled = !this.value"
                      placeholder="Add someone" size="40" type="text" name="resets">
               <button onclick="mnm.OhiAdd(this.previousElementSibling.value)"
                       disabled
                       class="btn-icontxt">o/</button>
            </form>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>To</th><th></th></tr>
               <tr v-for="a in mnm._data.ot">
                  <td>{{fmtD(a.Date)}}</td><td>{{a.Uid /*todo alias*/}}</td>
                  <td><button @click="mnm.OhiDrop(null,a.Uid)"
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
         fmtD: mnm._formatDate,
         rowId: function(iRec) { return iRec.Alias +'\0'+ (iRec.Gid || '') },
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
        @hidden="error = ''">
      <div class="uk-text-right uk-text-small">ADD ACCOUNT</div>
      <input @keyup.enter="send()" v-model="addr"
             placeholder="Net Address" size="33" type="text">
      <input @keyup.enter="send()" v-model="name"
             placeholder="Title" size="33" type="text">
      <input @keyup.enter="send()" v-model="alias"
             placeholder="Alias" size="33" type="text">
      <input @keyup.enter="send()" v-model.number="period"
             placeholder="(Login Frequency)" size="24" type="text">
      &nbsp; seconds<br>
      <span class="uk-text-danger">{{error}}</span> &nbsp;
      <button @click="send()">Register</button>
   </div>
</script><script>
   Vue.component('mnm-svcadd', {
      template: '#mnm-svcadd',
      data: function() { return {addr:'', name:'', alias:'', period:null, error:''} },
      methods: {
         send: function() {
            if (!this.addr || !this.name || !this.alias) {
               this.error = 'missing input';
               return;
            }
            mnm.SvcAdd({addr:this.addr, name:this.name,
                        alias:this.alias, loginperiod:this.period});
            this.addr = this.name = this.alias = this.error = '';
            this.period = null;
         }
      }
   });
</script>

<style>.svccfg { word-break: break-all }</style>

<script type="text/x-template" id="mnm-svccfg">
   <div uk-dropdown="mode:click; offset:-4; pos:left-top" class="uk-width-1-5">
      <div class="uk-text-right uk-text-small">SETTINGS</div>
      <table class="uk-table uk-table-small" style="margin:0">
         <tr><td>Net Address</td><td class="svccfg">{{mnm._data.cf.Addr }}</td></tr>
         <tr><td>Title      </td><td class="svccfg">{{mnm._data.cf.Name }}</td></tr>
         <tr><td>Alias      </td><td class="svccfg">{{mnm._data.cf.Alias}}</td></tr>
         <tr><td>Uid        </td><td class="svccfg">{{mnm._data.cf.Uid  }}</td></tr>
      </table>
   </div>
</script><script>
   Vue.component('mnm-svccfg', {
      template: '#mnm-svccfg',
      computed: { mnm: function() { return mnm } },
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
      props: ['set', 'state'],
      computed: { mnm: function() { return mnm } }
   });
</script>

<script>
;(function() {
   var sChange = 0;
   var sTemp = {ml:null, mo:null};

   var sApp = new Vue({
      el: '#app',
      data: mnm._data,
      methods: {
         fmtD: mnm._formatDate,
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
            var aObj = {alias: mnm._data.cf.Alias, cc: null, data: '',
                        subject: iIdxEl === mnm._data.ml[mnm._data.ml.length-1] ? '' : iIdxEl.Subject};
            var aMo = mnm._data.mo[iIdxEl.Id];
            if (aMo.From === mnm._data.cf.Uid) {
               aObj.cc = aMo.SubHead.Cc;
            } else {
               var aN = aMo.SubHead.For.findIndex(function(c){ return c.Id === mnm._data.cf.Uid });
               aObj.cc = aMo.SubHead.Cc.slice(0, aN).concat(aMo.SubHead.Cc.slice(aN+1),
                                                            aMo.SubHead.Alias);
            }
            return aObj;
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
               Alias:               iMo.SubHead.Alias,
               Cc:       iCc     || iMo.SubHead.Cc,
               Attach:   iAttach || iMo.SubHead.Attach,
               FormFill: iToSave.form_fill,
               Data:     iToSave.msg_data,
               Subject:  iToSave.Subject,
            });
            iToSave.suUpdt = iToSave.mdUpdt = iToSave.ffUpdt = false;
         },
         ccAdd: function(iId, iWidget) {
            if (iWidget.value.length === 0)
               return;
            var aCc = mnm._data.mo[iId].SubHead.Cc;
            aCc = aCc ? aCc.slice() : [];
            aCc.unshift(iWidget.value);
            var aPrev = aCc.lastIndexOf(iWidget.value);
            if (aPrev !== 0)
               aCc.splice(aPrev, 1);
            iWidget.value = '';
            this.draft_save(iId, aCc, null);
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
         ccDrop:  function(iId, iN) {
            var aCc = mnm._data.mo[iId].SubHead.Cc;
            this.draft_save(iId, aCc.slice(0, iN).concat(aCc.slice(iN+1)), null);
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
      case 'cs': case 'sl': case 'cf': case 'al': case 'ml':
      case 'ps': case 'pt': case 'pf': case 'it': case 'if': case 'gl': case 'ot': case 'of':
      case 't': case 'f':
         if (i === 'f' && iEtc) {
            mnm._data.fo = iData;
         } else {
            mnm._data[i] = JSON.parse(iData);
            if (i === 'al' || i === 't' || i === 'f')
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
         // avoid opening Cc & Attach menus if not changed
         var aOrig = mnm._data.mo[iEtc.Id] && mnm._data.mo[iEtc.Id].SubHead;
         if (aOrig) {
            if (!fDiff('Cc'    )) iEtc.SubHead.Cc     = aOrig.Cc;
            if (!fDiff('Attach')) iEtc.SubHead.Attach = aOrig.Attach;
         }
         Vue.set(mnm._data.mo, iEtc.Id, iEtc); //todo set ml Date
         function fDiff(c) {
            if ( aOrig[c]         === iEtc.SubHead[c])         return false;
            if (!aOrig[c]         || !iEtc.SubHead[c])         return true;
            if ( aOrig[c].length  !== iEtc.SubHead[c].length)  return true;
            if ( aOrig[c].length  === 0)                       return false;
            if (c === 'Cc'
               ? aOrig[c][0]      === iEtc.SubHead[c][0]
               : aOrig[c][0].Name === iEtc.SubHead[c][0].Name) return false; //todo full comparison?
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

   window.onload = mnm.Connect;

}).call(this);
</script>

</body></html>

