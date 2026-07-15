<template>
  <a-card class="panel" :bordered="false">
    <template #title>{{ title }}</template>
    <template #extra>
      <a-button :disabled="!endpoint" @click="load">Check API</a-button>
    </template>
    <a-space direction="vertical" fill>
      <a-typography-text type="secondary">{{ endpoint ? endpoint : 'Planned module' }}</a-typography-text>
      <a-spin :loading="loading" style="width: 100%">
        <a-alert v-if="message" :type="message.includes('awaits implementation') ? 'warning' : 'info'">{{ message }}</a-alert>
        <a-space v-if="canCreate" direction="vertical" fill>
          <a-divider />
          <a-form :model="form" layout="vertical">
            <a-grid :cols="{ xs: 1, sm: 2, md: 3, lg: 3 }" :col-gap="12">
              <a-grid-item v-for="field in fields" :key="field.key">
                <a-form-item :label="field.label">
                  <a-select v-if="field.type === 'select'" v-model="form[field.key]" :placeholder="field.label">
                    <a-option v-for="option in field.options" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </a-option>
                  </a-select>
                  <a-input-number v-else-if="field.type === 'number'" v-model="form[field.key]" :min="0" />
                  <a-switch v-else-if="field.type === 'switch'" v-model="form[field.key]" />
                  <a-textarea
                    v-else-if="field.type === 'textarea'"
                    v-model="form[field.key]"
                    :placeholder="field.label"
                    :auto-size="{ minRows: 3, maxRows: 8 }"
                  />
                  <a-input v-else v-model="form[field.key]" :placeholder="field.label" />
                </a-form-item>
              </a-grid-item>
            </a-grid>
            <a-button type="primary" @click="createItem">Create</a-button>
          </a-form>
        </a-space>
        <a-space v-if="props.endpoint === '/api/v1/users'" direction="vertical" fill>
          <a-divider>Update User</a-divider>
          <a-form :model="userUpdateForm" layout="vertical">
            <a-grid :cols="{ xs: 1, sm: 2, md: 4, lg: 4 }" :col-gap="12">
              <a-grid-item>
                <a-form-item label="User ID">
                  <a-input-number v-model="userUpdateForm.id" :min="1" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Email">
                  <a-input v-model="userUpdateForm.email" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Username">
                  <a-input v-model="userUpdateForm.username" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Display Name">
                  <a-input v-model="userUpdateForm.display_name" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Status">
                  <a-select v-model="userUpdateForm.status">
                    <a-option v-for="option in userStatusOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </a-option>
                  </a-select>
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Source">
                  <a-select v-model="userUpdateForm.source">
                    <a-option v-for="option in userSourceOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </a-option>
                  </a-select>
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="MFA Enabled">
                  <a-switch v-model="userUpdateForm.mfa_enabled" />
                </a-form-item>
              </a-grid-item>
            </a-grid>
            <a-button type="primary" :loading="actionId === Number(userUpdateForm.id)" @click="updateUser">
              Update User
            </a-button>
          </a-form>
        </a-space>
        <a-space v-if="props.endpoint === '/api/v1/devices'" direction="vertical" fill>
          <a-divider>Update Device</a-divider>
          <a-form :model="deviceUpdateForm" layout="vertical">
            <a-grid :cols="{ xs: 1, sm: 2, md: 4, lg: 4 }" :col-gap="12">
              <a-grid-item>
                <a-form-item label="Device ID">
                  <a-input-number v-model="deviceUpdateForm.id" :min="1" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="RustDesk ID">
                  <a-input v-model="deviceUpdateForm.rustdesk_id" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Name">
                  <a-input v-model="deviceUpdateForm.name" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Alias">
                  <a-input v-model="deviceUpdateForm.alias" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Status">
                  <a-select v-model="deviceUpdateForm.status">
                    <a-option v-for="option in deviceStatusOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </a-option>
                  </a-select>
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Platform">
                  <a-input v-model="deviceUpdateForm.platform" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Client Version">
                  <a-input v-model="deviceUpdateForm.client_version" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="OpenDesk Version">
                  <a-input v-model="deviceUpdateForm.opendesk_client_version" />
                </a-form-item>
              </a-grid-item>
            </a-grid>
            <a-button type="primary" :loading="actionId === Number(deviceUpdateForm.id)" @click="updateDevice">
              Update Device
            </a-button>
          </a-form>
        </a-space>
        <a-space v-if="props.endpoint === '/api/v1/relays'" direction="vertical" fill>
          <a-divider>Update Relay</a-divider>
          <a-form :model="relayUpdateForm" layout="vertical">
            <a-grid :cols="{ xs: 1, sm: 2, md: 4, lg: 4 }" :col-gap="12">
              <a-grid-item>
                <a-form-item label="Relay ID">
                  <a-input-number v-model="relayUpdateForm.id" :min="1" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Name">
                  <a-input v-model="relayUpdateForm.name" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Region">
                  <a-input v-model="relayUpdateForm.region" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Host">
                  <a-input v-model="relayUpdateForm.host" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Port">
                  <a-input-number v-model="relayUpdateForm.port" :min="0" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="WS Port">
                  <a-input-number v-model="relayUpdateForm.ws_port" :min="0" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Status">
                  <a-select v-model="relayUpdateForm.status">
                    <a-option v-for="option in relayStatusOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </a-option>
                  </a-select>
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Key Fingerprint">
                  <a-input v-model="relayUpdateForm.public_key_fingerprint" />
                </a-form-item>
              </a-grid-item>
            </a-grid>
            <a-button type="primary" :loading="actionId === Number(relayUpdateForm.id)" @click="updateRelay">
              Update Relay
            </a-button>
          </a-form>
        </a-space>
        <a-space v-if="props.endpoint === '/api/v1/access-rules'" direction="vertical" fill>
          <a-divider>Update Access Rule</a-divider>
          <a-form :model="accessRuleUpdateForm" layout="vertical">
            <a-grid :cols="{ xs: 1, sm: 2, md: 4, lg: 4 }" :col-gap="12">
              <a-grid-item>
                <a-form-item label="Rule ID">
                  <a-input-number v-model="accessRuleUpdateForm.id" :min="1" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Subject Type">
                  <a-select v-model="accessRuleUpdateForm.subject_type">
                    <a-option v-for="option in accessRuleSubjectOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </a-option>
                  </a-select>
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Subject ID">
                  <a-input-number v-model="accessRuleUpdateForm.subject_id" :min="1" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Target Type">
                  <a-select v-model="accessRuleUpdateForm.target_type">
                    <a-option v-for="option in accessRuleTargetOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </a-option>
                  </a-select>
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Target ID">
                  <a-input-number v-model="accessRuleUpdateForm.target_id" :min="1" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Effect">
                  <a-select v-model="accessRuleUpdateForm.effect">
                    <a-option v-for="option in accessRuleEffectOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </a-option>
                  </a-select>
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Priority">
                  <a-input-number v-model="accessRuleUpdateForm.priority" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Enabled">
                  <a-switch v-model="accessRuleUpdateForm.enabled" />
                </a-form-item>
              </a-grid-item>
            </a-grid>
            <a-button type="primary" :loading="actionId === Number(accessRuleUpdateForm.id)" @click="updateAccessRule">
              Update Access Rule
            </a-button>
          </a-form>
        </a-space>
        <a-space v-if="props.endpoint === '/api/v1/control-roles'" direction="vertical" fill>
          <a-divider>Update Control Role</a-divider>
          <a-form :model="controlRoleUpdateForm" layout="vertical">
            <a-grid :cols="{ xs: 1, sm: 2, md: 4, lg: 4 }" :col-gap="12">
              <a-grid-item>
                <a-form-item label="Role ID">
                  <a-input-number v-model="controlRoleUpdateForm.id" :min="1" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Role Name">
                  <a-input v-model="controlRoleUpdateForm.name" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Description">
                  <a-input v-model="controlRoleUpdateForm.description" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Enabled">
                  <a-switch v-model="controlRoleUpdateForm.enabled" />
                </a-form-item>
              </a-grid-item>
            </a-grid>
            <a-form-item label="Permissions JSON">
              <a-textarea
                v-model="controlRoleUpdateForm.permissions_json"
                :auto-size="{ minRows: 3, maxRows: 8 }"
              />
            </a-form-item>
            <a-button type="primary" :loading="actionId === Number(controlRoleUpdateForm.id)" @click="updateControlRole">
              Update Control Role
            </a-button>
          </a-form>
        </a-space>
        <a-space v-if="props.endpoint === '/api/v1/strategies'" direction="vertical" fill>
          <a-divider>Update Strategy</a-divider>
          <a-form :model="strategyUpdateForm" layout="vertical">
            <a-grid :cols="{ xs: 1, sm: 2, md: 4, lg: 4 }" :col-gap="12">
              <a-grid-item>
                <a-form-item label="Strategy ID">
                  <a-input-number v-model="strategyUpdateForm.id" :min="1" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Strategy Name">
                  <a-input v-model="strategyUpdateForm.name" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Description">
                  <a-input v-model="strategyUpdateForm.description" />
                </a-form-item>
              </a-grid-item>
              <a-grid-item>
                <a-form-item label="Enabled">
                  <a-switch v-model="strategyUpdateForm.enabled" />
                </a-form-item>
              </a-grid-item>
            </a-grid>
            <a-form-item label="Settings JSON">
              <a-textarea
                v-model="strategyUpdateForm.settings_json"
                :auto-size="{ minRows: 3, maxRows: 8 }"
              />
            </a-form-item>
            <a-form-item label="Assignments JSON">
              <a-textarea
                v-model="strategyUpdateForm.assignments_json"
                :auto-size="{ minRows: 3, maxRows: 8 }"
              />
            </a-form-item>
            <a-button type="primary" :loading="actionId === Number(strategyUpdateForm.id)" @click="updateStrategy">
              Update Strategy
            </a-button>
          </a-form>
        </a-space>
        <a-space v-if="canManageRelations" direction="vertical" fill>
          <a-divider>{{ relationTitle }}</a-divider>
          <a-form :model="relationForm" layout="vertical">
            <a-grid :cols="{ xs: 1, sm: 2, md: 4, lg: 4 }" :col-gap="12">
              <a-grid-item v-for="field in relationFields" :key="field.key">
                <a-form-item :label="field.label">
                  <a-input v-if="field.type === 'number'" v-model="relationForm[field.key]" :placeholder="field.label" />
                  <a-input v-else v-model="relationForm[field.key]" :placeholder="field.label" />
                </a-form-item>
              </a-grid-item>
            </a-grid>
            <a-space>
              <a-button type="primary" @click="addRelation">{{ addRelationLabel }}</a-button>
              <a-button status="danger" @click="removeRelation">{{ removeRelationLabel }}</a-button>
            </a-space>
          </a-form>
        </a-space>
        <a-table
          v-if="rows.length"
          :columns="columns"
          :data="displayRows"
          :pagination="{ pageSize: 8 }"
          row-key="id"
        >
          <template #actions="{ record }">
            <a-space v-if="props.endpoint === '/api/v1/users'">
              <a-button size="mini" @click="fillUserUpdate(record)">Load</a-button>
              <a-button
                size="mini"
                status="danger"
                :disabled="record.status === 'disabled'"
                :loading="actionId === record.id"
                @click="disableUser(record)"
              >
                Disable
              </a-button>
            </a-space>
            <a-space v-if="props.endpoint === '/api/v1/devices'">
              <a-button size="mini" @click="fillDeviceUpdate(record)">Load</a-button>
              <a-button
                size="mini"
                status="danger"
                :disabled="record.status === 'disabled'"
                :loading="actionId === record.id"
                @click="disableDevice(record)"
              >
                Disable
              </a-button>
            </a-space>
            <a-space v-if="props.endpoint === '/api/v1/relays'">
              <a-button size="mini" @click="fillRelayUpdate(record)">Load</a-button>
              <a-button
                size="mini"
                status="danger"
                :disabled="record.status === 'disabled'"
                :loading="actionId === record.id"
                @click="disableRelay(record)"
              >
                Disable
              </a-button>
            </a-space>
            <a-space v-if="props.endpoint === '/api/v1/access-rules'">
              <a-button size="mini" @click="fillAccessRuleUpdate(record)">Load</a-button>
              <a-button
                size="mini"
                status="danger"
                :loading="actionId === record.id"
                @click="deleteAccessRule(record)"
              >
                Delete
              </a-button>
            </a-space>
            <a-space v-if="props.endpoint === '/api/v1/control-roles'">
              <a-button size="mini" @click="fillControlRoleUpdate(record)">Load</a-button>
              <a-button
                size="mini"
                status="danger"
                :loading="actionId === record.id"
                @click="deleteControlRole(record)"
              >
                Delete
              </a-button>
            </a-space>
            <a-space v-if="props.endpoint === '/api/v1/strategies'">
              <a-button size="mini" @click="fillStrategyUpdate(record)">Load</a-button>
              <a-button
                size="mini"
                status="danger"
                :loading="actionId === record.id"
                @click="deleteStrategy(record)"
              >
                Delete
              </a-button>
            </a-space>
          </template>
        </a-table>
        <a-empty v-else-if="loaded && !message" />
        <a-textarea v-if="payload" :model-value="payload" readonly :auto-size="{ minRows: 6, maxRows: 18 }" />
      </a-spin>
    </a-space>
  </a-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { apiDelete, apiGet, apiPost, apiPut, humanError } from '../api/client';

