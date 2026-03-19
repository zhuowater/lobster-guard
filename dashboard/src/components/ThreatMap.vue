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
      <g v-for="s in imNodes" :key="'cl-im-'+s.id"><path :d="bezierH(s.x+s.r, s.y, gInbound.x-gInbound.r, gInbound.y)" fill="none" stroke="rgba(34,197,94,0.18)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/></g>
      <path :d="bezierH(gInbound.x+gInbound.r, gInbound.y, ocNode.x-ocNode.r, ocNode.y)" fill="none" stroke="rgba(59,130,246,0.20)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/>
      <path :d="bezierDown(ocNode.x, ocNode.y+ocNode.r, gLLM.x, gLLM.y-gLLM.r)" fill="none" stroke="rgba(139,92,246,0.20)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/>
      <path :d="bezierH(gLLM.x+gLLM.r, gLLM.y, claudeNode.x-claudeNode.r, claudeNode.y)" fill="none" stroke="rgba(139,92,246,0.20)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/>
      <path :d="returnArc(claudeNode.x-claudeNode.r, claudeNode.y, ocNode.x+ocNode.r, ocNode.y, 65)" fill="none" stroke="rgba(168,85,247,0.15)" stroke-width="1" stroke-dasharray="4 6" class="tm-flow-line-rev"/>
      <path :d="bezierDown(ocNode.x, ocNode.y+ocNode.r, gOutbound.x, gOutbound.y-gOutbound.r)" fill="none" stroke="rgba(245,158,11,0.20)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/>
      <g v-for="s in imNodes" :key="'cl-out-'+s.id"><path :d="returnArcH(gOutbound.x-gOutbound.r, gOutbound.y, s.x+s.r, s.y, 25)" fill="none" stroke="rgba(245,158,11,0.12)" stroke-width="1" stroke-dasharray="4 6" class="tm-flow-line-rev"/></g>
      <text :x="(colIM+colGuard)/2" :y="gInbound.y-22" text-anchor="middle" font-size="10" fill="rgba(34,197,94,0.55)" font-weight="600">① 用户消息</text>
      <text :x="(colGuard+colOC)/2" :y="gInbound.y-12" text-anchor="middle" font-size="10" fill="rgba(59,130,246,0.55)" font-weight="600">② 检测通过</text>
      <text :x="colGuard+160" :y="(ocNode.y+gLLM.y)/2-5" text-anchor="middle" font-size="10" fill="rgba(139,92,246,0.55)" font-weight="600">③ LLM调用</text>
      <text :x="(gLLM.x+claudeNode.x)/2" :y="gLLM.y-18" text-anchor="middle" font-size="10" fill="rgba(139,92,246,0.5)" font-weight="600">④ API请求</text>
      <text :x="(colOC+colLLM)/2" :y="claudeNode.y+55" text-anchor="middle" font-size="10" fill="rgba(168,85,247,0.5)" font-weight="600">⑤ LLM响应</text>
      <text :x="colGuard+160" :y="(ocNode.y+gOutbound.y)/2+18" text-anchor="middle" font-size="10" fill="rgba(245,158,11,0.55)" font-weight="600">⑥ Agent回复</text>
      <text :x="(colIM+colGuard)/2" :y="gOutbound.y+28" text-anchor="middle" font-size="10" fill="rgba(245,158,11,0.50)" font-weight="600">⑦ 消息送达</text>
      <template v-for="p in particles" :key="p.id">
        <circle v-if="p.seg==='im-inbound'" :r="p.r||4" :fill="p.color" opacity="0" :filter="p.filter"><animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath :href="'#path-im-inbound-'+p.nodeId"/></animateMotion><animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/></circle>
        <circle v-if="p.seg==='inbound-oc'" :r="p.r||3.5" :fill="p.color" opacity="0" :filter="p.filter"><animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath href="#path-inbound-oc"/></animateMotion><animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/></circle>
        <circle v-if="p.seg==='oc-llmproxy'" :r="p.r||3.5" :fill="p.color" opacity="0" :filter="p.filter"><animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath href="#path-oc-llmproxy"/></animateMotion><animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/></circle>
        <circle v-if="p.seg==='llmproxy-claude'" :r="p.r||3.5" :fill="p.color" opacity="0" :filter="p.filter"><animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath href="#path-llmproxy-claude"/></animateMotion><animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/></circle>
        <circle v-if="p.seg==='claude-oc'" :r="p.r||3" :fill="p.color" opacity="0" :filter="p.filter"><animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath href="#path-claude-oc"/></animateMotion><animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/></circle>
        <circle v-if="p.seg==='oc-outbound'" :r="p.r||3.5" :fill="p.color" opacity="0" :filter="p.filter"><animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath href="#path-oc-outbound"/></animateMotion><animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/></circle>
        <circle v-if="p.seg==='outbound-im'" :r="p.r||3" :fill="p.color" opacity="0" :filter="p.filter"><animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath :href="'#path-outbound-im-'+p.nodeId"/></animateMotion><animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/></circle>
        <circle v-if="p.seg==='block-inbound'" :cx="gInbound.x" :cy="gInbound.y" r="0" fill="none" stroke="#ef4444" stroke-width="2.5" opacity="0"><animate attributeName="r" values="0;36" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/><animate attributeName="opacity" values="0.9;0" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/></circle>
        <circle v-if="p.seg==='block-llm'" :cx="gLLM.x" :cy="gLLM.y" r="0" fill="none" stroke="#ef4444" stroke-width="2.5" opacity="0"><animate attributeName="r" values="0;34" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/><animate attributeName="opacity" values="0.9;0" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/></circle>
        <circle v-if="p.seg==='block-outbound'" :cx="gOutbound.x" :cy="gOutbound.y" r="0" fill="none" stroke="#ef4444" stroke-width="2.5" opacity="0"><animate attributeName="r" values="0;36" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/><animate attributeName="opacity" values="0.9;0" dur="0.7s" :begin="p.begin+'s'" fill="freeze"/></circle>
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
        <template v-if="selectedNode.type==='guard'&&selectedNode.data.id==='inbound'">
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
        <template v-if="selectedNode.type==='guard'&&selectedNode.data.id==='llm-proxy'">
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
        <template v-if="selectedNode.type==='guard'&&selectedNode.data.id==='outbound'">
          <div class="tm-panel-title">📤 出站检测 :18444</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ guardStat('outbound').total }}</div><div class="tm-pg-label">检测总数</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-red">{{ guardStat('outbound').blocked }}</div><div class="tm-pg-label">拦截</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-orange">{{ guardStat('outbound').warned }}</div><div class="tm-pg-label">告警</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ guardStat('outbound').rate }}%</div><div class="tm-pg-label">拦截率</div></div>
          </div>
          <div class="tm-panel-subtitle">PII / 敏感信息检测</div>
          <div class="tm-engine-list">
            <div class="tm-eng-row"><span class="tm-eng-name">自进化引擎</span><div class="tm-eng-bar-bg"><div class="tm-eng-bar" :style="{width:engBarW(engineLayers[4].count)+'%',background:'#f59e0b'}"></div></div><span class="tm-eng-count">{{ engineLayers[4].count }}</span></div>
          </div>
        </template>
        <template v-if="selectedNode.type==='openclaw'">
          <div class="tm-panel-title">🤖 OpenClaw Agent</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num tm-green">● 在线</div><div class="tm-pg-label">状态</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ avgLatency }}ms</div><div class="tm-pg-label">延迟</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ summary.total_requests }}</div><div class="tm-pg-label">处理数</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num" :style="{color:sc(healthScore)}">{{ healthScore }}</div><div class="tm-pg-label">健康分</div></div>
          </div>
        </template>
        <template v-if="selectedNode.type==='claude'">
          <div class="tm-panel-title">🧠 Claude API</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num tm-green">● 在线</div><div class="tm-pg-label">状态</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">85ms</div><div class="tm-pg-label">延迟</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ Math.max(summary.total_requests - summary.blocked, 0) }}</div><div class="tm-pg-label">请求数</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">Anthropic</div><div class="tm-pg-label">提供商</div></div>
          </div>
        </template>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, reactive } from 'vue'
