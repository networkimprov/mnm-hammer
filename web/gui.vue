<!DOCTYPE html>
<!--
   Copyright 2018, 2019 Liam Breck
   Published at https://github.com/networkimprov/mnm-hammer

   This Source Code Form is subject to the terms of the Mozilla Public
   License, v. 2.0. If a copy of the MPL was not distributed with this
   file, You can obtain one at http://mozilla.org/MPL/2.0/
-->
<html><head>
   <title><%html .Title%> - mnm</title>
   <link rel="icon" href="/w/img/logo-48nobg-bleed-bright.png"/>

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
              class="btn btn-icon"><span uk-icon="refresh"></span></button>
   </div></div>

<script type="text/x-template" id="mnm-main">
<div uk-grid class="uk-grid-small">

<mnm-adrsbkmenu ref="adrsbkmenu"/>

<div class="uk-width-2-5">
   <div :class="{vishide: mnm._isLocal}"
        class="uk-clearfix">
      <div style="float:left; max-width:calc(100% - 1em - 182px); overflow-x:hidden; white-space: pre">
         <span :class="{vishide: msgSubjects.length <= 1}"
               class="dropdown-icon">&nbsp;&#x25BD;</span>
         <span class="uk-text-large" style="font-style:oblique">{{ msgTitle }}</span>
      </div>
      <div v-if="msgSubjects.length > 1"
           uk-dropdown="mode:click; offset:2"
           class="menu-bg dropdown-scroll">
         <div class="dropdown-scroll-list">
            <div v-for="aSubject in msgSubjects" :key="aSubject.msgId">
               <span @click="tabSearch(':'+ aSubject.name, cs.ThreadTabs)"
                     class="uk-link">{{aSubject.name}}</span>
            </div></div></div>
      <div class="uk-float-right">
         <span uk-icon="social" class="dropdown-icon"
               title="Recipients of thread">{{ml.length === 1 && !ml[0].From ? '' : cl[1].length}}</span>
         <mnm-cc ref="cl"
                 :tid="ml.length ? ml[ml.length-1].Id : 'none'"/>
         &nbsp;
         <span class="dropdown-icon"
               title="Attachments to thread">{{al.length || '&nbsp;'}}<mnm-paperclip/></span>
         <mnm-attach ref="al"/>
         &nbsp;
         <button @click="mnm.ThreadNew({alias:cf.Alias, cc:[]})"
                 title="New thread draft"
                 class="btn btn-icon"><span uk-icon="pencil"></span></button>
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
   <div :class="{vishide: mnm._isLocal}"
        uk-grid class="uk-grid-collapse">
      <mnm-tabs class="uk-width-expand"
                :set="msgTabset" :state="cs.ThreadTabs"/>
      <span v-show="msgTags.length">
         <span title="Tags within thread"
               uk-icon="hashtag" class="dropdown-icon"></span> &nbsp;
         <div uk-dropdown="mode:click; offset:2; pos:left-top"
              class="menu-bg dropdown-scroll">
            <div class="dropdown-scroll-list">
               <div v-for="aTag in msgTags" :key="aTag.Id">
                  <a @click.prevent="tabSearch('#'+aTag.Name, cs.ThreadTabs)" href="#">{{aTag.Name}}</a>
               </div></div>
         </div>
      </span>
      <div class="uk-width-1-6">
         <input @keyup.enter="tabSearch($event.target.value, cs.ThreadTabs)"
                :placeholder="' \u2315'" type="text"
                title="Find messages with phrase"
                class="width100 search-box"></div>
      <div v-for="aCmp in draftRefs" :key="aCmp.msgid"
           style="align-self:flex-end">
         <!-- uk-dropdown toggled by icon in msg-list repositions on update -->
         <div :id="'pv_'+aCmp.msgid"></div>
         <mnm-draftpv :draft="aCmp"/>
      </div>
   </div>
   <div @scroll="msglistGetScroll"
        ref="msglist"
        uk-height-viewport="offset-top:true; offset-bottom:true"
        class="firefox-minheight-fix uk-overflow-auto message-bg message-list"
        style="position:relative">
      <mnm-viewer ref="viewer" :noparent="true"/>
      <mnm-tagset ref="tagset"/><!--nextElementSibling is message list-->
      <ul class="uk-list uk-list-divider" style="margin:0">
         <li v-for="aMsg in ml" :key="aMsg.Id"
             :class="{'message-edit': aMsg.From === '' && !aMsg.Queued}" style="margin:0">
            <span @click="msgToggle(aMsg.Id)"
                  class="message-title" :class="{'message-title-unread': aMsg.Seen === ''}">
               <mnm-date :iso="aMsg.Date" ymd="md" hms="hm"/>
               {{ aMsg.Alias || aMsg.From }}
               <span v-if="aMsg.ForwardBy"
                     :title="'Forward by: '+aMsg.ForwardBy"
                     >{{/failed$/.test(aMsg.ForwardBy) ? '[possibly forged]' : '[unverified]'}}</span>
            </span>
            <div v-if="aMsg.Queued"
                 title="Awaiting link to server"
                 style="float:right; font-weight:bold"><span uk-icon="bolt"></span></div>
            <template v-if="aMsg.Id in mo">
               <span v-show="'msg_data' in mo[aMsg.Id]">
                  <button v-if="aMsg.From === '' && !aMsg.Queued"
                          @click="mnm.ThreadDiscard(aMsg.Id)"
                          title="Discard draft"
                          class="btn btn-iconred btn-floatr"><span uk-icon="trash"></span></button>
                  <div v-else-if="!aMsg.Queued"
                       class="uk-float-right">
                     <a @click.prevent="mnm._toClipboard('[msg_link](#'+ cs.Thread +'&'+ aMsg.Id +')')"
                        title="Copy markdown to clipboard"
                        :href="'#'+ cs.Thread +'&'+ aMsg.Id"><span uk-icon="link"></span></a>
                     <button @click="mnm.ThreadReply(getReplyTemplate(aMsg))"
                             title="New reply draft"
                             class="btn btn-icon"><span uk-icon="comment"></span></button>
                  </div>
                  <div @click.stop="$refs.tagset.open(aMsg.Id, $event.currentTarget)"
                       title="Message tags"
                       class="uk-float-right tagset-icon">
                     <span uk-icon="tag">{{aMsg.Tags ? aMsg.Tags.length : '&numsp;'}}</span></div>
               </span>
               <div v-if="!('msg_data' in mo[aMsg.Id])"
                    class="uk-text-center"><span uk-icon="future"><!-- todo hourglass --></span></div>
               <mnm-draft v-else-if="aMsg.From === '' && !aMsg.Queued"
                          :msgid="aMsg.Id"/>
               <template v-else>
                  <div class="message-subhead">
                     <template v-if="mo[aMsg.Id].SubHead.Attach">
                        <!--todo move _hideAtc to state-->
                        <div @click="$set(mo[aMsg.Id], '_hideAtc', !mo[aMsg.Id]._hideAtc)"
                             title="Hide/show attachments"
                             class="uk-float-left message-paperclip"
                             :class="{'message-paperclip-close': mo[aMsg.Id]._hideAtc}">
                           {{ (mo[aMsg.Id].SubHead.Attach.length < 10 ? '&numsp;' : '') +
                              mo[aMsg.Id].SubHead.Attach.length }}<mnm-paperclip/></div>
                        <div v-show="mo[aMsg.Id]._hideAtc && !aMsg.Subject"
                             >&nbsp;</div>
                        <div v-for="aAtc in mo[aMsg.Id].SubHead.Attach"
                             v-show="!mo[aMsg.Id]._hideAtc"
                             class="message-attach">
                           <template v-if="'ForwardBy' in aMsg && !/failed$/.test(aMsg.ForwardBy)">
                              <span class="icon-blank"></span>
                              <span :title="'Awaiting receipt from '+ aMsg.ForwardBy">
                                 <span uk-icon="bolt"></span> {{aAtc.Name.substr(2)}}</span>
                           </template>
                           <template v-else-if="aAtc.Name.charAt(0) === 'r'">
                              <span class="icon-blank"></span>
                              <span @click="tabSearch('ffn:'+ aAtc.Ffn +
                                              (aMsg.From === cf.Uid ? '_sent' : '_recv'), cs.SvcTabs)"
                                    title="Open filled-form table"
                                    class="uk-link">
                                 <span uk-icon="file-edit"></span> {{aAtc.Name.substr(2)}}</span>
                           </template>
                           <template v-else>
                              <a :href="'?ad=' + encodeURIComponent(aMsg.Id +'_'+ aAtc.Name)" download
                                 title="Download attachment">
                                 <span uk-icon="download"></span></a>
                              <span v-if="!mnm._viewerType('svc', aMsg.Id +'_'+ aAtc.Name)">
                                 <span class="icon-blank"></span> {{aAtc.Name.substr(2)}}</span>
                              <a v-else
                                 @click.stop.prevent="$refs.viewer.open('svc', aMsg.Id +'_'+ aAtc.Name,
                                                                        $event.currentTarget, 'rhs')"
                                 :href="'?an=' + encodeURIComponent(aMsg.Id +'_'+ aAtc.Name)">
                                 <span uk-icon="triangle-right">&nbsp;</span>{{aAtc.Name.substr(2)}}</a>
                              <div class="uk-float-right">{{aAtc.Size}}</div>
                           </template>
                        </div>
                     </template>
                     <div v-if="aMsg.Subject"
                          >Re: {{aMsg.Subject}}</div>
                  </div>
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
   <div :class="{vishide: mnm._isLocal}"
        class="uk-clearfix">
      <span class="uk-text-large">
         <span uk-icon="star" title="Placeholder for site logo"></span>
         <%html .Title%>
      </span>
      <div class="uk-float-right">
         <span uk-icon="bell" class="dropdown-icon" style="font-weight:bold"
               :title="'Notices for <%.TitleJs%>'">
            <span v-show="mnm._data.errorFlag"
                  style="color:crimson" uk-icon="warning"></span>
            {{svcSelf.NoticeN || null}}<!---->
         </span>
         <mnm-notice ref="notice"
                     @beforeshow.native="mnm.NoticeOpen('<%.TitleJs%>')"
                     @hide.native="mnm.NoticeClose()"
                     offset="2" pos="bottom-right"/>
         <span uk-icon="users" class="dropdown-icon"
               :title="'Contacts for <%.TitleJs%>'">&nbsp;</span>
         <mnm-adrsbk ref="adrsbk"/>
         <span @mousedown="ohiFrom = !ohiFrom"
               style="color:#1e87f0; font-size:110%; vertical-align:text-top; cursor:pointer"
               title="Toggle ohi-from panel">&nbsp;o/</span>
         <span uk-icon="laptop" class="dropdown-icon"
               :title="'Replicas for <%.TitleJs%>'">&nbsp;</span>
         <mnm-nodes/>
         &nbsp;
         <span uk-icon="push" class="dropdown-icon"
               title="Attachable files">&nbsp;</span>
         <mnm-files ref="t" pos="bottom-right"/>
         <span uk-icon="file-edit" class="dropdown-icon"
               title="Blank forms">&nbsp;</span>
         <mnm-forms ref="f" pos="bottom-right"/>
         &nbsp;
      </div>
   </div>
   <div :class="{vishide: mnm._isLocal}"
        uk-grid class="uk-grid-collapse">
      <ul uk-tab class="uk-width-expand"><li style="display:none"></li>
         <li v-for="(aTerm, aI) in mnm._tabsStdService"
             :class="{'uk-active': cs.SvcTabs.PosFor === 0 && cs.SvcTabs.Pos === aI}">
            <a @click.prevent="mnm.TabSelect({type:cs.SvcTabs.Type, posfor:0, pos:aI})"
               href="#">
               <span v-if="aTerm.Term === 'Unread'"
                     >{{svcSelf.UnreadN || null}}</span>
               {{aTerm.Label || aTerm.Term}}<!---->
            </a>
         </li></ul>
      <span>
         <span title="Search by tag"
               uk-icon="hashtag" class="dropdown-icon"></span> &nbsp;
         <div uk-dropdown="mode:click; offset:2; pos:left-top"
              class="menu-bg dropdown-scroll">
            <div class="dropdown-scroll-list">
               <div v-for="aTag in g" :key="aTag.Id">
                  <a @click.prevent="tabSearch('#'+aTag.Name, cs.SvcTabs)" href="#">{{aTag.Name}}</a>
               </div></div>
         </div>
      </span>
      <div class="uk-width-1-2">
         <input @keyup.enter="tabSearch($event.target.value, cs.SvcTabs)"
                :placeholder="' \u2315'" type="text"
                title="Search all threads"
                class="width100 search-box"></div>
   </div>
   <mnm-tabs v-if="cs.SvcTabs.Pinned.length || cs.SvcTabs.Terms.length"
             :set="svcTabset" :state="cs.SvcTabs"/>
   <div uk-height-viewport="offset-top:true"
        class="thread-list firefox-minheight-fix uk-overflow-auto">
      <template v-if="ffn">
         <table class="uk-table uk-table-small uk-table-hover uk-text-small">
            <tr>
               <th v-for="(a, aKey) in ffnCol"
                   v-if="aKey.charAt(0) !== '$' || aKey === '$msgid'"
                   style="position:sticky; top:0"
                   >{{aKey === '$msgid' ? 'source' : aKey}}</th>
            </tr>
            <tr v-for="aRow in tl">
               <td v-for="(a, aKey) in ffnCol"
                   v-if="aKey.charAt(0) !== '$' || aKey === '$msgid'">
                  <a v-if="aKey === '$msgid'"
                     onclick="mnm.NavigateLink('Form result', this.href); return false"
                     title="Find message with result"
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
                  <template v-else
                            >{{aRow[aKey]}}</template>
               </td>
            </tr>
         </table></template>
      <template v-else-if="cs.SvcTabs.PosFor === 0 && mnm._tabsStdService[cs.SvcTabs.Pos].Term === 'FFT'">
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
              :style="{'background-color': aRow.Id === cs.Thread ? '#fff7cf' : null}"><!--todo class thread-row-thread-->
            <div class="uk-width-auto" style="padding:0"
                 :class="{'thread-unread': aRow.Unread}">
               <mnm-date :iso="aRow.LastDate" ymd="md"/></div>
            <div class="uk-width-1-6 overxhide"
                 :class="{'thread-unread': aRow.Unread}">{{aRow.LastAuthor}}</div>
            <div class="uk-width-expand overxhide"
                 :title="aRow.Id">{{aRow.Subject}}<!---->
               <i v-show="aRow.SubjectWas"
                  >&nbsp;f. {{aRow.SubjectWas}}</i>
            </div>
            <div class="uk-width-auto">
               <mnm-date :iso="aRow.OrigDate" ymd="_md"/></div>
            <div v-if="aRow.OrigAuthor !== mnm._data.cf.Alias"
                 class="uk-width-1-6 overxhide">{{aRow.OrigAuthor}}</div>
            <div v-else
                 :title="'Initial recipient'+ (aRow.OrigCc.length ? 's:\n'+ aRow.OrigCc.join('\n')
                                                                  : ': this account')"
                 class="uk-width-1-6 overxhide thread-origcc">{{aRow.OrigCc[0] || 'self'}}</div>
         </div></template>
      <div style="margin-top:1em">
         <div onclick="this.nextSibling.style.display = (this.nextSibling.style.display === 'none' ? 'block' : 'none')"
              style="float:right; margin-right:0.5em; font-size:75%;">
            <span class="uk-link">&#x25c1; mo</span></div
        ><div style="display:none">{{JSON.stringify(mo)}}</div><br>
         <div onclick="this.nextSibling.style.display = (this.nextSibling.style.display === 'none' ? 'block' : 'none')"
              style="float:right; margin-right:0.5em; font-size:75%;">
            <span class="uk-link">&#x25c1; log</span></div
        ><div style="display:none; white-space:pre-wrap" id="log"></div>
      </div>
      <mnm-tour v-if="mnm._isLocal || location.hash === '#tour'"/>
   </div>
   <div class="uk-position-relative">
      <div v-show="ohiFrom"
           class="uk-card uk-card-secondary uk-text-small uk-border-rounded"
           style="padding:8px; position:absolute; bottom:10px; right:10px">
         <div v-if="!of"
              class="uk-text-danger">no link</div>
         <template v-else>
            <div v-show="of.length === 0"
                 class="uk-text-warning">no o/</div>
            <ul class="uk-list uk-text-success" style="margin:0">
               <li v-for="aUser in of" :key="aUser.Uid"
                   >{{aUser.Alias}}</li>
            </ul>
         </template>
      </div></div>
