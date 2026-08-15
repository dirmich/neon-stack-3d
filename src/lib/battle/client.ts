import type { ServerMessage } from './types';

export class BattleClient {
  private ws: WebSocket | null = null;
  onMessage: ((msg: ServerMessage) => void) | null = null;
  onClose: (() => void) | null = null;

  connect(matchId: string, playerId: string): void {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const ws = new WebSocket(`${proto}://${location.host}/ws?match_id=${matchId}&player_id=${playerId}`);
    this.ws = ws;
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as ServerMessage;
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
