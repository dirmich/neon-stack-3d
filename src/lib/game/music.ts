/**
 * Web Audio 기반의 단순 생성형 배경음악.
 * A단조 계열 16스텝 루프 — 베이스(4스텝), 아르페지오(2스텝), 패드(8스텝).
 * AudioContext는 사용자 제스처 이후 setEnabled(true)에서 지연 생성한다.
 */
const BPM = 104;
const STEP_DURATION = 60 / BPM / 2; // 8분음표

const BASS_ROOTS = [45, 45, 48, 43]; // A1, A1, C2, G1
const ARP_NOTES = [57, 60, 64, 67, 64, 60, 57, 55]; // A3 C4 E4 G4 ...
const PAD_CHORD = [45, 57, 60, 64]; // A1 + Am

export class MusicPlayer {
  private ctx: AudioContext | null = null;
  private master: GainNode | null = null;
  private timer: ReturnType<typeof setInterval> | null = null;
  private nextTime = 0;
  private step = 0;
  private enabled = false;

  setEnabled(on: boolean): void {
    this.enabled = on;
    if (on) {
      if (!this.ctx) {
        const AudioContextCtor = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
        if (!AudioContextCtor) return;
        this.ctx = new AudioContextCtor();
        this.master = this.ctx.createGain();
        this.master.gain.value = 0.055;
        this.master.connect(this.ctx.destination);
      }
      void this.ctx.resume?.();
      this.nextTime = this.ctx.currentTime + 0.08;
      this.timer ??= setInterval(() => this.schedule(), 200);
    } else if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  /** 사용자 제스처 시점에 호출 — 지연 생성된 컨텍스트를 resume한다. */
  unlock(): void {
    if (this.ctx && this.ctx.state === 'suspended') void this.ctx.resume();
  }

  dispose(): void {
    this.setEnabled(false);
    void this.ctx?.close();
    this.ctx = null;
    this.master = null;
  }

  private schedule(): void {
    if (!this.ctx || !this.master) return;
    while (this.nextTime < this.ctx.currentTime + 0.5) {
      this.playStep(this.step, this.nextTime);
      this.step = (this.step + 1) % 16;
      this.nextTime += STEP_DURATION;
    }
  }

  private playStep(step: number, time: number): void {
    if (!this.ctx || !this.master || !this.enabled) return;
    if (step % 4 === 0) {
      this.note(BASS_ROOTS[Math.floor(step / 4) % 4], time, STEP_DURATION * 2, 'triangle', 0.5);
    }
    if (step % 2 === 0) {
      this.note(ARP_NOTES[Math.floor(step / 2) % 8], time, STEP_DURATION * 1.8, 'sine', 0.28);
    }
    if (step % 8 === 0) {
      for (const midi of PAD_CHORD.slice(1)) {
        this.note(midi, time, STEP_DURATION * 7, 'triangle', 0.1);
      }
    }
  }

  private note(midi: number, time: number, duration: number, type: OscillatorType, volume: number): void {
    if (!this.ctx || !this.master) return;
    const oscillator = this.ctx.createOscillator();
    const gain = this.ctx.createGain();
    oscillator.type = type;
    oscillator.frequency.value = 440 * 2 ** ((midi - 69) / 12);
    gain.gain.setValueAtTime(0, time);
    gain.gain.linearRampToValueAtTime(volume, time + 0.015);
    gain.gain.exponentialRampToValueAtTime(0.0001, time + duration);
    oscillator.connect(gain).connect(this.master);
    oscillator.start(time);
    oscillator.stop(time + duration + 0.05);
  }
}
