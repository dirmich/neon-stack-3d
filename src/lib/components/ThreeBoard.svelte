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
    clearFlash = 0
  }: {
    board: Board;
    active: Piece;
    status: 'ready' | 'playing' | 'paused' | 'over';
    clearFlash?: number;
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

  const blockGeometry = new RoundedBoxGeometry(0.92, 0.92, 0.92, 3, 0.09);
  const materials = new Map<PieceType, THREE.MeshStandardMaterial>();

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

  function positionBlock(mesh: THREE.Object3D, x: number, y: number, z = 0) {
    mesh.position.set(x - (BOARD_WIDTH - 1) / 2, BOARD_HEIGHT - 1 - y, z);
  }

  function rebuildPieces() {
    if (!scene || !piecesGroup || !ghostGroup) return;
    piecesGroup.clear();
    ghostGroup.clear();

    board.forEach((row, y) =>
      row.forEach((type, x) => {
        if (!type) return;
        const block = new THREE.Mesh(blockGeometry, materialFor(type));
        positionBlock(block, x, y, 0);
        block.castShadow = true;
        block.receiveShadow = true;
        piecesGroup?.add(block);
      })
    );

    if (status !== 'over') {
      for (const [x, y] of cellsFor(active)) {
        if (y < 0) continue;
        const block = new THREE.Mesh(blockGeometry, materialFor(active.type));
        positionBlock(block, x, y, 0.08);
        block.castShadow = true;
        piecesGroup.add(block);
      }

      const landingY = ghostY(board, active);
      const ghostMaterial = new THREE.MeshBasicMaterial({
        color: COLORS[active.type],
        transparent: true,
        opacity: 0.16,
        depthWrite: false,
        wireframe: true
      });
      for (const [x, y] of cellsFor({ ...active, y: landingY })) {
        if (y < 0) continue;
        const block = new THREE.Mesh(blockGeometry, ghostMaterial);
        positionBlock(block, x, y, -0.03);
        ghostGroup.add(block);
      }
    }
  }

  function resize() {
    if (!host || !renderer || !camera) return;
    const width = host.clientWidth;
    const height = host.clientHeight;
    renderer.setSize(width, height, false);
    camera.aspect = width / height;
    camera.updateProjectionMatrix();
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
    scene = new THREE.Scene();
    scene.fog = new THREE.FogExp2('#070910', 0.024);

    camera = new THREE.PerspectiveCamera(34, 1, 0.1, 100);
    camera.position.set(8.7, 10.2, 33.5);

    renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true, powerPreference: 'high-performance' });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = 1.16;

    controls = new OrbitControls(camera, canvas);
    controls.target.set(0, 9.2, 0);
    controls.enableDamping = true;
    controls.dampingFactor = 0.06;
    controls.enablePan = false;
    controls.enableZoom = false;
    controls.minAzimuthAngle = -0.48;
    controls.maxAzimuthAngle = 0.48;
    controls.minPolarAngle = 1.2;
    controls.maxPolarAngle = 1.86;
    controls.update();

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

    createBoardFrame(scene);
    piecesGroup = new THREE.Group();
    ghostGroup = new THREE.Group();
    scene.add(ghostGroup, piecesGroup);
    rebuildPieces();

    resizeObserver = new ResizeObserver(resize);
    resizeObserver.observe(host);
    resize();

    const animate = () => {
      frame = requestAnimationFrame(animate);
      controls?.update();
      renderer?.render(scene!, camera!);
    };
    animate();

    return () => {
      cancelAnimationFrame(frame);
      resizeObserver?.disconnect();
      controls?.dispose();
      renderer?.dispose();
      blockGeometry.dispose();
      materials.forEach((material) => material.dispose());
    };
  });

  $effect(() => {
    board;
    active;
    status;
    clearFlash;
    rebuildPieces();
  });
</script>

<div bind:this={host} class="relative h-full min-h-0 w-full touch-none overflow-hidden rounded-[1.35rem]">
  <canvas bind:this={canvas} class="block size-full cursor-grab active:cursor-grabbing" aria-label="3D 테트리스 게임 보드"></canvas>
  <div class="pointer-events-none absolute inset-x-0 top-4 flex justify-center">
    <span class="rounded-full border border-white/[.08] bg-black/25 px-3 py-1 text-[10px] font-semibold tracking-[.16em] text-white/35 backdrop-blur-md">
      DRAG TO VIEW
    </span>
  </div>
</div>
