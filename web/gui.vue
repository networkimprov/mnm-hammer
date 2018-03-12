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
      mnm._formatDate = function(iDt) { return iDt.substr(0, 10) };
   </script>

   <style>
      .firefox-minheight-fix {height:0}
      input[type=text] { font-family: Monaco, monospace; font-size:larger }
   </style>
</head><body>
<base target="_blank">

<div id="app" uk-grid class="uk-grid-small">

<div class="uk-width-2-5">
   <div class="uk-clearfix">
      Subject, orig Author &amp; Date
      <div class="uk-float-right">
         <button @click="mnm.ThreadRecv()" style="padding:0"><span uk-icon="cloud-download"></span></button>
         <span uk-icon="copy" style="cursor:default">{{al.length || ''}}</span>
         <mnm-attach list="al" :data="al" ref="al"></mnm-attach>
         &nbsp;
         <button @click="mnm.ThreadNew({alias:cf.Alias, cc:[]})"
                 style="padding:0"><span uk-icon="pencil"></span></button>
         <button :disabled="!cs.History || !cs.History.Prev" onclick="this.blur(); mnm.History(-1)"
                 class="uk-button uk-button-link">
            <span uk-icon="icon:arrow-left; ratio:1.6"></span></button>
         <button :disabled="!cs.History || !cs.History.Next" onclick="this.blur(); mnm.History( 1)"
                 class="uk-button uk-button-link">
            <span uk-icon="icon:arrow-right; ratio:1.6"></span></button>
      </div>
   </div>
   <div uk-grid class="uk-grid-collapse">
      <mnm-tabs class="uk-width-expand"
                :set="msgTabset" :state="cs.ThreadTabs"></mnm-tabs>
      <input class="uk-width-1-6" type="text" placeholder=" &#x2315;"
             @keyup.enter="tabSearch($event.target.value, cs.ThreadTabs)">
   </div>
   <div uk-height-viewport="offset-top:true; offset-bottom:true"
        class="firefox-minheight-fix uk-overflow-auto">
      <ul id="msg-panel" class="uk-list uk-list-divider" style="background:#FFF7CF">
         <li v-for="aMsg in ml" :key="aMsg.Id"
             style="margin:0">
            <div @click="msgToggle(aMsg.Id)" class="uk-link-text"
                 style="cursor:pointer; display:inline-block; line-height:2em;">
               {{ aMsg.Date }} <b>{{ aMsg.From }}</b></div>
            <template v-if="aMsg.Id in mo">
               <template v-if="mo[aMsg.Id].Posted === 'draft'">
                  <button @click="mnm.ThreadSend(aMsg.Id)"
                          style="padding:0; display:inline-block; vertical-align:top;">
                     <span uk-icon="forward"></span></button>
                  <span></span>
                  <button @click="mnm.ThreadDiscard(aMsg.Id)"
                          style="color:crimson; padding:0; display:inline-block; float:right;">
                     <span uk-icon="trash"></span></button>
               </template>
               <button v-else
                       @click="mnm.ThreadReply({alias:cf.Alias, cc:[]})"
                       style="padding:0; display:inline-block; float:right;">
                  <span uk-icon="reply"></span></button>
               <div v-if="!('msg_data' in mo[aMsg.Id])"
                    class="uk-text-center"><span uk-icon="future"><!-- todo hourglass --></span></div>
               <div v-else-if="mo[aMsg.Id].Posted === 'draft'">
                  <div style="position:relative; padding:1px;">
                     <input @keyup.enter="ccAdd(aMsg.Id, $event.target)" type="text" placeholder="+To">
                     <div style="height:100%; position:absolute; left:15em; right:2em; top:0;">
                        <mnm-draftmenu :list="(toSave[aMsg.Id] || mo[aMsg.Id].SubHead).Cc"
                                       :msgid="aMsg.Id" :drop="ccDrop"></mnm-draftmenu>
                        <mnm-draftmenu :list="(toSave[aMsg.Id] || mo[aMsg.Id].SubHead).Attach"
                                       :msgid="aMsg.Id" :drop="atcDrop"
                                       :getname="atcGetName" :style="{float:'right'}"></mnm-draftmenu>
                     </div>
                  </div>
                  <div style="float:right; margin-top:-1.7em;">
                     <span uk-icon="push"      :id="'t'+aMsg.Id"></span
                    ><span uk-icon="file-edit" :id="'f'+aMsg.Id"></span>
                     <span :id="'pv_'+aMsg.Id"></span>
                     <div uk-dropdown="mode:click; pos:right-top" class="uk-width-2-5"
                          style="overflow:auto; max-height:75vh;
                                 border-top:1em solid white; border-bottom:1em solid white;">
                        <mnm-markdown :src="aMsg.Id in toSave ? toSave[aMsg.Id].Data
                                                              : mo[aMsg.Id].msg_data"
                                      :msgid="aMsg.Id.substr(-12)"></mnm-markdown></div>
                  </div>
                  <mnm-textresize @input.native="textAdd(aMsg.Id, $event.target.value)"
                                  @keypress.native="keyAction('pv_'+aMsg.Id, $event)"
                                  :src="aMsg.Id in toSave ? toSave[aMsg.Id].Data : mo[aMsg.Id].msg_data"
                                  placeholder="Ctrl-J to Preview" style="width:100%"></mnm-textresize>
               </div>
               <div v-else-if="!mo[aMsg.Id].msg_data"
                    class="uk-text-center"><span uk-icon="comment"></span></div>
               <mnm-markdown v-else
                             :src="mo[aMsg.Id].msg_data" :msgid="aMsg.Id"></mnm-markdown>
            </template>
         </li></ul>
      <br/><div id="log"></div>
   </div>
