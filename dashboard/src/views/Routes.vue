<template>
  <div>
    <!-- Tab 切换 -->
    <div class="route-tabs">
      <button class="route-tab" :class="{ active: activeTab === 'routes' }" @click="activeTab = 'routes'">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="6" y1="3" x2="6" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/></svg>
        亲和路由
        <span v-if="allRoutes.length" class="route-tab-badge">{{ allRoutes.length }}</span>
      </button>
      <button class="route-tab" :class="{ active: activeTab === 'policies' }" @click="activeTab = 'policies'">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/><polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/><line x1="4" y1="4" x2="9" y2="9"/></svg>
        策略路由
        <span v-if="policies.length" class="route-tab-badge">{{ policies.length }}</span>
      </button>
      <button class="route-tab" :class="{ active: activeTab === 'visual' }" @click="activeTab = 'visual'">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="18" cy="18" r="3"/><circle cx="6" cy="6" r="3"/><path d="M13 6h3a2 2 0 0 1 2 2v7"/><path d="M11 18H8a2 2 0 0 1-2-2V9"/></svg>
        可视化
      </button>
    </div>

    <!-- ==================== 路由管理 Tab ==================== -->
    <div v-show="activeTab === 'routes'" class="card">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="6" y1="3" x2="6" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/></svg></span>
        <span class="card-title">亲和路由管理</span>
        <div class="card-actions">
          <button class="btn btn-sm" @click="showBindModal = true"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg> 绑定用户</button>
          <button class="btn btn-ghost btn-sm" @click="showBatchModal = true"><Icon name="import" :size="14" /> 批量绑定</button>
          <button class="btn btn-ghost btn-sm" @click="showMigrateModal = true"><Icon name="refresh" :size="14" /> 迁移用户</button>
          <button class="btn btn-ghost btn-sm" @click="refreshAllUserInfo" :disabled="refreshingAll" :title="'从蓝信批量获取所有用户的最新部门/邮箱信息'">
            <svg v-if="!refreshingAll" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
            <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="spin"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
            {{ refreshingAll ? '获取中...' : '刷新用户信息' }}
          </button>
          <button class="btn btn-ghost btn-sm" @click="refresh"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg></button>
        </div>
      </div>
      <div v-if="routeStats" class="route-stats-bar">
        <div class="route-stat-item"><span class="route-stat-label">Bot</span><span class="route-stat-value" style="color:var(--color-primary)">{{ routeStats.appCount }}</span></div><div class="route-stat-divider"></div>
        <div class="route-stat-item"><span class="route-stat-label">用户</span><span class="route-stat-value" style="color:var(--color-success)">{{ routeStats.senderCount }}</span></div><div class="route-stat-divider"></div>
        <div class="route-stat-item"><span class="route-stat-label">上游</span><span class="route-stat-value" style="color:var(--color-info)">{{ routeStats.upstreamCount }}</span></div><div class="route-stat-divider"></div>
        <div class="route-stat-item"><span class="route-stat-label">路由</span><span class="route-stat-value" style="color:var(--color-warning)">{{ routeStats.total }}</span></div><div class="route-stat-divider"></div>
        <div class="route-stat-item" v-if="conflictCount > 0" @click="toggleConflictFilter" style="cursor:pointer" :title="'点击' + (filterConflict ? '取消' : '') + '筛选冲突路由'">
          <span class="route-stat-label" :style="filterConflict ? 'color:var(--color-danger)' : ''">⚠️ 冲突</span>
          <span class="route-stat-value" style="color:var(--color-danger);font-weight:700">{{ conflictCount }}</span>
        </div>
      </div>
      <div class="filters">
        <select v-model="filterApp"><option value="">全部 Bot</option><option v-for="a in apps" :key="a" :value="a">{{ a.length > 20 ? a.substring(0, 20) + '...' : a }}</option></select>
        <select v-model="filterDept"><option value="">全部部门</option><option v-for="d in depts" :key="d" :value="d">{{ d }}</option></select>
        <select v-model="filterUpstream"><option value="">全部上游</option><option v-for="u in upstreamList" :key="u" :value="u">{{ u }}</option></select>
        <div class="search-input-wrap"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg><input type="text" v-model="searchText" placeholder="搜索 sender_id / upstream_id / 名称..." /></div>
        <select v-model="sortBy" style="min-width:120px"><option value="">默认排序</option><option value="created_desc">绑定时间 ↓</option><option value="created_asc">绑定时间 ↑</option></select>
      </div>
      <div v-if="selectedRoutes.size > 0" class="batch-bar">
        <span class="batch-count">已选 <strong>{{ selectedRoutes.size }}</strong> 项</span>
        <button class="btn btn-ghost btn-sm" @click="selectAllVisible">全选当前</button>
        <button class="btn btn-ghost btn-sm" @click="selectedRoutes.clear()">取消全选</button>
        <div style="flex:1"></div>
        <button class="btn btn-ghost btn-sm" @click="openBatchMigrateSelected"><Icon name="refresh" :size="14" /> 批量迁移</button>
        <button class="btn btn-danger btn-sm" @click="confirmBatchUnbind"><Icon name="trash" :size="14" /> 批量解绑</button>
      </div>
      <DataTable :columns="routeColumns" :data="filteredRoutes" :loading="loading" empty-text="尚未绑定用户" empty-desc="通过上方按钮绑定用户到指定上游" :expandable="true" :show-toolbar="false">
        <template #cell-_select="{ row }">
          <input type="checkbox" :checked="selectedRoutes.has(routeRowKey(row))" @click.stop="toggleSelect(row)" class="route-checkbox" />
        </template>
        <template #cell-sender_id="{ row }"><span style="font-size:.75rem;font-family:var(--font-mono)">{{ row.sender_id }}</span></template>
        <template #cell-app_id="{ row }"><span style="font-size:.75rem" :title="row.app_id">{{ (row.app_id||'--').length > 16 ? row.app_id.substring(0,16)+'...' : (row.app_id||'--') }}</span></template>
        <template #cell-upstream_id="{ row }">
          <span v-if="row.policy_conflict" class="tag policy-conflict-tag" :title="'策略冲突: 当前绑定 ' + row.upstream_id + '，策略指定 ' + row.policy_upstream + '\n规则: ' + (row.policy_rule || '未知')">
            ⚠️ {{ row.upstream_id }}
            <span class="conflict-arrow">→</span>
            <span class="conflict-target">{{ row.policy_upstream }}</span>
          </span>
          <span v-else class="tag" style="background:var(--color-primary-dim);color:var(--color-primary);font-weight:500">{{ row.upstream_id }}</span>
        </template>
        <template #cell-created_at="{ row }"><span style="font-size:.75rem;color:var(--text-tertiary)">{{ formatTime(row.created_at) }}</span></template>
        <template #expand="{ row }">
          <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:12px 24px;font-size:.82rem">
            <div><span class="expand-label">用户ID</span><span class="expand-value">{{ row.sender_id }}</span></div>
            <div><span class="expand-label">姓名</span><span class="expand-value">{{ getUserInfo(row,'name') }}</span></div>
            <div><span class="expand-label">邮箱</span><span class="expand-value">{{ getUserInfo(row,'email') }}</span></div>
            <div><span class="expand-label">手机</span><span class="expand-value">{{ getUserInfo(row,'mobile') }}</span></div>
            <div><span class="expand-label">部门</span><span class="expand-value">{{ getUserInfo(row,'department') }}</span></div>
            <div><span class="expand-label">App</span><span class="expand-value" style="font-family:var(--font-mono);font-size:.75rem">{{ row.app_id||'--' }}</span></div>
            <div><span class="expand-label">上游</span><span class="expand-value">{{ row.upstream_id }}</span></div>
            <div v-if="row.policy_conflict" style="grid-column:1/-1;background:var(--color-danger-dim);border-radius:8px;padding:10px 14px;border-left:3px solid var(--color-danger)">
              <div style="font-weight:600;color:var(--color-danger);margin-bottom:4px">⚠️ 策略路由冲突</div>
              <div style="font-size:.8rem;color:var(--text-secondary)">
                当前绑定: <strong>{{ row.upstream_id }}</strong> · 策略指定: <strong style="color:var(--color-danger)">{{ row.policy_upstream }}</strong>
              </div>
              <div v-if="row.policy_rule" style="font-size:.75rem;color:var(--text-tertiary);margin-top:2px">匹配规则: {{ row.policy_rule }}</div>
              <div style="font-size:.75rem;color:var(--text-tertiary);margin-top:4px">下次消息请求时将自动迁移到策略指定上游</div>
            </div>
            <div><span class="expand-label">绑定时间</span><span class="expand-value">{{ row.created_at || '--' }}</span></div>
            <div><span class="expand-label">更新时间</span><span class="expand-value">{{ row.updated_at || '--' }}</span></div>
          </div>
        </template>
        <template #actions="{ row }">
          <button class="btn btn-ghost btn-sm" @click.stop="refreshUserInfo(row)" :title="'从蓝信获取 ' + (row.display_name || row.sender_id) + ' 的最新信息'" :disabled="refreshingUsers.has(row.sender_id)">
            <svg v-if="!refreshingUsers.has(row.sender_id)" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
            <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="spin"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
          </button>
          <button class="btn btn-ghost btn-sm" @click.stop="openMigrateFor(row)" title="迁移"><Icon name="refresh" :size="14" /></button>
          <button class="btn btn-danger btn-sm" @click.stop="confirmUnbind(row)" style="margin-left:4px">解绑</button>
        </template>
      </DataTable>
    </div>

    <!-- ==================== 策略路由 Tab ==================== -->
    <div v-show="activeTab === 'policies'">
      <div class="card" style="margin-bottom:20px">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/><polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/><line x1="4" y1="4" x2="9" y2="9"/></svg></span>
          <span class="card-title">策略路由管理</span>
          <div class="card-actions">
            <button class="btn btn-sm" @click="openPolicyCreate"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg> 新建策略</button>
            <button class="btn btn-ghost btn-sm" @click="loadPolicies"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg> 刷新</button>
          </div>
        </div>
        <div>
          <div v-if="policiesLoading" style="padding:24px;text-align:center;color:var(--text-secondary)">加载中...</div>
          <div v-else-if="policies.length === 0" class="policy-empty">
            <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="var(--text-disabled)" stroke-width="1.5"><polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/></svg>
            <div style="margin-top:12px;color:var(--text-secondary)">暂无路由策略配置</div>
            <div style="font-size:var(--text-xs);color:var(--text-tertiary);margin-top:4px">点击「新建策略」按钮添加</div>
          </div>
          <table v-else class="policy-table">
            <thead><tr><th style="width:40px">#</th><th style="width:60px">优先级</th><th>匹配条件</th><th>目标上游</th><th>类型</th><th style="width:140px;text-align:right">操作</th></tr></thead>
            <tbody>
              <tr v-for="(p, idx) in policies" :key="idx" :class="{ 'policy-matched': policyTestResult && policyTestResult.matched && policyTestResult.policy_index === idx }">
                <td style="color:var(--text-tertiary);font-size:.75rem;font-weight:600">{{ idx + 1 }}</td>
                <td>
                  <div class="priority-controls">
                    <button class="prio-btn" :disabled="idx === 0" @click="movePolicyUp(idx)" title="上移"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="18 15 12 9 6 15"/></svg></button>
                    <button class="prio-btn" :disabled="idx === policies.length - 1" @click="movePolicyDown(idx)" title="下移"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg></button>
                  </div>
                </td>
                <td><div class="policy-conditions">
                  <span v-if="gmf(p,'department')" class="tag tag-info">部门: {{ gmf(p,'department') }}</span>
                  <span v-if="gmf(p,'email')" class="tag tag-info">邮箱: {{ gmf(p,'email') }}</span>
                  <span v-if="gmf(p,'email_suffix')" class="tag tag-info">后缀: {{ gmf(p,'email_suffix') }}</span>
                  <span v-if="gmf(p,'app_id')" class="tag tag-info">App: {{ gmf(p,'app_id') }}</span>
                  <span v-if="gmf(p,'default')" class="tag tag-pass">默认策略</span>
                </div></td>
                <td>
                  <span v-if="p.fixed_response && p.fixed_response.enabled" class="tag" style="background:var(--color-success-dim,#dcfce7);color:var(--color-success,#16a34a);font-weight:600">🎯 固定返回 ({{ p.fixed_response.status_code || 200 }})</span>
                  <span v-else class="tag" style="background:var(--color-primary-dim);color:var(--color-primary);font-weight:600">{{ p.upstream_id || '(默认分配)' }}</span>
                </td>
                <td><span v-if="gmf(p,'default')" class="tag" style="background:var(--color-warning-dim);color:var(--color-warning)">默认</span><span v-else class="tag tag-info">条件</span></td>
                <td style="text-align:right">
                  <button class="btn btn-ghost btn-sm" @click="openPolicyEdit(idx, p)" title="编辑"><Icon name="edit" :size="14" /></button>
                  <button class="btn btn-ghost btn-sm" @click="confirmDeletePolicy(idx, p)" style="margin-left:4px" title="删除"><Icon name="trash" :size="14" /></button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="policy-test-section">
          <div class="policy-test-header"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg><span>策略匹配测试</span></div>
          <div class="policy-test-form">
            <input v-model="policyTestForm.email" placeholder="邮箱" style="flex:1;min-width:120px" />
            <input v-model="policyTestForm.department" placeholder="部门" style="flex:1;min-width:100px" />
            <input v-model="policyTestForm.app_id" placeholder="App ID" style="flex:1;min-width:100px" />
            <button class="btn btn-sm" @click="testPolicy" :disabled="policyTesting">{{ policyTesting ? '测试中...' : '测试' }}</button>
          </div>
          <div v-if="policyTestResult" class="policy-test-result" :class="{ matched: policyTestResult.matched }">
            <div v-if="policyTestResult.matched">
              <span style="color:var(--color-success);font-weight:600">✅ 命中策略 #{{ policyTestResult.policy_index + 1 }}</span>
              <span style="margin-left:12px">→ <span class="tag" style="background:var(--color-primary-dim);color:var(--color-primary);font-weight:700">{{ policyTestResult.upstream_id }}</span></span>
            </div>
            <div v-else><span style="color:var(--color-danger);font-weight:600">❌ 未命中</span><span v-if="policyTestResult.message" style="margin-left:8px;font-size:.82rem;color:var(--text-secondary)">{{ policyTestResult.message }}</span></div>
          </div>
          <div v-if="policyTestResult && policyTestResult.matched && policies.length > 0" class="policy-test-visual">
            <div class="policy-test-flow">
              <template v-for="(p, idx) in policies" :key="idx">
                <div class="policy-flow-item" :class="{ 'flow-matched': policyTestResult.policy_index === idx, 'flow-skipped': idx < policyTestResult.policy_index, 'flow-after': idx > policyTestResult.policy_index }">
                  <div class="flow-badge">#{{ idx + 1 }}</div>
                  <div class="flow-label">{{ getPolicyLabel(p) }}</div>
                  <div v-if="policyTestResult.policy_index === idx" class="flow-hit">✓ 命中</div>
                  <div v-else-if="idx < policyTestResult.policy_index" class="flow-skip">跳过</div>
                </div>
                <div v-if="idx < policies.length - 1" class="flow-arrow">→</div>
              </template>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== 可视化 Tab ==================== -->
    <div v-show="activeTab === 'visual'">
      <div class="card" style="margin-bottom:20px">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21.21 15.89A10 10 0 1 1 8 2.83"/><path d="M22 12A10 10 0 0 0 12 2v10z"/></svg></span>
          <span class="card-title">上游负载分布</span>
        </div>
        <div style="padding:20px"><PieChart :data="upstreamPieData" :size="200" /></div>
      </div>
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="18" cy="18" r="3"/><circle cx="6" cy="6" r="3"/><path d="M13 6h3a2 2 0 0 1 2 2v7"/><path d="M11 18H8a2 2 0 0 1-2-2V9"/></svg></span>
          <span class="card-title">路由绑定关系</span>
          <div class="card-actions"><span style="font-size:var(--text-xs);color:var(--text-tertiary)">显示前 {{ Math.min(allRoutes.length, 50) }} 条</span></div>
        </div>
        <div class="bindmap-container" v-if="allRoutes.length > 0">
          <svg :viewBox="'0 0 ' + bindMapWidth + ' ' + bindMapHeight" :width="bindMapWidth" :height="bindMapHeight" class="bindmap-svg">
            <path v-for="(line, i) in bindMapLines" :key="'l'+i" :d="line.path" fill="none" :stroke="line.color" stroke-width="1.5" opacity="0.4" />
            <g v-for="(node, i) in bindMapSenders" :key="'s'+i">
              <rect :x="10" :y="node.y - 10" width="160" height="20" rx="4" fill="var(--bg-elevated)" stroke="var(--border-default)" stroke-width="1" />
              <text :x="90" :y="node.y + 4" text-anchor="middle" fill="var(--text-secondary)" font-size="10" font-family="monospace">{{ truncateStr(node.id, 20) }}</text>
            </g>
            <g v-for="(node, i) in bindMapUpstreams" :key="'u'+i">
              <rect :x="bindMapWidth - 170" :y="node.y - 12" width="160" height="24" rx="6" :fill="node.color + '22'" :stroke="node.color" stroke-width="1.5" />
              <text :x="bindMapWidth - 90" :y="node.y + 4" text-anchor="middle" :fill="node.color" font-size="11" font-weight="600">{{ truncateStr(node.id, 18) }}</text>
            </g>
          </svg>
        </div>
        <div v-else style="padding:32px;text-align:center;color:var(--text-tertiary)">暂无路由数据</div>
      </div>
    </div>

    <!-- Modals -->
    <BindModal :visible="showBindModal" title="绑定用户" icon="🔗" description="将用户绑定到指定上游服务" :fields="bindFields" v-model="bindForm" confirm-text="确认绑定" @confirm="doBind" @cancel="showBindModal=false"><template #field-upstream="{value,update}"><UpstreamSelect :modelValue="value" @update:modelValue="update"/></template></BindModal>
    <BindModal :visible="showBatchModal" title="批量绑定" icon="📥" description="批量绑定多个用户到同一上游" :fields="batchFields" v-model="batchForm" confirm-text="确认绑定" @confirm="doBatchBind" @cancel="showBatchModal=false"><template #field-upstream="{value,update}"><UpstreamSelect :modelValue="value" @update:modelValue="update"/></template><template #preview><div v-if="batchPreview" class="batch-preview">解析预览: <strong style="color:var(--color-info)">{{ batchPreview }}</strong> 条有效记录</div></template></BindModal>
    <BindModal :visible="showMigrateModal" title="迁移用户" icon="🔄" warning="迁移会将用户从当前上游移到新上游" type="warning" :fields="migrateFields" v-model="migrateForm" confirm-text="确认迁移" @confirm="doMigrate" @cancel="showMigrateModal=false"><template #field-upstream="{value,update}"><UpstreamSelect :modelValue="value" @update:modelValue="update"/></template></BindModal>
    <BindModal :visible="showBatchMigrateModal" title="批量迁移" icon="🔄" :warning="'将 ' + selectedRoutes.size + ' 个路由迁移到新上游'" type="warning" :fields="batchMigrateFields" v-model="batchMigrateForm" confirm-text="确认批量迁移" @confirm="doBatchMigrate" @cancel="showBatchMigrateModal=false"><template #field-upstream="{value,update}"><UpstreamSelect :modelValue="value" @update:modelValue="update"/></template></BindModal>
    <BindModal :visible="showPolicyModal" :title="policyEditIdx>=0?'编辑策略':'新建策略'" :icon="policyEditIdx>=0?'✏️':'➕'" :description="policyEditIdx>=0?'修改路由策略的匹配条件和目标上游':'添加新的路由策略规则'" :fields="policyFields" v-model="policyForm" :confirm-text="policyEditIdx>=0?'保存修改':'创建策略'" @confirm="doPolicySave" @cancel="showPolicyModal=false"><template #field-upstream="{value,update}"><UpstreamSelect :modelValue="value" @update:modelValue="update"/></template></BindModal>
    <ConfirmModal :visible="confirmVisible" :title="confirmTitle" :message="confirmMsg" type="danger" :confirm-text="confirmBtnText" @confirm="doConfirmAction" @cancel="confirmVisible=false"/>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import { showToast } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import Icon from '../components/Icon.vue'
