<script lang="ts">
	let { form } = $props();
</script>

<svelte:head><title>Sign in</title></svelte:head>
<main>
	<h1>Sign in to your household</h1>
	<p>Enter the session key supplied by the playtest host.</p>
	<form method="POST">
		<label for="session">Session key</label>
		<input id="session" name="session" type="password" autocomplete="off" required />
		<button type="submit">Sign in</button>
		{#if form?.message}<p role="alert">{form.message}</p>{/if}
	</form>
</main>
