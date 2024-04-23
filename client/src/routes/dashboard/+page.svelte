<script lang="ts">
	import { enhance } from '$app/forms';

	let count = 0;
</script>

DASHBOARD

<a href="/logout">logout</a>

<form
	method="POST"
	action="?/add"
	use:enhance={({ formData }) => {
		const data = formData.getAll('address');
		console.log(data);
	}}
>
	<button
		class="h-10 bg-gray-900 text-slate-50"
		type="button"
		on:click={() => {
			count += 1;
		}}>New address</button
	>

	<div class="flex flex-col gap-y-5">
		{#each Array.from({ length: count + 1 }) as _}
			<input type="text" name="address" id="addresss" placeholder="Address" class="h-10" />
		{/each}
	</div>

	<button type="submit">Add</button>
</form>
