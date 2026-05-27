import { A, useLocation, useNavigate } from "@solidjs/router";
import { Show, createSignal, onCleanup, onMount } from "solid-js";
import { toggleScene, toggleTheme, useSceneOn } from "~/stores/theme";
import { selectHouse, signOut, useCurrentHouseId, useHouses, useSession } from "~/stores/auth";
import { Bell, Landscape, LonghouseMark, Moon, Search, Sun } from "./Icons";
import { BugReportButton } from "./BugReportButton";

const NAV = [
  { href: "/",         label: "Dashboard" },
  { href: "/tasks",    label: "Tasks" },
  { href: "/calendar", label: "Events" },
  { href: "/projects", label: "Projects" },
  { href: "/members",  label: "Members" },
  { href: "/groups",   label: "Groups" },
  { href: "/skills",   label: "Skills" },
  { href: "/settings", label: "Settings" },
];

export const Header = () => {
  const loc = useLocation();
  const navigate = useNavigate();
  const sceneOn = useSceneOn();
  const session = useSession();
  const houses = useHouses();
  const currentHouseId = useCurrentHouseId();

  const isActive = (href: string) =>
    href === "/" ? loc.pathname === "/" : loc.pathname.startsWith(href);

  const onSignOut = () => {
    signOut();
    navigate(import.meta.env.DEV ? "/dev-login" : "/");
  };

  // Dev uses the dev-login picker (linkkeys usually isn't running locally);
  // prod kicks off the real assertion flow at the api. The prod path needs
  // rel="external" on the anchor so @solidjs/router lets the browser hit
  // the api instead of routing client-side and landing on the * Stub.
  const signInHref = import.meta.env.DEV ? "/dev-login" : "/api/v1/auth/start";
  const signInRel = import.meta.env.DEV ? undefined : "external";

  // Display name first, falling back to memberId so we always show
  // *something* when authenticated. Initial = first letter of display name.
  const displayName = () => session()?.displayName ?? session()?.userId ?? "";
  const initial = () => (displayName() || "?").charAt(0).toUpperCase();

  // User menu: the .who button opens a small popover with Account + Sign
  // out so a stray tap doesn't sign people out. Click-outside and Escape
  // both dismiss it. The whole menu lives inside the relative wrapper so
  // we can position the popover absolutely.
  const [menuOpen, setMenuOpen] = createSignal(false);
  let menuWrap: HTMLDivElement | undefined;

  const closeMenu = () => setMenuOpen(false);
  const onDocClick = (e: MouseEvent) => {
    if (!menuOpen()) return;
    if (menuWrap && !menuWrap.contains(e.target as Node)) closeMenu();
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") closeMenu();
  };

  onMount(() => {
    document.addEventListener("click", onDocClick);
    document.addEventListener("keydown", onKey);
  });
  onCleanup(() => {
    document.removeEventListener("click", onDocClick);
    document.removeEventListener("keydown", onKey);
  });

  const goAccount = () => {
    closeMenu();
    navigate("/account");
  };
  const doSignOut = () => {
    closeMenu();
    onSignOut();
  };

  return (
    <header class="head reveal">
      <A href="/" class="brand">
        <span class="mark" aria-hidden="true"><LonghouseMark /></span>
        Longhouse
      </A>

      <nav class="nav" aria-label="Primary">
        {NAV.map((n) => (
          <A href={n.href} class={isActive(n.href) ? "active" : ""}>{n.label}</A>
        ))}
      </nav>

      <div class="head-end">
        <BugReportButton />
        <button
          class="icon-btn"
          onClick={toggleScene}
          aria-pressed={sceneOn() ? "true" : "false"}
          aria-label="Toggle hero scene"
          title="Hero scene"
        >
          <Landscape />
        </button>
        <button class="icon-btn" onClick={toggleTheme} aria-label="Toggle dark mode" title="Theme">
          <Sun class="sun" />
          <Moon class="moon" />
        </button>
        <button class="icon-btn" aria-label="Search"><Search /></button>
        <button class="icon-btn" aria-label="Notifications"><Bell /></button>

        <Show
          when={session()}
          fallback={
            <a href={signInHref} rel={signInRel} class="btn btn-ghost" style="padding: 6px 14px;">
              Sign in
            </a>
          }
        >
          <Show when={houses().length > 0}>
            <label class="house-switch" title="Switch house">
              <select
                value={currentHouseId() ?? ""}
                onChange={(e) => selectHouse(e.currentTarget.value)}
              >
                {houses().map((h) => (
                  <option value={h.id}>{h.name || h.id}</option>
                ))}
              </select>
            </label>
          </Show>
          <div ref={menuWrap} style="position:relative">
            <button
              class="who"
              onClick={() => setMenuOpen((v) => !v)}
              aria-haspopup="menu"
              aria-expanded={menuOpen() ? "true" : "false"}
              title="Account menu"
              style="cursor:pointer"
            >
              <span class="a md a1">{initial()}</span>
              {displayName().split(" ")[0]}
            </button>
            <Show when={menuOpen()}>
              <div
                role="menu"
                style="position:absolute;top:calc(100% + 6px);right:0;min-width:180px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);box-shadow:var(--shadow-cloud);padding:6px;z-index:60;display:flex;flex-direction:column"
              >
                <button
                  role="menuitem"
                  type="button"
                  onClick={goAccount}
                  style="display:flex;align-items:center;gap:8px;padding:8px 12px;background:transparent;border:0;border-radius:var(--r-sm,6px);font-size:14px;color:var(--ink);text-align:left;cursor:pointer"
                >
                  Account
                </button>
                <button
                  role="menuitem"
                  type="button"
                  onClick={doSignOut}
                  style="display:flex;align-items:center;gap:8px;padding:8px 12px;background:transparent;border:0;border-radius:var(--r-sm,6px);font-size:14px;color:var(--rust);text-align:left;cursor:pointer"
                >
                  Sign out
                </button>
              </div>
            </Show>
          </div>
        </Show>
      </div>
    </header>
  );
};
