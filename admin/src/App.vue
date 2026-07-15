<template>
  <RouterView v-if="isLoginRoute" />
  <a-layout v-else class="layout">
    <a-layout-sider class="sidebar" :width="260">
      <div class="brand">
        <div class="brand-mark">OD</div>
        <div>
          <strong>OpenDesk Remote</strong>
          <span>Admin</span>
        </div>
      </div>
      <a-menu class="nav-menu" theme="dark" :selected-keys="[route.path]" @menu-item-click="go">
        <a-menu-item v-for="item in menu" :key="item.path">{{ item.label }}</a-menu-item>
      </a-menu>
    </a-layout-sider>
    <a-layout>
      <a-layout-header class="topbar">
        <div>
          <span class="eyebrow">Control Plane</span>
          <h1>{{ currentTitle }}</h1>
        </div>
        <a-space class="topbar-actions">
          <a-tag color="green">Relay auth required</a-tag>
          <a-tag v-if="sessionState.user">{{ sessionState.user.email }}</a-tag>
          <a-button size="small" @click="signOut">Logout</a-button>
        </a-space>
      </a-layout-header>
      <a-layout-content>
        <RouterView />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { logout, sessionState } from './api/session';

const menu = [
  { label: 'Dashboard', path: '/' },
  { label: 'Users', path: '/users' },
  { label: 'User Groups', path: '/user-groups' },
  { label: 'Devices', path: '/devices' },
  { label: 'Device Groups', path: '/device-groups' },
  { label: 'Address Books', path: '/address-books' },
  { label: 'Access Control', path: '/access-control' },
  { label: 'Control Roles', path: '/control-roles' },
  { label: 'Strategies', path: '/strategies' },
  { label: 'Relays', path: '/relays' },
  { label: 'Custom Client Builder', path: '/builder' },
  { label: 'Build Jobs', path: '/build-jobs' },
  { label: 'Logs', path: '/logs' },
  { label: 'Settings', path: '/settings' },
  { label: 'About / Licenses', path: '/about' }
];

const route = useRoute();
const router = useRouter();
const isLoginRoute = computed(() => route.path === '/login');
const currentTitle = computed(() => menu.find((item) => item.path === route.path)?.label || 'OpenDesk Remote');

function go(key: string) {
  router.push(key);
}

async function signOut() {
  await logout();
  await router.replace('/login');
}
</script>
