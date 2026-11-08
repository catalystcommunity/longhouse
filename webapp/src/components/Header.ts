import { cell, cleanup, afterMount } from "~/ui/state";
import { When, el, live } from "~/ui/dom";
import { Link, locationState, navigation } from "~/ui/router";
import { toggleScene, toggleTheme, useSceneOn } from "~/stores/theme";
import { hasRole, selectHouse, useCurrentHouseId, useHouses, useSession } from "~/stores/auth";
import { endSession } from "~/lib/session";
import { Bell, Landscape, LonghouseMark, Moon, Search, Sun } from "./Icons";
import { BugReportButton } from "./BugReportButton";
import { NotificationBell } from "./NotificationBell";
const NAV = [
    { href: "/tasks", label: "Tasks" },
    { href: "/calendar", label: "Events" },
    { href: "/projects", label: "Projects" },
    { href: "/members", label: "Members" },
    { href: "/groups", label: "Groups" },
    { href: "/skills", label: "Skills" },
    { href: "/settings", label: "Settings" },
];
export const Header = () => {
    const loc = locationState();
    const navigate = navigation();
    const sceneOn = useSceneOn();
    const session = useSession();
    const houses = useHouses();
    const currentHouseId = useCurrentHouseId();
    const isActive = (href: string) => href === "/" ? loc.pathname === "/" : loc.pathname.startsWith(href);
    const onSignOut = async () => {
        await endSession();
        navigate(import.meta.env.DEV ? "/dev-login" : "/");
    };
    // Dev uses the dev-login picker (linkkeys usually isn't running locally);
    // prod kicks off the real assertion flow at the api. The prod path needs
    // rel="external" on the anchor so the router lets the browser hit
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
    const [menuOpen, setMenuOpen] = cell(false);
    let menuWrap: HTMLDivElement | undefined;
    const closeMenu = () => setMenuOpen(false);
    const onDocClick = (e: MouseEvent) => {
        if (!menuOpen())
            return;
        if (menuWrap && !menuWrap.contains(e.target as Node))
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
    const goAccount = () => {
        closeMenu();
        navigate("/account");
    };
    const doSignOut = () => {
        closeMenu();
        onSignOut();
    };
    return (el("header", {
        "class": "head reveal",
        get children() {
            return [
                Link({
                    "href": "/",
                    get "class"() {
                        return `brand ${loc.pathname === "/" ? "active" : ""}`;
                    },
                    get "aria-current"() {
                        return loc.pathname === "/" ? "page" : undefined;
                    },
                    get children() {
                        return [
                            el("span", {
                                "class": "mark",
                                "aria-hidden": "true",
                                get children() {
                                    return LonghouseMark();
                                }
                            }),
                            "Longhouse"
                        ];
                    }
                }),
                el("nav", {
                    "class": "nav",
                    "aria-label": "Primary",
                    get children() {
                        return [
                            live(() => NAV.map((n) => (Link({
                                get "href"() {
                                    return n.href;
                                },
                                get "class"() {
                                    return isActive(n.href) ? "active" : "";
                                },
                                get children() {
                                    return live(() => n.label);
                                }
                            })))),
                            When({
                                get "when"() {
                                    return hasRole("admin");
                                },
                                get children() {
                                    return Link({
                                        "href": "/admin/activity",
                                        get "class"() {
                                            return isActive("/admin/activity") ? "active" : "";
                                        },
                                        get children() {
                                            return "Activity";
                                        }
                                    });
                                }
                            })
                        ];
                    }
                }),
                el("div", {
                    "class": "head-end",
                    get children() {
                        return [
                            BugReportButton(),
                            el("button", {
                                "class": "icon-btn",
                                get "onClick"() {
                                    return toggleScene;
                                },
                                get "aria-pressed"() {
                                    return sceneOn() ? "true" : "false";
                                },
                                "aria-label": "Toggle hero scene",
                                "title": "Hero scene",
                                get children() {
                                    return Landscape();
                                }
                            }),
                            el("button", {
                                "class": "icon-btn",
                                get "onClick"() {
                                    return toggleTheme;
                                },
                                "aria-label": "Toggle dark mode",
                                "title": "Theme",
                                get children() {
                                    return [
                                        Sun({
                                            "class": "sun"
                                        }),
                                        Moon({
                                            "class": "moon"
                                        })
                                    ];
                                }
                            }),
                            el("button", {
                                "class": "icon-btn",
                                "aria-label": "Search",
                                get children() {
                                    return Search();
                                }
                            }),
                            When({
                                get "when"() {
                                    return session();
                                },
                                get "fallback"() {
                                    return el("button", {
                                        "class": "icon-btn",
                                        "aria-label": "Notifications",
                                        get children() {
                                            return Bell();
                                        }
                                    });
                                },
                                get children() {
                                    return NotificationBell();
                                }
                            }),
                            When({
                                get "when"() {
                                    return session();
                                },
                                get "fallback"() {
                                    return el("a", {
                                        get "href"() {
                                            return signInHref;
                                        },
                                        get "rel"() {
                                            return signInRel;
                                        },
                                        "class": "btn btn-ghost",
                                        "style": "padding: 6px 14px;",
                                        get children() {
                                            return "Sign in";
                                        }
                                    });
                                },
                                get children() {
                                    return [
                                        When({
                                            get "when"() {
                                                return houses().length > 0;
                                            },
                                            get children() {
                                                return el("label", {
                                                    "class": "house-switch",
                                                    "title": "Switch house",
                                                    get children() {
                                                        return el("select", {
                                                            get "value"() {
                                                                return currentHouseId() ?? "";
                                                            },
                                                            "onChange": (e) => selectHouse(e.currentTarget.value),
                                                            get children() {
                                                                return live(() => houses().map((h) => (el("option", {
                                                                    get "value"() {
                                                                        return h.id;
                                                                    },
                                                                    get children() {
                                                                        return live(() => h.name || h.id);
                                                                    }
                                                                }))));
                                                            }
                                                        });
                                                    }
                                                });
                                            }
                                        }),
                                        el("div", {
                                            "ref": element => menuWrap = element,
                                            "style": "position:relative",
                                            get children() {
                                                return [
                                                    el("button", {
                                                        "class": "who",
                                                        "onClick": () => setMenuOpen((v) => !v),
                                                        "aria-haspopup": "menu",
                                                        get "aria-expanded"() {
                                                            return menuOpen() ? "true" : "false";
                                                        },
                                                        "title": "Account menu",
                                                        "style": "cursor:pointer",
                                                        get children() {
                                                            return [
                                                                el("span", {
                                                                    "class": "a md a1",
                                                                    get children() {
                                                                        return live(() => initial());
                                                                    }
                                                                }),
                                                                live(() => displayName().split(" ")[0])
                                                            ];
                                                        }
                                                    }),
                                                    When({
                                                        get "when"() {
                                                            return menuOpen();
                                                        },
                                                        get children() {
                                                            return el("div", {
                                                                "role": "menu",
                                                                "style": "position:absolute;top:calc(100% + 6px);right:0;min-width:180px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);box-shadow:var(--shadow-cloud);padding:6px;z-index:60;display:flex;flex-direction:column",
                                                                get children() {
                                                                    return [
                                                                        el("button", {
                                                                            "role": "menuitem",
                                                                            "type": "button",
                                                                            get "onClick"() {
                                                                                return goAccount;
                                                                            },
                                                                            "style": "display:flex;align-items:center;gap:8px;padding:8px 12px;background:transparent;border:0;border-radius:var(--r-sm,6px);font-size:14px;color:var(--ink);text-align:left;cursor:pointer",
                                                                            get children() {
                                                                                return "Account";
                                                                            }
                                                                        }),
                                                                        el("button", {
                                                                            "role": "menuitem",
                                                                            "type": "button",
                                                                            get "onClick"() {
                                                                                return doSignOut;
                                                                            },
                                                                            "style": "display:flex;align-items:center;gap:8px;padding:8px 12px;background:transparent;border:0;border-radius:var(--r-sm,6px);font-size:14px;color:var(--rust);text-align:left;cursor:pointer",
                                                                            get children() {
                                                                                return "Sign out";
                                                                            }
                                                                        })
                                                                    ];
                                                                }
                                                            });
                                                        }
                                                    })
                                                ];
                                            }
                                        })
                                    ];
                                }
                            })
                        ];
                    }
                })
            ];
        }
    }));
};
