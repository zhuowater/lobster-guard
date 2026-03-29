<template>
  <div class="empty-state">
    <div class="empty-state-icon" v-if="iconSvg" v-html="iconSvg"></div>
    <div class="empty-state-icon" v-else-if="icon">{{ icon }}</div>
    <div class="empty-state-title">{{ title || text || '暂无数据' }}</div>
    <div class="empty-state-desc" v-if="description"><slot>{{ description }}</slot></div>
    <div class="empty-state-desc" v-else-if="$slots.default"><slot></slot></div>
    <button v-if="actionText" class="btn btn-ghost empty-state-action" @click="$emit('action')">{{ actionText }}</button>
  </div>
</template>

<script setup>
defineProps({
  icon: { type: String, default: '' },
  iconSvg: { type: String, default: '' },
  text: { type: String, default: '' },
  title: { type: String, default: '' },
  description: { type: String, default: '' },
  actionText: { type: String, default: '' },
})

defineEmits(['action'])
</script>

<style scoped>
.empty-state {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: var(--spacing-xl) var(--spacing-md);
  text-align: center;
}
.empty-state-icon {
  color: var(--text-disabled);
  margin-bottom: var(--space-4);
  font-size: 2.5rem;
  opacity: .5;
}
.empty-state-icon :deep(svg) {
  width: 48px; height: 48px; stroke: var(--text-disabled);
}
.empty-state-title {
  font-size: var(--text-lg); color: var(--text-secondary); font-weight: 700;
  margin-bottom: var(--space-2);
}
.empty-state-desc {
  font-size: var(--text-sm); color: var(--text-tertiary); max-width: 320px; line-height: 1.6;
}
.empty-state-action {
  margin-top: var(--space-4);
}
</style>