const props = defineProps<{ title: string; endpoint: string }>();
type FieldValue = string | number | boolean;
interface FieldConfig {
  key: string;
  label: string;
  type?: 'text' | 'number' | 'select' | 'textarea' | 'switch';
  options?: { label: string; value: string }[];
  defaultValue?: FieldValue;
}

interface TableColumn {
  title: string;
  dataIndex?: string;
  slotName?: string;
}

const loading = ref(false);
const message = ref('API skeleton registered. Implementation will be added by module PRs.');
const payload = ref('');
const rows = ref<Record<string, unknown>[]>([]);
const loaded = ref(false);
const actionId = ref<number | null>(null);
const form = ref<Record<string, any>>({});
const relationForm = ref<Record<string, any>>({});
const userUpdateForm = ref<Record<string, any>>({
  id: 1,
  email: '',
  username: '',
  display_name: '',
  status: 'active',
  source: 'local',
  mfa_enabled: false
});
const deviceUpdateForm = ref<Record<string, any>>({
  id: 1,
  rustdesk_id: '',
  name: '',
  alias: '',
  status: 'offline',
  platform: '',
  client_version: '',
  opendesk_client_version: ''
});
const relayUpdateForm = ref<Record<string, any>>({
  id: 1,
  name: '',
  region: '',
  host: '',
  port: 21117,
  ws_port: 21119,
  status: 'active',
  public_key_fingerprint: ''
});
const accessRuleUpdateForm = ref<Record<string, any>>({
  id: 1,
  subject_type: 'user',
  subject_id: 1,
  target_type: 'device',
  target_id: 1,
  effect: 'allow',
  priority: 100,
  enabled: true
});
const controlRoleUpdateForm = ref<Record<string, any>>({
  id: 1,
  name: '',
  description: '',
  enabled: true,
  permissions_json: '[]'
});
const strategyUpdateForm = ref<Record<string, any>>({
  id: 1,
  name: '',
  description: '',
  enabled: true,
  settings_json: '{}',
  assignments_json: '[]'
});
const userStatusOptions = enumOptions(['active', 'disabled', 'locked']);
const userSourceOptions = enumOptions(['local', 'oidc', 'ldap']);
const deviceStatusOptions = enumOptions(['online', 'offline', 'disabled']);
const relayStatusOptions = enumOptions(['active', 'degraded', 'offline', 'disabled']);
const accessRuleSubjectOptions = enumOptions(['user', 'user_group']);
const accessRuleTargetOptions = enumOptions(['device', 'device_group']);
const accessRuleEffectOptions = enumOptions(['allow', 'deny']);