import BindModal from '../components/BindModal.vue'
import UpstreamSelect from '../components/UpstreamSelect.vue'
import PieChart from '../components/PieChart.vue'

// ==================== Tab State ====================
const activeTab = ref('routes')

// ==================== Policies ====================
const policies = ref([])
const policiesLoading = ref(false)
const policyTesting = ref(false)
const policyTestForm = reactive({ email: '', department: '', app_id: '' })
const policyTestResult = ref(null)
const showPolicyModal = ref(false)
const policyEditIdx = ref(-1)
const policyForm = ref({ matchType: 'department', matchValue: '', upstream: '' })

function gmf(p, field) {
  if (!p) return ''
  const match = p.match || p.Match || {}
  if (field === 'default') return match.default || match.Default || false
  return match[field] || ''
}

function getPolicyLabel(p) {
  const m = p.match || p.Match || {}
  if (m.department) return '部门:' + m.department
  if (m.email) return m.email
  if (m.email_suffix) return '后缀:' + m.email_suffix
  if (m.app_id) return 'App:' + m.app_id
  if (m.default) return '默认'
  return '?'
}

const MATCH_TYPE_OPTIONS = [
  { value: 'department', label: '部门' },
  { value: 'email', label: '邮箱' },
  { value: 'email_suffix', label: '邮箱后缀' },
  { value: 'app_id', label: 'App ID' },
  { value: 'default', label: '默认策略' },
]

