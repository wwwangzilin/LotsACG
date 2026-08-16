<template>
  <div class="settings">
    <h2>⚙️ 设置</h2>

    <var-card title="瀑布流设置" class="card">
      <var-cell title="每行最大列数 (宽屏)">
        <template #default>
          <div class="cell-control">
            <var-slider v-model="local.maxColumnCount" :min="2" :max="10" :step="1" />
            <span class="value">{{ local.maxColumnCount }}</span>
          </div>
        </template>
      </var-cell>
      <var-cell title="每行最小列数 (窄屏)">
        <template #default>
          <div class="cell-control">
            <var-slider v-model="local.minColumnCount" :min="1" :max="6" :step="1" />
            <span class="value">{{ local.minColumnCount }}</span>
          </div>
        </template>
      </var-cell>
      <var-cell title="卡片最小宽度 (px)">
        <template #default>
          <div class="cell-control">
            <var-slider v-model="local.itemMinWidth" :min="200" :max="600" :step="10" />
            <span class="value">{{ local.itemMinWidth }}</span>
          </div>
        </template>
      </var-cell>
      <var-cell title="预加载屏数 (越大加载越快, 越耗流量)">
        <template #default>
          <div class="cell-control">
            <var-slider v-model="local.preloadScreenCount" :min="0" :max="3" :step="1" />
            <span class="value">{{ local.preloadScreenCount }}</span>
          </div>
        </template>
      </var-cell>
      <var-cell title="滚动加载触发距离 (px, 越大越早加载)">
        <template #default>
          <div class="cell-control">
            <var-slider v-model="local.bottomDistance" :min="200" :max="2000" :step="50" />
            <span class="value">{{ local.bottomDistance }}</span>
          </div>
        </template>
      </var-cell>
      <var-cell title="仅图片模式 (不显示标题, 更流畅)">
        <template #default>
          <var-switch v-model="local.onlyImage" />
        </template>
      </var-cell>
    </var-card>

    <var-card title="内容设置" class="card">
      <var-cell title="R18 内容">
        <template #default>
          <var-switch v-model="local.r18" />
        </template>
      </var-cell>
      <var-cell title="浅色主题">
        <template #default>
          <var-switch v-model="local.preferLight" />
        </template>
      </var-cell>
    </var-card>

    <div class="actions">
      <var-button type="primary" @click="save">保存并刷新</var-button>
      <var-button @click="reset">恢复默认</var-button>
    </div>

    <var-card title="Bot 操控" class="card">
      <var-cell title="API Key" :border="false">
        <template #default>
          <var-input v-model="apiKey" placeholder="粘贴 X-API-KEY" password-toggle />
        </template>
      </var-cell>
      <var-cell title="推送到主频道">
        <template #default>
          <div class="bot-row">
            <var-input v-model="postUrl" placeholder="作品链接 (pixiv / twitter / ...)" class="flex-1" />
            <var-button type="primary" size="small" :loading="posting" @click="postToChannel">推送</var-button>
          </div>
        </template>
      </var-cell>
      <var-cell title="发送到指定群">
        <template #default>
          <div class="bot-col">
            <var-input v-model="sendUrl" placeholder="作品链接" />
            <div class="bot-row">
              <var-input v-model="chatId" placeholder="群/用户 Chat ID" class="flex-1" />
              <var-button type="primary" size="small" :loading="sending" @click="sendInfo">发送</var-button>
            </div>
          </div>
        </template>
      </var-cell>
      <var-cell title="Bot 状态">
        <template #default>
          <div class="bot-col">
            <var-button size="small" :loading="statusLoading" @click="fetchStatus">查看状态</var-button>
            <pre v-if="statusText" class="status">{{ statusText }}</pre>
          </div>
        </template>
      </var-cell>
    </var-card>
  </div>
</template>

<script lang="ts" setup>
import { Snackbar, StyleProvider } from '@varlet/ui'
import type { BaseResponse } from '~/types/artwork'

useHead({
  title: '设置'
})

const piniaStore = usePiniaStore()

const local = reactive({
  ...piniaStore.waterfall,
  r18: piniaStore.r18,
  preferLight: piniaStore.preferLight
})

const API_KEY_STORAGE = 'manyacg_api_key'
const apiKey = ref('')

onMounted(() => {
  if (import.meta.client) {
    apiKey.value = localStorage.getItem(API_KEY_STORAGE) || ''
  }
})

