<script setup lang="ts">
import { computed } from "vue"
import SystemIcon from "./SystemIcon.vue"
import type { Build } from "../data/downloads"
import { buildReleaseFileName } from "../data/downloads"
import type { Release } from "../data/github"

const props = defineProps<{
  build: Build
  product: string
  release: Release | undefined
}>()

function getVersion(release: Release) {
  return release.name?.trim() || release.tag_name
}

function getUrl(asset: { ext: string }) {
  if (!props.release) return undefined
  const filename = buildReleaseFileName({
    product: props.product,
    version: getVersion(props.release),
    os: props.build.os,
    arch: props.build.arch,
    ext: asset.ext,
  })
  const found = props.release.assets.find(a => a.name === filename)
  return found?.browser_download_url
}

function onClick(e: MouseEvent, url: string | undefined) {
  if (!url) e.preventDefault()
}

const assetLinks = computed(() =>
  props.build.assets.map(asset => ({
    asset,
    url: getUrl(asset),
  }))
)

const hasDownload = computed(() =>
  assetLinks.value.some(item => item.url)
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
        v-for="(item, i) in assetLinks"
        :key="item.asset.ext + i"
        :href="item.url ?? '#'"
        :class="{ disabled: !item.url }"
        :aria-disabled="!item.url"
        @click="(e) => onClick(e, item.url)"
      >
        {{ item.asset.label }}
      </a>
    </div>
  </div>
</template>