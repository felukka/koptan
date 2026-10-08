import { monitoringPlugin } from './plugin';

describe('monitoring', () => {
  it('should export plugin', () => {
    expect(monitoringPlugin).toBeDefined();
  });
});