import { api } from '../api.js'

const vw = 1400, vh = 600
const colIM = 120, colGuard = 440, colOC = 780, colLLM = 1100
const clock = ref('')
let clockT = null
function updClock() { const n = new Date(); clock.value = n.getFullYear()+'-'+P(n.getMonth()+1)+'-'+P(n.getDate())+' '+P(n.getHours())+':'+P(n.getMinutes())+':'+P(n.getSeconds()) }
function P(v) { return String(v).padStart(2, '0') }

const summary = reactive({ total_requests: 0, blocked: 0, warned: 0, passed: 0 })
const healthScore = ref(100)
const avgLatency = ref(3)
const auditLogs = ref([])
const particles = ref([])
let pid = 0, svgT0 = 0

const imNodes = computed(() => {
  const items = [
    { id: 'lanxin', icon: 'LX', label: '蓝信', color: '#6366f1', r: 32 },
    { id: 'feishu', icon: 'FS', label: '飞书', color: '#3b82f6', r: 24 },
    { id: 'dingtalk', icon: 'DD', label: '钉钉', color: '#22c55e', r: 24 },
    { id: 'wecom', icon: 'WX', label: '企微', color: '#f59e0b', r: 24 },
    { id: 'slack', icon: 'SK', label: 'Slack', color: '#a855f7', r: 24 },
  ]
  const sp = (vh - 140) / (items.length + 1)
  items.forEach((it, i) => { it.x = colIM; it.y = 90 + sp * (i + 1) })
  return items
})