const policyFields = computed(() => {
  const fields = [{ key: 'matchType', label: '匹配类型', type: 'select', required: true, options: MATCH_TYPE_OPTIONS }]
  if (policyForm.value.matchType !== 'default') {
    const ph = { department: '输入部门名称', email: '输入完整邮箱', email_suffix: '输入邮箱后缀，如 @qianxin.com', app_id: '输入 App ID' }
    fields.push({ key: 'matchValue', label: '匹配值', type: 'text', required: true, placeholder: ph[policyForm.value.matchType] || '' })
  }
  fields.push({ key: 'upstream', label: '目标上游', type: 'component', required: false, hint: '留空表示使用默认上游分配（启用固定返回时可为空）' })
  // v34.0: 固定返回内容配置
  fields.push({ key: 'fixedEnabled', label: '启用固定返回', type: 'checkbox', hint: '启用后直接返回固定内容，不转发上游' })
  if (policyForm.value.fixedEnabled) {
    fields.push({ key: 'fixedStatusCode', label: '状态码', type: 'number', placeholder: '200', hint: '默认 200' })
    fields.push({ key: 'fixedContentType', label: 'Content-Type', type: 'text', placeholder: 'application/json', hint: '默认 application/json' })
    fields.push({ key: 'fixedBody', label: '返回内容', type: 'textarea', placeholder: '{"code":0,"message":"ok"}', rows: 4 })
    fields.push({ key: 'fixedHeaders', label: '自定义响应头', type: 'textarea', placeholder: 'X-Custom: value\nX-Another: value2', rows: 2, hint: '每行一个，格式: Key: Value' })
  }
  return fields
})

