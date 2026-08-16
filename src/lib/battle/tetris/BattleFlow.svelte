<script lang="ts">
  import RoomList from '../RoomList.svelte';
  import BattleRoom from './BattleRoom.svelte';
  import { BattleClient } from '../client';
  import type { MatchInfo } from '../protocol';
  import type { BattlePlayerState } from './types';

  let { playTone, onExit }: { playTone: (frequency: number, duration?: number, volume?: number) => void; onExit: () => void } = $props();

  let info = $state<MatchInfo | null>(null);
  let client = $state<BattleClient<BattlePlayerState> | null>(null);
</script>

{#if info && client}
  <BattleRoom {info} {client} {playTone} onExit={() => (info = null)} />
{:else}
  <RoomList onStart={(i, c) => { info = i; client = c; }} onBack={onExit} />
{/if}