</div>

<div class="uk-width-expand service-panel">
   <div class="uk-clearfix uk-light">
      <span uk-icon="plus-circle" class="dropdown-icon"
            title="Add account"></span>
      <mnm-svcadd ref="svcadd"/>
      <div style="float:right; margin:0 1em 1em 0">
         <span uk-icon="cog" class="dropdown-icon"
               title="General settings">&nbsp;</span>
         <mnm-settings/>
         <span uk-icon="info" class="dropdown-icon"
               title="Documentation">&nbsp;</span>
         <div ref="doc"
              uk-dropdown="mode:click; offset:2; pos:bottom-right"
              onshow="this.firstElementChild.contentWindow.mnm_resetScroll()"
              class="width60v height75v menu-bg dropdown-scroll">
            <iframe src="/w/docs.html" style="width:100%; height:100%"></iframe></div>
      </div>
   </div>
   <div uk-height-viewport="offset-top:true" class="firefox-minheight-fix uk-overflow-auto uk-light">
      <ul class="uk-list uk-list-divider">
         <li v-for="aSvc in v" :key="aSvc.Name">
            <template v-if="aSvc.Name === '<%.TitleJs%>'">
               <span uk-icon="settings" class="dropdown-icon"
                     :title="'Settings for <%.TitleJs%>'">&numsp;</span>
               <mnm-svccfg/>
               {{aSvc.Name}}
            </template>
            <template v-else>
               <span uk-icon="bell" class="dropdown-icon"
                     :title="'Notices for '+ aSvc.Name">{{aSvc.NoticeN || '&numsp;'}}</span>
               <mnm-notice :svc="aSvc.Name"
                           @beforeshow.native="mnm.NoticeOpen(aSvc.Name)"
                           @hide.native="mnm.NoticeClose()"
                           offset="-4" pos="left-top"/><!--todo single notice menu-->
               <a :href="'/'+encodeURIComponent(aSvc.Name)"
                  :target="mnm._isLocal ? '_self' : 'mnm_svc_'+ aSvc.Name">{{aSvc.Name}}</a>
            </template>
         </li></ul>
   </div>
</div>

</div>
</script>

<script type="text/x-template" id="mnm-tour">
   <div style="min-height:260px; width:85%; margin:0 auto 3em; padding:0.7em;
               border-radius:12px; background-color:white">
      <div style="float:right">
         <button @click="--count"
                 :disabled="count === 0"
                 class="uk-button uk-button-link">
            <span uk-icon="icon:triangle-left; ratio:1.6"></span></button>&nbsp;
         <button @click="++count"
                 :disabled="count === last"
                 title="Next slide"
                 class="uk-button uk-button-link">
            <span uk-icon="icon:triangle-right; ratio:1.6"></span></button>
      </div>
      <template v-if="mnm._isLocal">
         <div v-show="count < 4"
              class="tour-heading">How mnm works</div>
         <div v-show="count === 0"
              class="tour-slide">
            <div><img src="/w/img/tour-orgs.png"></div>
            You're a member of many organizations and Internet sites.</div>
         <div v-show="count === 1"
              class="tour-slide">
            <div><img src="/w/img/tour-identity.png"></div>
            mnm gives you a separate identity in each place.</div>
         <div v-show="count === 2"
              class="tour-slide">
            <div><img src="/w/img/tour-contact.png"></div>
            Only other place members can contact you by that identity.</div>
         <div v-show="count === 3"
              class="tour-slide">
            <div><img src="/w/img/tour-invite.png"></div>
            You invite a member to connect before exchanging messages.</div>
         <div v-show="count === 4"
              class="tour-heading">Get Started</div>
         <div v-show="count === 4"
              class="tour-slide">
            <div class="tour-inset">
               <div>Linking mnm to a place</div>
               <img src="/w/img/tour-link.png">
            </div>
            <b>1.</b> Click the add account icon, then<br>
            <span @click.stop="UIkit.dropdown($root.$refs.svcadd.$el).show()"
                  uk-icon="plus-circle" title="Add account"></span>
            <div>
               a) Fill out the form:<div style="margin-left: 1.1em">
                  <i>Site Address</i> is provided by the site admin,<br>
                  <i>Your Name/Alias</i> is how others know you,<br>
                  <i>Account Title</i> is a private label.</div>
               b) To submit, click
                  <button class="btn btn-icon"><span uk-icon="forward"></span></button>
            </div>
            <div style="margin: 0.5em 0">
               <b>2.</b> Inform others of your <i>Name/Alias</i> by phone, etc.</div>
            <b>3.</b> Click the account to open it (and continue the tour).<br>
            <div class="service-panel uk-light"
                 style="width:20%; margin-top:0.5em; padding:1em; overflow:hidden">
               <span v-if="mnm._data.v.length === 0"
                     >awaiting link. . .</span>
               <div v-else>
                  <span uk-icon="bell" class="dropdown-icon">{{mnm._data.v[0].NoticeN || '&numsp;'}}</span>
                  <a :href="'/'+ encodeURIComponent(mnm._data.v[0].Name) +'#tour'"
                     target="_self"
                     title="Continue tour">{{mnm._data.v[0].Name}}</a>
               </div>
            </div>
         </div>
      </template>
      <template v-else>
         <div class="tour-heading">Basic Usage</div>
         <div v-show="count === 0"
              class="tour-slide">
            <div class="tour-inset">
               <div>Sending an invite</div>
               <img src="/w/img/tour-sendinvite.png">
            </div>
            Click the contacts icon, then<br>
            <span @click.stop="UIkit.dropdown($root.$refs.adrsbk.$el).show()"
                  uk-icon="users" title="Contacts"></span>
            <div>
               a) Enter the contact's alias, click
                  <button class="btn btn-icon"><span uk-icon="pencil"></span></button><br>
               b) Add a message to the draft.<br>
               c) To send, click
                  <button class="btn btn-icon"><span uk-icon="forward"></span></button><br>
            </div>
         </div>
         <div v-show="count === 1"
              class="tour-slide">
            <div class="tour-inset">
               <div>Acquiring contacts</div>
               <img src="/w/img/tour-contacts.png">
            </div>
            New contacts land in your address book when:<br>
            <span @click.stop="UIkit.dropdown($root.$refs.notice.$el).show()"
                  uk-icon="bell" title="Notices"></span>
            <div>
               - an invite arrives,<br>
               - a thread arrives from someone you invited.<br>
               Invites appear in the notices menu.<br>
            </div>
         </div>
         <div v-show="count === 2"
              class="tour-slide">
            <div class="tour-inset">
               <div>Sending a thread</div>
               <img src="/w/img/tour-sendthread.png">
            </div>
            <div><b>1.</b> To start a thread, click
               <button class="btn btn-icon"><span uk-icon="pencil"></span></button>
               (next to <span uk-icon="icon:arrow-left; ratio:1.6"></span
                       ><span uk-icon="icon:arrow-right; ratio:1.6"></span>)</div>
            <b>2.</b> Click the recipients icon, then<br>
            <span @click.stop="UIkit.dropdown($root.$refs.cl.$el).show()"
                  uk-icon="social" title="Recipients"></span>
            <div>
               a) Enter a contact's alias in the <i>To</i> field.<br>
               b) Select the contact in the menu, hit enter.<br>
            </div>
            <b>3.</b> In the new thread<br>
            <span></span>
            <div>
               a) Fill in the <i>Subject</i> field.<br>
               b) Write a message (Markdown is allowed).<br>
               c) To send, click
               <button class="btn btn-icon"><span uk-icon="forward"></span></button>
               (above <i>Subject</i>)<br>
            </div>
         </div>
         <div v-show="count === 3"
              class="tour-slide">
            <div class="tour-inset">
               <div>Sending a reply</div>
               <img src="/w/img/tour-sendreply.png">
            </div>
            Select a message to reply to, click
            <button class="btn btn-icon"><span uk-icon="comment"></span></button>, then<br>
            <span></span>
            <div>
               a) Write a message (<i>Subject</i> is optional).<br>
               b) To send, click
                  <button class="btn btn-icon"><span uk-icon="forward"></span></button><br>
            </div>
         </div>
         <div v-show="count === 4"
              class="tour-slide">
            <div class="tour-inset">
               <div>Forwarding a thread</div>
               <img src="/w/img/tour-forward.png">
            </div>
            Click the recipients icon, then<br>
            <span @click.stop="UIkit.dropdown($root.$refs.cl.$el).show()"
                  uk-icon="social" title="Recipients"></span>
            <div>
               a) Enter a contact's alias in the <i>Forward to</i> field.<br>
               b) Select the contact in the menu, hit enter.<br>
               c) To forward, click
                  <button class="btn btn-icon"><span uk-icon="forward"></span></button><br>
            </div>
         </div>
         <div v-show="count === 5"
              class="tour-slide">
            <div class="tour-inset">
               <div>Signaling online presence</div>
               <img src="/w/img/tour-sendohi.png">
            </div>
            Click the contacts icon, then<br>
            <span @click.stop="UIkit.dropdown($root.$refs.adrsbk.$el).show()"
                  uk-icon="users" title="Contacts"></span>
            <div>
               a) Select the <i>Ohi To</i> tab.<br>
               b) Enter a contact's alias in the <i>To</i> field.<br>
               c) Select the contact in the menu, hit enter.<br>
               d) To signal this contact, click <button>o/ <i>Name</i></button><br>
            </div>
         </div>
         <div v-show="count === 6"
              class="tour-slide">
            See also:<br>
            <span @click.stop="UIkit.dropdown($root.$refs.doc).show()"
                  uk-icon="info" title="Documentation"></span>
            <div>the info menu for full docs.</div>
         </div>
      </template>
   </div>
