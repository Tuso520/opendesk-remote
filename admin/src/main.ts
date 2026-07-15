import { createApp } from 'vue';
import { createRouter, createWebHistory } from 'vue-router';
import ArcoVue from '@arco-design/web-vue';
import '@arco-design/web-vue/dist/arco.css';
import App from './App.vue';
import DashboardView from './views/DashboardView.vue';
import SectionView from './views/SectionView.vue';
import BuilderView from './views/BuilderView.vue';
import LogsView from './views/LogsView.vue';
import SettingsView from './views/SettingsView.vue';
import LoginView from './views/LoginView.vue';
import { ApiError } from './api/client';
import { clearSession, fetchSession } from './api/session';
import './styles.css';

const routes = [
  { path: '/login', component: LoginView, meta: { public: true } },
  { path: '/', component: DashboardView },
  { path: '/users', component: SectionView, props: { title: 'Users', endpoint: '/api/v1/users' } },
  { path: '/user-groups', component: SectionView, props: { title: 'User Groups', endpoint: '/api/v1/user-groups' } },
  { path: '/devices', component: SectionView, props: { title: 'Devices', endpoint: '/api/v1/devices' } },
  { path: '/device-groups', component: SectionView, props: { title: 'Device Groups', endpoint: '/api/v1/device-groups' } },
  { path: '/address-books', component: SectionView, props: { title: 'Address Books', endpoint: '/api/v1/address-books' } },
  { path: '/access-control', component: SectionView, props: { title: 'Access Control', endpoint: '/api/v1/access-rules' } },
  { path: '/control-roles', component: SectionView, props: { title: 'Control Roles', endpoint: '/api/v1/control-roles' } },
  { path: '/strategies', component: SectionView, props: { title: 'Strategies', endpoint: '/api/v1/strategies' } },
  { path: '/relays', component: SectionView, props: { title: 'Relays', endpoint: '/api/v1/relays' } },
  { path: '/builder', component: BuilderView },
  { path: '/build-jobs', component: SectionView, props: { title: 'Build Jobs', endpoint: '/api/v1/build-jobs' } },
  { path: '/logs', component: LogsView },
  { path: '/settings', component: SettingsView },
  { path: '/about', component: SectionView, props: { title: 'About / Licenses', endpoint: '/api/v1/version' } }
];

const router = createRouter({ history: createWebHistory(), routes });

router.beforeEach(async (to) => {
  if (to.meta.public) return true;
  try {
    await fetchSession();
    return true;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      clearSession();
      return { path: '/login', query: { redirect: to.fullPath } };
    }
    return true;
  }
});

const app = createApp(App);

app
  .use(ArcoVue)
  .use(router)
  .mount('#app');
