const resourceNames: Record<string, string> = {
	provisions: 'Provisions',
	wood: 'Wood',
	trade_goods: 'Trade goods',
	silver: 'Silver'
};

const activityNames: Record<string, string> = {
	agriculture: 'Agriculture',
	fishing: 'Fishing',
	woodcutting: 'Woodcutting',
	building: 'Building',
	crafting: 'Crafting',
	training: 'Training',
	market: 'Market work',
	travel: 'Travel',
	ruler_service: 'Ruler service',
	rest: 'Rest'
};

export function labelResource(value: string): string {
	return resourceNames[value] ?? sentenceCase(value);
}

export function labelActivity(value: string): string {
	return activityNames[value] ?? sentenceCase(value);
}

export function sentenceCase(value: string): string {
	const words = value.replaceAll('_', ' ');
	return words.charAt(0).toUpperCase() + words.slice(1);
}

export function formatProjectedUnits(value: number): string {
	return new Intl.NumberFormat('en', { maximumFractionDigits: 1 }).format(value);
}

export function formatMilli(value: number): string {
	if (!Number.isSafeInteger(value)) return '—';
	const sign = value < 0 ? '-' : '';
	const absolute = Math.abs(value);
	const whole = Math.floor(absolute / 1000);
	const fraction = String(absolute % 1000)
		.padStart(3, '0')
		.replace(/0+$/, '');
	return `${sign}${whole}${fraction ? `.${fraction}` : ''}`;
}

export function parseMilli(value: string): number | null {
	const match = /^(\d+)(?:\.(\d{1,3}))?$/.exec(value.trim());
	if (!match) return null;
	const whole = Number(match[1]);
	const fraction = Number((match[2] ?? '').padEnd(3, '0'));
	const result = whole * 1000 + fraction;
	return Number.isSafeInteger(result) && result > 0 ? result : null;
}

export function shortId(value: string): string {
	return value.slice(-6);
}
