<script lang="ts" generics="S">
  import { onDestroy, onMount } from 'svelte';
  import { ArrowLeft, Bot, Copy, Crown, LoaderCircle, Plus, RefreshCw, Sparkles, Swords, Trophy, Users, X } from 'lucide-svelte';
  import Button from '../components/ui/Button.svelte';
  import Card from '../components/ui/Card.svelte';
  import ItemGuideDialog from './tetris/ItemGuideDialog.svelte';
  import { BattleClient } from './client';
  import type { LeaderboardEntry, LeaderboardResponse, MatchCreateResponse, MatchInfo, RoomRow } from './protocol';
  import { createRoom, createSoloRoom, fetchLeaderboard, joinRoom, listRooms } from './rooms';
  import { getToken } from './auth';

  /**
   * 게임 무관 로비: 방 리스트 + 방 만들기 + 코드/목록으로 참가.
   * 게임별 상태 타입 S는 클라이언트 제네릭으로만 사용된다 (여기선 메시지 봉투만 다룸).
   */
  let {
    game = 'tetris',
    onStart,
    onBack
  }: {
    game?: string;
    onStart: (info: MatchInfo, client: BattleClient<S>) => void;
    onBack: () => void;
  } = $props();

  let rooms = $state<RoomRow[]>([]);
  let leaderboard = $state<LeaderboardResponse>({ rows: [], my: null });
  let loading = $state(true);
  let error = $state<string | null>(null);
  let joinCode = $state('');
  let waiting = $state(false);
  let room: MatchCreateResponse | null = $state(null);
  let copied = $state(false);
  /** 게임 모드 — 방 생성/솔로 매치에 적용된다 */
  let mode = $state<'normal' | 'item'>('normal');
  let showItemGuide = $state(false);
  let timer: ReturnType<typeof setInterval> | null = null;

  const client = new BattleClient<S>();
  let handedOff = false;
  let closed = false;

  async function refresh() {
    try {
      const [roomRows, board] = await Promise.all([listRooms(game), fetchLeaderboard()]);
      rooms = roomRows;
      leaderboard = board;
      error = null;
    } catch (e) {
      if (!waiting) error = e instanceof Error ? e.message : '방 목록을 불러오지 못했습니다';
    } finally {
      loading = false;
    }
  }

  /** 내 행이 상위 목록에 없으면 따로 고정 표시한다 */
  function myRow(): LeaderboardEntry | null {
    if (!leaderboard.my) return null;
    return leaderboard.rows.some((r) => r.name === leaderboard.my!.name) ? null : leaderboard.my;
  }

  client.onClose = () => {
    if (waiting && !handedOff) {
      error = '서버 연결이 끊어졌습니다.';
      waiting = false;
      room = null;
    }
  };

  function beginWaiting(res: MatchCreateResponse) {
    room = res;
    waiting = true;
    error = null;
    client.onMessage = (msg) => {
      if (msg.type === 'start') {
        handedOff = true;
        onStart(
          {
            match_id: msg.match_id,
            player_id: res.player_id,
            player_name: res.player_name,
            opponent_name: msg.opponent_name,
            your_index: msg.your_index
          },
          client
        );
      } else if (msg.type === 'error') {
        error = msg.message;
        waiting = false;
        room = null;
      }
    };
    const token = getToken();
    if (token) client.connect(res.match_id, res.player_id, token);
  }

  async function handleCreate() {
    error = null;
    try {
      const res = await createRoom(game, mode);
      beginWaiting(res);
      void refresh();
    } catch (e) {
      error = e instanceof Error ? e.message : '방 생성 실패';
    }
  }

  async function handleSolo() {
    error = null;
    try {
      const res = await createSoloRoom(game, mode);
      beginWaiting(res);
    } catch (e) {
      error = e instanceof Error ? e.message : '솔로 매치 생성 실패';
    }
  }

  async function handleJoin(code: string) {
    error = null;
    if (!code.trim()) {
      error = '코드를 입력해 주세요.';
      return;
    }
    try {
      const res = await joinRoom(code);
      beginWaiting(res);
      joinCode = '';
    } catch (e) {
      error = e instanceof Error ? e.message : '참가 실패';
    }
  }

  function cancelWait() {
    closed = true;
    client.close();
    waiting = false;
    room = null;
    error = null;
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

  function formatTime(iso: string): string {
    const d = new Date(iso);
    const now = Date.now();
    const diff = Math.max(0, now - d.getTime());
    const min = Math.floor(diff / 60000);
    if (min < 1) return '방금';
    if (min < 60) return `${min}분 전`;
    return `${Math.floor(min / 60)}시간 전`;
  }

  onMount(() => {
    void refresh();
    timer = setInterval(() => void refresh(), 3000);
  });

  onDestroy(() => {
    if (timer) clearInterval(timer);
    if (!handedOff && !closed) client.close();
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
  </div>      {#if waiting && room}
    <Card class="mx-auto w-full max-w-md p-8 text-center">
      <div class="mx-auto mb-5 flex size-14 items-center justify-center rounded-2xl border border-primary/25 bg-primary/10">
        <LoaderCircle size={26} class="animate-spin text-primary" />
      </div>
      {#if room.mode === 'item'}
        <p class="mx-auto mb-3 inline-flex items-center gap-1.5 rounded-full border border-purple-400/25 bg-purple-500/10 px-3 py-1 text-[10px] font-bold tracking-[.16em] text-purple-300">
          <Sparkles size={11} /> 아이템 모드
        </p>
      {/if}
      <p class="text-[10px] font-bold tracking-[.3em] text-primary">WAITING FOR OPPONENT</p>
      <h3 class="mt-2 text-2xl font-black tracking-[-.04em] text-white">{room.solo ? 'CPU 봇과 연결 중' : '상대방을 기다리는 중'}</h3>
      {#if room.solo}
        <p class="mt-4 text-sm text-muted-foreground">곧 배틀이 시작됩니다.</p>
      {:else}
        <p class="mt-4 text-sm text-muted-foreground">친구에게 아래 코드를 알려주거나, 방 리스트에서 참가하게 하세요.</p>
      {/if}
      {#if !room.solo}
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
      {#if error}
        <p class="mt-4 rounded-xl border border-rose-400/20 bg-rose-500/10 px-4 py-2.5 text-center text-sm text-rose-300">{error}</p>
      {/if}
      <Button variant="ghost" size="sm" class="mt-8 text-muted-foreground" onclick={cancelWait}>취소</Button>
    </Card>
  {:else}
    <!-- 게임 모드 선택: 혼자/배틀은 아래 카드로, 일반/아이템은 여기서 선택 -->
    <Card class="mb-4 p-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-2 text-muted-foreground">
          <Sparkles size={15} />
          <span class="text-[10px] font-bold tracking-[.2em]">게임 모드</span>
        </div>
        <div class="flex items-center gap-2">
          <Button variant="outline" size="sm" class="text-white/70" aria-label="아이템 소개" onclick={() => (showItemGuide = true)}>
            <Sparkles size={13} class="text-primary" />
            <span class="hidden sm:inline">아이템 소개</span>
          </Button>
          <div class="flex rounded-xl border border-white/10 bg-black/25 p-1">
          <button
            class="rounded-lg px-4 py-1.5 text-sm font-bold transition {mode === 'normal' ? 'bg-primary text-black' : 'text-white/40 hover:text-white/70'}"
            onclick={() => (mode = 'normal')}
          >
            일반
          </button>
          <button
            class="rounded-lg px-4 py-1.5 text-sm font-bold transition {mode === 'item' ? 'bg-primary text-black' : 'text-white/40 hover:text-white/70'}"
            onclick={() => (mode = 'item')}
          >
            아이템
          </button>
          </div>
        </div>
      </div>
      {#if mode === 'item'}
        <p class="mt-2.5 text-xs leading-5 text-muted-foreground">
          보드에 아이템이 숨어 있습니다. 아이템이 있는 줄을 지우면 발동돼요 — 🎁 폭탄·가속·구멍은 <span class="text-rose-300">상대를 방해</span>하고, 🛡️ 정리·방패·감속은 <span class="text-emerald-300">나를 돕습니다</span>.
        </p>
      {:else}
        <p class="mt-2.5 text-xs leading-5 text-muted-foreground">아이템 없이 순수한 실력으로 겨루는 기본 배틀 모드입니다.</p>
      {/if}
    </Card>
    <ItemGuideDialog open={showItemGuide} onclose={() => (showItemGuide = false)} />

    <div class="grid gap-4 md:grid-cols-2">
      <Card class="p-6">
        <div class="mb-4 flex items-center gap-2 text-muted-foreground">
          <Plus size={15} />
          <span class="text-[10px] font-bold tracking-[.2em]">방 만들기</span>
        </div>
        <p class="text-sm leading-6 text-muted-foreground">새 배틀 방을 만들면 방 리스트에 나타납니다. 친구에게 코드를 공유할 수도 있어요.</p>
        <Button size="lg" class="mt-5 w-full" onclick={handleCreate}><Swords size={16} /> {mode === 'item' ? '아이템 방 생성' : '방 생성'}</Button>
      </Card>

      <Card class="p-6">
        <div class="mb-4 flex items-center gap-2 text-muted-foreground">
          <Users size={15} />
          <span class="text-[10px] font-bold tracking-[.2em]">코드로 참가</span>
        </div>
        <p class="text-sm leading-6 text-muted-foreground">친구의 4자리 코드를 입력해 바로 참가하세요.</p>
        <input
          bind:value={joinCode}
          class="mt-4 w-full rounded-xl border border-white/10 bg-black/25 px-3.5 py-2.5 font-mono text-lg font-bold tracking-[.25em] text-white uppercase outline-none transition focus:border-primary/50"
          placeholder="ABCD"
          maxlength="4"
          onkeydown={(e) => {
            if (e.key === 'Enter') handleJoin(joinCode);
          }}
        />
        <Button size="lg" variant="outline" class="mt-5 w-full" onclick={() => handleJoin(joinCode)}><Users size={16} /> 참가</Button>
      </Card>
    </div>

    <Card class="mt-4 border-primary/20 bg-primary/[.03] p-6">
      <div class="mb-3 flex items-center gap-2 text-primary">
        <Bot size={15} />
        <span class="text-[10px] font-bold tracking-[.2em]">혼자 연습</span>
      </div>
      <p class="text-sm leading-6 text-muted-foreground">상대 없이 CPU 봇과 바로 대전해 연습하세요. 연습 결과는 리더보드에 반영되지 않습니다.</p>
      <Button size="lg" variant="outline" class="mt-4 w-full border-primary/30 text-primary hover:bg-primary/10" onclick={handleSolo}>
        <Bot size={16} /> {mode === 'item' ? '아이템 봇과 연습' : 'CPU 봇과 연습'}
      </Button>
    </Card>

    {#if error}
      <p class="mt-5 rounded-xl border border-rose-400/20 bg-rose-500/10 px-4 py-2.5 text-center text-sm text-rose-300">{error}</p>
    {/if}

    <Card class="mt-5 p-5">
      <div class="mb-4 flex items-center justify-between">
        <div class="flex items-center gap-2 text-muted-foreground">
          <Swords size={15} />
          <span class="text-[10px] font-bold tracking-[.2em]">방 리스트</span>
        </div>
        <Button variant="ghost" size="sm" class="text-muted-foreground" onclick={() => void refresh()}>
          <RefreshCw size={13} class={loading ? 'animate-spin' : ''} /> <span class="hidden sm:inline">새로고침</span>
        </Button>
      </div>

      {#if rooms.length === 0}
        <div class="rounded-xl border border-dashed border-white/10 bg-black/10 px-4 py-10 text-center">
          <p class="text-sm text-muted-foreground">{loading ? '불러오는 중...' : '대기 중인 방이 없습니다.'}</p>
          {#if !loading}
            <p class="mt-1 text-xs text-white/25">방을 만들어 첫 번째 대전을 시작해 보세요.</p>
          {/if}
        </div>
      {:else}
        <div class="space-y-2">
          {#each rooms as row (row.match_id)}
            <div class="flex items-center justify-between gap-3 rounded-xl border border-white/[.06] bg-white/[.025] px-4 py-3">
              <div class="flex items-center gap-3">
                <span class="font-mono text-sm font-black tracking-[.2em] text-primary">{row.code}</span>
                <div class="text-left">
                  <p class="text-sm font-bold text-white">{row.host_name}</p>
                  <p class="text-[10px] text-white/30">방장 · {formatTime(row.created_at)}</p>
                </div>
                {#if row.mode === 'item'}
                  <span class="flex items-center gap-1 rounded-full border border-purple-400/25 bg-purple-500/10 px-2 py-0.5 text-[9px] font-bold tracking-[.1em] text-purple-300">
                    <Sparkles size={9} /> 아이템
                  </span>
                {/if}
              </div>
              <div class="flex items-center gap-3">
                <span class="rounded-full border border-white/10 bg-white/[.04] px-2.5 py-1 text-[10px] font-bold text-white/45">
                  {row.player_count}/2
                </span>
                {#if row.is_mine}
                  <span class="rounded-full border border-cyan-300/20 bg-cyan-400/10 px-2.5 py-1 text-[10px] font-bold text-cyan-300">내 방</span>
                {:else if row.player_count >= 2}
                  <span class="rounded-full border border-white/10 bg-white/[.04] px-2.5 py-1 text-[10px] font-bold text-white/35">만원</span>
                {:else}
                  <Button size="sm" onclick={() => handleJoin(row.code)}>참가</Button>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </Card>

    <!-- 리더보드 (승패 통계) -->
    <Card class="mt-5 p-5">
      <div class="mb-4 flex items-center justify-between">
        <div class="flex items-center gap-2 text-muted-foreground">
          <Trophy size={15} />
          <span class="text-[10px] font-bold tracking-[.2em]">리더보드</span>
        </div>
        {#if leaderboard.rows.length > 0}
          <span class="text-[10px] text-white/25">승률 기준 · 전체 배틀</span>
        {/if}
      </div>

      {#if leaderboard.rows.length === 0}
        <div class="rounded-xl border border-dashed border-white/10 bg-black/10 px-4 py-8 text-center">
          <p class="text-sm text-muted-foreground">{loading ? '불러오는 중...' : '아직 기록된 대전이 없습니다.'}</p>
          {#if !loading}
            <p class="mt-1 text-xs text-white/25">배틀을 완료하면 승패 통계가 여기에 쌓입니다.</p>
          {/if}
        </div>
      {:else}
        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead>
              <tr class="text-[10px] uppercase tracking-[.18em] text-white/30">
                <th class="pb-2 pr-2 font-bold">순위</th>
                <th class="pb-2 pr-2 font-bold">플레이어</th>
                <th class="pb-2 pr-2 text-right font-bold">승</th>
                <th class="pb-2 pr-2 text-right font-bold">패</th>
                <th class="pb-2 text-right font-bold">승률</th>
              </tr>
            </thead>
            <tbody>
              {#each leaderboard.rows as entry, i (entry.name)}
                {@const mine = entry.name === leaderboard.my?.name}
                <tr class="border-t border-white/[.05] {mine ? 'bg-cyan-400/[.06]' : ''}">
                  <td class="py-2.5 pr-2">
                    <span class="flex size-6 items-center justify-center rounded-full text-[11px] font-black {i === 0 ? 'bg-amber-400/15 text-amber-300' : i === 1 ? 'bg-white/10 text-white/70' : i === 2 ? 'bg-orange-400/10 text-orange-300' : 'bg-white/[.04] text-white/35'}">
                      {#if i === 0}<Crown size={12} />{:else}{entry.rank}{/if}
                    </span>
                  </td>
                  <td class="py-2.5 pr-2">
                    <span class="flex items-center gap-2 font-bold {mine ? 'text-cyan-300' : 'text-white'}">
                      {entry.name}
                      {#if mine}
                        <span class="rounded-full border border-cyan-300/25 bg-cyan-400/10 px-1.5 py-0.5 text-[9px] font-bold tracking-[.1em] text-cyan-300">나</span>
                      {/if}
                    </span>
                  </td>
                  <td class="py-2.5 pr-2 text-right font-mono font-bold text-emerald-300">{entry.wins}</td>
                  <td class="py-2.5 pr-2 text-right font-mono text-rose-300/80">{entry.losses}</td>
                  <td class="py-2.5 text-right">
                    <span class="inline-flex items-center gap-1.5">
                      <span class="h-1.5 w-12 overflow-hidden rounded-full bg-white/10">
                        <span class="block h-full rounded-full bg-primary" style="width: {entry.win_rate}%"></span>
                      </span>
                      <span class="w-9 text-right font-mono text-xs text-white/60">{entry.win_rate}%</span>
                    </span>
                  </td>
                </tr>
              {/each}
              {#if myRow()}
                {@const entry = myRow()!}
                <tr class="border-t-2 border-dashed border-cyan-300/20 bg-cyan-400/[.06]">
                  <td class="py-2.5 pr-2">
                    <span class="flex size-6 items-center justify-center rounded-full bg-white/[.06] text-[11px] font-black text-white/60">{entry.rank}</span>
                  </td>
                  <td class="py-2.5 pr-2">
                    <span class="flex items-center gap-2 font-bold text-cyan-300">
                      {entry.name}
                      <span class="rounded-full border border-cyan-300/25 bg-cyan-400/10 px-1.5 py-0.5 text-[9px] font-bold tracking-[.1em] text-cyan-300">나</span>
                    </span>
                  </td>
                  <td class="py-2.5 pr-2 text-right font-mono font-bold text-emerald-300">{entry.wins}</td>
                  <td class="py-2.5 pr-2 text-right font-mono text-rose-300/80">{entry.losses}</td>
                  <td class="py-2.5 text-right">
                    <span class="inline-flex items-center gap-1.5">
                      <span class="h-1.5 w-12 overflow-hidden rounded-full bg-white/10">
                        <span class="block h-full rounded-full bg-primary" style="width: {entry.win_rate}%"></span>
                      </span>
                      <span class="w-9 text-right font-mono text-xs text-white/60">{entry.win_rate}%</span>
                    </span>
                  </td>
                </tr>
              {/if}
            </tbody>
          </table>
        </div>
      {/if}
    </Card>
  {/if}
</div>
