<script lang="ts">
  import { onDestroy } from 'svelte';
  import { ArrowLeft, Copy, LoaderCircle, Swords, Users } from 'lucide-svelte';
  import Button from './ui/Button.svelte';
  import Card from './ui/Card.svelte';
  import { BattleClient } from '../battle/client';
  import type { MatchInfo, MatchCreateResponse } from '../battle/types';

  let { onStart, onBack }: { onStart: (info: MatchInfo, client: BattleClient) => void; onBack: () => void } = $props();

  let name = $state('플레이어' + Math.floor(100 + Math.random() * 900));
  let mode = $state<'create' | 'join'>('create');
  let joinCode = $state('');
  let error = $state<string | null>(null);
  let waiting = $state(false);
  let room: MatchCreateResponse | null = $state(null);
  let copied = $state(false);

  const client = new BattleClient();
  let started = false;
  // 시작되면 클라이언트를 BattleRoom으로 넘기므로 onDestroy에서 닫으면 안 된다
  let handedOff = false;

  client.onClose = () => {
    if (waiting && !started) {
      error = '서버 연결이 끊어졌습니다.';
      waiting = false;
    }
  };

  async function createRoom() {
    error = null;
    if (!name.trim()) {
      error = '이름을 입력해 주세요.';
      return;
    }
    try {
      const res = await fetch('/api/matches', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name.trim() })
      });
      if (!res.ok) throw new Error((await res.json()).error ?? '방 생성 실패');
      room = (await res.json()) as MatchCreateResponse;
      waiting = true;
      client.onMessage = (msg) => {
        if (msg.type === 'start') {
          started = true;
          handedOff = true;
          onStart(
            {
              match_id: msg.match_id,
              player_id: room!.player_id,
              player_name: room!.player_name,
              opponent_name: msg.opponent_name,
              your_index: msg.your_index
            },
            client
          );
        } else if (msg.type === 'error') {
          error = msg.message;
        }
      };
      client.connect(room.match_id, room.player_id);
    } catch (e) {
      error = e instanceof Error ? e.message : '방 생성 실패';
    }
  }

  async function joinRoom() {
    error = null;
    if (!name.trim() || !joinCode.trim()) {
      error = '이름과 코드를 입력해 주세요.';
      return;
    }
    try {
      const res = await fetch('/api/matches/join', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: joinCode.trim(), name: name.trim() })
      });
      if (!res.ok) throw new Error((await res.json()).error ?? '참가 실패');
      room = (await res.json()) as MatchCreateResponse;
      waiting = true;
      client.onMessage = (msg) => {
        if (msg.type === 'start') {
          started = true;
          handedOff = true;
          onStart(
            {
              match_id: msg.match_id,
              player_id: room!.player_id,
              player_name: room!.player_name,
              opponent_name: msg.opponent_name,
              your_index: msg.your_index
            },
            client
          );
        } else if (msg.type === 'error') {
          error = msg.message;
        }
      };
      client.connect(room.match_id, room.player_id);
    } catch (e) {
      error = e instanceof Error ? e.message : '참가 실패';
    }
  }

  async function copyCode() {
    if (!room) return;
    try {
      await navigator.clipboard.writeText(room.code);
      copied = true;
      setTimeout(() => (copied = false), 1500);
    } catch {
      /* 클립보드 미지원 환경 */
    }
  }

  function cancelWait() {
    client.close();
    started = false;
    waiting = false;
    room = null;
    error = null;
  }

  onDestroy(() => {
    if (!handedOff) client.close();
  });
</script>