const resourceFields: Record<string, FieldConfig[]> = {
  '/api/v1/users': [
    { key: 'email', label: 'Email' },
    { key: 'username', label: 'Username' },
    { key: 'display_name', label: 'Display Name' }
  ],
  '/api/v1/user-groups': [
    { key: 'name', label: 'Group Name' },
    { key: 'description', label: 'Description' },
    {
      key: 'member_user_ids_json',
      label: 'Member User IDs JSON',
      type: 'textarea',
      defaultValue: '[1]'
    }
  ],
  '/api/v1/devices': [
    { key: 'rustdesk_id', label: 'RustDesk ID' },
    { key: 'name', label: 'Device Name' },
    { key: 'platform', label: 'Platform' }
  ],
  '/api/v1/device-groups': [
    { key: 'name', label: 'Group Name' },
    { key: 'description', label: 'Description' },
    {
      key: 'member_device_ids_json',
      label: 'Member Device IDs JSON',
      type: 'textarea',
      defaultValue: '[1]'
    }
  ],
  '/api/v1/address-books': [
    { key: 'name', label: 'Address Book Name' },
    { key: 'description', label: 'Description' },
    { key: 'owner_user_id', label: 'Owner User ID', type: 'number', defaultValue: 1 },
    {
      key: 'entries_json',
      label: 'Entries JSON',
      type: 'textarea',
      defaultValue: '[{"device_id":1,"alias":"Demo Windows Workstation"}]'
    }
  ],
  '/api/v1/relays': [
    { key: 'name', label: 'Relay Name' },
    { key: 'region', label: 'Region' },
    { key: 'host', label: 'Host' }
  ],
  '/api/v1/access-rules': [
    { key: 'subject_type', label: 'Subject Type', type: 'select', options: enumOptions(['user', 'user_group']), defaultValue: 'user' },
    { key: 'subject_id', label: 'Subject ID', type: 'number', defaultValue: 1 },
    { key: 'target_type', label: 'Target Type', type: 'select', options: enumOptions(['device', 'device_group']), defaultValue: 'device' },
    { key: 'target_id', label: 'Target ID', type: 'number', defaultValue: 1 },
    { key: 'effect', label: 'Effect', type: 'select', options: enumOptions(['allow', 'deny']), defaultValue: 'allow' },
    { key: 'priority', label: 'Priority', type: 'number', defaultValue: 100 },
    { key: 'enabled', label: 'Enabled', type: 'switch', defaultValue: true }
  ],
  '/api/v1/control-roles': [
    { key: 'name', label: 'Role Name' },
    { key: 'description', label: 'Description' },
    { key: 'enabled', label: 'Enabled', type: 'switch', defaultValue: true },
    {
      key: 'permissions_json',
      label: 'Permissions JSON',
      type: 'textarea',
      defaultValue: '[{"permission_key":"file_transfer","mode":"disable"},{"permission_key":"terminal","mode":"disable"}]'
    }
  ],
  '/api/v1/strategies': [
    { key: 'name', label: 'Strategy Name' },
    { key: 'description', label: 'Description' },
    { key: 'enabled', label: 'Enabled', type: 'switch', defaultValue: true },
    {
      key: 'settings_json',
      label: 'Settings JSON',
      type: 'textarea',
      defaultValue: '{"verification-method":"use-both-passwords","allow-remote-config-modification":"N"}'
    },
    {
      key: 'assignments_json',
      label: 'Assignments JSON',
      type: 'textarea',
      defaultValue: '[{"target_type":"device","target_id":1}]'
    }
  ]
};

