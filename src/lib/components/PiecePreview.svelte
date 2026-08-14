<script lang="ts">
  import { COLORS, SHAPES, type PieceType } from '../game/tetris';

  let { type, size = 'md' }: { type: PieceType; size?: 'sm' | 'md' | 'lg' } = $props();

  let cells = $derived(
    SHAPES[type]
      .flatMap((row, y) => row.map((filled, x) => (filled ? { x, y } : null)))
      .filter((cell): cell is { x: number; y: number } => cell !== null)
  );
  let bounds = $derived({
    width: Math.max(...cells.map((cell) => cell.x)) + 1,
    height: Math.max(...cells.map((cell) => cell.y)) + 1
  });
  let unit = $derived(size === 'sm' ? 11 : size === 'lg' ? 22 : 16);
</script>

<div
  class="relative"
  style={`width:${bounds.width * unit}px;height:${bounds.height * unit}px`}
  aria-label={`${type} 블록`}
>
  {#each cells as cell}
    <span
      class="absolute rounded-[3px] border border-white/30 shadow-[inset_0_1px_2px_rgba(255,255,255,.35),0_0_10px_var(--glow)]"
      style={`--glow:${COLORS[type]}55;left:${cell.x * unit}px;top:${cell.y * unit}px;width:${unit - 2}px;height:${unit - 2}px;background:${COLORS[type]}`}
    ></span>
  {/each}
</div>
