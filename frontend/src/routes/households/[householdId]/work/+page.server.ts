import { assign, workPreview } from '$lib/server/household-actions';
import type { Actions } from './$types';

export const actions = { assign, workPreview } satisfies Actions;
