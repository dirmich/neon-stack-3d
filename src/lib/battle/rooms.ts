/** 방 리스트/생성/참가 API (게임 무관). 모든 요청은 로그인 토큰을 사용한다. */

import { getToken } from './auth';
import type { LeaderboardResponse, MatchCreateResponse, RoomRow } from './protocol';

function headers(): Record<string, string> {
  const h: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getToken();
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

async function errorFrom(res: Response, fallback: string): Promise<Error> {
  let message = fallback;
  try {
    const body = (await res.json()) as { error?: string };
    if (body.error) message = body.error;
  } catch {
    /* JSON 아님 */
  }
  return new Error(message);
}

export async function listRooms(game = 'tetris'): Promise<RoomRow[]> {
  const res = await fetch(`/api/rooms?game=${encodeURIComponent(game)}`, { headers: headers() });
  if (!res.ok) throw await errorFrom(res, '방 목록을 불러오지 못했습니다');
  return (await res.json()) as RoomRow[];
}

export async function createRoom(game = 'tetris', mode: 'normal' | 'item' = 'normal'): Promise<MatchCreateResponse> {
  const res = await fetch('/api/matches', { method: 'POST', headers: headers(), body: JSON.stringify({ game, mode }) });
  if (!res.ok) throw await errorFrom(res, '방 생성 실패');
  return (await res.json()) as MatchCreateResponse;
}

/** CPU 봇과 혼자 연습하는 솔로 매치 생성 (상대는 서버 봇) */
export async function createSoloRoom(game = 'tetris', mode: 'normal' | 'item' = 'normal'): Promise<MatchCreateResponse> {
  const res = await fetch('/api/matches/solo', { method: 'POST', headers: headers(), body: JSON.stringify({ game, mode }) });
  if (!res.ok) throw await errorFrom(res, '솔로 매치 생성 실패');
  return (await res.json()) as MatchCreateResponse;
}

export async function joinRoom(code: string): Promise<MatchCreateResponse> {
  const res = await fetch('/api/matches/join', {
    method: 'POST',
    headers: headers(),
    body: JSON.stringify({ code: code.toUpperCase().trim() })
  });
  if (!res.ok) throw await errorFrom(res, '참가 실패');
  return (await res.json()) as MatchCreateResponse;
}

export async function fetchLeaderboard(limit = 10): Promise<LeaderboardResponse> {
  const res = await fetch(`/api/leaderboard?limit=${limit}`, { headers: headers() });
  if (!res.ok) throw await errorFrom(res, '리더보드를 불러오지 못했습니다');
  return (await res.json()) as LeaderboardResponse;
}
