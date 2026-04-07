<template>
  <div class="sd-page" @keydown="onKey">
    <div class="sd-topbar">
      <div class="sd-nav-left">
        <a class="sd-back" @click="goBack"><Icon name="arrow-left" :size="14" /> 返回列表</a>
        <span class="sd-sep">›</span>
        <span class="mono sd-tid">{{ traceId }}</span>
      </div>
      <div class="sd-nav-right">
        <button class="btn btn-ghost btn-sm" @click="navPrev" title="上一个会话 (Alt+↑)"><Icon name="arrow-left" :size="14" /></button>
        <button class="btn btn-ghost btn-sm" @click="navNext" title="下一个会话 (Alt+↓)"><Icon name="chevron-right" :size="14" /></button>
        <button class="btn btn-ghost btn-sm" @click="exportJSON" title="导出 JSON"><Icon name="download" :size="14" /> JSON</button>
        <button class="btn btn-ghost btn-sm" @click="exportMD" title="导出 Markdown"><Icon name="file-text" :size="14" /> MD</button>
        <button class="btn btn-ghost btn-sm" @click="copyLink" title="复制链接"><Icon name="link" :size="14" /></button>
      </div>
    </div>
    <div v-if="loading" class="sd-loading">
      <div class="sk-bubble sk-l"></div><div class="sk-bubble sk-r"></div><div class="sk-bubble sk-l sk-w60"></div><div class="sk-bubble sk-r sk-w40"></div>
    </div>
    <div v-else-if="!timeline" class="empty-state">
      <div class="empty-icon">🔍</div><div class="empty-title">会话不存在</div><div class="empty-desc">未找到 trace_id: {{ traceId }}</div>
    </div>
    <template v-else>
      <!-- Summary -->
      <div class="sd-summary card">
        <div class="sds-top">
          <div class="sds-left">
            <div class="sds-trace mono">{{ sm.trace_id }}</div>
            <div class="sds-meta">
              <span v-if="sm.sender_id"><Icon name="users" :size="12" /> {{ sm.display_name || sm.sender_id }}<span v-if="sm.display_name" class="sds-sub"> ({{ sm.sender_id }})</span></span>
              <span v-if="sm.department" class="sds-dept">{{ sm.department }}</span>
              <span v-if="sm.model"><Icon name="brain" :size="12" /> {{ sm.model }}</span>
              <span><Icon name="clock" :size="12" /> {{ fmtD(sm.duration_ms) }}</span>
            </div>
          </div>
          <div class="sds-right">
            <span class="rb" :class="'b-'+sm.risk_level">{{ rl(sm.risk_level) }}</span>
            <span class="risk-score" v-if="riskScore>0">风险: <strong>{{ riskScore }}</strong></span>
          </div>
        </div>
        <div class="sds-pills">
          <div class="pill"><span class="pl">IM</span><span class="pv">{{ sm.im_events }}</span></div>
          <div class="pill"><span class="pl">LLM</span><span class="pv">{{ sm.llm_calls }}</span></div>
          <div class="pill"><span class="pl">Tools</span><span class="pv">{{ sm.tool_calls }}</span></div>
          <div class="pill"><span class="pl">Tokens</span><span class="pv">{{ fmtN(sm.total_tokens) }}</span></div>
          <div class="pill pw" v-if="sm.canary_leaked"><span class="pv">🔴 Canary Leaked</span></div>
          <div class="pill pw" v-if="sm.budget_exceeded"><span class="pv">⚠️ Budget Exceeded</span></div>
          <div class="pill pw" v-if="sm.blocked"><span class="pv">🚫 Blocked</span></div>
        </div>
        <div class="sds-tags">
          <span class="tag-label">标签:</span>
          <span class="stag" v-for="t in sessionTags" :key="t" :class="tagClass(t)">{{ t }} <button class="stx" @click="rmSessionTag(t)">×</button></span>
          <div class="tag-add" v-if="!showTagInput"><button class="btn-tag-add" @click="showTagInput=true"><Icon name="tag" :size="12" /> + 标签</button></div>
          <div class="tag-add-row" v-else>
            <select v-model="newTag" class="tag-sel"><option value="">选择...</option><option value="可疑">🟡 可疑</option><option value="已确认">🔴 已确认</option><option value="误报">🟢 误报</option><option value="已处理">✅ 已处理</option><option value="custom">✏️ 自定义</option></select>
            <input v-if="newTag==='custom'" v-model="customTag" placeholder="自定义..." class="tag-inp" @keyup.enter="addSessionTag" />
            <button class="btn btn-sm" @click="addSessionTag">添加</button>
            <button class="btn btn-ghost btn-sm" @click="showTagInput=false;newTag='';customTag=''">取消</button>
          </div>
        </div>
      </div>
      <!-- Notes -->
      <div class="sd-notes card">
        <div class="card-header"><span class="card-icon"><Icon name="edit" :size="16" /></span><span class="card-title">分析师注释</span></div>
        <div class="notes-list" v-if="notes.length">
          <div class="note" v-for="(n,i) in notes" :key="i">
            <div class="note-meta"><span class="note-author">{{ n.author }}</span><span class="note-time">{{ n.time }}</span></div>
            <div class="note-text">{{ n.text }}</div>
            <button class="note-del" @click="rmNote(i)">×</button>
          </div>
        </div>
        <div class="note-add"><input v-model="noteText" placeholder="添加注释..." @keyup.enter="addNote" class="note-inp" /><button class="btn btn-sm" @click="addNote" :disabled="!noteText.trim()">添加</button></div>
      </div>
      <!-- Chat Timeline -->
      <div class="sd-chat card">
        <div class="card-header">
          <span class="card-icon"><Icon name="message-circle" :size="16" /></span>
          <span class="card-title">会话时间线 ({{ timeline.events.length }})</span>
          <div class="card-actions"><select v-model="evFilter" class="ev-filter"><option value="all">全部事件</option><option value="im">仅 IM</option><option value="llm">仅 LLM</option><option value="tool">仅工具</option><option value="security">仅安全</option></select></div>
        </div>
        <div class="chat-area" ref="chatArea">
          <div v-for="(ev,idx) in filteredEvents" :key="idx" class="chat-row" :class="chatRowClass(ev)">
            <!-- IM Inbound (user → left blue bubble) -->
            <template v-if="ev.type==='im_inbound'">
              <div class="bubble-wrap bw-left">
                <div class="bubble-avatar ba-user"><Icon name="users" :size="14" /></div>
                <div class="bubble-body">
                  <div class="bubble-head"><span class="bh-name">{{ ev.sender_id||'用户' }}</span><span class="bh-time mono">{{ fmtTF(ev.timestamp) }}</span></div>
                  <div class="bubble b-user" :class="{'b-blocked':ev.action==='block','b-warn':ev.action==='warn'}">
                    <div class="b-text" :class="{'bt-blocked':ev.action==='block'}">{{ ev.content }}</div>
                  </div>
                  <div class="b-meta">
                    <span class="action-tag" :class="'at-'+ev.action" v-if="ev.action">{{ ev.action }}</span>
                    <span class="b-reason" v-if="ev.reason">{{ ev.reason }}</span>
                    <span class="b-lat" v-if="ev.latency_ms">{{ Math.round(ev.latency_ms) }}ms</span>
                  </div>
                  <div class="b-rule" v-if="ev.action&&ev.action!=='pass'&&ev.reason"><Icon name="shield" :size="12" /><span>规则: {{ ev.reason }}</span><a class="rule-link" @click="$router.push('/rules')">查看 →</a></div>
                </div>
              </div>
            </template>
            <!-- IM Outbound (agent → right green bubble) -->
            <template v-else-if="ev.type==='im_outbound'">
              <div class="bubble-wrap bw-right">
                <div class="bubble-body">
                  <div class="bubble-head bh-right"><span class="bh-time mono">{{ fmtTF(ev.timestamp) }}</span><span class="bh-name">Agent</span></div>
                  <div class="bubble b-agent" :class="{'b-blocked':ev.action==='block','b-warn':ev.action==='warn'}">
                    <div class="b-text" :class="{'bt-blocked':ev.action==='block'}">{{ ev.content }}</div>
                  </div>
                  <div class="b-meta bm-right">
                    <span class="b-lat" v-if="ev.latency_ms">{{ Math.round(ev.latency_ms) }}ms</span>
                    <span class="action-tag" :class="'at-'+ev.action" v-if="ev.action&&ev.action!=='pass'">{{ ev.action }}</span>
                    <span class="b-reason" v-if="ev.reason">{{ ev.reason }}</span>
                  </div>
                  <div class="b-rule" v-if="ev.action&&ev.action!=='pass'&&ev.reason"><Icon name="shield" :size="12" /><span>规则: {{ ev.reason }}</span><a class="rule-link" @click="$router.push('/rules')">查看 →</a></div>
                </div>
                <div class="bubble-avatar ba-agent"><Icon name="bot" :size="14" /></div>
              </div>
            </template>
            <!-- LLM Call -->
            <template v-else-if="ev.type==='llm_call'">
              <div class="sys-event se-llm" :class="{'se-expanded': ev._expanded}">
                <div class="se-dot"><span>◆</span></div>
                <div class="se-body">
                  <div class="se-head se-head-click" @click="ev._expanded = !ev._expanded">
                    <span class="se-label">LLM 调用</span><span class="mono se-time">{{ fmtTF(ev.timestamp) }}</span>
                    <span class="se-expand-icon">{{ ev._expanded ? '▾' : '▸' }}</span>
                  </div>
                  <div class="se-detail">
                    <span v-if="ev.model" class="se-model">{{ ev.model }}</span>
                    <span class="se-metric">tokens: {{ fmtN(ev.tokens) }}</span>
                    <span class="se-metric">{{ Math.round(ev.latency_ms||0) }}ms</span>
                    <span v-if="ev.status_code&&ev.status_code>=400" class="se-err">HTTP {{ ev.status_code }}</span>
                  </div>
                  <div v-if="ev._expanded" class="llm-content">
                    <div v-if="ev.request_preview" class="llm-block llm-req">
                      <div class="llm-block-label">Request</div>
                      <pre class="llm-block-body">{{ ev.request_preview }}</pre>
                    </div>
                    <div v-if="ev.response_preview" class="llm-block llm-resp">
                      <div class="llm-block-label">Response</div>
                      <pre class="llm-block-body">{{ ev.response_preview }}</pre>
                    </div>
                    <div v-if="!ev.request_preview && !ev.response_preview" class="llm-no-content">内容未记录（旧数据）</div>
                  </div>
                  <div class="se-alert ca" v-if="ev.canary_leaked">🔴 Canary Token 泄露！</div>
                  <div class="se-alert ba2" v-if="ev.budget_exceeded">⚠️ 响应预算超限</div>
                  <div class="se-alert ea" v-if="ev.error_message">❌ {{ ev.error_message }}</div>
                </div>
              </div>
            </template>
            <!-- Tool Call -->
            <template v-else-if="ev.type==='tool_call'">
              <div class="sys-event se-tool" :class="{'se-flagged':ev.flagged}">
                <div class="se-dot dot-tool" :class="{'dot-flagged':ev.flagged}"><span>▸</span></div>
                <div class="se-body">
                  <div class="se-head"><span class="se-label">工具调用</span><span class="mono se-time">{{ fmtTF(ev.timestamp) }}</span></div>
                  <div class="se-detail">
                    <span class="tool-name">{{ ev.tool_name }}</span>
                    <span class="tool-risk" :class="'tr-'+ev.risk_level">{{ ev.risk_level }}</span>
                    <span class="tool-flag" v-if="ev.flagged">⚠️ flagged</span>
                  </div>
                  <div class="se-code" v-if="ev.tool_input"><div class="code-lbl">Input:</div><pre class="code-blk">{{ ev.tool_input }}</pre></div>
                  <div class="se-code" v-if="ev.tool_result"><div class="code-lbl">Result:</div><pre class="code-blk">{{ ev.tool_result }}</pre></div>
                  <div class="se-flag-reason" v-if="ev.flag_reason">🚩 {{ ev.flag_reason }}</div>
                </div>
              </div>
            </template>
            <!-- Tag -->
            <template v-else-if="ev.type==='tag'">
              <div class="sys-event se-tag">
                <div class="se-dot dot-tag"><span>💬</span></div>
                <div class="se-body">
                  <div class="tag-bubble"><span class="tb-text">{{ ev.tag_text }}</span><span class="tb-author" v-if="ev.tag_author">— {{ ev.tag_author }}</span><button class="tb-del" @click="delTag(ev.tag_id)">×</button></div>
                </div>
              </div>
            </template>
            <!-- Per-event tag action -->
            <div class="ev-tag-action" v-if="ev.type!=='tag'">
              <button class="btn-add-tag" v-if="!ev._showTag" @click="ev._showTag=true"><Icon name="tag" :size="10" /> +</button>
              <div class="ev-tag-row" v-if="ev._showTag">
                <input v-model="ev._tagText" placeholder="标签..." @keyup.enter="submitTag(ev)" class="ev-tag-inp" />
                <button class="btn btn-sm" @click="submitTag(ev)">添加</button>
                <button class="btn btn-ghost btn-sm" @click="ev._showTag=false">取消</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, apiPost, apiDelete } from '../api.js'
