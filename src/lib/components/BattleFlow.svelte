<script lang="ts">
  import BattleLobby from './BattleLobby.svelte';
  import BattleRoom from './BattleRoom.svelte';
  import { BattleClient } from '../battle/client';
  import type { MatchInfo } from '../battle/types';

  let { playTone, onExit }: { playTone: (frequency: number, duration?: number, volume?: number) => void; onExit: () => void } = $props();

  let info = $state<MatchInfo | null>(null);
  let client = $state<BattleClient | null>(null);
</script>

{#if info && client}
  <BattleRoom {info} {client} {playTone} onExit={() => (info = null)} />
{:else}
  <BattleLobby onStart={(i, c) => { info = i; client = c; }} onBack={onExit} />
{/if}