function openPolicyCreate() { policyEditIdx.value = -1; policyForm.value = { matchType: 'department', matchValue: '', upstream: '', fixedEnabled: false, fixedStatusCode: 200, fixedContentType: 'application/json', fixedBody: '', fixedHeaders: '' }; showPolicyModal.value = true }
function openPolicyEdit(idx, p) {
  policyEditIdx.value = idx
  const match = p.match || p.Match || {}
  let mt = 'default', mv = ''
  if (match.department) { mt = 'department'; mv = match.department }
  else if (match.email) { mt = 'email'; mv = match.email }
  else if (match.email_suffix) { mt = 'email_suffix'; mv = match.email_suffix }
  else if (match.app_id) { mt = 'app_id'; mv = match.app_id }
  const fr = p.fixed_response || {}
  const hdrs = fr.headers ? Object.entries(fr.headers).map(([k,v]) => `${k}: ${v}`).join('\n') : ''
  policyForm.value = {
    matchType: mt, matchValue: mv, upstream: p.upstream_id || '',
    fixedEnabled: !!fr.enabled,
    fixedStatusCode: fr.status_code || 200,
    fixedContentType: fr.content_type || 'application/json',
    fixedBody: fr.body || '',
    fixedHeaders: hdrs,
  }
  showPolicyModal.value = true
}

async function doPolicySave() {
  const { matchType, matchValue, upstream } = policyForm.value
  const match = {}
  if (matchType === 'default') { match.default = true }
  else {
    if (!matchValue || !matchValue.trim()) { showToast('请填写匹配值', 'error'); return }
    match[matchType] = matchValue.trim()
  }
  const body = { match, upstream_id: upstream || '' }
  // v34.0: 固定返回内容
  if (policyForm.value.fixedEnabled) {
    const headers = {}
    if (policyForm.value.fixedHeaders) {
      policyForm.value.fixedHeaders.split('\n').forEach(line => {
        const idx = line.indexOf(':')
        if (idx > 0) headers[line.substring(0, idx).trim()] = line.substring(idx + 1).trim()
      })
    }
    body.fixed_response = {
      enabled: true,
      status_code: parseInt(policyForm.value.fixedStatusCode) || 200,
      content_type: policyForm.value.fixedContentType || 'application/json',
      body: policyForm.value.fixedBody || '',
      headers: Object.keys(headers).length > 0 ? headers : undefined,
    }
  }
  try {
    let result
    if (policyEditIdx.value >= 0) result = await apiPut('/api/v1/route-policies/' + policyEditIdx.value, body)
    else result = await apiPost('/api/v1/route-policies', body)
    policies.value = result.policies || []
    showToast(policyEditIdx.value >= 0 ? '策略修改成功' : '策略创建成功', 'success')
    showPolicyModal.value = false
  } catch (e) { showToast('保存失败: ' + e.message, 'error') }
}

