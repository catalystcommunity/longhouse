/**
 * The decorative landscape behind the dashboard hero — semi-cloudy sky,
 * pale Irish sun, ocean horizon band, layered hills, grass tufts.
 * Visibility is controlled by [data-scene] on <html>; CSS hides this
 * element when scene is off.
 */
export const HeroScene = () => (
  <div class="hero-scene" aria-hidden="true">
    <div class="sun" />
    <div class="cloud c1" />
    <div class="cloud c2" />
    <div class="cloud c3" />
    <div class="ocean-band" />
    <div class="ocean-shimmer" />

    <div class="hill">
      <svg viewBox="0 0 1200 400" preserveAspectRatio="none">
        <path d="M0,200 C160,160 260,210 420,180 C560,154 720,200 900,170 C1060,144 1140,180 1200,180 L1200,400 L0,400 Z"
              style="fill:var(--scene-mid)" opacity="0.85" />
        <path d="M0,250 C180,210 340,260 520,234 C680,210 860,260 1020,240 C1120,228 1180,250 1200,240 L1200,400 L0,400 Z"
              style="fill:var(--scene-field)" opacity="0.95" />
        <path d="M0,300 C200,270 360,320 520,300 C700,278 880,320 1080,300 C1140,294 1180,308 1200,300 L1200,400 L0,400 Z"
              style="fill:var(--scene-deep)" />
        <g style="stroke:var(--scene-deep)" stroke-width="1" opacity="0.45" fill="none">
          <path d="M40 360 q2 -14 6 -22" />
          <path d="M120 350 q3 -16 8 -24" />
          <path d="M260 358 q-2 -12 -6 -22" />
          <path d="M400 345 q4 -18 9 -28" />
          <path d="M540 360 q-3 -14 -7 -24" />
          <path d="M700 348 q3 -16 9 -26" />
          <path d="M860 355 q-3 -14 -8 -22" />
          <path d="M1020 350 q4 -16 9 -26" />
          <path d="M1130 360 q-2 -12 -7 -22" />
        </g>
      </svg>
    </div>
    <div class="tufts" />
  </div>
);
