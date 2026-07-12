<script lang="ts">
  import { ClipboardCheck, ClipboardCopy, Play, Shield } from '@lucide/svelte';
  import type { OpenApiOperation, OpenApiSpec, OpenApiParameter, OpenApiResponse } from '../openapi';
  import { methodColor, displaySummary, getServerUrl } from '../openapi';
  import SchemaView from './SchemaView.svelte';
  import RequestEditor from './RequestEditor.svelte';

  interface Props {
    operation: OpenApiOperation;
    spec: OpenApiSpec;
  }

  let { operation, spec }: Props = $props();

  let copied = $state(false);
  let copiedCurl = $state(false);
  let requestBody = $state('{}');
  let activeResponseTab = $state('200');

  const serverUrl = $derived(getServerUrl(spec));
  const fullUrl = $derived(`${serverUrl}${operation.path}`);
  const pathParams = $derived((operation.parameters ?? []).filter((p: OpenApiParameter) => p.in === 'path'));
  const queryParams = $derived((operation.parameters ?? []).filter((p: OpenApiParameter) => p.in === 'query'));
  const headerParams = $derived((operation.parameters ?? []).filter((p: OpenApiParameter) => p.in === 'header'));
  const responseCodes = $derived(Object.keys(operation.responses ?? {}).sort());
  const activeResponse = $derived(operation.responses?.[activeResponseTab]);
  const activeResponseSchema = $derived(responseSchema(activeResponseTab));

  function copyUrl() {
    void navigator.clipboard.writeText(fullUrl).then(() => {
      copied = true;
      setTimeout(() => (copied = false), 1500);
    });
  }

  function bodySchema(operation: OpenApiOperation): Record<string, unknown> | null {
    const content = operation.requestBody?.content ?? {};
    const json = content['application/json'];
    return json?.schema ?? null;
  }

  function responseSchema(code: string): Record<string, unknown> | null {
    const response = operation.responses?.[code];
    if (!response) return null;
    const content = response.content ?? {};
    return content['application/json']?.schema ?? null;
  }

  function exampleValue(schema: Record<string, unknown> | null): string {
    if (!schema) return '{}';
    if (schema.$ref) {
      const name = String(schema.$ref).split('/').pop();
      const def = spec.components?.schemas?.[name ?? ''];
      if (def) return JSON.stringify(exampleValue(def), null, 2);
      return '{}';
    }
    if (schema.type === 'object') {
      const obj: Record<string, unknown> = {};
      for (const [key, value] of Object.entries(schema.properties ?? {})) {
        obj[key] = exampleValue(value as Record<string, unknown>);
      }
      return JSON.stringify(obj, null, 2);
    }
    if (schema.type === 'array') {
      return JSON.stringify([exampleValue(schema.items as Record<string, unknown>)], null, 2);
    }
    if (schema.type === 'string') {
      if (schema.enum) return JSON.stringify((schema.enum as unknown[])[0] ?? 'string');
      return '"string"';
    }
    if (schema.type === 'integer' || schema.type === 'number') return '0';
    if (schema.type === 'boolean') return 'true';
    return '{}';
  }

  $effect(() => {
    const schema = bodySchema(operation);
    requestBody = exampleValue(schema);
  });

  function curlCommand(): string {
    const lines: string[] = [`curl -X ${operation.method.toUpperCase()} "${fullUrl}"`];
    lines.push('  -H "Content-Type: application/json"');
    if (operation.security?.length) {
      lines.push('  -H "Cookie: session=<your-session-cookie>"');
    }
    if (operation.requestBody) {
      lines.push(`  -d '${requestBody.replace(/'/g, "'\\''")}'`);
    }
    return lines.join(' \\\n');
  }

  function sendRequest() {
    alert('This demo UI does not send real requests. Use the curl command below.');
  }

  function copyCurl() {
    void navigator.clipboard.writeText(curlCommand()).then(() => {
      copiedCurl = true;
      setTimeout(() => (copiedCurl = false), 1500);
    });
  }
</script>