</script><script>
   Vue.component('mnm-tour', {
      template: '#mnm-tour',
      data: function() {
         return {last: mnm._isLocal ? 4 : 6, count:0};
      },
      computed: {
         mnm:   function() { return mnm },
         UIkit: function() { return UIkit },
      },
   });
</script>

<script type="text/x-template" id="mnm-date">
   <span :title="title">{{text}}</span>
</script><script>
   Vue.component('mnm-date', {
      template: '#mnm-date',
      props: {iso:String, ymd:String, hms:String},
      computed: {
         dt: function() { return luxon.DateTime.fromISO(this.iso) },
         text: function() {
            var aMd = this.ymd === 'md';
            if (this.ymd === '_md') {
               var aNow = luxon.DateTime.utc();
               aMd = this.dt.year === aNow.year ||
                     this.dt.year === aNow.year-1 && this.dt.month >= aNow.month+10;
            }
            var aDate = this.dt.toString();
            var aD = aDate.substring(aMd ? 5 : 0, 10);
            if (aD.charAt(0) === '0')
               aD = '\u2007' + aD.substr(1);
            if (!this.hms)
               return aD;
            return aD +' '+ aDate.substring(11, this.hms === 'hm' ? 16 : 19);
         },
         title: function() {
            return this.dt.toLocaleString(luxon.DateTime.DATETIME_FULL_WITH_SECONDS);
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-cc">
   <div uk-dropdown="mode:click; offset:2"
        class="widthmin33 menu-bg dropdown-scroll">
      <div class="dropdown-scroll-item">
         <form onsubmit="this.reset(); return false"
               style="margin-bottom:0.5em">
            <input v-model="note"
                   placeholder="Note (opt.)" maxlength="1024" type="text"
                   style="width:calc(50% - 2em)">
            <mnm-adrsbkinput @alias="alias = $event"
                             @input.native="alias = $event.target.value"
                             :type="3"
                             :placeholder="ccSet ? 'To' : 'Forward to'"
                             name="resets" autocomplete="off"
                             style="width:calc(50% - 1em)"/>
            <button @click="addUser"
                    :disabled="!(alias && alias in mnm._data.adrsbkmenuId)"
                    title="Add contact"
                    class="btn btn-icon btn-floatr"><span uk-icon="list"></span></button>
         </form>
         <button v-show="!ccSet"
                 @click="mnm.ForwardSend(tid, mnm._data.cl[0][0].Qid)"
                 :disabled="ccSet || !mnm._data.cl[ccSet].length"
                 title="Forward thread to new recipients"
                 style="margin-right:0.5em"
                 class="btn btn-icon"><span uk-icon="forward"></span></button>
         <div v-for="(aUser, aI) in menu" :key="aUser.Who"
              :title="aUser.Note"
              class="cc-new">{{
            aUser.Who
          }}<button @click="dropUser(aI)"
                    class="btnx"><span>&times;</span></button>
         </div>
         <br v-show="!menu.length">
      </div>
      <template v-if="!ccSet">
         <ul uk-tab class="dropdown-scroll-item"><li style="display:none"></li>
            <li v-for="aKey in ['Who','By','Date']"
                :class="{'uk-active': aKey === mnm._data.cs.Sort.cl}">
               <a @click.prevent="mnm.SortSelect('cl', aKey)" href="#">{{aKey}}</a>
            </li></ul>
         <ul class="uk-list uk-list-divider dropdown-scroll-list">
            <li v-for="aUser in mnm._data.cl[1]" :key="aUser.Who">
               <div style="float:left; width:40%">
                  <span :title="aUser.Note">{{aUser.Who}}</span>
                  <span v-show="aUser.Queued"
                        title="Awaiting link to server"
                        uk-icon="bolt"></span>
               </div>
               {{aUser.By}}
               <div style="float:right"><mnm-date :iso="aUser.Date"/></div>
            </li></ul>
      </template>
   </div>
</script><script>
   Vue.component('mnm-cc', {
      template: '#mnm-cc',
      props: {tid:String},
      data: function() { return {alias:'', note:'', lastMenu:[]} },
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
                  aMenu[aN++] = aCc[a];
            return this.lastMenu = aMenu;
         },
         mnm: function() { return mnm }
      },
      methods: {
         now: function() { return luxon.DateTime.local() },
         addUser: function() {
            var aCc = mnm._data.cl[this.ccSet].slice();
            var aPrev = aCc.findIndex(function(c) { return c.Who === this.alias }, this);
            var aEl = aPrev >= 0 ? aCc.splice(aPrev, 1)[0]
                                 : {Who:this.alias, WhoUid:mnm._data.adrsbkmenuId[this.alias]};
            aEl.Note = this.note;
            aCc.push(aEl);
            if (this.ccSet)
               mnm._data.draftRefs[this.tid].save(aCc, null);
            else
               mnm.ForwardSave(this.tid, aCc);
            this.alias = '';
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
   <div uk-dropdown="mode:click; offset:2"
        class="widthmin33 menu-bg dropdown-scroll"
        @hidden="$refs.viewer.close()" @click="$refs.viewer.close()">
      <ul uk-tab class="dropdown-scroll-item"><li style="display:none"></li>
         <li v-for="aKey in ['Date','Name','Size']"
             :class="{'uk-active': aKey === mnm._data.cs.Sort.al}">
            <a @click.prevent="mnm.SortSelect('al', aKey)" href="#">{{aKey}}</a>
         </li></ul>
      <mnm-viewer ref="viewer"/>
      <ul class="uk-list uk-list-divider dropdown-scroll-list">
         <li v-for="aFile in mnm._data.al" :key="aFile.File">
            <a @click.prevent="markdown(aFile)"
               title="Copy markdown to clipboard"
               :href="'#@'+ aFile.File"><span uk-icon="link"></span></a>
            <a onclick="mnm.NavigateLink('Attached', this.href); return false"
               title="Find message with attachment"
               :href="'#'+ mnm._data.cs.Thread +'&'+ aFile.MsgId"
               :class="{vishide: aFile.MsgId.charAt(0) === '_'}"><span uk-icon="mail"></span></a>
            <mnm-date :iso="aFile.Date" ymd="md" hms="hm"/>
            <!--todo button :title="aFile.File.charAt(17) === 'u' ? 'Copy to attachable files'
                                                          : 'Copy to blank forms'"
                    class="btn btn-icon">
               <span :uk-icon="aFile.File.charAt(17) === 'u' ? 'push' : 'file-edit'"></span></button>
            &nbsp;-->
            <a :href="'?ad=' + encodeURIComponent(aFile.File)" download
               title="Download attachment">
               <span uk-icon="download">&nbsp;</span></a>
            <span v-if="!mnm._viewerType('svc', aFile.File)">
               &nbsp;<span class="icon-blank"></span>{{aFile.Name}}</span>
            <a v-else
               @click.stop.prevent="$refs.viewer.open('svc', aFile.File, $event.currentTarget)"
               :href="'?an=' + encodeURIComponent(aFile.File)">
               <span uk-icon="triangle-left">&nbsp;</span>{{aFile.Name}}</a>
            <div class="uk-float-right">{{aFile.Size}}</div>
         </li></ul>
   </div>
</script><script>
   Vue.component('mnm-attach', {
      template: '#mnm-attach',
      computed: { mnm: function() { return mnm } },
      methods: {
         listSort: function(i) { return mnm._listSort('al', i) },
         markdown: function(iFile) {
            var aSep = iFile.File.indexOf('_')
            var aFile = iFile.File.substring(aSep + 1);
            var aRef = aSep === 12 ? 'this_'+ aFile : iFile.File;
            aRef = aRef.replace(/%/g, '%25').replace(/ /g, '%20')
                       .replace(/\(/g, '%28').replace(/\)/g, '%29');
            //todo mnm-draft validate attachment & message refs
            var aTxt;
            if (aFile[0] === 'f') {
               aTxt = '![?]('+ aRef +')';
            } else {
               var aDot = aFile.lastIndexOf('.');
               var aExt = aDot < 0 ? '' : aFile.substring(aDot + 1);
               switch (aExt.toLowerCase()) {
               case 'jpg': case 'jpeg': case 'png': case 'gif': case 'svg':
                  aTxt = '!['+ aExt +']('+ aRef +')';
                  break;
               default:
                  aTxt = '['+ iFile.Name +']('+ aRef +')';
               }
            }
            mnm._toClipboard(aTxt);
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-tagset">
   <div v-show="msgId"
        class="tagset message-edit dropdown-scroll uk-card uk-card-default"
        style="position:absolute" :style="{top:posTop, right:posRight}"
        @click.stop>
      <div class="dropdown-scroll-list">
         <div v-for="aTag in mnm._data.g" :key="aTag.Id"
              @click="toggle(aTag.Id)">
            <span><span :class="{vishide:!hasId[aTag.Id]}"
                        uk-icon="check"></span></span>
            {{aTag.Name}}
         </div></div>
      <form onsubmit="return false" @submit="newName = ''"
            class="dropdown-scroll-item">
         <input v-model="newName"
                placeholder="New Tag" type="text"
                style="width:7em">
         <button @click="mnm.TagAdd(newName)"
                 :disabled="!newName || mnm._data.g.find(function(c) { return c.Name === newName })"
                 title="New tag"
                 class="btn btn-icon"><span uk-icon="list"></span></button>
      </form>
   </div>
</script><script>
   Vue.component('mnm-tagset', {
      template: '#mnm-tagset',
      data: function() { return {msgId:'', newName:'', posTop:0, posRight:0} },
      computed: {
         mnm: function() { return mnm },
         hasId: function() {
            var aSet = {};
            var aMsg = mnm._data.ml.find(function(c) { return c.Id === this.msgId }, this);
            if (aMsg && aMsg.Tags)
               aMsg.Tags.forEach(function(cTag) { aSet[cTag] = true });
            return aSet;
         },
      },
      methods: {
         open: function(iId, iEl) {
            if (this.msgId === iId) {
               this.msgId = '';
            } else {
               this.msgId = iId;
               this.posTop = iEl.offsetTop +'px';
               this.posRight = (this.$el.nextElementSibling.offsetWidth - iEl.offsetLeft + 3) +'px';
                                // .offsetParent.offsetWidth doesn't vary with scrollbar
            }
         },
         toggle: function(iId) {
            if (!this.hasId[iId])
               mnm.ThreadTag(this.msgId, iId);
            else
               mnm.ThreadUntag(this.msgId, iId);
         },
      },
      created: function() {
         var that = this;
         document.addEventListener('click', function() { that.msgId = '' });
      },
   });
</script>

<script type="text/x-template" id="mnm-draft">
   <div @keydown="keyAction('pv_'+msgid, $event)">
      <div style="position:relative; padding:1px;">
         <button @click="send"
                 :disabled="mnm._data.ml.length < 2 && !subject"
                 title="Send draft"
                 class="btn btn-icon btn-alignt"><span uk-icon="forward"></span></button>
         <span v-if="mnm._data.cl[1].length < 2"
               class="draft-recip" :class="{'draft-recip-new': mnm._data.ml.length === 1}"
               >[self]</span>
         <span v-else
               class="draft-recip" :class="{'draft-recip-new': mnm._data.ml.length === 1}"
               >{{firstCc}}</span>
         <span v-if="mnm._data.cl[1].length > 2"
               class="draft-recip" :class="{'draft-recip-new': mnm._data.ml.length === 1}"
               >+{{mnm._data.cl[1].length - 2}}</span>
         <div style="height:100%; position:absolute; left:13em; right:42px; top:0;">
            <mnm-draftmenu @drop="atcDrop"
                           :list="mnm._data.mo[msgid].SubHead.Attach"
                           :getname="atcGetName" :getkey="atcGetKey"
                           :style="{float:'right'}"/>
         </div>
      </div>
      <div style="float:right; margin-top:-1.7em;">
         <span uk-icon="push" class="dropdown-icon" :id="'t_'+msgid"
               title="Attach files"></span
        ><span uk-icon="file-edit" class="dropdown-icon" :id="'f_'+msgid"
               title="Attach blank forms"></span>
      </div>
      <input @input="subjAdd"
             @focus="subjShow = true"
             @click.stop="clickPreview"
             :value="subject"
             :title="mnm._data.ml.length === 1 || subjShow ? '' : 'Edit subject'"
             :placeholder="mnm._data.ml.length === 1 ? 'Subject' : subjShow ? 'Re' : ''" type="text"
             class="width100"
             :class="{'draft-minsubject': mnm._data.ml.length > 1 && !subjShow && !subject}">
      <mnm-textresize @input.native="textAdd"
                      @click.native.stop="clickPreview"
                      @resize="$root.msglistSetScroll"
                      :src="(mnm._data.toSave[msgid] || mnm._data.mo[msgid]).msg_data"
                      placeholder="Message text, Markdown OK. Ctrl-J to preview!"
                      class="width100"/>
   </div>
</script><script>
   Vue.component('mnm-draft', {
      template: '#mnm-draft',
      props: {msgid:String},
      data: function() { return {subjShow: false} },
      computed: {
         mnm: function() { return mnm },
         subject: function() {
            return (mnm._data.toSave[this.msgid] || mnm._data.mo[this.msgid].SubHead).Subject;
         },
         firstCc: function() {
            var aUser;
            mnm._data.cl[1].forEach(function(cUser) {
               if (cUser.WhoUid !== mnm._data.cf.Uid && (!aUser || cUser.Date < aUser.Date))
                  aUser = cUser;
            });
            return aUser.Who;
         },
      },
      created: function() { Vue.set(mnm._data.draftRefs, this.msgid, this) }, // $refs not reactive
      beforeDestroy: function() { Vue.delete(mnm._data.draftRefs, this.msgid) },
      methods: {
         keyAction: function(iId, iEvent) {
            if (iEvent.ctrlKey && iEvent.key === 'j')
               mnm._lastPreview = iId;
         },
         clickPreview: function() {
            document.getElementById('pp_'+this.msgid).click();
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
               aToSave.timer = setTimeout(fDing, mnm._saveWaitTime, this);
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

<script type="text/x-template" id="mnm-draftpv">
   <div uk-dropdown="mode:click; pos:right-top; offset:3"
        :id="'pp_'+draft.msgid"
        class="draft-preview message-bg"
        onwheel="return mnm._canScroll(this, event.deltaY)">
      <div v-show="subject || attach || hasDeck">
         <div v-show="attach"
              style="float:left; margin-right:0.6em"
              >{{attach && attach.length}}<mnm-paperclip/></div>
         <div v-show="hasDeck"
              style="float:right">
            <a @click.prevent="allSlides = !allSlides"
               title="Toggle all slides"
               href="#">&Lang;. . .&Rang;</a></div>
         <span v-show="subject"
               >Re: {{subject}}</span>&nbsp;<!---->
      </div>
      <div v-show="!msg.msg_data">
         <p><span uk-icon="comment"></span></p></div>
      <mnm-markdown v-show="msg.msg_data"
                    @hasdeck="hasDeck = $event"
                    @formfill="draft.ffAdd.apply(draft, arguments)"
                    @toggle="draft.atcToggleFf.apply(draft, arguments)"
                    :src="msg.msg_data" :msgid="draft.msgid" :allslides="allSlides"
                    :formfill="msg.form_fill" :atchasff="draft.atcHasFf"/>
   </div>
</script><script>
   Vue.component('mnm-draftpv', {
      template: '#mnm-draftpv',
      props: {draft:Object},
      data: function() { return {hasDeck:false, allSlides:false} },
      computed: {
         subject: function() { return (mnm._data.toSave[this.draft.msgid] ||
                                       mnm._data.mo[this.draft.msgid].SubHead).Subject },
         msg: function() { return mnm._data.toSave[this.draft.msgid] || mnm._data.mo[this.draft.msgid] },
         attach: function() { return mnm._data.mo[this.draft.msgid].SubHead.Attach },
      },
   });
</script>

<script type="text/x-template" id="mnm-adrsbkinput">
   <div @click="emitAlias(menu.selectId($event.target.id))"
        class="adrsbkinput">
      <input @focus="menu.placeEl($el, type, $event.target.value)"
             @blur ="menu.hideEl()"
             @input="menu.search(type, $event.target.value)"
             @keydown.down ="emitAlias(menu.selectItem(1))"
             @keydown.up   ="emitAlias(menu.selectItem(-1))"
             @keydown.esc  ="emitAlias(menu.selectNone())"
             @keyup.enter  ="enterKey"
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
      methods: {
         enterKey: function(iEvt) {
            var aVal = this.menu.selectOrClear();
            this.emitAlias(aVal);
            if (aVal !== null) {
               iEvt.stopPropagation();
               iEvt.preventDefault();
            }
         },
         emitAlias: function(iVal) {
            if (iVal === null)
               return;
            this.$el.firstChild.value = iVal;
            this.$emit('alias', iVal);
         },
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
            if (iParent.lastChild !== this.$el || iQuery !== this.query) {
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
            this.select = iList.findIndex(function(c) { return c === this.query }, this);
         },
         selectNone: function() {
            this.select = -1;
            return this.query;
         },
         selectId: function(iId) {
            if (!iId)
               return null;
            this.select = parseInt(iId.substring(3), 10);
            return this.list[this.select];
         },
         selectOrClear: function() {
            if (this.select === -1 && this.list.length)
               return this.selectItem(1);
            this.clear();
            return null;
         },
         selectItem: function(iDirection) {
            if (this.select === -1 && iDirection === -1)
               this.select = this.list.length;
            this.select += iDirection;
            if (this.select === this.list.length || this.select === -1)
               return this.selectNone();
            return this.list[this.select];
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
            this.$el.style.height = 'auto'; // causes scrolling parent to zero scrollTop
            this.$el.style.height = this.$el.scrollHeight+4 + 'px';
            this.$emit('resize');
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-markdown">
   <div class="message" v-html="mdi.render(src, env)"></div>
</script><script>
   Vue.component('mnm-markdown', {
      template: '#mnm-markdown',
      props: {src:String, msgid:String, allslides:Boolean,
              formfill:Object, formreply:[Object,String], atchasff:Function},
      data: function() { return {hasDeck:false} },
      computed: {
         mdi: function() { return mnm._mdi },
      },
      watch: {
         allslides: function(i) {
            this.env.allSlides = i;
            this.$forceUpdate();
         },
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
         this.env = { allSlides:this.allslides, fillMap:{}, parent:this, formview:null,
                      thisVal: this.formreply && this.formreply !== 'Q' ? this.msgid
                                                                        : this.msgid.substr(-12) };
         if (this.formfill)
            for (var a in this.formfill)
               Vue.set(this.env.fillMap, a, this.formfill[a]);
      },
      beforeDestroy: function() { if (this.env.formview) this.env.formview.destroy() },
      mounted:       function() { this.onRender() },
      updated:       function() { this.onRender() },
      methods: {
         onRender: function() {
            if (this.env.formview)
               this.env.formview.remount();
            if (this.formreply)
               return;
            var aHasDeck = !!this.$el.querySelector('div.md-deck');
            if (this.hasDeck !== aHasDeck) {
               this.$emit('hasdeck', aHasDeck);
               this.hasDeck = aHasDeck;
            }
         },
      },
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
                 class="btn btn-icon btn-floatr"><span uk-icon="commenting"></span></button>
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
   <div uk-dropdown="mode:click; offset:2" :toggle="toggle"
        class="widthmin33 menu-bg dropdown-scroll"
        :class="{'message-edit':toggle}"
        @hidden="$refs.viewer.close()" @click="$refs.viewer.close()">
      <form :action="'/t/+' + encodeURIComponent(upname)"
            method="POST" enctype="multipart/form-data"
            onsubmit="mnm.Upload(this); this.reset(); return false;"
            class="dropdown-scroll-item">
         <div class="uk-float-right uk-text-small">ATTACHABLE FILES</div>
         <input @input="vis = !!(upname = $event.target.value.substr(12))" type="file"
                name="filename" required>
         <div :class="{vishide: !vis}" style="margin-top:0.5em">
            <input v-model="upname"
                   placeholder="Alt Name" type="text"
                   style="width:60%">
            <button @click="vis = false" type="submit"
                    :disabled="!upname"
                    title="Copy to attachable files"
                    class="btn btn-icon"><span uk-icon="push"></span></button>
            <button @click="vis = false" type="reset"
                    class="btnx"><span>&times;</span></button>
         </div>
      </form>
      <ul uk-tab class="dropdown-scroll-item" style="margin-top:0"><li style="display:none"></li>
         <li v-for="aKey in ['Date','Name']"
             :class="{'uk-active': aKey === mnm._data.cs.Sort.t}">
            <a @click.prevent="mnm.SortSelect('t', aKey)" href="#">{{aKey}}</a>
         </li></ul>
      <mnm-viewer ref="viewer" :toggle="toggle"/>
      <ul class="uk-list uk-list-divider dropdown-scroll-list">
         <li v-for="aFile in mnm._data.t" :key="aFile.Name">
            <mnm-date :iso="aFile.Date" hms="hm"/>
            <button v-if="toggle"
                    @click="$emit('attach', 'upload/'+aFile.Name)"
                    title="Attach file"
                    class="btn btn-icon"><mnm-paperclip/></button>
            <a :href="'/t/=' + encodeURIComponent(aFile.Name)" download
               title="Download file">
               <span uk-icon="download">&nbsp;</span></a>
            <span v-if="!mnm._viewerType(null, aFile.Name)">
               &nbsp;<span class="icon-blank"></span>{{aFile.Name}}</span>
            <a v-else
               @click.stop.prevent="$refs.viewer.open(null, aFile.Name, $event.currentTarget)"
               :href="'/t/' + encodeURIComponent(aFile.Name)">
               <span uk-icon="triangle-left">&nbsp;</span>{{aFile.Name}}</a>
            <div class="uk-float-right">
               {{aFile.Size}}
               <form v-if="!toggle"
                     :action="'/t/-' + encodeURIComponent(aFile.Name)" method="POST"
                     onsubmit="mnm.Upload(this); return false;"
                     style="display:inline!important">
                  <button title="Erase file"
                          class="btn btn-iconred"><span uk-icon="trash"></span></button>
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
   <div uk-dropdown="mode:click; offset:2" :toggle="toggle"
        class="widthmin33 menu-bg dropdown-scroll"
        :class="{'message-edit':toggle}"
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
                 class="btn btn-icon"><span uk-icon="pencil"></span></button>
      </form>
      <ul uk-tab class="dropdown-scroll-item" style="margin-top:0"><li style="display:none"></li>
         <li v-for="aKey in ['Date','Name']"
             :class="{'uk-active': aKey === mnm._data.cs.Sort.f}">
            <a @click.prevent="mnm.SortSelect('f', aKey)" href="#">{{aKey}}</a>
         </li></ul>
      <ul class="uk-list uk-list-divider dropdown-scroll-list">
         <template v-for="aSet in mnm._data.f">
         <li v-for="aFile in aSet.Revs" :key="aSet.Name+'.'+aFile.Id">
            <mnm-date :iso="aFile.Date" hms="hm"/>
            <button v-if="toggle"
                    @click="$emit('attach', 'form/'+aSet.Name+'.'+aFile.Id)"
                    title="Attach form"
                    class="btn btn-icon"><mnm-paperclip/></button>
            <a @click.stop.prevent="revOpen(aSet.Name,aFile.Id,$event.currentTarget)"
               :ref="aSet.Name+'.'+aFile.Id" href="#">
               <span uk-icon="triangle-left">&nbsp;</span>{{aSet.Name}}.{{aFile.Id}}</a>
            <form v-if="!toggle"
                  :action="'/f/-' + encodeURIComponent(aSet.Name+'.'+aFile.Id)" method="POST"
                  onsubmit="mnm.Upload(this); return false;"
                  style="float:right">
               <button @click="revDelete(aSet.Name,aFile.Id)"
                       title="Erase form"
                       class="btn btn-iconred"><span uk-icon="trash"></span></button>
            </form>
         </li></template></ul>
      <div v-show="setName"
           class="uk-card uk-card-default dropdown-scroll uk-width-1-1 menu-bg"
           :class="{'message-edit':toggle}"
           style="position:absolute" :style="{top:editTop, right:editRight}"
           @click.stop>
         <div class="uk-text-right uk-text-small dropdown-scroll-item"
              >{{(setName +'.'+ fileId).toUpperCase()}}</div>
         <div v-show="!mnm._data.fo"
              class="uk-text-center dropdown-scroll-list message-bg" style="padding:0.5em">
            <span uk-icon="future"></span></div>
         <div v-show="mnm._data.fo"
              class="pane-clip" style="margin-top:-1.5em">
            <span v-if="!toggle"
                  @click="showCode"
                  class="uk-link"><tt>{...}</tt></span>
            &nbsp;
            <div class="uk-text-right uk-text-small dropdown-scroll-item">&nbsp;{{parseError}}</div>
            <div class="pane-slider" :class="{'pane-slider-rhs':codeShow}">
               <div class="pane-scroller message-bg" style="min-height:1px">
                  <plugin-vfg :schema="formDef" :model="{}" :options="{}"/></div
              ><div @scroll="codePos = $refs.codepane.scrollTop"
                    ref="codepane"
                    class="pane-scroller">
                  <mnm-textresize @input.native="editCode"
                                  @resize="$refs.codepane.scrollTop = codePos"
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
                    class="btn btn-icon"><span uk-icon="copy"></span></button>
         </form>
      </div>
   </div>
</script><script>
   Vue.component('mnm-forms', {
      template: '#mnm-forms',
      props: {toggle:String},
      data: function() {
         return {upname:'', dupname:'', setName:'', fileId:'', codePos:0, codeShow:false, dupShow:'',
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
            mnm._data.cs.Sort.f = i;
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
            this.editRight = (iEl.offsetParent.offsetWidth - iEl.offsetLeft) +'px';
            this.codePos = 0;
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
                  this.toSave[aKey] = {timer:setTimeout(fDing, mnm._saveWaitTime, this), data:''};
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
   <div uk-dropdown="mode:click" :toggle="toggle"
        @show="showErr = !svc && mnm._data.errorFlag && !(mnm._data.errorFlag = false)"
        class="widthmin25 menu-bg dropdown-scroll">
      <button v-if="!svc"
              @click="showErr = !showErr"
              :disabled="!mnm._data.errors.length"
              style="float:left; margin-right:1em"
              title="Toggle errors list"
              class="uk-button uk-button-link"><span uk-icon="warning"></span></button>
      <button v-if="!svc"
              v-show="!showErr"
              @click="mnm.NoticeSeen(mnm._data.nlo[0].MsgId)"
              :disabled="!mnm._data.nlo.length || mnm._data.nlo[0].Seen > 0"
              title="Mark all as seen"
              class="btn btn-icon btn-floatr dropdown-scroll-item"><span uk-icon="check"></span></button>
      <div style="min-height:2em; font-style:oblique; color:#1e87f0"><!--uk-light workaround-->
         <span v-for="aType in [['i', 'invites']]"
               v-show="!showErr"
               @click="$data[aType[0]] = !$data[aType[0]]"
               style="margin-right:0.5em; cursor:pointer">
            <span :class="{vishide: !$data[aType[0]]}">&bull; </span>
            {{ aType[1] }}
         </span>
      </div>
      <div v-show="showErr"
           class="dropdown-scroll-list notice">
         <div v-for="aErr in mnm._data.errors" :key="aErr.Date">
            <div style="float:left; font-style:oblique">!</div>
            <div style="margin-left:1em">
               <div style="float:right"><mnm-date :iso="aErr.Date" ymd="md" hms="hm"/></div>
               {{aErr.Err}}
            </div>
         </div></div>
      <div v-show="!showErr"
           class="dropdown-scroll-list notice">
         <div v-if="!mnm._data.nlo.length"
              style="text-align:center">No notices yet</div>
         <div v-for="aNote in mnm._data.nlo" :key="aNote.MsgId"
              v-show="$data[aNote.Type]"
              @click="$set(aNote, 'open', !aNote.open)"
              :class="{'notice-seen':aNote.Seen, 'notice-hasblurb':aNote.Blurb}">
            <div style="float:left; font-style:oblique">{{aNote.Type}}</div>
            <div style="margin-left:1em">
               <div style="float:right"><mnm-date :iso="aNote.Date" ymd="md" hms="hm"/></div>
               {{aNote.Alias}}
               <template v-if="aNote.Gid"
                         >- {{aNote.Gid}}</template>
               <span v-show="aNote.Blurb && !aNote.open">. . .</span>
               <div v-show="aNote.open">{{aNote.Blurb}}</div>
            </div>
         </div></div>
   </div>
</script><script>
   Vue.component('mnm-notice', {
      template: '#mnm-notice',
      props: {svc:String, toggle:String},
      data: function() { return { i:true, showErr:false } },
      computed: {
         mnm: function() { return mnm },
      },
   });
</script>

<script type="text/x-template" id="mnm-adrsbk">
   <div uk-dropdown="mode:click; offset:2; pos:bottom-right"
        class="widthmin40 menu-bg dropdown-scroll">
      <ul uk-tab class="uk-child-width-expand dropdown-scroll-item" style="margin-top:0"
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
                             class="btn btn-icon"><span uk-icon="forward"></span></button>
                  </td>
                  <td>{{aPing.Text}}</td>
                  <td><mnm-pingresponse :ping="aPing"/></td>
               </tr></table></li>
         <li>
            <form onsubmit="return false"
                  style="width:76%; margin: 0 auto; display:table">
               <span @click="(hasGroup = !hasGroup) || (draft.gid = '')"
                     title="Include group in invite"
                     class="uk-link"><span uk-icon="world"></span></span>
               <div :class="{'vishide': !hasGroup}"
                    style="width:calc(50% - 2em); display:inline-block; vertical-align:top">
                  <input v-model="draft.gid"
                         name="gid" autocomplete="off" type="text"
                         placeholder="Group (<%.aliasMin%>+)"
                         class="width100">
                  <br>
                  <mnm-adrsbkinput @keyup.enter.native="setField('gid', $event.target)"
                                   @keydown.enter.native.prevent=""
                                   :type="2"
                                   placeholder="Choose group"
                                   class="width100"/>
               </div>
               <div style="width:calc(50% - 2em); display:inline-block; vertical-align:top">
                  <input v-model="draft.to"
                         name="to" autocomplete="off" type="text"
                         placeholder="To (<%.aliasMin%>+ characters)"
                         class="width100">
                  <mnm-adrsbkinput v-show="hasGroup"
                                   @keyup.enter.native="setField('to', $event.target)"
                                   @keydown.enter.native.prevent=""
                                   :type="1"
                                   placeholder="Choose contact"
                                   class="width100"/>
               </div>
               <button @click="startPing()"
                       :disabled="!validDraft || mnm._data.cf.Error || !mnm._data.cf.Uid"
                       title="New draft invitation"
                       class="btn btn-icon"><span uk-icon="pencil"></span></button>
            </form>
            <div v-show="mnm._data.ps.length === 0"
                 style="margin-top:0.5em; text-align:center; font-style:italic">
               <span v-if="!mnm._data.cf.Uid"
                     style="color:crimson"
                     >Open the <span uk-icon="settings"></span> menu and check the Site Address!</span>
               <span v-else-if="mnm._data.cf.Error"
                     style="color:crimson"
                     >Open the <span uk-icon="settings"></span> menu and update your Alias!</span>
               <template v-else
                         >To draft an invitation to someone, add their alias here.</template>
            </div>
            <table class="uk-table uk-table-small" style="margin:0">
               <tr><th>To / (Group)</th> <th></th> <th>Message</th> <th></th></tr>
               <tr v-for="aRec in mnm._data.ps" :key="rowId(aRec)">
                  <td>{{aRec.Alias}}<br>{{aRec.Gid && '('+aRec.Gid+')'}}</td>
                  <td><span v-if="aRec.Queued"
                            title="Awaiting link to server"
                            uk-icon="bolt"></span>
                      <button v-else
                              @click="sendPing(aRec)"
                              title="Send invitation"
                              class="btn btn-icon"><span uk-icon="forward"></span></button></td>
                  <td><textarea @input="editPing(aRec, $event.target.value)"
                                :value="(mnm._data.toSavePs[rowId(aRec)] || aRec).Text"
                                :disabled="aRec.Queued"
                                cols="40" rows="3" maxlength="120"></textarea></td>
                  <td><button v-if="!aRec.Queued"
                              @click="mnm.PingDiscard({to:aRec.Alias, gid:aRec.Gid})"
                              title="Discard draft"
                              class="btn btn-iconred"><span uk-icon="trash"></span></button></td>
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
                       class="btn btn-icontxt"><span>o/</span></button>
            </form>
            <div v-show="mnm._data.ot.length === 0"
                 style="margin-top:0.5em; text-align:center; font-style:italic"
                 >To notify a contact whenever you're online, add them here.</div>
            <table class="uk-table uk-table-small" style="margin:0">
               <tr><th>Date</th> <th>To</th> <th></th></tr>
               <tr v-for="aOhi in mnm._data.ot">
                  <td><mnm-date v-if="aOhi.Date !== 'pending'"
                                :iso="aOhi.Date"/>
                      <span v-else
                            title="Awaiting link to server"
                            uk-icon="bolt"></span>
                  </td>
                  <td>{{aOhi.Alias}}</td>
                  <td><button @click="mnm.OhiDrop(aOhi.Uid)"
                              :disabled="aOhi.Date === 'pending'"
                              title="Stop notifying contact"
                              class="btn btn-iconred"><span uk-icon="trash"></span></button></td>
               </tr></table></li>
      </ul>
   </div>
</script><script>
   Vue.component('mnm-adrsbk', {
      template: '#mnm-adrsbk',
      data: function() { return {draft:{to:'', gid:''}, hasGroup:false} },
      computed: {
         mnm: function() { return mnm },
         validDraft: function() {
            if (this.draft.to.length < <%.aliasMin%> ||
                this.draft.gid && this.draft.gid.length < <%.aliasMin%>)
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
         setField: function(iKey, iInput) {
            if (iInput.value) {
               this.draft[iKey] = iInput.value;
               iInput.value = '';
            }
            iInput.form.elements[iKey].focus();
         },
         startPing: function() {
            mnm.PingSave({alias:mnm._data.cf.Alias, to:this.draft.to, gid:this.draft.gid});
            this.draft.to = '';
         },
         editPing: function(iRec, iText) {
            var aKey = this.rowId(iRec);
            if (!(aKey in mnm._data.toSavePs))
               Vue.set(mnm._data.toSavePs, aKey, {timer:null, Text:''});
            var aToSave = mnm._data.toSavePs[aKey];
            if (!aToSave.timer)
               aToSave.timer = setTimeout(fDing, mnm._saveWaitTime);
            aToSave.Text = iText;
            function fDing() {
               aToSave.timer = null;
               mnm.PingSave({text:aToSave.Text, alias:iRec.MyAlias, to:iRec.Alias, gid:iRec.Gid});
            }
         },
         sendPing: function(iRec) {
            var aToSave = mnm._data.toSavePs[this.rowId(iRec)];
            if (aToSave && aToSave.timer) {
               clearTimeout(aToSave.timer);
               aToSave.timer = null;
               mnm.PingSave({text:aToSave.Text, alias:iRec.MyAlias, to:iRec.Alias, gid:iRec.Gid});
            }
            mnm.PingSend(iRec.Qid);
         },
         setOhiTo: function(iInput) {
            var aOk = iInput.value && iInput.value in mnm._data.adrsbkmenuId;
            iInput.form.elements[1].disabled = !aOk;
            iInput.form.elements[1].firstChild.innerText = aOk ? 'o/ '+ iInput.value : 'o/';
            iInput.form.elements[1].value = aOk ? mnm._data.adrsbkmenuId[iInput.value] : '';
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-pingresponse">
   <div class="pingresponse">
      <div v-if="ping.Response">
         <a v-if="ping.Response.Tid"
            onclick="mnm.NavigateLink('Response', this.href); return false"
            title="Find message with response"
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

<script type="text/x-template" id="mnm-nodes">
   <div uk-dropdown="mode:click; offset:2; pos:bottom-right"
        class="widthmin20 menu-bg dropdown-scroll">
      <form onsubmit="return false" @submit="name = ''"
            class="dropdown-scroll-item">
         <span v-show="mnm._data.cn.Xfer > 0"
               >{{Math.round(mnm._data.cn.Xfer / 1024/1024) || '< 1'}} MB sent</span>
         <div class="uk-float-right uk-text-small">REPLICAS</div>
         <br>
         <input v-model="mnm._data.cn.Addr"
                placeholder="Target Address" type="text"
                style="width:60%">
         <input v-model="mnm._data.cn.Pin"
                placeholder="Target Pin" type="text"
                style="width:calc(40% - 0.5em)">
         <br>
         <input v-model="name"
                :disabled="anyInProgress"
                placeholder="New replica name" type="text"
                style="width:60%">
         <button @click="mnm.NodeAdd(mnm._data.cn.Addr, mnm._data.cn.Pin, name)"
                 :disabled="anyInProgress || !mnm._data.cn.Addr || !validPin || !validName"
                 :title="'Replicate <%.TitleJs%>'"
                 class="btn btn-icon btn-floatr"><span uk-icon="laptop"></span></button>
      </form>
      <div style="margin-top:0.5em" class="dropdown-scroll-list">
         <div v-for="aNode in mnm._data.cf.NodeSet" :key="aNode.Name">
            {{aNode.Name}}
            <div class="uk-float-right">
               <button v-if="aNode.Status === 'p'.charCodeAt(0) ||
                             aNode.Status === 'l'.charCodeAt(0) ||
                             aNode.Status === 'r'.charCodeAt(0)"
                       @click="mnm.NodeAdd(mnm._data.cn.Addr, mnm._data.cn.Pin, aNode.Name)"
                       :disabled="!mnm._data.cn.Addr || !validPin"
                       :title="'Replicate <%.TitleJs%>'"
                       class="btn btn-icon"><span uk-icon="laptop"></span></button>
               {{status(aNode.Status)}}
            </div>
         </div></div>
   </div>
</script><script>
   Vue.component('mnm-nodes', {
      template: '#mnm-nodes',
      data: function() { return {name:''} },
      computed: {
         mnm: function() { return mnm },
         validPin: function() {
            return mnm._data.cn.Pin.replace(/ /g, '').length === 9;
         },
         validName: function() {
            if (!this.name)
               return false;
            return 0 > mnm._data.cf.NodeSet.findIndex(function(c) { return c.Name === this.name }, this);
         },
         anyInProgress: function() {
            return 0 <= mnm._data.cf.NodeSet.findIndex(function(c) {
               return c.Status !== 'a'.charCodeAt(0) && c.Status !== 'd'.charCodeAt(0);
            });
         },
      },
      methods: {
         status: function(i) {
            switch (i) {
            case 'p'.charCodeAt(0): return 'Pending';
            case 's'.charCodeAt(0): return 'Requested';
            case 'l'.charCodeAt(0): return 'Accepted';
            case 'r'.charCodeAt(0): return 'Ready';
            case 'a'.charCodeAt(0): return 'Active';
            case 'd'.charCodeAt(0): return 'Defunct';
            default:                return 'Unknown';
            }
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-svcadd">
   <div uk-dropdown="mode:click; offset:2; pos:bottom-right"
        class="widthmin20 menu-bg dropdown-static"
        @hidden="addr = name = alias = sent = lpin = loginperiod = null">
      <div class="uk-float-right uk-text-small">ADD ACCOUNT</div>
      <form :action="'/v/+' + encodeURIComponent(name)"
            method="POST" enctype="multipart/form-data"
            onsubmit="mnm.Upload(this); return false;">
         <input type="hidden" name="filename" :value="JSON.stringify($data)">
         <button @click="sent = true"
                 :disabled="!(name  && name.length  >= <%.serviceMin%> && nameUnused &&
                              alias && alias.length >= <%.aliasMin%> &&
                              addr  && addr.length  >= 2 && (addr[0] === '+' || addr[0] === '=') &&
                              !isNaN(loginperiod))"
                 title="Register new account"
                 class="btn btn-icon"><span uk-icon="forward"></span></button>
         <span v-show="sent && name && !nameUnused"
               class="uk-text-small">Done!</span>
         <input v-model="addr"
                placeholder="Site Address" type="text"
                title="Starts with '+' or '=' and may end with ':number'"
                class="width100">
         <input v-model="alias"
                placeholder="Your Name/Alias (<%.aliasMin%>+ characters)" type="text"
                title="Name by which other site members know you"
                class="width100">
         <input v-model="name"
                placeholder="Account Title (<%.serviceMin%>+ characters)" type="text"
                title="Private label for the new account"
                class="width100">
         <!--todo input v-model="lpin"
                @input="loginperiod = mnm._stringToSeconds($event.target.value)"
                placeholder="(Pd days:hh:mm:ss)"                  size="19" type="text">
         <div v-show="loginperiod"
              style="float:right">{{loginperiod}} sec</div -->
      </form>
   </div>
</script><script>
   Vue.component('mnm-svcadd', {
      template: '#mnm-svcadd',
      data: function() { return {addr:null, name:null, alias:null, sent:null, lpin:null, loginperiod:null} },
      computed: {
         nameUnused: function() {
            return !mnm._data.v.find(function(c){ return c.Name === this.name }, this);
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-svccfg">
   <div uk-dropdown="mode:click; offset:-4; pos:left-top"
        class="widthmin20 menu-bg dropdown-static">
      <div class="uk-float-right uk-text-small">SETTINGS</div>
      <form onsubmit="return false">
         <button @click="sendUpdate"
                 :disabled="!(addr || alias || historylen >= 0 || loginperiod >= 0)
                            || isNaN(historylen) || isNaN(loginperiod)"
                 title="Update settings"
                 class="btn btn-icon"><span uk-icon="forward"></span></button>
         <table class="svccfg">
            <tr><td>Thread history</td><td>
               {{mnm._data.cf.HistoryLen}}
               <input v-model="hlin"
                      @input="historylen = parseInt($event.target.value || '-1')"
                      placeholder="Length (4 to 1024)" type="text"
                      class="width100"></td></tr>
            <tr><td>Site</td><td>
               {{mnm._data.cf.Addr}}
               <input v-if="!mnm._data.cf.Uid"
                      v-model="addr"
                      placeholder="Site Address" type="text"
                      title="Starts with '+' or '=' and may end with ':number'"
                      class="width100">
               <div v-else
                    >{{mnm._data.cf.Verify ? 'V' : 'Not v'}}erified</div></td></tr>
            <!--todo tr><td>Login Period   </td><td>{{mnm._secondsToString(mnm._data.cf.LoginPeriod)}}<br>
               <input v-model="lpin"
                      @input="loginperiod = toSeconds($event.target.value)"
                      placeholder="New days:hh:mm:ss" size="25" type="text"></td></tr -->
            <tr><td>Alias<br>{{mnm._data.cf.Error && '(taken)'}}</td><td>
               <input v-if="mnm._data.cf.Uid && !mnm._data.cf.Alias"
                      v-model="alias"
                      placeholder="Your Name/Alias (<%.aliasMin%>+ chars)" type="text"
                      title="Name by which other site members know you"
                      class="width100">
               {{mnm._data.cf.Alias ||
                 mnm._data.cf.Error.slice('AddAlias: alias '.length, -' already taken'.length)}}</td></tr>
            <tr><td>Uid</td><td>
               {{mnm._data.cf.Uid}}</td></tr>
         </table>
      </form>
   </div>
</script><script>
   Vue.component('mnm-svccfg', {
      template: '#mnm-svccfg',
      data: function() { return {hlin:null, addr:null, alias:null, lpin:null, historylen:-1, loginperiod:-1} },
      computed: { mnm: function() { return mnm } },
      methods: {
         toSeconds: function(i) {
            var a = mnm._stringToSeconds(i);
            return a === null ? -1 : a;
         },
         sendUpdate: function() {
            mnm.ConfigUpdt(this.$data);
            this.hlin = this.addr = this.lpin = null;
            this.historylen = this.loginperiod = -1;
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-settings">
   <div uk-dropdown="mode:click; offset:2; pos:bottom-right"
        class="widthmin20 menu-bg dropdown-static">
      <div class="uk-text-right uk-text-small">SETTINGS</div>
      <form action="/l/" method="POST"
            onsubmit="mnm.Upload(this); return false;"
            style="float:left">
         <button title="Accept replicas"
                 class="btn btn-icon" style="padding-left:4px">
            <span uk-icon="laptop"></span><span uk-icon="arrow-left"></span></button></form>
      <div style="margin-left:3.5em">
         <span v-show="!mnm._data.l.Pin"
               >(not accepting replicas)</span>
         <span v-show="mnm._data.l.Pin"
               >Address {{mnm._data.l.Addr}}<br>
                Pin {{mnm._data.l.Pin}}</span>
      </div>
   </div>
</script><script>
   Vue.component('mnm-settings', {
      template: '#mnm-settings',
      computed: {
         mnm: function() { return mnm },
      },
   });
</script>

<script type="text/x-template" id="mnm-paperclip">
   <span class="paperclip">&#128206;</span>
</script><script>
   Vue.component('mnm-paperclip', {
      template: '#mnm-paperclip',
   });
</script>

<script type="text/x-template" id="mnm-tabs">
   <ul uk-tab style="margin-top:0; margin-bottom:0;"><li style="display:none"></li>
      <template v-for="(aTabs, aI) in set">
         <li v-for="(aTerm, aJ) in aTabs"
             :class="{'uk-active': aI === state.PosFor && aJ === state.Pos}">
            <a @click.prevent="mnm.TabSelect({type:state.Type, posfor:aI, pos:aJ})" href="#">
               {{ getLabel(aTerm) }}
               <span v-if="aI > 0"
                     @click.prevent.stop="mnm.TabDrop(state.Type)"
                     :class="{vishide: aI !== state.PosFor || aJ !== state.Pos}">&times;</span>
            </a>
         </li></template></ul>
</script><script>
   Vue.component('mnm-tabs', {
      template: '#mnm-tabs',
      props: {set:Array, state:Object},
      computed: { mnm: function() { return mnm } },
      methods: {
         getLabel: function(iTerm) {
            if (this.state.Type === 1)
               return iTerm.Label || iTerm.Term;
            switch (iTerm.Term.charAt(0)) {
            case '&': return iTerm.Label ? '\u2992 '+ iTerm.Label : iTerm.Term;
            case '#': return '# '+ iTerm.Term.substring(1);
            case ':': return 're '+ iTerm.Term.substring(1);
            default:  return iTerm.Term;
            }
         },
      },
   });
</script>

<script type="text/x-template" id="mnm-viewer">
   <div v-show="kind"
        class="uk-card uk-card-default viewer dropdown-scroll menu-bg"
        :class="{'message-edit': toggle || noparent}"
        style="position:absolute; z-index:3" :style="{top:editTop, right:editRight, left:editLeft}"
        @click.stop>
      <a v-show="kind !== 'form'"
         :href="src" :target="(src[0] === '/' ? 'mnm_upl_' : 'mnm_atc_<%.TitleJs%>_') + file"
         title="Open file in new tab">
         <span uk-icon="expand"></span></a>
      <div class="uk-text-small viewer-name dropdown-scroll-item">{{title}}</div>
      <div v-if="kind === 'form'"
           class="viewer-form">
         <div v-show="!(file in mnm._data.ao)"
              class="uk-text-center">
            <span uk-icon="future"></span></div>
         <plugin-vfg v-show="file in mnm._data.ao"
                     :schema="formDef" :model="{}" :options="{}"/>
      </div>
      <div v-else-if="kind === 'img'"
           class="dropdown-scroll-item" style="background-color:whitesmoke"><!--todo checkerboard-->
         <img :src="src"></div>
      <div v-else-if="kind === 'video'"
           class="dropdown-scroll-item">
         <video :src="src" controls>Player not available.</video></div>
      <div v-else-if="kind === 'audio'"
           class="dropdown-scroll-item">
         <audio :src="src" controls>Player not available.</audio></div>
      <div v-show="kind === 'page'"
           class="dropdown-scroll-item">
         <iframe v-if="srcPage"
                 x-load="$event.target.contentWindow.onbeforeunload = //todo prevent PDF.js error
                    function(iEv) { iEv.currentTarget.history.replaceState(null, null, srcPage) }"
                 :src="srcPage"></iframe></div>
   </div>
</script><script>
   Vue.component('mnm-viewer', {
      template: '#mnm-viewer',
      props: {toggle:String, noparent:Boolean},
      data: function() { return {file:'', kind:'', title:'', src:'', srcPage:'',
                                 editTop:'', editRight:null, editLeft:null} },
      computed: {
         mnm: function() { return mnm },
         formDef: function() {
            if (this.kind !== 'form')
               return null;
            if (!(this.file in mnm._data.ao))
               return {};
            try {
               return JSON.parse(mnm._data.ao[this.file]);
            } catch(aErr) {
               return {fields:[ {type:"label", label:aErr.message.slice(12, -17)} ]};
            }
         },
      },
      methods: {
         close: function() { this.kind = '' },
         open: function(iSvc, iId, iEl, iRhs) {
            this.file = iId;
            this.title = (iSvc ? iId.substring(iId.indexOf('_')+3) : iId).toUpperCase();
            this.src = (iSvc ? '?an=' : '/t/') + encodeURIComponent(iId);
            this.editTop = iEl.offsetTop +'px';
            if (iRhs)
               this.editLeft = (iEl.offsetLeft + 20) +'px'; // 20 == icon width
            else
               this.editRight = (iEl.offsetParent.offsetWidth - iEl.offsetLeft) +'px';
            this.kind = mnm._viewerType(iSvc, iId);
            if (this.kind === 'page')
               this.srcPage = this.src;
            else if (this.kind === 'form' && !(iId in mnm._data.ao))
               mnm.AttachOpen(iId);
         },
      },
      created: function() {
         if (!this.noparent)
            return;
         var that = this;
         document.addEventListener('click', function() { that.kind = '' });
      },
      components: { 'plugin-vfg': VueFormGenerator.component },
   });

   mnm._viewerType = function(iSvc, iId) {
      if (iSvc && iId.charAt(iId.indexOf('_')+1) === 'f')
         return 'form';
      var aExt = iId.lastIndexOf('.') + 1;
      switch (aExt ? iId.substring(aExt).toLowerCase() : '') {
      case 'jpg': case 'jpeg': case 'png': case 'gif': case 'svg':
         return 'img';
      case 'mp4': case 'webm':
         return 'video';
      case 'mp3': case 'wav':
         return 'audio';
      case 'htm': case 'html': case 'pdf': case 'txt':
         return 'page';
      }
      return null;
   };
</script>

<script>
;(function() {
   var sChange = 0;
   var sTemp = {ml:null, mo:null};
   var sMsglistPos = 0;

   mnm._saveWaitTime = 12*1000; // milliseconds
   mnm._isLocal = '<%.TitleJs%>' === 'local';
   mnm._mdi = markdownit();
   mnm._lastPreview = '';
   mnm._tabsStdService = <%.tabsStdService%>;
   mnm._tabsStdThread = <%.tabsStdThread%>;
   mnm._data = {
   // global
      v:[], g:[], l:{Pin:''}, t:[], f:[], fo:'', nlo:[], // fo populated by f requests
   // per client
      cs:{SvcTabs:{Pinned:[], Terms:[]}, ThreadTabs:{Terms:[]}, Sort:{}},
      ohiFrom: !mnm._isLocal, //todo move to cs
      adrsbkmenuId: {},
      errors: [], errorFlag: false,
   // per service
      cf:{NodeSet:[]}, cn:{}, tl:[], ffn:'', // ffn derived from tl
      ps:[], pt:[], pf:[], gl:[], ot:[], of:null,
      toSavePs:{}, // populated locally //todo rename toSave -> toSaveMo
   // per thread
      cl:[[],[]], al:[], ao:{}, ml:[], mo:{}, // ao populated by an requests
      toSave:{}, draftRefs:{}, // populated locally
   };

   var sApp = new Vue({
      template: '#mnm-main',
      data: mnm._data,
      methods: {
         msglistGetScroll: function() {
            sMsglistPos = sApp.$refs.msglist.scrollTop;
         },
         msglistSetScroll: function() {
            sApp.$refs.msglist.scrollTop = sMsglistPos;
         },
         tabSearch: function(iText, iState) {
            if (iText.length === 0)
               return;
            if (iState.Pinned)
               for (var a=0; a < iState.Pinned.length; ++a)
                  if (iState.Pinned[a].Term === iText)
                     return;
            for (var a=0; a < iState.Terms.length; ++a)
               if (iState.Terms[a].Term === iText)
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
         location:  function() { return location },
         svcTabset: function() {
            return [[], mnm._data.cs.SvcTabs.Pinned, mnm._data.cs.SvcTabs.Terms];
         },
         msgTabset: function() {
            var aT = mnm._data.cs.ThreadTabs;
            return aT ? [mnm._tabsStdThread, [], aT.Terms] : [];
         },
         msgTitle: function() { // mirrors slib/thread.go _updateSearchDoc()
            var aLastN = -1, aHasDraft = false;
            for (var a = mnm._data.ml.length-1; a >= 0; --a) {
               if (mnm._data.ml[a].From === '' || !aHasDraft) {
                  aLastN = a;
                  aHasDraft = mnm._data.ml[a].From === '';
               }
            }
            if (aLastN === -1)
               return null;
            return mnm._data.ml[aLastN].Subject || this.msgSubjects[this.msgSubjects.length-1].name;
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
         msgTags: function() {
            var aSet = {};
            mnm._data.ml.forEach(function(cMsg) {
               if (cMsg.Tags)
                  cMsg.Tags.forEach(function(cId) { aSet[cId] = true });
            });
            var aList = [];
            mnm._data.g.forEach(function(cTag) {
               if (aSet[cTag.Id])
                  aList.push(cTag);
            });
            return aList;
         },
         svcSelf: function() {
            return mnm._data.v.find(function(c) { return c.Name === '<%.TitleJs%>' }) || {};
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
         iTokens[iIdx].attrs.push(['onclick', "mnm.NavigateLink(this.innerText,this.href);return false"]);
      } else if (!sUrlStart.test(aHref[1])) {
         aHref[1] = '?ad='+ aHref[1].replace(/^this_/, iEnv.thisVal +'_');
         iTokens[iIdx].attrs.push(['download', '']);
         //todo add download icon and viewer
      }
      return iSelf.renderToken(iTokens, iIdx, iOptions);
   };

   var sMdiRenderImg = mnm._mdi.renderer.rules.image;
   mnm._mdi.renderer.rules.image = function(iTokens, iIdx, iOptions, iEnv, iSelf) {
      var aAlt = iSelf.renderInlineAsText(iTokens[iIdx].children, iOptions, iEnv);
      var aSrc = iTokens[iIdx].attrs[iTokens[iIdx].attrIndex('src')];
      var aParam = aSrc[1].replace(/^this_/, iEnv.thisVal +'_');
      if (aAlt.charAt(0) === '?') {
         if (!iEnv.formview)
            iEnv.formview = new mnm._FormViews(iEnv);
         var aId = iEnv.formview.make(decodeURIComponent(aParam));
         return '<component'+ iSelf.renderAttrs({attrs:[['id',aId]]}) +'></component>';
      }
      aSrc[1] = '?an='+ aParam;
      return sMdiRenderImg(iTokens, iIdx, iOptions, iEnv, iSelf);
   };

   var kSlideMarkNum  = 3,
       kSlideMark     = ':'.charCodeAt(0),
       kSlideMarkEnd  = '>'.charCodeAt(0),
       sSlideCount    = 0,
       sSlideInDeck   = false;

   mnm._mdi.block.ruler.before('fence', 'mnm-deck', _slideMarkdown,
                               {alt: ['paragraph', 'reference', 'blockquote', 'list'] });
   mnm._mdi.renderer.rules['md-slide_open'] = function(iTokens, iIdx, iOptions, iEnv, iSelf) {
      ++sSlideCount;
      if (iEnv.allSlides)
         iTokens[iIdx].attrSet('class', 'md-slide md-slide-show');
      return iSelf.renderToken(iTokens, iIdx, iOptions);
   };
   mnm._mdi.renderer.rules['md-deck_close'] = function(iTokens, iIdx, iOptions, iEnv, iSelf) {
      var aTwo = sSlideCount >= 2 ? '' : 'disabled ';
      sSlideCount = 0;
      if (iEnv.allSlides)
         return iSelf.renderToken(iTokens, iIdx, iOptions);
      return '<div class="md-deck-ctl">' +
                '<button onclick="return mnm._slideGo(this,-2)" disabled ' +
                        'class="uk-button uk-button-link">&Lang;</button>&nbsp;' +
                '<button onclick="return mnm._slideGo(this,-1)" disabled ' +
                        'class="uk-button uk-button-link">&lang;</button>&nbsp;' +
                '<button onclick="return mnm._slideGo(this,+1)" ' + aTwo +
                        'class="uk-button uk-button-link">&rang;</button>&nbsp;' +
                '<button onclick="return mnm._slideGo(this,+2)" ' + aTwo +
                        'class="uk-button uk-button-link">&Rang;</button></div>' +
             iSelf.renderToken(iTokens, iIdx, iOptions);
   };

   mnm._slideGo = function(iCtl, iStep) {
      var aSet = iCtl.parentNode.parentNode;
      var aShown = aSet.querySelector('div.md-slide-show');
      aShown.classList.remove('md-slide-show');
      var aEl;
      switch (iStep) {
      case -1: aEl = aShown.previousElementSibling;                break;
      case  1: aEl = aShown.nextElementSibling;                    break;
      case -2: aEl = aSet.firstElementChild;                       break;
      case  2: aEl = aSet.lastElementChild.previousElementSibling; break;
      }
      aEl.classList.add('md-slide-show');
      var aCtls = aSet.lastElementChild;
      aCtls.firstChild.disabled = aCtls.firstChild.nextElementSibling.disabled =
         aEl === aSet.firstElementChild;
      aCtls.lastChild.disabled = aCtls.lastChild.previousElementSibling.disabled =
         aEl === aSet.lastElementChild.previousElementSibling;
      return false;
   };

   function _slideMarkdown(iState, iStartLine, iEndLine, iSilent) {
      var aPos, aNextLine, aMarkup, aParams, aToken, aOldParent, aOldLineMax, aHasEod,
          aHasClose = false,
          aStart = iState.bMarks[iStartLine] + iState.tShift[iStartLine],
          aMax = iState.eMarks[iStartLine];

      if (iState.src.charCodeAt(aStart) !== kSlideMark)
         return false;
      for (aPos = aStart + 1; iState.src.charCodeAt(aPos) === kSlideMark; aPos++) {}
      if (aPos - aStart !== kSlideMarkNum)
         return false;
      aMarkup = iState.src.slice(aStart, aPos);
      aParams = iState.src.slice(aPos, aMax);
      if (aParams.trim() !== '')
         return false;
      if (iSilent)
         return true;

      for (aNextLine = iStartLine + 1; aNextLine < iEndLine; ++aNextLine) {
         aStart = iState.bMarks[aNextLine] + iState.tShift[aNextLine];
         aMax = iState.eMarks[aNextLine];
         if (aStart < aMax && iState.sCount[aNextLine] < iState.blkIndent)
            break; // non-empty line with negative indent stops the list
         if (iState.src.charCodeAt(aStart) !== kSlideMark)
            continue;
         if (iState.sCount[aNextLine] - iState.blkIndent >= 4)
            continue; // closing fence must be indented less than 4 spaces
         for (aPos = aStart + 1; iState.src.charCodeAt(aPos) === kSlideMark; aPos++) {}
         if (aPos - aStart !== kSlideMarkNum)
            continue;
         aHasEod = iState.src.charCodeAt(aPos) === kSlideMarkEnd;
         if (aHasEod)
            ++aPos;
         aPos = iState.skipSpaces(aPos); // make sure tail has spaces only
         if (aPos >= aMax) {
            aHasClose = true;
            break;
         }
      }
      aOldParent = iState.parentType;
      aOldLineMax = iState.lineMax;
      iState.parentType = 'container';
      iState.lineMax = aNextLine; // prevent lazy continuations from going past our end marker

      if (!sSlideInDeck) {
         aToken = iState.push('md-deck_open', 'div', 1);
         aToken.block = true;
         aToken.attrPush(['class', 'md-deck']);
      }
      aToken        = iState.push('md-slide_open', 'div', 1);
      aToken.markup = aMarkup;
      aToken.block  = true;
      aToken.info   = aParams;
      aToken.map    = [ iStartLine, aNextLine ];
      aToken.attrPush(['class', 'md-slide']);
      if (!sSlideInDeck) {
         aToken.attrJoin('class', 'md-slide-show');
         sSlideInDeck = true;
      }
      iState.md.block.tokenize(iState, iStartLine + 1, aNextLine);
      aToken        = iState.push('md-slide_close', 'div', -1);
      aToken.markup = iState.src.slice(aStart, aPos);
      aToken.block  = true;
      if (!aHasClose || aHasEod) {
         aToken = iState.push('md-deck_close', 'div', -1);
         aToken.block = true;
         sSlideInDeck = false;
      }

      iState.parentType = aOldParent;
      iState.lineMax = aOldLineMax;
      iState.line = aNextLine + (aHasClose && aHasEod ? 1 : 0);
      return true;
   }

   mnm._listSort = function(iName, iKey) {
      var aTmp;
      var aList = iName === 'cl' ? mnm._data.cl[1] : mnm._data[iName];
      mnm._data.cs.Sort[iName] = iKey;
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
      var aLog = document.getElementById('log').textContent;
      document.getElementById('log').textContent = (i.substr(-1) === '\n' ? i : i+'\n')+aLog;
   };

   mnm.Err = function(iMsg, iOnce) {
      if (iOnce && mnm._data.errors.length && mnm._data.errors[0].Err === iMsg)
         return;
      mnm._data.errors.unshift({Date: luxon.DateTime.utc().toISO(), Err: iMsg});
      mnm._data.errorFlag = true;
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
      case 'cf': case 'cn': case 'cl': case 'al': case 'ml':
      case 'pt': case 'pf': case 'gl': case 'ot': case 'of':
      case 't' : case 'f' : case 'v' : case 'g': case 'l': case 'nlo':
         if (i === 'f' && iEtc) {
            mnm._data.fo = iData;
         } else {
            mnm._data[i] = JSON.parse(iData);
            if (mnm._data.cs.Sort[i])
               sApp.$refs[i].listSort(mnm._data.cs.Sort[i]);
         }
         break;
      case 'cs':
         var aData = JSON.parse(iData);
         for (var a in aData.Sort)
            if (aData.Sort[a] !== mnm._data.cs.Sort[a])
               sApp.$refs[a].listSort(aData.Sort[a]);
         mnm._data.cs = aData;
         break;
      case 'ps':
         var aPs = JSON.parse(iData);
         for (var aK in mnm._data.toSavePs) {
            if (!mnm._data.toSavePs[aK].timer) {
               Vue.delete(mnm._data.toSavePs, aK);
            } else {
               var aEl = aPs.find(function(c) { return sApp.$refs.adrsbk.rowId(c) === aK });
               if (!aEl || aEl.Queued) {
                  clearTimeout(mnm._data.toSavePs[aK].timer);
                  Vue.delete(mnm._data.toSavePs, aK);
               }
            }
         }
         mnm._data.ps = aPs;
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
         var aIds = {}, aList = [];
         for (var a=0; a < iEtc.length; a+=2) {
            aIds[iEtc[a]] = iEtc[a+1];
            aList.push(iEtc[a]);
         }
         mnm._data.adrsbkmenuId = aIds;
         sApp.$refs.adrsbkmenu.results(aList);
         break;
      }
   };

   mnm.ThreadChange = function() {
      sChange = 1;
      sMsglistPos = 0;
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

   window.name = 'mnm_svc_<%.TitleJs%>';
   window.onload = mnm.Connect;
   window.onerror = function(iMsg /*etc*/) { mnm.Err(iMsg) };

   Vue.config.errorHandler = function(iErr /*etc*/) {
      mnm.Err(iErr.message, 'once');
      console.error(iErr);
   };
   sApp.$mount('#app');

}).call(this);
</script>

</body></html>