const save = () => {
  piniaStore.setWaterfall({
    itemMinWidth: local.itemMinWidth,
    minColumnCount: local.minColumnCount,
    maxColumnCount: local.maxColumnCount,
    preloadScreenCount: local.preloadScreenCount,
    bottomDistance: local.bottomDistance,
    onlyImage: local.onlyImage
  })
  piniaStore.setR18(local.r18)
  piniaStore.setpreferLight(local.preferLight)
  if (import.meta.client) {
    localStorage.setItem(API_KEY_STORAGE, apiKey.value)
  }
  StyleProvider(local.preferLight ? lightTheme : darkTheme)
  Snackbar.success('已保存')
  if (import.meta.client) {
    window.location.reload()
  }
}

const reset = () => {
  piniaStore.resetWaterfall()
  local.itemMinWidth = piniaStore.waterfall.itemMinWidth
  local.minColumnCount = piniaStore.waterfall.minColumnCount
  local.maxColumnCount = piniaStore.waterfall.maxColumnCount
  local.preloadScreenCount = piniaStore.waterfall.preloadScreenCount
  local.bottomDistance = piniaStore.waterfall.bottomDistance
  local.onlyImage = piniaStore.waterfall.onlyImage
  Snackbar.info('已恢复默认, 点击「保存并刷新」生效')
}

const posting = ref(false)
const postUrl = ref('')
const postToChannel = async () => {
  if (!apiKey.value) {
    Snackbar.warning('请先填写 API Key')
    return
  }
  if (!postUrl.value) {
    Snackbar.warning('请填写作品链接')
    return
  }
  posting.value = true
  try {
    await $acgapi<BaseResponse<string>>('/bot/post_artwork', {
      method: 'POST',
      headers: { 'X-API-KEY': apiKey.value },
      body: { source_url: postUrl.value }
    })
    Snackbar.success('已推送到主频道')
  } catch (e: any) {
    Snackbar.error('推送失败: ' + (e?.data?.message || e?.message || '未知错误'))
  } finally {
    posting.value = false
  }
}

const sending = ref(false)
const sendUrl = ref('')
const chatId = ref('')
const sendInfo = async () => {
  if (!apiKey.value) {
    Snackbar.warning('请先填写 API Key')
    return
  }
  if (!sendUrl.value) {
    Snackbar.warning('请填写作品链接')
    return
  }
  const id = Number(chatId.value)
  if (!id) {
    Snackbar.warning('请填写有效的 Chat ID')
    return
  }
  sending.value = true
  try {
    await $acgapi<BaseResponse<string>>('/bot/send_artwork_info', {
      method: 'POST',
      headers: { 'X-API-KEY': apiKey.value },
      body: { source_url: sendUrl.value, chat_id: id }
    })
    Snackbar.success('已发送')
  } catch (e: any) {
    Snackbar.error('发送失败: ' + (e?.data?.message || e?.message || '未知错误'))
  } finally {
    sending.value = false
  }
}

const statusLoading = ref(false)
const statusText = ref('')
const fetchStatus = async () => {
  if (!apiKey.value) {
    Snackbar.warning('请先填写 API Key')
    return
  }
  statusLoading.value = true
  try {
    const resp = await $acgapi<BaseResponse<any>>('/bot/status', {
      method: 'GET',
      headers: { 'X-API-KEY': apiKey.value }
    })
    const s = resp.data
    statusText.value = [
      `运行中: ${s.running ? '✅' : '❌'}`,
      `Bot: @${s.bot_username || '-'}`,
      `主频道: ${s.channel_name ? '@' + s.channel_name : s.channel_id || '-'}`,
      `群: ${s.group_id || '-'}`,
      `R18 频道: ${s.r18_channel_id || '-'}`
    ].join('\n')
    Snackbar.success('状态获取成功')
  } catch (e: any) {
    Snackbar.error('获取失败: ' + (e?.data?.message || e?.message || '未知错误'))
  } finally {
    statusLoading.value = false
  }
}
</script>

<style scoped>
.settings {
  margin: 0 auto;
  padding: 20px;
  max-width: 900px;
}

.card {
  margin-bottom: 20px;
}

.cell-control {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 240px;
}

.cell-control .value {
  min-width: 32px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.actions {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}

.bot-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.bot-col {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.flex-1 {
  flex: 1;
}

.status {
  margin: 8px 0 0;
  padding: 8px 12px;
  background: rgba(128, 128, 128, 0.12);
  border-radius: 6px;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
}
</style>
