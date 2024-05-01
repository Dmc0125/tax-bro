<script lang="ts">
	import { AppBar, Avatar, ListBox, ListBoxItem, popup } from '@skeletonlabs/skeleton';

	import type { LayoutData } from './$types';

	export let data: LayoutData;
</script>

<AppBar>
	<svelte:fragment slot="lead">
		<div class="w-full"></div>
	</svelte:fragment>
	<svelte:fragment slot="trail">
		<div class="flex items-center gap-x-6">
			<h5 class="h5">{data.username}</h5>
			<button
				use:popup={{
					event: 'click',
					placement: 'bottom',
					target: 'nav-popup',
					closeQuery: '.listbox-item',
				}}
			>
				<Avatar src={data.avatarUrl} rounded="rounded-full" width="w-10"></Avatar>
			</button>
			<div class="card w-48" data-popup="nav-popup">
				<nav class="list-nav">
					<ul>
						<li>
							<a href="/logout">Logout</a>
						</li>
					</ul>
				</nav>
			</div>
		</div>
	</svelte:fragment>
</AppBar>

<slot></slot>
