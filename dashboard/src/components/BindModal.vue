<template>
  <Teleport to="body">
    <div v-if="visible" class="bm-overlay" @click.self="handleClose" @keydown.esc="handleClose">
      <div class="bm-panel" :class="type" ref="panelRef">
        <!-- Header -->
        <div class="bm-header">
          <div class="bm-header-left">
            <span v-if="icon" class="bm-icon">{{ icon }}</span>
            <div>
              <div class="bm-title">{{ title }}</div>
              <div v-if="description" class="bm-desc">{{ description }}</div>
            </div>
          </div>
          <button class="bm-close" @click="handleClose" title="关闭 (ESC)">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>

        <!-- Warning banner -->
        <div v-if="warning" class="bm-warning">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
          <span>{{ warning }}</span>
        </div>

        <!-- Body -->
        <div class="bm-body">
          <div v-for="field in fields" :key="field.key" class="bm-field">
            <label class="bm-label">
              {{ field.label }}
              <span v-if="field.required" class="bm-required">*</span>
            </label>
            <!-- Select -->
            <div v-if="field.type === 'select'" class="bm-input-wrap">
              <select
                :value="modelValue[field.key]"
                @input="updateField(field.key, $event.target.value)"
                class="bm-select"
                :class="{ 'bm-error-border': fieldErrors[field.key] }"
              >
                <option value="" disabled>{{ field.placeholder || '请选择...' }}</option>
                <option v-for="opt in field.options" :key="opt.value" :value="opt.value" :disabled="opt.disabled">
                  {{ opt.label }}
                </option>
              </select>
            </div>
            <!-- Textarea -->
            <div v-else-if="field.type === 'textarea'" class="bm-input-wrap">
              <textarea
                :value="modelValue[field.key]"
                @input="updateField(field.key, $event.target.value)"
                :placeholder="field.placeholder"
                :rows="field.rows || 4"
                class="bm-textarea"
                :class="{ 'bm-error-border': fieldErrors[field.key] }"
              ></textarea>
            </div>
            <!-- Checkbox / Toggle -->
            <div v-else-if="field.type === 'checkbox'" class="bm-input-wrap bm-toggle-wrap">
              <label class="bm-toggle" @click.prevent="updateField(field.key, !modelValue[field.key])">
                <span class="bm-toggle-track" :class="{ active: !!modelValue[field.key] }">
                  <span class="bm-toggle-thumb"></span>
                </span>
                <span class="bm-toggle-label">{{ modelValue[field.key] ? '已启用' : '未启用' }}</span>
              </label>
            </div>
            <!-- Number -->
            <div v-else-if="field.type === 'number'" class="bm-input-wrap">
              <input
                type="number"
                :value="modelValue[field.key]"
                @input="updateField(field.key, parseInt($event.target.value) || 0)"
                :placeholder="field.placeholder"
                class="bm-input"
                :class="{ 'bm-error-border': fieldErrors[field.key] }"
              />
            </div>
            <!-- Slot for custom component (e.g. UpstreamSelect) -->
            <div v-else-if="field.type === 'component'" class="bm-input-wrap">
              <slot :name="'field-' + field.key" :value="modelValue[field.key]" :update="(v) => updateField(field.key, v)"></slot>
            </div>
            <!-- Default input -->
            <div v-else class="bm-input-wrap">
              <input
                :type="field.inputType || 'text'"
                :value="modelValue[field.key]"
                @input="updateField(field.key, $event.target.value)"
                :placeholder="field.placeholder"
                class="bm-input"
                :class="{ 'bm-error-border': fieldErrors[field.key] }"
              />
            </div>
            <!-- Hint -->
            <div v-if="field.hint && !fieldErrors[field.key]" class="bm-hint">{{ field.hint }}</div>
            <!-- Error -->
            <div v-if="fieldErrors[field.key]" class="bm-field-error">{{ fieldErrors[field.key] }}</div>
          </div>

          <!-- Preview slot -->
          <slot name="preview"></slot>
        </div>

        <!-- Footer -->
        <div class="bm-footer">
          <button class="btn btn-ghost btn-sm" @click="handleClose">{{ cancelText }}</button>
          <button class="btn btn-sm" :class="confirmBtnClass" @click="handleConfirm" :disabled="loading">
            <span v-if="loading" class="bm-spinner"></span>
            {{ confirmText }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'

const props = defineProps({
  visible: { type: Boolean, default: false },
  title: { type: String, default: '操作' },
  description: { type: String, default: '' },
  icon: { type: String, default: '' },
  warning: { type: String, default: '' },
  type: { type: String, default: 'default' }, // default | danger | warning
  fields: { type: Array, default: () => [] }, // [{ key, label, type, placeholder, required, options, hint, rows }]
  modelValue: { type: Object, default: () => ({}) },
  confirmText: { type: String, default: '确认' },
  cancelText: { type: String, default: '取消' },
  loading: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'confirm', 'cancel'])

const panelRef = ref(null)
const fieldErrors = ref({})

const confirmBtnClass = computed(() => {
  if (props.type === 'danger') return 'btn-danger'
  if (props.type === 'warning') return 'btn-purple'
  return ''
})

function updateField(key, value) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
  // Clear error on input
  if (fieldErrors.value[key]) {
    fieldErrors.value = { ...fieldErrors.value, [key]: '' }
  }
}

function validate() {
  const errors = {}
  let valid = true
  for (const field of props.fields) {
    if (field.required) {
      const val = props.modelValue[field.key]
      if (!val || (typeof val === 'string' && !val.trim())) {
        errors[field.key] = `${field.label} 不能为空`
        valid = false
      }
    }
  }
  fieldErrors.value = errors
  return valid
}