// Guard nodes: 3 proxies, vertically arranged in column 2
const gInbound = reactive({ id: 'inbound', icon: 'IN', label: '入站检测', sublabel: ':18443', color: '#6366f1', r: 36, x: colGuard, y: 150 })
const gLLM = reactive({ id: 'llm-proxy', icon: 'LLM', label: 'LLM 检测', sublabel: ':8445', color: '#a78bfa', r: 34, x: colGuard, y: 300 })
const gOutbound = reactive({ id: 'outbound', icon: 'OUT', label: '出站检测', sublabel: ':18444', color: '#f59e0b', r: 36, x: colGuard, y: 450 })
const guardNodesList = computed(() => [gInbound, gLLM, gOutbound])

const ocNode = reactive({ id: 'openclaw', icon: 'OC', label: 'OpenClaw', sublabel: 'Agent', color: '#818cf8', r: 40, x: colOC, y: 220 })
const claudeNode = reactive({ id: 'claude', icon: 'AI', label: 'Claude API', sublabel: 'Anthropic', color: '#a78bfa', r: 36, x: colLLM, y: 300 })

const engineLayers = ref([
  { label: 'L1: 模式匹配', count: 0, active: true },
  { label: 'L2: 语义检测', count: 0, active: true },
  { label: 'L3: 行为分析', count: 0, active: true },
  { label: 'L4: 密码学信封', count: 0, active: false },
  { label: 'L5: 自进化引擎', count: 0, active: false },
])

function engBarW(c) { const mx = Math.max(...engineLayers.value.map(l => l.count), 1); return Math.min((c / mx) * 100, 100) }

// Horizontal bezier (left to right)
function bezierH(x1, y1, x2, y2) {
  const dx = (x2 - x1) * 0.4
  return 'M'+x1+','+y1+' C'+(x1+dx)+','+y1+' '+(x2-dx)+','+y2+' '+x2+','+y2
}

// Bezier going down (from top node to lower-left node)
function bezierDown(x1, y1, x2, y2) {
  const my = (y1 + y2) / 2
  return 'M'+x1+','+y1+' C'+x1+','+(my)+' '+x2+','+(my)+' '+x2+','+y2
}

