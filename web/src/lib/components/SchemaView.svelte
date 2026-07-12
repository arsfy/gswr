<script lang="ts">
  import SchemaView from './SchemaView.svelte';
  import { resolveSchemaRef } from '../openapi';

  interface Props {
    schema: Record<string, unknown>;
    schemas?: Record<string, Record<string, unknown>>;
    name?: string;
    depth?: number;
  }

  let { schema, schemas = {}, name, depth = 0 }: Props = $props();

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

  const resolvedSchema = $derived(resolveSchemaRef(schema, schemas));
  const requiredFields = $derived(
    Array.isArray(resolvedSchema.required) ? (resolvedSchema.required as string[]) : [],
  );

  const entries = $derived(
    Object.entries(resolvedSchema.properties ?? {})
      .sort(([a], [b]) => {
        const ar = requiredFields.includes(a) ? 0 : 1;
        const br = requiredFields.includes(b) ? 0 : 1;
        return ar - br;
      })
  );

  const children = $derived.by(() => {
    if ((resolvedSchema.type === 'object' || resolvedSchema.properties) && entries.length > 0) {
      return entries.map(([key, value]) => ({ key, value: value as Record<string, unknown> }));
    }
    if (resolvedSchema.type === 'array' && isObject(resolvedSchema.items)) {
      return [{ key: 'items', value: resolvedSchema.items }];
    }
    return [];
  });

  function hasChildren(value: Record<string, unknown>): boolean {
    const resolved = resolveSchemaRef(value, schemas);
    return Boolean(
      value.$ref || resolved.type === 'object' || resolved.type === 'array' || resolved.properties,
    );
  }
</script>

{#if schema.$ref && depth >= 8}
  <span class="font-mono text-sm text-accent">{formatRef(String(schema.$ref))}</span>
{:else if (resolvedSchema.type === 'object' || resolvedSchema.properties) && children.length > 0}
  <div class="font-mono text-sm" style="margin-left: {depth > 0 ? 16 : 0}px">
    {#if name}
      <div class="mb-1 text-text-heading">{name}</div>
    {/if}
    <div class="rounded-lg border border-border bg-surface p-3">
      {#each children as { key, value }}
        <div class="py-1.5 border-b border-border last:border-b-0">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-text-heading text-xs font-medium">{key}</span>
            <span class="text-text-muted text-xs uppercase tracking-wide">{typeLabel(value)}</span>
            {#if requiredFields.includes(key)}
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
          {#if isObject(value) && hasChildren(value)}
            <div class="mt-2">
              <SchemaView schema={value} {schemas} depth={depth + 1} />
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </div>
{:else if resolvedSchema.type === 'array' && children.length > 0}
  <div class="font-mono text-sm" style="margin-left: {depth > 0 ? 16 : 0}px">
    <div class="text-text-muted mb-1">array of:</div>
    <SchemaView schema={resolvedSchema.items as Record<string, unknown>} {schemas} depth={depth + 1} />
  </div>
{:else}
  <div class="font-mono text-sm text-text-muted">
    {#if name}
      <span class="text-text-heading">{name}</span>
      <span class="ml-2">{typeLabel(resolvedSchema)}</span>
    {:else}
      {typeLabel(resolvedSchema)}
    {/if}
  </div>
{/if}
