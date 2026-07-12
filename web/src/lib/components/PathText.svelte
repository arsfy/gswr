<script lang="ts">
  interface Props {
    value: string;
  }

  let { value }: Props = $props();

  const parts = $derived(value.split(/(\{[^{}]+\})/).filter(Boolean));

  function isParameter(part: string): boolean {
    return part.startsWith('{') && part.endsWith('}');
  }
</script>

{#each parts as part}
  {#if isParameter(part)}
    <span class="rounded bg-accent-subtle p-0.5 mx-0.5 font-semibold text-accent">{part}</span>
  {:else}
    {part}
  {/if}
{/each}
