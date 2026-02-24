// One JSON object per line on stderr, with known secrets masked.

const secrets = new Set();

/** Registers values that must never appear in logs. */
export function addSecret(value) {
  if (value && value.length >= 6) secrets.add(value);
}

/** Replaces every registered secret in text. */
export function mask(text) {
  let out = String(text);
  for (const s of secrets) out = out.split(s).join('***');
  return out;
}

export function log(level, msg, fields = {}) {
  const line = JSON.stringify({
    time: new Date().toISOString(),
    level,
    msg,
    ...fields,
  });
  process.stderr.write(`${mask(line)}\n`);
}
