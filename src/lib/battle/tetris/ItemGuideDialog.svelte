<script lang="ts">
  import { onMount } from 'svelte';
  import { X } from 'lucide-svelte';
  import Button from '../../components/ui/Button.svelte';
  import Card from '../../components/ui/Card.svelte';
  import { ITEM_LABELS, type BattleItemKind } from './types';

  let { open, onclose }: { open: boolean; onclose: () => void } = $props();

  /** 마커 색상과 동일 (ThreeBoard ITEM_COLORS와 일치) */
  const ITEM_COLORS: Record<string, string> = {
    attack: '#ff4d6d',
    speed: '#ffd166',
    holes: '#b07cff',
    clear: '#5be39a',
    shield: '#4dd8ff',
    slow: '#6db3ff'
  };
  /** 3D 마커 모양과 비슷한 글리프 */
  const ITEM_GLYPHS: Record<BattleItemKind, string> = {
    attack: '💣',
    speed: '▲',
    holes: '↔',
    clear: '✨',
    shield: '🛡',
    slow: '▼'
  };
  const KINDS = Object.keys(ITEM_LABELS) as BattleItemKind[];

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose();
  }

  onMount(() => {
    const cleanup = () => window.removeEventListener('keydown', onKeydown);
    window.addEventListener('keydown', onKeydown);
    return cleanup;
  });
</script>

{#if open}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-5 backdrop-blur-sm"
    role="presentation"
    onclick={(event) => event.target === event.currentTarget && onclose()}
  >
    <Card class="w-full max-w-lg p-6 shadow-2xl shadow-black/60">
      <div class="flex items-start justify-between">
        <div>
          <p class="text-[10px] font-bold tracking-[.25em] text-primary">ITEM GUIDE</p>
          <h2 class="mt-1 text-2xl font-black tracking-[-.04em]">아이템 소개</h2>
        </div>
        <Button variant="ghost" size="icon" aria-label="닫기" onclick={onclose}><X size={19} /></Button>
      </div>
      <p class="mt-2 text-xs leading-5 text-muted-foreground">
        보드에 숨겨진 아이템 셀은 <span class="text-white/80">색상 마커</span>로 보입니다. 아이템이 있는 줄을 지우면 발동돼요 — <span class="text-rose-300">공격 아이템</span>은 상대를 방해하고, <span class="text-emerald-300">도움 아이템</span>은 나를 돕습니다.
      </p>
      <div class="mt-5 grid grid-cols-1 gap-2.5 sm:grid-cols-2">
        {#each KINDS as kind}
          {@const label = ITEM_LABELS[kind]}
          <div class="flex items-start gap-3 rounded-xl border border-white/[.06] bg-white/[.025] p-3">
            <span
              class="mt-0.5 grid size-9 shrink-0 place-items-center rounded-lg text-base"
              style="background: {ITEM_COLORS[kind]}22; border: 1px solid {ITEM_COLORS[kind]}55; box-shadow: 0 0 12px {ITEM_COLORS[kind]}33;"
            >
              {ITEM_GLYPHS[kind]}
            </span>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-1.5">
                <strong class="text-xs font-bold text-white">{label.name}</strong>
                {#if label.good}
                  <span class="rounded bg-emerald-500/15 px-1 py-px text-[9px] font-bold tracking-wide text-emerald-300">도움</span>
                {:else}
                  <span class="rounded bg-rose-500/15 px-1 py-px text-[9px] font-bold tracking-wide text-rose-300">공격</span>
                {/if}
              </div>
              <p class="mt-0.5 text-[11px] leading-snug text-muted-foreground">{label.desc}</p>
            </div>
          </div>
        {/each}
      </div>
      <Button class="mt-6 w-full" onclick={onclose}>확인</Button>
    </Card>
  </div>
{/if}
