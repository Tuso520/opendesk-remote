<template>
  <a-card class="panel" :bordered="false">
    <template #title>System Status</template>
    <template #extra><a-button @click="load">Refresh</a-button></template>
    <a-spin :loading="loading" style="width: 100%">
      <a-alert v-if="error" type="error">{{ error }}</a-alert>
      <a-grid v-else :cols="{ xs: 1, sm: 2, md: 4, lg: 4 }" :col-gap="12" :row-gap="12">
        <a-grid-item>
          <div class="metric"><span>API</span><strong>{{ health?.status || 'unknown' }}</strong></div>
        </a-grid-item>
        <a-grid-item>
          <div class="metric"><span>Service</span><strong>{{ health?.service || '-' }}</strong></div>
        </a-grid-item>
        <a-grid-item>
          <div class="metric"><span>Version</span><strong>{{ health?.version || '-' }}</strong></div>
        </a-grid-item>
        <a-grid-item>
          <div class="metric"><span>Relay Policy</span><strong>required</strong></div>
        </a-grid-item>
      </a-grid>
    </a-spin>
  </a-card>
  <a-card class="panel" :bordered="false">
    <template #title>Production Work Queue</template>
    <a-table :columns="columns" :data="rows" :pagination="false" row-key="area" />
  </a-card>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { apiGet, humanError } from '../api/client';

interface Health {
  status: string;
  service: string;
  version: string;
}

const health = ref<Health | null>(null);
const loading = ref(false);
const error = ref('');
const columns = [
  { title: 'Area', dataIndex: 'area' },
  { title: 'Status', dataIndex: 'status' },
  { title: 'Next Step', dataIndex: 'next' }
];
const rows = [
  { area: 'Relay Auth', status: 'API foundation', next: 'Patch hbbr hook' },
  { area: 'Builder', status: 'Schema and jobs', next: 'Configure Windows runner' },
  { area: 'Admin', status: 'Arco skeleton', next: 'Connect CRUD modules' }
];

async function load() {
  loading.value = true;
  error.value = '';
  try {
    health.value = await apiGet<Health>('/api/v1/health');
  } catch (err) {
    error.value = humanError(err);
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>
