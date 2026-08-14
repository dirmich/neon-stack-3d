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
- 10줄 단위 레벨 상승 및 자동 속도 조절
- 브라우저 최고 점수 저장
- Web Audio 효과음과 음소거
- 키보드·터치 조작 및 반응형 레이아웃
- Three.js 조명, 그림자, 안개, 3D 시점 조작
