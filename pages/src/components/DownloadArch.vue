<script setup lang="ts">
import { computed } from "vue"
import SystemIcon from "./SystemIcon.vue"
import type { Build } from "../data/downloads"
import type { Release } from "../data/github"

const props = defineProps<{
  build: Build
  release: Release | undefined
}>()

function getVersion(release: Release) {
  return release.name?.trim() || release.tag_name
}

function getFile(format: string) {
  if (!props.release) return undefined
  const version = getVersion(props.release)
  const filename = `LianT_${version}_${props.build.os}_${props.build.arch}.${format}`
  const asset = props.release.assets.find(a => a.name === filename)
  return asset?.browser_download_url
}

function onClick(e: MouseEvent, url: string | undefined) {
  if (!url) e.preventDefault()
}

const formatLinks = computed(() =>
  props.build.formats.map(format => ({
    format,
    url: getFile(format),
  }))
)

const hasDownload = computed(() =>
  formatLinks.value.some(item => item.url)
)
</script>

<template>
  <div class="card" :class="{ 'card-empty': !hasDownload }">
    <div class="card-head">
      <div class="card-icon">
        <SystemIcon :os="build.os" :size="32" />
      </div>
      <div>
        <h2>{{ build.os }}</h2>
        <h3>{{ build.arch }}</h3>
      </div>
    </div>

    <div class="formats">
      <a
        v-for="item in formatLinks"
        :key="item.format"
        :href="item.url ?? '#'"
        :class="{ disabled: !item.url }"
        :aria-disabled="!item.url"
        @click="(e) => onClick(e, item.url)"
      >
        .{{ item.format }}
      </a>
    </div>
  </div>
</template>