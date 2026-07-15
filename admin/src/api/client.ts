const baseURL = import.meta.env.VITE_OPENDESK_API_URL || '';

export interface ApiEnvelope<T> {
  data?: T;
  error?: {
    code: string;
    message: string;
  };
}

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
  }
}

export async function apiGet<T>(path: string): Promise<T> {
  return apiRequest<T>(path, { method: 'GET' });
}

export async function apiPost<T>(path: string, data: unknown): Promise<T> {
  return apiRequest<T>(path, {
    method: 'POST',
    body: JSON.stringify(data)
  });
}

export async function apiPut<T>(path: string, data: unknown): Promise<T> {
  return apiRequest<T>(path, {
    method: 'PUT',
    body: JSON.stringify(data)
  });
}

export async function apiDelete<T>(path: string): Promise<T> {
  return apiRequest<T>(path, { method: 'DELETE' });
}

export async function apiDownload(path: string): Promise<Blob> {
  const response = await fetch(`${baseURL}${path}`, {
    credentials: 'include',
    headers: {
      Accept: 'application/octet-stream'
    }
  });
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as ApiEnvelope<unknown>;
    throw new ApiError(response.status, body.error?.code || 'API_ERROR', body.error?.message || response.statusText);
  }
  return response.blob();
}

async function apiRequest<T>(path: string, init: RequestInit): Promise<T> {
  const response = await fetch(`${baseURL}${path}`, {
    ...init,
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers
    }
  });
  const body = (await response.json().catch(() => ({}))) as ApiEnvelope<T>;
  if (!response.ok) {
    throw new ApiError(response.status, body.error?.code || 'API_ERROR', body.error?.message || response.statusText);
  }
  return body.data as T;
}

export function humanError(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 501) return 'This module is registered in the API skeleton and awaits implementation.';
    if (error.status === 401) return 'Authentication is required.';
    if (error.status === 403) return 'You do not have permission to perform this action.';
    return error.message;
  }
  if (error instanceof TypeError) return 'Cannot connect to OpenDesk API.';
  return 'Unexpected error.';
}