<div class="mx-auto flex w-full max-w-3xl flex-col px-4 pb-8 pt-2">
  <div class="mb-6 flex items-center justify-between">
    <div class="flex items-center gap-3">
      <Button variant="ghost" size="icon" aria-label="메인 메뉴로" onclick={onBack}>
        <ArrowLeft size={18} />
      </Button>
      <div class="flex items-center gap-2.5">
        <span class="flex size-9 items-center justify-center rounded-xl border border-primary/25 bg-primary/10 text-primary">
          <Swords size={17} />
        </span>
        <div>
          <p class="text-[10px] font-bold tracking-[.24em] text-primary">2-PLAYER BATTLE</p>
          <h2 class="text-lg font-black tracking-[-.03em] text-white">배틀 모드</h2>
        </div>
      </div>
    </div>
  </div>

  {#if waiting && room}
    <Card class="mx-auto w-full max-w-md p-8 text-center">
      <div class="mx-auto mb-5 flex size-14 items-center justify-center rounded-2xl border border-primary/25 bg-primary/10">
        <LoaderCircle size={26} class="animate-spin text-primary" />
      </div>
      <p class="text-[10px] font-bold tracking-[.3em] text-primary">WAITING FOR OPPONENT</p>
      <h3 class="mt-2 text-2xl font-black tracking-[-.04em] text-white">
        {mode === 'create' ? '상대방을 기다리는 중' : '방에 입장하는 중'}
      </h3>
      {#if mode === 'create'}
        <p class="mt-4 text-sm text-muted-foreground">친구에게 아래 코드를 알려주세요.</p>
        <button
          class="mx-auto mt-4 flex items-center gap-3 rounded-2xl border border-white/10 bg-black/30 px-8 py-4 font-mono text-4xl font-black tracking-[.3em] text-primary transition hover:border-primary/40 hover:bg-primary/5"
          onclick={copyCode}
          aria-label="코드 복사"
        >
          {room.code}
          <Copy size={20} class="text-white/30" />
        </button>
        {#if copied}
          <p class="mt-2 text-xs text-cyan-300">복사되었습니다!</p>
        {/if}
      {/if}
      <Button variant="ghost" size="sm" class="mt-8 text-muted-foreground" onclick={cancelWait}>취소</Button>
    </Card>
  {:else}
    <div class="grid gap-4 md:grid-cols-2">
      <Card class="p-6">
        <div class="mb-4 flex items-center gap-2 text-muted-foreground">
          <Swords size={15} />
          <span class="text-[10px] font-bold tracking-[.2em]">방 만들기</span>
        </div>
        <p class="text-sm leading-6 text-muted-foreground">새 배틀 방을 만들고 코드를 공유하세요.</p>
        <input
          bind:value={name}
          class="mt-4 w-full rounded-xl border border-white/10 bg-black/25 px-3.5 py-2.5 text-sm text-white outline-none transition focus:border-primary/50"
          placeholder="플레이어 이름"
          maxlength="16"
        />
        <Button size="lg" class="mt-5 w-full" onclick={createRoom}><Swords size={16} /> 방 생성</Button>
      </Card>

      <Card class="p-6">
        <div class="mb-4 flex items-center gap-2 text-muted-foreground">
          <Users size={15} />
          <span class="text-[10px] font-bold tracking-[.2em]">코드로 참가</span>
        </div>
        <p class="text-sm leading-6 text-muted-foreground">친구의 4자리 코드를 입력해 참가하세요.</p>
        <input
          bind:value={name}
          class="mt-4 w-full rounded-xl border border-white/10 bg-black/25 px-3.5 py-2.5 text-sm text-white outline-none transition focus:border-primary/50"
          placeholder="플레이어 이름"
          maxlength="16"
        />
        <input
          bind:value={joinCode}
          class="mt-2.5 w-full rounded-xl border border-white/10 bg-black/25 px-3.5 py-2.5 font-mono text-lg font-bold tracking-[.25em] text-white uppercase outline-none transition focus:border-primary/50"
          placeholder="ABCD"
          maxlength="4"
          onkeydown={(e) => {
            if (e.key === 'Enter') joinRoom();
          }}
        />
        <Button size="lg" variant="outline" class="mt-5 w-full" onclick={joinRoom}><Users size={16} /> 참가</Button>
      </Card>
    </div>

    {#if error}
      <p class="mx-auto mt-5 rounded-xl border border-rose-400/20 bg-rose-500/10 px-4 py-2.5 text-center text-sm text-rose-300">{error}</p>
    {/if}
  {/if}
</div>
