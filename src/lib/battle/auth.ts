/** 인증 API — 토큰은 localStorage에 보관한다 (게임 무관). */

const TOKEN_KEY = 'neon-stack-token';

export interface AuthUser {
  id: string;
  name: string;
}

export function getToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setToken(token: string): void {
  try {
    localStorage.setItem(TOKEN_KEY, token);
  } catch {
    /* 프라이빗 모드 등 저장 불가 환경 */
  }
}

export function clearToken(): void {
  try {
    localStorage.removeItem(TOKEN_KEY);
  } catch {
    /* noop */
  }
}

async function post(path: string, body: unknown, auth = false): Promise<Response> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (auth) {
    const token = getToken();
    if (token) headers.Authorization = `Bearer ${token}`;
  }
  return fetch(path, { method: 'POST', headers, body: JSON.stringify(body) });
}

export async function register(name: string, password: string): Promise<{ token: string; user: AuthUser }> {
  const res = await post('/api/auth/register', { name, password });
  if (!res.ok) throw new Error(((await res.json()) as { error?: string }).error ?? '회원가입 실패');
  const out = (await res.json()) as { token: string; user: AuthUser };
  setToken(out.token);
  return out;
}

export async function login(name: string, password: string): Promise<{ token: string; user: AuthUser }> {
  const res = await post('/api/auth/login', { name, password });
  if (!res.ok) throw new Error(((await res.json()) as { error?: string }).error ?? '로그인 실패');
  const out = (await res.json()) as { token: string; user: AuthUser };
  setToken(out.token);
  return out;
}

export async function logout(): Promise<void> {
  try {
    await post('/api/auth/logout', {}, true);
  } catch {
    /* 서버가 죽어 있어도 로컬 토큰은 지운다 */
  }
  clearToken();
}

// ---------- Google SSO ----------

export interface GoogleConfig {
  client_id: string;
  enabled: boolean;
}

export async function getGoogleConfig(): Promise<GoogleConfig> {
  const res = await fetch('/api/auth/google/config');
  if (!res.ok) return { client_id: '', enabled: false };
  return (await res.json()) as GoogleConfig;
}

/** Google ID 토큰(credential)로 로그인/가입 — 성공 시 세션 토큰 저장 */
export async function googleLogin(credential: string): Promise<{ token: string; user: AuthUser }> {
  const res = await fetch('/api/auth/google', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ credential })
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? '구글 로그인 실패');
  }
  const out = (await res.json()) as { token: string; user: AuthUser };
  setToken(out.token);
  return out;
}

/** 현재 토큰 검증 — 유효하면 사용자, 아니면 null */
export async function me(): Promise<AuthUser | null> {
  const token = getToken();
  if (!token) return null;
  const res = await fetch('/api/auth/me', { headers: { Authorization: `Bearer ${token}` } });
  if (!res.ok) {
    clearToken();
    return null;
  }
  return (await res.json()) as AuthUser;
}