</div>

<div class="uk-width-1-2">
   <span v-for="aMsg in ml" :key="aMsg.Id"
         v-if="mo[aMsg.Id] && mo[aMsg.Id].Posted === 'draft'">
      <mnm-files @attach="atcAdd(aMsg.Id, arguments[0])"
                 list="t" :data="t" :toggle="'#t'+aMsg.Id" pos="right-top"></mnm-files>
      <mnm-forms @attach="atcAdd(aMsg.Id, arguments[0])"
                 list="f" :data="f" :toggle="'#f'+aMsg.Id" pos="right-top"></mnm-forms>
   </span>
   <div class="uk-clearfix">
      <span class="uk-text-large">
         <span uk-icon="world"></span>
         [{.Title}]
      </span>
      <div class="uk-float-right">
         <span @mousedown="ohiFrom = !ohiFrom" style="cursor:default">&nbsp;o/</span>
         <span uk-icon="users" style="cursor:default">&nbsp;</span>
         <mnm-adrsbk></mnm-adrsbk>
         &nbsp;
         <span uk-icon="push" style="cursor:default">&nbsp;</span>
         <mnm-files list="t" :data="t" ref="t" pos="bottom-right"></mnm-files>
         <span uk-icon="file-edit" style="cursor:default">&nbsp;</span>
         <mnm-forms list="f" :data="f" ref="f" pos="bottom-right"></mnm-forms>
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
      <input class="uk-width-1-2" type="text" placeholder=" &#x2315;"
             @keyup.enter="tabSearch($event.target.value, cs.SvcTabs)">
   </div>
   <mnm-tabs v-if="cs.SvcTabs.Pinned.length || cs.SvcTabs.Terms.length"
             :set="svcTabset" :state="cs.SvcTabs"></mnm-tabs>
   <div class="uk-position-relative"><!-- context for ohi card -->
      <div uk-height-viewport="offset-top:true" class="firefox-minheight-fix uk-overflow-auto"
           :class="{'uk-background-muted':ffn}">
         <template v-if="ffn">
            <div style="position:sticky; top:0; padding:1em"
                 class="uk-background-muted">{{ffn}}</div>
            <table class="uk-table uk-table-small uk-table-hover uk-text-small">
               <tr>
                  <th v-for="(a, aKey) in ffnCol"
                      style="position:sticky; top:0">{{aKey}}</th>
               </tr>
               <tr v-for="aRow in tl">
                  <td v-for="(a, aKey) in ffnCol">
                     <table v-if="aRow[aKey] instanceof Object"
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
               </tr></table>
         </template>
         <template v-else>
            <div v-for="aRow in tl">
               <a v-if="aRow.Id.indexOf('/') >= 0"
                  @click.prevent="tabSearch('ffn:'+aRow.Id, cs.SvcTabs)" href="#">{{aRow.Id}}</a>
               <a v-else
                  @click.prevent="mnm.ThreadGo(aRow.Id)" href="#">{{aRow.Id}}</a>
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

