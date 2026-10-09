/**
 * Reads a server-sent event stream from a fetch Response and calls onEvent
 * with each event's JSON data. Resolves when the stream ends.
 */
export async function readEvents<T>(
  res: Response,
  onEvent: (event: T) => void,
): Promise<void> {
  if (!res.body) return;
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let end = buffer.indexOf('\n\n');
    while (end >= 0) {
      const frame = buffer.slice(0, end);
      buffer = buffer.slice(end + 2);
      const data = frame
        .split('\n')
        .filter((l) => l.startsWith('data: '))
        .map((l) => l.slice(6))
        .join('\n');
      if (data) {
        try {
          onEvent(JSON.parse(data) as T);
        } catch {
          // Skip a malformed frame rather than end the run view.
        }
      }
      end = buffer.indexOf('\n\n');
    }
  }
}
