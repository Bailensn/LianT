<script setup lang="ts">
import DownloadArch from "./DownloadArch.vue"
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

// Release 数据在构建期由 scripts/fetch-releases.mjs 静态化，运行时直接读取
const release = getLatestRelease(props.product) as Release | undefined
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

        <span v-if="release" class="pill">版本 {{ release.name }}</span>
        <span v-else class="pill pill-error">暂无 Release 数据</span>
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