<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ArrowDown,
    ArrowLeft,
    ArrowRight,
    ChevronUp,
    CircleHelp,
    Gamepad2,
    Gauge,
    Headphones,
    Pause,
    Play,
    RefreshCw,
    RotateCw,
    Sparkles,
    Trophy,
    Volume2,
    VolumeX,
    X
  } from 'lucide-svelte';
  import Button from './lib/components/ui/Button.svelte';
  import Card from './lib/components/ui/Card.svelte';
  import PiecePreview from './lib/components/PiecePreview.svelte';
  import ThreeBoard from './lib/components/ThreeBoard.svelte';
  import {
    clearFullRows,
    cloneShape,
    collides,
    createBoard,
    createPiece,
    ghostY,
    mergePiece,
    rotateShape,
    shuffledBag,
    type Board,
    type Piece,
    type PieceType
  } from './lib/game/tetris';

  type GameStatus = 'ready' | 'playing' | 'paused' | 'over';

  let bag = $state<PieceType[]>(shuffledBag());
  let board = $state<Board>(createBoard());
  let active = $state<Piece>(createPiece(takeFromBag()));
  let held = $state<PieceType | null>(null);
  let canHold = $state(true);
  let score = $state(0);
  let lines = $state(0);
  let level = $state(1);
  let highScore = $state(0);
  let status = $state<GameStatus>('ready');
  let soundEnabled = $state(true);
  let showHelp = $state(false);
  let clearFlash = $state(0);
  let lastDropAt = 0;
  let raf = 0;
  let audioContext: AudioContext | null = null;

  let nextPieces = $derived(bag.slice(0, 3));
  let dropInterval = $derived(Math.max(95, 870 - (level - 1) * 68));
  let levelProgress = $derived((lines % 10) * 10);

  function takeFromBag(): PieceType {
    if (bag.length < 8) bag.push(...shuffledBag());
    return bag.shift()!;
  }

  function playTone(frequency: number, duration = 0.06, volume = 0.035) {
    if (!soundEnabled || typeof window === 'undefined') return;
    audioContext ??= new AudioContext();
    const oscillator = audioContext.createOscillator();
    const gain = audioContext.createGain();
    oscillator.type = 'sine';
    oscillator.frequency.value = frequency;
    gain.gain.setValueAtTime(volume, audioContext.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.0001, audioContext.currentTime + duration);
    oscillator.connect(gain).connect(audioContext.destination);
    oscillator.start();
    oscillator.stop(audioContext.currentTime + duration);
  }

  function resetGame(startImmediately = true) {
    bag = shuffledBag();
    board = createBoard();
    active = createPiece(takeFromBag());
    held = null;
    canHold = true;
    score = 0;
    lines = 0;
    level = 1;
    status = startImmediately ? 'playing' : 'ready';
    lastDropAt = performance.now();
    if (startImmediately) playTone(520, 0.09);
  }

  function finishGame() {
    status = 'over';
    highScore = Math.max(highScore, score);
    localStorage.setItem('neon-stack-high-score', String(highScore));
    playTone(120, 0.34, 0.06);
  }

  function spawnNext() {
    const next = createPiece(takeFromBag());
    active = next;
    canHold = true;
    if (collides(board, next)) finishGame();
  }

  function lockPiece(piece = active) {
    const merged = mergePiece(board, piece);
    const cleared = clearFullRows(merged);
    board = cleared.board;
    if (cleared.count > 0) {
      const points = [0, 100, 300, 500, 800][cleared.count] * level;
      score += points;
      lines += cleared.count;
      level = Math.floor(lines / 10) + 1;
      clearFlash += 1;
      playTone(cleared.count === 4 ? 960 : 740, 0.14, 0.055);
    } else {
      playTone(180, 0.035, 0.018);
    }
    spawnNext();
  }

  function move(dx: number) {
    if (status !== 'playing' || collides(board, active, dx, 0)) return;
    active = { ...active, x: active.x + dx };
    playTone(260, 0.025, 0.012);
  }

  function stepDown(manual = false) {
    if (status !== 'playing') return;
    if (!collides(board, active, 0, 1)) {
      active = { ...active, y: active.y + 1 };
      if (manual) score += 1;
    } else {
      lockPiece();
    }
    lastDropAt = performance.now();
  }

  function hardDrop() {
    if (status !== 'playing') return;
    const target = ghostY(board, active);
    const distance = target - active.y;
    const dropped = { ...active, y: target };
    active = dropped;
    score += distance * 2;
    playTone(110, 0.055, 0.04);
    lockPiece(dropped);
    lastDropAt = performance.now();
  }

  function rotate() {
    if (status !== 'playing' || active.type === 'O') return;
    const rotated = rotateShape(active.shape);
    for (const kick of [0, -1, 1, -2, 2]) {
      if (!collides(board, active, kick, 0, rotated)) {
        active = { ...active, x: active.x + kick, shape: rotated };
        playTone(410, 0.035, 0.018);
        return;
      }
    }
  }

  function holdPiece() {
    if (status !== 'playing' || !canHold) return;
    const outgoing = active.type;
    if (held) {
      active = createPiece(held);
      held = outgoing;
    } else {
      held = outgoing;
      active = createPiece(takeFromBag());
    }
    canHold = false;
    playTone(330, 0.06, 0.025);
    if (collides(board, active)) finishGame();
  }

  function togglePause() {
    if (status === 'ready') {
      status = 'playing';
      lastDropAt = performance.now();
      playTone(520, 0.08);
      return;
    }
    if (status === 'over') return;
    status = status === 'paused' ? 'playing' : 'paused';
    lastDropAt = performance.now();
    playTone(status === 'playing' ? 520 : 230, 0.06);
  }

  function handleKeydown(event: KeyboardEvent) {
    if (showHelp && event.key === 'Escape') {
      showHelp = false;
      return;
    }
    if (['ArrowLeft', 'ArrowRight', 'ArrowDown', 'ArrowUp', ' ', 'c', 'C', 'p', 'P'].includes(event.key)) {
      event.preventDefault();
    }
    if (event.key === 'p' || event.key === 'P' || event.key === 'Escape') return togglePause();
    if (status !== 'playing') return;
    if (event.key === 'ArrowLeft' || event.key === 'a' || event.key === 'A') move(-1);
    if (event.key === 'ArrowRight' || event.key === 'd' || event.key === 'D') move(1);
    if (event.key === 'ArrowDown' || event.key === 's' || event.key === 'S') stepDown(true);
    if (event.key === 'ArrowUp' || event.key === 'w' || event.key === 'W') rotate();
    if (event.key === ' ') hardDrop();
    if (event.key === 'c' || event.key === 'C' || event.key === 'Shift') holdPiece();
  }

  onMount(() => {
    highScore = Number(localStorage.getItem('neon-stack-high-score') || 0);
    const frameLoop = (now: number) => {
      if (status === 'playing' && now - lastDropAt >= dropInterval) stepDown();
      raf = requestAnimationFrame(frameLoop);
    };
    lastDropAt = performance.now();
    raf = requestAnimationFrame(frameLoop);
    window.addEventListener('keydown', handleKeydown);
    const handleVisibility = () => {
      if (document.hidden && status === 'playing') status = 'paused';
    };
    document.addEventListener('visibilitychange', handleVisibility);
    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener('keydown', handleKeydown);
      document.removeEventListener('visibilitychange', handleVisibility);
      audioContext?.close();
    };
  });