const preferredColumns: Record<string, string[]> = {
  '/api/v1/users': ['id', 'email', 'username', 'display_name', 'status', 'source', 'mfa_enabled', 'created_at'],
  '/api/v1/user-groups': ['id', 'name', 'description', 'member_user_ids', 'created_at'],
  '/api/v1/devices': ['id', 'rustdesk_id', 'name', 'status', 'platform', 'opendesk_client_version', 'last_seen_at', 'updated_at'],
  '/api/v1/device-groups': ['id', 'name', 'description', 'member_device_ids', 'created_at'],
  '/api/v1/address-books': ['id', 'name', 'description', 'owner_user_id', 'entries', 'created_at'],
  '/api/v1/relays': ['id', 'name', 'region', 'host', 'status', 'current_sessions', 'last_health_at', 'updated_at'],
  '/api/v1/access-rules': ['id', 'subject_type', 'subject_id', 'target_type', 'target_id', 'effect', 'priority', 'enabled'],
  '/api/v1/control-roles': ['id', 'name', 'description', 'enabled', 'permissions', 'created_at'],
  '/api/v1/strategies': ['id', 'name', 'description', 'enabled', 'settings_json', 'assignments', 'created_at']
};

const relationFieldConfigs: Record<string, FieldConfig[]> = {
  '/api/v1/user-groups': [
    { key: 'group_id', label: 'Group ID', type: 'number', defaultValue: 1 },
    { key: 'user_id', label: 'User ID', type: 'number', defaultValue: 1 }
  ],
  '/api/v1/device-groups': [
    { key: 'group_id', label: 'Group ID', type: 'number', defaultValue: 1 },
    { key: 'device_id', label: 'Device ID', type: 'number', defaultValue: 1 }
  ],
  '/api/v1/address-books': [
    { key: 'book_id', label: 'Address Book ID', type: 'number', defaultValue: 1 },
    { key: 'device_id', label: 'Device ID', type: 'number', defaultValue: 1 },
    { key: 'alias', label: 'Alias', defaultValue: 'Demo Windows Workstation' },
    { key: 'entry_id', label: 'Entry ID', type: 'number', defaultValue: 1 }
  ]
};

