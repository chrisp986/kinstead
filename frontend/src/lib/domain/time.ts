export type CalendarView = {
	game_day: number;
	year_index: number;
	day_of_year: number;
	week_of_year: number;
	day_of_week: number;
	production_season: string;
	half_year: string;
	seasonal_phase: string;
};

export function formatGameDay(value?: CalendarView): string {
	if (!value) return 'World calendar';
	return `Year ${value.year_index + 1}, week ${value.week_of_year}`;
}

export function formatPhase(value?: CalendarView): string {
	if (!value) return 'season unknown';
	return (value.seasonal_phase || value.production_season).replaceAll('_', ' ');
}
