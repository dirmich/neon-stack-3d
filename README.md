# NEON STACK — 3D Tetris

Svelte 5, Tailwind CSS 4, shadcn/ui 스타일 컴포넌트, Three.js로 만든 반응형 3D 테트리스입니다.

## 실행

```bash
npm install
npm run dev
```

프로덕션 빌드와 타입 검사는 다음 명령으로 실행합니다.

```bash
npm run check
npm run build
npm run preview
```

## 조작

| 동작 | 키보드 |
| --- | --- |
| 좌우 이동 | `←` `→` 또는 `A` `D` |
| 소프트 드롭 | `↓` 또는 `S` |
| 회전 | `↑` 또는 `W` |
| 하드 드롭 | `Space` |
| 블록 보관 | `C` 또는 `Shift` |
| 일시정지 | `P` 또는 `Esc` |

모바일에서는 화면의 터치 컨트롤을 사용할 수 있습니다. 3D 보드를 마우스나 손가락으로 드래그하면 시점이 바뀝니다.

## 주요 기능

- 7-bag 블록 셔플과 벽 차기 회전
- 고스트 피스, HOLD, 3개 NEXT 큐
- 소프트/하드 드롭 보너스와 동시 제거 점수
- T-spin/콤보/백투백 점수
- 10줄 단위 레벨 상승 및 자동 속도 조절
- 브라우저 최고 점수 저장
- Web Audio 효과음과 음소거
- 키보드·터치 조작 및 반응형 레이아웃
- Three.js 조명, 그림자, 안개, 3D 시점 조작
- **배틀 모드(2인)** — 줄 제거로 상대에게 가비지를 보내는 실시간 대전 (Go + Rust + PostgreSQL 백엔드)

## 배틀 모드 (2인 대전)

단일 플레이와 달리 배틀 모드는 서버가 권위(authoritative) 상태를 소유한다.

- **Go 게이트웨이** (`backend/go`) — 인증, 방 리스트/매치메이킹, WebSocket 허브, PostgreSQL 저장, Rust 연동
- **Rust 레퍼리** (`backend/rust`) — 결정적 배틀 엔진(가비지 공격/수신, 탑아웃 판정)을 독립 서비스로 수행
- **PostgreSQL** — 사용자/세션, 매치 기록, 승패, 리더보드

### 로그인 → 방 리스트 → 대전

첫 화면은 **구글 로그인(Google SSO)** 화면이다. 로그인 후 **방 리스트**가 기본 화면으로 나타난다.

1. 구글 계정으로 로그인 (계정 이름이 배틀 닉네임, 토큰은 브라우저에 유지)
2. 방 리스트에서 **방 만들기** 또는 **참가** (4자리 코드로도 참가 가능)
3. 두 명이 모이면 배틀 시작 — 줄을 지워 상대에게 가비지를 보내 승리

#### Google SSO 설정

1. Google Cloud Console → APIs & Services → Credentials → **OAuth client ID (Web application)** 생성
2. Authorized JavaScript origins에 등록: `http://localhost:3000`, `http://localhost:5173`
3. 루트에 `.env` 파일을 만들고 client id를 넣는다 (형식은 `.env.example` 참고):
   ```
   GOOGLE_CLIENT_ID=xxxxxxxx.apps.googleusercontent.com
   ```
4. `docker compose up -d --build gateway` 재시작

설정 전에는 로그인 화면에 안내가 표시된다. 서버는 Google의 `tokeninfo` 엔드포인트로 ID 토큰의
서명·만료·aud를 검증하고, `sub`/이메일 기준으로 사용자를 upsert한다.
(개발/테스트용 비밀번호 API `/api/auth/register`·`/api/auth/login`도 유지됨)

### 배틀 모듈 (게임 재사용 가능)

배틀은 특정 게임에 묶여 있지 않고 재사용 가능한 모듈로 분리돼 있다.

- **백엔드** `backend/go/internal/battle` — 게임 무관 허브(방/룸/매치/WS). 게임 규칙은 `Referee` 인터페이스로 플러그인된다 (테트리스 → `backend/rust`). 새 게임을 추가하려면 `Referee`를 구현하고 `matches.game`에 게임명을 쓰면 된다.
- **프론트엔드** `src/lib/battle/` — 게임 무관 코어(`auth.ts`·`rooms.ts`·`protocol.ts`·`client.ts`·`RoomList.svelte`)와 게임별 구현(`tetris/` 하위)이 분리돼 있다.

### 도커 실행 (전체 스택)

```bash
docker compose up -d --build
# 프론트엔드 :3000 (nginx — SPA + /api·/ws 프록시)
# Go 게이트웨이 :8000 (REST + WebSocket)
# PostgreSQL·Rust 레퍼리는 내부망 전용
```

`http://localhost:3000`으로 접속하면 배틀 모드를 포함한 전체 게임을 즐길 수 있다.

### 개발(네이티브) 실행

프론트엔드 개발 서버는 `/api`, `/ws`를 자동으로 `localhost:8000`(게이트웨이)에 프록시한다.

```bash
npm run dev
```

플레이 방법: 로그인 → 방 리스트 → 방 생성(코드 공유) 또는 참가 → 줄을 지워 상대에게 가비지를 보내 승리하세요.

배틀 조작은 단일 플레이와 동일하고(방향키/Space/Z/C), `Q` 또는 `Esc`로 대기실로 나갈 수 있습니다.
