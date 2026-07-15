<template>
  <a-card class="panel" :bordered="false">
    <template #title>Settings</template>
    <template #extra>
      <a-space>
        <a-button @click="load">Refresh</a-button>
        <a-button v-if="activeSection !== 'tokens'" type="primary" :loading="saving" @click="save">Save</a-button>
      </a-space>
    </template>
    <a-spin :loading="loading" style="width: 100%">
      <a-space direction="vertical" fill>
        <a-alert v-if="message" :type="messageType">{{ message }}</a-alert>
        <a-tabs v-model:active-key="activeSection" @change="load">
          <a-tab-pane v-for="section in sections" :key="section.key" :title="section.label" />
        </a-tabs>
        <template v-if="activeSection === 'tokens'">
          <a-form :model="tokenForm" layout="inline">
            <a-form-item label="Name">
              <a-input v-model="tokenForm.name" placeholder="Automation token" />
            </a-form-item>
            <a-form-item label="Scopes">
              <a-input v-model="tokenForm.scopes" placeholder="relay:grant,build:read" />
            </a-form-item>
            <a-form-item>
              <a-button type="primary" :loading="creatingToken" @click="createToken">Create</a-button>
            </a-form-item>
          </a-form>
          <a-alert v-if="newToken" type="success">
            New token: {{ newToken }}
          </a-alert>
          <a-table
            :columns="tokenColumns"
            :data="tokenRows"
            :pagination="{ pageSize: 10 }"
            row-key="id"
          >
            <template #scopes="{ record }">
              {{ record.scopes.join(', ') }}
            </template>
            <template #actions="{ record }">
              <a-button
                size="mini"
                status="danger"
                :disabled="Boolean(record.revoked_at)"
                :loading="tokenActionId === record.id"
                @click="revokeToken(record)"
              >
                Revoke
              </a-button>
            </template>
          </a-table>
        </template>
        <a-form v-else :model="form" layout="vertical">
          <a-grid :cols="{ xs: 1, sm: 2, md: 3, lg: 3 }" :col-gap="14" :row-gap="10">
            <a-grid-item v-for="setting in editableSettings" :key="setting.key">
              <a-form-item :label="setting.key">
                <a-switch v-if="typeof form[setting.key] === 'boolean'" v-model="form[setting.key]" />
                <a-input-number v-else-if="typeof form[setting.key] === 'number'" v-model="form[setting.key]" :min="0" />
                <a-textarea
                  v-else
                  v-model="form[setting.key]"
                  :auto-size="{ minRows: 2, maxRows: 5 }"
                />
              </a-form-item>
            </a-grid-item>
          </a-grid>
        </a-form>
        <a-table
          v-if="activeSection !== 'tokens' && rows.length"
          :columns="columns"
          :data="displayRows"
          :pagination="{ pageSize: 10 }"
          row-key="key"
        />
      </a-space>
    </a-spin>
  </a-card>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { apiGet, apiPost, apiPut, humanError } from '../api/client';

type SettingValue = string | number | boolean | null | Record<string, unknown> | unknown[];

interface SettingRow {
  key: string;
  value: SettingValue;
  value_json: string;
  source: string;
  updated_by?: number;
  updated_at?: string;
}

interface ApiTokenRow {
  id: number;
  name: string;
  scopes: string[];
  user_id?: number;
  expires_at?: string;
  last_used_at?: string;
  created_at: string;
  revoked_at?: string;
}

interface CreateApiTokenResponse extends ApiTokenRow {
  token: string;
}

const rows = ref<SettingRow[]>([]);
const tokenRows = ref<ApiTokenRow[]>([]);
const loading = ref(false);
const saving = ref(false);
const creatingToken = ref(false);
const tokenActionId = ref<number | null>(null);
const message = ref('');
const messageType = ref<'success' | 'warning'>('success');
const form = reactive<Record<string, any>>({});
const tokenForm = reactive({ name: '', scopes: 'relay:grant' });
const newToken = ref('');
const activeSection = ref('general');

const sections = [
  { key: 'general', label: 'General', endpoint: '/api/v1/settings' },
  { key: 'oidc', label: 'OIDC', endpoint: '/api/v1/settings/oidc' },
  { key: 'ldap', label: 'LDAP', endpoint: '/api/v1/settings/ldap' },
  { key: 'smtp', label: 'SMTP', endpoint: '/api/v1/settings/smtp' },
  { key: 'tokens', label: 'Tokens', endpoint: '/api/v1/api-tokens' }
];