import { showToast } from '../stores/app.js'
import Icon from '../components/Icon.vue'

const route = useRoute()
const router = useRouter()
const traceId = route.params.traceId
const loading = ref(true)
const timeline = ref(null)
const evFilter = ref('all')
const chatArea = ref(null)
const showTagInput = ref(false)
const newTag = ref('')
const customTag = ref('')
const noteText = ref('')

const sm = computed(() => timeline.value?.summary || {})
const riskScore = computed(() => {
  const s = sm.value; let sc = 0
  if (s.canary_leaked) sc += 40
  if (s.blocked) sc += 30
  if (s.budget_exceeded) sc += 15
  sc += (s.flagged_tools || 0) * 10
  if (s.risk_level === 'critical') sc += 20
  else if (s.risk_level === 'high') sc += 10
  return Math.min(sc, 100)
})
const filteredEvents = computed(() => {
  const evs = timeline.value?.events || []
  if (evFilter.value === 'all') return evs
  if (evFilter.value === 'im') return evs.filter(e => e.type === 'im_inbound' || e.type === 'im_outbound')
  if (evFilter.value === 'llm') return evs.filter(e => e.type === 'llm_call')
  if (evFilter.value === 'tool') return evs.filter(e => e.type === 'tool_call')
  if (evFilter.value === 'security') return evs.filter(e => e.flagged || e.canary_leaked || e.budget_exceeded || e.action === 'block' || e.action === 'warn')
  return evs
})

