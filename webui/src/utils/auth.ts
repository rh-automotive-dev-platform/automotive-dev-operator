export const API_BASE: string = (window as any).__API_BASE || '';

export const BUILD_API_BASE: string = API_BASE || `https://${window.location.host.replace('ado-webui-', 'ado-build-api-')}`;

function getReturnUrl(): string {
  const path = window.location.pathname + window.location.search + window.location.hash;
  return window.location.origin + (path || '/');
}

export async function authFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const url = typeof input === 'string' || input instanceof URL ? String(input) : '';
  const useProxy = url.startsWith('/v1');
  const finalUrl = useProxy ? url : (API_BASE ? API_BASE + url : url);
  const response = await fetch(finalUrl, {
    credentials: 'include',
    cache: 'no-store',
    keepalive: true,
    ...init,
  });
  if ((response.status === 401 || response.status === 403) && window.location.hostname.indexOf('localhost') === -1) {
    const rd = encodeURIComponent(getReturnUrl());
    window.location.href = `/oauth/start?rd=${rd}`;
    throw new Error('Redirecting to login');
  }
  return response;
}


