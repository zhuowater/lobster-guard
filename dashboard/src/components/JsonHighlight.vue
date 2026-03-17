<template>
  <div class="json-hl-wrap">
    <button class="json-copy-btn" @click="copyContent" :title="copied ? '已复制' : '复制内容'">
      <svg v-if="!copied" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
      <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
    </button>
    <pre class="json-hl" v-if="isJson" v-html="highlighted"></pre>
    <pre class="json-hl json-plain" v-else>{{ content }}</pre>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  content: { type: String, default: '' },
})

const copied = ref(false)

const isJson = computed(() => {
  const trimmed = (props.content || '').trim()
  return (trimmed.startsWith('{') || trimmed.startsWith('[')) && tryParseJson(trimmed)
})

function tryParseJson(s) {
  try { JSON.parse(s); return true } catch { return false }
}

const highlighted = computed(() => {
  if (!isJson.value) return escHtml(props.content || '')
  try {
    const formatted = JSON.stringify(JSON.parse(props.content.trim()), null, 2)
    return syntaxHighlight(formatted)
  } catch {
    return escHtml(props.content || '')
  }
})

function escHtml(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function syntaxHighlight(json) {
  const escaped = escHtml(json)
  return escaped.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
    (match) => {
      let cls = 'json-number' // number
      if (/^"/.test(match)) {
        if (/:$/.test(match)) {
          cls = 'json-key' // key
        } else {
          cls = 'json-string' // string
        }
      } else if (/true|false/.test(match)) {
        cls = 'json-boolean'
      } else if (/null/.test(match)) {
        cls = 'json-null'
      }
      return '<span class="' + cls + '">' + match + '</span>'
    }
  )
}

async function copyContent() {
  try {
    await navigator.clipboard.writeText(props.content || '')
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  } catch { /* ignore */ }
}
</script>

<style scoped>
.json-hl-wrap {
  position: relative;
}
.json-copy-btn {
  position: absolute; top: 8px; right: 8px;
  background: var(--bg-overlay); border: 1px solid var(--border-default);
  border-radius: var(--radius-sm); color: var(--text-tertiary);
  padding: 4px 6px; cursor: pointer; display: flex; align-items: center;
  transition: all var(--transition-fast); z-index: 2;
}
.json-copy-btn:hover { color: var(--text-primary); background: var(--bg-elevated); border-color: var(--color-primary); }
.json-hl {
  background: var(--bg-base);
  padding: var(--space-3);
  border-radius: var(--radius-md);
  margin-top: var(--space-1);
  font-size: var(--text-xs);
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--text-primary);
  border: 1px solid var(--border-subtle);
  font-family: var(--font-mono);
  line-height: 1.6;
  max-height: 300px;
  overflow-y: auto;
}
.json-hl :deep(.json-key) { color: #6CB6FF; }
.json-hl :deep(.json-string) { color: #7EE787; }
.json-hl :deep(.json-number) { color: #FFA657; }
.json-hl :deep(.json-boolean) { color: #D2A8FF; }
.json-hl :deep(.json-null) { color: #D2A8FF; font-style: italic; }
</style>