// Session tags (localStorage)
const sessionTags = ref([])
function loadSessionTags() { try { const all = JSON.parse(localStorage.getItem('lg_session_tags') || '{}'); sessionTags.value = all[traceId] || [] } catch { sessionTags.value = [] } }
function saveSessionTags() { try { const all = JSON.parse(localStorage.getItem('lg_session_tags') || '{}'); all[traceId] = sessionTags.value; localStorage.setItem('lg_session_tags', JSON.stringify(all)) } catch {} }
function addSessionTag() {
  const tag = newTag.value === 'custom' ? customTag.value.trim() : newTag.value
  if (!tag || sessionTags.value.includes(tag)) return
  sessionTags.value.push(tag); saveSessionTags(); newTag.value = ''; customTag.value = ''; showTagInput.value = false; showToast('标签已添加', 'success')
}
function rmSessionTag(tag) { sessionTags.value = sessionTags.value.filter(t => t !== tag); saveSessionTags() }
function tagClass(t) { return { '可疑': 'stag-warn', '已确认': 'stag-danger', '误报': 'stag-safe', '已处理': 'stag-done' }[t] || '' }

// Notes (localStorage)
const notes = ref([])
function loadNotes() { try { notes.value = JSON.parse(localStorage.getItem('lg_notes_' + traceId) || '[]') } catch { notes.value = [] } }
function saveNotes() { try { localStorage.setItem('lg_notes_' + traceId, JSON.stringify(notes.value)) } catch {} }
function addNote() { const t = noteText.value.trim(); if (!t) return; notes.value.push({ text: t, author: 'admin', time: new Date().toLocaleString('zh-CN', { hour12: false }) }); saveNotes(); noteText.value = ''; showToast('注释已添加', 'success') }
function rmNote(i) { notes.value.splice(i, 1); saveNotes() }

