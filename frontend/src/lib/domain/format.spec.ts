import { describe, expect, it } from 'vitest';
import { formatMilli, parseMilli } from './format';

describe('milli-unit formatting', () => {
	it('formats units without floating-point arithmetic', () => {
		expect(formatMilli(30_000)).toBe('30');
		expect(formatMilli(7_500)).toBe('7.5');
		expect(formatMilli(1_001)).toBe('1.001');
	});

	it('parses positive unit input into exact milli-units', () => {
		expect(parseMilli('5')).toBe(5_000);
		expect(parseMilli('1.25')).toBe(1_250);
		expect(parseMilli('0.001')).toBe(1);
		expect(parseMilli('1.0001')).toBeNull();
		expect(parseMilli('0')).toBeNull();
	});
});