function handleConfirm() {
  if (validate()) {
    emit('confirm', { ...props.modelValue })
  }
}

function handleClose() {
  fieldErrors.value = {}
  emit('cancel')
}

function onKeydown(e) {
  if (e.key === 'Escape' && props.visible) {
    handleClose()
  }
}

watch(() => props.visible, (v) => {
  if (v) {
    fieldErrors.value = {}
    document.addEventListener('keydown', onKeydown)
  } else {
    document.removeEventListener('keydown', onKeydown)
  }
})

onUnmounted(() => {
  document.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
.bm-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,.6); backdrop-filter: blur(4px);
  z-index: 1000;
  display: flex; align-items: flex-start; justify-content: center;
  padding-top: 80px;
  animation: bm-fade .2s;
}
@keyframes bm-fade { from { opacity: 0; } to { opacity: 1; } }

.bm-panel {
  background: var(--bg-surface); border: 1px solid var(--border-default);
  border-radius: var(--radius-lg); width: 520px; max-width: 95vw;
  box-shadow: 0 20px 60px rgba(0,0,0,.5), 0 0 0 1px rgba(99,102,241,0.08);
  animation: bm-slide .25s ease-out;
  max-height: calc(100vh - 120px);
  display: flex; flex-direction: column;
}
.bm-panel.danger { border-color: rgba(239,68,68,.2); }
.bm-panel.warning { border-color: rgba(234,179,8,.2); }
@keyframes bm-slide { from { opacity: 0; transform: translateY(16px) scale(0.98); } to { opacity: 1; transform: translateY(0) scale(1); } }

.bm-header {
  display: flex; align-items: flex-start; justify-content: space-between;
  padding: var(--space-5); border-bottom: 1px solid var(--border-subtle);
}
.bm-header-left { display: flex; align-items: flex-start; gap: var(--space-3); }
.bm-icon { font-size: 1.4rem; line-height: 1; }
.bm-title { font-size: var(--text-lg); font-weight: 600; color: var(--text-primary); }
.bm-desc { font-size: var(--text-sm); color: var(--text-secondary); margin-top: 2px; }
.bm-close {
  background: none; border: none; color: var(--text-tertiary); cursor: pointer;
  padding: var(--space-1); border-radius: var(--radius-sm);
  transition: all var(--transition-fast);
  display: flex; align-items: center; justify-content: center;
}
.bm-close:hover { background: var(--bg-elevated); color: var(--text-primary); }

.bm-warning {
  display: flex; align-items: center; gap: var(--space-2);
  padding: var(--space-3) var(--space-5);
  background: var(--color-warning-dim);
  color: var(--color-warning);
  font-size: var(--text-sm); font-weight: 500;
  border-bottom: 1px solid rgba(234,179,8,0.1);
}

.bm-body {
  padding: var(--space-5);
  overflow-y: auto; flex: 1;
}

.bm-field { margin-bottom: var(--space-4); }
.bm-field:last-child { margin-bottom: 0; }
.bm-label {
  display: block; font-size: var(--text-sm); color: var(--text-secondary);
  font-weight: 500; margin-bottom: var(--space-1);
}
.bm-required { color: var(--color-danger); margin-left: 2px; }

.bm-input-wrap { position: relative; }

.bm-input, .bm-select, .bm-textarea {
  width: 100%;
  background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary);
  padding: var(--space-2) var(--space-3);
  font-size: var(--text-sm); outline: none;
  font-family: var(--font-sans);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
}
.bm-input:focus, .bm-select:focus, .bm-textarea:focus {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-dim);
}
.bm-input::placeholder, .bm-textarea::placeholder { color: var(--text-disabled); }
.bm-select option { background: var(--bg-elevated); color: var(--text-primary); }
.bm-textarea {
  font-family: var(--font-mono); font-size: var(--text-xs);
  resize: vertical; min-height: 80px;
}

.bm-error-border {
  border-color: var(--color-danger) !important;
  box-shadow: 0 0 0 3px var(--color-danger-dim) !important;
}

.bm-hint { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); }
.bm-field-error { font-size: var(--text-xs); color: var(--color-danger); margin-top: var(--space-1); }

.bm-footer {
  display: flex; justify-content: flex-end; gap: var(--space-2);
  padding: var(--space-4) var(--space-5);
  border-top: 1px solid var(--border-subtle);
}

/* Toggle switch */
.bm-toggle-wrap { padding: var(--space-1) 0; }
.bm-toggle {
  display: inline-flex; align-items: center; gap: var(--space-2); cursor: pointer;
  user-select: none;
}
.bm-toggle-track {
  position: relative; width: 40px; height: 22px;
  background: var(--border-default); border-radius: 11px;
  transition: background var(--transition-fast);
}
.bm-toggle-track.active {
  background: var(--color-primary, #6366f1);
}
.bm-toggle-thumb {
  position: absolute; top: 2px; left: 2px;
  width: 18px; height: 18px; background: #fff;
  border-radius: 50%; transition: transform var(--transition-fast);
  box-shadow: 0 1px 3px rgba(0,0,0,.2);
}
.bm-toggle-track.active .bm-toggle-thumb {
  transform: translateX(18px);
}
.bm-toggle-label {
  font-size: var(--text-sm); color: var(--text-secondary); font-weight: 500;
}

.bm-spinner {
  display: inline-block; width: 12px; height: 12px;
  border: 2px solid rgba(255,255,255,0.3); border-top-color: #fff;
  border-radius: 50%; animation: spn .6s linear infinite;
}
</style>