// ==================== Policy Priority (Up/Down) ====================
async function swapPolicies(idxA, idxB) {
  const list = [...policies.value]
  const temp = list[idxA]
  list[idxA] = list[idxB]
  list[idxB] = temp
  try {
    const result = await apiPost('/api/v1/route-policies/reorder', { policies: list })
    policies.value = result.policies || []
    showToast('策略顺序已更新', 'success')
  } catch (e) { showToast('调整优先级失败: ' + e.message, 'error'); await loadPolicies() }
}
function movePolicyUp(idx) { if (idx > 0) swapPolicies(idx, idx - 1) }
function movePolicyDown(idx) { if (idx < policies.value.length - 1) swapPolicies(idx, idx + 1) }

// ==================== Confirm Modal ====================
const confirmVisible = ref(false)
const confirmTitle = ref('确认操作')
const confirmMsg = ref('')
const confirmBtnText = ref('确认')
const pendingConfirmAction = ref(null)
function doConfirmAction() { confirmVisible.value = false; if (pendingConfirmAction.value) { pendingConfirmAction.value(); pendingConfirmAction.value = null } }

function confirmDeletePolicy(idx, p) {
  const match = p.match || p.Match || {}
  let desc = '策略 #' + (idx + 1)
  if (match.department) desc += ' (部门: ' + match.department + ')'
  else if (match.email_suffix) desc += ' (后缀: ' + match.email_suffix + ')'
  else if (match.email) desc += ' (邮箱: ' + match.email + ')'
  else if (match.app_id) desc += ' (App: ' + match.app_id + ')'
  else if (match.default) desc += ' (默认策略)'
  confirmTitle.value = '删除策略'; confirmMsg.value = '确认删除 ' + desc + '？此操作不可撤销。'; confirmBtnText.value = '删除'
  pendingConfirmAction.value = async () => { try { const r = await apiDelete('/api/v1/route-policies/' + idx); policies.value = r.policies || []; showToast('策略删除成功', 'success') } catch (e) { showToast('删除失败: ' + e.message, 'error') } }
  confirmVisible.value = true
}

async function loadPolicies() { policiesLoading.value = true; try { const d = await api('/api/v1/route-policies'); policies.value = d.policies || [] } catch { policies.value = [] } policiesLoading.value = false }
async function testPolicy() {
  if (!policyTestForm.email && !policyTestForm.department && !policyTestForm.app_id) { showToast('请至少填写一个匹配条件', 'error'); return }
  policyTesting.value = true
  try { policyTestResult.value = await apiPost('/api/v1/route-policies/test', { email: policyTestForm.email, department: policyTestForm.department, app_id: policyTestForm.app_id }) } catch (e) { showToast('测试失败: ' + e.message, 'error') }
  policyTesting.value = false
}

// ==================== Routes ====================
const loading = ref(false)
const allRoutes = ref([])
const userCache = ref({})
const routeStats = ref(null)
const rawStats = ref(null)
const filterApp = ref('')
const filterDept = ref('')
const filterUpstream = ref('')
const searchText = ref('')
const sortBy = ref('')
const filterConflict = ref(false)

const conflictCount = computed(() => allRoutes.value.filter(r => r.policy_conflict).length)
function toggleConflictFilter() { filterConflict.value = !filterConflict.value }
const showBindModal = ref(false)
const showBatchModal = ref(false)
const showMigrateModal = ref(false)
const showBatchMigrateModal = ref(false)
const bindForm = ref({ sender: '', app: '', upstream: '', name: '', dept: '' })
const batchForm = ref({ app: '', upstream: '', text: '' })
const migrateForm = ref({ sender: '', app: '', upstream: '' })
const batchMigrateForm = ref({ upstream: '' })
const selectedRoutes = reactive(new Set())

const bindFields = [
  { key: 'sender', label: '用户 ID', type: 'text', required: true, placeholder: '输入用户 ID' },
  { key: 'app', label: 'App ID (Bot)', type: 'text', placeholder: '输入 App ID（可选）' },
  { key: 'upstream', label: '目标上游', type: 'component', required: true },
  { key: 'name', label: '显示名', type: 'text', placeholder: '用户姓名（可选）' },
  { key: 'dept', label: '部门', type: 'text', placeholder: '所属部门（可选）' },
]
const batchFields = [
  { key: 'app', label: 'App ID (Bot)', type: 'text', placeholder: '输入 App ID（可选）' },
  { key: 'upstream', label: '目标上游', type: 'component', required: true },
  { key: 'text', label: '用户列表', type: 'textarea', required: true, placeholder: '每行: 用户ID,显示名,部门', rows: 6, hint: '格式: 用户ID,显示名,部门' },
]
const migrateFields = [
  { key: 'sender', label: '用户 ID', type: 'text', required: true, placeholder: '输入要迁移的用户 ID' },
  { key: 'app', label: 'App ID', type: 'text', placeholder: '输入 App ID（可选）' },
  { key: 'upstream', label: '目标上游', type: 'component', required: true },
]
const batchMigrateFields = [
  { key: 'upstream', label: '目标上游', type: 'component', required: true },
]

const batchPreview = computed(() => { const t = batchForm.value.text; if (!t || !t.trim()) return null; return t.trim().split('\n').filter(l => l.split(',')[0]?.trim()).length })

const routeColumns = [
  { key: '_select', label: '', sortable: false, width: '36px' },
  { key: 'sender_id', label: '用户 ID', sortable: true },
  { key: 'display_name', label: '姓名', sortable: true },
  { key: 'department', label: '部门', sortable: true },
  { key: 'app_id', label: 'Bot', sortable: true },
  { key: 'upstream_id', label: '上游', sortable: true },
  { key: 'created_at', label: '绑定时间', sortable: true },
]

