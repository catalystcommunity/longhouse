import type { JSX } from "solid-js";
import { Show } from "solid-js";
import { useCurrentHouseId, useSession } from "~/stores/auth";

/**
 * Wraps a page's body. When the caller has no session, renders a
 * "you're not signed in" CTA card (link to /dev-login in dev, to the
 * api's /auth/start in prod — same rel="external" trick so Solid Router
 * lets the browser hit the api). When signed in but no house is yet
 * selected, shows a short status while loadHouses fills in.
 *
 * Pages stay simple: they wrap their content in <AuthGate> and the
 * fallback handling lives once, here.
 */
export const AuthGate = (props: { children: JSX.Element }) => {
  const session = useSession();
  const houseId = useCurrentHouseId();
  return (
    <Show when={session()} fallback={<SignedOutCTA />}>
      <Show when={houseId()} fallback={<HousePicker />}>
        {props.children}
      </Show>
    </Show>
  );
};

const signInHref = import.meta.env.DEV ? "/dev-login" : "/api/v1/auth/start";
const signInRel = import.meta.env.DEV ? undefined : "external";

const SignedOutCTA = () => (
  <section
    class="reveal d1"
    style="margin-top:48px;padding:48px 28px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low);text-align:center;max-width:560px;margin-left:auto;margin-right:auto"
  >
    <h2
      style="font-family:var(--display);font-variation-settings:'opsz' 80, 'SOFT' 100, 'wght' 420;font-size:32px;color:var(--grass-4);margin:0 0 12px"
    >
      You're not signed in
    </h2>
    <p style="font-size:15px;color:var(--ink-mute);max-width:46ch;margin:0 auto 24px">
      Sign in to see your house's tasks, calendar, projects, and members.
    </p>
    <a href={signInHref} rel={signInRel} class="btn btn-primary" style="padding:10px 22px">
      Sign in
    </a>
  </section>
);

const HousePicker = () => (
  <section
    class="reveal d1"
    style="margin-top:48px;padding:32px 24px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);text-align:center;max-width:520px;margin-left:auto;margin-right:auto"
  >
    <p style="margin:0;color:var(--ink-mute)">
      Loading your houses…
    </p>
  </section>
);
