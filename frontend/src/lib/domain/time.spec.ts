import { describe, expect, it } from 'vitest';
import {
	calendarForGameDay,
	calendarGroupForEvent,
	formatCalendarPosition,
	formatInterval,
	formatPhaseName,
	formatRelativeGameDay,
	settingYear
} from './time';

describe('game calendar presentation', () => {
	it('formats recurring intervals', () => {
		expect(formatInterval(7)).toBe('every week');
		expect(formatInterval(14)).toBe('every two weeks');
		expect(formatInterval(28)).toBe('every four weeks');
	});

	it('formats relative days', () => {
		expect(formatRelativeGameDay(10, 10)).toBe('today');
		expect(formatRelativeGameDay(10, 11)).toBe('tomorrow');
		expect(formatRelativeGameDay(10, 16)).toBe('in 6 days');
		expect(formatRelativeGameDay(10, 4)).toBe('6 days ago');
	});

	it('uses the production season when a shoulder season has no named phase', () => {
		expect(formatPhaseName(calendarForGameDay(0).production_season)).toBe('Spring');
		expect(formatCalendarPosition('spring', 1)).toBe('Spring · first week');
		expect(formatPhaseName('late_winter')).toBe('Late winter');
	});

	it('formats the historical setting year from the projected year index', () => {
		expect(settingYear(980, { year_index: 0 })).toBe(980);
		expect(settingYear(980, { year_index: 1 })).toBe(981);
		expect(settingYear(980, { year_index: 5 })).toBe(985);
	});

	it('groups urgent action before ordinary upcoming events', () => {
		expect(calendarGroupForEvent(100, 102, true, 'critical')).toBe('urgent');
		expect(calendarGroupForEvent(100, 100, false, 'context')).toBe('today');
		expect(calendarGroupForEvent(100, 105, false, 'important')).toBe('this_week');
		expect(calendarGroupForEvent(100, 110, false, 'important')).toBe('next_week');
		expect(calendarGroupForEvent(100, 150, false, 'context')).toBe('later_current_half');
		expect(calendarGroupForEvent(100, 190, false, 'context')).toBe('next_half');
	});
});