const fields = computed(() => resourceFields[props.endpoint] || []);
const relationFields = computed(() => relationFieldConfigs[props.endpoint] || []);
const canCreate = computed(() => fields.value.length > 0);
const canManageRelations = computed(() => relationFields.value.length > 0);
const relationTitle = computed(() => (props.endpoint === '/api/v1/address-books' ? 'Address Book Entries' : 'Group Members'));
const addRelationLabel = computed(() => (props.endpoint === '/api/v1/address-books' ? 'Add Entry' : 'Add Member'));
const removeRelationLabel = computed(() => (props.endpoint === '/api/v1/address-books' ? 'Remove Entry' : 'Remove Member'));
const displayRows = computed(() => rows.value.map((row) => {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(row)) {
    out[key] = formatCell(value);
  }
  return out;
}));
const columns = computed(() => {
  const first = rows.value[0] || {};
  const keys = preferredColumns[props.endpoint] || Object.keys(first)
    .filter((key) => !key.endsWith('_hash'))
    .slice(0, 8);
  const out: TableColumn[] = keys.map((key) => ({ title: key, dataIndex: key }));
  if (props.endpoint === '/api/v1/users' || props.endpoint === '/api/v1/devices' || props.endpoint === '/api/v1/relays' || props.endpoint === '/api/v1/access-rules' || props.endpoint === '/api/v1/control-roles' || props.endpoint === '/api/v1/strategies') {
    out.push({ title: 'Actions', slotName: 'actions' });
  }
  return out;
});

