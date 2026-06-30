import { For, Show, createMemo, createSignal, onCleanup } from "solid-js";

/**
 * Material-style clock-face time picker. Two-step:
 *   1. Hour ring: 12 numbers (1..12) around the perimeter. Tap a number
 *      or drag the hand to set the hour, then we advance to the minute
 *      ring. AM/PM toggle lives in the center.
 *   2. Minute ring: 12 anchor numbers (00, 05, 10, …) but the hand
 *      snaps to whole minutes — so dragging gives single-minute fidelity.
 *
 * Pointer Events drive both tap and drag, with a single set of handlers
 * that work on touch and mouse without branching. The component is
 * controlled — parent passes `value` ("HH:mm", 24h) and gets `onChange`
 * on every commit (tap up, drag move).
 */

interface Props {
  /** "HH:mm" in 24-hour form. */
  value: string;
  onChange: (v: string) => void;
  /** Disables interaction + dims the surface (e.g. when no date is picked). */
  disabled?: boolean;
}

const SIZE = 220;
const CENTER = SIZE / 2;
const RADIUS_RING = 92;
const RADIUS_NUMBER = 78; // where the number labels sit
const HAND_LEN = 76;

const pad = (n: number) => String(n).padStart(2, "0");

const polar = (angleDeg: number, radius: number) => {
  const r = ((angleDeg - 90) * Math.PI) / 180;
  return { x: CENTER + radius * Math.cos(r), y: CENTER + radius * Math.sin(r) };
};

// Convert a 24-hour time into a {hour12, minute, period} triple.
const parse = (s: string): { h24: number; m: number } => {
  const parts = (s || "00:00").split(":");
  const h = Math.max(0, Math.min(23, Number(parts[0]) || 0));
  const m = Math.max(0, Math.min(59, Number(parts[1]) || 0));
  return { h24: h, m };
};

const HOURS_12: number[] = [12, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11];
const MINUTE_LABELS: number[] = [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55];

