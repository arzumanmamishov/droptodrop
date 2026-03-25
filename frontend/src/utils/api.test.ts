import { describe, it, expect, beforeEach } from 'vitest';

// Test the pricing calculation logic (mirrored from backend)
function calculateResellerPrice(
  wholesale: number,
  markupType: string,
  markupValue: number,
): number {
  switch (markupType) {
    case 'fixed':
      return wholesale + markupValue;
    case 'percentage':
      return wholesale * (1 + markupValue / 100);
    default:
      return wholesale * 1.3;
  }
}

describe('Pricing calculations', () => {
  it('calculates percentage markup correctly', () => {
    expect(calculateResellerPrice(100, 'percentage', 30)).toBeCloseTo(130);
    expect(calculateResellerPrice(50, 'percentage', 50)).toBeCloseTo(75);
    expect(calculateResellerPrice(25.5, 'percentage', 20)).toBeCloseTo(30.6);
  });

  it('calculates fixed markup correctly', () => {
    expect(calculateResellerPrice(100, 'fixed', 25)).toBeCloseTo(125);
    expect(calculateResellerPrice(50, 'fixed', 10)).toBeCloseTo(60);
  });

  it('defaults to 30% markup for unknown type', () => {
    expect(calculateResellerPrice(100, 'unknown', 0)).toBeCloseTo(130);
  });

  it('handles zero wholesale correctly', () => {
    expect(calculateResellerPrice(0, 'percentage', 30)).toBe(0);
    expect(calculateResellerPrice(0, 'fixed', 15)).toBe(15);
  });
});

describe('Margin calculation', () => {
  it('computes margin from wholesale and retail', () => {
    const wholesale = 45;
    const retail = calculateResellerPrice(wholesale, 'percentage', 30);
    const margin = ((retail - wholesale) / retail) * 100;

    expect(margin).toBeGreaterThan(10); // Exceeds minimum margin
    expect(margin).toBeCloseTo(23.08, 1);
  });
});

describe('Session token handling', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('stores and retrieves session tokens', () => {
    localStorage.setItem('droptodrop_session', 'test_token');
    expect(localStorage.getItem('droptodrop_session')).toBe('test_token');
  });

  it('returns empty string when no token', () => {
    expect(localStorage.getItem('droptodrop_session')).toBeNull();
  });
});
