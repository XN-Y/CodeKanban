import { createAlova } from 'alova';
import fetchAdapter from 'alova/fetch';
import VueHook from 'alova/vue';
import { createApis, withConfigType, mountApis } from './createApis';

export { useReq, useInit, FORCE } from './composable';

export class ApiError extends Error {
  public status: number;
  public statusText: string;
  public data: any;

  constructor(status: number, statusText: string, data: any) {
    const message =
      typeof data === 'object' && data?.detail
        ? data.detail
        : typeof data === 'string'
          ? data
          : statusText;
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.statusText = statusText;
    this.data = data;
  }
}

export const urlBase = import.meta.env.DEV ? '' : window.location.origin;

export const alovaInstance = createAlova({
  baseURL: urlBase,
  requestAdapter: fetchAdapter(),
  statesHook: VueHook,
  beforeRequest: method => {
    method.config.credentials = 'include';
  },
  responded: async (response, method) => {
    if (!response.ok) {
      const responseText = await response.text();
      let data;

      try {
        data = JSON.parse(responseText);
      } catch {
        data = responseText;
      }

      const requestURL = String((method as any).url || '');
      if (response.status === 401 && !requestURL.includes('/api/v1/auth/')) {
        if (typeof window !== 'undefined') {
          window.dispatchEvent(
            new CustomEvent('codekanban:unauthorized', {
              detail: { url: requestURL },
            })
          );
        }
      }

      throw new ApiError(response.status, response.statusText, data);
    }

    const responseType = (method as any).config?.responseType;
    if (responseType === 'blob') {
      return response.blob();
    }
    if (responseType === 'arraybuffer') {
      return response.arrayBuffer();
    }
    if (responseType === 'text') {
      return response.text();
    }

    const contentType = response.headers.get('content-type') || '';
    if (contentType.includes('application/json')) {
      const clone = response.clone();
      try {
        return await response.json();
      } catch {
        try {
          return await clone.blob();
        } catch {
          return await clone.text();
        }
      }
    }
    if (contentType.includes('application/pdf')) {
      return response.blob();
    }
    return response.text();
  },
});

export const $$userConfigMap = withConfigType({});

const Apis = createApis(alovaInstance, $$userConfigMap);

mountApis(Apis);

export default Apis;
export { Apis };

// @ts-ignore
export * from './globals.d.ts';
