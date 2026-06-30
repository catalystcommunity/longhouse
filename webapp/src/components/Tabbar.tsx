import { A, useLocation, useNavigate } from "@solidjs/router";
import { For, Show, createSignal, onCleanup, onMount } from "solid-js";
import { Calendar, Clock, Cloud, Home, Menu, People, Pin } from "./Icons";

/**
 * Mobile bottom tabbar. The first four entries deep-link to top-level
 * pages; the last "More" entry pops a sheet of the routes that don't fit
 * (Projects, Groups, Skills, Settings, Account). The deep-linkable /more
 * route is preserved as a fallback for shared URLs.
 */

const TABS = [
  { href: "/",         label: "Home",    Icon: Home },
  { href: "/tasks",    label: "Tasks",   Icon: Clock },
  { href: "/calendar", label: "Events",  Icon: Calendar },
  { href: "/members",  label: "Members", Icon: People },
];

const MORE_ITEMS: { href: string; label: string; Icon: typeof Home }[] = [
  { href: "/projects", label: "Projects", Icon: Pin },
  { href: "/groups",   label: "Groups",   Icon: People },
  { href: "/skills",   label: "Skills",   Icon: Cloud },
  { href: "/settings", label: "Settings", Icon: Calendar },
  { href: "/account",  label: "Account",  Icon: Clock },
];

export const Tabbar = () => {
  const loc = useLocation();
  const navigate = useNavigate();
  const isActive = (href: string) =>
    href === "/" ? loc.pathname === "/" : loc.pathname.startsWith(href);
  const moreActive = () => MORE_ITEMS.some((m) => isActive(m.href));

  const [open, setOpen] = createSignal(false);
  let wrapRef: HTMLLIElement | undefined;

  const closeMenu = () => setOpen(false);

  const onDocClick = (e: MouseEvent) => {
    if (!open()) return;
    if (wrapRef && !wrapRef.contains(e.target as Node)) closeMenu();
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

  const go = (href: string) => {
    closeMenu();
    navigate(href);
  };

  return (
    <nav class="tabbar" aria-label="Mobile">
      <ul>
        {TABS.map(({ href, label, Icon }) => (
          <li>
            <A href={href} class={isActive(href) ? "on" : ""}>
              <Icon />
              {label}
            </A>
          </li>
        ))}
        <li ref={wrapRef!} style="position:relative">
          <button
            type="button"
            class={moreActive() || open() ? "on" : ""}
            aria-haspopup="menu"
            aria-expanded={open() ? "true" : "false"}
            onClick={() => setOpen((v) => !v)}
            style="display:flex;flex-direction:column;align-items:center;gap:2px;padding:9px 0 6px;background:transparent;border:0;font-size:10.5px;font-weight:600;letter-spacing:0.04em;color:inherit;cursor:pointer;width:100%"
          >
            <Menu />
            More
          </button>
          <Show when={open()}>
            <div
              role="menu"
              aria-label="More navigation"
              style="position:absolute;right:0;bottom:calc(100% + 8px);min-width:200px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);box-shadow:var(--shadow-cloud);padding:6px;display:flex;flex-direction:column;z-index:60"
            >
              <For each={MORE_ITEMS}>
                {(item) => (
                  <button
                    role="menuitem"
                    type="button"
                    onClick={() => go(item.href)}
                    style={`display:flex;align-items:center;gap:10px;padding:10px 12px;background:${isActive(item.href) ? "color-mix(in oklab, var(--grass-2) 25%, transparent)" : "transparent"};border:0;border-radius:var(--r-sm,6px);font-size:14px;color:var(--ink);text-align:left;cursor:pointer`}
                  >
                    <span aria-hidden="true" style="display:grid;place-items:center;width:20px;height:20px;color:var(--grass-4)">
                      <item.Icon />
                    </span>
                    {item.label}
                  </button>
                )}
              </For>
            </div>
          </Show>
        </li>
      </ul>
    </nav>
  );
};
