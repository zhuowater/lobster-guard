<template>
  <div class="login-page">
    <canvas ref="particleCanvas" class="particle-bg"></canvas>
    <div class="glow-orb glow-orb-1"></div>
    <div class="glow-orb glow-orb-2"></div>
    <div class="login-card">
      <div class="login-logo">
        <span class="login-emoji">🦞</span>
        <h1 class="login-title">龙虾卫士</h1>
        <p class="login-subtitle">Lobster Guard</p>
      </div>

      <form class="login-form" @submit.prevent="doLogin">
        <div class="login-field" :class="{ 'field-invalid': fieldErrors.username }">
          <label for="username">用户名</label>
          <input id="username" v-model="username" type="text" placeholder="请输入用户名" autocomplete="username" autofocus @input="fieldErrors.username = ''" />
          <Transition name="fade"><span v-if="fieldErrors.username" class="field-hint">{{ fieldErrors.username }}</span></Transition>
        </div>
        <div class="login-field" :class="{ 'field-invalid': fieldErrors.password }">
          <label for="password">密码</label>
          <div class="password-wrap">
            <input id="password" v-model="password" :type="showPwd ? 'text' : 'password'" placeholder="请输入密码" autocomplete="current-password" @keyup.enter="doLogin" @input="fieldErrors.password = ''" />
            <button type="button" class="pwd-toggle" @click="showPwd = !showPwd" tabindex="-1">{{ showPwd ? '🙈' : '👁️' }}</button>
          </div>
          <Transition name="fade"><span v-if="fieldErrors.password" class="field-hint">{{ fieldErrors.password }}</span></Transition>
        </div>

        <label class="remember-row">
          <input type="checkbox" v-model="rememberMe" />
          <span>记住用户名</span>
        </label>

        <button class="login-btn" type="submit" :disabled="loading">
          <span v-if="loading" class="login-spinner"></span>
          <span v-else>登 录</span>
        </button>

        <Transition name="fade">
          <div v-if="error" class="login-error">
            <span class="login-error-icon">⚠️</span>
            <span>{{ error }}</span>
          </div>
        </Transition>
      </form>

      <div class="login-footer">
        <span class="login-footer-brand">🛡️ AI Agent 安全网关</span>
        <span class="login-footer-ver">v1.0</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { saveToken } from '../api.js'
import { loginUser } from '../stores/app.js'

const REMEMBER_KEY = 'lg_remember_user'
const router = useRouter()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const rememberMe = ref(false)
const showPwd = ref(false)
const fieldErrors = reactive({ username: '', password: '' })

// --- Particle background ---
const particleCanvas = ref(null)
let animId = null
let resizeHandler = null

onMounted(() => {
  const canvas = particleCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  let w, h

  function resize() {
    w = canvas.width = window.innerWidth
    h = canvas.height = window.innerHeight
  }
  resize()
  resizeHandler = resize
  window.addEventListener('resize', resize)

  // Generate particles
  const particles = []
  const PARTICLE_COUNT = 90
  const colors = [
    'rgba(99,102,241,0.7)',   // indigo — bright
    'rgba(168,85,247,0.6)',   // purple
    'rgba(245,158,11,0.5)',   // amber
    'rgba(59,130,246,0.6)',   // blue
    'rgba(239,68,68,0.45)',   // red
    'rgba(255,255,255,0.35)', // white
  ]

  for (let i = 0; i < PARTICLE_COUNT; i++) {
    particles.push({
      x: Math.random() * w,
      y: Math.random() * h,
      r: Math.random() * 3 + 1,          // 半径 1-4（原来 0.5-2.5）
      vx: (Math.random() - 0.5) * 0.4,
      vy: (Math.random() - 0.5) * 0.4,
      color: colors[Math.floor(Math.random() * colors.length)],
      glow: Math.random() > 0.4,          // 60% 有 glow（原来 30%）
      pulsePhase: Math.random() * Math.PI * 2,
      pulseSpeed: 0.008 + Math.random() * 0.015,
    })
  }

  function draw() {
    ctx.clearRect(0, 0, w, h)

    particles.forEach(p => {
      p.x += p.vx
      p.y += p.vy

      if (p.x < -10) p.x = w + 10
      if (p.x > w + 10) p.x = -10
      if (p.y < -10) p.y = h + 10
      if (p.y > h + 10) p.y = -10

      p.pulsePhase += p.pulseSpeed
      const alpha = 0.5 + 0.5 * Math.sin(p.pulsePhase)

      ctx.save()
      ctx.globalAlpha = alpha

      if (p.glow) {
        ctx.shadowBlur = 20
        ctx.shadowColor = p.color
      }

      ctx.beginPath()
      ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2)
      ctx.fillStyle = p.color
      ctx.fill()
      ctx.restore()
    })

    animId = requestAnimationFrame(draw)
  }

  draw()
})

onUnmounted(() => {
  if (animId) cancelAnimationFrame(animId)
  if (resizeHandler) window.removeEventListener('resize', resizeHandler)
})

// --- Restore remembered username ---
const savedUser = localStorage.getItem(REMEMBER_KEY)
if (savedUser) { username.value = savedUser; rememberMe.value = true }

