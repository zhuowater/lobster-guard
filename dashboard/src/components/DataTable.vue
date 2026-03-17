<template>
  <div class="data-table-wrapper">
    <!-- Toolbar: column visibility -->
    <div class="dt-toolbar" v-if="showToolbar">
      <div class="dt-col-toggle" v-if="columns.length > 3">
        <button class="btn btn-sm" @click="colMenuOpen = !colMenuOpen" title="列显隐">⚙️ 列</button>
        <div class="dt-col-menu" v-show="colMenuOpen">
          <label v-for="col in columns" :key="col.key" class="dt-col-item">
            <input type="checkbox" :checked="visibleCols.has(col.key)" @change="toggleCol(col.key)" />
            <span>{{ col.label }}</span>
          </label>
        </div>
      </div>
      <slot name="toolbar"></slot>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="loading">加载中...</div>

    <!-- Empty -->
    <div v-else-if="!sortedData.length" class="empty">
      <div class="empty-icon">{{ emptyIcon }}</div>
      {{ emptyText }}
      <div class="empty-hint"><slot name="empty-hint"></slot></div>
    </div>

    <!-- Table -->
    <div v-else class="table-wrap">
      <table>
        <thead>
          <tr>
            <th v-if="expandable" style="width:30px"></th>
            <th
              v-for="col in visibleColumns" :key="col.key"
              :style="col.width ? { width: col.width } : {}"
              :class="{ sortable: col.sortable, sorted: sortKey === col.key }"
              @click="col.sortable && toggleSort(col.key)"
            >
              {{ col.label }}
              <span v-if="col.sortable" class="sort-icon">
                {{ sortKey === col.key ? (sortDir === 'asc' ? '▲' : '▼') : '⇅' }}
              </span>
            </th>
            <th v-if="$slots.actions" style="white-space:nowrap">操作</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="(row, idx) in pagedData" :key="rowKeyFn(row, idx)">
            <tr
              :class="rowClassFn(row)"
              @click="expandable && toggleExpand(idx)"
              :style="expandable ? { cursor: 'pointer' } : {}"
            >
              <td v-if="expandable" style="width:30px;text-align:center">
                {{ expandedRows.has(idx) ? '▼' : '▶' }}
              </td>
              <td v-for="col in visibleColumns" :key="col.key" :style="col.tdStyle || {}">
                <slot :name="'cell-' + col.key" :row="row" :value="getCellValue(row, col)">
                  <span v-html="formatCell(row, col)"></span>
                </slot>
              </td>
              <td v-if="$slots.actions">
                <slot name="actions" :row="row" :index="idx"></slot>
              </td>
            </tr>
            <!-- Expand row -->
            <tr v-if="expandable && expandedRows.has(idx)" class="expand-row">
              <td :colspan="visibleColumns.length + (expandable ? 1 : 0) + ($slots.actions ? 1 : 0)">
                <slot name="expand" :row="row" :index="idx"></slot>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <div v-if="sortedData.length > 0" class="dt-pagination">
      <div class="dt-page-info">
        共 {{ sortedData.length }} 条，第 {{ currentPage }} / {{ totalPages }} 页
      </div>
      <div class="dt-page-controls">
        <select v-model.number="currentPageSize" @change="currentPage = 1" class="dt-page-select">
          <option v-for="s in pageSizes" :key="s" :value="s">{{ s }} 条/页</option>
        </select>
        <button class="btn btn-sm" :disabled="currentPage <= 1" @click="currentPage--">上一页</button>
        <button class="btn btn-sm" :disabled="currentPage >= totalPages" @click="currentPage++">下一页</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, reactive } from 'vue'

const props = defineProps({
  columns: { type: Array, required: true },
  data: { type: Array, default: () => [] },
  pageSize: { type: Number, default: 20 },
  pageSizes: { type: Array, default: () => [20, 50, 100] },
  loading: { type: Boolean, default: false },
  emptyText: { type: String, default: '暂无数据' },
  emptyIcon: { type: String, default: '📭' },
  expandable: { type: Boolean, default: false },
  rowKey: { type: [String, Function], default: null },
  rowClass: { type: Function, default: null },
  showToolbar: { type: Boolean, default: true },
})

// Column visibility
const visibleCols = reactive(new Set(props.columns.map(c => c.key)))
const colMenuOpen = ref(false)

