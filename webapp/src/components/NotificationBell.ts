import { observe, query, cell, cleanup, afterMount } from "~/ui/state";
import { Repeat, When, live, el } from "~/ui/dom";
import { navigation } from "~/ui/router";
import { Bell } from "./Icons";
import { notificationClient } from "~/data/clients";
import { useCurrentHouseId } from "~/stores/auth";
import { lastSeenLabel } from "~/lib/derive";
import type { Notification as Notif } from "@longhouse/client";
/**
 * Header notification bell: an unread badge that polls in the background, and
 * a dropdown feed of the caller's notifications. Clicking a notification marks
 * it read (and deep-links to the target when one still exists); "Mark all
 * read" clears the badge. The feed is each member's own — the API scopes every
 * call to the caller, so there's nothing house-admin-special here.
 *
 * Notifications are self-contained snapshots, so we render actor/target/body
 * straight from the row; if the underlying task/project was deleted the text
 * still reads fine and the deep-link is simply skipped.
 */
const POLL_MS = 45000;
export const NotificationBell = () => {
    const houseId = useCurrentHouseId();
    const navigate = navigation();
    const [open, setOpen] = cell(false);
    const [unread, setUnread] = cell(0);
    let wrap: HTMLDivElement | undefined;
    const [feed, { refetch, mutate }] = query(() => (open() ? houseId() : null), async (h) => (h ? await notificationClient.listNotifications({ houseId: h, limit: 30 }) : []));
    const refreshUnread = async () => {
        const h = houseId();
        if (!h) {
            setUnread(0);
            return;
        }
        try {
            const r = await notificationClient.unreadCount(h);
            setUnread(Number(r.count ?? 0));
        }
        catch {
            /* transient — keep the last good count */
        }
    };
    const onDoc = (e: MouseEvent) => {
        if (open() && wrap && !wrap.contains(e.target as Node))
            setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
        if (e.key === "Escape")
            setOpen(false);
    };
    let timer: ReturnType<typeof setInterval> | undefined;
    afterMount(() => {
        timer = setInterval(() => void refreshUnread(), POLL_MS);
        document.addEventListener("click", onDoc);
        document.addEventListener("keydown", onKey);
    });
    cleanup(() => {
        if (timer)
            clearInterval(timer);
        document.removeEventListener("click", onDoc);
        document.removeEventListener("keydown", onKey);
    });
    // Initial fetch + refresh whenever the selected house changes.
    observe(() => {
        houseId();
        void refreshUnread();
    });
    const toggle = () => {
        const next = !open();
        setOpen(next);
        if (next)
            void refetch();
    };
    const markRead = async (n: Notif) => {
        if (n.read)
            return;
        mutate((prev) => (prev ?? []).map((x) => (x.notificationId === n.notificationId ? { ...x, read: true } : x)));
        setUnread((c) => Math.max(0, c - 1));
        try {
            await notificationClient.markRead(n.notificationId);
        }
        catch {
            void refetch();
        }
        void refreshUnread();
    };
    const activate = (n: Notif) => {
        void markRead(n);
        if (n.targetType === "project" && n.targetId) {
            setOpen(false);
            navigate(`/projects/${n.targetId}`);
        }
        else if (n.targetType === "task") {
            setOpen(false);
            navigate("/tasks");
        }
    };
    const markAll = async () => {
        const h = houseId();
        if (!h)
            return;
        mutate((prev) => (prev ?? []).map((x) => ({ ...x, read: true })));
        setUnread(0);
        try {
            await notificationClient.markAllRead(h);
        }
        catch {
            void refetch();
        }
    };
    const badge = () => (unread() > 9 ? "9+" : String(unread()));
    return (el("div", {
        "ref": element => wrap = element,
        "style": "position:relative",
        get children() {
            return [
                el("button", {
                    "class": "icon-btn",
                    get "onClick"() {
                        return toggle;
                    },
                    "aria-label": "Notifications",
                    "aria-haspopup": "menu",
                    get "aria-expanded"() {
                        return open() ? "true" : "false";
                    },
                    "style": "position:relative",
                    get children() {
                        return [
                            Bell(),
                            When({
                                get "when"() {
                                    return unread() > 0;
                                },
                                get children() {
                                    return el("span", {
                                        get "aria-label"() {
                                            return `${unread()} unread`;
                                        },
                                        "style": "position:absolute;top:-3px;right:-3px;min-width:16px;height:16px;padding:0 4px;display:flex;align-items:center;justify-content:center;background:var(--rust);color:#fff;border-radius:999px;font-size:10px;font-weight:700;line-height:1",
                                        get children() {
                                            return live(() => badge());
                                        }
                                    });
                                }
                            })
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
                            "aria-label": "Notifications",
                            "style": "position:absolute;top:calc(100% + 6px);right:0;width:340px;max-height:60vh;overflow:auto;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);box-shadow:var(--shadow-cloud);z-index:60;display:flex;flex-direction:column",
                            get children() {
                                return [
                                    el("div", {
                                        "style": "display:flex;align-items:center;justify-content:space-between;padding:10px 14px;border-bottom:1px solid var(--line);position:sticky;top:0;background:var(--paper)",
                                        get children() {
                                            return [
                                                el("span", {
                                                    "style": "font-size:13px;font-weight:600;color:var(--ink)",
                                                    get children() {
                                                        return "Notifications";
                                                    }
                                                }),
                                                When({
                                                    get "when"() {
                                                        return (feed() ?? []).some((n) => !n.read);
                                                    },
                                                    get children() {
                                                        return el("button", {
                                                            "type": "button",
                                                            get "onClick"() {
                                                                return markAll;
                                                            },
                                                            "style": "background:transparent;border:0;color:var(--ocean-2);font-size:12px;cursor:pointer;padding:0",
                                                            get children() {
                                                                return "Mark all read";
                                                            }
                                                        });
                                                    }
                                                })
                                            ];
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return !feed.loading;
                                        },
                                        get "fallback"() {
                                            return el("p", {
                                                "style": "padding:18px 14px;font-size:13px;color:var(--ink-faint)",
                                                get children() {
                                                    return "Loading\u2026";
                                                }
                                            });
                                        },
                                        get children() {
                                            return When({
                                                get "when"() {
                                                    return (feed() ?? []).length > 0;
                                                },
                                                get "fallback"() {
                                                    return el("p", {
                                                        "style": "padding:18px 14px;font-size:13px;color:var(--ink-faint)",
                                                        get children() {
                                                            return "You're all caught up.";
                                                        }
                                                    });
                                                },
                                                get children() {
                                                    return Repeat({
                                                        get "each"() {
                                                            return feed();
                                                        },
                                                        children: (n) => (el("button", {
                                                            "type": "button",
                                                            "role": "menuitem",
                                                            "onClick": () => activate(n),
                                                            get "style"() {
                                                                return `display:flex;gap:10px;align-items:flex-start;padding:10px 14px;border:0;border-bottom:1px solid var(--line);text-align:left;cursor:pointer;background:${n.read ? "transparent" : "color-mix(in oklab, var(--grass-1) 22%, transparent)"}`;
                                                            },
                                                            get children() {
                                                                return [
                                                                    el("span", {
                                                                        "aria-hidden": "true",
                                                                        get "style"() {
                                                                            return `flex:none;width:7px;height:7px;margin-top:6px;border-radius:999px;background:${n.read ? "transparent" : "var(--grass-4)"}`;
                                                                        }
                                                                    }),
                                                                    el("span", {
                                                                        "style": "flex:1;min-width:0",
                                                                        get children() {
                                                                            return [
                                                                                el("span", {
                                                                                    "style": "font-size:13px;color:var(--ink);line-height:1.35",
                                                                                    get children() {
                                                                                        return [
                                                                                            el("strong", {
                                                                                                get children() {
                                                                                                    return live(() => n.actorName || "Someone");
                                                                                                }
                                                                                            }),
                                                                                            live(() => " commented on "),
                                                                                            live(() => n.targetType ?? "a thread"),
                                                                                            When({
                                                                                                get "when"() {
                                                                                                    return n.targetTitle;
                                                                                                },
                                                                                                get children() {
                                                                                                    return [
                                                                                                        " \u201C",
                                                                                                        live(() => n.targetTitle),
                                                                                                        "\u201D"
                                                                                                    ];
                                                                                                }
                                                                                            })
                                                                                        ];
                                                                                    }
                                                                                }),
                                                                                When({
                                                                                    get "when"() {
                                                                                        return n.body;
                                                                                    },
                                                                                    get children() {
                                                                                        return el("span", {
                                                                                            "style": "display:block;font-size:12.5px;color:var(--ink-mute);margin-top:2px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap",
                                                                                            get children() {
                                                                                                return live(() => n.body);
                                                                                            }
                                                                                        });
                                                                                    }
                                                                                }),
                                                                                el("span", {
                                                                                    "style": "display:block;font-size:11px;color:var(--ink-faint);margin-top:3px",
                                                                                    get children() {
                                                                                        return live(() => lastSeenLabel({ lastSeenAt: n.createdAt }) ?? "");
                                                                                    }
                                                                                })
                                                                            ];
                                                                        }
                                                                    })
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
                        });
                    }
                })
            ];
        }
    }));
};
