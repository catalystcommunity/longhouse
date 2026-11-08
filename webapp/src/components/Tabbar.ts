import { cell, cleanup, afterMount } from "~/ui/state";
import { Repeat, When, live, el } from "~/ui/dom";
import { Link, locationState, navigation } from "~/ui/router";
import { Calendar, Clock, Cloud, Home, Menu, People, Pin } from "./Icons";
/**
 * Mobile bottom tabbar. The first four entries (Home, Tasks, Events,
 * Projects) deep-link to top-level pages; the last "More" entry pops a
 * sheet of the routes that don't fit (Members, Groups, Skills, Settings,
 * Account). The deep-linkable /more route is preserved as a fallback for
 * shared URLs and lists the same sections.
 */
const TABS = [
    { href: "/", label: "Home", Icon: Home },
    { href: "/tasks", label: "Tasks", Icon: Clock },
    { href: "/calendar", label: "Events", Icon: Calendar },
    { href: "/projects", label: "Projects", Icon: Pin },
];
const MORE_ITEMS: {
    href: string;
    label: string;
    Icon: typeof Home;
}[] = [
    { href: "/members", label: "Members", Icon: People },
    { href: "/groups", label: "Groups", Icon: People },
    { href: "/skills", label: "Skills", Icon: Cloud },
    { href: "/settings", label: "Settings", Icon: Calendar },
    { href: "/account", label: "Account", Icon: Clock },
];
export const Tabbar = () => {
    const loc = locationState();
    const navigate = navigation();
    const isActive = (href: string) => href === "/" ? loc.pathname === "/" : loc.pathname.startsWith(href);
    const moreActive = () => MORE_ITEMS.some((m) => isActive(m.href));
    const [open, setOpen] = cell(false);
    let wrapRef: HTMLLIElement | undefined;
    const closeMenu = () => setOpen(false);
    const onDocClick = (e: MouseEvent) => {
        if (!open())
            return;
        if (wrapRef && !wrapRef.contains(e.target as Node))
            closeMenu();
    };
    const onKey = (e: KeyboardEvent) => {
        if (e.key === "Escape")
            closeMenu();
    };
    afterMount(() => {
        document.addEventListener("click", onDocClick);
        document.addEventListener("keydown", onKey);
    });
    cleanup(() => {
        document.removeEventListener("click", onDocClick);
        document.removeEventListener("keydown", onKey);
    });
    const go = (href: string) => {
        closeMenu();
        navigate(href);
    };
    return (el("nav", {
        "class": "tabbar",
        "aria-label": "Mobile",
        get children() {
            return el("ul", {
                get children() {
                    return [
                        live(() => TABS.map(({ href, label, Icon }) => (el("li", {
                            get children() {
                                return Link({
                                    get "href"() {
                                        return href;
                                    },
                                    get "class"() {
                                        return isActive(href) ? "on" : "";
                                    },
                                    get children() {
                                        return [
                                            Icon(),
                                            live(() => label)
                                        ];
                                    }
                                });
                            }
                        })))),
                        el("li", {
                            "ref": element => wrapRef = element,
                            "style": "position:relative",
                            get children() {
                                return [
                                    el("button", {
                                        "type": "button",
                                        get "class"() {
                                            return moreActive() || open() ? "on" : "";
                                        },
                                        "aria-haspopup": "menu",
                                        get "aria-expanded"() {
                                            return open() ? "true" : "false";
                                        },
                                        "onClick": () => setOpen((v) => !v),
                                        "style": "display:flex;flex-direction:column;align-items:center;gap:2px;padding:9px 0 6px;background:transparent;border:0;font-size:10.5px;font-weight:600;letter-spacing:0.04em;color:inherit;cursor:pointer;width:100%",
                                        get children() {
                                            return [
                                                Menu(),
                                                "More"
                                            ];
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return open();
                                        },
                                        get children() {
                                            return el("div", {
                                                "role": "menu",
                                                "aria-label": "More navigation",
                                                "style": "position:absolute;right:0;bottom:calc(100% + 8px);min-width:200px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);box-shadow:var(--shadow-cloud);padding:6px;display:flex;flex-direction:column;z-index:60",
                                                get children() {
                                                    return Repeat({
                                                        get "each"() {
                                                            return MORE_ITEMS;
                                                        },
                                                        children: (item) => (el("button", {
                                                            "role": "menuitem",
                                                            "type": "button",
                                                            "onClick": () => go(item.href),
                                                            get "style"() {
                                                                return `display:flex;align-items:center;gap:10px;padding:10px 12px;background:${isActive(item.href) ? "color-mix(in oklab, var(--grass-2) 25%, transparent)" : "transparent"};border:0;border-radius:var(--r-sm,6px);font-size:14px;color:var(--ink);text-align:left;cursor:pointer`;
                                                            },
                                                            get children() {
                                                                return [
                                                                    el("span", {
                                                                        "aria-hidden": "true",
                                                                        "style": "display:grid;place-items:center;width:20px;height:20px;color:var(--grass-4)",
                                                                        get children() {
                                                                            return item.Icon();
                                                                        }
                                                                    }),
                                                                    live(() => item.label)
                                                                ];
                                                            }
                                                        }))
                                                    });
                                                }
                                            });
                                        }
                                    })
                                ];
                            }
                        })
                    ];
                }
            });
        }
    }));
};