function toggleCol(key) {
  if (visibleCols.has(key)) {
    if (visibleCols.size > 1) visibleCols.delete(key)
  } else {
    visibleCols.add(key)
  }
}

const visibleColumns = computed(() => props.columns.filter(c => visibleCols.has(c.key)))

// Sorting
const sortKey = ref(null)
const sortDir = ref('asc')

function toggleSort(key) {
  if (sortKey.value === key) {
    if (sortDir.value === 'asc') sortDir.value = 'desc'
    else if (sortDir.value === 'desc') { sortKey.value = null; sortDir.value = 'asc' }
  } else {
    sortKey.value = key
    sortDir.value = 'asc'
  }
}

const sortedData = computed(() => {
  const d = [...props.data]
  if (!sortKey.value) return d
  const key = sortKey.value
  const dir = sortDir.value === 'asc' ? 1 : -1
  return d.sort((a, b) => {
    let va = a[key], vb = b[key]
    if (va == null) va = ''
    if (vb == null) vb = ''
    if (typeof va === 'number' && typeof vb === 'number') return (va - vb) * dir
    return String(va).localeCompare(String(vb)) * dir
  })
})

// Pagination
const currentPage = ref(1)
const currentPageSize = ref(props.pageSize)
const totalPages = computed(() => Math.max(1, Math.ceil(sortedData.value.length / currentPageSize.value)))
const pagedData = computed(() => {
  const start = (currentPage.value - 1) * currentPageSize.value
  return sortedData.value.slice(start, start + currentPageSize.value)
})

// Reset page on data change
watch(() => props.data, () => { currentPage.value = 1 })

// Expand
const expandedRows = reactive(new Set())
function toggleExpand(idx) {
  if (expandedRows.has(idx)) expandedRows.delete(idx)
  else expandedRows.add(idx)
}

// Row key
function rowKeyFn(row, idx) {
  if (!props.rowKey) return idx
  if (typeof props.rowKey === 'function') return props.rowKey(row)
  return row[props.rowKey] ?? idx
}

// Row class
function rowClassFn(row) {
  if (props.rowClass) return props.rowClass(row)
  return ''
}

// Cell value
function getCellValue(row, col) {
  return row[col.key]
}

function formatCell(row, col) {
  const val = row[col.key]
  if (col.format) return col.format(val, row)
  if (val == null) return '--'
  return escHtml(String(val))
}

function escHtml(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}
</script>

<style scoped>
.data-table-wrapper { position: relative; }
.dt-toolbar { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; justify-content: flex-end; position: relative; }
.dt-col-toggle { position: relative; }
.dt-col-menu {
  position: absolute; right: 0; top: 100%; z-index: 50;
  background: var(--bg-card-2); border: 1px solid rgba(0,212,255,.2);
  border-radius: 8px; padding: 8px; min-width: 160px;
  box-shadow: 0 8px 24px rgba(0,0,0,.4);
}
.dt-col-item { display: flex; align-items: center; gap: 6px; padding: 3px 0; font-size: .8rem; color: var(--text); cursor: pointer; }
.dt-col-item input { accent-color: var(--neon-blue); }

th.sortable { cursor: pointer; user-select: none; }
th.sortable:hover { color: var(--neon-green); }
th.sorted { color: var(--neon-green); }
.sort-icon { font-size: .65rem; margin-left: 2px; opacity: .6; }
th.sorted .sort-icon { opacity: 1; }

.expand-row td {
  background: rgba(0,0,0,.15); padding: 12px 16px;
  border-bottom: 1px solid rgba(0,212,255,.08);
}

.dt-pagination {
  display: flex; justify-content: space-between; align-items: center;
  margin-top: 12px; padding-top: 8px;
  border-top: 1px solid rgba(0,212,255,.08);
  font-size: .8rem; color: var(--text-dim);
}
.dt-page-controls { display: flex; gap: 6px; align-items: center; }
.dt-page-select {
  background: rgba(0,0,0,.3); border: 1px solid rgba(0,212,255,.2);
  border-radius: 6px; color: var(--text); padding: 4px 8px; font-size: .78rem; outline: none;
}
.dt-page-select option { background: var(--bg-card); }
.btn:disabled { opacity: .4; cursor: not-allowed; transform: none; }
</style>
