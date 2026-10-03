import { scanningPlugin } from './plugin';

describe('scanning', () => {
  it('should export plugin', () => {
    expect(scanningPlugin).toBeDefined();
  });
});
