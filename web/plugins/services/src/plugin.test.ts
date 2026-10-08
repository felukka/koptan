import { servicesPlugin } from './plugin';

describe('services', () => {
  it('should export plugin', () => {
    expect(servicesPlugin).toBeDefined();
  });
});
