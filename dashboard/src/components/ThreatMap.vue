<template>
  <div class="threat-map-root" @click="selectedNode = null">
    <div class="tm-hud-top-left">
      <span class="tm-live-badge"><span class="tm-live-dot"></span>LIVE</span>
      <span class="tm-clock">{{ clock }}</span>
    </div>
    <div class="tm-stats-bar">
      <div class="tm-stat"><span class="tm-stat-label">总请求</span><span class="tm-stat-val">{{ summary.total_requests }}</span></div>
      <div class="tm-stat"><span class="tm-stat-label">拦截</span><span class="tm-stat-val tm-red">{{ summary.blocked }}</span></div>
      <div class="tm-stat"><span class="tm-stat-label">告警</span><span class="tm-stat-val tm-orange">{{ summary.warned }}</span></div>
      <div class="tm-stat"><span class="tm-stat-label">通过</span><span class="tm-stat-val tm-green">{{ summary.passed }}</span></div>
    </div>
    <svg class="tm-svg" :viewBox="'0 0 '+vw+' '+vh" preserveAspectRatio="xMidYMid meet">
      <defs>
        <filter id="glow-green" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur in="SourceGraphic" stdDeviation="4" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter>
        <filter id="glow-red" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur in="SourceGraphic" stdDeviation="5" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter>
        <filter id="glow-blue" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur in="SourceGraphic" stdDeviation="4" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter>
        <filter id="glow-purple" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur in="SourceGraphic" stdDeviation="4" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter>
        <filter id="glow-orange" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur in="SourceGraphic" stdDeviation="4" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter>
        <marker id="arrow-fwd" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto" markerUnits="userSpaceOnUse">
          <path d="M0,0 L8,3 L0,6" fill="rgba(99,102,241,0.35)"/>
        </marker>
        <path v-for="s in imNodes" :key="'p-im-in-'+s.id" :id="'path-im-inbound-'+s.id" :d="bezierH(s.x+s.r, s.y, gInbound.x-gInbound.r, gInbound.y)" fill="none"/>
        <path id="path-inbound-oc" :d="bezierH(gInbound.x+gInbound.r, gInbound.y, ocNode.x-ocNode.r, ocNode.y)" fill="none"/>
        <path id="path-oc-llmproxy" :d="bezierDown(ocNode.x, ocNode.y+ocNode.r, gLLM.x, gLLM.y-gLLM.r)" fill="none"/>
        <path id="path-llmproxy-claude" :d="bezierH(gLLM.x+gLLM.r, gLLM.y, claudeNode.x-claudeNode.r, claudeNode.y)" fill="none"/>
        <path id="path-claude-oc" :d="returnArc(claudeNode.x-claudeNode.r, claudeNode.y, ocNode.x+ocNode.r, ocNode.y, 65)" fill="none"/>
        <path id="path-oc-outbound" :d="bezierDown(ocNode.x, ocNode.y+ocNode.r, gOutbound.x, gOutbound.y-gOutbound.r)" fill="none"/>
        <path v-for="s in imNodes" :key="'p-out-im-'+s.id" :id="'path-outbound-im-'+s.id" :d="returnArcH(gOutbound.x-gOutbound.r, gOutbound.y, s.x+s.r, s.y, 25)" fill="none"/>
      </defs>

      <line v-for="i in 6" :key="'gh'+i" :x1="0" :y1="vh*i/7" :x2="vw" :y2="vh*i/7" stroke="rgba(99,102,241,0.04)" stroke-width="1"/>
      <line v-for="i in 9" :key="'gv'+i" :x1="vw*i/10" :y1="0" :x2="vw*i/10" :y2="vh" stroke="rgba(99,102,241,0.04)" stroke-width="1"/>

      <text :x="colIM" :y="38" text-anchor="middle" font-size="12" fill="rgba(99,102,241,0.45)" font-weight="700" letter-spacing="0.15em">消 息 平 台</text>
      <text :x="colGuard" :y="38" text-anchor="middle" font-size="12" fill="rgba(99,102,241,0.45)" font-weight="700" letter-spacing="0.15em">安 全 检 测</text>
      <text :x="colOC" :y="38" text-anchor="middle" font-size="12" fill="rgba(99,102,241,0.45)" font-weight="700" letter-spacing="0.15em">智 能 体</text>
      <text :x="colLLM" :y="38" text-anchor="middle" font-size="12" fill="rgba(99,102,241,0.45)" font-weight="700" letter-spacing="0.15em">大 模 型</text>
      <text :x="colGuard" :y="68" text-anchor="middle" font-size="15" fill="#a5b4fc" font-weight="800">🦞 龙虾卫士</text>

      <g v-for="s in imNodes" :key="'cl-im-'+s.id">
        <path :d="bezierH(s.x+s.r, s.y, gInbound.x-gInbound.r, gInbound.y)" fill="none" stroke="rgba(99,102,241,0.10)" stroke-width="2" stroke-dasharray="6 4"/>
        <path :d="bezierH(s.x+s.r, s.y, gInbound.x-gInbound.r, gInbound.y)" fill="none" stroke="rgba(34,197,94,0.20)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/>
      </g>
      <path :d="bezierH(gInbound.x+gInbound.r, gInbound.y, ocNode.x-ocNode.r, ocNode.y)" fill="none" stroke="rgba(59,130,246,0.20)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/>
      <path :d="bezierDown(ocNode.x, ocNode.y+ocNode.r, gLLM.x, gLLM.y-gLLM.r)" fill="none" stroke="rgba(139,92,246,0.20)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/>
      <path :d="bezierH(gLLM.x+gLLM.r, gLLM.y, claudeNode.x-claudeNode.r, claudeNode.y)" fill="none" stroke="rgba(139,92,246,0.20)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/>
      <path :d="returnArc(claudeNode.x-claudeNode.r, claudeNode.y, ocNode.x+ocNode.r, ocNode.y, 65)" fill="none" stroke="rgba(168,85,247,0.15)" stroke-width="1" stroke-dasharray="4 6" class="tm-flow-line-rev"/>
      <path :d="bezierDown(ocNode.x, ocNode.y+ocNode.r, gOutbound.x, gOutbound.y-gOutbound.r)" fill="none" stroke="rgba(245,158,11,0.20)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/>
      <g v-for="s in imNodes" :key="'cl-out-'+s.id">
        <path :d="returnArcH(gOutbound.x-gOutbound.r, gOutbound.y, s.x+s.r, s.y, 25)" fill="none" stroke="rgba(245,158,11,0.12)" stroke-width="1" stroke-dasharray="4 6" class="tm-flow-line-rev"/>
      </g>

      <text :x="(colIM+colGuard)/2" :y="gInbound.y - 22" text-anchor="middle" font-size="10" fill="rgba(34,197,94,0.55)" font-weight="600">① 用户消息</text>
      <text :x="(colGuard+colOC)/2" :y="gInbound.y - 12" text-anchor="middle" font-size="10" fill="rgba(59,130,246,0.55)" font-weight="600">② 检测通过</text>
      <text :x="(colGuard+colOC)/2 + 20" :y="(ocNode.y+gLLM.y)/2 - 5" text-anchor="middle" font-size="10" fill="rgba(139,92,246,0.55)" font-weight="600">③ LLM调用</text>
      <text :x="(colGuard+colLLM)/2 + 60" :y="gLLM.y - 18" text-anchor="middle" font-size="10" fill="rgba(139,92,246,0.5)" font-weight="600">④ API请求</text>
      <text :x="(colOC+colLLM)/2" :y="claudeNode.y + 55" text-anchor="middle" font-size="10" fill="rgba(168,85,247,0.5)" font-weight="600">⑤ LLM响应</text>
      <text :x="(colGuard+colOC)/2 + 20" :y="(ocNode.y+gOutbound.y)/2 + 18" text-anchor="middle" font-size="10" fill="rgba(245,158,11,0.55)" font-weight="600">⑥ Agent回复</text>
      <text :x="(colIM+colGuard)/2" :y="gOutbound.y + 28" text-anchor="middle" font-size="10" fill="rgba(245,158,11,0.50)" font-weight="600">⑦ 消息送达</text>

      <template v-for="p in particles" :key="p.id">
        <circle v-if="p.seg==='im-inbound'" :r="p.r||4" :fill="p.color" opacity="0" :filter="p.filter">
          <animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath :href="'#path-im-inbound-'+p.nodeId"/></animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
        <circle v-if="p.seg==='inbound-oc'" :r="p.r||3.5" :fill="p.color" opacity="0" :filter="p.filter">
          <animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath href="#path-inbound-oc"/></animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
        <circle v-if="p.seg==='oc-llmproxy'" :r="p.r||3.5" :fill="p.color" opacity="0" :filter="p.filter">
          <animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath href="#path-oc-llmproxy"/></animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
        <circle v-if="p.seg==='llmproxy-claude'" :r="p.r||3.5" :fill="p.color" opacity="0" :filter="p.filter">
          <animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath href="#path-llmproxy-claude"/></animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
        <circle v-if="p.seg==='claude-oc'" :r="p.r||3" :fill="p.color" opacity="0" :filter="p.filter">
          <animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath href="#path-claude-oc"/></animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
        <circle v-if="p.seg==='oc-outbound'" :r="p.r||3.5" :fill="p.color" opacity="0" :filter="p.filter">
          <animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath href="#path-oc-outbound"/></animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
        <circle v-if="p.seg==='outbound-im'" :r="p.r||3" :fill="p.color" opacity="0" :filter="p.filter">
          <animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath :href="'#path-outbound-im-'+p.nodeId"/></animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
        <circle v-if="p.seg==='block-inbound'" :cx="gInbound.x" :cy="gInbound.y" r="0" fill="none" stroke="#ef4444" stroke-width="2.5" opacity="0">
          <animate attributeName="r" values="0;36" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/>
          <animate attributeName="opacity" values="0.9;0" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
        <circle v-if="p.seg==='block-llm'" :cx="gLLM.x" :cy="gLLM.y" r="0" fill="none" stroke="#ef4444" stroke-width="2.5" opacity="0">
          <animate attributeName="r" values="0;34" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/>
          <animate attributeName="opacity" values="0.9;0" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
        <circle v-if="p.seg==='block-outbound'" :cx="gOutbound.x" :cy="gOutbound.y" r="0" fill="none" stroke="#ef4444" stroke-width="2.5" opacity="0">
          <animate attributeName="r" values="0;36" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/>
          <animate attributeName="opacity" values="0.9;0" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
      </template>

      <g v-for="s in imNodes" :key="'sn-'+s.id" class="tm-node" @click.stop="selectNode('im',s)">
        <circle :cx="s.x" :cy="s.y" :r="s.r+6" fill="none" :stroke="s.color" stroke-width="1.5" opacity="0.3" class="tm-pulse"/>
        <circle :cx="s.x" :cy="s.y" :r="s.r" fill="rgba(15,15,35,0.9)" :stroke="s.color" stroke-width="2"/>
        <circle :cx="s.x" :cy="s.y" :r="s.r-3" fill="none" :stroke="s.color" stroke-width="0.5" opacity="0.4"/>
        <text :x="s.x" :y="s.y+1" text-anchor="middle" dominant-baseline="middle" :font-size="s.r>28?16:13" :fill="s.color" font-weight="800" font-family="'JetBrains Mono',monospace" letter-spacing="0.05em">{{ s.icon }}</text>
        <text :x="s.x" :y="s.y+s.r+14" text-anchor="middle" font-size="10" fill="#94a3b8" font-weight="600">{{ s.label }}</text>
      </g>

      <g v-for="g in guardNodesList" :key="'gn-'+g.id" class="tm-node" @click.stop="selectNode('guard',g)">
        <circle :cx="g.x" :cy="g.y" :r="g.r+8" fill="none" :stroke="g.color" stroke-width="1.5" opacity="0.25" class="tm-pulse"/>
        <circle :cx="g.x" :cy="g.y" :r="g.r" fill="rgba(10,10,30,0.95)" :stroke="g.color" stroke-width="2.5"/>
        <circle :cx="g.x" :cy="g.y" :r="g.r-4" fill="none" :stroke="g.color" stroke-width="0.8" opacity="0.3" stroke-dasharray="3 2"/>
        <text :x="g.x" :y="g.y+1" text-anchor="middle" dominant-baseline="middle" :font-size="g.id==='llm-proxy'?11:13" :fill="g.color" font-weight="800" font-family="'JetBrains Mono',monospace" letter-spacing="0.05em">{{ g.icon }}</text>
        <text :x="g.x" :y="g.y+g.r+14" text-anchor="middle" font-size="10" fill="#94a3b8" font-weight="600">{{ g.label }}</text>
        <text :x="g.x" :y="g.y+g.r+26" text-anchor="middle" font-size="9" fill="#475569" font-family="'JetBrains Mono',monospace">{{ g.sublabel }}</text>
      </g>

      <g class="tm-node" @click.stop="selectNode('openclaw',ocNode)">
        <circle :cx="ocNode.x" :cy="ocNode.y" :r="ocNode.r+8" fill="none" stroke="#818cf8" stroke-width="1.5" opacity="0.25" class="tm-pulse"/>
        <circle :cx="ocNode.x" :cy="ocNode.y" :r="ocNode.r" fill="rgba(10,10,30,0.95)" stroke="#818cf8" stroke-width="2.5"/>
        <circle :cx="ocNode.x" :cy="ocNode.y" :r="ocNode.r-4" fill="none" stroke="#818cf8" stroke-width="0.5" opacity="0.4"/>
        <text :x="ocNode.x" :y="ocNode.y+1" text-anchor="middle" dominant-baseline="middle" font-size="16" fill="#818cf8" font-weight="800" font-family="'JetBrains Mono',monospace" letter-spacing="0.05em">OC</text>
        <text :x="ocNode.x" :y="ocNode.y+ocNode.r+14" text-anchor="middle" font-size="11" fill="#94a3b8" font-weight="600">OpenClaw</text>
        <text :x="ocNode.x" :y="ocNode.y+ocNode.r+26" text-anchor="middle" font-size="9" fill="#475569">Agent</text>
      </g>

      <g class="tm-node" @click.stop="selectNode('claude',claudeNode)">
        <circle :cx="claudeNode.x" :cy="claudeNode.y" :r="claudeNode.r+7" fill="none" stroke="#a78bfa" stroke-width="1.5" opacity="0.25" class="tm-pulse"/>
        <circle :cx="claudeNode.x" :cy="claudeNode.y" :r="claudeNode.r" fill="rgba(10,10,30,0.95)" stroke="#a78bfa" stroke-width="2.5"/>
        <circle :cx="claudeNode.x" :cy="claudeNode.y" :r="claudeNode.r-4" fill="none" stroke="#a78bfa" stroke-width="0.5" opacity="0.4"/>
        <text :x="claudeNode.x" :y="claudeNode.y+1" text-anchor="middle" dominant-baseline="middle" font-size="14" fill="#a78bfa" font-weight="800" font-family="'JetBrains Mono',monospace" letter-spacing="0.05em">AI</text>
        <text :x="claudeNode.x" :y="claudeNode.y+claudeNode.r+14" text-anchor="middle" font-size="11" fill="#94a3b8" font-weight="600">Claude API</text>
        <text :x="claudeNode.x" :y="claudeNode.y+claudeNode.r+26" text-anchor="middle" font-size="9" fill="#475569">Anthropic</text>
      </g>
    </svg>

    <transition name="tm-panel-fade">
      <div class="tm-detail-panel" v-if="selectedNode" @click.stop>
        <button class="tm-panel-close" @click="selectedNode=null">✕</button>
        <template v-if="selectedNode.type==='im'">
          <div class="tm-panel-title">{{ selectedNode.data.icon }} {{ selectedNode.data.label }}</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ srcStat(selectedNode.data.id).requests }}</div><div class="tm-pg-label">请求数</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-red">{{ srcStat(selectedNode.data.id).blocked }}</div><div class="tm-pg-label">拦截</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-orange">{{ srcStat(selectedNode.data.id).warned }}</div><div class="tm-pg-label">告警</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ srcStat(selectedNode.data.id).blockRate }}%</div><div class="tm-pg-label">拦截率</div></div>
          </div>
          <div class="tm-panel-subtitle">最近事件</div>
          <div class="tm-panel-events">
            <div class="tm-pe-row" v-for="e in srcEvents(selectedNode.data.id)" :key="e.id"><span class="tm-pe-time">{{ fmtT(e.timestamp) }}</span><span class="tm-pe-action" :class="'a-'+e.action">{{ e.action }}</span><span class="tm-pe-desc">{{ trn(e.reason||e.content_preview||'-',30) }}</span></div>
            <div v-if="!srcEvents(selectedNode.data.id).length" class="tm-pe-empty">暂无事件</div>
          </div>
        </template>
        <template v-if="selectedNode.type==='guard' && selectedNode.data.id==='inbound'">
          <div class="tm-panel-title">🛡️ 入站检测 :18443</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ guardStat('inbound').total }}</div><div class="tm-pg-label">检测总数</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-red">{{ guardStat('inbound').blocked }}</div><div class="tm-pg-label">拦截</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-orange">{{ guardStat('inbound').warned }}</div><div class="tm-pg-label">告警</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ guardStat('inbound').rate }}%</div><div class="tm-pg-label">拦截率</div></div>
          </div>
          <div class="tm-panel-subtitle">Prompt Injection 检测</div>
          <div class="tm-engine-list">
            <div class="tm-eng-row"><span class="tm-eng-name">模式匹配</span><div class="tm-eng-bar-bg"><div class="tm-eng-bar" :style="{width:engBarW(engineLayers[0].count)+'%',background:'#6366f1'}"></div></div><span class="tm-eng-count">{{ engineLayers[0].count }}</span></div>
            <div class="tm-eng-row"><span class="tm-eng-name">语义检测</span><div class="tm-eng-bar-bg"><div class="tm-eng-bar" :style="{width:engBarW(engineLayers[1].count)+'%',background:'#818cf8'}"></div></div><span class="tm-eng-count">{{ engineLayers[1].count }}</span></div>
          </div>
        </template>
        <template v-if="selectedNode.type==='guard' && selectedNode.data.id==='llm-proxy'">
          <div class="tm-panel-title">🔬 LLM 检测 :8445</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ guardStat('llm').total }}</div><div class="tm-pg-label">检测总数</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-red">{{ guardStat('llm').blocked }}</div><div class="tm-pg-label">拦截</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-orange">{{ guardStat('llm').warned }}</div><div class="tm-pg-label">告警</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ guardStat('llm').rate }}%</div><div class="tm-pg-label">拦截率</div></div>
          </div>
          <div class="tm-panel-subtitle">tool_calls / 语义检测</div>
          <div class="tm-engine-list">
            <div class="tm-eng-row"><span class="tm-eng-name">行为分析</span><div class="tm-eng-bar-bg"><div class="tm-eng-bar" :style="{width:engBarW(engineLayers[2].count)+'%',background:'#a78bfa'}"></div></div><span class="tm-eng-count">{{ engineLayers[2].count }}</span></div>
            <div class="tm-eng-row"><span class="tm-eng-name">密码学信封</span><div class="tm-eng-bar-bg"><div class="tm-eng-bar" :style="{width:engBarW(engineLayers[3].count)+'%',background:'#818cf8'}"></div></div><span class="tm-eng-count">{{ engineLayers[3].count }}</span></div>
          </div>
        </template>
        <template v-if="selectedNode.type==='guard' && selectedNode.data.id==='outbound'">
          <div class="tm-panel-title">📤 出站检测 :18444</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ guardStat('outbound').total }}</div><div class="tm-pg-label">检测总数</div></div>
            <div class="