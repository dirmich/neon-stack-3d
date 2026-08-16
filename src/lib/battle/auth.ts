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
