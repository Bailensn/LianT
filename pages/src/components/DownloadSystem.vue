<script setup lang="ts">
import DownloadArch from "./DownloadArch.vue"
import { onMounted, ref } from "vue"
import type { Build } from "../data/downloads"
import { getLatestRelease, type Release } from "../data/github"

const props = defineProps<{
  title: string
  description?: string
  builds: Build[]
  /** 文件名与 tag 前缀中的产品名，例如 "Manager" / "Service" */
  product: string
  num?: string
}>()

const release = ref<Release>()
const error = ref<string>()
const loading = ref(true)

onMounted(async () => {
  try {
    // 自动获取该产品最新版本（tag 形如 "Manager-v0.01" / "Service-v0.01"）
    release.value = await getLatestRelease(props.product)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <section class="section">
    <div class="container">
      <div class="page-meta">
        <div>
          <h1 class="section-title">
            <span class="num">{{ props.num ?? "00" }}</span>
            {{ title }}
          </h1>
          <p v-if="description" class="muted">{{ description }}</p>
        </div>

        <span v-if="loading" class="pill pill-loading">加载中…</span>
        <span v-else-if="release" class="pill">版本 {{ release.name }}</span>
        <span v-else-if="error" class="pill pill-error">{{ error }}</span>
      </div>

      <div class="systems">
        <DownloadArch
          v-for="item in builds"
          :key="item.os + item.arch"
          :build="item"
          :product="product"
          :release="release"
        />
      </div>
    </div>
  </section>
</template>