const apps = computed(() => { const s = new Set(); allRoutes.value.forEach(r => { if (r.app_id) s.add(r.app_id) }); return [...s].sort() })
const depts = computed(() => { const s = new Set(); allRoutes.value.forEach(r => { if (r.department) s.add(r.department) }); return [...s].sort() })
const upstreamList = computed(() => { const s = new Set(); allRoutes.value.forEach(r => { if (r.upstream_id) s.add(r.upstream_id) }); return [...s].sort() })

const filteredRoutes = computed(() => {
  let list = allRoutes.value
  if (filterApp.value) list = list.filter(r => r.app_id === filterApp.value)
  if (filterDept.value) list = list.filter(r => r.department === filterDept.value)
  if (filterUpstream.value) list = list.filter(r => r.upstream_id === filterUpstream.value)
  if (filterConflict.value) list = list.filter(r => r.policy_conflict)
  if (searchText.value) {
    const q = searchText.value.toLowerCase()
    list = list.filter(r =>
      (r.sender_id || '').toLowerCase().includes(q) ||
      (r.upstream_id || '').toLowerCase().includes(q) ||
      (r.display_name || '').toLowerCase().includes(q)
    )
  }
  list = list.map(r => { const u = userCache.value[r.sender_id] || {}; return { ...r, display_name: u.name || r.display_name || '--', department: u.department || r.department || '--' } })
  if (sortBy.value === 'created_desc') list = [...list].sort((a, b) => (b.created_at || '').localeCompare(a.created_at || ''))
  else if (sortBy.value === 'created_asc') list = [...list].sort((a, b) => (a.created_at || '').localeCompare(b.created_at || ''))
  return list
})

function routeRowKey(row) { return (row.sender_id || '') + '|' + (row.app_id || '') }
function toggleSelect(row) { const k = routeRowKey(row); if (selectedRoutes.has(k)) selectedRoutes.delete(k); else selectedRoutes.add(k) }
function selectAllVisible() { filteredRoutes.value.forEach(r => selectedRoutes.add(routeRowKey(r))) }
function getSelectedEntries() {
  return [...selectedRoutes].map(k => { const [sid, aid] = k.split('|'); return { sender_id: sid, app_id: aid || '' } })
}

function getUserInfo(row, field) {
  const u = userCache.value[row.sender_id] || {}
  if (field === 'name') return u.name || row.display_name || '--'
  if (field === 'email') return u.email || '--'
  if (field === 'mobile') return u.mobile || '--'
  if (field === 'department') return u.department || row.department || '--'
  return '--'
}

function formatTime(t) {
  if (!t) return '--'
  try { return new Date(t).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) } catch { return t }
}

function truncateStr(s, n) { return s && s.length > n ? s.substring(0, n) + '...' : (s || '') }

async function loadRoutes() { loading.value = true; try { allRoutes.value = (await api('/api/v1/routes')).routes || [] } catch { allRoutes.value = [] } loading.value = false }
async function loadRouteStats() {
  try {
    const d = await api('/api/v1/routes/stats')
    rawStats.value = d
    routeStats.value = {
      appCount: d.by_app ? Object.keys(d.by_app).length : (d.total_apps || 0),
      senderCount: d.total_users || d.unique_senders || 0,
      upstreamCount: d.by_upstream ? Object.keys(d.by_upstream).length : 0,
      total: d.total_routes || d.total || 0
    }
  } catch { rawStats.value = null }
}
async function loadUsers() { try { const d = await api('/api/v1/users'); const m = {}; (d.users || []).forEach(u => { m[u.sender_id] = u }); userCache.value = m } catch {} }
function refresh() { loadRoutes(); loadRouteStats(); loadUsers(); loadPolicies() }

// ==================== 用户信息刷新（从蓝信获取） ====================
const refreshingUsers = ref(new Set())
const refreshingAll = ref(false)

async function refreshUserInfo(row) {
  const sid = row.sender_id
  refreshingUsers.value.add(sid)
  refreshingUsers.value = new Set(refreshingUsers.value) // trigger reactivity
  try {
    const d = await apiPost('/api/v1/users/' + encodeURIComponent(sid) + '/refresh', {})
    const name = d.name || d.display_name || ''
    const dept = d.department || ''
    showToast('已更新: ' + (name || sid) + (dept ? ' (' + dept + ')' : ''), 'success')
    refresh() // 刷新列表以显示最新信息和冲突状态
  } catch (e) {
    showToast('获取失败: ' + e.message, 'error')
  } finally {
    refreshingUsers.value.delete(sid)
    refreshingUsers.value = new Set(refreshingUsers.value)
  }
}

async function refreshAllUserInfo() {
  refreshingAll.value = true
  try {
    const d = await apiPost('/api/v1/users/refresh-all', {})
    const ok = d.success || 0
    const fail = d.failed || 0
    showToast('批量刷新完成: ' + ok + ' 成功' + (fail > 0 ? ', ' + fail + ' 失败' : ''), ok > 0 ? 'success' : 'warning')
    refresh()
  } catch (e) {
    showToast('批量刷新失败: ' + e.message, 'error')
  } finally {
    refreshingAll.value = false
  }
}

