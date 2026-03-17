<template>
  <div class="regex-tester">
    <div class="regex-tester-header">
      <span class="card-icon">🔍</span>
      <span class="card-title" style="font-size:.9rem">正则表达式测试器</span>
    </div>
    <div class="regex-row">
      <label>正则表达式</label>
      <input type="text" v-model="pattern" placeholder="输入正则表达式..." class="regex-input" :class="{ 'regex-error': regexError }" />
    </div>
    <div v-if="regexError" class="regex-error-msg">{{ regexError }}</div>
    <div class="regex-row">
      <label>测试文本</label>
      <textarea v-model="testText" rows="4" placeholder="输入测试文本..." class="regex-textarea"></textarea>
    </div>
    <div class="regex-result">
      <div class="regex-status">
        <span v-if="!pattern" class="tag tag-info">请输入正则</span>
        <span v-else-if="regexError" class="tag tag-block">正则无效</span>
        <span v-else-if="matchCount > 0" class="tag tag-block">✅ 匹配 {{ matchCount }} 处</span>
        <span v-else class="tag tag-pass">❌ 未匹配</span>
      </div>
      <div v-if="matches.length > 0" class="match-positions">
        匹配位置:
        <span v-for="(m, i) in matches" :key="i" class="match-pos">[{{ m.start }}-{{ m.end }}]</span>
      </div>
    </div>
    <div v-if="testText && pattern && !regexError" class="highlighted-text" v-html="highlightedHtml"></div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'

const props = defineProps({
  initialPattern: { type: String, default: '' },
})

const emit = defineEmits(['update:pattern'])

const pattern = ref(props.initialPattern)
const testText = ref('')

watch(() => props.initialPattern, (v) => { pattern.value = v })
watch(pattern, (v) => emit('update:pattern', v))

const regexError = computed(() => {
  if (!pattern.value) return ''
  try {
    new RegExp(pattern.value, 'g')
    return ''
  } catch (e) {
    return e.message
  }
})

const matches = computed(() => {
  if (!pattern.value || !testText.value || regexError.value) return []
  try {
    const re = new RegExp(pattern.value, 'g')
    const result = []
    let m
    let safety = 0
    while ((m = re.exec(testText.value)) !== null && safety < 1000) {
      result.push({ start: m.index, end: m.index + m[0].length, text: m[0] })
      if (m[0].length === 0) re.lastIndex++
      safety++
    }
    return result
  } catch {
    return []
  }
})

const matchCount = computed(() => matches.value.length)

const highlightedHtml = computed(() => {
  if (!matches.value.length) return escapeHtml(testText.value)
  const text = testText.value
  let result = ''
  let lastIdx = 0
  for (const m of matches.value) {
    result += escapeHtml(text.slice(lastIdx, m.start))
    result += '<mark class="regex-highlight">' + escapeHtml(text.slice(m.start, m.end)) + '</mark>'
    lastIdx = m.end
  }
  result += escapeHtml(text.slice(lastIdx))
  return result
})

function escapeHtml(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}
</script>

<style scoped>
.regex-tester {
  background: rgba(0,0,0,.2);
  border: 1px solid rgba(0,212,255,.15);
  border-radius: var(--radius);
  padding: 16px;
  margin-top: 12px;
}
.regex-tester-header {
  display: flex; align-items: center; gap: 8px; margin-bottom: 12px;
}
.regex-row {
  margin-bottom: 10px;
}
.regex-row label {
  display: block; font-size: .8rem; color: var(--text-dim); margin-bottom: 4px;
}
.regex-input, .regex-textarea {
  width: 100%; background: rgba(0,0,0,.3); color: var(--text);
  border: 1px solid rgba(0,212,255,.2); border-radius: 6px;
  padding: 8px; font-size: .85rem; font-family: 'Courier New', monospace;
  resize: vertical;
}
.regex-input:focus, .regex-textarea:focus {
  border-color: var(--neon-blue); outline: none;
}
.regex-input.regex-error {
  border-color: var(--neon-red);
}
.regex-error-msg {
  color: var(--neon-red); font-size: .75rem; margin: -6px 0 8px;
}
.regex-result {
  display: flex; align-items: center; gap: 12px; margin-bottom: 8px; flex-wrap: wrap;
}
.regex-status { display: flex; align-items: center; gap: 6px; }
.match-positions { font-size: .75rem; color: var(--text-dim); }
.match-pos {
  display: inline-block; background: rgba(255,68,102,.15); color: var(--neon-red);
  padding: 1px 4px; border-radius: 3px; margin: 0 2px; font-family: monospace; font-size: .72rem;
}
.highlighted-text {
  background: rgba(0,0,0,.3); padding: 10px; border-radius: 6px;
  font-family: 'Courier New', monospace; font-size: .85rem;
  white-space: pre-wrap; word-break: break-all; color: var(--text);
  max-height: 200px; overflow-y: auto; line-height: 1.6;
}
:deep(.regex-highlight) {
  background: rgba(255,68,102,.35); color: #fff; border-radius: 2px;
  padding: 1px 2px;
}
</style>
