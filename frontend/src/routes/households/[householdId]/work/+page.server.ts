import { changeOccupation, occupationPreview } from '$lib/server/household-actions';
import type { Actions } from './$types';

export const actions = { changeOccupation, occupationPreview } satisfies Actions;
