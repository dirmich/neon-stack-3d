/** Google Identity Services (gsi/client) 전역 타입 */
export {};

declare global {
  interface Window {
    google?: {
      accounts?: {
        id: {
          initialize: (opts: { client_id: string; callback: (resp: { credential: string }) => void }) => void;
          renderButton: (parent: HTMLElement, opts: Record<string, unknown>) => void;
        };
      };
    };
  }
}
