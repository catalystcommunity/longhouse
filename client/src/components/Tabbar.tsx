import { A, useLocation } from "@solidjs/router";
import { Calendar, Clock, Home, Menu, People } from "./Icons";

const TABS = [
  { href: "/",         label: "Home",    Icon: Home },
  { href: "/tasks",    label: "Tasks",   Icon: Clock },
  { href: "/calendar", label: "Events",  Icon: Calendar },
  { href: "/members",  label: "Members", Icon: People },
  { href: "/more",     label: "More",    Icon: Menu },
];

export const Tabbar = () => {
  const loc = useLocation();
  const isActive = (href: string) =>
    href === "/" ? loc.pathname === "/" : loc.pathname.startsWith(href);

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
      </ul>
    </nav>
  );
};
