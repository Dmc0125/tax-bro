import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ locals }) => {
	return {
		username: locals.user.username,
		avatarUrl: locals.user.avatarUrl,
	};
};