async function load() {
  if (!props.endpoint) return;
  loading.value = true;
  message.value = '';
  payload.value = '';
  loaded.value = false;
  try {
    const data = await apiGet<unknown>(props.endpoint);
    if (Array.isArray(data)) {
      rows.value = data as Record<string, unknown>[];
    } else {
      payload.value = JSON.stringify(data, null, 2);
    }
  } catch (err) {
    message.value = humanError(err);
  } finally {
    loaded.value = true;
    loading.value = false;
  }
}

async function createItem() {
  loading.value = true;
  message.value = '';
  try {
    await apiPost(props.endpoint, preparePayload());
    resetForm();
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    loading.value = false;
  }
}

async function addRelation() {
  message.value = '';
  const target = relationTarget();
  if (!target) return;
  loading.value = true;
  try {
    if (props.endpoint === '/api/v1/user-groups') {
      const userID = positiveRelationNumber('user_id', 'user_id must be positive');
      if (!userID) return;
      await apiPost(`${props.endpoint}/${target}/members`, { user_id: userID });
      message.value = 'Member added.';
    }
    if (props.endpoint === '/api/v1/device-groups') {
      const deviceID = positiveRelationNumber('device_id', 'device_id must be positive');
      if (!deviceID) return;
      await apiPost(`${props.endpoint}/${target}/members`, { device_id: deviceID });
      message.value = 'Member added.';
    }
    if (props.endpoint === '/api/v1/address-books') {
      const deviceID = positiveRelationNumber('device_id', 'device_id must be positive');
      if (!deviceID) return;
      await apiPost(`${props.endpoint}/${target}/entries`, {
        device_id: deviceID,
        alias: String(relationForm.value.alias || '').trim()
      });
      message.value = 'Entry added.';
    }
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    loading.value = false;
  }
}

async function removeRelation() {
	message.value = '';
	const target = relationTarget();
  if (!target) return;
  loading.value = true;
  try {
    if (props.endpoint === '/api/v1/user-groups') {
      const userID = positiveRelationNumber('user_id', 'user_id must be positive');
      if (!userID) return;
      await apiDelete(`${props.endpoint}/${target}/members/${userID}`);
      message.value = 'Member removed.';
    }
    if (props.endpoint === '/api/v1/device-groups') {
      const deviceID = positiveRelationNumber('device_id', 'device_id must be positive');
      if (!deviceID) return;
      await apiDelete(`${props.endpoint}/${target}/members/${deviceID}`);
      message.value = 'Member removed.';
    }
    if (props.endpoint === '/api/v1/address-books') {
      const entryID = positiveRelationNumber('entry_id', 'entry_id must be positive');
      if (!entryID) return;
      await apiDelete(`${props.endpoint}/${target}/entries/${entryID}`);
      message.value = 'Entry removed.';
    }
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    loading.value = false;
	}
}

async function disableUser(record: Record<string, unknown>) {
  const id = Number(record.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'user id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiDelete(`/api/v1/users/${id}`);
    message.value = 'User disabled.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
  }
}

async function updateUser() {
  const id = Number(userUpdateForm.value.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'user id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiPut(`/api/v1/users/${id}`, {
      email: String(userUpdateForm.value.email || '').trim(),
      username: String(userUpdateForm.value.username || '').trim(),
      display_name: String(userUpdateForm.value.display_name || '').trim(),
      status: String(userUpdateForm.value.status || '').trim(),
      source: String(userUpdateForm.value.source || '').trim(),
      mfa_enabled: Boolean(userUpdateForm.value.mfa_enabled)
    });
    message.value = 'User updated.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
  }
}

function fillUserUpdate(record: Record<string, unknown>) {
  userUpdateForm.value = {
    id: Number(record.id || 1),
    email: String(record.email || ''),
    username: String(record.username || ''),
    display_name: String(record.display_name || ''),
    status: String(record.status || 'active'),
    source: String(record.source || 'local'),
    mfa_enabled: Boolean(record.mfa_enabled)
  };
}

async function disableDevice(record: Record<string, unknown>) {
  const id = Number(record.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'device id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiPost(`/api/v1/devices/${id}/disable`, {});
    message.value = 'Device disabled.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
  }
}