const columns = [
  { title: 'key', dataIndex: 'key' },
  { title: 'value', dataIndex: 'value' },
  { title: 'source', dataIndex: 'source' },
  { title: 'updated_by', dataIndex: 'updated_by' },
  { title: 'updated_at', dataIndex: 'updated_at' }
];

const tokenColumns = [
  { title: 'ID', dataIndex: 'id' },
  { title: 'Name', dataIndex: 'name' },
  { title: 'Scopes', slotName: 'scopes' },
  { title: 'User', dataIndex: 'user_id' },
  { title: 'Last Used', dataIndex: 'last_used_at' },
  { title: 'Created', dataIndex: 'created_at' },
  { title: 'Revoked', dataIndex: 'revoked_at' },
  { title: 'Actions', slotName: 'actions' }
];

const editableSettings = computed(() => rows.value);
const displayRows = computed(() => rows.value.map((row) => ({
  ...row,
  value: formatValue(row.value)
})));

async function load() {
  loading.value = true;
  message.value = '';
  try {
    if (activeSection.value === 'tokens') {
      tokenRows.value = await apiGet<ApiTokenRow[]>('/api/v1/api-tokens');
    } else {
      rows.value = await apiGet<SettingRow[]>(activeEndpoint());
      resetForm(rows.value);
    }
  } catch (err) {
    messageType.value = 'warning';
    message.value = humanError(err);
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (activeSection.value === 'tokens') return;
  saving.value = true;
  message.value = '';
  try {
    rows.value = await apiPut<SettingRow[]>(activeEndpoint(), { settings: serializeForm() });
    resetForm(rows.value);
    messageType.value = 'success';
    message.value = 'Saved';
  } catch (err) {
    messageType.value = 'warning';
    message.value = humanError(err);
  } finally {
    saving.value = false;
  }
}

async function createToken() {
  creatingToken.value = true;
  message.value = '';
  newToken.value = '';
  try {
    const created = await apiPost<CreateApiTokenResponse>('/api/v1/api-tokens', {
      name: tokenForm.name,
      scopes: tokenForm.scopes.split(',').map((scope) => scope.trim()).filter(Boolean)
    });
    newToken.value = created.token;
    tokenForm.name = '';
    tokenRows.value = await apiGet<ApiTokenRow[]>('/api/v1/api-tokens');
    messageType.value = 'success';
    message.value = 'Token created';
  } catch (err) {
    messageType.value = 'warning';
    message.value = humanError(err);
  } finally {
    creatingToken.value = false;
  }
}

async function revokeToken(row: ApiTokenRow) {
  tokenActionId.value = row.id;
  message.value = '';
  try {
    await apiPost<ApiTokenRow>(`/api/v1/api-tokens/${row.id}/revoke`, {});
    tokenRows.value = await apiGet<ApiTokenRow[]>('/api/v1/api-tokens');
    messageType.value = 'success';
    message.value = 'Token revoked';
  } catch (err) {
    messageType.value = 'warning';
    message.value = humanError(err);
  } finally {
    tokenActionId.value = null;
  }
}

function activeEndpoint(): string {
  return sections.find((section) => section.key === activeSection.value)?.endpoint || '/api/v1/settings';
}

function resetForm(settings: SettingRow[]) {
  for (const key of Object.keys(form)) {
    delete form[key];
  }
  for (const setting of settings) {
    form[setting.key] = setting.value;
  }
}

function serializeForm(): Record<string, SettingValue> {
  const out: Record<string, SettingValue> = {};
  for (const setting of rows.value) {
    const value = form[setting.key];
    out[setting.key] = coerceValue(setting.value, value);
  }
  return out;
}

function coerceValue(original: SettingValue, value: unknown): SettingValue {
  if (typeof original === 'number') {
    return Number(value);
  }
  if (typeof original === 'boolean') {
    return Boolean(value);
  }
  if (typeof value === 'string' && value.trim().startsWith('{')) {
    return JSON.parse(value) as Record<string, unknown>;
  }
  if (typeof value === 'string' && value.trim().startsWith('[')) {
    return JSON.parse(value) as unknown[];
  }
  return value as SettingValue;
}

function formatValue(value: SettingValue): string {
  if (Array.isArray(value) || (value && typeof value === 'object')) {
    return JSON.stringify(value);
  }
  if (value === null) return 'null';
  return String(value);
}

onMounted(load);
</script>