// Return arc (right to left, curving downward)
function returnArc(x1, y1, x2, y2, yOffset) {
  const mx = (x1 + x2) / 2
  const cy1 = y1 + yOffset
  const cy2 = y2 + yOffset
  return 'M'+x1+','+y1+' C'+x1+','+cy1+' '+x2+','+cy2+' '+x2+','+y2
}

// Return arc horizontal (from guard outbound back to IM, curving below)
function returnArcH(x1, y1, x2, y2, yOffset) {
  const dx = (x1 - x2) * 0.35
  const cy1 = y1 + yOffset
  const cy2 = y2 + yOffset
  return 'M'+x1+','+y1+' C'+(x1-dx)+','+cy1+' '+(x2+dx)+','+cy2+' '+x2+','+y2
}

function sc(s) { return s>=90?'#22c55e':s>=70?'#84cc16':s>=50?'#f59e0b':s>=30?'#f97316':'#ef4444' }
function fmtT(ts) { if(!ts)return''; const d=new Date(ts); return P(d.getHours())+':'+P(d.getMinutes())+':'+P(d.getSeconds()) }
function trn(s, n) { return s&&s.length>n?s.slice(0,n)+'…':(s||'-') }

const selectedNode = ref(null)
function selectNode(type, data) {
  if (selectedNode.value && selectedNode.value.type===type && selectedNode.value.data?.id===data?.id) { selectedNode.value=null; return }
  selectedNode.value = { type, data }
}

function srcStat(id) {
  const idx = imNodes.value.findIndex(s => s.id === id)
  const evts = auditLogs.value.filter((_, i) => (i % imNodes.value.length) === idx)
  const b = evts.filter(e => e.action === 'block').length
  const w = evts.filter(e => e.action === 'warn').length
  const t = evts.length
  return { requests: t, blocked: b, warned: w, blockRate: t > 0 ? ((b / t) * 100).toFixed(1) : '0.0' }
}
function srcEvents(id) {
  const idx = imNodes.value.findIndex(s => s.id === id)
  return auditLogs.value.filter((_, i) => (i % imNodes.value.length) === idx).slice(0, 5)
}

function guardStat(type) {
  // Distribute audit logs across guard types based on direction field or round-robin
  let evts
  if (type === 'inbound') {
    evts = auditLogs.value.filter(e => e.direction === 'inbound' || (!e.direction && auditLogs.value.indexOf(e) % 3 === 0))
  } else if (type === 'llm') {
    evts = auditLogs.value.filter(e => e.direction === 'llm' || (!e.direction && auditLogs.value.indexOf(e) % 3 === 1))
  } else {
    evts = auditLogs.value.filter(e => e.direction === 'outbound' || (!e.direction && auditLogs.value.indexOf(e) % 3 === 2))
  }
  const b = evts.filter(e => e.action === 'block').length
  const w = evts.filter(e => e.action === 'warn').length
  const t = evts.length
  return { total: t, blocked: b, warned: w, rate: t > 0 ? ((b / t) * 100).toFixed(1) : '0.0' }
}

