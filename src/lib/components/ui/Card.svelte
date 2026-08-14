<script lang="ts">
  import type { Snippet } from 'svelte';

  let { children, class: className = '' }: { children: Snippet; class?: string } = $props();
</script>

<section class={`rounded-2xl border border-white/[.08] bg-[#10131c]/85 shadow-2xl shadow-black/20 backdrop-blur-xl ${className}`}>
  {@render children()}
</section>