// Formatters
function fmtTF(ts) { if (!ts) return '--'; const d = new Date(ts); if (isNaN(d.getTime())) return ts; return String(d.getHours()).padStart(2,'0')+':'+String(d.getMinutes()).padStart(2,'0')+':'+String(d.getSeconds()).padStart(2,'0')+'.'+String(d.getMilliseconds()).padStart(3,'0') }
function fmtD(ms) { if (!ms||ms<=0) return '--'; if (ms<1000) return Math.round(ms)+'ms'; if (ms<60000) return (ms/1000).toFixed(1)+'s'; return Math.floor(ms/60000)+'m '+Math.floor((ms%60000)/1000)+'s' }
function fmtN(n) { if (!n) return '0'; return n>=1000?(n/1000).toFixed(1)+'K':String(n) }
function rl(l) { return {critical:'🔴 严重',high:'🟠 高危',medium:'🟡 中等',low:'🟢 低风险'}[l]||l||'未知' }
function chatRowClass(ev) { const c=['cr-'+ev.type]; if(ev.flagged)c.push('cr-flagged'); if(ev.action==='block')c.push('cr-blocked'); if(ev.canary_leaked)c.push('cr-canary'); return c }

// API tag ops
async function submitTag(ev) {
  if (!ev._tagText) return
  try { await apiPost('/api/v1/sessions/replay/'+encodeURIComponent(traceId)+'/tags',{text:ev._tagText,event_type:ev.type,event_id:ev.id||0,author:'admin'}); ev._showTag=false;ev._tagText='';showToast('标签已添加','success');loadTimeline() } catch(e){showToast('失败: '+e.message,'error')}
}
async function delTag(tagId) { if(!tagId)return; try{await apiDelete('/api/v1/sessions/replay/tags/'+tagId);showToast('标签已删除','success');loadTimeline()}catch(e){showToast('失败: '+e.message,'error')} }