// Spawn particles for real audit events
function spawnFromEvents(events) {
  if (!events || !events.length) return
  const now = (Date.now() - svgT0) / 1000
  const sIds = imNodes.value.map(s => s.id)
  const np = []

  events.forEach((ev, i) => {
    const act = ev.action || 'pass'
    const dir = ev.direction || ['inbound', 'llm', 'outbound'][i % 3]
    const si = sIds[i % sIds.length]
    const d = now + i * 0.8

    if (dir === 'inbound') {
      // IM -> Inbound
      np.push({ id: ++pid, seg: 'im-inbound', nodeId: si, color: act === 'block' ? '#ef4444' : '#22c55e', filter: act === 'block' ? 'url(#glow-red)' : 'url(#glow-green)', dur: 1.5, begin: d, r: 4 })
      if (act === 'block') {
        np.push({ id: ++pid, seg: 'block-inbound', begin: d + 1.5, dur: 0.7 })
      } else {
        // Continue: Inbound -> OC
        np.push({ id: ++pid, seg: 'inbound-oc', color: '#3b82f6', filter: 'url(#glow-blue)', dur: 1, begin: d + 1.8, r: 3.5 })
      }
    } else if (dir === 'llm') {
      // OC -> LLM Proxy
      np.push({ id: ++pid, seg: 'oc-llmproxy', color: '#8b5cf6', filter: 'url(#glow-purple)', dur: 1.2, begin: d, r: 3.5 })
      if (act === 'block') {
        np.push({ id: ++pid, seg: 'block-llm', begin: d + 1.2, dur: 0.7 })
      } else {
        // LLM Proxy -> Claude
        np.push({ id: ++pid, seg: 'llmproxy-claude', color: '#a78bfa', filter: 'url(#glow-purple)', dur: 1.5, begin: d + 1.5, r: 3.5 })
      }
    } else {
      // OC -> Outbound
      np.push({ id: ++pid, seg: 'oc-outbound', color: '#f59e0b', filter: 'url(#glow-orange)', dur: 1.2, begin: d, r: 3.5 })
      if (act === 'block') {
        np.push({ id: ++pid, seg: 'block-outbound', begin: d + 1.2, dur: 0.7 })
      } else {
        // Outbound -> IM
        np.push({ id: ++pid, seg: 'outbound-im', nodeId: si, color: '#22c55e', filter: 'url(#glow-green)', dur: 1.8, begin: d + 1.5, r: 3 })
      }
    }
  })

  const cut = now - 25
  particles.value = [...particles.value.filter(p => p.begin + (p.dur || 1) > cut), ...np]
}

// Ambient particles: full chain every 4 seconds
function spawnAmbient() {
  const now = (Date.now() - svgT0) / 1000
  const sIds = imNodes.value.map(s => s.id)
  const np = []
  const cnt = 1 + Math.floor(Math.random() * 2)

  for (let i = 0; i < cnt; i++) {
    const si = sIds[Math.floor(Math.random() * sIds.length)]
    const d = now + Math.random() * 2

    // Full chain: IM -> Inbound (green)
    np.push({ id: ++pid, seg: 'im-inbound', nodeId: si, color: '#22c55e', filter: 'url(#glow-green)', dur: 1.5, begin: d, r: 4 })
    // Inbound -> OC (blue)
    np.push({ id: ++pid, seg: 'inbound-oc', color: '#3b82f6', filter: 'url(#glow-blue)', dur: 1.0, begin: d + 1.8, r: 3.5 })
    // OC -> LLM Proxy (purple)
    np.push({ id: ++pid, seg: 'oc-llmproxy', color: '#8b5cf6', filter: 'url(#glow-purple)', dur: 1.2, begin: d + 3.2, r: 3.5 })
    // LLM Proxy -> Claude (purple)
    np.push({ id: ++pid, seg: 'llmproxy-claude', color: '#a78bfa', filter: 'url(#glow-purple)', dur: 1.5, begin: d + 4.7, r: 3.5 })
    // Claude -> OC (return, light purple)
    np.push({ id: ++pid, seg: 'claude-oc', color: '#c4b5fd', filter: 'url(#glow-purple)', dur: 1.5, begin: d + 6.5, r: 3 })
    // OC -> Outbound (orange)
    np.push({ id: ++pid, seg: 'oc-outbound', color: '#f59e0b', filter: 'url(#glow-orange)', dur: 1.2, begin: d + 8.3, r: 3.5 })
    // Outbound -> IM (return, green)
    np.push({ id: ++pid, seg: 'outbound-im', nodeId: si, color: '#22c55e', filter: 'url(#glow-green)', dur: 1.8, begin: d + 9.8, r: 3 })
  }

  const cut = now - 25
  particles.value = [...particles.value.filter(p => p.begin + (p.dur || 1) > cut), ...np]
}

