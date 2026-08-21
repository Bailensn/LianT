<script setup lang="ts">
import { ref } from "vue"
import LogoIcon from "./components/LogoIcon.vue"

const menuOpen = ref(false)
const year = new Date().getFullYear()

function closeMenu() {
  menuOpen.value = false
}
</script>

<template>
  <div class="noise" aria-hidden="true"></div>

  <header class="nav">
    <div class="container nav-inner">
      <router-link class="logo" to="/" @click="closeMenu">
        <LogoIcon :size="30" />
        <span class="logo-text">LianT · 联T</span>
      </router-link>

      <nav class="nav-links" aria-label="站点导航">
        <router-link to="/">主页</router-link>
        <router-link to="/manager">Manager</router-link>
        <router-link to="/service">Service</router-link>
      </nav>

      <button
        class="menu-btn"
        type="button"
        aria-label="打开菜单"
        :aria-expanded="menuOpen"
        @click="menuOpen = !menuOpen"
      >
        <span class="bar"></span>
        <span class="bar"></span>
        <span class="bar"></span>
      </button>
    </div>

    <transition name="menu">
      <div v-if="menuOpen" class="menu-popover" @click.self="closeMenu">
        <router-link to="/" @click="closeMenu">主页</router-link>
        <router-link to="/manager" @click="closeMenu">Manager</router-link>
        <router-link to="/service" @click="closeMenu">Service</router-link>
      </div>
    </transition>
  </header>

  <router-view v-slot="{ Component }">
    <transition name="page" mode="out-in">
      <component :is="Component" />
    </transition>
  </router-view>
</template>