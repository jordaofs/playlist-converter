export class ApiError extends Error {
  constructor(
    public code: string,
    message: string,
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  options: {
    method?: string;
    body?: unknown;
    csrf?: string;
    key?: string;
    signal?: AbortSignal;
  } = {},
): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (options.body !== undefined) headers['Content-Type'] = 'application/json';
  if (options.csrf) headers['X-CSRF-Token'] = options.csrf;
  if (options.key) headers['Idempotency-Key'] = options.key;
  const response = await fetch(path, {
    method: options.method ?? 'GET',
    credentials: 'same-origin',
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
    signal: options.signal,
  });
  let result;
  try {
    result = await response.json();
  } catch {
    throw new ApiError(
      'invalid_response',
      'The server returned an unexpected response. Please try again.',
    );
  }
  if (!response.ok || !result.success)
    throw new ApiError(
      result.error?.code ?? 'request_failed',
      result.error?.message ?? 'Something went wrong. Please try again.',
    );
  return result.data as T;
}
export function safeLink(value?: string): string | undefined {
  if (!value) return undefined;
  try {
    const u = new URL(value);
    return u.protocol === 'https:' &&
      !u.username &&
      !u.password &&
      [
        'open.spotify.com',
        'youtube.com',
        'www.youtube.com',
        'music.youtube.com',
      ].includes(u.hostname)
      ? u.href
      : undefined;
  } catch {
    return undefined;
  }
}
