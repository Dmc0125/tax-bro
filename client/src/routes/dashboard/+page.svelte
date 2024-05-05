<script lang="ts">
	import { enhance } from '$app/forms';
	import type { PageData } from './$types';

	export let data: PageData;

	let count = 0;
</script>

<main class="w-1/2 mx-auto">
	<form method="POST" action="?/add" use:enhance>
		<button
			class="btn"
			type="button"
			on:click={() => {
				count += 1;
			}}>New address</button
		>

		<div class="flex flex-col gap-y-5">
			<input type="text" name="address" id="addresss" placeholder="Address" class="input" />

			{#each Array.from({ length: count }) as _}
				<input type="text" name="address" id="addresss" placeholder="Address" class="input" />
			{/each}
		</div>

		<button type="submit" class="btn variant-filled">Add</button>
	</form>

	<div class="mt-10">
		<h4 class="h4">Saved wallets</h4>

		<div class="table-container mt-5">
			<table class="table">
				<thead>
					<tr>
						<th></th>
						<th>Address</th>
						<th>Signatures</th>
						<th>Status</th>
					</tr>
				</thead>
				<tbody>
					{#each data.wallets as wallet, i}
						<tr>
							<td>{i + 1}</td>
							<td>
								{wallet.address}
							</td>
							<td>{wallet.signatures_count}</td>
							<td>{wallet.signatures_status}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
</main>