<div class="uk-width-expand uk-light" style="background:#003333">
   <div class="uk-text-right" style="margin:0 1em 1em 0">
      <span uk-icon="plus-circle" style="cursor:default">&nbsp;</span>
      <mnm-svcadd></mnm-svcadd>
      <span uk-icon="cog" style="cursor:default">&nbsp;</span>
      <div uk-dropdown="mode:click; offset:2; pos:bottom-right" class="uk-width-1-5">
          <div class="uk-text-right uk-text-small">SETTINGS</div>
      </div>
   </div>
   <div uk-height-viewport="offset-top:true" class="firefox-minheight-fix uk-overflow-auto">
      <ul class="uk-list uk-list-divider">
         <li v-for="aSvc in sl" :key="aSvc">
            <template v-if="aSvc === '[{.Title}]'">
               <span style="visibility:hidden">1</span
              ><span uk-icon="settings" style="cursor:default">&nbsp;</span>
               <mnm-svccfg></mnm-svccfg>
               {{aSvc}}
            </template>
            <template v-else>
               <span uk-icon="commenting" style="cursor:default">0{{aSvc.todo}} </span>
               <div uk-dropdown="mode:click; offset:-4; pos:left-top" class="uk-width-1-5">
                  <div class="uk-text-right uk-text-small">UPDATES</div>
               </div>
               <a :href="'/'+aSvc" :target="'mnm_'+aSvc">{{aSvc}}</a>
            </template>
         </li></ul>
   </div>
</div>

</div>

<script type="text/x-template" id="mnm-attach">
   <div uk-dropdown="mode:click; offset:2" class="uk-width-1-3">
      <ul uk-tab>
         <!-- todo Date -->
         <li v-for="aKey in ['Size','Name']"
             :class="{'uk-active': aKey === sort}">
            <a @click.prevent="listSort(aKey)" href="#">{{aKey}}</a>
         </li></ul>
      <ul class="uk-list uk-list-divider">
         <li v-for="aFile in data" :key="aFile.File">
            <a @click.prevent="" href="#"><span uk-icon="mail"></span></a>
            2018-01-17T04:16:57Z{{aFile.Date}} &nbsp;
            <button @click="" style="padding:0"><span uk-icon="push"></span></button>
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
      props: ['list', 'data'],
      computed: { sort: function() { return mnm._data.sort[this.list] } },
      methods: { listSort: function(i) { return mnm._listSort(this.list, i) } },
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
.menu-x { float: right; margin-left:  0.5em ; font-family: initial; }
.menu-v { float: right; margin-left: -0.75em; }
.draft-hiddn .menu-x { visibility: hidden; }
.draft-shown .menu-v { display: none; }
</style>

<script type="text/x-template" id="mnm-draftmenu">
   <div v-show="list && list.length > 0"
        @click="draftMenu" class="draft-hiddn draft-menu">
      <span v-for="(aEl, aI) in list" :key="getname ? getname(aEl) : aEl">
         {{getname ? getname(aEl) : aEl}}
         <div v-if="aI === 0 && list.length > 1"
              class="menu-v">&#x25BC;</div>
         <div @click="drop(msgid, aI)" class="menu-x">&times;</div>
         <br>
      </span>
   </div>
