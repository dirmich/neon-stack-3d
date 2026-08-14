<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    children,
    variant = 'default',
    size = 'default',
    class: className = '',
    type = 'button',
    ...rest
  }: {
    children: Snippet;
    variant?: 'default' | 'secondary' | 'ghost' | 'outline';
    size?: 'default' | 'sm' | 'icon' | 'lg';
    class?: string;
    type?: 'button' | 'submit' | 'reset';
    [key: string]: unknown;
  } = $props();

  const variants = {
    default: 'bg-primary text-primary-foreground hover:bg-[#e5ff7c] shadow-[0_0_24px_rgba(215,255,69,.16)]',
    secondary: 'bg-white/[.07] text-white hover:bg-white/[.11]',
    ghost: 'text-muted-foreground hover:bg-white/[.06] hover:text-white',
    outline: 'border border-white/10 bg-white/[.025] text-white hover:border-white/20 hover:bg-white/[.06]'
  };

  const sizes = {
    default: 'h-10 px-4 py-2',
    sm: 'h-8 rounded-lg px-3 text-xs',
    icon: 'size-10 p-0',
    lg: 'h-12 rounded-xl px-6'
  };
</script>

<button
  {type}
  class={`inline-flex shrink-0 cursor-pointer items-center justify-center gap-2 rounded-xl text-sm font-bold tracking-wide transition-all duration-200 disabled:pointer-events-none disabled:opacity-40 active:scale-[.97] ${variants[variant]} ${sizes[size]} ${className}`}
  {...rest}
>
  {@render children()}
</button>
