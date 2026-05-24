import { A, useLocation, useNavigate } from "@solidjs/router";
import { Show } from "solid-js";
import { toggleScene, toggleTheme, useSceneOn } from "~/stores/theme";
import { selectHouse, signOut, useCurrentHouseId, useHouses, useSession } from "~/stores/auth";
import { Bell, Landscape, LonghouseMark, Moon, Search, Sun } from "./Icons";

const NAV = [
  { href: "/",         label: "Dashboard" },
  { href: "/tasks",    label: "Tasks" },
  { href: "/calendar", label: "Events" },
  { href: "/projects", label: "Projects" },
  { href: "/members",  label: "Members" },
  { href: "/shares",   label: "Shares" },
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
  // prod kicks off the real assertion flow at the api.
  const signInHref = import.meta.env.DEV ? "/dev-login" : "/api/v1/auth/start";

  // Display name first, falling back to memberId so we always show
  // *something* when authenticated. Initial = first letter of display name.
  const displayName = () => session()?.displayName ?? session()?.userId ?? "";
  const initial = () => (displayName() || "?").charAt(0).toUpperCase();

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
            <a href={signInHref} class="btn btn-ghost" style="padding: 6px 14px;">
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
          <button class="who" onClick={onSignOut} title="Click to sign out" style="cursor: pointer;">
            <span class="a md a1">{initial()}</span>
            {displayName().split(" ")[0]}
          </button>
        </Show>
      </div>
    </header>
  );
};
