<template>
  <main class="login-shell">
    <section class="login-panel">
      <div class="login-brand">
        <div class="brand-mark">OD</div>
        <div>
          <strong>OpenDesk Remote</strong>
          <span>Admin Control Plane</span>
        </div>
      </div>
      <a-card class="login-card" :bordered="false">
        <template #title>Administrator Sign In</template>
        <a-space direction="vertical" fill>
          <a-alert v-if="error" type="error">{{ error }}</a-alert>
          <a-form :model="form" layout="vertical">
            <a-form-item label="Email">
              <a-input v-model="form.email" autocomplete="username" />
            </a-form-item>
            <a-form-item label="Password">
              <a-input-password v-model="form.password" autocomplete="current-password" @press-enter="submit" />
            </a-form-item>
            <a-button type="primary" long :loading="loading" @click="submit">Sign in</a-button>
          </a-form>
        </a-space>
      </a-card>
    </section>
  </main>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { humanError } from '../api/client';
import { login } from '../api/session';

const route = useRoute();
const router = useRouter();
const loading = ref(false);
const error = ref('');
const form = reactive({
  email: '',
  password: ''
});

async function submit() {
  if (!form.email || !form.password || loading.value) return;
  loading.value = true;
  error.value = '';
  try {
    await login(form.email, form.password);
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/';
    await router.replace(redirect);
  } catch (err) {
    error.value = humanError(err);
  } finally {
    loading.value = false;
  }
}
</script>