export const ClockTimePicker = (props: Props) => {
  const [mode, setMode] = createSignal<"hours" | "minutes">("hours");
  const [dragging, setDragging] = createSignal(false);

  let svgRef: SVGSVGElement | undefined;

  const time = createMemo(() => parse(props.value));
  const period = () => (time().h24 >= 12 ? "PM" : "AM");
  const hour12 = () => {
    const h = time().h24 % 12;
    return h === 0 ? 12 : h;
  };

  const setFromParts = (h12: number, m: number, p: "AM" | "PM") => {
    let h24 = h12 % 12;
    if (p === "PM") h24 += 12;
    props.onChange(`${pad(h24)}:${pad(m)}`);
  };

  // Pointer math: compute angle from center, snap to mode-specific
  // resolution, then commit.
  const pointerToValue = (e: PointerEvent) => {
    if (!svgRef) return;
    const rect = svgRef.getBoundingClientRect();
    // Translate viewport-relative pointer into SVG viewport coords.
    const px = ((e.clientX - rect.left) / rect.width) * SIZE;
    const py = ((e.clientY - rect.top) / rect.height) * SIZE;
    const dx = px - CENTER;
    const dy = py - CENTER;
    let angle = (Math.atan2(dy, dx) * 180) / Math.PI + 90;
    if (angle < 0) angle += 360;

    if (mode() === "hours") {
      // 12 positions, every 30°. The label "12" is at 0°.
      const idx = Math.round(angle / 30) % 12;
      const h12 = idx === 0 ? 12 : idx;
      setFromParts(h12, time().m, period());
    } else {
      // 60 positions, every 6° → minute resolution.
      const minute = Math.round(angle / 6) % 60;
      setFromParts(hour12(), minute, period());
    }
  };

  const onPointerDown = (e: PointerEvent) => {
    if (props.disabled) return;
    if (!svgRef) return;
    setDragging(true);
    svgRef.setPointerCapture(e.pointerId);
    pointerToValue(e);
  };
  const onPointerMove = (e: PointerEvent) => {
    if (!dragging() || props.disabled) return;
    pointerToValue(e);
  };
  const onPointerUp = (e: PointerEvent) => {
    if (!dragging()) return;
    setDragging(false);
    svgRef?.releasePointerCapture(e.pointerId);
    // After committing the hour, advance to minutes; from minutes we
    // stay (user can tap somewhere outside or close the picker).
    if (mode() === "hours") {
      setMode("minutes");
    }
  };
  // Defensive: if a parent removes us mid-drag we don't leak capture.
  onCleanup(() => setDragging(false));

  const handAngle = () => {
    if (mode() === "hours") {
      const idx = hour12() === 12 ? 0 : hour12();
      return idx * 30;
    }
    return time().m * 6;
  };

  const handTip = () => polar(handAngle(), HAND_LEN);

  const togglePeriod = () => {
    if (props.disabled) return;
    setFromParts(hour12(), time().m, period() === "AM" ? "PM" : "AM");
  };

  return (
    <div
      style={`display:inline-flex;flex-direction:column;align-items:center;gap:10px;padding:12px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low);${props.disabled ? "opacity:0.55;pointer-events:none" : ""}`}
    >
      <div style="display:flex;align-items:center;gap:10px;font-family:var(--display);font-size:30px;color:var(--ink);font-variation-settings:'opsz' 60, 'wght' 460">
        <button
          type="button"
          onClick={() => setMode("hours")}
          aria-pressed={mode() === "hours"}
          style={`background:transparent;border:0;cursor:pointer;font:inherit;color:${mode() === "hours" ? "var(--grass-4)" : "var(--ink-mute)"}`}
        >
          {pad(hour12())}
        </button>
        <span style="color:var(--ink-mute)">:</span>
        <button
          type="button"
          onClick={() => setMode("minutes")}
          aria-pressed={mode() === "minutes"}
          style={`background:transparent;border:0;cursor:pointer;font:inherit;color:${mode() === "minutes" ? "var(--grass-4)" : "var(--ink-mute)"}`}
        >
          {pad(time().m)}
        </button>
        <button
          type="button"
          onClick={togglePeriod}
          aria-label={`Toggle ${period() === "AM" ? "PM" : "AM"}`}
          style="margin-left:6px;background:transparent;border:1px solid var(--line);border-radius:var(--r-pill);padding:2px 10px;cursor:pointer;font-family:var(--sans);font-size:14px;font-weight:600;color:var(--ink-soft)"
        >
          {period()}
        </button>
      </div>

      <svg
        ref={svgRef!}
        width={SIZE}
        height={SIZE}
        viewBox={`0 0 ${SIZE} ${SIZE}`}
        role="application"
        aria-label={`${mode() === "hours" ? "Pick hour" : "Pick minute"}`}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        style="touch-action:none;cursor:pointer;user-select:none"
      >
        {/* Outer ring */}
        <circle
          cx={CENTER}
          cy={CENTER}
          r={RADIUS_RING}
          fill="color-mix(in oklab, var(--grass-1) 18%, var(--paper))"
          stroke="var(--line)"
          stroke-width="1"
        />
        {/* Hand */}
        <line
          x1={CENTER}
          y1={CENTER}
          x2={handTip().x}
          y2={handTip().y}
          stroke="var(--grass-4)"
          stroke-width="2.5"
          stroke-linecap="round"
        />
        <circle cx={CENTER} cy={CENTER} r="3.5" fill="var(--grass-4)" />
        <circle cx={handTip().x} cy={handTip().y} r="14" fill="var(--grass-4)" opacity="0.85" />
        {/* Number labels */}
        <Show
          when={mode() === "hours"}
          fallback={
            <For each={MINUTE_LABELS}>
              {(m) => {
                const angle = m * 6;
                const p = polar(angle, RADIUS_NUMBER);
                const onHand = time().m === m;
                return (
                  <text
                    x={p.x}
                    y={p.y + 5}
                    text-anchor="middle"
                    style={`font-family:var(--sans);font-size:13px;font-weight:600;fill:${onHand ? "white" : "var(--ink)"};pointer-events:none`}
                  >
                    {pad(m)}
                  </text>
                );
              }}
            </For>
          }
        >
          <For each={HOURS_12}>
            {(h, i) => {
              const angle = i() * 30;
              const p = polar(angle, RADIUS_NUMBER);
              const onHand = hour12() === h;
              return (
                <text
                  x={p.x}
                  y={p.y + 5}
                  text-anchor="middle"
                  style={`font-family:var(--display);font-size:18px;font-variation-settings:'opsz' 30,'wght' 460;fill:${onHand ? "white" : "var(--ink)"};pointer-events:none`}
                >
                  {h}
                </text>
              );
            }}
          </For>
        </Show>
      </svg>
    </div>
  );
};
