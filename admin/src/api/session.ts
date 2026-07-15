import { reactive } from 'vue';
import { apiGet, apiPost } from './client';

export interface CurrentUser {
  id: number;
  email: string;
  username: string;
  display_name: string;
  status: string;
  source: string;
}

interface LoginResponse {
  user: CurrentUser;
  access_token: string;
  expires_at: string;
}

interface MeResponse {
  user: CurrentUser;
  expires_at: string;
}

export const sessionState = reactive<{
  user: CurrentUser | null;
  expiresAt: string;
  checked: boolean;
}>({
  user: null,
  expiresAt: '',
  checked: false
});

let pendingSession: Promise<CurrentUser> | null = null;

export async function fetchSession(force = false): Promise<CurrentUser> {
  if (sessionState.user && !force) return sessionState.user;
  if (pendingSession) return pendingSession;
  pendingSession = apiGet<MeResponse>('/api/v1/auth/me')
    .then((resp) => {
      sessionState.user = resp.user;
      sessionState.expiresAt = resp.expires_at;
      sessionState.checked = true;
      return resp.user;
    })
    .finally(() => {
      pendingSession = null;
    });
  return pendingSession;
}

export async function login(email: string, password: string): Promise<CurrentUser> {
  const resp = await apiPost<LoginResponse>('/api/v1/auth/login', { email, password });
  sessionState.user = resp.user;
  sessionState.expiresAt = resp.expires_at;
  sessionState.checked = true;
  return resp.user;
}

export async function logout(): Promise<void> {
  try {
    await apiPost<{ logged_out: boolean }>('/api/v1/auth/logout', {});
  } finally {
    clearSession();
  }
}

export function clearSession() {
  sessionState.user = null;
  sessionState.expiresAt = '';
  sessionState.checked = false;
}