// ==================== Route Actions ====================
function confirmUnbind(row) {
  confirmTitle.value = '确认解绑'; confirmMsg.value = '确认解绑用户 ' + row.sender_id + ' (' + (row.display_name || '--') + ') ?'; confirmBtnText.value = '解绑'
  pendingConfirmAction.value = async () => { try { await apiPost('/api/v1/routes/unbind', { sender_id: row.sender_id, app_id: row.app_id }); showToast('解绑成功', 'success'); refresh() } catch (e) { showToast('解绑失败: ' + e.message, 'error') } }
  confirmVisible.value = true
}
async function doBind(data) {
  if (!data.sender || !data.sender.trim()) { showToast('用户 ID 不能为空', 'error'); return }
  if (!data.upstream) { showToast('请选择目标上游', 'error'); return }
  const body = { sender_id: data.sender, upstream_id: data.upstream }
  if (data.app) body.app_id = data.app; if (data.name) body.display_name = data.name; if (data.dept) body.department = data.dept
  try { await apiPost('/api/v1/routes/bind', body); showToast('绑定成功', 'success'); showBindModal.value = false; bindForm.value = { sender: '', app: '', upstream: '', name: '', dept: '' }; refresh() } catch (e) { showToast('绑定失败: ' + e.message, 'error') }
}
async function doBatchBind(data) {
  if (!data.upstream) { showToast('请选择上游', 'error'); return }
  const lines = data.text.trim().split('\n').filter(l => l.trim())
  if (!lines.length) { showToast('请输入用户列表', 'error'); return }
  const entries = lines.map(l => { const p = l.split(','); return { sender_id: p[0]?.trim(), display_name: p[1]?.trim(), department: p[2]?.trim() } }).filter(e => e.sender_id)
  try { const d = await apiPost('/api/v1/routes/batch-bind', { app_id: data.app, upstream_id: data.upstream, entries }); showToast('批量绑定 ' + (d.count || entries.length) + ' 条成功', 'success'); showBatchModal.value = false; batchForm.value = { app: '', upstream: '', text: '' }; refresh() } catch (e) { showToast('批量绑定失败: ' + e.message, 'error') }
}
async function doMigrate(data) {
  if (!data.sender || !data.upstream) { showToast('请填写用户ID和目标上游', 'error'); return }
  // Validate: target upstream != current upstream
  const cur = allRoutes.value.find(r => r.sender_id === data.sender && (!data.app || r.app_id === data.app))
  if (cur && cur.upstream_id === data.upstream) { showToast('目标上游不能与当前上游相同', 'error'); return }
  const body = { sender_id: data.sender, to: data.upstream }; if (data.app) body.app_id = data.app
  try { await apiPost('/api/v1/routes/migrate', body); showToast('迁移成功', 'success'); showMigrateModal.value = false; migrateForm.value = { sender: '', app: '', upstream: '' }; refresh() } catch (e) { showToast('迁移失败: ' + e.message, 'error') }
}
function openMigrateFor(row) { migrateForm.value = { sender: row.sender_id, app: row.app_id || '', upstream: '' }; showMigrateModal.value = true }

// ==================== Batch Operations ====================
function confirmBatchUnbind() {
  const count = selectedRoutes.size
  confirmTitle.value = '批量解绑'; confirmMsg.value = '确认解绑选中的 ' + count + ' 个路由？此操作不可撤销。'; confirmBtnText.value = '解绑 ' + count + ' 个'
  pendingConfirmAction.value = async () => {
    try {
      await apiPost('/api/v1/routes/batch-unbind', { entries: getSelectedEntries() })
      showToast('批量解绑 ' + count + ' 条成功', 'success')
      selectedRoutes.clear(); refresh()
    } catch (e) { showToast('批量解绑失败: ' + e.message, 'error') }
  }
  confirmVisible.value = true
}
function openBatchMigrateSelected() { batchMigrateForm.value = { upstream: '' }; showBatchMigrateModal.value = true }
async function doBatchMigrate(data) {
  if (!data.upstream) { showToast('请选择目标上游', 'error'); return }
  try {
    const res = await apiPost('/api/v1/routes/batch-migrate', { entries: getSelectedEntries(), to: data.upstream })
    showToast('批量迁移 ' + (res.count || selectedRoutes.size) + ' 条成功', 'success')
    showBatchMigrateModal.value = false; selectedRoutes.clear(); refresh()
  } catch (e) { showToast('批量迁移失败: ' + e.message, 'error') }
}

// ==================== Visualization ====================
const PIE_COLORS = ['#6366F1', '#22C55E', '#F59E0B', '#EF4444', '#06B6D4', '#A855F7', '#EC4899', '#14B8A6', '#F97316', '#8B5CF6']
const upstreamPieData = computed(() => {
  const byUp = rawStats.value?.by_upstream || {}
  return Object.entries(byUp).map(([k, v], i) => ({ label: k, value: v, color: PIE_COLORS[i % PIE_COLORS.length] }))
})

// Bind Map
const bindMapWidth = 600
const bindMapRoutes = computed(() => allRoutes.value.slice(0, 50))
const bindMapSenders = computed(() => {
  const senders = [...new Set(bindMapRoutes.value.map(r => r.sender_id))]
  const gap = Math.max(24, 400 / Math.max(senders.length, 1))
  return senders.map((id, i) => ({ id, y: 30 + i * gap }))
})
const bindMapUpstreams = computed(() => {
  const ups = [...new Set(bindMapRoutes.value.map(r => r.upstream_id))]
  const gap = Math.max(30, 400 / Math.max(ups.length, 1))
  return ups.map((id, i) => ({ id, y: 30 + i * gap, color: PIE_COLORS[i % PIE_COLORS.length] }))
})
const bindMapHeight = computed(() => Math.max(
  (bindMapSenders.value.length) * Math.max(24, 400 / Math.max(bindMapSenders.value.length, 1)) + 40,
  (bindMapUpstreams.value.length) * Math.max(30, 400 / Math.max(bindMapUpstreams.value.length, 1)) + 40,
  100
))
const bindMapLines = computed(() => {
  const sMap = {}; bindMapSenders.value.forEach(s => { sMap[s.id] = s.y })
  const uMap = {}; bindMapUpstreams.value.forEach(u => { uMap[u.id] = { y: u.y, color: u.color } })
  return bindMapRoutes.value.map(r => {
    const sy = sMap[r.sender_id] || 0
    const u = uMap[r.upstream_id] || { y: 0, color: '#666' }
    const sx = 170, ex = bindMapWidth - 170
    return { path: 'M' + sx + ',' + sy + ' C' + ((sx+ex)/2) + ',' + sy + ' ' + ((sx+ex)/2) + ',' + u.y + ' ' + ex + ',' + u.y, color: u.color }
  })
})

onMounted(refresh)
</script>