async function updateDevice() {
  const id = Number(deviceUpdateForm.value.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'device id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiPut(`/api/v1/devices/${id}`, {
      rustdesk_id: String(deviceUpdateForm.value.rustdesk_id || '').trim(),
      name: String(deviceUpdateForm.value.name || '').trim(),
      alias: String(deviceUpdateForm.value.alias || '').trim(),
      status: String(deviceUpdateForm.value.status || '').trim(),
      platform: String(deviceUpdateForm.value.platform || '').trim(),
      client_version: String(deviceUpdateForm.value.client_version || '').trim(),
      opendesk_client_version: String(deviceUpdateForm.value.opendesk_client_version || '').trim()
    });
    message.value = 'Device updated.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
  }
}

function fillDeviceUpdate(record: Record<string, unknown>) {
  deviceUpdateForm.value = {
    id: Number(record.id || 1),
    rustdesk_id: String(record.rustdesk_id || ''),
    name: String(record.name || ''),
    alias: String(record.alias || ''),
    status: String(record.status || 'offline'),
    platform: String(record.platform || ''),
    client_version: String(record.client_version || ''),
    opendesk_client_version: String(record.opendesk_client_version || '')
  };
}

async function disableRelay(record: Record<string, unknown>) {
	const id = Number(record.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'relay id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiPost(`/api/v1/relays/${id}/disable`, {});
    message.value = 'Relay disabled.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
	}
}

async function updateRelay() {
  const id = Number(relayUpdateForm.value.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'relay id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiPut(`/api/v1/relays/${id}`, {
      name: String(relayUpdateForm.value.name || '').trim(),
      region: String(relayUpdateForm.value.region || '').trim(),
      host: String(relayUpdateForm.value.host || '').trim(),
      port: Number(relayUpdateForm.value.port || 0),
      ws_port: Number(relayUpdateForm.value.ws_port || 0),
      status: String(relayUpdateForm.value.status || '').trim(),
      public_key_fingerprint: String(relayUpdateForm.value.public_key_fingerprint || '').trim()
    });
    message.value = 'Relay updated.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
  }
}

function fillRelayUpdate(record: Record<string, unknown>) {
  relayUpdateForm.value = {
    id: Number(record.id || 1),
    name: String(record.name || ''),
    region: String(record.region || ''),
    host: String(record.host || ''),
    port: Number(record.port || 21117),
    ws_port: Number(record.ws_port || 21119),
    status: String(record.status || 'active'),
    public_key_fingerprint: String(record.public_key_fingerprint || '')
  };
}

async function deleteAccessRule(record: Record<string, unknown>) {
  const id = Number(record.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'access rule id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiDelete(`/api/v1/access-rules/${id}`);
    message.value = 'Access rule deleted.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
  }
}

async function updateAccessRule() {
  const id = Number(accessRuleUpdateForm.value.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'access rule id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiPut(`/api/v1/access-rules/${id}`, {
      subject_type: String(accessRuleUpdateForm.value.subject_type || '').trim(),
      subject_id: Number(accessRuleUpdateForm.value.subject_id || 0),
      target_type: String(accessRuleUpdateForm.value.target_type || '').trim(),
      target_id: Number(accessRuleUpdateForm.value.target_id || 0),
      effect: String(accessRuleUpdateForm.value.effect || '').trim(),
      priority: Number(accessRuleUpdateForm.value.priority || 0),
      enabled: Boolean(accessRuleUpdateForm.value.enabled)
    });
    message.value = 'Access rule updated.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
  }
}

function fillAccessRuleUpdate(record: Record<string, unknown>) {
  accessRuleUpdateForm.value = {
    id: Number(record.id || 1),
    subject_type: String(record.subject_type || 'user'),
    subject_id: Number(record.subject_id || 1),
    target_type: String(record.target_type || 'device'),
    target_id: Number(record.target_id || 1),
    effect: String(record.effect || 'allow'),
    priority: Number(record.priority || 0),
    enabled: record.enabled !== false
  };
}

async function deleteControlRole(record: Record<string, unknown>) {
  const id = Number(record.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'control role id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiDelete(`/api/v1/control-roles/${id}`);
    message.value = 'Control role deleted.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
  }
}

async function updateControlRole() {
  const id = Number(controlRoleUpdateForm.value.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'control role id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiPut(`/api/v1/control-roles/${id}`, {
      name: String(controlRoleUpdateForm.value.name || '').trim(),
      description: String(controlRoleUpdateForm.value.description || '').trim(),
      enabled: Boolean(controlRoleUpdateForm.value.enabled),
      permissions: parseJSONValue(controlRoleUpdateForm.value.permissions_json, [])
    });
    message.value = 'Control role updated.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
  }
}

function fillControlRoleUpdate(record: Record<string, unknown>) {
  controlRoleUpdateForm.value = {
    id: Number(record.id || 1),
    name: String(record.name || ''),
    description: String(record.description || ''),
    enabled: record.enabled !== false,
    permissions_json: String(record.permissions || '[]')
  };
}