let prevLogIds = new Set()
async function fetchData() {
  try {
    const [sumR, hR, lR] = await Promise.allSettled([
      api('/api/v1/overview/summary'),
      api('/api/v1/health/score'),
      api('/api/v1/audit/logs?limit=10')
    ])
    if (sumR.status === 'fulfilled' && sumR.value) {
      const d = sumR.value
      summary.total_requests = d.total_requests || d.total || 0
      summary.blocked = d.blocked_requests || d.blocked || 0
      summary.warned = d.warned_requests || d.warned || 0
      summary.passed = Math.max(summary.total_requests - summary.blocked - summary.warned, 0)
    }
    if (hR.status === 'fulfilled' && hR.value) {
      const h = hR.value
      healthScore.value = h.score || 100
      avgLatency.value = h.avg_latency_ms || h.latency || 3
      if (h.layer_stats || h.details) {
        const ls = h.layer_stats || h.details
        const ly = engineLayers.value
        if (ls.pattern_match !== undefined) ly[0].count = ls.pattern_match
        if (ls.semantic !== undefined) ly[1].count = ls.semantic
        if (ls.behavior !== undefined) ly[2].count = ls.behavior
        if (ls.envelope !== undefined) { ly[3].count = ls.envelope; ly[3].active = ls.envelope > 0 }
        if (ls.evolution !== undefined) { ly[4].count = ls.evolution; ly[4].active = ls.evolution > 0 }
      }
    }
    if (lR.status === 'fulfilled' && lR.value) {
      const logs = Array.isArray(lR.value) ? lR.value : (lR.value.logs || lR.value.items || [])
      auditLogs.value = logs
      const newLogs = logs.filter(l => !prevLogIds.has(l.id))
      if (newLogs.length > 0) { spawnFromEvents(newLogs); prevLogIds = new Set(logs.map(l => l.id)) }
    }
  } catch (e) { console.error('[ThreatMap] fetch error', e) }
}

let dataT = null, ambientT = null
onMounted(() => {
  svgT0 = Date.now(); updClock(); clockT = setInterval(updClock, 1000)
  fetchData(); dataT = setInterval(fetchData, 5000)
  spawnAmbient(); ambientT = setInterval(spawnAmbient, 4000)
})
onUnmounted(() => { clearInterval(clockT); clearInterval(dataT); clearInterval(ambientT) })
</script>

