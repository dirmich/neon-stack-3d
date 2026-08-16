<script lang="ts">
  import { LoaderCircle, LogIn, UserPlus } from 'lucide-svelte';
  import Button from './ui/Button.svelte';
  import Card from './ui/Card.svelte';
  import { login, register, type AuthUser } from '../battle/auth';

  let { onLogin }: { onLogin: (user: AuthUser) => void } = $props();

  let mode = $state<'login' | 'register'>('login');
  let name = $state('');
  let password = $state('');
  let error = $state<string | null>(null);
  let busy = $state(false);

  async function submit() {
    error = null;
    if (!name.trim() || !password) {
      error = '이름과 비밀번호를 입력해 주세요.';
      return;
    }
    busy = true;
    try {
      const res =
        mode === 'login' ? await login(name.trim(), password) : await register(name.trim(), password);
      onLogin(res.user);
    } catch (e) {
      error = e instanceof Error ? e.message : '요청에 실패했습니다';
    } finally {
      busy = false;
    }
  }
</script>

<div class="relative flex min-h-screen items-center justify-center overflow-hidden px-4">
  <div class="pointer-events-none absolute left-1/2 top-[-22rem] h-[38rem] w-[65rem] -translate-x-1/2 rounded-full border border-cyan-300/[.04] bg-cyan-300/[.025] blur-3xl"></div>

  <Card class="relative w-full max-w-sm p-8 shadow-2xl shadow-black/50">
    <div class="flex flex-col items-center text-center">
      <div class="grid size-14 grid-cols-2 gap-1.5 rounded-2xl border border-white/10 bg-white/[.045] p-3 shadow-xl shadow-black/20">
        <span class="rounded-[3px] bg-primary"></span><span class="rounded-[3px] bg-cyan-400"></span>
        <span class="rounded-[3px] bg-violet-400"></span><span class="rounded-[3px] bg-rose-400"></span>
      </div>
      <p class="mt-5 text-[10px] font-bold tracking-[.28em] text-primary">3D ARCADE</p>
      <h1 class="mt-1 text-2xl font-black tracking-[-.04em] text-white">NEON STACK</h1>
      <p class="mt-3 text-sm text-muted-foreground">배틀을 시작하려면 로그인하세요.</p>
    </div>

    <div class="mt-7 flex rounded-xl border border-white/10 bg-black/20 p-1">
      <button
        class={`flex-1 rounded-lg py-2 text-xs font-bold transition ${mode === 'login' ? 'bg-white/[.08] text-white' : 'text-muted-foreground hover:text-white/70'}`}
        onclick={() => { mode = 'login'; error = null; }}
      >
        로그인
      </button>
      <button
        class={`flex-1 rounded-lg py-2 text-xs font-bold transition ${mode === 'register' ? 'bg-white/[.08] text-white' : 'text-muted-foreground hover:text-white/70'}`}
        onclick={() => { mode = 'register'; error = null; }}
      >
        회원가입
      </button>
    </div>

    <div class="mt-5 space-y-3">
      <div>
        <label for="login-name" class="mb-1.5 block text-[10px] font-bold tracking-[.18em] text-muted-foreground">이름</label>
        <input
          id="login-name"
          bind:value={name}
          class="w-full rounded-xl border border-white/10 bg-black/25 px-3.5 py-2.5 text-sm text-white outline-none transition focus:border-primary/50"
          placeholder="닉네임 (2~16자)"
          maxlength="16"
          onkeydown={(e) => e.key === 'Enter' && submit()}
        />
      </div>
      <div>
        <label for="login-password" class="mb-1.5 block text-[10px] font-bold tracking-[.18em] text-muted-foreground">비밀번호</label>
        <input
          id="login-password"
          bind:value={password}
          type="password"
          class="w-full rounded-xl border border-white/10 bg-black/25 px-3.5 py-2.5 text-sm text-white outline-none transition focus:border-primary/50"
          placeholder="4자 이상"
          onkeydown={(e) => e.key === 'Enter' && submit()}
        />
      </div>
    </div>

    {#if error}
      <p class="mt-4 rounded-xl border border-rose-400/20 bg-rose-500/10 px-4 py-2.5 text-center text-sm text-rose-300">{error}</p>
    {/if}

    <Button size="lg" class="mt-6 w-full" disabled={busy} onclick={submit}>
      {#if busy}
        <LoaderCircle size={16} class="animate-spin" />
      {:else if mode === 'login'}
        <LogIn size={16} />
      {:else}
        <UserPlus size={16} />
      {/if}
      {mode === 'login' ? '로그인' : '가입하고 시작'}
    </Button>

    <p class="mt-5 text-center text-[10px] leading-4 text-white/25">로그인 상태는 이 브라우저에 유지됩니다.<br />배틀 승패가 기록되고 리더보드에 반영됩니다.</p>
  </Card>
</div>
