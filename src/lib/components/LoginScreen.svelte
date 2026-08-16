<script lang="ts">
  import { onMount } from 'svelte';
  import { LoaderCircle } from 'lucide-svelte';
  import Button from './ui/Button.svelte';
  import Card from './ui/Card.svelte';
  import { getGoogleConfig, googleLogin, type AuthUser, type GoogleConfig } from '../battle/auth';

  let { onLogin }: { onLogin: (user: AuthUser) => void } = $props();

  let config = $state<GoogleConfig | null>(null);
  let error = $state<string | null>(null);
  let busy = $state(false);
  let rendered = false;
  let gsiReady = $state(false);

  function loadGsiScript(): Promise<void> {
    return new Promise((resolve, reject) => {
      if (window.google?.accounts?.id) {
        resolve();
        return;
      }
      const script = document.createElement('script');
      script.src = 'https://accounts.google.com/gsi/client';
      script.async = true;
      script.defer = true;
      script.onload = () => resolve();
      script.onerror = () => reject(new Error('Google 로그인 스크립트를 불러오지 못했습니다'));
      document.head.appendChild(script);
    });
  }

  onMount(async () => {
    try {
      config = await getGoogleConfig();
    } catch {
      config = { client_id: '', enabled: false };
    }
    if (!config?.enabled || !config.client_id) return;
    try {
      await loadGsiScript();
      window.google?.accounts?.id.initialize({
        client_id: config.client_id,
        callback: (resp) => void handleCredential(resp.credential)
      });
      gsiReady = true;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Google 로그인 초기화 실패';
    }
  });

  async function handleCredential(credential: string) {
    if (busy) return;
    busy = true;
    error = null;
    try {
      const { user } = await googleLogin(credential);
      onLogin(user);
    } catch (e) {
      error = e instanceof Error ? e.message : '구글 로그인 실패';
      busy = false;
    }
  }

  /** GIS 버튼 렌더는 스크립트 로드 + DOM 준비 후에 — 구글 버튼 컨테이너에 그린다 */
  let googleBtn = $state<HTMLDivElement | undefined>();
  $effect(() => {
    if (config?.enabled && gsiReady && !rendered && googleBtn && window.google?.accounts?.id) {
      rendered = true;
      window.google.accounts.id.renderButton(googleBtn, {
        theme: 'filled_black',
        size: 'large',
        shape: 'pill',
        width: 280,
        text: 'continue_with'
      });
    }
  });
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
      <p class="mt-3 text-sm text-muted-foreground">구글 계정으로 로그인하고 배틀을 시작하세요.</p>
    </div>

    <div class="mt-8 flex flex-col items-center gap-4">
      {#if config === null}
        <div class="flex h-12 items-center gap-2 text-sm text-muted-foreground">
          <LoaderCircle size={18} class="animate-spin text-primary" /> 설정 확인 중...
        </div>
      {:else if config.enabled}
        {#if busy}
          <div class="flex h-12 items-center gap-2 text-sm text-muted-foreground">
            <LoaderCircle size={18} class="animate-spin text-primary" /> 로그인 처리 중...
          </div>
        {:else}
          <div bind:this={googleBtn} class="flex min-h-12 items-center justify-center"></div>
        {/if}
      {:else}
        <div class="w-full rounded-2xl border border-amber-300/20 bg-amber-400/[.06] px-5 py-4 text-center">
          <p class="text-xs font-bold tracking-[.14em] text-amber-300">GOOGLE SSO NOT CONFIGURED</p>
          <p class="mt-2 text-xs leading-5 text-muted-foreground">
            서버에 <code class="rounded bg-black/30 px-1.5 py-0.5 font-mono text-[10px] text-amber-200">GOOGLE_CLIENT_ID</code>를
            설정하면 구글 로그인이 활성화됩니다.
          </p>
        </div>
      {/if}

      {#if error}
        <p class="w-full rounded-xl border border-rose-400/20 bg-rose-500/10 px-4 py-2.5 text-center text-sm text-rose-300">{error}</p>
      {/if}
    </div>

    <p class="mt-6 text-center text-[10px] leading-4 text-white/25">
      구글 계정 이름이 배틀 닉네임으로 사용됩니다.<br />배틀 승패가 기록되고 리더보드에 반영됩니다.
    </p>
  </Card>
</div>
