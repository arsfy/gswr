<script lang="ts">
  import { onMount, tick } from 'svelte';

  interface Props {
    value?: string;
    onChange?: (value: string) => void;
    readonly?: boolean;
    placeholder?: string;
  }

  let { value = $bindable('{}'), onChange, readonly = false, placeholder = '{}' }: Props = $props();

  let container: HTMLDivElement | undefined = $state();
  let view: import('@codemirror/view').EditorView | null = $state(null);

  async function initEditor() {
    if (!container || view) return;
    const [{ EditorView }, { EditorState }, { json }] = await Promise.all([
      import('@codemirror/view'),
      import('@codemirror/state'),
      import('@codemirror/lang-json'),
    ]);

    const state = EditorState.create({
      doc: value || placeholder,
      extensions: [
        json(),
        EditorView.lineWrapping,
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            value = update.state.doc.toString();
            onChange?.(value);
          }
        }),
        EditorView.theme({
          '&': { height: '100%' },
          '.cm-scroller': { fontFamily: 'var(--font-mono)' },
        }),
        EditorView.editable.of(!readonly),
      ],
    });

    view = new EditorView({ state, parent: container });
  }

  onMount(() => {
    void initEditor();
    return () => {
      view?.destroy();
      view = null;
    };
  });

  $effect(() => {
    if (view && value !== view.state.doc.toString()) {
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: value },
      });
    }
  });

  export function getValue() {
    return view?.state.doc.toString() ?? value;
  }
</script>

<div bind:this={container} class="h-full min-h-[180px] w-full rounded-lg overflow-hidden">
  <div class="h-full min-h-[180px] flex items-center justify-center text-text-muted text-sm"
  >
    Loading editor…
  </div>
</div>

<style>
  div :global(.cm-editor) {
    height: 100%;
  }
</style>
