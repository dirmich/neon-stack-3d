<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { ArrowLeft, ArrowDown, ArrowLeft as LeftIcon, ArrowRight as RightIcon, ChevronUp, LoaderCircle, RotateCcw, RotateCw, Swords, X } from 'lucide-svelte';
  import Button from '../../components/ui/Button.svelte';
  import Card from '../../components/ui/Card.svelte';
  import ThreeBoard from '../../components/ThreeBoard.svelte';
  import { BattleClient } from '../client';
  import type { BattleEvent, MatchInfo } from '../protocol';
  import { CLEAR_LABELS, ITEM_LABELS, type BattleItemKind, type BattlePlayerState } from './types';
  import { ARR_DELAY, DAS_DELAY } from '../../game/engine';
  import type { Piece } from '../../game/tetris';

  let {
    info,
    client,
    playTone,
    onExit
  }: {
    info: MatchInfo;
    client: BattleClient<BattlePlayerState>;
    playTone: (frequency: number, duration?: number, volume?: number) => void;
    onExit: () => void;
  } = $props();

  let you = $state<BattlePlayerState | null>(null);
  let opponent = $state<BattlePlayerState | null>(null);
  let result = $state<{ your_result: 'win' | 'loss' | 'draw'; your_score: number; opponent_score: number; forfeit?: boolean } | null>(null);
  let connected = $state(true);
  let feed = $state<{ id: number; text: string; tone: number; kind: 'clear' | 'good' | 'bad' }[]>([]);
  let feedId = 0;
  let ready = $state(false);

  const GAME_KEYS = [
    'ArrowLeft', 'ArrowRight', 'ArrowDown', 'ArrowUp', ' ', 'z', 'Z', 'c', 'C', 'x', 'X', 'w', 'W', 'a', 'A', 's', 'S', 'd', 'D', 'Shift', 'Escape', 'q', 'Q'
  ];

  function toPiece(state: BattlePlayerState): Piece {
    return { type: state.piece.t as Piece['type'], shape: state.piece.shape, x: state.piece.x, y: state.piece.y };
  }

  function boardStatus(state: BattlePlayerState): 'playing' | 'over' {
    return state.status === 'topout' ? 'over' : 'playing';
  }

  function pushFeed(event: BattleEvent, byName: string) {
    const label = event.clear ? CLEAR_LABELS[event.clear] : null;
    if (!label || event.attack <= 0) return;
    const text = `${byName} · ${label} +${event.attack}줄 공격!`;
    const tone = 760 + Math.min(event.attack, 6) * 90;
    const id = ++feedId;
    feed = [...feed.slice(-3), { id, text, tone, kind: 'clear' }];
    playTone(tone, 0.14, 0.05);
    setTimeout(() => {
      feed = feed.filter((f) => f.id !== id);
    }, 4200);
  }

  /** 아이템 발동 이벤트 피드 — 이로운 것은 초록, 악영향은 빨강 */
  function pushItemFeed(event: BattleEvent, byName: string) {
    const label = event.item ? ITEM_LABELS[event.item as BattleItemKind] : null;
    if (!label) return;
    const mine = event.by === info.player_id;
    const verb = label.good || mine ? '발동!' : '당했다!';
    const text = `${byName} · ${label.name} ${verb} (${label.desc})`;
    const tone = label.good ? 980 : 320;
    const id = ++feedId;
    feed = [...feed.slice(-3), { id, text, tone, kind: label.good ? 'good' : 'bad' }];
    playTone(tone, 0.12, 0.04);
    setTimeout(() => {
      feed = feed.filter((f) => f.id !== id);
    }, 4200);
  }

  // ---------- 입력 (키보드 + DAS 반복) ----------
  let repeatTimer: ReturnType<typeof setTimeout> | null = null;
  let softDrop = false;

  function stopRepeat() {
    if (repeatTimer) {
      clearTimeout(repeatTimer);
      repeatTimer = null;
    }
  }

  function startRepeat(dir: 'left' | 'right') {
    if (result) return;
    client.sendAction(dir);
    stopRepeat();
    repeatTimer = setTimeout(function tick() {
      client.sendAction(dir);
      repeatTimer = setTimeout(tick, ARR_DELAY);
    }, DAS_DELAY);
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!GAME_KEYS.includes(event.key)) return;
    event.preventDefault();
    if (result) return;
    switch (event.key) {
      case 'ArrowLeft':
      case 'a':
      case 'A':
        startRepeat('left');
        break;
      case 'ArrowRight':
      case 'd':
      case 'D':
        startRepeat('right');
        break;
      case 'ArrowDown':
      case 's':
      case 'S':
        if (!softDrop) {
          softDrop = true;
          client.sendAction('softdrop_start');
        }
        break;
      case 'ArrowUp':
      case 'w':
      case 'W':
        client.sendAction('rotate_cw');
        break;
      case 'x':
      case 'X':
        client.sendAction('rotate_cw');
        break;
      case 'z':
      case 'Z':
        client.sendAction('rotate_ccw');
        break;
      case ' ':
        client.sendAction('harddrop');
        break;
      case 'c':
      case 'C':
      case 'Shift':
        client.sendAction('hold');
        break;
      case 'Escape':
      case 'q':
      case 'Q':
        onExit();
        break;
    }
  }

  function handleKeyup(event: KeyboardEvent) {
    switch (event.key) {
      case 'ArrowLeft':
      case 'a':
      case 'A':
      case 'ArrowRight':
      case 'd':
      case 'D':
        stopRepeat();
        break;
      case 'ArrowDown':
      case 's':
      case 'S':
        softDrop = false;
        client.sendAction('softdrop_end');
        break;
    }
  }

  const touchRepeat = (dir: 'left' | 'right') => ({
    onpointerdown: () => startRepeat(dir),
    onpointerup: stopRepeat,
    onpointerleave: stopRepeat,
    onpointercancel: stopRepeat
  });

  const touchSoftDrop = {
    onpointerdown: () => {
      softDrop = true;
      client.sendAction('softdrop_start');
    },
    onpointerup: () => {
      softDrop = false;
      client.sendAction('softdrop_end');
    },
    onpointerleave: () => {
      softDrop = false;
      client.sendAction('softdrop_end');
    },
    onpointercancel: () => {
      softDrop = false;
      client.sendAction('softdrop_end');
    }
  };

  onMount(() => {
    client.onMessage = (msg) => {
      if (msg.type === 'state') {
        you = msg.you;
        opponent = msg.opponent;
        ready = true;
        const ownName = info.player_name;
        const oppName = info.opponent_name;
        for (const ev of msg.events) {
          if (ev.kind === 'clear' && ev.attack > 0) {
            pushFeed(ev, ev.by === info.player_id ? ownName : oppName);
          } else if (ev.kind === 'item' && ev.item) {
            pushItemFeed(ev, ev.by === info.player_id ? ownName : oppName);
          }
        }
      } else if (msg.type === 'gameover') {
        result = { your_result: msg.your_result, your_score: msg.your_score, opponent_score: msg.opponent_score, forfeit: msg.forfeit };
        if (msg.your_result === 'win') playTone(660, 0.4, 0.06);
        else if (msg.your_result === 'loss') playTone(180, 0.4, 0.05);
      } else if (msg.type === 'error') {
        connected = false;
      }
    };
    client.onClose = () => {
      // 게임오버 후 서버가 연결을 닫는 것은 정상 — 결과 화면에서는 "연결 끊김" 표시를 하지 않는다
      if (!result) connected = false;
    };
    window.addEventListener('keydown', handleKeydown);
    window.addEventListener('keyup', handleKeyup);
  });

  onDestroy(() => {
    window.removeEventListener('keydown', handleKeydown);
    window.removeEventListener('keyup', handleKeyup);
    stopRepeat();
    softDrop = false;
    client.onMessage = null;
    client.onClose = null;
    client.close();
  });