async function deleteStrategy(record: Record<string, unknown>) {
  const id = Number(record.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'strategy id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiDelete(`/api/v1/strategies/${id}`);
    message.value = 'Strategy deleted.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
  }
}

async function updateStrategy() {
  const id = Number(strategyUpdateForm.value.id);
  if (!Number.isFinite(id) || id <= 0) {
    message.value = 'strategy id must be positive';
    return;
  }
  actionId.value = id;
  message.value = '';
  try {
    await apiPut(`/api/v1/strategies/${id}`, {
      name: String(strategyUpdateForm.value.name || '').trim(),
      description: String(strategyUpdateForm.value.description || '').trim(),
      enabled: Boolean(strategyUpdateForm.value.enabled),
      settings_json: parseJSONValue(strategyUpdateForm.value.settings_json, {}),
      assignments: parseJSONValue(strategyUpdateForm.value.assignments_json, [])
    });
    message.value = 'Strategy updated.';
    await load();
  } catch (err) {
    message.value = humanError(err);
  } finally {
    actionId.value = null;
  }
}

function fillStrategyUpdate(record: Record<string, unknown>) {
  strategyUpdateForm.value = {
    id: Number(record.id || 1),
    name: String(record.name || ''),
    description: String(record.description || ''),
    enabled: record.enabled !== false,
    settings_json: String(record.settings_json || '{}'),
    assignments_json: String(record.assignments || '[]')
  };
}

function preparePayload(): Record<string, unknown> {
  const payload: Record<string, unknown> = { ...form.value };
  if (props.endpoint === '/api/v1/access-rules') {
    payload.subject_id = Number(payload.subject_id);
    payload.target_id = Number(payload.target_id);
    payload.priority = Number(payload.priority || 0);
    payload.enabled = Boolean(payload.enabled);
  }
  if (props.endpoint === '/api/v1/user-groups') {
    payload.member_user_ids = parseJSONField('member_user_ids_json', []);
    delete payload.member_user_ids_json;
  }
  if (props.endpoint === '/api/v1/device-groups') {
    payload.member_device_ids = parseJSONField('member_device_ids_json', []);
    delete payload.member_device_ids_json;
  }
  if (props.endpoint === '/api/v1/address-books') {
    payload.owner_user_id = Number(payload.owner_user_id || 0) || undefined;
    payload.entries = parseJSONField('entries_json', []);
    delete payload.entries_json;
  }
  if (props.endpoint === '/api/v1/control-roles') {
    payload.permissions = parseJSONField('permissions_json', []);
    delete payload.permissions_json;
    payload.enabled = Boolean(payload.enabled);
  }
  if (props.endpoint === '/api/v1/strategies') {
    payload.settings_json = parseJSONField('settings_json', {});
    payload.assignments = parseJSONField('assignments_json', []);
    delete payload.assignments_json;
    payload.enabled = Boolean(payload.enabled);
  }
  return payload;
}

function parseJSONField(key: string, fallback: unknown): unknown {
  const value = form.value[key];
  if (typeof value !== 'string' || value.trim() === '') return fallback;
  return JSON.parse(value);
}

function parseJSONValue(value: unknown, fallback: unknown): unknown {
  if (typeof value !== 'string' || value.trim() === '') return fallback;
  return JSON.parse(value);
}

function resetForm() {
  const next: Record<string, any> = {};
  for (const field of fields.value) {
    next[field.key] = field.defaultValue ?? '';
  }
  form.value = next;
}

function resetRelationForm() {
  const next: Record<string, any> = {};
  for (const field of relationFields.value) {
    next[field.key] = field.defaultValue ?? '';
  }
  relationForm.value = next;
}

function relationTarget(): number | null {
  if (props.endpoint === '/api/v1/address-books') {
    return positiveRelationNumber('book_id', 'book_id must be positive');
  }
  return positiveRelationNumber('group_id', 'group_id must be positive');
}

function positiveRelationNumber(key: string, errorMessage: string): number | null {
  const value = Number(relationForm.value[key]);
  if (!Number.isFinite(value) || value <= 0) {
    message.value = errorMessage;
    return null;
  }
  return value;
}

function enumOptions(values: string[]) {
  return values.map((value) => ({ label: value, value }));
}

function formatCell(value: unknown): unknown {
  if (Array.isArray(value) || (value && typeof value === 'object')) {
    return JSON.stringify(value);
  }
  return value;
}

onMounted(() => {
  resetForm();
  resetRelationForm();
  load();
});
watch(() => props.endpoint, () => {
  resetForm();
  resetRelationForm();
  load();
});
</script>
