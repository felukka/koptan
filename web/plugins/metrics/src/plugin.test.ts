import { metricsPlugin } from './plugin';

describe('metrics', () => {
  it('should export plugin', () => {
    expect(metricsPlugin).toBeDefined();
  });
});
