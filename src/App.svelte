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
    Pause,
    Play,
    RefreshCw,
    RotateCcw,
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
  import { TetrisEngine, type GameStatus } from './lib/game/engine';
  import { MusicPlayer } from './lib/game/music';
  import type { Board, Piece, PieceType } from './lib/game/tetris';

  interface GameView {
    status: GameStatus;
    board: Board;
    active: Piece;
    held: PieceType | null;
    canHold: boolean;
    score: number;
    lines: number;
    level: number;
    combo: number;
    backToBack: boolean;
    clearFlash: number;
    next: PieceType[];
  }

  // 엔진은 프레임워크 무관한 평범한 클래스 — Svelte 반응성은 스냅샷 뷰가 담당한다.
  const engine = new TetrisEngine();
  let view = $state<GameView>(snapshot());
  let highScore = $state(0);
  let soundEnabled = $state(true);
  let showHelp = $state(false);
  let audioContext: AudioContext | null = null;
  const music = new MusicPlayer();
  let raf = 0;

  let nextPieces = $derived(view.next);
  let levelProgress = $derived((view.lines % 10) * 10);

  function snapshot(): GameView {
    return {
      status: engine.status,
      board: engine.board,
      active: engine.active,
      held: engine.held,
      canHold: engine.canHold,
      score: engine.score,
      lines: engine.lines,
      level: engine.level,
      combo: engine.combo,
      backToBack: engine.backToBack,
      clearFlash: engine.clearFlash,
      next: engine.nextQueue(3)
    };
  }

  function playTone(frequency: number, duration = 0.06, volume = 0.035) {
    if (!soundEnabled || typeof window === 'undefined') return;
    audioContext ??= new AudioContext();
    if (audioContext.state === 'suspended') void audioContext.resume();
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

  function unlockAudio() {
    music.unlock();
    if (audioContext && audioContext.state === 'suspended') void audioContext.resume();
  }

  function togglePause() {
    unlockAudio();
    engine.togglePause();
  }

  function handleKeydown(event: KeyboardEvent) {
    const key = event.key;
    if (showHelp && key === 'Escape') {
      showHelp = false;
      return;
    }
    if (
      ['ArrowLeft', 'ArrowRight', 'ArrowDown', 'ArrowUp', ' ', 'z', 'Z', 'c', 'C', 'p', 'P', 'r', 'R', 'Shift'].includes(
        key
      )
    ) {
      event.preventDefault();
    }
    unlockAudio();
    if (key === 'p' || key === 'P' || key === 'Escape') return togglePause();
    if (key === 'r' || key === 'R') return engine.reset(true);
    if (key === 'z' || key === 'Z') return engine.rotate(-1);
    if (key === 'ArrowUp' || key === 'w' || key === 'W') return engine.rotate(1);
    if (key === ' ') return engine.hardDrop();
    if (key === 'c' || key === 'C' || key === 'Shift') return engine.hold();
    if (key === 'ArrowLeft' || key === 'a' || key === 'A') return engine.press('left');
    if (key === 'ArrowRight' || key === 'd' || key === 'D') return engine.press('right');
    if (key === 'ArrowDown' || key === 's' || key === 'S') return engine.press('down');
  }

  function handleKeyup(event: KeyboardEvent) {
    const key = event.key;
    if (key === 'ArrowLeft' || key === 'a' || key === 'A') return engine.release('left');
    if (key === 'ArrowRight' || key === 'd' || key === 'D') return engine.release('right');
    if (key === 'ArrowDown' || key === 's' || key === 'S') return engine.release('down');
  }

  $effect(() => {
    music.setEnabled(soundEnabled && !document.hidden);
  });

  onMount(() => {
    try {
      highScore = Number(localStorage.getItem('neon-stack-high-score') || 0);
    } catch {
      highScore = 0;
    }
    engine.onEvent = (event) => {
      if (event.type === 'stateChange') {
        view = snapshot();
      } else if (event.type === 'tone') {
        playTone(event.frequency, event.duration, event.volume);
      } else if (event.type === 'gameOver') {
        highScore = Math.max(highScore, engine.score);
        try {
          localStorage.setItem('neon-stack-high-score', String(highScore));
        } catch {
          /* 프라이빗 모드 등 저장 불가 환경 */
        }
        playTone(120, 0.34, 0.06);
      }
    };

    let last = performance.now();
    const frameLoop = (now: number) => {
      const delta = Math.min(now - last, 100);
      last = now;
      engine.update(delta);
      raf = requestAnimationFrame(frameLoop);
    };
    raf = requestAnimationFrame(frameLoop);

    window.addEventListener('keydown', handleKeydown);
    window.addEventListener('keyup', handleKeyup);
    window.addEventListener('pointerdown', unlockAudio);
    // 창 포커스를 잃으면 눌린 키를 전부 해제 (DAS/소프트드롭 잔상 방지)
    const handleBlur = () => {
      engine.release('left');
      engine.release('right');
      engine.release('down');
    };
    window.addEventListener('blur', handleBlur);
    const handleVisibility = () => {
      if (document.hidden && engine.status === 'playing') engine.togglePause();
      music.setEnabled(soundEnabled && !document.hidden);
    };
    document.addEventListener('visibilitychange', handleVisibility);
    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener('keydown', handleKeydown);
      window.removeEventListener('keyup', handleKeyup);
      window.removeEventListener('pointerdown', unlockAudio);
      window.removeEventListener('blur', handleBlur);
      document.removeEventListener('visibilitychange', handleVisibility);
      audioContext?.close();
      music.dispose();
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
      <Button variant="outline" size="sm" onclick={() => engine.reset(true)}>
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
        <p class="font-mono text-3xl font-black tracking-[-.06em] text-white sm:text-4xl">{view.score.toLocaleString()}</p>
        <div class="mt-4 h-px bg-gradient-to-r from-white/10 to-transparent"></div>
        <div class="mt-4 flex items-center justify-between text-xs">
          <span class="text-muted-foreground">최고 점수</span>
          <span class="font-mono font-bold text-primary">{highScore.toLocaleString()}</span>
        </div>
        <div class="mt-3 flex min-h-6 flex-wrap items-center gap-1.5">
          {#if view.combo > 1}
            <span class="rounded-full border border-cyan-300/20 bg-cyan-400/10 px-2 py-0.5 text-[10px] font-bold text-cyan-300">COMBO ×{view.combo}</span>
          {/if}
          {#if view.backToBack}
            <span class="rounded-full border border-violet-300/20 bg-violet-400/10 px-2 py-0.5 text-[10px] font-bold text-violet-300">BACK-TO-BACK ×1.5</span>
          {/if}
        </div>
      </Card>

      <Card class="col-span-1 p-5 lg:col-auto">
        <div class="mb-4 flex items-center gap-2 text-muted-foreground">
          <Sparkles size={15} />
          <span class="text-[10px] font-bold tracking-[.2em]">HOLD</span>
        </div>
        <div class="flex h-[68px] items-center justify-center rounded-xl border border-dashed border-white/10 bg-black/15">
          {#if view.held}
            <PiecePreview type={view.held} size="lg" />
          {:else}
            <span class="text-[10px] font-semibold tracking-[.2em] text-white/20">EMPTY</span>
          {/if}
        </div>
        <Button variant="ghost" size="sm" class="mt-2 w-full text-[10px]" disabled={!view.canHold || view.status !== 'playing'} onclick={() => engine.hold()}>
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
            <div class="flex items-center justify-between"><span class="text-muted-foreground">회전</span><kbd class="key">↑ / Z</kbd></div>
            <div class="flex items-center justify-between"><span class="text-muted-foreground">바로 내리기</span><kbd class="key wide">SPACE</kbd></div>
            <div class="flex items-center justify-between"><span class="text-muted-foreground">블록 보관</span><kbd class="key">C</kbd></div>
            <div class="flex items-center justify-between"><span class="text-muted-foreground">일시정지</span><kbd class="key">P</kbd></div>
            <div class="flex items-center justify-between"><span class="text-muted-foreground">다시 시작</span><kbd class="key">R</kbd></div>
          </div>
        </div>
        <p class="mt-6 text-[10px] leading-5 text-white/25">키를 누르고 있으면 자동 이동(DAS).<br />보드를 드래그하면 3D 시점이 바뀝니다.</p>
      </Card>
    </aside>

    <section class="order-1 min-h-[600px] lg:order-2 lg:min-h-0">
      <Card class="relative h-[72vh] min-h-[600px] overflow-hidden bg-[#0c0f17]/75 p-2 sm:h-[760px] lg:h-full">
        <div class="pointer-events-none absolute inset-x-8 top-0 h-px bg-gradient-to-r from-transparent via-cyan-300/50 to-transparent"></div>
        <ThreeBoard board={view.board} active={view.active} status={view.status} clearFlash={view.clearFlash} />

        {#if view.status !== 'playing'}
          <div class="absolute inset-2 z-10 flex items-center justify-center rounded-[1.35rem] bg-[#070910]/68 p-6 backdrop-blur-[5px]">
            <div class="max-w-sm text-center">
              {#if view.status === 'ready'}
                <div class="mx-auto mb-5 flex size-14 items-center justify-center rounded-2xl border border-primary/20 bg-primary/10 text-primary shadow-[0_0_35px_rgba(215,255,69,.12)]"><Gamepad2 size={26} /></div>
                <p class="mb-2 text-[10px] font-bold tracking-[.3em] text-primary">READY PLAYER ONE</p>
                <h2 class="text-3xl font-black tracking-[-.05em]">쌓을 준비가<br />되셨나요?</h2>
                <p class="mx-auto mt-3 max-w-[260px] text-sm leading-6 text-muted-foreground">빈틈없이 쌓고 한 번에 네 줄을 완성해 최고 점수를 경신하세요.</p>
                <Button size="lg" class="mt-7 min-w-40" onclick={togglePause}><Play size={17} fill="currentColor" /> 게임 시작</Button>
              {:else if view.status === 'paused'}
                <div class="mx-auto mb-5 flex size-14 items-center justify-center rounded-2xl border border-white/10 bg-white/[.06] text-white"><Pause size={25} /></div>
                <p class="mb-2 text-[10px] font-bold tracking-[.3em] text-cyan-300">GAME PAUSED</p>
                <h2 class="text-3xl font-black tracking-[-.05em]">잠시 멈춤</h2>
                <p class="mt-3 text-sm text-muted-foreground">현재 진행 상황이 그대로 보존됩니다.</p>
                <Button size="lg" class="mt-7 min-w-40" onclick={togglePause}><Play size={17} fill="currentColor" /> 계속하기</Button>
              {:else}
                <div class="mx-auto mb-5 flex size-14 items-center justify-center rounded-2xl border border-rose-400/20 bg-rose-400/10 text-rose-300"><Trophy size={26} /></div>
                <p class="mb-2 text-[10px] font-bold tracking-[.3em] text-rose-300">GAME OVER</p>
                <h2 class="font-mono text-4xl font-black tracking-[-.06em]">{view.score.toLocaleString()}</h2>
                <p class="mt-3 text-sm text-muted-foreground">{view.lines}줄 제거 · 레벨 {view.level}</p>
                <Button size="lg" class="mt-7 min-w-40" onclick={() => engine.reset(true)}><RefreshCw size={17} /> 다시 도전</Button>
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
          <span class="font-mono text-2xl font-black text-primary">{String(view.level).padStart(2, '0')}</span>
        </div>
        <div class="h-1.5 overflow-hidden rounded-full bg-white/[.06]">
          <div class="h-full rounded-full bg-gradient-to-r from-cyan-400 to-primary transition-[width] duration-500" style={`width:${levelProgress}%`}></div>
        </div>
        <div class="mt-3 flex items-center justify-between text-[10px] text-muted-foreground"><span>{view.lines} LINES</span><span>{10 - (view.lines % 10)} TO GO</span></div>
      </Card>

      <Card class="col-span-2 p-4 lg:col-auto lg:mt-auto">
        <div class="grid grid-cols-3 gap-2">
          <!-- 이동/소프트드롭은 누르고 있으면 DAS가 작동하도록 포인터 이벤트 사용 -->
          <Button variant="secondary" size="icon" class="touch-manipulation select-none" aria-label="왼쪽 이동"
            onpointerdown={() => engine.press('left')}
            onpointerup={() => engine.release('left')}
            onpointerleave={() => engine.release('left')}
            onpointercancel={() => engine.release('left')}><ArrowLeft size={19} /></Button>
          <Button variant="secondary" size="icon" class="touch-manipulation select-none" aria-label="아래로 이동"
            onpointerdown={() => engine.press('down')}
            onpointerup={() => engine.release('down')}
            onpointerleave={() => engine.release('down')}
            onpointercancel={() => engine.release('down')}><ArrowDown size={19} /></Button>
          <Button variant="secondary" size="icon" class="touch-manipulation select-none" aria-label="오른쪽 이동"
            onpointerdown={() => engine.press('right')}
            onpointerup={() => engine.release('right')}
            onpointerleave={() => engine.release('right')}
            onpointercancel={() => engine.release('right')}><ArrowRight size={19} /></Button>
          <Button variant="secondary" size="icon" aria-label="시계 방향 회전" onclick={() => engine.rotate(1)}><RotateCw size={18} /></Button>
          <Button variant="secondary" size="icon" aria-label="반시계 방향 회전" onclick={() => engine.rotate(-1)}><RotateCcw size={18} /></Button>
          <Button variant="default" size="icon" aria-label="바로 내리기" onclick={() => engine.hardDrop()}><ChevronUp class="rotate-180" size={20} /></Button>
        </div>
        <div class="mt-2 grid grid-cols-2 gap-2">
          <Button variant="ghost" size="sm" onclick={() => engine.hold()}>HOLD</Button>
          <Button variant="ghost" size="sm" onclick={togglePause}>{view.status === 'paused' ? 'RESUME' : 'PAUSE'}</Button>
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
          <p>블록을 이동하고 회전해 가로 한 줄을 빈틈없이 채우면 그 줄이 사라집니다. 여러 줄을 동시에 지울수록 더 높은 점수를 얻습니다. T-spin과 백투백(연속 테트리스)은 추가 보너스를 줍니다.</p>
          <div class="grid grid-cols-2 gap-3">
            <div class="rounded-xl border border-white/[.06] bg-white/[.025] p-3"><strong class="block text-xs text-white">고스트 블록</strong><span class="text-xs">현재 착지 위치</span></div>
            <div class="rounded-xl border border-white/[.06] bg-white/[.025] p-3"><strong class="block text-xs text-white">HOLD</strong><span class="text-xs">블록 1개 보관</span></div>
          </div>
          <p class="text-xs">키보드: 방향키(이동/회전), Z(반시계 회전), Space(바로 내리기), C(보관), P(일시정지), R(다시 시작). 방향키를 누르고 있으면 자동 이동(DAS)이 작동합니다. 터치 환경에서는 화면 아래 조작 버튼을 이용할 수 있습니다.</p>
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
