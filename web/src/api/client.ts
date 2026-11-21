import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios';
import type { AuthResponse } from './types';

// The access token lives only in memory (never localStorage) to limit XSS impact.
// The refresh token is an HttpOnly cookie managed by the browser.
let accessToken: string | null = null;
let onSessionExpired: () => void = () => {};

export const setAccessToken = (token: string | null) => {
  accessToken = token;
};

export const setSessionExpiredHandler = (handler: () => void) => {
  onSessionExpired = handler;
};

export const api = axios.create({ baseURL: '/api/v1', withCredentials: true, timeout: 15000 });

api.interceptors.request.use((config) => {
  if (accessToken) config.headers.Authorization = `Bearer ${accessToken}`;
  return config;
});

// Share one in-flight refresh so parallel 401s (or React StrictMode) don't
// rotate the refresh token twice and log the user out.
let refreshPromise: Promise<AuthResponse> | null = null;

export function refreshSession(): Promise<AuthResponse> {
  refreshPromise ??= axios
    .post<AuthResponse>('/api/v1/auth/refresh', null, { withCredentials: true })
    .then((res) => {
      setAccessToken(res.data.accessToken);
      return res.data;
    })
    .finally(() => {
      refreshPromise = null;
    });
  return refreshPromise;
}

type RetriableConfig = InternalAxiosRequestConfig & { _retried?: boolean };

// On 401, refresh the access token once and replay the original request.
api.interceptors.response.use(
  (res) => res,
  async (error: AxiosError) => {
    const config = error.config as RetriableConfig | undefined;
    const isAuthCall = config?.url?.startsWith('/auth/');
    if (error.response?.status !== 401 || !config || config._retried || isAuthCall) {
      return Promise.reject(error);
    }
    config._retried = true;
    try {
      await refreshSession();
      return api(config);
    } catch {
      setAccessToken(null);
      onSessionExpired();
      return Promise.reject(error);
    }
  },
);