</script>

<svelte:head>
  <title>NEON STACK — 3D Tetris</title>
</svelte:head>

<div class="relative min-h-screen overflow-hidden">
  <div class="pointer-events-none absolute left-1/2 top-[-22rem] h-[38rem] w-[65rem] -translate-x-1/2 rounded-full border border-cyan-300/[.04] bg-cyan-300/[.025] blur-3xl"></div>

  <header class="relative z-20 mx-auto flex w-full max-w-[1480px] items-center justify-between px-5 py-5 lg:px-10 lg:py-7">
    <div class="flex items-center gap-3">
      <div class="grid size-10 grid-cols-2 gap-1 rounded-xl border border-white/10 bg-white/[.045] p-2 shadow-xl shadow-black/20">
        <span class="rounded-[2px] bg-primary"></span><span class="rounded-[2px] bg-cyan-400"></span>
        <span class="rounded-[2px] bg-violet-400"></span><span class="rounded-[2px] bg-rose-400"></span>
      </div>
      <div>
        <p class="text-[10px] font-bold tracking-[.28em] text-primary">3D ARCADE</p>
        <h1 class="text-lg font-black tracking-[-.04em] text-white sm:text-xl">NEON STACK</h1>
      </div>
    </div>
    <div class="flex items-center gap-1.5">
      <Button variant="ghost" size="icon" class="hidden sm:inline-flex" aria-label="도움말" onclick={() => (showHelp = true)}>
        <CircleHelp size={19} />
      </Button>
      <Button variant="ghost" size="icon" aria-label={soundEnabled ? '소리 끄기' : '소리 켜기'} onclick={() => (soundEnabled = !soundEnabled)}>
        {#if soundEnabled}<Volume2 size={19} />{:else}<VolumeX size={19} />{/if}
      </Button>
      <Button variant="outline" size="sm" onclick={() => resetGame(true)}>
        <RefreshCw size={14} /> <span class="hidden sm:inline">새 게임</span>
      </Button>
    </div>
  </header>

  <main class="relative z-10 mx-auto grid w-full max-w-[1320px] grid-cols-1 gap-4 px-4 pb-7 sm:px-6 lg:h-[calc(100vh-105px)] lg:min-h-[700px] lg:grid-cols-[230px_minmax(430px,650px)_230px] lg:items-stretch lg:justify-center lg:gap-5 lg:px-8 lg:pb-9">
    <aside class="order-2 grid grid-cols-2 gap-4 lg:order-1 lg:flex lg:flex-col">
      <Card class="col-span-1 p-5 lg:col-auto">
        <div class="mb-4 flex items-center gap-2 text-muted-foreground">
          <Gauge size={15} />
          <span class="text-[10px] font-bold tracking-[.2em]">SCORE</span>
        </div>
        <p class="font-mono text-3xl font-black tracking-[-.06em] text-white sm:text-4xl">{score.toLocaleString()}</p>
        <div class="mt-4 h-px bg-gradient-to-r from-white/10 to-transparent"></div>
        <div class="mt-4 flex items-center justify-between text-xs">
          <span class="text-muted-foreground">최고 점수</span>
          <span class="font-mono font-bold text-primary">{highScore.toLocaleString()}</span>
        </div>
      </Card>

      <Card class="col-span-1 p-5 lg:col-auto">
        <div class="mb-4 flex items-center gap-2 text-muted-foreground">
          <Sparkles size={15} />
          <span class="text-[10px] font-bold tracking-[.2em]">HOLD</span>
        </div>
        <div class="flex h-[68px] items-center justify-center rounded-xl border border-dashed border-white/10 bg-black/15">
          {#if held}
            <PiecePreview type={held} size="lg" />
          {:else}
            <span class="text-[10px] font-semibold tracking-[.2em] text-white/20">EMPTY</span>
          {/if}
        </div>
        <Button variant="ghost" size="sm" class="mt-2 w-full text-[10px]" disabled={!canHold || status !== 'playing'} onclick={holdPiece}>
          C / SHIFT
        </Button>
      </Card>

      <Card class="col-span-2 hidden flex-1 flex-col justify-between p-5 lg:flex">
        <div>
          <div class="mb-5 flex items-center gap-2 text-muted-foreground">
            <Gamepad2 size={15} />
            <span class="text-[10px] font-bold tracking-[.2em]">CONTROLS</span>
          </div>
          <div class="space-y-3.5 text-xs">
            <div class="flex items-center justify-between"><span class="text-muted-foreground">이동</span><kbd class="key">← ↓ →</kbd></div>
            <div class="flex items-center justify-between"><span class="text-muted-foreground">회전</span><kbd class="key">↑</kbd></div>
            <div class="flex items-center justify-between"><span class="text-muted-foreground">바로 내리기</span><kbd class="key wide">SPACE</kbd></div>
            <div class="flex items-center justify-between"><span class="text-muted-foreground">블록 보관</span><kbd class="key">C</kbd></div>
            <div class="flex items-center justify-between"><span class="text-muted-foreground">일시정지</span><kbd class="key">P</kbd></div>
          </div>
        </div>
        <p class="mt-6 text-[10px] leading-5 text-white/25">보드를 드래그하면<br />3D 시점을 바꿀 수 있습니다.</p>
      </Card>
    </aside>

    <section class="order-1 min-h-[600px] lg:order-2 lg:min-h-0">
      <Card class="relative h-[72vh] min-h-[600px] overflow-hidden bg-[#0c0f17]/75 p-2 sm:h-[760px] lg:h-full">
        <div class="pointer-events-none absolute inset-x-8 top-0 h-px bg-gradient-to-r from-transparent via-cyan-300/50 to-transparent"></div>
        <ThreeBoard {board} {active} {status} {clearFlash} />

        {#if status !== 'playing'}
          <div class="absolute inset-2 z-10 flex items-center justify-center rounded-[1.35rem] bg-[#070910]/68 p-6 backdrop-blur-[5px]">
            <div class="max-w-sm text-center">
              {#if status === 'ready'}
                <div class="mx-auto mb-5 flex size-14 items-center justify-center rounded-2xl border border-primary/20 bg-primary/10 text-primary shadow-[0_0_35px_rgba(215,255,69,.12)]"><Gamepad2 size={26} /></div>
                <p class="mb-2 text-[10px] font-bold tracking-[.3em] text-primary">READY PLAYER ONE</p>
                <h2 class="text-3xl font-black tracking-[-.05em]">쌓을 준비가<br />되셨나요?</h2>
                <p class="mx-auto mt-3 max-w-[260px] text-sm leading-6 text-muted-foreground">빈틈없이 쌓고 한 번에 네 줄을 완성해 최고 점수를 경신하세요.</p>
                <Button size="lg" class="mt-7 min-w-40" onclick={togglePause}><Play size={17} fill="currentColor" /> 게임 시작</Button>
              {:else if status === 'paused'}
                <div class="mx-auto mb-5 flex size-14 items-center justify-center rounded-2xl border border-white/10 bg-white/[.06] text-white"><Pause size={25} /></div>
                <p class="mb-2 text-[10px] font-bold tracking-[.3em] text-cyan-300">GAME PAUSED</p>
                <h2 class="text-3xl font-black tracking-[-.05em]">잠시 멈춤</h2>
                <p class="mt-3 text-sm text-muted-foreground">현재 진행 상황이 그대로 보존됩니다.</p>
                <Button size="lg" class="mt-7 min-w-40" onclick={togglePause}><Play size={17} fill="currentColor" /> 계속하기</Button>
              {:else}
                <div class="mx-auto mb-5 flex size-14 items-center justify-center rounded-2xl border border-rose-400/20 bg-rose-400/10 text-rose-300"><Trophy size={26} /></div>
                <p class="mb-2 text-[10px] font-bold tracking-[.3em] text-rose-300">GAME OVER</p>
                <h2 class="font-mono text-4xl font-black tracking-[-.06em]">{score.toLocaleString()}</h2>
                <p class="mt-3 text-sm text-muted-foreground">{lines}줄 제거 · 레벨 {level}</p>
                <Button size="lg" class="mt-7 min-w-40" onclick={() => resetGame(true)}><RefreshCw size={17} /> 다시 도전</Button>
              {/if}
            </div>
          </div>
        {/if}
      </Card>
    </section>

    <aside class="order-3 grid grid-cols-2 gap-4 lg:flex lg:flex-col">
      <Card class="p-5">
        <div class="mb-5 flex items-center justify-between">
          <div class="flex items-center gap-2 text-muted-foreground">
            <ChevronUp size={15} />
            <span class="text-[10px] font-bold tracking-[.2em]">NEXT</span>
          </div>
          <span class="text-[9px] font-bold tracking-[.16em] text-white/20">QUEUE</span>
        </div>
        <div class="space-y-2">
          {#each nextPieces as piece, index}
            <div class={`flex items-center rounded-xl border border-white/[.055] bg-black/15 ${index === 0 ? 'h-[72px] justify-center' : 'h-12 justify-between px-4'}`}>
              {#if index > 0}<span class="font-mono text-[10px] text-white/20">0{index + 1}</span>{/if}
              <PiecePreview type={piece} size={index === 0 ? 'lg' : 'sm'} />
            </div>
          {/each}
        </div>
      </Card>

      <Card class="p-5">
        <div class="mb-5 flex items-center justify-between">
          <span class="text-[10px] font-bold tracking-[.2em] text-muted-foreground">LEVEL</span>
          <span class="font-mono text-2xl font-black text-primary">{String(level).padStart(2, '0')}</span>
        </div>
        <div class="h-1.5 overflow-hidden rounded-full bg-white/[.06]">
          <div class="h-full rounded-full bg-gradient-to-r from-cyan-400 to-primary transition-[width] duration-500" style={`width:${levelProgress}%`}></div>
        </div>
        <div class="mt-3 flex items-center justify-between text-[10px] text-muted-foreground"><span>{lines} LINES</span><span>{10 - (lines % 10)} TO GO</span></div>
      </Card>

      <Card class="col-span-2 p-4 lg:col-auto lg:mt-auto">
        <div class="grid grid-cols-5 gap-2">
          <Button variant="secondary" size="icon" aria-label="왼쪽 이동" onclick={() => move(-1)}><ArrowLeft size={19} /></Button>
          <Button variant="secondary" size="icon" aria-label="아래로 이동" onclick={() => stepDown(true)}><ArrowDown size={19} /></Button>
          <Button variant="secondary" size="icon" aria-label="오른쪽 이동" onclick={() => move(1)}><ArrowRight size={19} /></Button>
          <Button variant="secondary" size="icon" aria-label="회전" onclick={rotate}><RotateCw size={18} /></Button>
          <Button variant="default" size="icon" aria-label="바로 내리기" onclick={hardDrop}><ChevronUp class="rotate-180" size={20} /></Button>
        </div>
        <div class="mt-2 grid grid-cols-2 gap-2">
          <Button variant="ghost" size="sm" onclick={holdPiece}>HOLD</Button>
          <Button variant="ghost" size="sm" onclick={togglePause}>{status === 'paused' ? 'RESUME' : 'PAUSE'}</Button>
        </div>
      </Card>
    </aside>
  </main>

  {#if showHelp}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-5 backdrop-blur-sm" role="presentation" onclick={(event) => event.target === event.currentTarget && (showHelp = false)}>
      <Card class="w-full max-w-md p-6 shadow-2xl shadow-black/60">
        <div class="flex items-start justify-between">
          <div>
            <p class="text-[10px] font-bold tracking-[.25em] text-primary">HOW TO PLAY</p>
            <h2 class="mt-1 text-2xl font-black tracking-[-.04em]">게임 방법</h2>
          </div>
          <Button variant="ghost" size="icon" aria-label="닫기" onclick={() => (showHelp = false)}><X size={19} /></Button>
        </div>
        <div class="mt-6 space-y-4 text-sm leading-6 text-muted-foreground">
          <p>블록을 이동하고 회전해 가로 한 줄을 빈틈없이 채우면 그 줄이 사라집니다. 여러 줄을 동시에 지울수록 더 높은 점수를 얻습니다.</p>
          <div class="grid grid-cols-2 gap-3">
            <div class="rounded-xl border border-white/[.06] bg-white/[.025] p-3"><strong class="block text-xs text-white">고스트 블록</strong><span class="text-xs">현재 착지 위치</span></div>
            <div class="rounded-xl border border-white/[.06] bg-white/[.025] p-3"><strong class="block text-xs text-white">HOLD</strong><span class="text-xs">블록 1개 보관</span></div>
          </div>
          <p class="text-xs">키보드는 방향키, Space, C, P를 사용합니다. 터치 환경에서는 화면 아래 조작 버튼을 이용할 수 있습니다.</p>
        </div>
        <Button class="mt-6 w-full" onclick={() => (showHelp = false)}>확인</Button>
      </Card>
    </div>
  {/if}

  <footer class="relative z-10 pb-6 text-center text-[9px] font-semibold tracking-[.24em] text-white/15 lg:hidden">NEON STACK · 3D BLOCK PUZZLE</footer>
</div>

<style>
  :global(.key) {
    display: inline-flex;
    min-width: 25px;
    height: 24px;
    align-items: center;
    justify-content: center;
    border: 1px solid rgba(255, 255, 255, 0.11);
    border-bottom-color: rgba(255, 255, 255, 0.22);
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.045);
    padding: 0 6px;
    color: rgba(255, 255, 255, 0.62);
    font-family: ui-monospace, monospace;
    font-size: 9px;
    font-weight: 700;
  }

  :global(.key.wide) {
    min-width: 45px;
  }
</style>
