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

export function calendarForGameDay(gameDay: number): CalendarView {
	const yearIndex = Math.floor(gameDay / 364);
	const dayOfYear = ((gameDay % 364) + 364) % 364;
	const halfYear = dayOfYear < 182 ? 'summer' : 'winter';
	const productionSeason =
		dayOfYear < 91 ? 'spring' : dayOfYear < 182 ? 'summer' : dayOfYear < 273 ? 'autumn' : 'winter';
	const phase =
		dayOfYear >= 91 && dayOfYear < 121
			? 'early_summer'
			: dayOfYear >= 121 && dayOfYear < 152
				? 'high_summer'
				: dayOfYear >= 152 && dayOfYear < 182
					? 'late_summer'
					: dayOfYear >= 273 && dayOfYear < 304
						? 'early_winter'
						: dayOfYear >= 304 && dayOfYear < 334
							? 'midwinter'
							: dayOfYear >= 334
								? 'late_winter'
								: '';
	return {
		game_day: gameDay,
		year_index: yearIndex,
		day_of_year: dayOfYear,
		week_of_year: Math.floor(dayOfYear / 7) + 1,
		day_of_week: (dayOfYear % 7) + 1,
		production_season: productionSeason,
		half_year: halfYear,
		seasonal_phase: phase,
		phase,
		week_of_half: Math.floor((dayOfYear < 182 ? dayOfYear : dayOfYear - 182) / 7) + 1
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
	return formatCalendarPosition(value.phase || value.seasonal_phase, value.week_of_half);
}

export function formatPhase(value?: CalendarView): string {
	if (!value) return 'Season unknown';
	return formatPhaseName(value.phase || value.seasonal_phase || value.production_season);
}

export function formatPhaseName(value: string): string {
	return (
		phaseNames[value] ?? value.replaceAll('_', ' ').replace(/^\w/, (letter) => letter.toUpperCase())
	);
}

export function formatCalendarPosition(phase: string, weekOfHalf: number): string {
	return `${formatPhaseName(phase)} · ${ordinal(weekOfHalf)} week`;
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
