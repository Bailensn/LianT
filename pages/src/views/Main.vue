<script setup lang="ts">
const steps = [
  { num: "01", title: "选择组件", desc: "Manager 或 Service" },
  { num: "02", title: "选择平台", desc: "系统与架构" },
  { num: "03", title: "直接下载", desc: "GitHub 直链" },
]
</script>

<template>
  <main>
    <section class="hero">
      <div class="container hero-grid">
        <div class="hero-copy">
          <p class="eyebrow">‖ 你好，这里是</p>
          <h1>LianT <span class="hero-accent">联T</span></h1>
          <p class="lead">
            Telegram Bot 管理器与服务端组件。服务器自备。
          </p>

          <div class="hero-cta">
            <router-link class="btn btn-primary" to="/manager">
              <span class="btn-icon">↓</span>
              获取 Manager
            </router-link>
            <router-link class="btn btn-ghost" to="/service">
              获取 Service
              <span class="btn-arrow">→</span>
            </router-link>
          </div>
        </div>

        <div class="code-window" aria-hidden="true">
          <div class="code-header">
            <span class="dot red"></span>
            <span class="dot yellow"></span>
            <span class="dot green"></span>
            <span class="code-title">quick-start.txt</span>
          </div>
          <pre class="code-body"><code><span class="code-comment"># 快速开始</span>
<span class="code-line"><span class="code-num">1.</span> 选择要下载的组件</span>
<span class="code-line"><span class="code-num">2.</span> 选系统与架构</span>
<span class="code-line"><span class="code-num">3.</span> 一键直链下载</span>

<span class="code-tag">.exe</span> <span class="code-tag">.deb</span> <span class="code-tag">.rpm</span> <span class="code-tag">.apk</span></code></pre>
        </div>
      </div>
    </section>

    <section class="section">
      <div class="container">
        <h2 class="section-title">
          <span class="num">01</span>
          如何使用
        </h2>

        <div class="steps">
          <div v-for="step in steps" :key="step.num" class="step-card">
            <span class="step-num">{{ step.num }}</span>
            <div>
              <h3>{{ step.title }}</h3>
              <p>{{ step.desc }}</p>
            </div>
          </div>
        </div>
      </div>
    </section>
  </main>
</template>