import { query, cell } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { AuthGate } from "~/components/AuthGate";
import { auditClient, trashClient } from "~/data/clients";
import { hasRole, useCurrentHouseId } from "~/stores/auth";
import type { AuditEntry, TrashItem } from "@longhouse/client";
// Admin-only "Activity & Trash": the per-house audit log (who did what) and the
// trash bin (soft-deleted items an admin can restore or purge now). Both calls
// are admin-gated server-side too; this page just hides the controls for
// non-admins. Theming is via CSS vars, so dark mode comes for free.
const when = (ts: string): string => {
    const d = new Date(ts);
    return isNaN(d.getTime()) ? ts : d.toLocaleString();
};
const actionTone = (action: string): string => {
    switch (action) {
        case "delete":
        case "purge":
        case "login_failed":
            return "var(--rust)";
        case "restore":
        case "create":
        case "login":
            return "var(--grass-4)";
        default:
            return "var(--ocean-2)";
    }
};
export const ActivityTrashPage = () => {
    const houseId = useCurrentHouseId();
    const isAdmin = () => hasRole("admin");
    const [audit, { refetch: refetchAudit }] = query(() => (isAdmin() ? houseId() : null), async (h) => (await auditClient.queryAudit({ houseId: h, limit: 100 })).entries);
    const [trash, { refetch: refetchTrash }] = query(() => (isAdmin() ? houseId() : null), async (h) => (await trashClient.listTrash({ houseId: h })).items);
    const [busy, setBusy] = cell<string | null>(null);
    const restore = async (item: TrashItem) => {
        const h = houseId();
        if (!h)
            return;
        setBusy(item.resourceId);
        try {
            await trashClient.restore({
                houseId: h,
                resourceType: item.resourceType,
                resourceId: item.resourceId,
            });
            await Promise.all([refetchTrash(), refetchAudit()]);
        }
        finally {
            setBusy(null);
        }
    };
    const purge = async (item: TrashItem) => {
        const h = houseId();
        if (!h)
            return;
        if (!window.confirm(`Permanently delete this ${item.resourceType}? This cannot be undone.`))
            return;
        setBusy(item.resourceId);
        try {
            await trashClient.purge({
                houseId: h,
                resourceType: item.resourceType,
                resourceId: item.resourceId,
            });
            await Promise.all([refetchTrash(), refetchAudit()]);
        }
        finally {
            setBusy(null);
        }
    };
    return (AuthGate({
        get children() {
            return [
                el("div", {
                    "class": "section-hd reveal",
                    get children() {
                        return [
                            el("h2", {
                                get children() {
                                    return [
                                        "Activity & Trash ",
                                        el("em", {
                                            get children() {
                                                return "house log";
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("p", {
                                "class": "lead",
                                get children() {
                                    return "Who did what, and a recoverable bin for deleted items.";
                                }
                            })
                        ];
                    }
                }),
                When({
                    get "when"() {
                        return isAdmin();
                    },
                    get "fallback"() {
                        return el("section", {
                            "class": "card reveal",
                            "style": "margin-top:0;padding:40px 24px;text-align:center",
                            get children() {
                                return [
                                    el("h3", {
                                        "style": "color:var(--grass-4);margin:0 0 6px",
                                        get children() {
                                            return "Admins only";
                                        }
                                    }),
                                    el("p", {
                                        "style": "color:var(--ink-mute);margin:0",
                                        get children() {
                                            return "The activity log and trash bin are visible to house admins.";
                                        }
                                    })
                                ];
                            }
                        });
                    },
                    get children() {
                        return [
                            el("section", {
                                "class": "card reveal d1",
                                "style": "margin-top:0;padding:8px 22px 18px",
                                get children() {
                                    return [
                                        el("div", {
                                            "class": "card-hd",
                                            get children() {
                                                return el("h3", {
                                                    get children() {
                                                        return "Trash";
                                                    }
                                                });
                                            }
                                        }),
                                        When({
                                            get "when"() {
                                                return !trash.loading;
                                            },
                                            get "fallback"() {
                                                return el("p", {
                                                    "style": "padding:18px;color:var(--ink-mute)",
                                                    get children() {
                                                        return "Loading\u2026";
                                                    }
                                                });
                                            },
                                            get children() {
                                                return When({
                                                    get "when"() {
                                                        return (trash() ?? []).length > 0;
                                                    },
                                                    get "fallback"() {
                                                        return el("p", {
                                                            "style": "padding:18px;color:var(--ink-mute)",
                                                            get children() {
                                                                return "Nothing in the trash.";
                                                            }
                                                        });
                                                    },
                                                    get children() {
                                                        return Repeat({
                                                            get "each"() {
                                                                return trash()!;
                                                            },
                                                            children: (item) => (el("div", {
                                                                "style": "padding:12px 0;border-bottom:1px solid var(--line);display:flex;align-items:center;gap:12px",
                                                                get children() {
                                                                    return [
                                                                        el("span", {
                                                                            "style": "font-size:11px;text-transform:uppercase;letter-spacing:.04em;color:var(--ink-mute);min-width:78px",
                                                                            get children() {
                                                                                return live(() => item.resourceType);
                                                                            }
                                                                        }),
                                                                        el("div", {
                                                                            "style": "flex:1;min-width:0",
                                                                            get children() {
                                                                                return [
                                                                                    el("div", {
                                                                                        "style": "color:var(--ink);overflow:hidden;text-overflow:ellipsis;white-space:nowrap",
                                                                                        get children() {
                                                                                            return live(() => item.title || item.resourceId);
                                                                                        }
                                                                                    }),
                                                                                    el("div", {
                                                                                        "style": "font-size:12px;color:var(--ink-mute)",
                                                                                        get children() {
                                                                                            return [
                                                                                                "deleted ",
                                                                                                live(() => when(item.deletedAt))
                                                                                            ];
                                                                                        }
                                                                                    })
                                                                                ];
                                                                            }
                                                                        }),
                                                                        el("button", {
                                                                            "class": "btn",
                                                                            get "disabled"() {
                                                                                return busy() === item.resourceId;
                                                                            },
                                                                            "onClick": () => restore(item),
                                                                            get children() {
                                                                                return "Restore";
                                                                            }
                                                                        }),
                                                                        el("button", {
                                                                            "class": "btn",
                                                                            "style": "color:var(--rust)",
                                                                            get "disabled"() {
                                                                                return busy() === item.resourceId;
                                                                            },
                                                                            "onClick": () => purge(item),
                                                                            get children() {
                                                                                return "Purge";
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
                            }),
                            el("section", {
                                "class": "card reveal d2",
                                "style": "padding:8px 22px 18px",
                                get children() {
                                    return [
                                        el("div", {
                                            "class": "card-hd",
                                            get children() {
                                                return el("h3", {
                                                    get children() {
                                                        return "Activity log";
                                                    }
                                                });
                                            }
                                        }),
                                        When({
                                            get "when"() {
                                                return !audit.loading;
                                            },
                                            get "fallback"() {
                                                return el("p", {
                                                    "style": "padding:18px;color:var(--ink-mute)",
                                                    get children() {
                                                        return "Loading\u2026";
                                                    }
                                                });
                                            },
                                            get children() {
                                                return When({
                                                    get "when"() {
                                                        return (audit() ?? []).length > 0;
                                                    },
                                                    get "fallback"() {
                                                        return el("p", {
                                                            "style": "padding:18px;color:var(--ink-mute)",
                                                            get children() {
                                                                return "No activity recorded yet.";
                                                            }
                                                        });
                                                    },
                                                    get children() {
                                                        return Repeat({
                                                            get "each"() {
                                                                return audit()!;
                                                            },
                                                            children: (e) => AuditRow({
                                                                get "e"() {
                                                                    return e;
                                                                }
                                                            })
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
                })
            ];
        }
    }));
};
const AuditRow = (props: {
    e: AuditEntry;
}) => {
    const e = props.e;
    const who = e.actorUserId ? `${e.actorUserId}@${e.actorDomain}` : "—";
    const target = e.resourceType ? `${e.resourceType}${e.resourceId ? " " + e.resourceId : ""}` : "";
    return (el("div", {
        "style": "padding:11px 0;border-bottom:1px solid var(--line);display:flex;align-items:baseline;gap:12px",
        get children() {
            return [
                el("span", {
                    get "style"() {
                        return `font-size:11px;text-transform:uppercase;letter-spacing:.04em;font-weight:600;min-width:92px;color:${actionTone(e.action)}`;
                    },
                    get children() {
                        return live(() => e.action);
                    }
                }),
                el("div", {
                    "style": "flex:1;min-width:0",
                    get children() {
                        return [
                            el("div", {
                                "style": "color:var(--ink);overflow:hidden;text-overflow:ellipsis;white-space:nowrap",
                                get children() {
                                    return [
                                        live(() => who),
                                        When({
                                            get "when"() {
                                                return target;
                                            },
                                            get children() {
                                                return el("span", {
                                                    "style": "color:var(--ink-mute)",
                                                    get children() {
                                                        return [
                                                            " \u00B7 ",
                                                            live(() => target)
                                                        ];
                                                    }
                                                });
                                            }
                                        }),
                                        When({
                                            get "when"() {
                                                return e.outcome && e.outcome !== "ok";
                                            },
                                            get children() {
                                                return el("span", {
                                                    "style": "color:var(--rust)",
                                                    get children() {
                                                        return [
                                                            " \u00B7 ",
                                                            live(() => e.outcome)
                                                        ];
                                                    }
                                                });
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("div", {
                                "style": "font-size:12px;color:var(--ink-mute)",
                                get children() {
                                    return [
                                        live(() => e.serviceName),
                                        ".",
                                        live(() => e.method)
                                    ];
                                }
                            })
                        ];
                    }
                }),
                el("span", {
                    "style": "font-size:12px;color:var(--ink-mute);white-space:nowrap",
                    get children() {
                        return live(() => when(e.createdAt));
                    }
                })
            ];
        }
    }));
};
