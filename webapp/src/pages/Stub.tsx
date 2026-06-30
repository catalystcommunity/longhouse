import { useLocation } from "@solidjs/router";

export const Stub = () => {
  const loc = useLocation();
  return (
    <div
      class="reveal d1"
      style="margin-top:64px;padding:64px 24px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low);text-align:center"
    >
      <h2 style="font-family:var(--display);font-variation-settings:'opsz' 96, 'SOFT' 100, 'wght' 420;font-size:40px;color:var(--grass-4);margin:0 0 12px">
        {loc.pathname} <em style="color:var(--ocean-2);font-style:italic">— coming soon</em>
      </h2>
      <p style="font-size:15px;color:var(--ink-mute);max-width:48ch;margin:0 auto">
        This surface isn't built yet in the SPA. Once the API exposes the relevant endpoints
        we'll wire it up.
      </p>
    </div>
  );
};
