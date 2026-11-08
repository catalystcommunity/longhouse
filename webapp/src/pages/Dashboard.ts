import { observe, computed, query, cell } from "~/ui/state";
import { Repeat, When, live, el } from "~/ui/dom";
import { Link, navigation } from "~/ui/router";
import { HeroScene } from "~/components/HeroScene";
import { Check, Pin, Plus } from "~/components/Icons";
import { AuthGate } from "~/components/AuthGate";
import { AvatarStack } from "~/components/Avatar";
import { DayStrip } from "~/components/DayStrip";
import { TaskDetailEditor } from "~/components/TaskDetailEditor";
import { TaskFeedList } from "~/components/TaskFeedList";
import { eventClient, memberClient, projectClient, taskClient } from "~/data/clients";
import { dashboardKicker, displayName, dueLabel, eventTone, initial, isTaskClosed, memberHandle, memberStatus, memberSwatch, partOfDayGreeting, taskGroup, timeLabel, toAvatar, } from "~/lib/derive";
import { PROJECT_LIST_LIMIT, loadMyProjectIds, loadProjectTasks, openTaskCount } from "~/lib/projects";
import { createTaskFeed, dueBoundaries, dueGroupStarts } from "~/lib/taskFeed";
import { loadEvents, startOfRange } from "~/lib/events";
import { currentMemberId, useCurrentHouseId, useSession } from "~/stores/auth";
import type { Member, Project, Task, TaskListRequest } from "@longhouse/client";
/** Rows shown in the "Your projects" card before the "All projects" link. */
const MY_PROJECTS_CAP = 6;
// Group labels in due order: the order of the list and of dueBoundaries().
const DASHBOARD_GROUP_LABELS = ["Overdue", "Today", "This week", "Later", "No due date"];
export const Dashboard = () => {
    const houseId = useCurrentHouseId();
    const session = useSession();
    const navigate = navigation();
    // Open the calendar's day view at a given date, optionally jumping the
    // user straight into the event editor. The Calendar page reads these
    // query params on mount.
    const openInCalendar = (dateLocal: Date, eventId?: string) => {
        const y = dateLocal.getFullYear();
        const m = String(dateLocal.getMonth() + 1).padStart(2, "0");
        const d = String(dateLocal.getDate()).padStart(2, "0");
        const qs = new URLSearchParams({ view: "day", date: `${y}-${m}-${d}` });
        if (eventId)
            qs.set("event", eventId);
        navigate(`/calendar?${qs.toString()}`);
    };
    const list = (r: TaskListRequest) => taskClient.listTasks(r);
    // The Tasks card: my open tasks that are in no project, paged by the
    // server in due-date order. Project work is summarized per project in the
    // "Your projects" card instead.
    const miscFeed = createTaskFeed();
    observe(() => {
        const h = houseId();
        if (!h) {
            miscFeed.reset(null);
            return;
        }
        const bounds = dueBoundaries();
        miscFeed.reset((offset, limit, withCounts) => list({ houseId: h, state: "open", mine: true, loose: true, offset, limit, ...(withCounts ? { dueBefore: bounds } : {}) }));
    });
    // Recently completed: the newest five, and the done count from the total.
    const [recent, { refetch: refetchRecent }] = query(() => houseId(), (h) => list({ houseId: h, state: "closed", mine: true, loose: true, limit: 5 }));
    // Hero "due today" counts ALL my open tasks due today, project work
    // included — the hero is the whole-day summary, not the Tasks card.
    const [dueToday, { refetch: refetchDueToday }] = query(() => houseId(), (h) => {
        const from = new Date();
        from.setHours(0, 0, 0, 0);
        const to = new Date(from);
        to.setDate(to.getDate() + 1);
        return list({ houseId: h, state: "open", mine: true, dueFrom: from.toISOString(), dueTo: to.toISOString(), limit: 1 });
    });
    const allDueToday = () => (dueToday.state === "ready" ? Number(dueToday().total ?? dueToday().tasks.length) : 0);
    const tasksRecent = () => (recent.state === "ready" ? recent().tasks : []);
    const doneCount = () => (recent.state === "ready" ? Number(recent().total ?? recent().tasks.length) : 0);
    const [members] = query(() => houseId(), async (h) => memberClient.listMembers({ houseId: h }));
    const [events] = query(() => houseId(), (h) => {
        const start = startOfRange(new Date(), "day");
        const end = new Date(start);
        end.setFullYear(end.getFullYear() + 2);
        return loadEvents(eventClient, { houseId: h, rangeStart: start.toISOString(), rangeEnd: end.toISOString() });
    });
    const [projects, { refetch: refetchProjects }] = query(() => houseId(), async (h) => (await projectClient.listProjects({ houseId: h, limit: PROJECT_LIST_LIMIT })).projects);
    // Which projects are mine (owner via created_by, explicit owner, or
    // member), and the task list of each, for the open counts. Tolerant of
    // per-project failures (see lib/projects.ts).
    const [projectIndex] = query(() => {
        const h = houseId();
        const me = currentMemberId();
        const ps = projects();
        if (!h || !me || !ps)
            return null;
        return { h, me, projects: ps };
    }, async ({ h, me, projects: ps }) => {
        const mine = await loadMyProjectIds(projectClient, ps, me);
        const tasksByProject = await loadProjectTasks(projectClient, h, ps.filter((p) => mine.has(p.projectId)));
        return { mine, tasksByProject };
    });
    // The index as last loaded. Uses `latest` so a refetch keeps the card
    // populated instead of flashing, and never throws on an errored load.
    const index = () => projectIndex.state === "ready" || projectIndex.state === "refreshing"
        ? projectIndex.latest
        : undefined;
    // An errored load (or no project list to fan out over) counts as done.
    const indexSettled = () => !!index() ||
        projectIndex.state === "errored" ||
        projects.state === "errored" ||
        (projects.state === "ready" && !currentMemberId());
    // Projects I own or belong to, in list order, with their open task count.
    const myProjects = computed((): {
        project: Project;
        open: number;
    }[] => {
        const idx = index();
        if (!idx)
            return [];
        return (projects() ?? [])
            .filter((p) => idx.mine.has(p.projectId))
            .map((p) => ({
            project: p,
            open: openTaskCount(idx.tasksByProject.get(p.projectId)?.tasks ?? []),
        }));
    });
    // A task change can move it between the cards (into a project, closed,
    // due today), so a row's mutate hook refreshes every source. A new
    // project list (a Personal project made on first use) re-runs the
    // project index through its source.
    const refresh = async () => {
        await Promise.all([miscFeed.refresh(), refetchRecent(), refetchDueToday(), refetchProjects()]);
    };
    const memberById = computed(() => {
        const map = new Map<string, Member>();
        for (const m of members() ?? [])
            map.set(m.memberId, m);
        return map;
    });
    const miscGroups = computed(() => dueGroupStarts(miscFeed.dueCounts(), DASHBOARD_GROUP_LABELS));
    const miscFirstLoad = () => !miscFeed.ready();
    const activeMembers = computed(() => (members() ?? []).filter((m) => memberStatus(m) === "active").length);
    // Upcoming events only — anything whose end (or start, if no end) is still
    // in the future. Past events never surface in dashboard lists/counts; they
    // remain findable on the calendar.
    const upcomingEvents = computed(() => {
        const now = Date.now();
        return (events() ?? []).filter((e) => {
            const ref = e.endsAt ?? e.startsAt;
            return ref ? Date.parse(ref) >= now : false;
        });
    });
    // Next event = the earliest upcoming event. Falls back to undefined when
    // there are no future events.
    const nextEvent = computed(() => {
        const futures = [...upcomingEvents()];
        futures.sort((a, b) => {
            const ka = Date.parse(a.startsAt ?? a.endsAt ?? "");
            const kb = Date.parse(b.startsAt ?? b.endsAt ?? "");
            return ka - kb;
        });
        return futures[0];
    });
    // The estimate needs every open row, so it shows once the list is loaded.
    const footHours = computed(() => {
        if (!miscFeed.allLoaded())
            return 0;
        return miscFeed.loadedRows().reduce((acc, t) => acc + (t.estimateMinutes ?? 0), 0) / 60;
    });
    const [composerOpen, setComposerOpen] = cell(false);
    const greetingName = () => session()?.displayName?.split(/\s+/)[0] ?? "there";
    return (AuthGate({
        get children() {
            return [
                el("section", {
                    "class": "hero reveal d1",
                    get children() {
                        return [
                            HeroScene(),
                            el("div", {
                                "class": "hero-content",
                                get children() {
                                    return [
                                        el("span", {
                                            "class": "kicker",
                                            get children() {
                                                return live(() => dashboardKicker());
                                            }
                                        }),
                                        el("h1", {
                                            "class": "greet",
                                            get children() {
                                                return [
                                                    live(() => partOfDayGreeting()),
                                                    ",",
                                                    el("br", {}),
                                                    live(() => greetingName()),
                                                    "."
                                                ];
                                            }
                                        }),
                                        el("p", {
                                            "class": "greet-sub",
                                            get children() {
                                                return [
                                                    "You have ",
                                                    el("b", {
                                                        get children() {
                                                            return [
                                                                live(() => allDueToday()),
                                                                " ",
                                                                live(() => allDueToday() === 1 ? "task" : "tasks"),
                                                                " due today"
                                                            ];
                                                        }
                                                    }),
                                                    live(() => " "),
                                                    "and ",
                                                    live(() => nextEventCopy(upcomingEvents().length)),
                                                    ".",
                                                    live(() => " "),
                                                    live(() => activeMembers()),
                                                    " ",
                                                    live(() => activeMembers() === 1 ? "member is" : "members are"),
                                                    " active right now."
                                                ];
                                            }
                                        }),
                                        el("div", {
                                            "class": "hero-stats",
                                            get children() {
                                                return [
                                                    Stat({
                                                        get "n"() {
                                                            return allDueToday();
                                                        },
                                                        "label": "tasks due",
                                                        "onClick": () => navigate("/tasks")
                                                    }),
                                                    Stat({
                                                        get "n"() {
                                                            return upcomingEvents().length;
                                                        },
                                                        "label": "upcoming events",
                                                        "onClick": () => navigate("/calendar")
                                                    }),
                                                    Stat({
                                                        get "n"() {
                                                            return activeMembers();
                                                        },
                                                        "label": "active members",
                                                        "onClick": () => navigate("/members")
                                                    }),
                                                    Stat({
                                                        get "n"() {
                                                            return (projects() ?? []).length;
                                                        },
                                                        "label": "projects",
                                                        "onClick": () => navigate("/projects")
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
                el("div", {
                    "class": "grid",
                    get children() {
                        return [
                            el("section", {
                                "class": "card reveal d2",
                                "data-area": "tasks",
                                "aria-label": "Tasks",
                                get children() {
                                    return [
                                        el("div", {
                                            "class": "card-hd",
                                            get children() {
                                                return el("div", {
                                                    get children() {
                                                        return [
                                                            el("h2", {
                                                                get children() {
                                                                    return "Tasks";
                                                                }
                                                            }),
                                                            el("div", {
                                                                "class": "sub",
                                                                get children() {
                                                                    return "Yours, not in a project.";
                                                                }
                                                            })
                                                        ];
                                                    }
                                                });
                                            }
                                        }),
                                        When({
                                            get "when"() {
                                                return !miscFirstLoad();
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
                                                return el("div", {
                                                    "class": "tasks",
                                                    get children() {
                                                        return [
                                                            When({
                                                                get "when"() {
                                                                    return miscFeed.estimatedTotal() > 0;
                                                                },
                                                                get "fallback"() {
                                                                    return EmptyTasks();
                                                                },
                                                                get children() {
                                                                    return TaskFeedList({
                                                                        get "feed"() {
                                                                            return miscFeed;
                                                                        },
                                                                        get "groups"() {
                                                                            return miscGroups();
                                                                        },
                                                                        "renderTask": (t) => TaskRow({
                                                                            get "task"() {
                                                                                return t;
                                                                            },
                                                                            get "byId"() {
                                                                                return memberById();
                                                                            },
                                                                            get "members"() {
                                                                                return members() ?? [];
                                                                            },
                                                                            get "onMutate"() {
                                                                                return refresh;
                                                                            }
                                                                        })
                                                                    });
                                                                }
                                                            }),
                                                            When({
                                                                get "when"() {
                                                                    return tasksRecent().length > 0;
                                                                },
                                                                get children() {
                                                                    return [
                                                                        el("div", {
                                                                            "class": "group-lbl",
                                                                            "style": "margin-top:18px;color:var(--ink-mute)",
                                                                            get children() {
                                                                                return "Recently completed";
                                                                            }
                                                                        }),
                                                                        Repeat({
                                                                            get "each"() {
                                                                                return tasksRecent();
                                                                            },
                                                                            children: (t) => TaskRow({
                                                                                get "task"() {
                                                                                    return t;
                                                                                },
                                                                                get "byId"() {
                                                                                    return memberById();
                                                                                },
                                                                                get "members"() {
                                                                                    return members() ?? [];
                                                                                },
                                                                                get "onMutate"() {
                                                                                    return refresh;
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
                                        }),
                                        When({
                                            get "when"() {
                                                return composerOpen();
                                            },
                                            get children() {
                                                return PersonalTaskComposer({
                                                    get "houseId"() {
                                                        return houseId()!;
                                                    },
                                                    "onCancel": () => setComposerOpen(false),
                                                    "onCreated": async () => {
                                                        setComposerOpen(false);
                                                        await refresh();
                                                    }
                                                });
                                            }
                                        }),
                                        el("div", {
                                            "class": "tasks-foot",
                                            get children() {
                                                return [
                                                    el("span", {
                                                        get children() {
                                                            return [
                                                                live(() => doneCount()),
                                                                " done \u00B7 ",
                                                                live(() => miscFeed.estimatedTotal()),
                                                                " remaining",
                                                                When({
                                                                    get "when"() {
                                                                        return footHours() > 0;
                                                                    },
                                                                    get children() {
                                                                        return [
                                                                            live(() => " "),
                                                                            "\u2014 estimated",
                                                                            live(() => " "),
                                                                            el("b", {
                                                                                "style": "color:var(--grass-4);font-weight:600",
                                                                                get children() {
                                                                                    return [
                                                                                        live(() => footHours().toFixed(1)),
                                                                                        " hours"
                                                                                    ];
                                                                                }
                                                                            })
                                                                        ];
                                                                    }
                                                                })
                                                            ];
                                                        }
                                                    }),
                                                    el("button", {
                                                        "class": "btn-quiet",
                                                        "type": "button",
                                                        "onClick": () => setComposerOpen((v) => !v),
                                                        get children() {
                                                            return [
                                                                Plus(),
                                                                " New task"
                                                            ];
                                                        }
                                                    })
                                                ];
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("aside", {
                                "class": "side",
                                get children() {
                                    return [
                                        el("section", {
                                            "class": "card myprojects reveal d3",
                                            "aria-label": "Your projects",
                                            get children() {
                                                return [
                                                    el("div", {
                                                        "class": "card-hd",
                                                        "style": "padding:0 0 12px;border:0",
                                                        get children() {
                                                            return el("div", {
                                                                get children() {
                                                                    return [
                                                                        el("h2", {
                                                                            get children() {
                                                                                return "Your projects";
                                                                            }
                                                                        }),
                                                                        el("div", {
                                                                            "class": "sub",
                                                                            get children() {
                                                                                return "Projects you own or belong to.";
                                                                            }
                                                                        })
                                                                    ];
                                                                }
                                                            });
                                                        }
                                                    }),
                                                    When({
                                                        get "when"() {
                                                            return indexSettled();
                                                        },
                                                        get "fallback"() {
                                                            return el("p", {
                                                                "class": "proj-empty",
                                                                get children() {
                                                                    return "Loading\u2026";
                                                                }
                                                            });
                                                        },
                                                        get children() {
                                                            return When({
                                                                get "when"() {
                                                                    return myProjects().length > 0;
                                                                },
                                                                get "fallback"() {
                                                                    return el("p", {
                                                                        "class": "proj-empty",
                                                                        get children() {
                                                                            return [
                                                                                "You are not in a project yet. ",
                                                                                Link({
                                                                                    "href": "/projects",
                                                                                    get children() {
                                                                                        return "See all projects";
                                                                                    }
                                                                                })
                                                                            ];
                                                                        }
                                                                    });
                                                                },
                                                                get children() {
                                                                    return [
                                                                        el("ul", {
                                                                            "class": "proj-list",
                                                                            get children() {
                                                                                return Repeat({
                                                                                    get "each"() {
                                                                                        return myProjects().slice(0, MY_PROJECTS_CAP);
                                                                                    },
                                                                                    children: (row) => (el("li", {
                                                                                        get children() {
                                                                                            return Link({
                                                                                                get "href"() {
                                                                                                    return `/projects/${row.project.projectId}`;
                                                                                                },
                                                                                                "class": "proj-row",
                                                                                                get children() {
                                                                                                    return [
                                                                                                        el("span", {
                                                                                                            "class": "proj-name",
                                                                                                            get children() {
                                                                                                                return [
                                                                                                                    live(() => row.project.name),
                                                                                                                    When({
                                                                                                                        get "when"() {
                                                                                                                            return row.project.personal;
                                                                                                                        },
                                                                                                                        get children() {
                                                                                                                            return el("span", {
                                                                                                                                "style": "margin-left:6px;font-size:11px;color:var(--ink-mute)",
                                                                                                                                get children() {
                                                                                                                                    return "private";
                                                                                                                                }
                                                                                                                            });
                                                                                                                        }
                                                                                                                    })
                                                                                                                ];
                                                                                                            }
                                                                                                        }),
                                                                                                        el("span", {
                                                                                                            "class": "proj-count",
                                                                                                            get children() {
                                                                                                                return [
                                                                                                                    live(() => row.open),
                                                                                                                    " open ",
                                                                                                                    live(() => row.open === 1 ? "task" : "tasks")
                                                                                                                ];
                                                                                                            }
                                                                                                        })
                                                                                                    ];
                                                                                                }
                                                                                            });
                                                                                        }
                                                                                    }))
                                                                                });
                                                                            }
                                                                        }),
                                                                        el("div", {
                                                                            "class": "proj-foot",
                                                                            get children() {
                                                                                return Link({
                                                                                    "href": "/projects",
                                                                                    get children() {
                                                                                        return [
                                                                                            "All projects",
                                                                                            When({
                                                                                                get "when"() {
                                                                                                    return myProjects().length > MY_PROJECTS_CAP;
                                                                                                },
                                                                                                get children() {
                                                                                                    return live(() => ` (${myProjects().length - MY_PROJECTS_CAP} more of yours)`);
                                                                                                }
                                                                                            })
                                                                                        ];
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
                                        }),
                                        el("section", {
                                            "class": "card gather reveal d4",
                                            get children() {
                                                return [
                                                    el("div", {
                                                        "class": "card-hd",
                                                        "style": "padding:0 0 12px;border:0",
                                                        get children() {
                                                            return [
                                                                el("div", {
                                                                    get children() {
                                                                        return [
                                                                            el("h2", {
                                                                                get children() {
                                                                                    return "Next event";
                                                                                }
                                                                            }),
                                                                            el("div", {
                                                                                "class": "sub",
                                                                                get children() {
                                                                                    return When({
                                                                                        get "when"() {
                                                                                            return nextEvent();
                                                                                        },
                                                                                        "fallback": "\u2014",
                                                                                        children: (e) => [
                                                                                            live(() => relativeWhen(e().startsAt)),
                                                                                            When({
                                                                                                get "when"() {
                                                                                                    return e().location;
                                                                                                },
                                                                                                children: (loc) => [
                                                                                                    " \u00B7 ",
                                                                                                    live(() => loc())
                                                                                                ]
                                                                                            })
                                                                                        ]
                                                                                    });
                                                                                }
                                                                            })
                                                                        ];
                                                                    }
                                                                }),
                                                                When({
                                                                    get "when"() {
                                                                        return nextEvent();
                                                                    },
                                                                    children: (e) => (el("span", {
                                                                        "class": "kicker plain",
                                                                        get "data-tone"() {
                                                                            return eventTone(e().ownerMemberId);
                                                                        },
                                                                        get children() {
                                                                            return live(() => timeLabel(e()));
                                                                        }
                                                                    }))
                                                                })
                                                            ];
                                                        }
                                                    }),
                                                    When({
                                                        get "when"() {
                                                            return nextEvent();
                                                        },
                                                        get "fallback"() {
                                                            return el("p", {
                                                                "style": "color:var(--ink-mute)",
                                                                get children() {
                                                                    return "No upcoming events.";
                                                                }
                                                            });
                                                        },
                                                        children: (e) => {
                                                            const day = () => new Date(e().startsAt!);
                                                            return (el("div", {
                                                                "role": "button",
                                                                "tabIndex": 0,
                                                                "onClick": () => openInCalendar(day(), e().eventId),
                                                                "onKeyDown": (ev) => {
                                                                    if (ev.key === "Enter" || ev.key === " ") {
                                                                        ev.preventDefault();
                                                                        openInCalendar(day(), e().eventId);
                                                                    }
                                                                },
                                                                "style": "cursor:pointer",
                                                                get "aria-label"() {
                                                                    return `Open ${e().title} in the calendar`;
                                                                },
                                                                get children() {
                                                                    return [
                                                                        DayStrip({
                                                                            get "date"() {
                                                                                return day();
                                                                            },
                                                                            get "events"() {
                                                                                return events() ?? [];
                                                                            },
                                                                            "onPick": (ev) => openInCalendar(new Date(ev.startsAt!), ev.eventId)
                                                                        }),
                                                                        el("div", {
                                                                            "style": "margin-top:8px;font-size:13px;color:var(--ink-mute)",
                                                                            get children() {
                                                                                return [
                                                                                    el("b", {
                                                                                        "style": "color:var(--ink)",
                                                                                        get children() {
                                                                                            return live(() => e().title);
                                                                                        }
                                                                                    }),
                                                                                    When({
                                                                                        get "when"() {
                                                                                            return e().location;
                                                                                        },
                                                                                        children: (loc) => [
                                                                                            " \u00B7 ",
                                                                                            el("span", {
                                                                                                "style": "display:inline-flex;align-items:center;gap:2px",
                                                                                                get children() {
                                                                                                    return [
                                                                                                        Pin(),
                                                                                                        " ",
                                                                                                        live(() => loc())
                                                                                                    ];
                                                                                                }
                                                                                            })
                                                                                        ]
                                                                                    })
                                                                                ];
                                                                            }
                                                                        })
                                                                    ];
                                                                }
                                                            }));
                                                        }
                                                    })
                                                ];
                                            }
                                        }),
                                        el("section", {
                                            "class": "card folks reveal d5",
                                            get children() {
                                                return [
                                                    el("div", {
                                                        "class": "card-hd",
                                                        "style": "padding:0 0 12px;border:0",
                                                        get children() {
                                                            return el("div", {
                                                                get children() {
                                                                    return [
                                                                        el("h2", {
                                                                            get children() {
                                                                                return "Active members";
                                                                            }
                                                                        }),
                                                                        el("div", {
                                                                            "class": "sub",
                                                                            get children() {
                                                                                return "Currently online or recently active";
                                                                            }
                                                                        })
                                                                    ];
                                                                }
                                                            });
                                                        }
                                                    }),
                                                    When({
                                                        get "when"() {
                                                            return (members() ?? []).filter((m) => m.memberId !== ownMemberId()).length > 0;
                                                        },
                                                        get "fallback"() {
                                                            return el("p", {
                                                                "style": "color:var(--ink-mute);padding:8px 0",
                                                                get children() {
                                                                    return "Just you here so far.";
                                                                }
                                                            });
                                                        },
                                                        get children() {
                                                            return Repeat({
                                                                get "each"() {
                                                                    return (members() ?? []).filter((m) => m.memberId !== ownMemberId());
                                                                },
                                                                children: (m) => (el("div", {
                                                                    get "class"() {
                                                                        return `folk ${memberStatus(m) === "away" ? "away" : ""}`;
                                                                    },
                                                                    get children() {
                                                                        return [
                                                                            el("span", {
                                                                                get "class"() {
                                                                                    return `a lg ${memberSwatch(m.memberId)}`;
                                                                                },
                                                                                get children() {
                                                                                    return live(() => initial(m));
                                                                                }
                                                                            }),
                                                                            el("div", {
                                                                                get children() {
                                                                                    return [
                                                                                        el("div", {
                                                                                            "class": "who-name",
                                                                                            get children() {
                                                                                                return live(() => displayName(m));
                                                                                            }
                                                                                        }),
                                                                                        When({
                                                                                            get "when"() {
                                                                                                return memberHandle(m);
                                                                                            },
                                                                                            children: (h) => el("div", {
                                                                                                "class": "doing",
                                                                                                get children() {
                                                                                                    return live(() => h());
                                                                                                }
                                                                                            })
                                                                                        })
                                                                                    ];
                                                                                }
                                                                            })
                                                                        ];
                                                                    }
                                                                }))
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
// ownMemberId pulls the caller's member id (in the current house) out of the
// auth store. Used so the "active members" panel doesn't echo the caller
// back to themselves. Sourced from /me's HouseSummary now that it carries
// the per-house member_id.
const ownMemberId = (): string | undefined => currentMemberId() ?? undefined;
const EmptyTasks = () => (el("div", {
    "style": "padding:24px;color:var(--ink-mute);text-align:center",
    get children() {
        return el("p", {
            "style": "margin:0",
            get children() {
                return el("b", {
                    get children() {
                        return "No open tasks.";
                    }
                });
            }
        });
    }
}));
const Stat = (props: {
    n: number;
    label: string;
    onClick?: () => void;
}) => (el("button", {
    "type": "button",
    "class": "stat",
    get "onClick"() {
        return props.onClick;
    },
    get "aria-label"() {
        return `${props.n} ${props.label} — open`;
    },
    get children() {
        return [
            el("span", {
                "class": "n",
                get children() {
                    return live(() => props.n);
                }
            }),
            el("span", {
                "class": "lbl",
                get children() {
                    return live(() => props.label);
                }
            })
        ];
    }
}));
const TaskRow = (props: {
    task: Task;
    byId: Map<string, Member>;
    members: Member[];
    onMutate: () => Promise<unknown>;
}) => {
    const [busy, setBusy] = cell(false);
    const [showDetail, setShowDetail] = cell(false);
    const closed = () => isTaskClosed(props.task);
    const toggleDone = async () => {
        if (busy())
            return;
        setBusy(true);
        try {
            await taskClient.updateTask({
                ...props.task,
                status: closed() ? "open" : "done",
            } as any);
            await props.onMutate();
        }
        finally {
            setBusy(false);
        }
    };
    const assignees = () => (props.task.assignees ?? [])
        .map((mid) => props.byId.get(mid))
        .filter((m): m is Member => Boolean(m))
        .map(toAvatar);
    return (el("div", {
        get children() {
            return [
                el("div", {
                    get "class"() {
                        return `task ${closed() ? "done" : ""}`;
                    },
                    get "style"() {
                        return busy() ? "opacity:0.5;pointer-events:none" : "";
                    },
                    get children() {
                        return [
                            el("button", {
                                "class": "check",
                                get "title"() {
                                    return closed() ? "Mark as open" : "Mark as done";
                                },
                                "onClick": (ev) => { ev.stopPropagation(); toggleDone(); },
                                "style": "cursor:pointer",
                                get children() {
                                    return Check();
                                }
                            }),
                            el("div", {
                                "onClick": () => setShowDetail((v) => !v),
                                "title": "Click to view / edit",
                                "style": "cursor:pointer;min-width:0",
                                get children() {
                                    return [
                                        el("div", {
                                            "class": "t-title",
                                            get children() {
                                                return live(() => props.task.title);
                                            }
                                        }),
                                        el("div", {
                                            "class": "t-meta",
                                            get children() {
                                                return [
                                                    When({
                                                        get "when"() {
                                                            return props.task.tag;
                                                        },
                                                        children: (tag) => el("span", {
                                                            get "class"() {
                                                                return `tag ${(tag() ?? "").toLowerCase()}`;
                                                            },
                                                            get children() {
                                                                return live(() => cap(tag()!));
                                                            }
                                                        })
                                                    }),
                                                    When({
                                                        get "when"() {
                                                            return dueLabel(props.task);
                                                        },
                                                        children: (label) => [
                                                            el("span", {
                                                                "class": "dot"
                                                            }),
                                                            live(() => label())
                                                        ]
                                                    })
                                                ];
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("div", {
                                "class": "t-end",
                                get children() {
                                    return When({
                                        get "when"() {
                                            return assignees().length > 0;
                                        },
                                        get children() {
                                            return AvatarStack({
                                                get "bits"() {
                                                    return assignees();
                                                }
                                            });
                                        }
                                    });
                                }
                            })
                        ];
                    }
                }),
                When({
                    get "when"() {
                        return showDetail();
                    },
                    get children() {
                        return TaskDetailEditor({
                            get "task"() {
                                return props.task;
                            },
                            get "members"() {
                                return props.members;
                            },
                            "onClose": () => setShowDetail(false),
                            "onSaved": async () => { await props.onMutate(); },
                            "onDelete": async () => { await taskClient.deleteTask(props.task.taskId); await props.onMutate(); }
                        });
                    }
                })
            ];
        }
    }));
};
const nextEventCopy = (n: number): string => {
    if (n === 0)
        return "nothing on the calendar yet";
    if (n === 1)
        return "one event on the calendar";
    return `${n} events on the calendar`;
};
const relativeWhen = (iso?: string): string => {
    if (!iso)
        return "—";
    const t = Date.parse(iso);
    if (Number.isNaN(t))
        return iso;
    const d = new Date(t);
    const now = new Date();
    const sameDay = d.toDateString() === now.toDateString();
    if (sameDay)
        return "Today";
    const tomorrow = new Date(now);
    tomorrow.setDate(now.getDate() + 1);
    if (d.toDateString() === tomorrow.toDateString())
        return "Tomorrow";
    return d.toLocaleDateString(undefined, { weekday: "long", month: "short", day: "numeric" });
};
const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);
/**
 * The dashboard's quick "New task": a private task for the caller. A loose
 * task cannot be private (docs/rbac.md), so the server puts it in the
 * caller's private "Personal" project, made on first use.
 */
const PersonalTaskComposer = (props: {
    houseId: string;
    onCancel: () => void;
    onCreated: () => Promise<void>;
}) => {
    const [title, setTitle] = cell("");
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        const t = title().trim();
        if (!t || busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            await taskClient.createPersonalTask({
                taskId: "",
                houseId: props.houseId,
                ownerMemberId: "",
                title: t,
                assignees: [],
                createdAt: "",
                updatedAt: "",
            } as unknown as Task);
            setTitle("");
            await props.onCreated();
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
        "style": "display:flex;flex-direction:column;gap:6px;padding:12px 18px;border-top:1px dashed var(--line)",
        get children() {
            return [
                el("div", {
                    "style": "display:flex;gap:8px;align-items:center",
                    get children() {
                        return [
                            el("input", {
                                "type": "text",
                                get "value"() {
                                    return title();
                                },
                                "onInput": (e) => setTitle(e.currentTarget.value),
                                "placeholder": "What needs doing?",
                                "aria-label": "New private task",
                                "autofocus": true,
                                "style": "flex:1;min-width:0;padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);color:var(--ink);font-size:14px"
                            }),
                            el("button", {
                                "class": "btn btn-primary",
                                "type": "submit",
                                get "disabled"() {
                                    return busy() || !title().trim();
                                },
                                get children() {
                                    return live(() => busy() ? "Adding…" : "Add");
                                }
                            }),
                            el("button", {
                                "class": "btn btn-ghost",
                                "type": "button",
                                "onClick": () => props.onCancel(),
                                get children() {
                                    return "Cancel";
                                }
                            })
                        ];
                    }
                }),
                el("span", {
                    "style": "font-size:12px;color:var(--ink-mute)",
                    get children() {
                        return "Private to you. It goes in your Personal project.";
                    }
                }),
                When({
                    get "when"() {
                        return err();
                    },
                    children: (m) => el("span", {
                        "style": "color:var(--rust);font-size:12px",
                        get children() {
                            return live(() => m());
                        }
                    })
                })
            ];
        }
    }));
};