<style scoped>
/* Tabs */
.route-tabs { display: flex; gap: 4px; margin-bottom: 16px; background: var(--bg-elevated); padding: 4px; border-radius: var(--radius-lg); }
.route-tab { display: flex; align-items: center; gap: 6px; padding: 8px 16px; border: none; background: none; color: var(--text-secondary); font-size: var(--text-sm); font-weight: 500; border-radius: var(--radius-md); cursor: pointer; transition: all var(--transition-fast); font-family: var(--font-sans); }
.route-tab:hover { color: var(--text-primary); background: var(--bg-hover); }
.route-tab.active { background: var(--color-primary); color: #fff; box-shadow: 0 2px 8px rgba(99,102,241,.3); }
.route-tab-badge { font-size: .7rem; background: rgba(255,255,255,.2); padding: 1px 6px; border-radius: 10px; font-weight: 600; }
.route-tab.active .route-tab-badge { background: rgba(255,255,255,.25); }
.route-tab:not(.active) .route-tab-badge { background: var(--bg-hover); color: var(--text-tertiary); }

/* Policy Table */
.policy-table { width: 100%; border-collapse: collapse; }
.policy-table th { text-align: left; padding: 10px 14px; color: var(--text-secondary); border-bottom: 1px solid var(--border-default); font-weight: 500; font-size: .78rem; text-transform: uppercase; }
.policy-table td { padding: 10px 14px; border-bottom: 1px solid var(--border-subtle); color: var(--text-primary); font-size: .85rem; }
.policy-table tr:hover td { background: var(--bg-hover); }
.policy-table tr.policy-matched td { background: rgba(34,197,94,.08) !important; border-color: rgba(34,197,94,.15); }
.policy-conditions { display: flex; gap: 6px; flex-wrap: wrap; }
.policy-empty { text-align: center; padding: 32px; }

/* Priority controls */
.priority-controls { display: flex; flex-direction: column; gap: 2px; align-items: center; }
.prio-btn { display: flex; align-items: center; justify-content: center; width: 22px; height: 18px; border: 1px solid var(--border-default); background: var(--bg-elevated); border-radius: 3px; cursor: pointer; color: var(--text-secondary); transition: all var(--transition-fast); padding: 0; }
.prio-btn:hover:not(:disabled) { border-color: var(--color-primary); color: var(--color-primary); background: var(--color-primary-dim); }
.prio-btn:disabled { opacity: .25; cursor: not-allowed; }

/* Policy test */
.policy-test-section { padding: 16px; border-top: 1px solid var(--border-subtle); margin-top: 4px; }
.policy-test-header { display: flex; align-items: center; gap: 8px; font-size: var(--text-sm); color: var(--color-primary); font-weight: 600; margin-bottom: 10px; }
.policy-test-form { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
.policy-test-form input { background: var(--bg-elevated); color: var(--text-primary); border: 1px solid var(--border-default); border-radius: var(--radius-md); padding: 6px 10px; font-size: .82rem; outline: none; transition: border-color var(--transition-fast); }
.policy-test-form input:focus { border-color: var(--color-primary); box-shadow: 0 0 0 3px var(--color-primary-dim); }
.policy-test-result { margin-top: 10px; padding: 10px 14px; background: var(--bg-elevated); border-radius: var(--radius-md); border-left: 3px solid var(--color-danger); font-size: .85rem; }
.policy-test-result.matched { border-left-color: var(--color-success); }

/* Policy test flow visualization */
.policy-test-visual { margin-top: 12px; }
.policy-test-flow { display: flex; align-items: center; gap: 0; flex-wrap: wrap; padding: 12px; background: var(--bg-elevated); border-radius: var(--radius-md); }
.policy-flow-item { display: flex; flex-direction: column; align-items: center; gap: 4px; padding: 8px 12px; border-radius: var(--radius-md); border: 1px solid var(--border-default); min-width: 80px; transition: all .2s; }
.policy-flow-item.flow-matched { background: rgba(34,197,94,.1); border-color: rgba(34,197,94,.4); }
.policy-flow-item.flow-skipped { opacity: .5; }
.policy-flow-item.flow-after { opacity: .3; }
.flow-badge { font-size: .7rem; font-weight: 700; color: var(--text-tertiary); }
.flow-label { font-size: .72rem; color: var(--text-secondary); max-width: 100px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; text-align: center; }
.flow-hit { font-size: .68rem; color: var(--color-success); font-weight: 700; }
.flow-skip { font-size: .68rem; color: var(--text-disabled); }
.flow-arrow { color: var(--text-disabled); font-size: .9rem; padding: 0 4px; }

/* Route stats & filters */
.route-stats-bar { display: flex; align-items: center; gap: 0; margin-bottom: 16px; padding: 12px 16px; background: var(--bg-elevated); border-radius: var(--radius-md); }
.route-stat-item { display: flex; align-items: center; gap: 8px; flex: 1; justify-content: center; }
.route-stat-label { font-size: var(--text-xs); color: var(--text-tertiary); }
.route-stat-value { font-size: var(--text-lg); font-weight: 700; font-family: var(--font-mono); }
.route-stat-divider { width: 1px; height: 24px; background: var(--border-default); }

.search-input-wrap { position: relative; flex: 2; min-width: 150px; }
.search-input-wrap svg { position: absolute; left: 10px; top: 50%; transform: translateY(-50%); color: var(--text-tertiary); pointer-events: none; }
.search-input-wrap input { width: 100%; padding-left: 32px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); color: var(--text-primary); padding-top: 8px; padding-bottom: 8px; padding-right: 12px; font-size: var(--text-sm); outline: none; }
.search-input-wrap input:focus { border-color: var(--color-primary); }

/* Batch bar */
.batch-bar { display: flex; align-items: center; gap: 8px; padding: 10px 16px; margin-bottom: 8px; background: var(--color-primary-dim); border: 1px solid rgba(99,102,241,.2); border-radius: var(--radius-md); animation: batchIn .2s ease-out; }
@keyframes batchIn { from { opacity: 0; transform: translateY(-4px); } to { opacity: 1; transform: translateY(0); } }
.batch-count { font-size: var(--text-sm); color: var(--color-primary); }

/* Checkbox */
.route-checkbox { accent-color: var(--color-primary); cursor: pointer; width: 16px; height: 16px; }

/* Expand */
.expand-label { color: var(--text-tertiary); font-size: var(--text-xs); display: block; margin-bottom: 2px; }
.expand-value { color: var(--text-primary); font-weight: 500; }
.batch-preview { margin-top: 12px; padding: 10px 14px; background: var(--bg-elevated); border-radius: var(--radius-md); font-size: var(--text-sm); color: var(--text-secondary); display: flex; align-items: center; gap: 8px; }

/* Bind Map */
.bindmap-container { padding: 16px; overflow-x: auto; }
.bindmap-svg { display: block; margin: 0 auto; }

/* 策略冲突高亮 */
.policy-conflict-tag {
  background: var(--color-danger-dim, rgba(239,68,68,0.12)) !important;
  color: var(--color-danger, #ef4444) !important;
  font-weight: 600;
  border: 1px solid var(--color-danger, #ef4444);
  cursor: help;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: .78rem;
  line-height: 1.4;
}
.conflict-arrow {
  color: var(--text-tertiary);
  font-size: .7rem;
  margin: 0 1px;
}
.conflict-target {
  color: var(--color-success, #22c55e);
  font-weight: 700;
}
@keyframes spin { to { transform: rotate(360deg) } }
.spin { animation: spin 1s linear infinite; }
</style>