// --- Login logic ---
async function doLogin() {
  // Validate fields
  fieldErrors.username = ''; fieldErrors.password = ''
  let valid = true
  if (!username.value.trim()) { fieldErrors.username = '请输入用户名'; valid = false }
  if (!password.value) { fieldErrors.password = '请输入密码'; valid = false }
  if (!valid) return

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
      error.value = data.error || '登录失败，请检查用户名和密码'
      loading.value = false
      return
    }

    // Remember me
    if (rememberMe.value) { localStorage.setItem(REMEMBER_KEY, username.value) }
    else { localStorage.removeItem(REMEMBER_KEY) }

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
  background:
    radial-gradient(ellipse 80% 50% at 50% -10%, rgba(99,102,241,0.25) 0%, transparent 60%),
    radial-gradient(ellipse 50% 60% at 90% 90%, rgba(245,158,11,0.15) 0%, transparent 50%),
    radial-gradient(ellipse 40% 50% at 10% 80%, rgba(239,68,68,0.12) 0%, transparent 50%),
    radial-gradient(ellipse 60% 40% at 70% 20%, rgba(168,85,247,0.12) 0%, transparent 50%),
    #0a0a0f;
  padding: 20px;
  position: relative;
  overflow: hidden;
}

/* Dot grid overlay */
.login-page::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: radial-gradient(circle 0.8px at center, rgba(99,102,241,0.08) 0.8px, transparent 0.8px);
  background-size: 32px 32px;
  pointer-events: none;
  z-index: 0;
}

/* Slow-drifting large glow spot */
.login-page::after {
  content: '';
  position: absolute;
  width: 500px;
  height: 500px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(99,102,241,0.18) 0%, transparent 70%);
  top: -100px;
  left: 30%;
  animation: float-glow 20s ease-in-out infinite alternate;
  pointer-events: none;
  z-index: 0;
}

/* Particle canvas */
.particle-bg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  z-index: 0;
  pointer-events: none;
}

/* Floating glow orbs */
.glow-orb {
  position: absolute;
  border-radius: 50%;
  pointer-events: none;
  z-index: 0;
}

.glow-orb-1 {
  width: 400px;
  height: 400px;
  background: radial-gradient(circle, rgba(168,85,247,0.18) 0%, transparent 70%);
  bottom: -80px;
  right: 10%;
  animation: float-glow-2 25s ease-in-out infinite alternate;
}

.glow-orb-2 {
  width: 350px;
  height: 350px;
  background: radial-gradient(circle, rgba(245,158,11,0.14) 0%, transparent 70%);
  top: 20%;
  left: -60px;
  animation: float-glow-3 18s ease-in-out infinite alternate;
}

@keyframes float-glow {
  0% { transform: translate(0, 0) scale(1); }
  33% { transform: translate(50px, 30px) scale(1.1); }
  66% { transform: translate(-30px, 50px) scale(0.95); }
  100% { transform: translate(20px, -20px) scale(1.05); }
}

@keyframes float-glow-2 {
  0% { transform: translate(0, 0) scale(1); }
  50% { transform: translate(-40px, -30px) scale(1.15); }
  100% { transform: translate(30px, 20px) scale(0.9); }
}

@keyframes float-glow-3 {
  0% { transform: translate(0, 0) scale(1); }
  40% { transform: translate(30px, 40px) scale(1.1); }
  100% { transform: translate(-20px, -30px) scale(1.05); }
}

.login-card {
  width: 100%;
  max-width: 380px;
  background: rgba(18, 18, 26, 0.85);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid var(--border-subtle, rgba(255,255,255,0.06));
  border-radius: 16px;
  padding: 40px 32px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
  z-index: 1;
  position: relative;
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

/* Password toggle */
.password-wrap { position: relative; display: flex; align-items: center; }
.password-wrap input { width: 100%; padding-right: 40px; }
.pwd-toggle { position: absolute; right: 10px; background: none; border: none; cursor: pointer; font-size: 16px; padding: 0; opacity: 0.5; transition: opacity 0.15s; }
.pwd-toggle:hover { opacity: 1; }

/* Field validation */
.field-invalid input { border-color: rgba(239, 68, 68, 0.5); }
.field-invalid input:focus { box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.15); border-color: rgba(239, 68, 68, 0.5); }
.field-hint { font-size: 11px; color: #f87171; margin-top: 2px; display: block; }

/* Remember me */
.remember-row { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--text-tertiary, rgba(255,255,255,0.4)); cursor: pointer; user-select: none; }
.remember-row input { accent-color: var(--color-primary, #6366f1); cursor: pointer; }

.login-error {
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 8px;
  color: #f87171;
  font-size: 13px;
  padding: 10px 14px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.login-error-icon { flex-shrink: 0; }

.login-footer {
  text-align: center;
  margin-top: 28px;
  font-size: 11px;
  color: var(--text-disabled, rgba(255,255,255,0.15));
  letter-spacing: 1px;
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 8px;
}
.login-footer-brand { opacity: 0.6; }
.login-footer-ver { background: rgba(99,102,241,0.15); color: rgba(99,102,241,0.5); padding: 1px 6px; border-radius: 4px; font-size: 10px; font-family: var(--font-mono, monospace); }

.fade-enter-active { animation: fade-in 0.2s ease-out; }
.fade-leave-active { animation: fade-in 0.15s ease-in reverse; }
@keyframes fade-in { from { opacity: 0; transform: translateY(-4px); } to { opacity: 1; transform: translateY(0); } }
</style>