</script><script>
   Vue.component('mnm-draftmenu', {
      template: '#mnm-draftmenu',
      props: ['msgid', 'list', 'drop', 'getname'],
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
   <div class="message" v-html="mdi.render(src, $data)"></div>
</script><script>
   Vue.component('mnm-markdown', {
      template: '#mnm-markdown',
      props: ['src', 'msgid'],
      data: function() {
         return { msgId:this.msgid, formview:null }; // formview fields not reactive
      },
      computed: { mdi: function() { return mnm._mdi } },
      mounted:       function() { if (this.formview) this.formview.remount() },
      updated:       function() { if (this.formview) this.formview.remount() },
      beforeDestroy: function() { if (this.formview) this.formview.destroy() },
   });
</script>

<script type="text/x-template" id="mnm-formview">
   <div>
      <plugin-vfg :schema="formDef" :model="{}" :options="{}"></plugin-vfg>
   </div>
</script><script>
   Vue.component('mnm-formview', {
      template: '#mnm-formview',
      props: ['file'],
      data: function() { return {} },
      computed: {
         formDef: function() {
            if (!this.file)
               try { return JSON.parse(mnm._data.fo) } catch(a) { return {} }
            try {
               return this.file in mnm._data.ao ? JSON.parse(mnm._data.ao[this.file]) : {};
            } catch(a) {
               return {fields:[ {type:"label",label:"file not found or invalid"} ]};
            }
         },
         mnm: function() { return mnm },
      },
      components: { 'plugin-vfg': VueFormGenerator.component },
   });

   mnm._FormViews = function() {
      this.comp = {};
   };
   mnm._FormViews.prototype.make = function(iKey) {
      if (iKey in this.comp)
         return;
      mnm.AttachOpen(iKey);
      this.comp[iKey] =
         [ new (Vue.component('mnm-formview'))({ propsData: { file: iKey } }), null ];
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
   <div uk-dropdown="mode:click; offset:2" :toggle="toggle" class="uk-width-1-3">
      <form :action="'/'+list+'/+' + encodeURIComponent(upname)"
            method="POST" enctype="multipart/form-data"
            onsubmit="mnm.Upload(this); this.reset(); return false;">
         <div class="uk-float-right uk-text-small">ATTACHABLE FILES</div>
         <input @input="vis = !!(upname = $event.target.value.substr(12))" type="file"
                name="filename" required>
         <div :style="{visibility: vis ? 'visible' : 'hidden'}" style="margin-top:0.5em">
            <input v-model="upname" type="text" size="32" placeholder="Alt Name">
            <button @click="vis = false" type="submit" :disabled="!upname" style="padding:0">
               <span uk-icon="push"></span></button>
            <button @click="vis = false" type="reset"
                    style="padding:0 4px; background:none; border:none;">&times;</button>
         </div>
      </form>
      <ul uk-tab style="margin-top:0">
         <li v-for="aKey in ['Date','Name']"
             :class="{'uk-active': aKey === sort}">
            <a @click.prevent="listSort(aKey)" href="#">{{aKey}}</a>
         </li></ul>
      <ul class="uk-list uk-list-divider uk-overflow-auto" style="max-height:75vh; margin:0">
         <li v-for="aFile in data" :key="aFile.Name">
            {{aFile.Date}}
            <button v-if="toggle"
                    @click="$emit('attach', 'upload/'+aFile.Name)" style="padding:0">
               <span uk-icon="copy"></span></button>
            <a :href="'/'+list.charAt(0)+'d/' + encodeURIComponent(aFile.Name)">
               <span uk-icon="download">&nbsp;</span></a>
            <a :href="'/'+list+'/' + encodeURIComponent(aFile.Name)" target="mnm_atc_[{.Title}]">
               {{aFile.Name}}</a>
            <div class="uk-float-right">
               {{aFile.Size}}
               <form v-if="!toggle"
                     :action="'/'+list+'/-' + encodeURIComponent(aFile.Name)" method="POST"
                     onsubmit="mnm.Upload(this); return false;" style="display:inline!important">
                  <button style="padding:0">
                     <span uk-icon="trash" style="color:crimson"></span></button></form>
            </div>
         </li></ul>
   </div>
</script><script>
   Vue.component('mnm-files', {
      template: '#mnm-files',
      props: ['list', 'data', 'toggle'],
      data: function() { return {upname:'', vis:false} },
      computed: { sort: function() { return mnm._data.sort[this.list] } },
      methods: { listSort: function(i) { return mnm._listSort(this.list, i) } },
   });
</script>

<script type="text/x-template" id="mnm-forms">
   <div uk-dropdown="mode:click; offset:2" :toggle="toggle" class="uk-width-1-3"
        @hidden="revClose" @click="revClose">
      <form :action="'/'+list+'/+' + encodeURIComponent(upname)"
            method="POST" enctype="multipart/form-data"
            onsubmit="mnm.Upload(this); this.reset(); return false;">
         <div class="uk-float-right uk-text-small">ATTACHABLE FORMS</div>
         <input type="hidden" name="filename" value='{}'>
         <input v-model="upname" type="text" size="24" placeholder="New Type">
         <button @click="upname = ''" :disabled="!validName(upname.split('.'))" style="padding:0">
            <span uk-icon="pencil"></span></button>
      </form>
      <ul uk-tab style="margin-top:0">
         <li v-for="aKey in ['Date','Name']"
             :class="{'uk-active': aKey === sort}">
            <a @click.prevent="listSort(aKey)" href="#">{{aKey}}</a>
         </li></ul>
      <div style="position:relative"><!--context for rev card-->
         <ul class="uk-list uk-list-divider uk-overflow-auto" style="max-height:75vh; margin:0">
            <template v-for="aSet in data">
            <li v-for="aFile in aSet.Revs" :key="aSet.Name+'.'+aFile.Id">
               {{aFile.Date}}
               <button v-if="toggle"
                       @click="$emit('attach', 'form/'+aSet.Name+'.'+aFile.Id)" style="padding:0">
                  <span uk-icon="copy"></span></button>
               <a @click.stop.prevent="revOpen(aSet.Name,aFile.Id,$event.target)"
                  :id="'bf_'+aSet.Name+'.'+aFile.Id" href="#">
                  <span uk-icon="triangle-left">&nbsp;</span>{{aSet.Name}}.{{aFile.Id}}</a>
               <form v-if="!toggle"
                     :action="'/'+list+'/-' + encodeURIComponent(aSet.Name+'.'+aFile.Id)" method="POST"
                     onsubmit="mnm.Upload(this); return false;" style="float:right">
                  <button style="padding:0">
                     <span uk-icon="trash" style="color:crimson"></span></button></form>
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
                  :action="'/'+list+'/+' + encodeURIComponent(setName+'.'+fileId)"
                  method="POST" enctype="multipart/form-data"
                  onsubmit="mnm.Upload(this); return false;"
                  style="margin-top:-1.5em" class="pane-clip">
               <span @click="codeShow = !codeShow" style="cursor:default">{...}</span>
               <button :disabled="!!parseError" style="padding:0">
                  <span uk-icon="file-edit"></span></button>
               <div style="font-size:smaller; text-align:right">&nbsp;{{parseError}}</div>
               <div class="pane-slider" :class="{'pane-slider-rhs':codeShow}">
                  <div class="pane-scroller">
                     <mnm-formview style="min-height:1px"></mnm-formview></div>
                  <div class="pane-scroller">
                     <mnm-textresize @input.native="mnm._data.fo=$event.target.value"
                                     :src="mnm._data.fo"
                                     name="filename" style="width:100%"></mnm-textresize></div>
               </div>
            </form>
            <form :action="'/'+list+'/*' + encodeURIComponent(setName+'.'+fileId) +
                                     '+' + encodeURIComponent(dupname)" method="POST"
                  onsubmit="mnm.Upload(this); return false;">
               <input v-model="dupname" type="text" size="32" placeholder="New Revision">
               <button @click="dupShow = dupname"
                       :disabled="!validName([].concat(setName,dupname.split('.')))"
                       style="padding:0">
                  <span uk-icon="copy"></span></button>
            </form>
         </div>
      </div>
   </div>
</script><script>
   Vue.component('mnm-forms', {
      template: '#mnm-forms',
      props: ['list', 'data', 'toggle'],
      data: function() {
         return {upname:'', dupname:'', setName:'', fileId:'', revPos:'', codeShow:false, dupShow:''};
      },
      computed: {
         sort: function() { return mnm._data.sort[this.list] },
         mnm: function() { return mnm },
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
            for (var aF=0; aF < this.data.length; ++aF) {
               if (this.data[aF].Name === iPair[0]) {
                  for (var aR=0; aR < this.data[aF].Revs.length; ++aR)
                     if (this.data[aF].Revs[aR].Id === iPair[1])
                        return false;
                  return true;
               }
            }
            return true;
         },
         listSort: function(i) {
            mnm._data.sort[this.list] = i;
            mnm._data[this.list].sort(function(cA, cB) {
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
   });
</script>

<script type="text/x-template" id="mnm-pingresponse">
   <span v-if="response">
      <a v-if="response.MsgId"
         @click.prevent="" class=""><span uk-icon="mail"></span></a>
      <template v-else>
         ping</template>
      {{fmtD(response.Date)}}
   </span>
</script><script>
   Vue.component('mnm-pingresponse', {
      template: '#mnm-pingresponse',
      props: ['response'],
      computed: { mnm: function() { return mnm } },
      methods: { fmtD: mnm._formatDate }
   });
</script>

<script type="text/x-template" id="mnm-adrsbk">
   <div uk-dropdown="mode:click; offset:2; pos:bottom-right" class="uk-width-2-5">
      <ul uk-tab class="uk-child-width-expand" style="margin-top:0">
         <li v-for="aName in ['pings','invites','drafts','pinged','invited','groups','ohi to']">
            <a @click.prevent="" href="#" style="padding:0; cursor:default">{{aName}}</a>
         </li></ul>
      <ul class="uk-switcher" style="max-height:50vh; overflow-y:auto">
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>From</th><th>Message</th><th>Response</th></tr>
               <tr v-for="a in data.pf">
                  <td>{{fmtD(a.Date)}}</td><td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"></mnm-pingresponse></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>Group</th><th>From</th><th>Msg</th><th>Response</th></tr>
               <tr v-for="a in data.if">
                  <td>{{fmtD(a.Date)}}</td>
                  <td>{{a.Gid}}
                     <span v-if="data.gl.find(function(c){return c.Gid === a.Gid})"
                           class="uk-badge">in</span></td>
                  <td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"></mnm-pingresponse></td>
               </tr></table></li>
         <li>
            <form onsubmit="return false" style="text-align:center">
               <input v-model="draft.to" type="text" placeholder="To">
               <input v-model="draft.gid" type="text" placeholder="(Group)">
               <button :disabled="!validDraft" @click="startPing()" style="padding:0">
                  <span uk-icon="pencil"></span></button>
            </form>
            <table class="uk-table uk-table-small">
               <tr><th>To / (Group)</th><th></th><th>Message</th><th></th></tr>
               <tr v-for="a in data.ps" :key="rowId(a)">
                  <td>{{a.Alias}}<br>{{a.Gid && '('+a.Gid+')'}}</td>
                  <td><button @click="mnm.PingSend({to:a.Alias, gid:a.Gid})"
                              style="padding:0"><span uk-icon="forward"></span></button></td>
                  <td><textarea cols="40" rows="3" maxlength="120"
                                @input="timer(a, $event.target.value)"
                     >{{toSave[rowId(a)] || a.Text}}</textarea></td>
                  <td><button @click="mnm.PingDiscard({to:a.Alias, gid:a.Gid})"
                              style="color:crimson; padding:0"><span uk-icon="trash"></span></button></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>To</th><th>Message</th><th>Response</th></tr>
               <tr v-for="a in data.pt">
                  <td>{{fmtD(a.Date)}}</td><td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"></mnm-pingresponse></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>Group</th><th>To</th><th>Message</th><th>Response</th></tr>
               <tr v-for="a in data.it">
                  <td>{{fmtD(a.Date)}}</td><td>{{a.Gid}}</td><td>{{a.Alias}}</td><td>{{a.Text}}</td>
                  <td><mnm-pingresponse :response="a.Response"></mnm-pingresponse></td>
               </tr></table></li>
         <li>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>Group</th></tr>
               <tr v-for="a in data.gl">
                  <td>{{fmtD(a.Date)}}</td>
                  <td>{{a.Gid}}
                     <span v-if="a.Admin"
                           class="uk-badge">A</span></td>
               </tr></table></li>
         <li>
            <form onsubmit="this.reset(); return false;" style="text-align:center">
               <input oninput="this.nextElementSibling.disabled = !this.value"
                      type="text" size="32" name="resets" placeholder="Add someone">
               <button onclick="mnm.OhiAdd(this.previousElementSibling.value)" disabled
                       style="padding:0 4px">o/</button>
            </form>
            <table class="uk-table uk-table-small">
               <tr><th>Date</th><th>To</th><th></th></tr>
               <tr v-for="a in data.ot">
                  <td>{{fmtD(a.Date)}}</td><td>{{a.Uid /*todo alias*/}}</td>
                  <td><button @click="mnm.OhiDrop(null,a.Uid)"
                              style="color:crimson; padding:0"><span uk-icon="trash"></span></button></td>
               </tr></table></li>
      </ul>
   </div>
</script><script>
   Vue.component('mnm-adrsbk', {
      template: '#mnm-adrsbk',
      data: function() { return {draft:{to:'', gid:''}, toSave:{}} },
      computed: {
         mnm: function() { return mnm },
         data: function() { return mnm._data },
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
      <input v-model="addr"          @keyup.enter="send()" type="text" size="22"
             placeholder="Net Address">
      <input v-model="name"          @keyup.enter="send()" type="text" size="22"
             placeholder="Title">
      <input v-model="alias"         @keyup.enter="send()" type="text" size="22"
             placeholder="Alias">
      <input v-model.number="period" @keyup.enter="send()" type="text" size="16"
             placeholder="Login Freq. (op)">
      &nbsp;seconds<br>
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
         <tr><td>Net Address</td><td class="svccfg">{{cf.Addr }}</td></tr>
         <tr><td>Title      </td><td class="svccfg">{{cf.Name }}</td></tr>
         <tr><td>Alias      </td><td class="svccfg">{{cf.Alias}}</td></tr>
         <tr><td>Uid        </td><td class="svccfg">{{cf.Uid  }}</td></tr>
      </table>
   </div>
</script><script>
   Vue.component('mnm-svccfg', {
      template: '#mnm-svccfg',
      data: function() { return {} },
      computed: { cf: function() { return mnm._data.cf } },
      methods: {
      }
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
               this.$set(mnm._data.mo, iId, {});
               mnm.ThreadOpen(iId);
            } else {
               mnm.ThreadClose(iId);
               this.$delete(mnm._data.mo, iId);
            }
         },
         keyAction: function(iId, iEvent) {
            if (iEvent.ctrlKey && iEvent.key === 'j')
               mnm._lastPreview = iId;
         },
         to_save: function(iId) {
            if (!(iId in mnm._data.toSave)) {
               var aMo = mnm._data.mo[iId];
               sApp.$set(mnm._data.toSave, iId, {Alias:aMo.SubHead.Alias, Cc:aMo.SubHead.Cc,
                         Attach:aMo.SubHead.Attach, Data:aMo.msg_data, Id:iId});
            }
            if (!mnm._data.toSave[iId].timer)
               mnm._data.toSave[iId].timer = setTimeout(fDing, 2000, mnm._data.toSave[iId]);
            return mnm._data.toSave[iId];
            function fDing(cRec) {
               cRec.timer = undefined;
               mnm.ThreadSave(cRec);
            }
         },
         ccAdd: function(iId, iWidget) {
            if (iWidget.value.length === 0)
               return;
            var aCc = this.to_save(iId).Cc;
            if (!aCc)
               aCc = this.to_save(iId).Cc = [];
            aCc.unshift(iWidget.value);
            var aPrev = aCc.lastIndexOf(iWidget.value);
            if (aPrev !== 0)
               aCc.splice(aPrev, 1);
            iWidget.value = '';
         },
         atcAdd: function(iId, iPath) {
            var aAtc = this.to_save(iId).Attach;
            if (!aAtc)
               aAtc = this.to_save(iId).Attach = [];
            aAtc.unshift({Name:iPath});
            var aPrev = aAtc.findIndex(function(cEl, cI) {
               return cI > 0 && cEl.Name === iPath;
            });
            if (aPrev > 0)
               aAtc.splice(aPrev, 1);
         },
         textAdd: function(iId, iText) {
            this.to_save(iId).Data = iText;
         },
         ccDrop:  function(iId, iN) { this.to_save(iId).Cc    .splice(iN, 1) },
         atcDrop: function(iId, iN) { this.to_save(iId).Attach.splice(iN, 1) },
         atcGetName: function(iEl) { return iEl.Name },
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
      if (!sUrlStart.test(aHref[1])) {
         var aParam = aHref[1].replace(/^this_/, iEnv.msgId+'_');
         aHref[1] = '?an=' + encodeURIComponent(aParam);
      }
      return iSelf.renderToken(iTokens, iIdx, iOptions);
   };

   var sMdiRenderImg = mnm._mdi.renderer.rules.image;
   mnm._mdi.renderer.rules.image = function(iTokens, iIdx, iOptions, iEnv, iSelf) {
      var aAlt = iSelf.renderInlineAsText(iTokens[iIdx].children, iOptions, iEnv);
      var aSrc = iTokens[iIdx].attrs[iTokens[iIdx].attrIndex('src')];
      var aParam = aSrc[1].replace(/^this_/, iEnv.msgId+'_');
      if (aAlt.charAt(0) === '?') {
         if (!iEnv.formview)
            iEnv.formview = new mnm._FormViews;
         iEnv.formview.make(aParam);
         return '<component'+ iSelf.renderAttrs({attrs:[['id',aParam]]}) +'></component>';
      }
      aSrc[1] = '?an=' + encodeURIComponent(aParam);
      return sMdiRenderImg(iTokens, iIdx, iOptions, iEnv, iSelf);
   };

   mnm.Log = function(i) {
      var aLog = document.getElementById('log').innerText;
      document.getElementById('log').innerText = i+' '+aLog;
   };

   mnm.Render = function(i, iData, iEtc) {
      if (i.charAt(0) === '/')
         i = i.substr(1);

      if (sChange && (i === 'ml' || i === 'mo')) {
         sTemp[i] = iEtc ? iEtc : JSON.parse(iData);
         if (++sChange === 2)
            return;
         sChange = 0;
         mnm._data.ml = sTemp.ml;
         iEtc         = sTemp.mo;
         i = 'mo';
      }

      switch (i) {
      case 'cs': case 'sl': case 'cf': case 'al':
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
         sApp.$set(mnm._data.ao, iEtc, iData)
         break;
      case 'mo':
         for (var aK in mnm._data.mo)
            if (!(aK in iEtc))
               sApp.$delete(mnm._data.mo, aK);
         for (var aK in mnm._data.toSave)
            if (!(aK in iEtc))
               sApp.$delete(mnm._data.toSave, aK);
         for (var aK in iEtc)
            if (!(aK in mnm._data.mo))
               sApp.$set(mnm._data.mo, aK, iEtc[aK]);
         break;
      case 'mn':
         // avoid opening Cc & Attach menus if not changed
         var aEtcSubHead = {Cc: iEtc.SubHead.Cc, Attach: iEtc.SubHead.Attach};
         var aOrig = mnm._data.mo[iEtc.Id] && mnm._data.mo[iEtc.Id].SubHead;
         if (aOrig) {
            if (!fDiff(aOrig, 'Cc'    )) iEtc.SubHead.Cc     = aOrig.Cc;
            if (!fDiff(aOrig, 'Attach')) iEtc.SubHead.Attach = aOrig.Attach;
         }
         sApp.$set(mnm._data.mo, iEtc.Id, iEtc); //todo set ml Date
         var aOrig = mnm._data.toSave[iEtc.Id];
         if (aOrig) {
            aOrig.Alias = iEtc.SubHead.Alias;
            aOrig.Data = iEtc.msg_data;
            if (fDiff(aOrig, 'Cc'))     aOrig.Cc     = aEtcSubHead.Cc;
            if (fDiff(aOrig, 'Attach')) aOrig.Attach = aEtcSubHead.Attach;
         }
         function fDiff(cO, c) { return !(
            !cO[c]        && !aEtcSubHead[c]        ||
             cO[c]        &&  aEtcSubHead[c]        &&
             cO[c].length === aEtcSubHead[c].length &&
           (!cO[c].length                           ||
             cO[c][0]     === aEtcSubHead[c][0]     )); //todo need full comparison?
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

