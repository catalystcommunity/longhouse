import { When, el, live } from "~/ui/dom";
import { Link } from "~/ui/router";
import { Calendar, Clock, Cloud, Home, People } from "~/components/Icons";
import { hasRole } from "~/stores/auth";
/**
 * Mobile "More" surface — every section not pinned to the bottom tabbar
 * lives here so phone users can still reach Members, Groups, Skills,
 * Settings, and Account (the same list as the tabbar's More menu). Desktop users get the full top-nav so they
 * shouldn't see this page in normal flow; the route still exists so
 * deep-links work.
 */
const SECTIONS: {
    href: string;
    label: string;
    sub: string;
    Icon: typeof Home;
}[] = [
    { href: "/members", label: "Members", sub: "Everyone in the house and their roles.", Icon: People },
    { href: "/groups", label: "Groups", sub: "Working groups across the house.", Icon: People },
    { href: "/skills", label: "Skills", sub: "Who can do what, and which groups hold which skills.", Icon: Cloud },
    { href: "/settings", label: "Settings", sub: "Rename the house, create a new one, manage features.", Icon: Calendar },
    { href: "/account", label: "Account", sub: "Your identity, houses, and sign out.", Icon: Clock },
];
export const MorePage = () => (el("div", {
    "class": "section-hd reveal",
    "style": "margin-top:12px",
    get children() {
        return [
            el("h2", {
                get children() {
                    return [
                        "More ",
                        el("em", {
                            get children() {
                                return "everything else";
                            }
                        })
                    ];
                }
            }),
            el("p", {
                "class": "lead",
                get children() {
                    return "Sections that don't fit in the bottom bar on a phone.";
                }
            }),
            el("ul", {
                "style": "list-style:none;padding:0;margin:18px 0 0;display:flex;flex-direction:column;gap:10px",
                get children() {
                    return [
                        live(() => SECTIONS.map(({ href, label, sub, Icon }) => (el("li", {
                            get children() {
                                return Link({
                                    get "href"() {
                                        return href;
                                    },
                                    "style": "display:flex;align-items:center;gap:14px;padding:14px 16px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low);text-decoration:none;color:inherit",
                                    get children() {
                                        return [
                                            el("span", {
                                                "aria-hidden": "true",
                                                "style": "width:36px;height:36px;display:grid;place-items:center;border-radius:9px;background:color-mix(in oklab, var(--grass-2) 25%, transparent);color:var(--grass-4)",
                                                get children() {
                                                    return Icon();
                                                }
                                            }),
                                            el("span", {
                                                "style": "display:flex;flex-direction:column;gap:2px",
                                                get children() {
                                                    return [
                                                        el("span", {
                                                            "style": "font-family:var(--display);font-size:18px;color:var(--ink)",
                                                            get children() {
                                                                return live(() => label);
                                                            }
                                                        }),
                                                        el("span", {
                                                            "style": "font-size:13px;color:var(--ink-mute)",
                                                            get children() {
                                                                return live(() => sub);
                                                            }
                                                        })
                                                    ];
                                                }
                                            })
                                        ];
                                    }
                                });
                            }
                        })))),
                        When({
                            get "when"() {
                                return hasRole("admin");
                            },
                            get children() {
                                return el("li", {
                                    get children() {
                                        return Link({
                                            "href": "/admin/activity",
                                            "style": "display:flex;align-items:center;gap:14px;padding:14px 16px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low);text-decoration:none;color:inherit",
                                            get children() {
                                                return [
                                                    el("span", {
                                                        "aria-hidden": "true",
                                                        "style": "width:36px;height:36px;display:grid;place-items:center;border-radius:9px;background:color-mix(in oklab, var(--grass-2) 25%, transparent);color:var(--grass-4)",
                                                        get children() {
                                                            return Cloud();
                                                        }
                                                    }),
                                                    el("span", {
                                                        "style": "display:flex;flex-direction:column;gap:2px",
                                                        get children() {
                                                            return [
                                                                el("span", {
                                                                    "style": "font-family:var(--display);font-size:18px;color:var(--ink)",
                                                                    get children() {
                                                                        return "Activity & Trash";
                                                                    }
                                                                }),
                                                                el("span", {
                                                                    "style": "font-size:13px;color:var(--ink-mute)",
                                                                    get children() {
                                                                        return "Audit log and recoverable trash bin (admins).";
                                                                    }
                                                                })
                                                            ];
                                                        }
                                                    })
                                                ];
                                            }
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
}));