// Export
function exportJSON() { const d=JSON.stringify(timeline.value,null,2); dl(d,traceId+'.json','application/json'); showToast('已导出 JSON','success') }
function exportMD() {
  const s=sm.value; let md=`# 会话回放: ${s.trace_id}\n\n- **用户**: ${s.sender_id||'N/A'}\n- **模型**: ${s.model||'N/A'}\n- **风险**: ${s.risk_level}\n- **时长**: ${fmtD(s.duration_ms)}\n\n## 时间线\n\n`
  for(const ev of(timeline.value?.events||[])){
    if(ev.type==='im_inbound')md+=`### ⬅ 用户 (${fmtTF(ev.timestamp)})\n> ${ev.content||''}\n\n`
    else if(ev.type==='im_outbound')md+=`### ➡ Agent (${fmtTF(ev.timestamp)})\n> ${ev.content||''}\n\n`
    else if(ev.type==='llm_call')md+=`### ◆ LLM (${fmtTF(ev.timestamp)}) ${ev.model||''} — ${fmtN(ev.tokens)} tokens\n\n`
    else if(ev.type==='tool_call')md+=`### ▸ ${ev.tool_name} (${fmtTF(ev.timestamp)})\n\`\`\`\n${ev.tool_input||''}\n\`\`\`\n\n`
  }
  if(notes.value.length){md+=`## 注释\n\n`;for(const n of notes.value)md+=`- **${n.author}** (${n.time}): ${n.text}\n`}
  dl(md,traceId+'.md','text/markdown');showToast('已导出 MD','success')
}
function dl(content,filename,type){const b=new Blob([content],{type});const a=document.createElement('a');a.href=URL.createObjectURL(b);a.download=filename;a.click();URL.revokeObjectURL(a.href)}
function copyLink(){const u=location.origin+'/#/sessions/'+encodeURIComponent(traceId);navigator.clipboard.writeText(u).then(()=>showToast('链接已复制','success')).catch(()=>showToast('复制失败','error'))}

// Navigation
function goBack(){router.push('/sessions')}
function navPrev(){try{const l=JSON.parse(sessionStorage.getItem('lg_nav_ids')||'[]');const i=l.indexOf(traceId);if(i>0)router.push('/sessions/'+encodeURIComponent(l[i-1]))}catch{}}
function navNext(){try{const l=JSON.parse(sessionStorage.getItem('lg_nav_ids')||'[]');const i=l.indexOf(traceId);if(i>=0&&i<l.length-1)router.push('/sessions/'+encodeURIComponent(l[i+1]))}catch{}}
function onKey(e){if(e.key==='Escape')goBack();if(e.key==='ArrowUp'&&e.altKey)navPrev();if(e.key==='ArrowDown'&&e.altKey)navNext()}

async function loadTimeline() {
  loading.value = true
  try {
    const d = await api('/api/v1/sessions/replay/' + encodeURIComponent(traceId))
    if (d.events) d.events.forEach(ev => { ev._showTag = false; ev._tagText = ''; ev._expanded = false })
    timeline.value = d
    await nextTick()
    if (chatArea.value) chatArea.value.scrollTop = 0
  } catch { timeline.value = null }
  loading.value = false
}

onMounted(() => { loadTimeline(); loadSessionTags(); loadNotes() })
</script>