<style scoped>
.threat-map-root{position:absolute;inset:0;background:#050510;overflow:hidden;display:flex;flex-direction:column;align-items:center;justify-content:center;font-family:'Inter',-apple-system,BlinkMacSystemFont,sans-serif;color:#e2e8f0}
.tm-hud-top-left{position:absolute;top:12px;left:20px;display:flex;align-items:center;gap:14px;z-index:10}
.tm-live-badge{display:flex;align-items:center;gap:6px;background:rgba(239,68,68,0.15);border:1px solid rgba(239,68,68,0.3);border-radius:20px;padding:3px 12px 3px 8px;font-size:11px;font-weight:800;color:#f87171;letter-spacing:0.08em}
.tm-live-dot{width:8px;height:8px;border-radius:50%;background:#ef4444;animation:tm-blink 1.2s ease-in-out infinite}
@keyframes tm-blink{0%,100%{opacity:1}50%{opacity:0.3}}
.tm-clock{font-family:'JetBrains Mono',monospace;font-size:12px;color:#64748b;letter-spacing:0.05em}
.tm-stats-bar{position:absolute;bottom:16px;left:50%;transform:translateX(-50%);display:flex;gap:24px;z-index:10;background:rgba(10,10,25,0.7);backdrop-filter:blur(10px);-webkit-backdrop-filter:blur(10px);border:1px solid rgba(99,102,241,0.15);border-radius:12px;padding:8px 28px}
.tm-stat{display:flex;flex-direction:column;align-items:center;gap:2px}
.tm-stat-label{font-size:10px;color:#475569;font-weight:500}
.tm-stat-val{font-size:18px;font-weight:800;color:#818cf8;font-family:'JetBrains Mono',monospace}
.tm-stat-val.tm-red{color:#ef4444}.tm-stat-val.tm-orange{color:#f59e0b}.tm-stat-val.tm-green{color:#22c55e}
.tm-svg{width:95%;max-height:82vh;flex-shrink:0}
.tm-flow-line{animation:tm-dash 1.5s linear infinite}
.tm-flow-line-rev{animation:tm-dash-rev 2s linear infinite}
@keyframes tm-dash{to{stroke-dashoffset:-20px}}
@keyframes tm-dash-rev{to{stroke-dashoffset:20px}}
.tm-node{cursor:pointer;transition:transform 0.2s}.tm-node:hover{filter:brightness(1.3)}
.tm-pulse{animation:tm-pulse-ring 2.5s ease-in-out infinite}
@keyframes tm-pulse-ring{0%{r:inherit;opacity:0.3}50%{opacity:0.1}100%{opacity:0.3}}
.tm-detail-panel{position:absolute;right:20px;top:50%;transform:translateY(-50%);width:280px;background:rgba(10,10,30,0.85);backdrop-filter:blur(16px);-webkit-backdrop-filter:blur(16px);border:1px solid rgba(99,102,241,0.25);border-radius:12px;padding:20px;z-index:20;box-shadow:0 8px 32px rgba(0,0,0,0.5)}
.tm-panel-close{position:absolute;top:8px;right:10px;background:none;border:none;color:#64748b;font-size:14px;cursor:pointer;padding:4px 8px;border-radius:4px}.tm-panel-close:hover{background:rgba(255,255,255,0.1);color:#e2e8f0}
.tm-panel-title{font-size:15px;font-weight:700;margin-bottom:14px;color:#e2e8f0}
.tm-panel-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-bottom:14px}
.tm-pg-cell{text-align:center;padding:8px 4px;background:rgba(255,255,255,0.03);border-radius:8px;border:1px solid rgba(255,255,255,0.06)}
.tm-pg-num{font-size:18px;font-weight:800;color:#818cf8;font-family:'JetBrains Mono',monospace}
.tm-pg-num.tm-red{color:#ef4444}.tm-pg-num.tm-orange{color:#f59e0b}.tm-pg-num.tm-green{color:#22c55e}
.tm-pg-label{font-size:10px;color:#475569;margin-top:2px}
.tm-panel-subtitle{font-size:11px;color:#64748b;font-weight:600;margin-bottom:8px;text-transform:uppercase;letter-spacing:0.05em}
.tm-panel-events{max-height:150px;overflow-y:auto}
.tm-pe-row{display:flex;align-items:center;gap:6px;padding:4px 0;border-bottom:1px solid rgba(255,255,255,0.04);font-size:11px}
.tm-pe-time{color:#475569;font-family:monospace;flex-shrink:0;width:55px}
.tm-pe-action{padding:1px 6px;border-radius:3px;font-weight:700;font-size:10px;flex-shrink:0;text-transform:uppercase}
.a-block{background:rgba(239,68,68,0.15);color:#f87171}.a-warn{background:rgba(245,158,11,0.15);color:#fbbf24}.a-pass{background:rgba(100,116,139,0.1);color:#64748b}
.tm-pe-desc{color:#94a3b8;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.tm-pe-empty{color:#475569;font-size:11px;padding:8px 0;text-align:center}
.tm-engine-list{display:flex;flex-direction:column;gap:6px}
.tm-eng-row{display:flex;align-items:center;gap:8px;font-size:11px}
.tm-eng-name{width:100px;flex-shrink:0;color:#94a3b8}
.tm-eng-bar-bg{flex:1;height:8px;background:rgba(255,255,255,0.05);border-radius:4px;overflow:hidden}
.tm-eng-bar{height:100%;border-radius:4px;transition:width 0.8s ease}
.tm-eng-count{width:30px;text-align:right;font-weight:700;font-family:monospace;color:#818cf8}
.tm-panel-fade-enter-active,.tm-panel-fade-leave-active{transition:opacity 0.3s ease,transform 0.3s ease}
.tm-panel-fade-enter-from{opacity:0;transform:translateY(-50%) translateX(20px)}.tm-panel-fade-leave-to{opacity:0;transform:translateY(-50%) translateX(20px)}
</style>
