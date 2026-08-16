import type { ServerMessage } from './protocol';

/**
 * 배틀 WebSocket 클라이언트 (게임 무관).
 * 게임별 상태 타입 S를 받아 메시지를 그대로 전달한다 — 액션 문자열도 게임이 정한다.
 */
export class BattleClient<S = unknown> {
  private ws: WebSocket | null = null;
  onMessage: ((msg: ServerMessage<S>) => void) | null = null;
  onClose: (() => void) | null = null;

  connect(matchId: string, playerId: string, token: string): void {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const ws = new WebSocket(
      `${proto}://${location.host}/ws?match_id=${encodeURIComponent(matchId)}&player_id=${encodeURIComponent(playerId)}&token=${encodeURIComponent(token)}`
    );
    this.ws = ws;
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as ServerMessage<S>;
        this.onMessage?.(msg);
      } catch {
        /* 잘못된 메시지는 무시 */
      }
    };
    ws.onclose = () => {
      this.ws = null;
      this.onClose?.();
    };
    ws.onerror = () => {
      ws.close();
    };
  }

  sendAction(action: string): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type: 'action', action }));
    }
  }

  close(): void {
    this.ws?.close();
    this.ws = null;
  }
}
