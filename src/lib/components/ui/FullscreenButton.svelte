<script lang="ts">
  import { onMount } from 'svelte';
  import { Maximize, Minimize } from 'lucide-svelte';
  import Button from './Button.svelte';

  let {
    variant = 'ghost',
    size = 'icon',
    class: className = ''
  }: {
    variant?: 'default' | 'secondary' | 'ghost' | 'outline';
    size?: 'default' | 'sm' | 'icon' | 'lg';
    class?: string;
  } = $props();

  let isFullscreen = $state(false);

  function toggle() {
    try {
      if (document.fullscreenElement) {
        document.exitFullscreen().catch(() => {});
      } else {
        document.documentElement.requestFullscreen().catch(() => {});
      }
    } catch {
      // 전체화면 API 미지원 환경은 무시
    }
  }

  onMount(() => {
    const onFs = () => {
      isFullscreen = !!document.fullscreenElement;
    };
    document.addEventListener('fullscreenchange', onFs);
    return () => document.removeEventListener('fullscreenchange', onFs);
  });
</script>

<Button {variant} {size} class={className} aria-label={isFullscreen ? '전체화면 종료' : '전체화면'} title={isFullscreen ? '전체화면 종료' : '전체화면'} onclick={toggle}>
  {#if isFullscreen}<Minimize size={18} />{:else}<Maximize size={18} />{/if}
</Button>