<article class="max-w-4xl">
  <header class="mb-8">
    <div class="flex items-center gap-3 mb-3">
      <span
        class="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-semibold uppercase tracking-wider border {methodColor(
          operation.method,
        )}"
      >
        {operation.method.toUpperCase()}
      </span>
      <h1 class="text-2xl font-semibold text-text-heading tracking-tight">{displaySummary(operation)}</h1>
    </div>

    {#if operation.description}
      <p class="text-text leading-relaxed">{operation.description}</p>
    {/if}

    <div class="mt-4 flex items-stretch gap-2">
      <div class="flex-1 font-mono text-sm bg-surface-inset border border-border rounded-lg px-3 py-2 break-all text-text-heading"
      >
        {fullUrl}
      </div>
      <button
        onclick={copyUrl}
        class="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg border border-border bg-surface hover:bg-surface-elevated transition-colors text-sm font-medium text-text-heading"
      >
        {#if copied}
          <ClipboardCheck class="w-4 h-4 text-get" />
          <span>Copied</span>
        {:else}
          <ClipboardCopy class="w-4 h-4" />
          <span>Copy</span>
        {/if}
      </button>
    </div>

    {#if operation.operationId}
      <div class="mt-3 flex flex-wrap gap-4 text-xs text-text-muted"
      >
        <span>operationId: <code class="text-text-heading">{operation.operationId}</code></span>
        {#if operation['x-middlewares']?.length}
          <span class="flex items-center gap-1"
          >
            <Shield class="w-3.5 h-3.5" />
            {operation['x-middlewares'].join(', ')}
          </span>
        {/if}
      </div>
    {/if}
  </header>

  {#if pathParams.length > 0}
    <section class="mb-8">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-text-muted mb-3">Path parameters</h2>
      <div class="rounded-lg border border-border overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-surface-inset text-text-muted">
            <tr>
              <th class="text-left px-4 py-2 font-medium">Name</th>
              <th class="text-left px-4 py-2 font-medium">Type</th>
              <th class="text-left px-4 py-2 font-medium">Description</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            {#each pathParams as param}
              <tr>
                <td class="px-4 py-2.5 font-mono text-text-heading">{param.name}</td>
                <td class="px-4 py-2.5">
                  <span class="text-xs uppercase tracking-wide text-text-muted"
                    >{param.schema?.type ?? 'string'}</span>
                </td>
                <td class="px-4 py-2.5 text-text-muted">{param.description ?? '—'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/if}

  {#if queryParams.length > 0}
    <section class="mb-8">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-text-muted mb-3">Query parameters</h2>
      <div class="rounded-lg border border-border overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-surface-inset text-text-muted">
            <tr>
              <th class="text-left px-4 py-2 font-medium">Name</th>
              <th class="text-left px-4 py-2 font-medium">Type</th>
              <th class="text-left px-4 py-2 font-medium">Required</th>
              <th class="text-left px-4 py-2 font-medium">Description</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            {#each queryParams as param}
              <tr>
                <td class="px-4 py-2.5 font-mono text-text-heading">{param.name}</td>
                <td class="px-4 py-2.5">
                  <span class="text-xs uppercase tracking-wide text-text-muted"
                  >{param.schema?.type ?? 'string'}</span>
                </td>
                <td class="px-4 py-2.5">
                  {#if param.required}
                    <span class="text-delete text-xs">required</span>
                  {:else}
                    <span class="text-text-muted text-xs">optional</span>
                  {/if}
                </td>
                <td class="px-4 py-2.5 text-text-muted">{param.description ?? '—'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/if}

  {#if headerParams.length > 0}
    <section class="mb-8">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-text-muted mb-3">Headers</h2>
      <div class="rounded-lg border border-border overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-surface-inset text-text-muted">
            <tr>
              <th class="text-left px-4 py-2 font-medium">Name</th>
              <th class="text-left px-4 py-2 font-medium">Type</th>
              <th class="text-left px-4 py-2 font-medium">Description</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            {#each headerParams as param}
              <tr>
                <td class="px-4 py-2.5 font-mono text-text-heading">{param.name}</td>
                <td class="px-4 py-2.5">
                  <span class="text-xs uppercase tracking-wide text-text-muted"
                  >{param.schema?.type ?? 'string'}</span>
                </td>
                <td class="px-4 py-2.5 text-text-muted">{param.description ?? '—'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/if}

  {#if operation.requestBody}
    {@const schema = bodySchema(operation)}
    <section class="mb-8">
      <div class="flex items-center justify-between mb-3">
        <h2 class="text-sm font-semibold uppercase tracking-wide text-text-muted">Request body</h2>
        {#if operation.requestBody.required}
          <span class="text-delete text-xs">required</span>
        {/if}
      </div>

      {#if schema}
        <div class="mb-4">
          <SchemaView {schema} />
        </div>
      {/if}

      <div class="rounded-lg border border-border overflow-hidden">
        <div class="bg-surface-inset px-3 py-2 border-b border-border flex items-center justify-between">
          <span class="text-xs font-medium text-text-muted">application/json</span>
          <button
            onclick={sendRequest}
            class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-accent text-white text-xs font-medium hover:bg-accent/90 transition-colors"
          >
            <Play class="w-3.5 h-3.5" />
            Try
          </button>
        </div>
        <div class="h-[220px] p-2 bg-surface">
          <RequestEditor bind:value={requestBody} />
        </div>
      </div>
    </section>
  {/if}

  {#if responseCodes.length > 0}
    <section class="mb-8">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-text-muted mb-3">Responses</h2>
      <div class="rounded-lg border border-border overflow-hidden">
        <div class="flex border-b border-border bg-surface-inset">
          {#each responseCodes as code}
            <button
              class="px-4 py-2 text-sm font-medium border-r border-border transition-colors {activeResponseTab === code
                ? 'bg-surface text-text-heading'
                : 'text-text-muted hover:text-text-heading'}"
              onclick={() => (activeResponseTab = code)}
            >
              {code}
            </button>
          {/each}
        </div>
        <div class="p-4">
          {#if activeResponse}
            <p class="text-text mb-4">{activeResponse.description}</p>
            {#if activeResponseSchema}
              <SchemaView schema={activeResponseSchema} />
            {:else}
              <div class="font-mono text-sm text-text-muted">No schema defined.</div>
            {/if}
          {/if}
        </div>
      </div>
    </section>
  {/if}

  <section class="mb-8">
    <div class="flex items-center justify-between mb-3">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-text-muted">cURL</h2>
      <button
        onclick={copyCurl}
        class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md border border-border bg-surface hover:bg-surface-elevated text-xs font-medium text-text-heading transition-colors"
      >
        {#if copiedCurl}
          <ClipboardCheck class="w-3.5 h-3.5 text-get" />
          <span>Copied</span>
        {:else}
          <ClipboardCopy class="w-3.5 h-3.5" />
          <span>Copy</span>
        {/if}
      </button>
    </div>
    <div class="rounded-lg border border-border bg-surface-inset p-4 overflow-x-auto">
      <code class="font-mono text-sm text-text-heading whitespace-pre"
          >{curlCommand()}</code>
    </div>
  </section>
</article>
