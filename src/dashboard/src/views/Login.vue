<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-logo">
        <span class="login-emoji">🦞</span>
        <h1 class="login-title">龙虾卫士</h1>
        <p class="login-subtitle">Lobster Guard</p>
      </div>

      <form class="login-form" @submit.prevent="doLogin">
        <div class="login-field">
          <label for="username">用户名</label>
          <input id="username" v-model="username" type="text" placeholder="请输入用户名" autocomplete="username" autofocus />
        </div>
        <div class="login-field">
          <label for="password">密码</label>
          <input id="password" v-model="password" type="password" placeholder="请输入密码" autocomplete="current-password" @keyup.enter="doLogin" />
        </div>

        <button class="login-btn" type="submit" :disabled="loading">
          <span v-if="loading" class="login-spinner"></span>
          <span v-else>登 录</span>
        </button>

        <Transition name="fade">
          <div v-if="error" class="login-error">{{ error }}</div>
        </Transition>
      </form>

      <div class="login-footer">
        AI Agent 安全网关
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { saveToken } from '../api.js'
import { loginUser } from '../stores/app.js'

const router = useRouter()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function doLogin() {
  if (!username.value || !password.value) {
    error.value = '请输入用户名和密码'
    return
  }
  error.value = ''
  loading.value = true

  try {
    const res = await fetch(location.origin + '/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username.value, password: password.value })
    })
    const data = await res.json()

    if (!res.ok || !data.token) {
      error.value = data.error || '登录失败'
      loading.value = false
      return
    }

    // 保存 token 和用户信息
    saveToken(data.token)
    loginUser(data.token, data.user)
    router.replace('/')
  } catch (e) {
    error.value = '网络错误，请检查连接'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-base, #0a0a0f);
  background-image:
    radial-gradient(ellipse 60% 50% at 50% 0%, rgba(99, 102, 241, 0.08) 0%, transparent 60%),
    radial-gradient(ellipse 40% 40% at 80% 80%, rgba(239, 68, 68, 0.04) 0%, transparent 60%);
  padding: 20px;
}

.login-card {
  width: 100%;
  max-width: 380px;
  background: var(--bg-surface, #12121a);
  border: 1px solid var(--border-subtle, rgba(255,255,255,0.06));
  border-radius: 16px;
  padding: 40px 32px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
}

.login-logo {
  text-align: center;
  margin-bottom: 32px;
}

.login-emoji {
  font-size: 48px;
  display: block;
  margin-bottom: 8px;
  filter: drop-shadow(0 2px 8px rgba(239, 68, 68, 0.3));
}

.login-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary, #f1f1f4);
  margin: 0 0 4px 0;
  letter-spacing: 2px;
}

.login-subtitle {
  font-size: 13px;
  color: var(--text-tertiary, rgba(255,255,255,0.35));
  margin: 0;
  font-family: var(--font-mono, 'SF Mono', monospace);
  letter-spacing: 1px;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.login-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.login-field label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary, rgba(255,255,255,0.55));
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.login-field input {
  width: 100%;
  background: var(--bg-elevated, rgba(255,255,255,0.04));
  border: 1px solid var(--border-default, rgba(255,255,255,0.1));
  border-radius: 8px;
  color: var(--text-primary, #f1f1f4);
  padding: 10px 14px;
  font-size: 14px;
  font-family: var(--font-sans, system-ui);
  outline: none;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
  box-sizing: border-box;
}

.login-field input:focus {
  border-color: var(--color-primary, #6366f1);
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.15);
}

.login-field input::placeholder {
  color: var(--text-disabled, rgba(255,255,255,0.2));
}

.login-btn {
  width: 100%;
  padding: 11px 0;
  background: var(--color-primary, #6366f1);
  color: #fff;
  border: none;
  border-radius: 8px;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease, opacity 0.15s ease;
  letter-spacing: 4px;
  margin-top: 4px;
}

.login-btn:hover:not(:disabled) {
  background: #5558e6;
}

.login-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.login-spinner {
  display: inline-block;
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.login-error {
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 8px;
  color: #f87171;
  font-size: 13px;
  padding: 10px 14px;
  text-align: center;
}

.login-footer {
  text-align: center;
  margin-top: 28px;
  font-size: 11px;
  color: var(--text-disabled, rgba(255,255,255,0.15));
  letter-spacing: 1px;
}

.fade-enter-active { animation: fade-in 0.2s ease-out; }
.fade-leave-active { animation: fade-in 0.15s ease-in reverse; }
@keyframes fade-in { from { opacity: 0; transform: translateY(-4px); } to { opacity: 1; transform: translateY(0); } }
</style>
