import { alertingPlugin } from './plugin';

describe('alerting', () => {
  it('should export plugin', () => {
    expect(alertingPlugin).toBeDefined();
  });
});
