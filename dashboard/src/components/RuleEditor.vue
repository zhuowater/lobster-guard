<template>
  <Teleport to="body">
    <div v-if="visible" class="modal-overlay" @click.self="close">
      <div class="editor-panel">
        <div class="editor-header">
          <span class="modal-icon">{{ isEdit ? '✏️' : '➕' }}</span>
          <span class="modal-title">{{ isEdit ? '编辑规则' : '新建规则' }}</span>
          <button class="editor-close" @click="close">✕</button>
        </div>
        <div class="editor-body">
          <div class="form-group">
            <label>名称 <span class="required">*</span></label>
            <input type="text" v-model="form.name" placeholder="如 my_custom_rule" :disabled="isEdit" />
          </div>
          <div class="form-row">
            <div class="form-group" style="flex:1">
              <label>类型</label>
              <select v-model="form.type">
                <option value="keyword">keyword (关键词)</option>
                <option value="regex">regex (正则)</option>
              </select>
            </div>
            <div class="form-group" style="flex:1">
              <label>动作</label>
              <select v-model="form.action">
                <option value="block">block (拦截)</option>
                <option value="warn">warn (警告)</option>
                <option value="log">log (记录)</option>
              </select>
            </div>
          </div>
          <div class="form-row">
            <div class="form-group" style="flex:1">
              <label>优先级 (0-100)</label>
              <input type="number" v-model.number="form.priority" min="0" max="100" />
            </div>
            <div class="form-group" style="flex:1">
              <label>分组</label>
              <input type="text" v-model="form.group" placeholder="如 injection / jailbreak / pii" />
            </div>
          </div>
          <div class="form-group">
            <label>模式列表 <span class="required">*</span> <span class="hint">(每行一个 pattern)</span></label>
            <textarea v-model="form.patternsText" rows="6" placeholder="每行一个模式&#10;如:&#10;ignore previous instructions&#10;忽略之前的指令" class="mono-textarea"></textarea>
          </div>
          <div class="form-group">
            <label>自定义拦截消息 <span class="hint">(可选)</span></label>
            <textarea v-model="form.message" rows="2" placeholder="如: 检测到攻击，请求已拦截。"></textarea>
          </div>

          <!-- Regex Tester for regex type -->
          <RegexTester v-if="form.type === 'regex' && form.patternsText.trim()" :initial-pattern="firstPattern" />
        </div>
        <div class="editor-footer">
          <button class="btn btn-sm" @click="close">取消</button>
          <button class="btn btn-sm btn-green" @click="submit" :disabled="!canSubmit">
            {{ isEdit ? '更新' : '创建' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import RegexTester from './RegexTester.vue'

const props = defineProps({
  visible: { type: Boolean, default: false },
  rule: { type: Object, default: null }, // null = create, object = edit
})

const emit = defineEmits(['close', 'save'])

const isEdit = computed(() => !!props.rule)

const form = ref({
  name: '',
  type: 'keyword',
  action: 'block',
  priority: 0,
  group: '',
  patternsText: '',
  message: '',
})

watch(() => props.visible, (v) => {
  if (v && props.rule) {
    // Edit mode - populate from rule
    form.value = {
      name: props.rule.name || '',
      type: props.rule.type || 'keyword',
      action: props.rule.action || 'block',
      priority: props.rule.priority || 0,
      group: props.rule.group || '',
      patternsText: (props.rule.patterns || []).join('\n'),
      message: props.rule.message || '',
    }
  } else if (v) {
    // Create mode - reset
    form.value = { name: '', type: 'keyword', action: 'block', priority: 0, group: '', patternsText: '', message: '' }
  }
})

const firstPattern = computed(() => {
  const lines = form.value.patternsText.split('\n').filter(l => l.trim())
  return lines[0] || ''
})

const canSubmit = computed(() => {
  return form.value.name.trim() && form.value.patternsText.trim()
})

function close() {
  emit('close')
}

function submit() {
  const patterns = form.value.patternsText.split('\n').map(l => l.trim()).filter(l => l)
  if (!patterns.length) return
  emit('save', {
    name: form.value.name.trim(),
    type: form.value.type,
    action: form.value.action,
    priority: form.value.priority,
    group: form.value.group.trim(),
    patterns: patterns,
    message: form.value.message.trim(),
  })
}
</script>

<style scoped>
.modal-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,.5); z-index: 1000;
  display: flex; align-items: flex-start; justify-content: center;
  padding-top: 40px;
  animation: fadeIn .2s;
  overflow-y: auto;
}
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
.editor-panel {
  background: var(--bg-surface); border: 1px solid var(--border-default);
  border-radius: var(--radius); width: 600px; max-width: 95vw;
  box-shadow: 0 16px 64px rgba(0,0,0,.5);
  animation: slideUp .25s ease-out;
  margin-bottom: 40px;
}
@keyframes slideUp { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
.editor-header {
  display: flex; align-items: center; gap: 8px;
  padding: 16px 20px; border-bottom: 1px solid var(--border-subtle);
}
.modal-icon { font-size: 1.2rem; }
.modal-title { font-size: 1.05rem; font-weight: 600; flex: 1; color: var(--text); }
.editor-close {
  background: none; border: none; color: var(--text-secondary); font-size: 1.2rem;
  cursor: pointer; padding: 4px 8px; border-radius: 4px;
}
.editor-close:hover { background: rgba(255,255,255,.1); color: var(--text); }
.editor-body { padding: 20px; max-height: 70vh; overflow-y: auto; }
.editor-footer {
  display: flex; justify-content: flex-end; gap: 8px;
  padding: 12px 20px; border-top: 1px solid var(--border-subtle);
}
.form-group { margin-bottom: 14px; }
.form-group label {
  display: block; font-size: .82rem; color: var(--text-secondary); margin-bottom: 4px; font-weight: 500;
}
.form-group input, .form-group select, .form-group textarea {
  width: 100%; background: rgba(0,0,0,.3); color: var(--text);
  border: 1px solid var(--border-default); border-radius: 6px;
  padding: 8px 10px; font-size: .85rem;
}
.form-group input:focus, .form-group select:focus, .form-group textarea:focus {
  border-color: var(--color-primary); outline: none;
}
.form-group select { cursor: pointer; }
.form-group textarea { resize: vertical; font-family: inherit; }
.mono-textarea { font-family: 'Courier New', monospace !important; }
.form-row { display: flex; gap: 12px; }
.required { color: var(--color-danger); }
.hint { color: var(--text-secondary); font-size: .75rem; font-weight: 400; }
</style>
