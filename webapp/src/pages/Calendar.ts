import { observe, computed, query, cell, cleanup, afterMount } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { searchState } from "~/ui/router";
import { ChevronLeft, ChevronRight, People, Plus } from "~/components/Icons";
import { AuthGate } from "~/components/AuthGate";
import { DateTimePicker } from "~/components/DateTimePicker";
import { RecurrenceFields, recurrenceLabel, toRecurrence, type RecurrenceFreq } from "~/components/RecurrenceFields";
import { calendarClient, eventClient, memberClient } from "~/data/clients";
import { CalendarConnection } from "~/components/CalendarConnection";
import { displayName, eventTone, timeLabel, ymdLocal } from "~/lib/derive";
import { normalizeEventCreate } from "~/lib/taskCreate";
import { loadEvents, startOfRange, viewRange, VIEW_DAYS, type CalendarViewMode as ViewMode } from "~/lib/events";
import { currentMemberId, hasRole, useCurrentHouseId } from "~/stores/auth";
import { buildCells, groupSingleByDate, placeSpans } from "~/lib/month";
import type { CalendarSubscription, Event as ApiEvent } from "@longhouse/client";
const WEEKDAYS_MON_FIRST = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const MONTH_NAMES = [
    "January", "February", "March", "April", "May", "June",
    "July", "August", "September", "October", "November", "December",
];
// Time-grid configuration. Hours rendered from 6am to 11pm — enough for
// a working day plus evenings, without scrolling away the empty pre-dawn
// hours. Each cell is one hour tall; drag-select snaps to hour bounds.
const GRID_START_HOUR = 6;
const GRID_END_HOUR = 23;
const GRID_HOURS = GRID_END_HOUR - GRID_START_HOUR + 1;
const HOUR_HEIGHT = 44; // px per hour row
export const CalendarPage = () => {
    const houseId = useCurrentHouseId();
    const [calendarEnabled, setCalendarEnabled] = cell(false);
    const [localRefresh, setLocalRefresh] = cell(0);
    const [connectionError, setConnectionError] = cell("");
    const [view, setView] = cell<ViewMode>("month");
    const [anchor, setAnchor] = cell(new Date());
    const [allEvents, { refetch: refetchEvents }] = query(() => {
        const h = houseId();
        if (!h)
            return null;
        const { start, end } = viewRange(view(), anchor());
        return { houseId: h, rangeStart: start.toISOString(), rangeEnd: end.toISOString() };
    }, (request) => loadEvents(eventClient, request));
    const [members] = query(() => houseId(), async (h) => memberClient.listMembers({ houseId: h }));
    // Calendar sharing — like any calendar app, you see your own calendar by
    // default and opt other people's calendars in. Your view (which members
    // you've ADDED, and which of those are ENABLED) is persisted server-side per
    // member, so it follows you across devices and survives logout. A subject's
    // events render only when you've both added them AND left them enabled. Each
    // member's events keep their own owner-derived color, so overlaid calendars
    // read as distinct from yours.
    const me = () => currentMemberId();
    const [savedView] = query(() => houseId(), async (h) => (await eventClient.getCalendarView(h)).subscriptions ?? []);
    // Local, optimistic copy of the roster; seeded from the server view, and
    // written straight back to the server on every edit.
    const [subs, setSubs] = cell<CalendarSubscription[]>([]);
    observe(() => setSubs(savedView() ?? []));
    const persist = (next: CalendarSubscription[]) => {
        setSubs(next);
        const h = houseId();
        const viewer = me();
        if (!h || !viewer)
            return;
        // Fire-and-forget: the local signal already reflects the change; a reload
        // re-reads the server truth if the write ever fails.
        void eventClient
            .setCalendarView({ houseId: h, viewerMemberId: viewer, subscriptions: next })
            .catch(() => { });
    };
    const addSubject = (memberId: string) => {
        if (subs().some((s) => s.subjectMemberId === memberId))
            return;
        persist([...subs(), { subjectMemberId: memberId, enabled: true }]);
    };
    const setEnabled = (memberId: string, enabled: boolean) => persist(subs().map((s) => (s.subjectMemberId === memberId ? { ...s, enabled } : s)));
    const removeSubject = (memberId: string) => persist(subs().filter((s) => s.subjectMemberId !== memberId));
    const enabledCount = () => subs().filter((s) => s.enabled).length;
    const [peopleOpen, setPeopleOpen] = cell(false);
    // The set of member ids whose calendars are visible: always me, plus any
    // subscription that is currently enabled.
    const visibleOwnerIds = computed(() => {
        const ids = new Set<string>();
        const mine = me();
        if (mine)
            ids.add(mine);
        for (const s of subs())
            if (s.enabled)
                ids.add(s.subjectMemberId);
        return ids;
    });
    // Composer state. `{ mode: "create" }` opens a blank form; with optional
    // `start`/`end` ISO strings to pre-fill the time range (cell-click or
    // drag-select). `{ mode: "edit", event }` pre-fills from an existing row.
    type ComposerState = {
        mode: "create";
        start?: string;
        end?: string;
        allDay?: boolean;
    } | {
        mode: "edit";
        event: ApiEvent;
    };
    const [composer, setComposer] = cell<ComposerState | null>(null);
    // Deep-link via query params: /calendar?view=day&date=YYYY-MM-DD&event=ID.
    // Lets the dashboard's Next-event card jump straight to the day view and
    // optionally pop the event editor. The params are consumed (set to
    // undefined) once applied so a later in-app nav back to /calendar
    // doesn't keep re-opening the editor.
    const [searchParams, setSearchParams] = searchState<{
        view?: string;
        date?: string;
        event?: string;
        act_as_grant_id?: string;
        nonce?: string;
    }>();
    let completingConnection = false;
    const [linkedEvent] = query(() => {
        const id = searchParams.event;
        if (!id || allEvents.loading || allEvents.error || (allEvents() ?? []).some(event => event.eventId === id))
            return null;
        return id;
    }, id => eventClient.getEvent(id));
    observe(() => {
        const grantId = searchParams.act_as_grant_id, nonce = searchParams.nonce;
        if (!houseId() || !grantId || !nonce || completingConnection)
            return;
        completingConnection = true;
        const connectHouse = sessionStorage.getItem("calendar-connect-house");
        if (!connectHouse) {
            setConnectionError("Calendar connection expired. Start again.");
            return;
        }
        void calendarClient.completeConnect({ houseId: connectHouse, grantId, nonce }).then(() => {
            sessionStorage.removeItem("calendar-connect-house");
            setSearchParams({ act_as_grant_id: undefined, nonce: undefined }, { replace: true });
            setLocalRefresh(n => n + 1);
        }).catch(e => setConnectionError(String(e)));
    });
    observe(() => {
        const v = searchParams.view;
        const d = searchParams.date;
        if (v && (v === "month" || v === "week" || v === "3day" || v === "day")) {
            setView(v);
        }
        if (d && /^\d{4}-\d{2}-\d{2}$/.test(d)) {
            const [y, m, day] = d.split("-").map(Number);
            setAnchor(new Date(y, m - 1, day));
        }
    });
    observe(() => {
        const evId = searchParams.event;
        if (evId) {
            const match = (allEvents() ?? []).find((e) => e.eventId === evId) ?? (!linkedEvent.error ? linkedEvent() : undefined);
            if (match) {
                if (match.startsAt) {
                    const date = new Date(match.startsAt);
                    const range = viewRange(view(), anchor());
                    if (date < range.start || date >= range.end)
                        setAnchor(date);
                }
                setComposer({ mode: "edit", event: match });
                setSearchParams({ view: undefined, date: undefined, event: undefined }, { replace: true });
            }
        }
    });
    const goPrev = () => setAnchor((a) => shiftAnchor(a, view(), -1));
    const goNext = () => setAnchor((a) => shiftAnchor(a, view(), 1));
    const goToday = () => setAnchor(new Date());
    // Visible events come straight from the server now — the recurrence
    // worker spawns real Event rows for each occurrence up to a 2-year
    // horizon, so there's nothing to expand client-side. We still tag
    // children with the 🔁 indicator so users know they're part of a series.
    // Filtered to the calendars the viewer has chosen to see (their own plus
    // any explicitly added members) — you don't see everyone's calendar by
    // default. Past events are kept: the calendar is where you go to find them.
    const visibleEvents = computed(() => {
        const owners = visibleOwnerIds();
        return (allEvents() ?? []).filter((e) => owners.has(e.ownerMemberId));
    });
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
                                        "Calendar ",
                                        el("em", {
                                            get children() {
                                                return "\u2014 the month at a glance";
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("p", {
                                "class": "lead",
                                get children() {
                                    return "Switch views in the toolbar. Click an empty cell (month view) or drag across hours (day/week views) to start a new event.";
                                }
                            })
                        ];
                    }
                }),
                When({
                    get "when"() {
                        return connectionError();
                    },
                    get children() {
                        return el("p", {
                            "role": "alert",
                            get children() {
                                return live(() => connectionError());
                            }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return linkedEvent.error;
                    },
                    get children() {
                        return el("p", {
                            "role": "alert",
                            get children() {
                                return [
                                    "Could not open the linked event: ",
                                    live(() => String(linkedEvent.error))
                                ];
                            }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return houseId();
                    },
                    children: id => CalendarConnection({
                        get "houseId"() {
                            return id();
                        },
                        get "refresh"() {
                            return localRefresh();
                        },
                        "onEdit": event => setComposer({ mode: "edit", event }),
                        get "onEnabled"() {
                            return setCalendarEnabled;
                        }
                    })
                }),
                When({
                    get "when"() {
                        return composer();
                    },
                    "keyed": true,
                    children: (state) => (EventComposer({
                        get "houseId"() {
                            return houseId()!;
                        },
                        get "state"() {
                            return state;
                        },
                        get "calendarEnabled"() {
                            return calendarEnabled();
                        },
                        get "members"() {
                            return members() ?? [];
                        },
                        "onClose": () => setComposer(null),
                        "onOpen": event => setComposer({ mode: "edit", event }),
                        "onSaved": async () => { setComposer(null); setLocalRefresh(n => n + 1); await refetchEvents(); }
                    }))
                }),
                el("section", {
                    "class": "cal reveal d1",
                    "data-honor": "false",
                    get children() {
                        return [
                            el("div", {
                                "class": "cal-hd",
                                get children() {
                                    return [
                                        el("div", {
                                            "class": "cal-title",
                                            get children() {
                                                return [
                                                    el("h3", {
                                                        "class": "cal-month",
                                                        get children() {
                                                            return live(() => viewTitle(view(), anchor()));
                                                        }
                                                    }),
                                                    el("span", {
                                                        "class": "cal-sub",
                                                        get children() {
                                                            return [
                                                                live(() => visibleEvents().length),
                                                                " shown",
                                                                When({
                                                                    get "when"() {
                                                                        return enabledCount() > 0;
                                                                    },
                                                                    get children() {
                                                                        return [
                                                                            live(() => " "),
                                                                            "\u00B7 you + ",
                                                                            live(() => enabledCount()),
                                                                            " ",
                                                                            live(() => enabledCount() === 1 ? "calendar" : "calendars")
                                                                        ];
                                                                    }
                                                                })
                                                            ];
                                                        }
                                                    })
                                                ];
                                            }
                                        }),
                                        el("div", {
                                            "class": "cal-controls",
                                            get children() {
                                                return [
                                                    el("div", {
                                                        "class": "cal-nav",
                                                        get children() {
                                                            return [
                                                                el("button", {
                                                                    "class": "icon-btn",
                                                                    get "onClick"() {
                                                                        return goPrev;
                                                                    },
                                                                    "aria-label": "Previous",
                                                                    get children() {
                                                                        return ChevronLeft();
                                                                    }
                                                                }),
                                                                el("button", {
                                                                    "class": "icon-btn today",
                                                                    get "onClick"() {
                                                                        return goToday;
                                                                    },
                                                                    "aria-label": "Today",
                                                                    get children() {
                                                                        return "Today";
                                                                    }
                                                                }),
                                                                el("button", {
                                                                    "class": "icon-btn",
                                                                    get "onClick"() {
                                                                        return goNext;
                                                                    },
                                                                    "aria-label": "Next",
                                                                    get children() {
                                                                        return ChevronRight();
                                                                    }
                                                                })
                                                            ];
                                                        }
                                                    }),
                                                    el("div", {
                                                        "style": "position:relative",
                                                        get children() {
                                                            return [
                                                                el("button", {
                                                                    "class": "btn-quiet",
                                                                    "onClick": () => setPeopleOpen((v) => !v),
                                                                    "aria-haspopup": "menu",
                                                                    get "aria-expanded"() {
                                                                        return peopleOpen() ? "true" : "false";
                                                                    },
                                                                    "title": "Choose whose calendars to show",
                                                                    get children() {
                                                                        return [
                                                                            People(),
                                                                            " People",
                                                                            When({
                                                                                get "when"() {
                                                                                    return enabledCount() > 0;
                                                                                },
                                                                                get children() {
                                                                                    return [
                                                                                        " \u00B7 ",
                                                                                        live(() => enabledCount())
                                                                                    ];
                                                                                }
                                                                            })
                                                                        ];
                                                                    }
                                                                }),
                                                                When({
                                                                    get "when"() {
                                                                        return peopleOpen();
                                                                    },
                                                                    get children() {
                                                                        return PeoplePicker({
                                                                            get "members"() {
                                                                                return members() ?? [];
                                                                            },
                                                                            get "meId"() {
                                                                                return me();
                                                                            },
                                                                            get "subs"() {
                                                                                return subs();
                                                                            },
                                                                            get "onAdd"() {
                                                                                return addSubject;
                                                                            },
                                                                            get "onSetEnabled"() {
                                                                                return setEnabled;
                                                                            },
                                                                            get "onRemove"() {
                                                                                return removeSubject;
                                                                            },
                                                                            "onClose": () => setPeopleOpen(false)
                                                                        });
                                                                    }
                                                                })
                                                            ];
                                                        }
                                                    }),
                                                    el("div", {
                                                        "class": "seg",
                                                        "role": "tablist",
                                                        "style": "display:inline-flex;border:1px solid var(--line);border-radius:var(--r-md);overflow:hidden",
                                                        get children() {
                                                            return Repeat({
                                                                get "each"() {
                                                                    return [
                                                                        ["day", "Day"],
                                                                        ["3day", "3-day"],
                                                                        ["week", "Week"],
                                                                        ["month", "Month"],
                                                                    ] as const;
                                                                },
                                                                children: ([v, label]) => (el("button", {
                                                                    "onClick": () => setView(v),
                                                                    get "class"() {
                                                                        return view() === v ? "on" : "";
                                                                    },
                                                                    get "style"() {
                                                                        return `padding:5px 12px;background:${view() === v ? "var(--ocean-1)" : "var(--paper)"};color:${view() === v ? "var(--paper)" : "var(--ink)"};border:0;border-right:1px solid var(--line);font-size:12px;cursor:pointer`;
                                                                    },
                                                                    get children() {
                                                                        return live(() => label);
                                                                    }
                                                                }))
                                                            });
                                                        }
                                                    }),
                                                    el("button", {
                                                        "class": "btn-quiet",
                                                        "onClick": () => setComposer({ mode: "create" }),
                                                        get children() {
                                                            return [
                                                                Plus(),
                                                                " New event"
                                                            ];
                                                        }
                                                    })
                                                ];
                                            }
                                        })
                                    ];
                                }
                            }),
                            When({
                                get "when"() {
                                    return view() === "month";
                                },
                                get children() {
                                    return MonthView({
                                        get "anchor"() {
                                            return anchor();
                                        },
                                        get "events"() {
                                            return visibleEvents();
                                        },
                                        "onCreate": (ymd) => setComposer({ mode: "create", start: dateAt(ymd, 9, 0), end: dateAt(ymd, 10, 0) }),
                                        "onEdit": (e) => setComposer({ mode: "edit", event: e })
                                    });
                                }
                            }),
                            When({
                                get "when"() {
                                    return view() !== "month";
                                },
                                get children() {
                                    return TimeGridView({
                                        get "anchor"() {
                                            return anchor();
                                        },
                                        get "days"() {
                                            return VIEW_DAYS[view() as Exclude<ViewMode, "month">];
                                        },
                                        get "events"() {
                                            return visibleEvents();
                                        },
                                        "onCreate": (startIso, endIso) => setComposer({ mode: "create", start: startIso, end: endIso }),
                                        "onEdit": (e) => setComposer({ mode: "edit", event: e })
                                    });
                                }
                            })
                        ];
                    }
                })
            ];
        }
    }));
};
// ─── People picker (whose calendars to show) ──────────────────────────
//
// Two tiers, like a normal calendar app's "Other calendars":
//   • "You" — always shown, non-removable baseline.
//   • Your roster — members you've ADDED. Each has a checkbox (ENABLED = shown)
//     and an × (remove from roster). A subject's events appear only when added
//     AND enabled, so unchecking hides without forgetting the choice.
//   • "Add someone" — a select of house members not yet in your roster.
// The color swatch mirrors the event tone so the legend maps onto the calendar.
const TONE_VAR: Record<string, string> = {
    ocean: "var(--ocean-1)",
    grass: "var(--grass-3)",
    sky: "var(--sky-3)",
    heather: "var(--heather)",
    moss: "var(--moss)",
};
type PickerMember = {
    memberId: string;
    displayName?: string;
    handle?: string;
    email?: string;
    linkkeysUserId?: string;
};
const PeoplePicker = (props: {
    members: PickerMember[];
    meId?: string | null;
    subs: CalendarSubscription[];
    onAdd: (memberId: string) => void;
    onSetEnabled: (memberId: string, enabled: boolean) => void;
    onRemove: (memberId: string) => void;
    onClose: () => void;
}) => {
    let wrap: HTMLDivElement | undefined;
    const onDocClick = (e: MouseEvent) => {
        if (wrap && !wrap.contains(e.target as Node))
            props.onClose();
    };
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape")
        props.onClose(); };
    afterMount(() => {
        // Defer so the opening click doesn't immediately close the panel.
        setTimeout(() => document.addEventListener("click", onDocClick), 0);
        document.addEventListener("keydown", onKey);
    });
    cleanup(() => {
        document.removeEventListener("click", onDocClick);
        document.removeEventListener("keydown", onKey);
    });
    const byId = () => {
        const map = new Map<string, PickerMember>();
        for (const m of props.members)
            map.set(m.memberId, m);
        return map;
    };
    const swatch = (memberId: string) => TONE_VAR[eventTone(memberId)] ?? "var(--grass-3)";
    // House members not yet in the roster (and not me) — the "add" candidates.
    const candidates = () => {
        const inRoster = new Set(props.subs.map((s) => s.subjectMemberId));
        return props.members.filter((m) => m.memberId !== props.meId && !inRoster.has(m.memberId));
    };
    const nameFor = (id: string) => {
        const m = byId().get(id);
        return m ? displayName(m) : id;
    };
    return (el("div", {
        "ref": element => wrap = element,
        "role": "menu",
        "aria-label": "Calendars to show",
        "style": "position:absolute;top:calc(100% + 6px);right:0;min-width:250px;max-height:360px;overflow-y:auto;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);box-shadow:var(--shadow-cloud);padding:6px;z-index:60",
        get children() {
            return [
                el("div", {
                    "style": "padding:6px 10px;font-size:11px;letter-spacing:0.08em;text-transform:uppercase;color:var(--ink-mute)",
                    get children() {
                        return "Calendars";
                    }
                }),
                el("div", {
                    "style": "display:flex;align-items:center;gap:8px;padding:8px 10px;font-size:14px;color:var(--ink)",
                    get children() {
                        return [
                            el("span", {
                                get "style"() {
                                    return `width:10px;height:10px;border-radius:3px;flex-shrink:0;background:${props.meId ? swatch(props.meId) : "var(--grass-3)"}`;
                                }
                            }),
                            el("span", {
                                "style": "flex:1",
                                get children() {
                                    return "You";
                                }
                            }),
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
                                get children() {
                                    return "always shown";
                                }
                            })
                        ];
                    }
                }),
                When({
                    get "when"() {
                        return props.subs.length > 0;
                    },
                    get children() {
                        return Repeat({
                            get "each"() {
                                return props.subs;
                            },
                            children: (s) => (el("div", {
                                "style": "display:flex;align-items:center;gap:8px;padding:8px 10px;font-size:14px;color:var(--ink);border-radius:var(--r-sm,6px)",
                                get children() {
                                    return [
                                        el("input", {
                                            "type": "checkbox",
                                            get "checked"() {
                                                return s.enabled;
                                            },
                                            "onChange": (e) => props.onSetEnabled(s.subjectMemberId, e.currentTarget.checked),
                                            get "title"() {
                                                return s.enabled ? "Enabled — showing on your calendar" : "Added but hidden";
                                            },
                                            "style": "cursor:pointer"
                                        }),
                                        el("span", {
                                            get "style"() {
                                                return `width:10px;height:10px;border-radius:3px;flex-shrink:0;background:${swatch(s.subjectMemberId)};opacity:${s.enabled ? "1" : "0.35"}`;
                                            }
                                        }),
                                        el("span", {
                                            "style": "flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap",
                                            get children() {
                                                return live(() => nameFor(s.subjectMemberId));
                                            }
                                        }),
                                        el("button", {
                                            "type": "button",
                                            "onClick": () => props.onRemove(s.subjectMemberId),
                                            "title": "Remove from your calendars",
                                            "style": "background:transparent;border:0;color:var(--ink-mute);cursor:pointer;font-size:15px;line-height:1;padding:0 2px",
                                            get children() {
                                                return "\u00D7";
                                            }
                                        })
                                    ];
                                }
                            }))
                        });
                    }
                }),
                el("div", {
                    "style": "border-top:1px solid var(--line);margin:6px 4px 0;padding-top:8px",
                    get children() {
                        return When({
                            get "when"() {
                                return candidates().length > 0;
                            },
                            get "fallback"() {
                                return el("div", {
                                    "style": "padding:4px 6px;font-size:12px;color:var(--ink-mute)",
                                    get children() {
                                        return live(() => props.members.filter((m) => m.memberId !== props.meId).length === 0
                                            ? "No other members yet."
                                            : "Everyone's added.");
                                    }
                                });
                            },
                            get children() {
                                return el("label", {
                                    "style": "display:flex;flex-direction:column;gap:4px;padding:0 6px",
                                    get children() {
                                        return [
                                            el("span", {
                                                "style": "font-size:11px;color:var(--ink-mute)",
                                                get children() {
                                                    return "Add a calendar";
                                                }
                                            }),
                                            el("select", {
                                                "value": "",
                                                "onChange": (e) => { const v = e.currentTarget.value; if (v)
                                                    props.onAdd(v); e.currentTarget.value = ""; },
                                                "style": "padding:6px 8px;border:1px solid var(--line);border-radius:var(--r-sm,6px);background:var(--paper);font-size:13px;cursor:pointer",
                                                get children() {
                                                    return [
                                                        el("option", {
                                                            "value": "",
                                                            get children() {
                                                                return "+ add someone\u2026";
                                                            }
                                                        }),
                                                        Repeat({
                                                            get "each"() {
                                                                return candidates();
                                                            },
                                                            children: (m) => el("option", {
                                                                get "value"() {
                                                                    return m.memberId;
                                                                },
                                                                get children() {
                                                                    return live(() => displayName(m));
                                                                }
                                                            })
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
    }));
};
// ─── Month view ───────────────────────────────────────────────────────
const MonthView = (props: {
    anchor: Date;
    events: ApiEvent[];
    onCreate: (ymd: string) => void;
    onEdit: (e: ApiEvent) => void;
}) => {
    const year = () => props.anchor.getFullYear();
    const month = () => props.anchor.getMonth();
    const todayYMD = (() => {
        const d = new Date();
        return ymdLocal(d.toISOString());
    })();
    const cells = computed(() => buildCells(year(), month(), todayYMD));
    const single = computed(() => groupSingleByDate(props.events));
    const spans = computed(() => placeSpans(props.events, cells()));
    return ([
        el("div", {
            "class": "cal-week",
            "aria-hidden": "true",
            get children() {
                return Repeat({
                    get "each"() {
                        return WEEKDAYS_MON_FIRST;
                    },
                    children: (d, i) => el("div", {
                        get "class"() {
                            return i() >= 5 ? "we" : "";
                        },
                        get children() {
                            return live(() => d);
                        }
                    })
                });
            }
        }),
        el("div", {
            "class": "cal-grid",
            get children() {
                return [
                    Repeat({
                        get "each"() {
                            return cells();
                        },
                        children: (c) => (el("div", {
                            get "class"() {
                                return [
                                    "cal-cell",
                                    c.inMonth ? "" : "other",
                                    c.isWeekend ? "we" : "",
                                    c.isToday ? "today" : "",
                                ].filter(Boolean).join(" ");
                            },
                            "onClick": (e) => {
                                if ((e.target as HTMLElement).closest(".evt, .evt-span"))
                                    return;
                                props.onCreate(c.ymd);
                            },
                            "style": "cursor:pointer",
                            get children() {
                                return [
                                    el("span", {
                                        "class": "day",
                                        get children() {
                                            return live(() => c.date.getDate());
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return single().get(c.ymd);
                                        },
                                        children: (evts) => (el("div", {
                                            "class": "cal-events",
                                            get children() {
                                                return [
                                                    Repeat({
                                                        get "each"() {
                                                            return evts().slice(0, 3);
                                                        },
                                                        children: (e) => (el("div", {
                                                            "class": "evt",
                                                            get "data-tone"() {
                                                                return eventTone(e.ownerMemberId);
                                                            },
                                                            get "title"() {
                                                                return e.title;
                                                            },
                                                            "onClick": (ev) => { ev.stopPropagation(); props.onEdit(rootEvent(e)); },
                                                            get children() {
                                                                return [
                                                                    el("time", {
                                                                        get children() {
                                                                            return live(() => timeLabel(e));
                                                                        }
                                                                    }),
                                                                    el("span", {
                                                                        "class": "title",
                                                                        get children() {
                                                                            return live(() => e.title);
                                                                        }
                                                                    }),
                                                                    When({
                                                                        get "when"() {
                                                                            return isInstance(e);
                                                                        },
                                                                        get children() {
                                                                            return el("span", {
                                                                                "title": "recurring instance",
                                                                                "style": "margin-left:auto;font-size:10px;opacity:0.6",
                                                                                get children() {
                                                                                    return "\uD83D\uDD01";
                                                                                }
                                                                            });
                                                                        }
                                                                    })
                                                                ];
                                                            }
                                                        }))
                                                    }),
                                                    When({
                                                        get "when"() {
                                                            return evts().length > 3;
                                                        },
                                                        get children() {
                                                            return el("div", {
                                                                "class": "evt-more",
                                                                get children() {
                                                                    return [
                                                                        "+",
                                                                        live(() => evts().length - 3),
                                                                        " more"
                                                                    ];
                                                                }
                                                            });
                                                        }
                                                    })
                                                ];
                                            }
                                        }))
                                    })
                                ];
                            }
                        }))
                    }),
                    Repeat({
                        get "each"() {
                            return spans();
                        },
                        children: (s) => (el("div", {
                            "class": "evt-span",
                            get "data-tone"() {
                                return eventTone(s.evt.ownerMemberId);
                            },
                            get "style"() {
                                return {
                                    "grid-row": s.row,
                                    "grid-column": `${s.col} / span ${s.span}`,
                                } as any;
                            },
                            get "title"() {
                                return s.evt.title;
                            },
                            "onClick": (ev) => { ev.stopPropagation(); props.onEdit(rootEvent(s.evt)); },
                            get children() {
                                return el("span", {
                                    "class": "title",
                                    get children() {
                                        return live(() => s.evt.title);
                                    }
                                });
                            }
                        }))
                    })
                ];
            }
        }),
        When({
            get "when"() {
                return props.events.length === 0;
            },
            get children() {
                return el("p", {
                    "style": "padding:18px;text-align:center;color:var(--ink-mute)",
                    get children() {
                        return "No events in this range. Click any day or \"New event\" to add one.";
                    }
                });
            }
        })
    ]);
};
// ─── Time-grid view (day / 3-day / week) ──────────────────────────────
interface DragState {
    dayIdx: number;
    startHour: number;
    endHour: number; // inclusive (mouse-up snaps end to that hour cell)
}
const TimeGridView = (props: {
    anchor: Date;
    days: number;
    events: ApiEvent[];
    onCreate: (startIso: string, endIso: string) => void;
    onEdit: (e: ApiEvent) => void;
}) => {
    // Compute the visible day list (Date objects, midnight local).
    const visibleDays = computed(() => {
        const start = startOfRange(props.anchor, props.days === 7 ? "week" : props.days === 3 ? "3day" : "day");
        return Array.from({ length: props.days }, (_, i) => {
            const d = new Date(start);
            d.setDate(start.getDate() + i);
            return d;
        });
    });
    // Bucket events by their local YYYY-MM-DD. Multi-day events appear in
    // every day they touch within the visible range.
    const eventsByDay = computed(() => {
        const map = new Map<string, ApiEvent[]>();
        for (const e of props.events) {
            if (e.allDay)
                continue;
            if (!e.startsAt)
                continue;
            const s = new Date(e.startsAt);
            const en = new Date(e.endsAt ?? e.startsAt);
            // Walk across days for spans (capped at a sane number to avoid
            // pathological multi-year events filling the bucket).
            const max = 30;
            for (let i = 0; i < max; i++) {
                const d = new Date(s);
                d.setDate(s.getDate() + i);
                if (d.getTime() > en.getTime() && i > 0)
                    break;
                const key = ymdLocal(d.toISOString());
                const arr = map.get(key) ?? [];
                arr.push(e);
                map.set(key, arr);
                if (d.toDateString() === en.toDateString())
                    break;
            }
        }
        return map;
    });
    // All-day events grouped by day for the top strip.
    const allDayByDay = computed(() => {
        const map = new Map<string, ApiEvent[]>();
        for (const e of props.events) {
            if (!e.allDay)
                continue;
            if (!e.startsAt)
                continue;
            const key = ymdLocal(e.startsAt);
            const arr = map.get(key) ?? [];
            arr.push(e);
            map.set(key, arr);
        }
        return map;
    });
    // Drag-to-create state.
    const [drag, setDrag] = cell<DragState | null>(null);
    const onMouseDown = (dayIdx: number, hour: number) => {
        setDrag({ dayIdx, startHour: hour, endHour: hour });
        // Wire up window-level move/up so a drag that leaves the cell still
        // resolves cleanly. The handlers self-remove on mouseup.
        const onMove = (ev: MouseEvent) => {
            const target = ev.target as HTMLElement | null;
            const cell = target?.closest("[data-hour]") as HTMLElement | null;
            if (!cell)
                return;
            const dayAttr = cell.getAttribute("data-day");
            const hourAttr = cell.getAttribute("data-hour");
            if (dayAttr === null || hourAttr === null)
                return;
            const dIdx = Number(dayAttr);
            const h = Number(hourAttr);
            // We only let drag extend within the original day column so the
            // resulting event is a single contiguous time range.
            setDrag((d) => (d ? { ...d, endHour: dIdx === d.dayIdx ? h : d.endHour } : d));
        };
        const onUp = () => {
            window.removeEventListener("mousemove", onMove);
            window.removeEventListener("mouseup", onUp);
            const d = drag();
            setDrag(null);
            if (!d)
                return;
            const day = visibleDays()[d.dayIdx];
            if (!day)
                return;
            const lo = Math.min(d.startHour, d.endHour);
            const hi = Math.max(d.startHour, d.endHour);
            // End at the bottom of the last selected hour, so dragging 9–10
            // gives a 9:00–11:00 event (selection spans two hour-blocks).
            const startIso = atHour(day, lo, 0).toISOString();
            const endIso = atHour(day, hi + 1, 0).toISOString();
            props.onCreate(startIso, endIso);
        };
        window.addEventListener("mousemove", onMove);
        window.addEventListener("mouseup", onUp);
    };
    const isCellInDrag = (dayIdx: number, hour: number): boolean => {
        const d = drag();
        if (!d)
            return false;
        if (d.dayIdx !== dayIdx)
            return false;
        const lo = Math.min(d.startHour, d.endHour);
        const hi = Math.max(d.startHour, d.endHour);
        return hour >= lo && hour <= hi;
    };
    const eventTopOffset = (e: ApiEvent, day: Date): number => {
        const s = new Date(e.startsAt!);
        // Clip to the visible day's window so a multi-day span starts at the
        // top of the column it didn't begin on.
        const sameDay = s.toDateString() === day.toDateString();
        if (!sameDay)
            return 0;
        const minutesFromTop = (s.getHours() - GRID_START_HOUR) * 60 + s.getMinutes();
        return Math.max(0, (minutesFromTop / 60) * HOUR_HEIGHT);
    };
    const eventHeight = (e: ApiEvent, day: Date): number => {
        const s = new Date(e.startsAt!);
        const en = new Date(e.endsAt ?? e.startsAt!);
        const dayStart = atHour(day, GRID_START_HOUR, 0);
        const dayEnd = atHour(day, GRID_END_HOUR + 1, 0);
        const visStart = s < dayStart ? dayStart : s;
        const visEnd = en > dayEnd ? dayEnd : en;
        const minutes = Math.max(30, (visEnd.getTime() - visStart.getTime()) / 60000);
        return (minutes / 60) * HOUR_HEIGHT;
    };
    return (el("div", {
        "style": "display:flex;flex-direction:column;border-top:1px solid var(--line)",
        get children() {
            return [
                el("div", {
                    get "style"() {
                        return {
                            display: "grid",
                            "grid-template-columns": `60px repeat(${props.days}, 1fr)`,
                            "border-bottom": "1px solid var(--line)",
                            background: "var(--paper)",
                        };
                    },
                    get children() {
                        return [
                            el("div", {}),
                            Repeat({
                                get "each"() {
                                    return visibleDays();
                                },
                                children: (d) => (el("div", {
                                    "style": "padding:8px 10px;text-align:center;border-left:1px solid var(--line)",
                                    get children() {
                                        return [
                                            el("div", {
                                                "style": "font-size:11px;letter-spacing:0.12em;text-transform:uppercase;color:var(--ink-mute)",
                                                get children() {
                                                    return live(() => d.toLocaleDateString(undefined, { weekday: "short" }));
                                                }
                                            }),
                                            el("div", {
                                                get "style"() {
                                                    return `font-size:18px;font-weight:600;color:${d.toDateString() === new Date().toDateString() ? "var(--ocean-2)" : "var(--ink)"}`;
                                                },
                                                get children() {
                                                    return live(() => d.getDate());
                                                }
                                            })
                                        ];
                                    }
                                }))
                            })
                        ];
                    }
                }),
                When({
                    get "when"() {
                        return Array.from(allDayByDay().values()).flat().length > 0;
                    },
                    get children() {
                        return el("div", {
                            get "style"() {
                                return {
                                    display: "grid",
                                    "grid-template-columns": `60px repeat(${props.days}, 1fr)`,
                                    "border-bottom": "1px solid var(--line)",
                                    background: "color-mix(in oklab, var(--paper) 96%, var(--ocean-1) 4%)",
                                };
                            },
                            get children() {
                                return [
                                    el("div", {
                                        "style": "padding:6px 8px;font-size:11px;color:var(--ink-mute);text-align:right",
                                        get children() {
                                            return "all-day";
                                        }
                                    }),
                                    Repeat({
                                        get "each"() {
                                            return visibleDays();
                                        },
                                        children: (d) => {
                                            const items = () => allDayByDay().get(ymdLocal(d.toISOString())) ?? [];
                                            return (el("div", {
                                                "style": "padding:4px;display:flex;flex-direction:column;gap:3px;border-left:1px solid var(--line);min-height:24px",
                                                get children() {
                                                    return Repeat({
                                                        get "each"() {
                                                            return items();
                                                        },
                                                        children: (e) => (el("div", {
                                                            "class": "evt",
                                                            get "data-tone"() {
                                                                return eventTone(e.ownerMemberId);
                                                            },
                                                            get "title"() {
                                                                return e.title;
                                                            },
                                                            "style": "padding:2px 6px;font-size:11px;cursor:pointer",
                                                            "onClick": (ev) => { ev.stopPropagation(); props.onEdit(rootEvent(e)); },
                                                            get children() {
                                                                return el("span", {
                                                                    "class": "title",
                                                                    get children() {
                                                                        return live(() => e.title);
                                                                    }
                                                                });
                                                            }
                                                        }))
                                                    });
                                                }
                                            }));
                                        }
                                    })
                                ];
                            }
                        });
                    }
                }),
                el("div", {
                    get "style"() {
                        return {
                            display: "grid",
                            "grid-template-columns": `60px repeat(${props.days}, 1fr)`,
                            position: "relative",
                        };
                    },
                    get children() {
                        return [
                            el("div", {
                                get children() {
                                    return Repeat({
                                        get "each"() {
                                            return Array.from({ length: GRID_HOURS }, (_, i) => GRID_START_HOUR + i);
                                        },
                                        children: (h) => (el("div", {
                                            get "style"() {
                                                return `height:${HOUR_HEIGHT}px;padding:2px 8px;text-align:right;font-size:11px;color:var(--ink-mute);border-top:1px dashed var(--line)`;
                                            },
                                            get children() {
                                                return live(() => formatHour(h));
                                            }
                                        }))
                                    });
                                }
                            }),
                            Repeat({
                                get "each"() {
                                    return visibleDays();
                                },
                                children: (day, dayIdx) => {
                                    const dayEvents = () => eventsByDay().get(ymdLocal(day.toISOString())) ?? [];
                                    return (el("div", {
                                        "style": "position:relative;border-left:1px solid var(--line)",
                                        get children() {
                                            return [
                                                Repeat({
                                                    get "each"() {
                                                        return Array.from({ length: GRID_HOURS }, (_, i) => GRID_START_HOUR + i);
                                                    },
                                                    children: (h) => (el("div", {
                                                        get "data-day"() {
                                                            return dayIdx();
                                                        },
                                                        get "data-hour"() {
                                                            return h;
                                                        },
                                                        "onMouseDown": (e) => { e.preventDefault(); onMouseDown(dayIdx(), h); },
                                                        get "style"() {
                                                            return `height:${HOUR_HEIGHT}px;border-top:1px dashed var(--line);cursor:cell;background:${isCellInDrag(dayIdx(), h) ? "color-mix(in oklab, var(--paper) 78%, var(--grass-1) 22%)" : "transparent"}`;
                                                        }
                                                    }))
                                                }),
                                                Repeat({
                                                    get "each"() {
                                                        return dayEvents();
                                                    },
                                                    children: (e) => (el("div", {
                                                        "class": "evt",
                                                        get "data-tone"() {
                                                            return eventTone(e.ownerMemberId);
                                                        },
                                                        get "title"() {
                                                            return e.title;
                                                        },
                                                        get "style"() {
                                                            return `position:absolute;left:4px;right:4px;top:${eventTopOffset(e, day)}px;height:${eventHeight(e, day) - 2}px;overflow:hidden;padding:4px 6px;font-size:12px;border-radius:var(--r-sm);cursor:pointer;z-index:1`;
                                                        },
                                                        "onClick": (ev) => { ev.stopPropagation(); props.onEdit(rootEvent(e)); },
                                                        get children() {
                                                            return [
                                                                el("div", {
                                                                    "style": "font-weight:600;line-height:1.1",
                                                                    get children() {
                                                                        return live(() => e.title);
                                                                    }
                                                                }),
                                                                el("div", {
                                                                    "style": "font-size:10px;opacity:0.85",
                                                                    get children() {
                                                                        return live(() => timeLabel(e));
                                                                    }
                                                                }),
                                                                When({
                                                                    get "when"() {
                                                                        return isInstance(e);
                                                                    },
                                                                    get children() {
                                                                        return el("span", {
                                                                            "title": "recurring",
                                                                            "style": "position:absolute;top:2px;right:4px;font-size:10px;opacity:0.6",
                                                                            get children() {
                                                                                return "\uD83D\uDD01";
                                                                            }
                                                                        });
                                                                    }
                                                                })
                                                            ];
                                                        }
                                                    }))
                                                })
                                            ];
                                        }
                                    }));
                                }
                            })
                        ];
                    }
                })
            ];
        }
    }));
};
// ─── Event composer (date/time + recurrence) ──────────────────────────
const EventComposer = (props: {
    houseId: string;
    calendarEnabled: boolean;
    members: Awaited<ReturnType<typeof memberClient.listMembers>>;
    state: {
        mode: "create";
        start?: string;
        end?: string;
        allDay?: boolean;
    } | {
        mode: "edit";
        event: ApiEvent;
    };
    onClose: () => void;
    onSaved: () => Promise<void> | void;
    onOpen: (event: ApiEvent) => void;
}) => {
    const editing = () => props.state.mode === "edit";
    const initial = () => (props.state.mode === "edit" ? props.state.event : null);
    const [owner, setOwner] = cell(initial()?.ownerMemberId ?? "");
    const [dirty, setDirty] = cell(false);
    const [rule, setRule] = cell(initial()?.recurrenceRule ?? "");
    const [scope, setScope] = cell(initial()?.recurrenceRootEventId ? "this" : "all");
    const frozen = () => initial()?.calendarState === "publishing" || Boolean(initial()?.editPendingMemberId) || Boolean(initial()?.calendarUnbound);
    const canRecover = () => initial()?.calendarState === "publishing" && initial()?.ownerMemberId === currentMemberId();
    const canRecoverEdit = () => initial()?.editPendingMemberId === currentMemberId();
    const canAccept = () => initial()?.ownerMemberId === currentMemberId() &&
        (initial()?.calendarState === "pending" || (props.calendarEnabled && ["legacy", "local"].includes(initial()?.calendarState ?? "")));
    // Seed defaults: edit mode reads the event row; create mode prefers the
    // start/end the caller passed (drag-select or cell-click), else tomorrow
    // 9–10am.
    const seedStart = (): string => {
        if (initial()?.startsAt)
            return initial()!.startsAt!;
        if (props.state.mode === "create" && props.state.start)
            return props.state.start;
        const d = new Date();
        d.setDate(d.getDate() + 1);
        d.setHours(9, 0, 0, 0);
        return d.toISOString();
    };
    const seedEnd = (): string => {
        if (initial()?.endsAt)
            return initial()!.endsAt!;
        if (props.state.mode === "create" && props.state.end)
            return props.state.end;
        const d = new Date();
        d.setDate(d.getDate() + 1);
        d.setHours(10, 0, 0, 0);
        return d.toISOString();
    };
    const [title, setTitle] = cell(initial()?.title ?? "");
    const [description, setDesc] = cell(initial()?.description ?? "");
    const [location, setLocation] = cell(initial()?.location ?? "");
    const [allDay, setAllDay] = cell(initial()?.allDay ?? (props.state.mode === "create" ? Boolean(props.state.allDay) : false));
    const [startsAt, setStartsAt] = cell(seedStart());
    const [endsAt, setEndsAt] = cell(seedEnd());
    // The server creates local occurrences. Catalystcal owns remote series.
    const initialRecFreq = (): RecurrenceFreq => {
        const f = initial()?.recurrenceFreq;
        return typeof f === "string" ? (f as RecurrenceFreq) : "";
    };
    const [recFreq, setRecFreq] = cell<RecurrenceFreq>(initialRecFreq());
    const [recInterval, setRecInterval] = cell(initial()?.recurrenceInterval ?? 1);
    // The event start is the recurrence anchor. The server sets the next
    // occurrence, so this form does not need a separate anchor input.
    const [recNextAt, setRecNextAt] = cell("");
    const initialByWeekday = (): number[] => {
        const v = initial()?.recurrenceByWeekday;
        return Array.isArray(v) ? v.map((n) => Number(n)) : [];
    };
    const [recByWeekday, setRecByWeekday] = cell<number[]>(initialByWeekday());
    const [recBySetpos, setRecBySetpos] = cell(initial()?.recurrenceBySetpos ?? 1);
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        if (!title().trim() || busy() || frozen())
            return;
        setBusy(true);
        setErr(null);
        try {
            const body: any = {
                houseId: props.houseId,
                ownerMemberId: owner(),
                title: title().trim(),
                description: description().trim() || undefined,
                location: location().trim() || undefined,
                startsAt: startsAt() || undefined,
                endsAt: endsAt() || undefined,
                allDay: allDay(),
                timezone: initial()?.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone,
                editScope: scope(),
            };
            // Recurrence: send the freq + interval + by_weekday/by_setpos.
            // The wire enum has no empty value. Clear recurrence explicitly.
            body.recurrenceFreq = recFreq() || undefined;
            body.clearRecurrence = initial()?.calendarState === "remote" ? !rule() : !recFreq();
            if (initial()?.calendarState === "remote") {
                body.recurrenceRule = rule();
            }
            if (recFreq()) {
                body.recurrenceInterval = recInterval();
                if (recByWeekday().length > 0) {
                    body.recurrenceByWeekday = recByWeekday();
                }
                else {
                    body.recurrenceByWeekday = [];
                }
                if (recFreq() === "monthly" || recFreq() === "quarterly" || recFreq() === "yearly") {
                    body.recurrenceBySetpos = recBySetpos();
                }
                else {
                    body.recurrenceBySetpos = 0;
                }
            }
            // Suppress unused-var lint without coupling to setRecNextAt usage.
            void recNextAt;
            if (editing()) {
                await eventClient.updateEvent({
                    ...(props.state as {
                        event: ApiEvent;
                    }).event,
                    ...body,
                } as any);
            }
            else {
                await eventClient.createEvent(normalizeEventCreate(body));
            }
            await props.onSaved();
        }
        catch (e2) {
            setErr(e2 instanceof Error ? e2.message : String(e2));
            if (initial()?.calendarState === "remote") {
                try {
                    const current = await eventClient.getEvent(initial()!.eventId);
                    if (current.editPendingMemberId)
                        props.onOpen(current);
                }
                catch { /* Keep the save error if the recovery status cannot be read. */ }
            }
        }
        finally {
            setBusy(false);
        }
    };
    const decide = async (accept: boolean) => {
        const event = initial();
        if (!event?.revision || busy() || (dirty() && !canRecover()))
            return;
        setBusy(true);
        setErr(null);
        try {
            const request = { eventId: event.eventId, revision: event.revision };
            if (accept)
                await eventClient.acceptEvent(request);
            else
                await eventClient.declineEvent(request);
            await props.onSaved();
        }
        catch (e) {
            setErr(e instanceof Error ? e.message : String(e));
        }
        finally {
            setBusy(false);
        }
    };
    const remove = async (andFuture: boolean) => {
        if (props.state.mode !== "edit" || busy() || frozen())
            return;
        const t = props.state.event.title;
        const msg = andFuture
            ? `Delete "${t}" and every later occurrence in this series?`
            : `Delete "${t}"?`;
        if (!confirm(msg))
            return;
        setBusy(true);
        setErr(null);
        try {
            if (andFuture) {
                await eventClient.deleteEventAndFuture(props.state.event.eventId);
            }
            else {
                await eventClient.deleteEvent(props.state.event.eventId);
            }
            await props.onSaved();
        }
        catch (e2) {
            setErr(e2 instanceof Error ? e2.message : String(e2));
        }
        finally {
            setBusy(false);
        }
    };
    return (el("form", {
        get "onSubmit"() {
            return submit;
        },
        "onInput": () => setDirty(true),
        "style": "margin:0 0 16px;padding:18px 20px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)",
        get children() {
            return [
                el("div", {
                    "style": "display:flex;justify-content:space-between;align-items:center;margin-bottom:12px",
                    get children() {
                        return [
                            el("h3", {
                                "style": "margin:0;font-family:var(--display);font-size:20px;color:var(--grass-4)",
                                get children() {
                                    return live(() => editing() ? "Edit event" : "New event");
                                }
                            }),
                            el("button", {
                                "type": "button",
                                "class": "btn btn-ghost",
                                get "onClick"() {
                                    return props.onClose;
                                },
                                get "disabled"() {
                                    return busy();
                                },
                                get children() {
                                    return "Cancel";
                                }
                            })
                        ];
                    }
                }),
                When({
                    get "when"() {
                        return initial()?.publicationError;
                    },
                    get children() {
                        return el("p", {
                            "role": "alert",
                            get children() {
                                return live(() => initial()?.publicationError);
                            }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return initial()?.calendarState === "publishing";
                    },
                    get children() {
                        return el("p", {
                            get children() {
                                return "Publication is not confirmed. This revision is locked. Retry publication to recover the saved request.";
                            }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return canAccept();
                    },
                    get children() {
                        return el("p", {
                            get children() {
                                return [
                                    "Review the saved details before accepting. ",
                                    live(() => props.calendarEnabled ? "Acceptance puts this event in catalystcal using your calendar connection." : "Acceptance adds this event to your Longhouse calendar.")
                                ];
                            }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return !editing() && hasRole("admin");
                    },
                    get children() {
                        return [
                            el("label", {
                                get children() {
                                    return [
                                        "Assign to ",
                                        el("select", {
                                            get "value"() {
                                                return owner();
                                            },
                                            "onChange": e => setOwner(e.currentTarget.value),
                                            get children() {
                                                return [
                                                    el("option", {
                                                        "value": "",
                                                        get children() {
                                                            return "Me";
                                                        }
                                                    }),
                                                    Repeat({
                                                        get "each"() {
                                                            return props.members.filter(m => !m.deactivatedAt && m.memberId !== currentMemberId());
                                                        },
                                                        children: member => el("option", {
                                                            get "value"() {
                                                                return member.memberId;
                                                            },
                                                            get children() {
                                                                return live(() => displayName(member));
                                                            }
                                                        })
                                                    })
                                                ];
                                            }
                                        })
                                    ];
                                }
                            }),
                            When({
                                get "when"() {
                                    return owner();
                                },
                                get children() {
                                    return el("p", {
                                        get children() {
                                            return "The assigned member must accept this event. It stays in Longhouse until then.";
                                        }
                                    });
                                }
                            })
                        ];
                    }
                }),
                el("div", {
                    "class": "lh-form-grid",
                    "style": "display:grid;grid-template-columns:1fr 1fr;gap:12px",
                    get children() {
                        return [
                            el("label", {
                                "style": "display:flex;flex-direction:column;gap:4px;grid-column:1/-1",
                                get children() {
                                    return [
                                        el("span", {
                                            "style": "font-size:12px;color:var(--ink-mute)",
                                            get children() {
                                                return "Title";
                                            }
                                        }),
                                        el("input", {
                                            "type": "text",
                                            get "value"() {
                                                return title();
                                            },
                                            "onInput": (e) => setTitle(e.currentTarget.value),
                                            "required": true,
                                            "autofocus": true,
                                            "style": "padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
                                        })
                                    ];
                                }
                            }),
                            el("label", {
                                "style": "display:flex;flex-direction:column;gap:4px;grid-column:1/-1",
                                get children() {
                                    return [
                                        el("span", {
                                            "style": "font-size:12px;color:var(--ink-mute)",
                                            get children() {
                                                return "Description (optional)";
                                            }
                                        }),
                                        el("textarea", {
                                            get "value"() {
                                                return description();
                                            },
                                            "onInput": (e) => setDesc(e.currentTarget.value),
                                            "rows": "2",
                                            "style": "padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;resize:vertical"
                                        })
                                    ];
                                }
                            }),
                            el("label", {
                                "style": "display:flex;flex-direction:column;gap:4px;grid-column:1/-1",
                                get children() {
                                    return [
                                        el("span", {
                                            "style": "font-size:12px;color:var(--ink-mute)",
                                            get children() {
                                                return "Location (optional)";
                                            }
                                        }),
                                        el("input", {
                                            "type": "text",
                                            get "value"() {
                                                return location();
                                            },
                                            "onInput": (e) => setLocation(e.currentTarget.value),
                                            "style": "padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
                                        })
                                    ];
                                }
                            }),
                            el("label", {
                                "style": "display:flex;flex-direction:column;gap:4px",
                                get children() {
                                    return [
                                        el("span", {
                                            "style": "font-size:12px;color:var(--ink-mute)",
                                            get children() {
                                                return "Starts";
                                            }
                                        }),
                                        DateTimePicker({
                                            get "value"() {
                                                return startsAt();
                                            },
                                            get "onChange"() {
                                                return setStartsAt;
                                            },
                                            "title": "Start"
                                        })
                                    ];
                                }
                            }),
                            el("label", {
                                "style": "display:flex;flex-direction:column;gap:4px",
                                get children() {
                                    return [
                                        el("span", {
                                            "style": "font-size:12px;color:var(--ink-mute)",
                                            get children() {
                                                return "Ends";
                                            }
                                        }),
                                        DateTimePicker({
                                            get "value"() {
                                                return endsAt();
                                            },
                                            get "onChange"() {
                                                return setEndsAt;
                                            },
                                            "title": "End"
                                        })
                                    ];
                                }
                            }),
                            el("label", {
                                "style": "display:flex;gap:8px;align-items:center;grid-column:1/-1",
                                get children() {
                                    return [
                                        el("input", {
                                            "type": "checkbox",
                                            get "checked"() {
                                                return allDay();
                                            },
                                            "onChange": (e) => {
                                                setAllDay(e.currentTarget.checked);
                                                if (e.currentTarget.checked) {
                                                    const start = new Date(startsAt());
                                                    start.setHours(0, 0, 0, 0);
                                                    const end = new Date(start);
                                                    end.setDate(end.getDate() + 1);
                                                    setStartsAt(start.toISOString());
                                                    setEndsAt(end.toISOString());
                                                }
                                            }
                                        }),
                                        el("span", {
                                            "style": "font-size:14px",
                                            get children() {
                                                return "All day";
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("div", {
                                "style": "grid-column:1/-1",
                                get children() {
                                    return [
                                        When({
                                            get "when"() {
                                                return initial()?.calendarState === "remote";
                                            },
                                            get "fallback"() {
                                                return RecurrenceFields({
                                                    get "freq"() {
                                                        return recFreq;
                                                    },
                                                    get "setFreq"() {
                                                        return setRecFreq;
                                                    },
                                                    get "interval"() {
                                                        return recInterval;
                                                    },
                                                    get "setInterval"() {
                                                        return setRecInterval;
                                                    },
                                                    get "nextAt"() {
                                                        return recNextAt;
                                                    },
                                                    get "setNextAt"() {
                                                        return setRecNextAt;
                                                    },
                                                    get "byWeekday"() {
                                                        return recByWeekday;
                                                    },
                                                    get "setByWeekday"() {
                                                        return setRecByWeekday;
                                                    },
                                                    get "bySetpos"() {
                                                        return recBySetpos;
                                                    },
                                                    get "setBySetpos"() {
                                                        return setRecBySetpos;
                                                    }
                                                });
                                            },
                                            get children() {
                                                return el("label", {
                                                    get children() {
                                                        return [
                                                            "Recurrence rule ",
                                                            el("input", {
                                                                get "value"() {
                                                                    return rule();
                                                                },
                                                                "onInput": e => setRule(e.currentTarget.value),
                                                                "placeholder": "No recurrence"
                                                            })
                                                        ];
                                                    }
                                                });
                                            }
                                        }),
                                        When({
                                            get "when"() {
                                                return initial()?.calendarState === "remote" && initial()?.recurrenceRootEventId;
                                            },
                                            get children() {
                                                return [
                                                    el("label", {
                                                        get children() {
                                                            return [
                                                                "Apply changes to ",
                                                                el("select", {
                                                                    get "value"() {
                                                                        return scope();
                                                                    },
                                                                    get "disabled"() {
                                                                        return busy() || frozen();
                                                                    },
                                                                    "onChange": e => setScope(e.currentTarget.value),
                                                                    get children() {
                                                                        return [
                                                                            el("option", {
                                                                                "value": "this",
                                                                                get children() {
                                                                                    return "This occurrence";
                                                                                }
                                                                            }),
                                                                            el("option", {
                                                                                "value": "this_and_future",
                                                                                get children() {
                                                                                    return "This and future occurrences";
                                                                                }
                                                                            })
                                                                        ];
                                                                    }
                                                                })
                                                            ];
                                                        }
                                                    }),
                                                    el("button", {
                                                        "type": "button",
                                                        "class": "btn btn-ghost",
                                                        get "disabled"() {
                                                            return busy();
                                                        },
                                                        "onClick": async () => {
                                                            setBusy(true);
                                                            setErr(null);
                                                            try {
                                                                props.onOpen(await eventClient.getEvent(initial()!.recurrenceRootEventId!));
                                                            }
                                                            catch (e) {
                                                                setErr(String(e));
                                                            }
                                                            finally {
                                                                setBusy(false);
                                                            }
                                                        },
                                                        get children() {
                                                            return "Open whole series";
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
                }),
                When({
                    get "when"() {
                        return err();
                    },
                    children: (m) => el("p", {
                        "style": "color:var(--rust);font-size:13px;margin:10px 0 0",
                        get children() {
                            return live(() => m());
                        }
                    })
                }),
                el("div", {
                    "style": "display:flex;justify-content:space-between;gap:10px;margin-top:14px;flex-wrap:wrap",
                    get children() {
                        return [
                            When({
                                get "when"() {
                                    return editing();
                                },
                                get children() {
                                    return el("div", {
                                        "style": "display:flex;gap:8px;flex-wrap:wrap",
                                        get children() {
                                            return [
                                                el("button", {
                                                    "type": "button",
                                                    "class": "btn btn-ghost",
                                                    "onClick": () => remove(false),
                                                    get "disabled"() {
                                                        return busy() || frozen();
                                                    },
                                                    "style": "color:var(--rust)",
                                                    get children() {
                                                        return "Delete";
                                                    }
                                                }),
                                                When({
                                                    get "when"() {
                                                        return isRecurring(initial());
                                                    },
                                                    get children() {
                                                        return el("button", {
                                                            "type": "button",
                                                            "class": "btn btn-ghost",
                                                            "onClick": () => remove(true),
                                                            get "disabled"() {
                                                                return busy() || frozen();
                                                            },
                                                            "style": "color:var(--rust);border:1px dashed var(--rust)",
                                                            "title": "Drop this occurrence and every later one. Clears the series's recurrence so the spawner stops respawning.",
                                                            get children() {
                                                                return "Delete this & future";
                                                            }
                                                        });
                                                    }
                                                })
                                            ];
                                        }
                                    });
                                }
                            }),
                            el("div", {
                                "style": "display:flex;gap:10px;margin-left:auto",
                                get children() {
                                    return [
                                        When({
                                            get "when"() {
                                                return canRecoverEdit();
                                            },
                                            get children() {
                                                return el("button", {
                                                    "type": "button",
                                                    "class": "btn btn-primary",
                                                    get "disabled"() {
                                                        return busy();
                                                    },
                                                    "onClick": async () => {
                                                        setBusy(true);
                                                        setErr(null);
                                                        try {
                                                            await eventClient.recoverEdit(initial()!.eventId);
                                                            await props.onSaved();
                                                        }
                                                        catch (e) {
                                                            setErr(e instanceof Error ? e.message : String(e));
                                                        }
                                                        finally {
                                                            setBusy(false);
                                                        }
                                                    },
                                                    get children() {
                                                        return "Retry saved edit";
                                                    }
                                                });
                                            }
                                        }),
                                        When({
                                            get "when"() {
                                                return canRecover();
                                            },
                                            get children() {
                                                return el("button", {
                                                    "type": "button",
                                                    "class": "btn btn-primary",
                                                    get "disabled"() {
                                                        return busy();
                                                    },
                                                    "onClick": () => decide(true),
                                                    get children() {
                                                        return "Retry publication";
                                                    }
                                                });
                                            }
                                        }),
                                        When({
                                            get "when"() {
                                                return canAccept();
                                            },
                                            get children() {
                                                return [
                                                    el("button", {
                                                        "type": "button",
                                                        "class": "btn btn-ghost",
                                                        get "disabled"() {
                                                            return busy() || dirty();
                                                        },
                                                        "onClick": () => decide(false),
                                                        get children() {
                                                            return "Decline";
                                                        }
                                                    }),
                                                    el("button", {
                                                        "type": "button",
                                                        "class": "btn btn-primary",
                                                        get "disabled"() {
                                                            return busy() || dirty();
                                                        },
                                                        "onClick": () => decide(true),
                                                        get children() {
                                                            return "Accept event";
                                                        }
                                                    }),
                                                    When({
                                                        get "when"() {
                                                            return dirty();
                                                        },
                                                        get children() {
                                                            return el("span", {
                                                                get children() {
                                                                    return "Save changes before accepting.";
                                                                }
                                                            });
                                                        }
                                                    })
                                                ];
                                            }
                                        }),
                                        el("button", {
                                            "class": "btn btn-primary",
                                            get "disabled"() {
                                                return busy() || frozen() || !title().trim();
                                            },
                                            "type": "submit",
                                            get children() {
                                                return live(() => busy() ? "Saving…" : editing() ? "Save changes" : "Create event");
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
// ─── helpers ──────────────────────────────────────────────────────────
function viewTitle(view: ViewMode, anchor: Date): string {
    if (view === "month")
        return `${MONTH_NAMES[anchor.getMonth()]} ${anchor.getFullYear()}`;
    if (view === "day") {
        return anchor.toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric", year: "numeric" });
    }
    const start = startOfRange(anchor, view);
    const end = new Date(start);
    end.setDate(start.getDate() + (view === "week" ? 6 : 2));
    const sameMonth = start.getMonth() === end.getMonth();
    if (sameMonth) {
        return `${start.toLocaleDateString(undefined, { month: "long", day: "numeric" })}–${end.getDate()}, ${start.getFullYear()}`;
    }
    return `${start.toLocaleDateString(undefined, { month: "short", day: "numeric" })} – ${end.toLocaleDateString(undefined, { month: "short", day: "numeric" })}, ${start.getFullYear()}`;
}
function shiftAnchor(anchor: Date, view: ViewMode, dir: -1 | 1): Date {
    const d = new Date(anchor);
    if (view === "month")
        d.setMonth(d.getMonth() + dir);
    else if (view === "day")
        d.setDate(d.getDate() + dir);
    else if (view === "3day")
        d.setDate(d.getDate() + dir * 3);
    else if (view === "week")
        d.setDate(d.getDate() + dir * 7);
    return d;
}
function atHour(day: Date, h: number, m: number): Date {
    const d = new Date(day);
    d.setHours(h, m, 0, 0);
    return d;
}
function dateAt(ymd: string, h: number, m: number): string {
    const [y, mo, d] = ymd.split("-").map(Number);
    return new Date(y, mo - 1, d, h, m, 0, 0).toISOString();
}
function formatHour(h: number): string {
    if (h === 0)
        return "12 am";
    if (h === 12)
        return "noon";
    if (h < 12)
        return `${h} am`;
    return `${h - 12} pm`;
}
// ─── Recurring-event expansion ────────────────────────────────────────
//
// The api stores recurring events as a single root row with `recurrence_freq`
// + `recurrence_interval`. We expand that into per-occurrence pseudo-events
// at render time, only within the visible range. Each instance carries the
// same metadata as the root but with shifted starts_at/ends_at; an internal
// flag distinguishes them so the edit click routes back to the root row.
// Either a recurring root (has recurrence_freq) or a spawned child (has
// recurrence_root_event_id) is considered "recurring" for UI purposes —
// both should expose the delete-and-future option.
function isRecurring(e: ApiEvent | null): boolean {
    if (!e)
        return false;
    const rootSet = typeof e.recurrenceRootEventId === "string" && e.recurrenceRootEventId !== "";
    // recurrenceFreq is an enum union (no "" member); set ⇔ truthy.
    const freqSet = !!e.recurrenceFreq;
    return rootSet || freqSet;
}
// A row is a recurring-series instance when the server has set
// recurrence_root_event_id (the worker tags each spawned child this way).
// Editing one needs to walk back to the root so changes apply to the
// whole series instead of silently diverging.
function isInstance(e: ApiEvent): boolean {
    return typeof e.recurrenceRootEventId === "string" && e.recurrenceRootEventId !== "";
}
// rootEvent returns the row to edit when the user clicks any occurrence:
// the root for an instance (so series edits land in one place), or the
// row itself otherwise. The actual root fetch happens server-side via
// GetEvent; we just return the id for the edit composer to resolve.
function rootEvent(e: ApiEvent): ApiEvent {
    return e; // editor uses event_id directly; the API does the right thing.
}
// Re-export so recurrenceLabel stays usable in the time-grid event chips
// without an extra import bookkeeping line.
export { recurrenceLabel };
