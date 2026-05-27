import { A } from "@solidjs/router";
import { Calendar, Clock, Cloud, Home, People, Pin } from "~/components/Icons";

/**
 * Mobile "More" surface — every section not pinned to the bottom tabbar
 * lives here so phone users can still reach Projects, Groups, Skills,
 * Settings, and Account. Desktop users get the full top-nav so they
 * shouldn't see this page in normal flow; the route still exists so
 * deep-links work.
 */

const SECTIONS: { href: string; label: string; sub: string; Icon: typeof Home }[] = [
  { href: "/projects", label: "Projects", sub: "Initiatives with milestones, owners, and tasks.", Icon: Pin },
  { href: "/groups",   label: "Groups",   sub: "Working groups across the house.", Icon: People },
  { href: "/skills",   label: "Skills",   sub: "Who can do what, and which groups hold which skills.", Icon: Cloud },
  { href: "/settings", label: "Settings", sub: "Rename the house, create a new one, manage features.", Icon: Calendar },
  { href: "/account",  label: "Account",  sub: "Your identity, houses, and sign out.", Icon: Clock },
];

export const MorePage = () => (
  <div class="section-hd reveal" style="margin-top:12px">
    <h2>More <em>everything else</em></h2>
    <p class="lead">Sections that don't fit in the bottom bar on a phone.</p>
    <ul style="list-style:none;padding:0;margin:18px 0 0;display:flex;flex-direction:column;gap:10px">
      {SECTIONS.map(({ href, label, sub, Icon }) => (
        <li>
          <A
            href={href}
            style="display:flex;align-items:center;gap:14px;padding:14px 16px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low);text-decoration:none;color:inherit"
          >
            <span
              aria-hidden="true"
              style="width:36px;height:36px;display:grid;place-items:center;border-radius:9px;background:color-mix(in oklab, var(--grass-2) 25%, transparent);color:var(--grass-4)"
            >
              <Icon />
            </span>
            <span style="display:flex;flex-direction:column;gap:2px">
              <span style="font-family:var(--display);font-size:18px;color:var(--ink)">{label}</span>
              <span style="font-size:13px;color:var(--ink-mute)">{sub}</span>
            </span>
          </A>
        </li>
      ))}
    </ul>
  </div>
);
