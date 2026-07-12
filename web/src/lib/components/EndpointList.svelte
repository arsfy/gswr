<script lang="ts">
  import { Search } from '@lucide/svelte';
  import { Collapsible } from 'bits-ui';
  import type { GroupedOperations, OpenApiOperation } from '../openapi';
  import { methodColor, operationSlug } from '../openapi';

  interface Props {
    groups: GroupedOperations[];
    selected: OpenApiOperation | null;
    onSelect: (operation: OpenApiOperation) => void;
  }

  let { groups, selected, onSelect }: Props = $props();

  let query = $state('');
  let expanded = $state<Set<string>>(new Set());

  const filteredGroups = $derived.by(() => {
    const q = query.trim().toLowerCase();
    if (!q) return groups;
    return groups
      .map((g) => ({
        ...g,
        operations: g.operations.filter(
          (op) =>
            op.path.toLowerCase().includes(q) ||
            op.method.toLowerCase().includes(q) ||
            (op.summary?.toLowerCase().includes(q) ?? false) ||
            (op.operationId?.toLowerCase().includes(q) ?? false),
        ),
      }))
      .filter((g) => g.operations.length > 0);
  });

  function isExpanded(tag: string) {
    return expanded.has(tag);
  }

  function setExpanded(tag: string, open: boolean) {
    const next = new Set(expanded);
    if (open) next.add(tag);
    else next.delete(tag);
    expanded = next;
  }

  $effect(() => {
    if (selected) {
      const tag = selected.tags?.[0] ?? 'untagged';
      if (!expanded.has(tag)) {
        expanded = new Set(expanded).add(tag);
      }
    }
  });

  function shortPath(path: string): string {
    if (path.length <= 42) return path;
    return path.slice(0, 19) + '…' + path.slice(-22);
  }
</script>

<div class="flex flex-col h-full">
  <div class="p-4 border-b border-border">
    <div class="relative">
      <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" />
      <input
        type="text"
        bind:value={query}
        placeholder="Search endpoints…"
        class="w-full pl-9 pr-3 py-2 rounded-lg border border-border bg-surface text-sm text-text-heading placeholder:text-text-muted focus:outline-none focus:border-accent-muted transition-colors"
      />
    </div>
  </div>

  <div class="flex-1 overflow-y-auto p-2 space-y-1">
    {#if filteredGroups.length === 0}
      <div class="px-3 py-8 text-center text-sm text-text-muted">No endpoints match "{query}".</div>
    {/if}

    {#each filteredGroups as group}
      <Collapsible.Root
        open={isExpanded(group.tag)}
        onOpenChange={(open) => setExpanded(group.tag, open)}
        class="rounded-lg border border-transparent overflow-hidden"
      >
        <Collapsible.Trigger
          class="w-full flex items-center justify-between px-3 py-2 text-sm font-medium text-text-heading hover:bg-surface-elevated rounded-lg transition-colors"
        >
          <span class="truncate">{group.tag}</span>
          <span class="text-xs text-text-muted tabular-nums ml-2">{group.operations.length}</span>
        </Collapsible.Trigger>

        <Collapsible.Content class="mt-0.5 space-y-0.5">
          {#each group.operations as op}
            <button
              class="w-full flex items-center gap-2 px-3 py-2 rounded-lg text-left transition-colors {selected &&
              operationSlug(selected) === operationSlug(op)
                ? 'bg-accent-subtle'
                : 'hover:bg-surface-elevated'}"
              onclick={() => onSelect(op)}
            >
              <span
                class="shrink-0 inline-flex items-center justify-center w-12 px-1.5 py-0.5 rounded text-[10px] font-bold uppercase tracking-wide border {methodColor(
                  op.method,
                )}"
              >
                {op.method.toUpperCase()}
              </span>
              <span class="truncate font-mono text-xs text-text-heading" title={op.path}
              >{shortPath(op.path)}</span>
            </button>
          {/each}
        </Collapsible.Content>
      </Collapsible.Root>
    {/each}
  </div>
</div>
