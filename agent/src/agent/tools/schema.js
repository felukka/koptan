// A tiny validator for the flat object schemas the tools declare: required
// keys, primitive types, no unknown keys. Model output is untrusted.

/** @returns {string | undefined} what is wrong, if anything */
export function checkInput(schema, input) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) return 'input must be an object';
  for (const key of schema.required ?? []) {
    if (!(key in input)) return `missing "${key}"`;
  }
  for (const [key, value] of Object.entries(input)) {
    const prop = schema.properties?.[key];
    if (!prop) return `unknown field "${key}"`;
    const type = prop.type === 'integer' ? 'number' : prop.type;
    if (typeof value !== type || (prop.type === 'integer' && !Number.isInteger(value))) {
      return `"${key}" must be ${prop.type}`;
    }
  }
  return undefined;
}

/** A strict object schema from property definitions. */
export const object = (properties, required = Object.keys(properties)) => ({
  type: 'object',
  properties,
  required,
  additionalProperties: false,
});
