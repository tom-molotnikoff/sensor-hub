import createClient from 'openapi-fetch';
import type { FetchResponse, MaybeOptionalInit } from 'openapi-fetch';
import type { HttpMethod, MediaType, PathsWithMethod } from 'openapi-typescript-helpers';
import type { paths } from './schema';
import { getCsrfToken } from '../api/Csrf';

type StrictInit<P extends keyof paths, M extends HttpMethod> = MaybeOptionalInit<paths[P], M>;

type StrictMethod<M extends HttpMethod> = <P extends PathsWithMethod<paths, M>>(
  url: P,
  ...init: undefined extends StrictInit<P, M> ? [init?: StrictInit<P, M>] : [init: StrictInit<P, M>]
) => Promise<FetchResponse<Extract<paths[P][M], Record<string | number, unknown>>, StrictInit<P, M>, MediaType>>;

type StrictClient = {
  [M in Uppercase<HttpMethod>]: StrictMethod<Lowercase<M>>;
};

const client = createClient<paths>({
  baseUrl: import.meta.env.VITE_API_BASE || '/api',
  credentials: 'include',
});

client.use({
  async onRequest({ request }) {
    const token = getCsrfToken();
    if (token) request.headers.set('X-CSRF-Token', token);
    request.headers.set('X-Requested-With', 'XMLHttpRequest');
    return request;
  },
});

export const apiClient = client as unknown as StrictClient;
