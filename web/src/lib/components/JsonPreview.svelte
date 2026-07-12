<script lang="ts">
  interface Props {
    value: string;
  }

  type TokenType = 'key' | 'string' | 'number' | 'boolean' | 'null' | 'punctuation' | 'plain';

  interface Token {
    value: string;
    type: TokenType;
  }

  let { value }: Props = $props();

  const tokenPattern = /"(?:\\.|[^"\\])*"|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|\b(?:true|false|null)\b|[{}[\],:]|\s+|./g;

  function tokenType(token: string, rest: string): TokenType {
    if (token.startsWith('"')) return /^\s*:/.test(rest) ? 'key' : 'string';
    if (/^-?\d/.test(token)) return 'number';
    if (token === 'true' || token === 'false') return 'boolean';
    if (token === 'null') return 'null';
    if (/^[{}[\],:]$/.test(token)) return 'punctuation';
    return 'plain';
  }

  function tokenize(line: string): Token[] {
    return Array.from(line.matchAll(tokenPattern), (match) => ({
      value: match[0],
      type: tokenType(match[0], line.slice((match.index ?? 0) + match[0].length)),
    }));
  }

  const lines = $derived(value.split('\n').map(tokenize));
</script>

<pre class="overflow-x-auto bg-surface p-4 font-mono text-sm leading-relaxed"><code>{#each lines as line, index}{#each line as token}<span class="json-{token.type}">{token.value}</span>{/each}{#if index < lines.length - 1}{'\n'}{/if}{/each}</code></pre>

<style>
  .json-key {
    color: var(--color-accent);
  }

  .json-string {
    color: var(--color-get);
  }

  .json-number {
    color: var(--color-post);
  }

  .json-boolean {
    color: var(--color-put);
  }

  .json-null {
    color: var(--color-delete);
    font-style: italic;
  }

  .json-punctuation,
  .json-plain {
    color: var(--color-text-heading);
  }
</style>