</script>

<div class="relative mx-auto flex w-full max-w-6xl flex-col px-4 pb-5 pt-2">
  <div class="mb-4 flex items-center justify-between">
    <Button variant="ghost" size="sm" class="text-muted-foreground" onclick={onExit}><ArrowLeft size={15} /> 나가기</Button>
    <div class="flex items-center gap-2 text-xs">
      <span class="flex size-2 rounded-full {connected ? 'bg-emerald-400' : 'bg-rose-400'}"></span>
      <span class="text-muted-foreground">{connected ? '연결됨' : '연결 끊김'}</span>
    </div>
    <div class="flex items-center gap-2 text-[10px] font-bold tracking-[.18em] text-muted-foreground">
      <Swords size={13} class="text-primary" />
      <span>BATTLE</span>
    </div>
  </div>

  {#if !ready}
    <div class="flex min-h-[50vh] flex-col items-center justify-center gap-4 text-muted-foreground">
      <LoaderCircle size={30} class="animate-spin text-primary" />
      <p class="text-sm">상대방과 연결하는 중...</p>
    </div>
  {:else if you && opponent}
    <!-- 모바일: 두 보드를 나란히 배치(카메라 핏이 어떤 비율에서도 전체를 보여줌),
         데스크톱: 기존처럼 전체 높이로 나란히 -->
    <div class="grid grid-cols-2 gap-3 md:gap-4">
      <!-- 상대 보드 -->
      <div class="flex min-w-0 flex-col gap-1.5 md:gap-2">
        <div class="flex items-center justify-between gap-1 px-0.5">
          <div class="flex min-w-0 items-center gap-1.5">
            <span class="size-2 shrink-0 rounded-full bg-rose-400"></span>
            <span class="truncate text-xs font-bold text-white sm:text-sm">{info.opponent_name}</span>
            <span class="hidden rounded bg-white/[.06] px-1.5 py-0.5 text-[9px] font-bold tracking-[.14em] text-white/35 sm:inline-flex">OPPONENT</span>
          </div>
          <div class="flex shrink-0 items-center gap-1.5 font-mono text-[10px] text-muted-foreground sm:gap-3 sm:text-xs">
            <span>{opponent.score.toLocaleString()}</span>
            {#if opponent.garbage > 0}
              <span class="rounded-full border border-rose-400/30 bg-rose-500/15 px-1.5 py-0.5 text-[9px] font-bold text-rose-300">+{opponent.garbage}</span>
            {/if}
            {#if opponent.shield}
              <span class="rounded-full border border-cyan-400/30 bg-cyan-500/15 px-1.5 py-0.5 text-[9px] font-bold text-cyan-300" title="방패 {opponent.shield}줄">🛡{opponent.shield}</span>
            {/if}
            {#if opponent.speed}
              <span class="rounded-full border border-yellow-400/30 bg-yellow-500/15 px-1.5 py-0.5 text-[9px] font-bold text-yellow-300" title="중력 가속">⚡</span>
            {/if}
            {#if opponent.slow}
              <span class="rounded-full border border-blue-400/30 bg-blue-500/15 px-1.5 py-0.5 text-[9px] font-bold text-blue-300" title="중력 감속">🐢</span>
            {/if}
          </div>
        </div>
        <Card class="relative h-[30dvh] min-h-[200px] overflow-hidden bg-[#0c0f17]/75 p-2 md:h-[calc(100dvh-200px)] md:min-h-[430px]">
          <!-- 상대 보드도 드래그로 회전 가능 — 각 보드가 독립된 카메라/OrbitControls를 가져 각도가 독립적으로 유지된다 -->
          <ThreeBoard board={opponent.board} items={opponent.items} active={toPiece(opponent)} status={boardStatus(opponent)} clearFlash={opponent.clear_flash} interactive showHint={false} />
          {#if opponent.status === 'topout'}
            <div class="absolute inset-2 z-10 flex items-center justify-center rounded-[1.35rem] bg-[#070910]/70 backdrop-blur-[3px]">
              <span class="rounded-full border border-rose-400/30 bg-rose-500/15 px-4 py-1.5 text-xs font-bold tracking-[.2em] text-rose-300">TOP OUT</span>
            </div>
          {/if}
        </Card>
      </div>

      <!-- 내 보드 -->
      <div class="flex min-w-0 flex-col gap-1.5 md:gap-2">
        <div class="flex items-center justify-between gap-1 px-0.5">
          <div class="flex min-w-0 items-center gap-1.5">
            <span class="size-2 shrink-0 rounded-full bg-primary"></span>
            <span class="truncate text-xs font-bold text-white sm:text-sm">{info.player_name}</span>
            <span class="hidden rounded bg-primary/15 px-1.5 py-0.5 text-[9px] font-bold tracking-[.14em] text-primary sm:inline-flex">YOU</span>
          </div>
          <div class="flex shrink-0 items-center gap-1.5 font-mono text-[10px] text-muted-foreground sm:gap-3 sm:text-xs">
            <span>{you.score.toLocaleString()}</span>
            {#if you.garbage > 0}
              <span class="rounded-full border border-rose-400/30 bg-rose-500/15 px-1.5 py-0.5 text-[9px] font-bold text-rose-300">+{you.garbage}</span>
            {/if}
            {#if you.shield}
              <span class="rounded-full border border-cyan-400/30 bg-cyan-500/15 px-1.5 py-0.5 text-[9px] font-bold text-cyan-300" title="방패 {you.shield}줄">🛡{you.shield}</span>
            {/if}
            {#if you.speed}
              <span class="rounded-full border border-yellow-400/30 bg-yellow-500/15 px-1.5 py-0.5 text-[9px] font-bold text-yellow-300" title="중력 가속">⚡</span>
            {/if}
            {#if you.slow}
              <span class="rounded-full border border-blue-400/30 bg-blue-500/15 px-1.5 py-0.5 text-[9px] font-bold text-blue-300" title="중력 감속">🐢</span>
            {/if}
          </div>
        </div>
        <Card class="relative h-[30dvh] min-h-[200px] overflow-hidden border-primary/15 bg-[#0c0f17]/75 p-2 md:h-[calc(100dvh-200px)] md:min-h-[430px]">
          <ThreeBoard board={you.board} items={you.items} active={toPiece(you)} status={boardStatus(you)} clearFlash={you.clear_flash} />
          {#if you.status === 'topout'}
            <div class="absolute inset-2 z-10 flex items-center justify-center rounded-[1.35rem] bg-[#070910]/70 backdrop-blur-[3px]">
              <span class="rounded-full border border-rose-400/30 bg-rose-500/15 px-4 py-1.5 text-xs font-bold tracking-[.2em] text-rose-300">TOP OUT</span>
            </div>
          {/if}
        </Card>
      </div>
    </div>

    <!-- 터치 컨트롤 -->
    <div class="mt-2 flex flex-wrap items-center justify-center gap-1.5 md:hidden">
      <Button variant="outline" size="icon" aria-label="왼쪽" {...touchRepeat('left')}><LeftIcon size={18} /></Button>
      <Button variant="outline" size="icon" aria-label="아래로" {...touchSoftDrop}><ArrowDown size={18} /></Button>
      <Button variant="outline" size="icon" aria-label="오른쪽" {...touchRepeat('right')}><RightIcon size={18} /></Button>
      <Button variant="outline" size="icon" aria-label="시계 방향 회전" onpointerdown={() => client.sendAction('rotate_cw')}><RotateCw size={18} /></Button>
      <Button variant="outline" size="icon" aria-label="반시계 방향 회전" onpointerdown={() => client.sendAction('rotate_ccw')}><RotateCcw size={18} /></Button>
      <Button variant="outline" size="icon" aria-label="바로 내리기" onpointerdown={() => client.sendAction('harddrop')}><ChevronUp size={18} /></Button>
      <Button variant="outline" size="icon" aria-label="보관" onpointerdown={() => client.sendAction('hold')}>H</Button>
    </div>
  {/if}

  <!-- 공격 피드 -->
  {#if feed.length > 0}
    <div class="pointer-events-none absolute right-5 top-16 z-20 flex flex-col items-end gap-2">
      {#each feed as item (item.id)}
        <div
          class="animate-[feedIn_.25s_ease-out] rounded-xl border bg-black/70 px-4 py-2 text-sm font-bold shadow-lg shadow-black/30 backdrop-blur-md
            {item.kind === 'clear' ? 'border-primary/25 text-primary' : item.kind === 'good' ? 'border-emerald-400/30 text-emerald-300' : 'border-rose-400/30 text-rose-300'}"
        >
          {item.text}
        </div>
      {/each}
    </div>
  {/if}

  <!-- 게임오버 오버레이 -->
  {#if result}
    <div class="absolute inset-0 z-30 flex items-center justify-center bg-[#070910]/82 p-6 backdrop-blur-md">
      <div class="w-full max-w-sm rounded-3xl border border-white/10 bg-[#0d1019]/95 p-8 text-center shadow-2xl shadow-black/50">
        <p class="text-[10px] font-bold tracking-[.3em] text-muted-foreground">MATCH OVER</p>
        <h2 class="mt-2 text-4xl font-black tracking-[-.05em]
          {result.your_result === 'win' ? 'text-primary' : result.your_result === 'loss' ? 'text-rose-400' : 'text-white'}">
          {result.your_result === 'win' ? '승리!' : result.your_result === 'loss' ? '패배' : '무승부'}
        </h2>
        {#if result.forfeit}
          <p class="mt-2 text-xs text-muted-foreground">상대방이 연결을 끊었습니다.</p>
        {/if}
        <div class="mt-6 flex items-center justify-center gap-6 font-mono">
          <div>
            <p class="text-[10px] font-bold tracking-[.2em] text-muted-foreground">YOU</p>
            <p class="mt-1 text-2xl font-black text-white">{result.your_score.toLocaleString()}</p>
          </div>
          <span class="text-xl text-white/20">VS</span>
          <div>
            <p class="text-[10px] font-bold tracking-[.2em] text-muted-foreground">{info.opponent_name}</p>
            <p class="mt-1 text-2xl font-black text-white">{result.opponent_score.toLocaleString()}</p>
          </div>
        </div>
        <Button size="lg" class="mt-8 w-full" onclick={onExit}><X size={16} /> 대기실로</Button>
      </div>
    </div>
  {/if}
</div>

<style>
  @keyframes feedIn {
    from {
      opacity: 0;
      transform: translateY(6px) scale(0.96);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }
</style>
