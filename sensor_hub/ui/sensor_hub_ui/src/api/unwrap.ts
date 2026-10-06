export async function unwrap<T>(request: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
  const { data, error, response } = await request;
  if (data !== undefined) return data;
  const message = (error as { message?: string } | undefined)?.message;
  throw new Error(message ?? `${response.status} ${response.statusText}`);
}
