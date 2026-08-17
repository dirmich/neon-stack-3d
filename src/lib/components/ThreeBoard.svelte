<script lang="ts">
  import { onMount } from 'svelte';
  import * as THREE from 'three';
  import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
  import { RoundedBoxGeometry } from 'three/examples/jsm/geometries/RoundedBoxGeometry.js';
  import {
    BOARD_HEIGHT,
    BOARD_WIDTH,
    COLORS,
    cellsFor,
    ghostY,
    type Board,
    type Piece,
    type PieceType
  } from '../game/tetris';

  let {
    board,
    active,
    status,
    clearFlash = 0,
    interactive = true,
    showHint = true,
    items = []
  }: {
    board: Board;
    active: Piece;
    status: 'ready' | 'playing' | 'paused' | 'over';
    clearFlash?: number;
    /** 드래그/회전 활성화 (상대 보드도 독립 각도로 회전 가능) */
    interactive?: boolean;
    /** "DRAG TO VIEW" 힌트 배지 표시 여부 (기본: interactive와 동일) */
    showHint?: boolean;
    /** 아이템 배틀 모드 — 보드와 같은 크기의 아이템 셀 (이름 또는 null) */
    items?: (string | null)[][];
  } = $props();

  let canvas: HTMLCanvasElement;
  let host: HTMLDivElement;
  let scene: THREE.Scene | undefined;
  let camera: THREE.PerspectiveCamera | undefined;
  let renderer: THREE.WebGLRenderer | undefined;
  let controls: OrbitControls | undefined;
  let piecesGroup: THREE.Group | undefined;
  let ghostGroup: THREE.Group | undefined;
  let frame = 0;
  let resizeObserver: ResizeObserver | undefined;
  let needsRender = true;

  // 줄 제거 플래시
  const FLASH_DURATION = 350; // ms
  let reducedMotion = false;
  let flashMesh: THREE.Mesh | undefined;
  let flashStartedAt = 0;
  let prevClearFlash = 0;

  const blockGeometry = new RoundedBoxGeometry(0.92, 0.92, 0.92, 3, 0.09);
  const materials = new Map<PieceType, THREE.MeshStandardMaterial>();
  const ghostMaterials = new Map<PieceType, THREE.MeshBasicMaterial>();

  // 아이템 마커 — 종류별 색상
  const ITEM_COLORS: Record<string, string> = {
    attack: '#ff4d6d',
    speed: '#ffd166',
    holes: '#b07cff',
    clear: '#5be39a',
    shield: '#4dd8ff',
    slow: '#6db3ff'
  };
  const itemGeometry = new THREE.OctahedronGeometry(0.46, 0);
  const itemRingGeometry = new THREE.RingGeometry(0.32, 0.52, 28);
  const itemGroups = new Map<string, THREE.Group>();
  // 종류별 고유 3D 모양 — 지오메트리/재질 공유, 마커마다 clone(true)로 인스턴스화
  const itemShapeCache = new Map<string, THREE.Group>();
  let itemsGroup: THREE.Group | undefined;

  // 아이템 종류별로 효과가 한눈에 드러나는 모양을 만든다:
  //  attack=폭탄  speed=▲속도상승  slow=▼속도감소  holes=↔방향반전  shield=방패  clear=✨스파클
  function shapeForKind(kind: string): THREE.Group {
    const cached = itemShapeCache.get(kind);
    if (cached) return cached;
    const color = new THREE.Color(ITEM_COLORS[kind] ?? '#ffffff');
    const mat = () =>
      new THREE.MeshStandardMaterial({
        color,
        emissive: color,
        emissiveIntensity: 2.2,
        metalness: 0.05,
        roughness: 0.3,
        transparent: true,
        opacity: 0.95
      });
    const group = new THREE.Group();
    const add = (
      geo: THREE.BufferGeometry,
      material: THREE.Material,
      x = 0,
      y = 0,
      z = 0,
      rx = 0,
      ry = 0,
      rz = 0
    ) => {
      const mesh = new THREE.Mesh(geo, material);
      mesh.position.set(x, y, z);
      mesh.rotation.set(rx, ry, rz);
      group.add(mesh);
    };
    switch (kind) {
      case 'attack': {
        // 폭탄 — 저폴리 구 + 심지 + 불꽃
        add(new THREE.IcosahedronGeometry(0.3, 0), mat(), 0, 0, 0.05);
        add(new THREE.CylinderGeometry(0.045, 0.045, 0.16, 8), mat(), 0, 0.4, 0.12);
        add(
          new THREE.SphereGeometry(0.07, 8, 8),
          new THREE.MeshBasicMaterial({
            color: '#ffe27a',
            transparent: true,
            opacity: 0.95,
            blending: THREE.AdditiveBlending
          }),
          0,
          0.52,
          0.14
        );
        break;
      }
      case 'speed':
        // 속도 상승 ▲
        add(new THREE.ConeGeometry(0.3, 0.62, 4), mat(), 0, 0.1, 0.05);
        break;
      case 'slow':
        // 속도 감소 ▼ (위쪽 콘을 뒤집음)
        add(new THREE.ConeGeometry(0.3, 0.62, 4), mat(), 0, 0.1, 0.05, Math.PI);
        break;
      case 'holes': {
        // 방향키 반전 ↔ — 양방향 화살표
        add(new THREE.BoxGeometry(0.52, 0.12, 0.12), mat());
        add(new THREE.ConeGeometry(0.14, 0.22, 4), mat(), 0.34, 0, 0, 0, 0, -Math.PI / 2);
        add(new THREE.ConeGeometry(0.14, 0.22, 4), mat(), -0.34, 0, 0, 0, 0, Math.PI / 2);
        break;
      }
      case 'shield': {
        // 방패 — 세로 라운드 박스 + 십자 배지
        add(new RoundedBoxGeometry(0.34, 0.44, 0.09, 2, 0.05), mat());
        add(new THREE.BoxGeometry(0.2, 0.07, 0.14), mat(), 0, 0.02, 0.03);
        add(new THREE.BoxGeometry(0.07, 0.26, 0.14), mat(), 0, 0.02, 0.03);
        break;
      }
      case 'clear': {
        // 스파클 ✨ — 4각 별(ExtrudeGeometry)
        const shape = new THREE.Shape();
        const R = 0.34;
        const r = 0.1;
        for (let i = 0; i < 8; i++) {
          const radius = i % 2 === 0 ? R : r;
          const angle = (i / 8) * Math.PI * 2;
          const px = Math.cos(angle) * radius;
          const py = Math.sin(angle) * radius;
          if (i === 0) shape.moveTo(px, py);
          else shape.lineTo(px, py);
        }
        shape.closePath();
        const geo = new THREE.ExtrudeGeometry(shape, {
          depth: 0.08,
          bevelEnabled: true,
          bevelSize: 0.02,
          bevelThickness: 0.02,
          bevelSegments: 1
        });
        geo.center();
        add(geo, mat());
        break;
      }
      default:
        add(new THREE.OctahedronGeometry(0.46, 0), mat());
    }
    itemShapeCache.set(kind, group);
    return group;
  }

  function itemGroupFor(key: string, kind: string): THREE.Group {
    const existing = itemGroups.get(key);
    if (existing) {
      existing.visible = true;
      existing.userData.kind = kind;
      return existing;
    }
    const group = new THREE.Group();
    group.userData.kind = kind;
    // 메인 마커 — 종류별 고유 모양. 블록이 쌓여 있어도 가려지지 않게 셀 위로 떠오른다.
    const marker = new THREE.Group();
    marker.position.set(0, 0.35, 0.4);
    // clone(true): 지오메트리/재질은 공유하고 트랜스폼만 분리한다
    marker.add(shapeForKind(kind).clone(true));
    // 종류 색상 헤일로 — 가산 블렌딩으로 어두운 배경에서 발광
    const halo = new THREE.Mesh(
      itemGeometry,
      new THREE.MeshBasicMaterial({
        color: ITEM_COLORS[kind] ?? '#ffffff',
        transparent: true,
        opacity: 0.4,
        blending: THREE.AdditiveBlending,
        depthWrite: false
      })
    );
    halo.scale.setScalar(1.65);
    marker.add(halo);
    group.add(marker);
    // 셀 위치를 알려주는 발광 링 — 블록 윗면(z≈0.46) 바로 위에 붙어서,
    // 떠 있는 결정 아래로 블록이 쌓여도 아이템 셀이 어디인지 항상 보인다.
    const ring = new THREE.Mesh(
      itemRingGeometry,
      new THREE.MeshBasicMaterial({
        color: ITEM_COLORS[kind] ?? '#ffffff',
        transparent: true,
        opacity: 0.85,
        side: THREE.DoubleSide,
        depthWrite: false,
        blending: THREE.AdditiveBlending
      })
    );
    ring.rotation.x = -Math.PI / 2;
    ring.position.z = -0.05; // 결정 바닥(0.49) 아래, 블록 윗면 위
    group.add(ring);
    itemsGroup?.add(group);
    itemGroups.set(key, group);
    return group;
  }

  // 위치 키(x:y:z) → 메시 풀. 재사용해서 매 프레임 새 메시 생성을 피한다.
  const pool = new Map<string, THREE.Mesh>();

  function materialFor(type: PieceType) {
    if (!materials.has(type)) {
      const color = new THREE.Color(COLORS[type]);
      materials.set(
        type,
        new THREE.MeshStandardMaterial({
          color,
          emissive: color,
          emissiveIntensity: 0.23,
          metalness: 0.08,
          roughness: 0.27
        })
      );
    }
    return materials.get(type)!;
  }

  function ghostMaterialFor(type: PieceType) {
    if (!ghostMaterials.has(type)) {
      ghostMaterials.set(
        type,
        new THREE.MeshBasicMaterial({
          color: COLORS[type],
          transparent: true,
          opacity: 0.16,
          depthWrite: false,
          wireframe: true
        })
      );
    }
    return ghostMaterials.get(type)!;
  }

  function positionBlock(mesh: THREE.Object3D, x: number, y: number, z = 0) {
    mesh.position.set(x - (BOARD_WIDTH - 1) / 2, BOARD_HEIGHT - 1 - y, z);
  }

  function meshFor(key: string, type: PieceType, ghost: boolean): THREE.Mesh {
    const existing = pool.get(key);
    if (existing) {
      existing.material = ghost ? ghostMaterialFor(type) : materialFor(type);
      existing.visible = true;
      return existing;
    }
    const mesh = new THREE.Mesh(blockGeometry, ghost ? ghostMaterialFor(type) : materialFor(type));
    mesh.castShadow = !ghost;
    mesh.receiveShadow = !ghost;
    // 새 메시는 반드시 scene에 attach해야 렌더링된다.
    (ghost ? ghostGroup : piecesGroup)?.add(mesh);
    pool.set(key, mesh);
    return mesh;
  }

  function place(key: string, type: PieceType, x: number, y: number, z: number, ghost: boolean) {
    const mesh = meshFor(key, type, ghost);
    positionBlock(mesh, x, y, z);
  }

  function ensureFlashMesh() {
    if (flashMesh || !scene) return;
    const geometry = new THREE.PlaneGeometry(11.1, 21.1);
    flashMesh = new THREE.Mesh(
      geometry,
      new THREE.MeshBasicMaterial({
        color: '#d7ff45',
        transparent: true,
        opacity: 0,
        depthWrite: false,
        depthTest: false,
        side: THREE.DoubleSide
      })
    );
    flashMesh.position.set(0, 9.5, 0.35); // 블록(z 0.08)보다 앞
    scene.add(flashMesh);
  }

  function startClearFlash() {
    if (reducedMotion || !scene || !renderer) return;
    ensureFlashMesh();
    if (!flashMesh) return;
    flashMesh.visible = true;
    flashStartedAt = performance.now();
    needsRender = true;
  }

  function rebuildPieces() {
    if (!scene || !renderer) return;
    const wanted = new Set<string>();

    board.forEach((row, y) =>
      row.forEach((type, x) => {
        if (!type) return;
        const key = `b:${x}:${y}`;
        wanted.add(key);
        place(key, type, x, y, 0, false);
      })
    );

    // 아이템 셀 마커 (보드 위에 떠 있는 오브젝트)
    items.forEach((row, y) =>
      row.forEach((kind, x) => {
        if (!kind) return;
        const key = `i:${x}:${y}`;
        wanted.add(key);
        const group = itemGroupFor(key, kind);
        group.position.set(x - (BOARD_WIDTH - 1) / 2, BOARD_HEIGHT - 1 - y, 0.55);
      })
    );

    if (status !== 'over') {
      for (const [x, y] of cellsFor(active)) {
        if (y < 0) continue;
        const key = `a:${x}:${y}`;
        wanted.add(key);
        place(key, active.type, x, y, 0.08, false);
      }

      const landingY = ghostY(board, active);
      for (const [x, y] of cellsFor({ ...active, y: landingY })) {
        if (y < 0) continue;
        const key = `g:${x}:${y}`;
        wanted.add(key);
        place(key, active.type, x, y, -0.03, true);
      }
    }

    // 이번 상태에서 쓰이지 않는 풀 메시는 숨긴다.
    for (const [key, mesh] of pool) {
      if (!wanted.has(key)) mesh.visible = false;
    }
    for (const [key, group] of itemGroups) {
      if (!wanted.has(key)) group.visible = false;
    }
    // 자동화 검증용 훅 — 렌더링된 아이템 마커 수를 노출한다.
    if (host) host.dataset.items = String(items.flat().filter(Boolean).length);
    needsRender = true;
  }

  // 보드 프레임 실치수 + 여유(유닛). 카메라가 이 영역을 항상 전체 담도록 거리를 계산한다.
  const BOARD_FRAME_W = 11.1;
  const BOARD_FRAME_H = 21.1;
  const FIT_PAD = 2.5;

  function resize() {
    if (!host || !renderer || !camera) return;
    const width = Math.max(1, host.clientWidth);
    const height = Math.max(1, host.clientHeight);
    const aspect = width / height;
    renderer.setSize(width, height, false);
    camera.aspect = aspect;

    // 화면 비율에 맞춰 카메라 거리를 조정 — 보드가 어떤 창 크기에서도 전체가 보이게 한다.
    // 세로/가로 중 더 여유가 필요한 쪽을 기준으로 잡는다.
    const center = new THREE.Vector3(0, 9.2, 0);
    const dir = new THREE.Vector3().subVectors(center, camera.position);
    if (dir.lengthSq() < 1e-9) {
      dir.set(8.7, 1, 33.5); // 기본 방향 폴백
    }
    dir.normalize();
    const halfTan = Math.tan((34 * Math.PI) / 180 / 2);
    const needByHeight = (BOARD_FRAME_H + FIT_PAD) / (2 * halfTan);
    const needByWidth = (BOARD_FRAME_W + FIT_PAD) / (2 * halfTan * aspect);
    const dist = Math.max(needByHeight, needByWidth);
    camera.position.copy(center).addScaledVector(dir, dist);
    camera.lookAt(center);
    camera.updateProjectionMatrix();
    needsRender = true;
  }

  function createBoardFrame(targetScene: THREE.Scene) {
    const back = new THREE.Mesh(
      new RoundedBoxGeometry(11.1, 21.1, 0.34, 4, 0.16),
      new THREE.MeshStandardMaterial({ color: '#0a0d16', metalness: 0.2, roughness: 0.72 })
    );
    back.position.set(0, 9.5, -0.77);
    back.receiveShadow = true;
    targetScene.add(back);

    const gridMaterial = new THREE.LineBasicMaterial({ color: '#3e4659', transparent: true, opacity: 0.22 });
    const points: THREE.Vector3[] = [];
    for (let x = 0; x <= BOARD_WIDTH; x += 1) {
      const px = x - BOARD_WIDTH / 2;
      points.push(new THREE.Vector3(px, -0.5, -0.57), new THREE.Vector3(px, 19.5, -0.57));
    }
    for (let y = 0; y <= BOARD_HEIGHT; y += 1) {
      const py = y - 0.5;
      points.push(new THREE.Vector3(-5, py, -0.57), new THREE.Vector3(5, py, -0.57));
    }
    const geometry = new THREE.BufferGeometry().setFromPoints(points);
    targetScene.add(new THREE.LineSegments(geometry, gridMaterial));

    const railMaterial = new THREE.MeshStandardMaterial({
      color: '#252b39',
      metalness: 0.62,
      roughness: 0.27
    });
    const railGeometry = new RoundedBoxGeometry(0.28, 20.7, 0.55, 3, 0.08);
    [-5.42, 5.42].forEach((x) => {
      const rail = new THREE.Mesh(railGeometry, railMaterial);
      rail.position.set(x, 9.5, -0.28);
      rail.castShadow = true;
      targetScene.add(rail);
    });
    const base = new THREE.Mesh(new RoundedBoxGeometry(11.1, 0.34, 0.65, 3, 0.09), railMaterial);
    base.position.set(0, -0.7, -0.24);
    targetScene.add(base);
  }

  onMount(() => {
    reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    scene = new THREE.Scene();
    scene.fog = new THREE.FogExp2('#070910', 0.024);

    camera = new THREE.PerspectiveCamera(34, 1, 0.1, 100);
    camera.position.set(8.7, 10.2, 33.5);
    // 상대 보드(interactive=false)는 OrbitControls가 없어 카메라가 기본 -Z 방향을
    // 그대로 보게 된다 — 보드가 왼쪽 아래로 밀려 잘리는 버그. 항상 보드 중심을 바라본다.
    // (interactive인 경우는 OrbitControls.update()가 다시 정렬하므로 무해하다)
    camera.lookAt(0, 9.2, 0);

    renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true, powerPreference: 'high-performance' });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = 1.16;

    if (interactive) {
      controls = new OrbitControls(camera, canvas);
      controls.target.set(0, 9.2, 0);
      controls.enableDamping = !reducedMotion;
      controls.dampingFactor = 0.06;
      controls.enablePan = false;
      controls.enableZoom = false;
      controls.minAzimuthAngle = -0.48;
      controls.maxAzimuthAngle = 0.48;
      controls.minPolarAngle = 1.2;
      controls.maxPolarAngle = 1.86;
      controls.addEventListener('change', () => {
        needsRender = true;
      });
      controls.update();
    }

    scene.add(new THREE.HemisphereLight('#cce9ff', '#080912', 1.65));
    const keyLight = new THREE.DirectionalLight('#ffffff', 3.1);
    keyLight.position.set(-8, 22, 17);
    keyLight.castShadow = true;
    keyLight.shadow.mapSize.set(1024, 1024);
    scene.add(keyLight);
    const limeLight = new THREE.PointLight('#d7ff45', 18, 26, 2);
    limeLight.position.set(8, 2, 10);
    scene.add(limeLight);
    const blueLight = new THREE.PointLight('#4dbdff', 14, 25, 2);
    blueLight.position.set(-8, 18, 8);
    scene.add(blueLight);

    piecesGroup = new THREE.Group();
    ghostGroup = new THREE.Group();
    itemsGroup = new THREE.Group();
    scene.add(ghostGroup, piecesGroup, itemsGroup);

    createBoardFrame(scene);
    rebuildPieces();

    resizeObserver = new ResizeObserver(resize);
    resizeObserver.observe(host);
    resize();

    const animate = () => {
      frame = requestAnimationFrame(animate);
      controls?.update();

      // 줄 제거 플래시 페이드아웃
      if (flashMesh?.visible) {
        const elapsed = performance.now() - flashStartedAt;
        if (elapsed >= FLASH_DURATION) {
          flashMesh.visible = false;
        } else {
          const t = elapsed / FLASH_DURATION;
          (flashMesh.material as THREE.MeshBasicMaterial).opacity = 0.5 * (1 - t);
          needsRender = true;
        }
      }

      // 아이템 마커 — 맥동 + 회전 (reduced-motion이면 정지)
      if (!reducedMotion && itemsGroup?.children.length) {
        const t = performance.now();
        const pulse = 0.9 + 0.18 * Math.sin(t / 420);
        for (const g of itemsGroup.children) {
          if (g.visible && g.userData.kind) {
            g.scale.setScalar(pulse);
            g.rotation.y = t / 520;
          }
        }
        needsRender = true;
      }

      // 플레이 중에는 매 프레임 렌더(낙하/회전 반응), 그 외에는 변경 시에만 렌더.
      if (needsRender || status === 'playing') {
        renderer?.render(scene!, camera!);
        needsRender = false;
      }
    };
    animate();

    return () => {
      cancelAnimationFrame(frame);
      resizeObserver?.disconnect();
      controls?.dispose();
      renderer?.dispose();
      blockGeometry.dispose();
      itemGeometry.dispose();
      itemRingGeometry.dispose();
      materials.forEach((material) => material.dispose());
      ghostMaterials.forEach((material) => material.dispose());
      itemShapeCache.forEach((shape) =>
        shape.traverse((obj) => {
          if ((obj as THREE.Mesh).isMesh) {
            (obj as THREE.Mesh).geometry.dispose();
            ((obj as THREE.Mesh).material as THREE.Material).dispose();
          }
        })
      );
      itemGroups.clear();
      if (flashMesh) {
        flashMesh.geometry.dispose();
        (flashMesh.material as THREE.Material).dispose();
      }
      pool.clear();
    };
  });

  $effect(() => {
    board;
    active;
    status;
    clearFlash;
    items;
    rebuildPieces();
    // clearFlash 증가 시에만 플래시 연출 (reset으로 0으로 되돌아갈 때는 제외)
    if (clearFlash > prevClearFlash) {
      startClearFlash();
      prevClearFlash = clearFlash;
    }
  });
</script>  <div bind:this={host} class="relative h-full min-h-0 w-full touch-none overflow-hidden rounded-[1.35rem]">
  <canvas bind:this={canvas} class="block size-full {interactive ? 'cursor-grab active:cursor-grabbing' : ''}" aria-label="3D 테트리스 게임 보드"></canvas>
  {#if interactive && showHint}
    <div class="pointer-events-none absolute inset-x-0 top-4 flex justify-center">
      <span class="rounded-full border border-white/[.08] bg-black/25 px-3 py-1 text-[10px] font-semibold tracking-[.16em] text-white/35 backdrop-blur-md">
        DRAG TO VIEW
      </span>
    </div>
  {/if}
</div>
