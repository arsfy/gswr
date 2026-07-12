<script lang="ts">
  import SchemaView from './SchemaView.svelte';

  interface Props {
    schema: Record<string, unknown>;
    required?: string[];
    name?: string;
    depth?: number;
  }

  let { schema, required = [], name, depth = 0 }: Props = $props();

  function isObject(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null && !Array.isArray(value);
  }

  function typeLabel(value: Record<string, unknown>): string {
    const t = value.type;
    if (typeof t === 'string') return t;
    if (Array.isArray(t)) return t.join(' | ');
    if (value.enum) return 'enum';
    if (value.$ref) return String(value.$ref).split('/').pop() ?? 'ref';
    return 'any';
  }

  function enumItems(value: Record<string, unknown>): unknown[] {
    const e = value.enum;
    return Array.isArray(e) ? e : [];
  }

  function formatRef(ref: string): string {
    return ref.split('/').pop() ?? ref;
  }

  const entries = $derived(
    Object.entries(schema.properties ?? {})
      .sort(([, a], [, b]) => {
        const ar = required.includes(a as unknown as string) ? 0 : 1;
        const br = required.includes(b as unknown as string) ? 0 : 1;
        return ar - br;
      })
  );

  const children = $derived.by(() => {
    if (schema.type === 'object' && entries.length > 0) {
      return entries.map(([key, value]) => ({ key, value: value as Record<string, unknown> }));
    }
    if (schema.type === 'array' && isObject(schema.items)) {
      return [{ key: 'items', value: schema.items }];
    }
    return [];
  });
</script>

{#if schema.$ref}
  <span class="font-mono text-sm text-accent">{formatRef(String(schema.$ref))}</span>
{:else if schema.type === 'object' && children.length > 0}
  <div class="font-mono text-sm" style="margin-left: {depth > 0 ? 16 : 0}px">
    {#if name}
      <div class="mb-1 text-text-heading">{name}</div>
    {/if}
    <div class="rounded-lg border border-border bg-surface p-3">
      {#each children as { key, value }}
        <div class="py-1.5 border-b border-border last:border-b-0">
          <div class="flex items-start gap-2 flex-wrap">
            <span class="text-text-heading font-medium">{key}</span>
            <span class="text-text-muted text-xs uppercase tracking-wide">{typeLabel(value)}</span>
            {#if required.includes(key)}
              <span class="text-delete text-xs">required</span>
            {/if}
          </div>
          {#if value.description}
            <div class="text-text-muted text-xs mt-0.5">{String(value.description)}</div>
          {/if}
          {#if enumItems(value).length > 0}
            <div class="flex flex-wrap gap-1 mt-1">
              {#each enumItems(value) as item (item)}
                <span class="text-xs px-1.5 py-0.5 rounded bg-surface-inset text-text-muted border border-border"
                >{JSON.stringify(item)}</span>
              {/each}
            </div>
          {/if}
          {#if (isObject(value) && (value.type === 'object' || value.type === 'array'))}
            <div class="mt-2">
              <SchemaView schema={value} required={Array.isArray(value.required) ? value.required as string[] : []} depth={depth + 1} />
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </div>
{:else if schema.type === 'array' && children.length > 0}
  <div class="font-mono text-sm" style="margin-left: {depth > 0 ? 16 : 0}px">
    <div class="text-text-muted mb-1">array of:</div>
    <SchemaView schema={schema.items as Record<string, unknown>} depth={depth + 1} />
  </div>
{:else}
  <div class="font-mono text-sm text-text-muted">
    {#if name}
      <span class="text-text-heading">{name}</span>
      <span class="ml-2">{typeLabel(schema)}</span>
    {:else}
      {typeLabel(schema)}
    {/if}
  </div>
{/if}