<style scoped>
.sd-page{outline:none}
.sd-topbar{display:flex;justify-content:space-between;align-items:center;margin-bottom:16px;flex-wrap:wrap;gap:8px}
.sd-nav-left{display:flex;align-items:center;gap:8px;font-size:13px}
.sd-back{display:flex;align-items:center;gap:4px;color:var(--color-primary);cursor:pointer}
.sd-back:hover{text-decoration:underline}
.sd-sep{color:var(--text-tertiary)}
.sd-tid{color:var(--text-secondary);font-size:12px}
.sd-nav-right{display:flex;align-items:center;gap:6px}
.mono{font-family:var(--font-mono)}
.sd-loading{padding:32px;display:flex;flex-direction:column;gap:12px}
.sk-bubble{height:48px;border-radius:16px;animation:skp 1.5s ease-in-out infinite}
.sk-l{width:60%;background:rgba(59,130,246,.1);align-self:flex-start;margin-left:48px}
.sk-r{width:50%;background:rgba(34,197,94,.1);align-self:flex-end;margin-right:48px}
.sk-w60{width:65%}.sk-w40{width:45%}
@keyframes skp{0%,100%{opacity:.4}50%{opacity:1}}
.empty-state{text-align:center;padding:32px}
.empty-icon{font-size:3rem;margin-bottom:8px}
.empty-title{font-size:18px;font-weight:600;color:var(--text-primary);margin-bottom:4px}
.empty-desc{font-size:13px;color:var(--text-tertiary)}
.sd-summary{margin-bottom:16px}
.sds-top{display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:12px;flex-wrap:wrap;gap:8px}
.sds-left{flex:1}
.sds-trace{font-size:16px;font-weight:700;color:var(--color-primary)}
.sds-meta{display:flex;gap:12px;font-size:12px;color:var(--text-secondary);margin-top:4px;flex-wrap:wrap}
.sds-meta span{display:flex;align-items:center;gap:4px}
.sds-sub{color:var(--text-tertiary);font-size:10px}
.sds-dept{color:#0d9488!important;background:rgba(20,184,166,0.1);border-radius:4px;padding:1px 6px}
.sds-right{display:flex;align-items:center;gap:12px}
.risk-score{font-size:12px;color:var(--text-secondary)}
.risk-score strong{color:var(--text-primary);font-size:16px;font-family:var(--font-mono)}
.sds-pills{display:flex;gap:8px;flex-wrap:wrap;margin-bottom:12px}
.pill{display:flex;align-items:center;gap:4px;background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:20px;padding:4px 12px;font-size:11px}
.pill.pw{background:rgba(239,68,68,.1);border-color:rgba(239,68,68,.3)}
.pl{color:var(--text-tertiary);text-transform:uppercase;font-weight:500;letter-spacing:.05em}
.pv{color:var(--text-primary);font-weight:600;font-family:var(--font-mono)}
.rb{font-size:12px;font-weight:700;padding:4px 12px;border-radius:16px}
.b-critical{background:rgba(239,68,68,.15);color:#EF4444}
.b-high{background:rgba(249,115,22,.15);color:#F97316}
.b-medium{background:rgba(234,179,8,.15);color:#EAB308}
.b-low{background:rgba(34,197,94,.15);color:#22C55E}
.sds-tags{display:flex;align-items:center;gap:8px;flex-wrap:wrap;padding-top:12px;border-top:1px solid var(--border-subtle)}
.tag-label{font-size:11px;color:var(--text-tertiary);font-weight:500}
.stag{display:inline-flex;align-items:center;gap:4px;font-size:11px;padding:3px 10px;border-radius:12px;background:rgba(255,255,255,.08);color:var(--text-secondary)}
.stag-warn{background:rgba(234,179,8,.15);color:#EAB308}
.stag-danger{background:rgba(239,68,68,.15);color:#EF4444}
.stag-safe{background:rgba(34,197,94,.15);color:#22C55E}
.stag-done{background:rgba(99,102,241,.15);color:#818CF8}
.stx{background:none;border:none;color:inherit;cursor:pointer;font-size:14px;line-height:1;opacity:.6}
.stx:hover{opacity:1}
.btn-tag-add{display:flex;align-items:center;gap:4px;background:none;border:1px dashed var(--border-subtle);border-radius:6px;color:var(--text-tertiary);font-size:11px;padding:3px 8px;cursor:pointer}
.btn-tag-add:hover{color:var(--color-primary);border-color:var(--color-primary)}
.tag-add-row{display:flex;align-items:center;gap:6px}
.tag-sel,.tag-inp{background:var(--bg-elevated);border:1px solid var(--border-default);border-radius:6px;color:var(--text-primary);padding:4px 8px;font-size:11px;outline:none}
.tag-inp{width:120px}
.tag-sel:focus,.tag-inp:focus{border-color:var(--color-primary)}
.sd-notes{margin-bottom:16px}
.notes-list{display:flex;flex-direction:column;gap:8px;margin-bottom:12px}
.note{display:flex;align-items:flex-start;gap:8px;padding:8px 12px;background:var(--bg-base);border-radius:8px;position:relative}
.note-meta{display:flex;flex-direction:column;min-width:80px}
.note-author{font-size:12px;font-weight:600;color:var(--text-primary)}
.note-time{font-size:10px;color:var(--text-tertiary)}
.note-text{flex:1;font-size:13px;color:var(--text-secondary);line-height:1.5}
.note-del{position:absolute;top:8px;right:8px;background:none;border:none;color:var(--text-tertiary);cursor:pointer;font-size:14px;opacity:0;transition:opacity .15s}
.note:hover .note-del{opacity:1}
.note-del:hover{color:#EF4444}
.note-add{display:flex;gap:8px}
.note-inp{flex:1;background:var(--bg-elevated);border:1px solid var(--border-default);border-radius:8px;color:var(--text-primary);padding:6px 10px;font-size:13px;outline:none}
.note-inp:focus{border-color:var(--color-primary)}
.sd-chat{position:relative}
.ev-filter{background:var(--bg-elevated);border:1px solid var(--border-default);border-radius:6px;color:var(--text-primary);padding:4px 8px;font-size:11px;outline:none}
.chat-area{padding:8px 0}
.chat-row{margin-bottom:4px;padding:4px 0}
.chat-row.cr-flagged{background:rgba(239,68,68,.03);border-radius:8px}
.chat-row.cr-blocked{background:rgba(239,68,68,.04)}
.chat-row.cr-canary{background:rgba(239,68,68,.06)}
.bubble-wrap{display:flex;gap:10px;padding:4px 8px;max-width:85%}
.bw-left{align-self:flex-start}
.bw-right{align-self:flex-end;margin-left:auto}
.bubble-avatar{width:32px;height:32px;border-radius:50%;display:flex;align-items:center;justify-content:center;flex-shrink:0}
.ba-user{background:rgba(59,130,246,.15);color:#3B82F6}
.ba-agent{background:rgba(34,197,94,.15);color:#22C55E}
.bubble-body{flex:1;min-width:0}
.bubble-head{display:flex;align-items:center;gap:8px;margin-bottom:2px}
.bh-right{justify-content:flex-end}
.bh-name{font-size:12px;font-weight:600;color:var(--text-secondary)}
.bh-time{font-size:10px;color:var(--text-tertiary)}
.bubble{padding:10px 14px;border-radius:16px;font-size:13px;line-height:1.6;color:var(--text-primary);word-break:break-word}
.b-user{background:rgba(59,130,246,.1);border-top-left-radius:4px}
.b-agent{background:rgba(34,197,94,.1);border-top-right-radius:4px}
.b-blocked{background:rgba(239,68,68,.12)!important;border:1px solid rgba(239,68,68,.25)}
.b-warn{background:rgba(234,179,8,.1)!important;border:1px solid rgba(234,179,8,.25)}
.bt-blocked{text-decoration:line-through;color:var(--text-tertiary)}
.b-text{white-space:pre-wrap}
.b-meta{display:flex;align-items:center;gap:6px;margin-top:4px;flex-wrap:wrap}
.bm-right{justify-content:flex-end}
.action-tag{font-size:10px;font-weight:700;padding:1px 6px;border-radius:6px;text-transform:uppercase}
.at-pass{background:rgba(34,197,94,.15);color:#22C55E}
.at-block{background:rgba(239,68,68,.15);color:#EF4444}
.at-warn{background:rgba(234,179,8,.15);color:#EAB308}
.b-reason{font-size:11px;color:var(--text-tertiary);font-style:italic}
.b-lat{font-size:10px;color:var(--text-tertiary);font-family:var(--font-mono)}
.b-rule{display:flex;align-items:center;gap:6px;margin-top:6px;padding:6px 10px;background:rgba(239,68,68,.06);border-radius:6px;font-size:11px;color:#EF4444}
.rule-link{color:var(--color-primary);cursor:pointer;margin-left:auto;white-space:nowrap}
.rule-link:hover{text-decoration:underline}
.sys-event{display:flex;gap:12px;padding:8px;margin:8px 0}
.se-dot{width:28px;height:28px;border-radius:50%;display:flex;align-items:center;justify-content:center;font-size:14px;flex-shrink:0}
.se-llm .se-dot{background:rgba(168,85,247,.15);color:#A855F7}
.se-tool .se-dot{background:rgba(34,197,94,.15);color:#22C55E}
.se-tool.se-flagged .se-dot{background:rgba(239,68,68,.15);color:#EF4444;animation:pulse 2s infinite}
.se-tag .se-dot{background:rgba(255,255,255,.08)}
@keyframes pulse{0%,100%{box-shadow:0 0 0 0 rgba(239,68,68,.4)}50%{box-shadow:0 0 0 8px rgba(239,68,68,0)}}
.se-body{flex:1;min-width:0}
.se-head{display:flex;align-items:center;gap:8px;margin-bottom:4px}
.se-label{font-size:11px;font-weight:600;color:var(--text-secondary)}
.se-time{font-size:10px;color:var(--text-tertiary)}
.se-detail{display:flex;align-items:center;gap:8px;flex-wrap:wrap;font-size:12px;margin-bottom:4px}
.se-model{color:#A855F7;font-weight:600;font-family:var(--font-mono);font-size:11px}
.se-metric{color:var(--text-tertiary);font-family:var(--font-mono);font-size:11px}
.se-err{color:#EF4444;font-weight:600}
.se-alert{font-size:12px;font-weight:600;padding:6px 10px;border-radius:6px;margin-top:4px}
.ca{background:rgba(239,68,68,.15);color:#EF4444;border:1px solid rgba(239,68,68,.3)}
.ba2{background:rgba(249,115,22,.15);color:#F97316;border:1px solid rgba(249,115,22,.3)}
.ea{background:rgba(239,68,68,.1);color:#EF4444}
.tool-name{font-weight:700;color:var(--text-primary);font-family:var(--font-mono)}
.tool-risk{font-size:10px;font-weight:600;padding:1px 6px;border-radius:8px}
.tr-low{background:rgba(34,197,94,.15);color:#22C55E}
.tr-medium{background:rgba(234,179,8,.15);color:#EAB308}
.tr-high{background:rgba(249,115,22,.15);color:#F97316}
.tr-critical{background:rgba(239,68,68,.15);color:#EF4444}
.tool-flag{font-size:10px;color:#EF4444;font-weight:700}
.se-code{margin:4px 0}
.se-head-click{cursor:pointer;display:flex;align-items:center;gap:6px}
.se-head-click:hover{opacity:.8}
.se-expand-icon{font-size:10px;color:var(--text-tertiary);margin-left:auto}
.llm-content{margin-top:8px}
.llm-block{margin-bottom:8px;border-radius:6px;overflow:hidden}
.llm-block-label{font-size:11px;font-weight:600;padding:4px 10px;text-transform:uppercase;letter-spacing:.03em}
.llm-req .llm-block-label{background:rgba(99,102,241,.15);color:#818cf8}
.llm-resp .llm-block-label{background:rgba(34,197,94,.1);color:#4ade80}
.llm-block-body{font-size:12px;padding:8px 10px;margin:0;background:rgba(15,23,42,.5);color:var(--text-secondary);white-space:pre-wrap;word-break:break-word;max-height:300px;overflow-y:auto;line-height:1.5}
.llm-no-content{font-size:12px;color:var(--text-tertiary);font-style:italic;padding:8px}
.code-lbl{font-size:10px;color:var(--text-tertiary);text-transform:uppercase;letter-spacing:.05em;margin-bottom:2px}
.code-blk{background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:6px;padding:6px 8px;font-family:var(--font-mono);font-size:11px;color:var(--text-secondary);overflow-x:auto;max-height:120px;white-space:pre-wrap;word-break:break-all;margin:0}
.se-flag-reason{font-size:11px;color:#EF4444;font-weight:600;margin-top:4px}
.tag-bubble{display:inline-flex;align-items:center;gap:8px;background:rgba(255,255,255,.06);padding:6px 10px;border-radius:8px;border:1px dashed var(--border-subtle)}
.tb-text{font-size:13px;color:var(--text-primary)}
.tb-author{font-size:11px;color:var(--text-tertiary)}
.tb-del{background:none;border:none;color:var(--text-tertiary);cursor:pointer;font-size:16px;line-height:1}
.tb-del:hover{color:#EF4444}
.ev-tag-action{padding:2px 8px 2px 50px}
.btn-add-tag{display:inline-flex;align-items:center;gap:4px;background:none;border:none;color:var(--text-tertiary);font-size:10px;padding:2px 6px;cursor:pointer;opacity:.4;transition:opacity .15s}
.btn-add-tag:hover{opacity:1;color:var(--color-primary)}
.ev-tag-row{display:flex;align-items:center;gap:6px}
.ev-tag-inp{background:var(--bg-elevated);border:1px solid var(--border-default);border-radius:6px;color:var(--text-primary);padding:3px 8px;font-size:11px;outline:none;width:200px}
.ev-tag-inp:focus{border-color:var(--color-primary)}
</style>
