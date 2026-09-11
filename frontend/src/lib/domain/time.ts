export type CalendarView = {
	game_day: number;
	year_index: number;
	day_of_year: number;
	week_of_year: number;
	day_of_week: number;
	production_season: string;
	half_year: string;
	seasonal_phase: string;
	phase?: string;
	week_of_half: number;
};

export type CalendarGroup =
	'urgent' | 'today' | 'this_week' | 'next_week' | 'later_current_half' | 'next_half' | 'later';

export function settingYear(startYear: number, calendar: Pick<CalendarView, 'year_index'>): number {
	return startYear + calendar.year_index;
}

type TimeModel = 'legacy' | 'daily_labor_v1' | 'monthly_seasons_v1';

export function calendarForGameDay(gameDay: number, model: TimeModel = 'legacy'): CalendarView {
	const daysPerYear = model === 'legacy' ? 364 : 365;
	const summerEnd = model === 'legacy' ? 182 : 183;
	const autumnEnd = model === 'legacy' ? 273 : 274;
	const yearIndex = Math.floor(gameDay / daysPerYear);
	const dayOfYear = ((gameDay % daysPerYear) + daysPerYear) % daysPerYear;
	const halfYear = dayOfYear < summerEnd ? 'summer' : 'winter';
	const productionSeason =
		dayOfYear < 91
			? 'spring'
			: dayOfYear < summerEnd
				? 'summer'
				: dayOfYear < autumnEnd
					? 'autumn'
					: 'winter';
	const phase =
		dayOfYear >= 91 && dayOfYear < 121
			? 'early_summer'
			: dayOfYear >= 121 && dayOfYear < 152
				? 'high_summer'
				: dayOfYear >= 152 && dayOfYear < summerEnd
					? 'late_summer'
					: dayOfYear >= autumnEnd && dayOfYear < autumnEnd + 30
						? 'early_winter'
						: dayOfYear >= 304 && dayOfYear < 334
							? 'midwinter'
							: dayOfYear >= autumnEnd + 60
								? 'late_winter'
								: '';
	return {
		game_day: gameDay,
		year_index: yearIndex,
		day_of_year: dayOfYear,
		week_of_year: Math.floor(dayOfYear / 7) + 1,
		day_of_week: (((gameDay % 7) + 7) % 7) + 1,
		production_season: productionSeason,
		half_year: halfYear,
		seasonal_phase: phase,
		phase,
		week_of_half: Math.floor((dayOfYear < summerEnd ? dayOfYear : dayOfYear - summerEnd) / 7) + 1
	};
}

const phaseNames: Record<string, string> = {
	early_summer: 'Early summer',
	high_summer: 'High summer',
	late_summer: 'Late summer',
	early_winter: 'Early winter',
	midwinter: 'Midwinter',
	late_winter: 'Late winter'
};

export function formatGameDay(value?: CalendarView): string {
	if (!value) return 'World calendar';
	return formatCalendarPosition(
		value.phase || value.seasonal_phase || value.production_season,
		value.week_of_half
	);
}

export function formatPhase(value?: CalendarView): string {
	if (!value) return 'Season unknown';
	return formatPhaseName(value.phase || value.seasonal_phase || value.production_season);
}

export function formatPhaseName(value: string): string {
	if (!value) return 'Season unknown';
	return (
		phaseNames[value] ?? value.replaceAll('_', ' ').replace(/^\w/, (letter) => letter.toUpperCase())
	);
}

export function formatCalendarPosition(phase: string, weekOfHalf: number): string {
	return `${formatPhaseName(phase)} · ${ordinal(weekOfHalf)} week`;
}

export function nextHalfYearStart(gameDay: number, model: TimeModel = 'legacy'): number {
	const daysPerYear = model === 'legacy' ? 364 : 365;
	const half = model === 'legacy' ? 182 : 183;
	const year = Math.floor(gameDay / daysPerYear);
	const dayOfYear = ((gameDay % daysPerYear) + daysPerYear) % daysPerYear;
	return dayOfYear < half ? year * daysPerYear + half : (year + 1) * daysPerYear;
}

export function calendarGroupForEvent(
	currentGameDay: number,
	targetGameDay: number,
	actionRequired: boolean,
	importance: string,
	nextHalfStart = nextHalfYearStart(currentGameDay),
	model: TimeModel = 'legacy'
): CalendarGroup {
	const days = targetGameDay - currentGameDay;
	if (days === 0) return 'today';
	if (days > 0 && actionRequired && importance === 'critical' && days <= 7) return 'urgent';
	if (days > 0 && days <= 7) return 'this_week';
	if (days > 7 && days <= 14) return 'next_week';
	if (targetGameDay < nextHalfStart) return 'later_current_half';
	const halfLength = model === 'legacy' ? 364 - 182 : 365 - 183;
	if (targetGameDay < nextHalfStart + halfLength) return 'next_half';
	return 'later';
}

export function formatRelativeGameDay(current: number, target: number): string {
	const days = target - current;
	if (days === 0) return 'today';
	if (days === 1) return 'tomorrow';
	if (days === -1) return 'yesterday';
	return days > 0 ? `in ${days} days` : `${Math.abs(days)} days ago`;
}

export function formatInterval(days: number): string {
	switch (days) {
		case 7:
			return 'every week';
		case 14:
			return 'every two weeks';
		case 28:
			return 'every four weeks';
		default:
			return `every ${days} days`;
	}
}

export function formatGameDayDuration(current: number, target: number): string {
	return formatRelativeGameDay(current, target);
}

export function formatHour(hour: number): string {
	return `${String(hour).padStart(2, '0')}:00`;
}

export function formatMoment(moment: { day: number; hour: number }, currentDay: number): string {
	return `${formatRelativeGameDay(currentDay, moment.day)} at ${formatHour(moment.hour)}`;
}

export function formatUTCOffset(minutes: number): string {
	const sign = minutes >= 0 ? '+' : '-';
	const absolute = Math.abs(minutes);
	return `UTC${sign}${String(Math.floor(absolute / 60)).padStart(2, '0')}:${String(absolute % 60).padStart(2, '0')}`;
}

function ordinal(value: number): string {
	const names = [
		'first',
		'second',
		'third',
		'fourth',
		'fifth',
		'sixth',
		'seventh',
		'eighth',
		'ninth',
		'tenth',
		'eleventh',
		'twelfth',
		'thirteenth',
		'fourteenth',
		'fifteenth',
		'sixteenth',
		'seventeenth',
		'eighteenth',
		'nineteenth',
		'twentieth',
		'twenty-first',
		'twenty-second',
		'twenty-third',
		'twenty-fourth',
		'twenty-fifth',
		'twenty-sixth',
		'twenty-seventh',
		'twenty-eighth',
		'twenty-ninth',
		'thirtieth',
		'thirty-first',
		'thirty-second',
		'thirty-third',
		'thirty-fourth',
		'thirty-fifth',
		'thirty-sixth',
		'thirty-seventh',
		'thirty-eighth',
		'thirty-ninth',
		'fortieth',
		'forty-first',
		'forty-second',
		'forty-third',
		'forty-fourth',
		'forty-fifth',
		'forty-sixth',
		'forty-seventh',
		'forty-eighth',
		'forty-ninth',
		'fiftieth',
		'fifty-first',
		'fifty-second'
	];
	return names[value - 1] ?? `${value}th`;
}
