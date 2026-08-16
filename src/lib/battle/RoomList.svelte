<script lang="ts" generics="S">
  import { onDestroy, onMount } from 'svelte';
  import { ArrowLeft, Copy, LoaderCircle, Plus, RefreshCw, Swords, Users, X } from 'lucide-svelte';
  import Button from '../components/ui/Button.svelte';
  import Card from '../components/ui/Card.svelte';
  import { BattleClient } from './client';
  import type { MatchCreateResponse, MatchInfo, RoomRow } from './protocol';
  import { createRoom, joinRoom, listRooms } from './rooms';
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
  let loading = $state(true);
  let error = $state<string | null>(null);
  let joinCode = $state('');
  let waiting = $state(false);
  let room: MatchCreateResponse | null = $state(null);
  let copied = $state(false);
  let timer: ReturnType<typeof setInterval> | null = null;

  const client = new BattleClient<S>();
  let handedOff = false;
  let closed = false;

  async function refresh() {
    try {
      rooms = await listRooms(game);
      error = null;
    } catch (e) {
      if (!waiting) error = e instanceof Error ? e.message : '방 목록을 불러오지 못했습니다';
    } finally {
      loading = false;
    }
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
      const res = await createRoom(game);
      beginWaiting(res);
      void refresh();
    } catch (e) {
      error = e instanceof Error ? e.message : '방 생성 실패';
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
  </div>

  {#if waiting && room}
    <Card class="mx-auto w-full max-w-md p-8 text-center">
      <div class="mx-auto mb-5 flex size-14 items-center justify-center rounded-2xl border border-primary/25 bg-primary/10">
        <LoaderCircle size={26} class="animate-spin text-primary" />
      </div>
      <p class="text-[10px] font-bold tracking-[.3em] text-primary">WAITING FOR OPPONENT</p>
      <h3 class="mt-2 text-2xl font-black tracking-[-.04em] text-white">상대방을 기다리는 중</h3>
      <p class="mt-4 text-sm text-muted-foreground">친구에게 아래 코드를 알려주거나, 방 리스트에서 참가하게 하세요.</p>
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
      {#if error}
        <p class="mt-4 rounded-xl border border-rose-400/20 bg-rose-500/10 px-4 py-2.5 text-center text-sm text-rose-300">{error}</p>
      {/if}
      <Button variant="ghost" size="sm" class="mt-8 text-muted-foreground" onclick={cancelWait}>취소</Button>
    </Card>
  {:else}
    <div class="grid gap-4 md:grid-cols-2">
      <Card class="p-6">
        <div class="mb-4 flex items-center gap-2 text-muted-foreground">
          <Plus size={15} />
          <span class="text-[10px] font-bold tracking-[.2em]">방 만들기</span>
        </div>
        <p class="text-sm leading-6 text-muted-foreground">새 배틀 방을 만들면 방 리스트에 나타납니다. 친구에게 코드를 공유할 수도 있어요.</p>
        <Button size="lg" class="mt-5 w-full" onclick={handleCreate}><Swords size={16} /> 방 생성</Button>
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
  {/if}
</div>
