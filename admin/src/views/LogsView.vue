<template>
  <a-card class="panel" :bordered="false">
    <template #title>Logs</template>
    <template #extra>
      <a-button @click="loadActive">Refresh</a-button>
    </template>
    <a-spin :loading="loading" style="width: 100%">
      <a-space direction="vertical" fill>
        <a-alert v-if="message" type="warning">{{ message }}</a-alert>
        <a-tabs v-model:active-key="activeKey" @change="loadActive">
          <a-tab-pane v-for="tab in tabs" :key="tab.key" :title="tab.label" />
        </a-tabs>
        <a-form :model="filters" layout="vertical">
          <a-grid :cols="{ xs: 1, sm: 2, md: 4, lg: 4 }" :col-gap="12">
            <a-grid-item v-for="field in activeFilterFields" :key="field.key">
              <a-form-item :label="field.label">
                <a-select v-if="field.options" v-model="filters[field.key]" allow-clear>
                  <a-option v-for="option in field.options" :key="option" :value="option">{{ option }}</a-option>
                </a-select>
                <a-input-number v-else-if="field.type === 'number'" v-model="filters[field.key]" :min="field.min ?? 1" :max="field.max ?? 500" />
                <a-input v-else v-model="filters[field.key]" :placeholder="field.label" />
              </a-form-item>
            </a-grid-item>
          </a-grid>
          <a-space>
            <a-button type="primary" @click="loadActive">Apply</a-button>
            <a-button @click="resetFilters">Reset</a-button>
          </a-space>
        </a-form>
        <a-table
          v-if="displayRows.length"
          :columns="columns"
          :data="displayRows"
          :pagination="{ pageSize: 10 }"
          row-key="_rowKey"
        />
        <a-empty v-else-if="loaded" />
      </a-space>
    </a-spin>
  </a-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { apiGet, humanError } from '../api/client';

type LogRow = Record<string, unknown>;
interface FilterField {
  key: string;
  label: string;
  type?: 'text' | 'number';
  options?: string[];
  min?: number;
  max?: number;
}

const tabs = [
  { key: 'logins', label: 'Login Logs', endpoint: '/api/v1/logs/logins' },
  { key: 'connections', label: 'Connection Logs', endpoint: '/api/v1/logs/connections' },
  { key: 'file-transfers', label: 'File Transfer Logs', endpoint: '/api/v1/logs/file-transfers' },
  { key: 'audit', label: 'Audit Events', endpoint: '/api/v1/logs/audit' }
];

const preferredColumns: Record<string, string[]> = {
  logins: ['id', 'user_id', 'email', 'status', 'failure_reason', 'ip', 'user_agent', 'created_at'],
  connections: ['id', 'controller_user_id', 'controller_device_id', 'target_device_id', 'connection_type', 'status', 'started_at', 'ended_at'],
  'file-transfers': ['id', 'connection_log_id', 'direction', 'filename_hash', 'size_bytes', 'status', 'created_at'],
  audit: ['id', 'actor_user_id', 'actor_type', 'action', 'resource_type', 'resource_id', 'created_at']
};

const filterFields: Record<string, FilterField[]> = {
  logins: [
    { key: 'email', label: 'Email' },
    { key: 'limit', label: 'Limit', type: 'number', min: 1, max: 500 },
    { key: 'offset', label: 'Offset', type: 'number', min: 0, max: 100000 },
    { key: 'from', label: 'From RFC3339' },
    { key: 'to', label: 'To RFC3339' }
  ],
  connections: [
    { key: 'status', label: 'Status', options: ['started', 'ended', 'failed', 'denied'] },
    { key: 'connection_type', label: 'Connection Type', options: ['direct', 'relay', 'websocket'] },
    { key: 'limit', label: 'Limit', type: 'number', min: 1, max: 500 },
    { key: 'offset', label: 'Offset', type: 'number', min: 0, max: 100000 },
    { key: 'from', label: 'From RFC3339' },
    { key: 'to', label: 'To RFC3339' }
  ],
  'file-transfers': [
    { key: 'direction', label: 'Direction', options: ['upload', 'download'] },
    { key: 'status', label: 'Status' },
    { key: 'limit', label: 'Limit', type: 'number', min: 1, max: 500 },
    { key: 'offset', label: 'Offset', type: 'number', min: 0, max: 100000 },
    { key: 'from', label: 'From RFC3339' },
    { key: 'to', label: 'To RFC3339' }
  ],
  audit: [
    { key: 'actor_type', label: 'Actor Type', options: ['user', 'system', 'api_token'] },
    { key: 'action', label: 'Action' },
    { key: 'resource_type', label: 'Resource Type' },
    { key: 'resource_id', label: 'Resource ID' },
    { key: 'limit', label: 'Limit', type: 'number', min: 1, max: 500 },
    { key: 'offset', label: 'Offset', type: 'number', min: 0, max: 100000 },
    { key: 'from', label: 'From RFC3339' },
    { key: 'to', label: 'To RFC3339' }
  ]
};

const activeKey = ref('logins');
const rows = ref<LogRow[]>([]);
const loading = ref(false);
const loaded = ref(false);
const message = ref('');
const filters = ref<Record<string, any>>({ limit: 500 });

const activeTab = computed(() => tabs.find((tab) => tab.key === activeKey.value) || tabs[0]);
const activeFilterFields = computed(() => filterFields[activeKey.value] || []);
const displayRows = computed(() => rows.value.map((row, index) => ({
  _rowKey: String(row.id || row.user_id || index),
  ...formatRow(row)
})));
const columns = computed(() => {
  const keys = preferredColumns[activeKey.value] || Object.keys(rows.value[0] || {});
  return keys.map((key) => ({ title: key, dataIndex: key }));
});

async function loadActive() {
  loading.value = true;
  loaded.value = false;
  message.value = '';
  try {
    rows.value = await apiGet<LogRow[]>(filteredEndpoint());
  } catch (err) {
    rows.value = [];
    message.value = humanError(err);
  } finally {
    loaded.value = true;
    loading.value = false;
  }
}

function filteredEndpoint(): string {
  const params = new URLSearchParams();
  for (const field of activeFilterFields.value) {
    const value = filters.value[field.key];
    if (value === undefined || value === null || value === '') continue;
    params.set(field.key, String(value));
  }
  const query = params.toString();
  return query ? `${activeTab.value.endpoint}?${query}` : activeTab.value.endpoint;
}

function resetFilters() {
  filters.value = { limit: 500 };
  loadActive();
}

function formatRow(row: LogRow): LogRow {
  const out: LogRow = {};
  for (const [key, value] of Object.entries(row)) {
    out[key] = formatValue(value);
  }
  return out;
}

function formatValue(value: unknown): unknown {
  if (Array.isArray(value) || (value && typeof value === 'object')) {
    return JSON.stringify(value);
  }
  return value;
}

onMounted(loadActive);
</script>
