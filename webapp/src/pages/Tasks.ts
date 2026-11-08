import { transaction, observe, computed, query, cell } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { Check, Plus } from "~/components/Icons";
import { AuthGate } from "~/components/AuthGate";
import { AvatarStack } from "~/components/Avatar";
import { DateTimePicker } from "~/components/DateTimePicker";
import { RecurrenceFields, recurrenceLabel, toRecurrence, type RecurrenceFreq } from "~/components/RecurrenceFields";
import { TaskDetailEditor } from "~/components/TaskDetailEditor";
import { TaskFeedList } from "~/components/TaskFeedList";
import { memberClient, taskClient } from "~/data/clients";
import { displayName, dueLabel, initial, isTaskClosed, memberSwatch, taskGroup, toAvatar } from "~/lib/derive";
import { normalizeTaskCreate } from "~/lib/taskCreate";
import { createTaskFeed, dueBoundaries, dueGroupStarts, listAllTasks } from "~/lib/taskFeed";
import { currentMemberId, useCurrentHouseId } from "~/stores/auth";
import type { Member, Task, TaskListRequest } from "@longhouse/client";
// Group labels in due order: the order of the list and of dueBoundaries().
const GROUP_LABELS = ["Overdue", "Today", "This week", "Later", "No date"];
export const TasksPage = () => {
    const houseId = useCurrentHouseId();
    // Open tasks arrive due-date first (no date last). The first page also
    // brings the due counts that place each group header, so a header sits
    // in the right spot before the rows around it load. Closed tasks are a
    // second feed, most recently finished first.
    const openFeed = createTaskFeed();
    const closedFeed = createTaskFeed();
    observe(() => {
        const h = houseId();
        if (!h) {
            openFeed.reset(null);
            closedFeed.reset(null);
            return;
        }
        const bounds = dueBoundaries();
        openFeed.reset((offset, limit, withCounts) => taskClient.listTasks({ houseId: h, state: "open", offset, limit, ...(withCounts ? { dueBefore: bounds } : {}) }));
        closedFeed.reset((offset, limit) => taskClient.listTasks({ houseId: h, state: "closed", offset, limit }));
    });
    const refreshAll = async () => {
        await Promise.all([openFeed.refresh(), closedFeed.refresh()]);
    };
    const [members] = query(() => houseId(), async (h) => memberClient.listMembers({ houseId: h }));
    const memberById = computed(() => {
        const map = new Map<string, Member>();
        for (const m of members() ?? [])
            map.set(m.memberId, m);
        return map;
    });
    const openGroups = computed(() => dueGroupStarts(openFeed.dueCounts(), GROUP_LABELS));
    const openCount = () => openFeed.estimatedTotal();
    const closedCount = () => closedFeed.estimatedTotal();
    const firstLoad = () => !openFeed.ready() || !closedFeed.ready();
    const [composerOpen, setComposerOpen] = cell(false);
    const [completedOpen, setCompletedOpen] = cell(false);
    const renderTask = (t: Task) => Row({
        get "task"() {
            return t;
        },
        get "members"() {
            return members() ?? [];
        },
        get "byId"() {
            return memberById();
        },
        get "onMutate"() {
            return refreshAll;
        }
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
                                        "Tasks ",
                                        el("em", {
                                            get children() {
                                                return "everything on the slate";
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("p", {
                                "class": "lead",
                                get children() {
                                    return "Everything assigned to you, watched by you, or shared with you.";
                                }
                            })
                        ];
                    }
                }),
                el("section", {
                    "class": "card reveal d1",
                    "style": "margin-top: 0",
                    get children() {
                        return [
                            el("div", {
                                "class": "card-hd",
                                get children() {
                                    return [
                                        el("div", {
                                            get children() {
                                                return [
                                                    el("h2", {
                                                        get children() {
                                                            return "Open tasks";
                                                        }
                                                    }),
                                                    el("div", {
                                                        "class": "sub",
                                                        get children() {
                                                            return live(() => firstLoad() ? "…" : `${openCount()} open · ${openCount() + closedCount()} total`);
                                                        }
                                                    })
                                                ];
                                            }
                                        }),
                                        el("button", {
                                            "class": "btn-quiet",
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
                            }),
                            When({
                                get "when"() {
                                    return composerOpen();
                                },
                                get children() {
                                    return TaskComposer({
                                        get "houseId"() {
                                            return houseId()!;
                                        },
                                        get "members"() {
                                            return members() ?? [];
                                        },
                                        "onCancel": () => setComposerOpen(false),
                                        "onCreated": async () => {
                                            setComposerOpen(false);
                                            await refreshAll();
                                        }
                                    });
                                }
                            }),
                            When({
                                get "when"() {
                                    return openFeed.error();
                                },
                                children: (e) => el("p", {
                                    "class": "err",
                                    "role": "alert",
                                    "style": "padding:0 20px",
                                    get children() {
                                        return live(() => e() instanceof Error ? (e() as Error).message : "Could not load tasks.");
                                    }
                                })
                            }),
                            When({
                                get "when"() {
                                    return !firstLoad();
                                },
                                get "fallback"() {
                                    return el("p", {
                                        "style": "padding:20px;color:var(--ink-mute)",
                                        get children() {
                                            return "Loading tasks\u2026";
                                        }
                                    });
                                },
                                get children() {
                                    return When({
                                        get "when"() {
                                            return openCount() + closedCount() > 0;
                                        },
                                        get "fallback"() {
                                            return EmptyTasks({
                                                "onCreate": () => setComposerOpen(true)
                                            });
                                        },
                                        get children() {
                                            return el("div", {
                                                "class": "tasks",
                                                get children() {
                                                    return [
                                                        TaskFeedList({
                                                            get "feed"() {
                                                                return openFeed;
                                                            },
                                                            get "groups"() {
                                                                return openGroups();
                                                            },
                                                            get "renderTask"() {
                                                                return renderTask;
                                                            }
                                                        }),
                                                        When({
                                                            get "when"() {
                                                                return closedCount() > 0;
                                                            },
                                                            get children() {
                                                                return el("div", {
                                                                    "style": "margin-top:24px;padding-top:12px;border-top:1px solid var(--line)",
                                                                    get children() {
                                                                        return [
                                                                            el("button", {
                                                                                "type": "button",
                                                                                "onClick": () => setCompletedOpen((v) => !v),
                                                                                "style": "background:transparent;border:0;cursor:pointer;display:flex;align-items:center;gap:8px;padding:6px 0;color:var(--ink-mute);font-size:13px;letter-spacing:0.08em;text-transform:uppercase;font-weight:600",
                                                                                get children() {
                                                                                    return [
                                                                                        el("span", {
                                                                                            get children() {
                                                                                                return live(() => completedOpen() ? "▾" : "▸");
                                                                                            }
                                                                                        }),
                                                                                        "Completed (",
                                                                                        live(() => closedCount()),
                                                                                        ")"
                                                                                    ];
                                                                                }
                                                                            }),
                                                                            When({
                                                                                get "when"() {
                                                                                    return completedOpen();
                                                                                },
                                                                                get children() {
                                                                                    return TaskFeedList({
                                                                                        get "feed"() {
                                                                                            return closedFeed;
                                                                                        },
                                                                                        get "renderTask"() {
                                                                                            return renderTask;
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
};
// ─── Composer ─────────────────────────────────────────────────────────
const TaskComposer = (props: {
    houseId: string;
    members: Member[];
    onCancel: () => void;
    onCreated: () => Promise<void> | void;
}) => {
    // The parent picker needs every open task, not just the loaded pages.
    const [openTasks] = query(() => props.houseId, (h) => listAllTasks((offset, limit) => taskClient.listTasks({ houseId: h, state: "open", offset, limit })));
    const [title, setTitle] = cell("");
    const [tag, setTag] = cell("");
    const [due, setDue] = cell("");
    const [estimate, setEstimate] = cell("");
    const [parentId, setParentId] = cell("");
    // assignees is undefined while the user hasn't changed the default
    // ("inherit caller / parent on the server"). Toggling any chip flips
    // to a concrete array we send on the wire.
    const [assignees, setAssignees] = cell<string[] | null>(null);
    const [recFreq, setRecFreq] = cell<RecurrenceFreq>("");
    const [recInterval, setRecInterval] = cell(1);
    const [recNextAt, setRecNextAt] = cell("");
    const [recByWeekday, setRecByWeekday] = cell<number[]>([]);
    const [recBySetpos, setRecBySetpos] = cell(1);
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    // Default chip set for the picker. When the parent picker is set, show
    // the parent's current assignees as the implicit default so the user
    // can opt into "same as parent" by leaving them untouched. Otherwise
    // show the caller as the implicit default. The chip toggle materializes
    // those into the concrete list on first interaction.
    const defaultAssigneeIds = computed(() => {
        const pid = parentId();
        if (pid) {
            const parent = (openTasks() ?? []).find((t) => t.taskId === pid);
            return parent?.assignees ?? [];
        }
        const me = currentMemberId();
        return me ? [me] : [];
    });
    // The selected list shown to the user: explicit selection if any,
    // otherwise the implicit default.
    const effectiveAssignees = () => assignees() ?? defaultAssigneeIds();
    const toggleAssignee = (id: string) => {
        setAssignees((prev) => {
            const base = prev ?? defaultAssigneeIds();
            const next = base.includes(id) ? base.filter((x) => x !== id) : [...base, id];
            return next;
        });
    };
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        if (!title().trim() || busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            // If the user never touched the chip set, omit assignees so the
            // server applies its default-assignee policy (caller for top-level,
            // parent's for subtasks). If they touched it, send the explicit list
            // — including the empty list for "nobody yet."
            const dueIso = due() || undefined;
            const body: any = {
                houseId: props.houseId,
                ownerMemberId: "",
                title: title().trim(),
                tag: tag().trim() || undefined,
                dueAt: dueIso,
                estimateMinutes: estimate() ? Number(estimate()) : undefined,
                ...toRecurrence(recFreq(), recInterval(), recNextAt(), recByWeekday(), recBySetpos(), dueIso),
            };
            if (parentId())
                body.parentTaskId = parentId();
            if (assignees() !== null)
                body.assignees = assignees();
            await taskClient.createTask(normalizeTaskCreate(body));
            transaction(() => {
                setTitle("");
                setTag("");
                setDue("");
                setEstimate("");
                setParentId("");
                setRecFreq("");
                setRecInterval(1);
                setRecNextAt("");
                setRecByWeekday([]);
                setRecBySetpos(1);
                setAssignees(null);
            });
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
        "style": "display:flex;flex-direction:column;gap:10px;padding:14px 18px;background:color-mix(in oklab, var(--paper) 92%, var(--ocean-1) 8%);border-top:1px solid var(--line);border-bottom:1px solid var(--line)",
        get children() {
            return [
                el("div", {
                    "style": "display:flex;flex-wrap:wrap;gap:10px;align-items:center",
                    get children() {
                        return [
                            el("input", {
                                "type": "text",
                                "placeholder": "What needs doing?",
                                get "value"() {
                                    return title();
                                },
                                "onInput": (e) => setTitle(e.currentTarget.value),
                                "style": "flex:2 1 320px;padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px",
                                "autofocus": true,
                                "required": true
                            }),
                            el("input", {
                                "type": "text",
                                "placeholder": "tag (house, barn, \u2026)",
                                get "value"() {
                                    return tag();
                                },
                                "onInput": (e) => setTag(e.currentTarget.value),
                                "style": "flex:1 1 140px;padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
                            }),
                            DateTimePicker({
                                get "value"() {
                                    return due();
                                },
                                get "onChange"() {
                                    return setDue;
                                },
                                "title": "Due (optional)"
                            }),
                            el("input", {
                                "type": "number",
                                "placeholder": "est (min)",
                                "min": "0",
                                get "value"() {
                                    return estimate();
                                },
                                "onInput": (e) => setEstimate(e.currentTarget.value),
                                "style": "width:96px;padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
                            })
                        ];
                    }
                }),
                el("div", {
                    "style": "display:flex;flex-wrap:wrap;gap:12px;align-items:center",
                    get children() {
                        return [
                            el("label", {
                                "style": "display:flex;align-items:center;gap:6px;font-size:12px;color:var(--ink-mute)",
                                get children() {
                                    return [
                                        "Subtask of",
                                        el("select", {
                                            get "value"() {
                                                return parentId();
                                            },
                                            "onChange": (e) => {
                                                setParentId(e.currentTarget.value);
                                                // resetting parent → fall back to the implicit default again
                                                setAssignees(null);
                                            },
                                            "style": "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px",
                                            get children() {
                                                return [
                                                    el("option", {
                                                        "value": "",
                                                        get children() {
                                                            return "\u2014 top-level \u2014";
                                                        }
                                                    }),
                                                    Repeat({
                                                        get "each"() {
                                                            return openTasks() ?? [];
                                                        },
                                                        children: (t) => el("option", {
                                                            get "value"() {
                                                                return t.taskId;
                                                            },
                                                            get children() {
                                                                return live(() => t.title);
                                                            }
                                                        })
                                                    })
                                                ];
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("div", {
                                "style": "display:flex;flex-wrap:wrap;align-items:center;gap:6px",
                                get children() {
                                    return [
                                        el("span", {
                                            "style": "font-size:12px;color:var(--ink-mute)",
                                            get children() {
                                                return "Assignees";
                                            }
                                        }),
                                        Repeat({
                                            get "each"() {
                                                return props.members;
                                            },
                                            children: (m) => {
                                                const selected = () => effectiveAssignees().includes(m.memberId);
                                                return (el("button", {
                                                    "type": "button",
                                                    "onClick": () => toggleAssignee(m.memberId),
                                                    get "style"() {
                                                        return `display:inline-flex;align-items:center;gap:4px;padding:3px 8px 3px 3px;border-radius:9999px;font-size:12px;cursor:pointer;border:1px solid ${selected() ? "var(--grass-4)" : "var(--line)"};background:${selected() ? "color-mix(in oklab, var(--grass-1) 30%, var(--paper))" : "var(--paper)"};color:var(--ink)`;
                                                    },
                                                    get children() {
                                                        return [
                                                            el("span", {
                                                                get "class"() {
                                                                    return `a sm ${memberSwatch(m.memberId)}`;
                                                                },
                                                                "style": "width:18px;height:18px;font-size:10px",
                                                                get children() {
                                                                    return live(() => initial(m));
                                                                }
                                                            }),
                                                            live(() => displayName(m))
                                                        ];
                                                    }
                                                }));
                                            }
                                        }),
                                        When({
                                            get "when"() {
                                                return assignees() === null;
                                            },
                                            get children() {
                                                return el("span", {
                                                    "style": "font-size:11px;color:var(--ink-mute);font-style:italic",
                                                    get children() {
                                                        return [
                                                            "(",
                                                            live(() => parentId() ? "inherits from parent" : "defaults to you"),
                                                            ")"
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
                }),
                RecurrenceFields({
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
                }),
                When({
                    get "when"() {
                        return err();
                    },
                    children: (m) => el("span", {
                        "style": "color:var(--rust);font-size:13px",
                        get children() {
                            return live(() => m());
                        }
                    })
                }),
                el("div", {
                    "style": "display:flex;gap:10px;justify-content:flex-end",
                    get children() {
                        return [
                            el("button", {
                                "type": "button",
                                "class": "btn btn-ghost",
                                get "onClick"() {
                                    return props.onCancel;
                                },
                                get "disabled"() {
                                    return busy();
                                },
                                get children() {
                                    return "Cancel";
                                }
                            }),
                            el("button", {
                                "class": "btn btn-primary",
                                get "disabled"() {
                                    return busy() || !title().trim();
                                },
                                "type": "submit",
                                get children() {
                                    return live(() => busy() ? "Saving…" : "Add");
                                }
                            })
                        ];
                    }
                })
            ];
        }
    }));
};
const EmptyTasks = (props: {
    onCreate: () => void;
}) => (el("div", {
    "style": "padding:32px;color:var(--ink-mute);text-align:center",
    get children() {
        return [
            el("p", {
                "style": "margin:0 0 8px",
                get children() {
                    return el("b", {
                        get children() {
                            return "No tasks yet.";
                        }
                    });
                }
            }),
            el("p", {
                "style": "margin:0 0 16px",
                get children() {
                    return "Create your first task to get the slate started.";
                }
            }),
            el("button", {
                "class": "btn btn-primary",
                get "onClick"() {
                    return props.onCreate;
                },
                get children() {
                    return [
                        Plus(),
                        " New task"
                    ];
                }
            })
        ];
    }
}));
// ─── Group + Row ──────────────────────────────────────────────────────
const GroupSection = (props: {
    label: string;
    items: Task[];
    members: Member[];
    byId: Map<string, Member>;
    onMutate: () => Promise<unknown>;
}) => (When({
    get "when"() {
        return props.items.length > 0;
    },
    get children() {
        return [
            el("div", {
                "class": "group-lbl",
                get children() {
                    return live(() => props.label);
                }
            }),
            Repeat({
                get "each"() {
                    return props.items;
                },
                children: (t) => Row({
                    get "task"() {
                        return t;
                    },
                    get "members"() {
                        return props.members;
                    },
                    get "byId"() {
                        return props.byId;
                    },
                    get "onMutate"() {
                        return props.onMutate;
                    }
                })
            })
        ];
    }
}));
const Row = (props: {
    task: Task;
    members: Member[];
    byId: Map<string, Member>;
    onMutate: () => Promise<unknown>;
}) => {
    const [busy, setBusy] = cell(false);
    const [showAssignees, setShowAssignees] = cell(false);
    const [showSubtask, setShowSubtask] = cell(false);
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
    const remove = async (e: MouseEvent) => {
        e.stopPropagation();
        if (busy())
            return;
        if (!confirm(`Delete "${props.task.title}"?`))
            return;
        setBusy(true);
        try {
            await taskClient.deleteTask(props.task.taskId);
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
    const saveAssignees = async (next: string[]) => {
        if (busy())
            return;
        setBusy(true);
        try {
            await taskClient.updateTask({
                ...props.task,
                assignees: next,
            } as any);
            await props.onMutate();
            setShowAssignees(false);
        }
        finally {
            setBusy(false);
        }
    };
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
                                                    }),
                                                    When({
                                                        get "when"() {
                                                            return props.task.estimateMinutes !== undefined && props.task.estimateMinutes! > 0;
                                                        },
                                                        get children() {
                                                            return [
                                                                el("span", {
                                                                    "class": "dot"
                                                                }),
                                                                live(() => props.task.estimateMinutes),
                                                                "m est."
                                                            ];
                                                        }
                                                    }),
                                                    When({
                                                        get "when"() {
                                                            return recurrenceLabel(props.task);
                                                        },
                                                        children: (label) => [
                                                            el("span", {
                                                                "class": "dot"
                                                            }),
                                                            "\uD83D\uDD01 ",
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
                                "style": "display:flex;align-items:center;gap:6px",
                                get children() {
                                    return [
                                        el("button", {
                                            "type": "button",
                                            "onClick": () => setShowAssignees((v) => !v),
                                            "title": "Edit assignees",
                                            "style": "background:transparent;border:0;padding:2px;cursor:pointer;border-radius:9999px",
                                            get children() {
                                                return When({
                                                    get "when"() {
                                                        return assignees().length > 0;
                                                    },
                                                    get "fallback"() {
                                                        return When({
                                                            get "when"() {
                                                                return ownerMember(props.task, props.byId);
                                                            },
                                                            get "fallback"() {
                                                                return el("span", {
                                                                    "style": "font-size:11px;color:var(--ink-mute);padding:0 6px",
                                                                    get children() {
                                                                        return "+ assign";
                                                                    }
                                                                });
                                                            },
                                                            children: (m) => (el("span", {
                                                                get "class"() {
                                                                    return `a sm ${memberSwatch(m().memberId)}`;
                                                                },
                                                                get "title"() {
                                                                    return `owner: ${displayName(m())}`;
                                                                },
                                                                get children() {
                                                                    return live(() => initial(m()));
                                                                }
                                                            }))
                                                        });
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
                                        }),
                                        el("button", {
                                            "type": "button",
                                            "onClick": () => setShowSubtask((v) => !v),
                                            "title": "Add a subtask",
                                            "style": "background:transparent;border:1px dashed var(--line);color:var(--ink-mute);font-size:11px;padding:2px 8px;border-radius:9999px;cursor:pointer",
                                            get children() {
                                                return "+ subtask";
                                            }
                                        }),
                                        el("button", {
                                            "aria-label": "Delete task",
                                            "title": "Delete task",
                                            get "onClick"() {
                                                return remove;
                                            },
                                            "style": "background:transparent;border:0;color:var(--ink-mute);font-size:18px;line-height:1;padding:4px 8px;cursor:pointer",
                                            get children() {
                                                return "\u00D7";
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
                        return showAssignees();
                    },
                    get children() {
                        return AssigneeEditor({
                            get "current"() {
                                return (props.task.assignees ?? []).slice();
                            },
                            get "members"() {
                                return props.members;
                            },
                            "onCancel": () => setShowAssignees(false),
                            get "onSave"() {
                                return saveAssignees;
                            }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return showSubtask();
                    },
                    get children() {
                        return SubtaskComposer({
                            get "parent"() {
                                return props.task;
                            },
                            get "parentAssignees"() {
                                return (props.task.assignees ?? []).slice();
                            },
                            get "members"() {
                                return props.members;
                            },
                            "onCancel": () => setShowSubtask(false),
                            "onCreated": async () => { setShowSubtask(false); await props.onMutate(); }
                        });
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
// ─── Inline assignee editor ───────────────────────────────────────────
const AssigneeEditor = (props: {
    current: string[];
    members: Member[];
    onCancel: () => void;
    onSave: (next: string[]) => Promise<void> | void;
}) => {
    const [selected, setSelected] = cell<string[]>(props.current.slice());
    const [busy, setBusy] = cell(false);
    const toggle = (id: string) => setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
    const save = async () => {
        if (busy())
            return;
        setBusy(true);
        try {
            await props.onSave(selected());
        }
        finally {
            setBusy(false);
        }
    };
    return (el("div", {
        "style": "margin:6px 0 10px 38px;padding:10px 12px;background:color-mix(in oklab, var(--paper) 92%, var(--ocean-1) 8%);border:1px solid var(--line);border-radius:var(--r-md);display:flex;flex-wrap:wrap;gap:6px;align-items:center",
        get children() {
            return [
                Repeat({
                    get "each"() {
                        return props.members;
                    },
                    children: (m) => {
                        const on = () => selected().includes(m.memberId);
                        return (el("button", {
                            "type": "button",
                            "onClick": () => toggle(m.memberId),
                            get "style"() {
                                return `display:inline-flex;align-items:center;gap:4px;padding:3px 8px 3px 3px;border-radius:9999px;font-size:12px;cursor:pointer;border:1px solid ${on() ? "var(--grass-4)" : "var(--line)"};background:${on() ? "color-mix(in oklab, var(--grass-1) 30%, var(--paper))" : "var(--paper)"};color:var(--ink)`;
                            },
                            get children() {
                                return [
                                    el("span", {
                                        get "class"() {
                                            return `a sm ${memberSwatch(m.memberId)}`;
                                        },
                                        "style": "width:18px;height:18px;font-size:10px",
                                        get children() {
                                            return live(() => initial(m));
                                        }
                                    }),
                                    live(() => displayName(m))
                                ];
                            }
                        }));
                    }
                }),
                el("button", {
                    "type": "button",
                    "class": "btn btn-primary",
                    get "onClick"() {
                        return save;
                    },
                    get "disabled"() {
                        return busy();
                    },
                    "style": "margin-left:auto;padding:4px 12px",
                    get children() {
                        return live(() => busy() ? "…" : "Save");
                    }
                }),
                el("button", {
                    "type": "button",
                    "class": "btn btn-ghost",
                    get "onClick"() {
                        return props.onCancel;
                    },
                    get "disabled"() {
                        return busy();
                    },
                    "style": "padding:4px 10px",
                    get children() {
                        return "Cancel";
                    }
                })
            ];
        }
    }));
};
// ─── Inline subtask composer ──────────────────────────────────────────
const SubtaskComposer = (props: {
    parent: Task;
    parentAssignees: string[];
    members: Member[];
    onCancel: () => void;
    onCreated: () => Promise<void> | void;
    /** Optional hook so a project context can also attach the new subtask
     *  to the same project after the task is created. */
    afterCreate?: (newTaskId: string) => Promise<void> | void;
}) => {
    const [title, setTitle] = cell("");
    const [tag, setTag] = cell("");
    const [due, setDue] = cell("");
    const [estimate, setEstimate] = cell("");
    // assignees null = inherit from parent (server already does this when
    // assignees field is omitted). User can override by toggling chips.
    const [assignees, setAssignees] = cell<string[] | null>(null);
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const effective = () => assignees() ?? props.parentAssignees;
    const toggleA = (id: string) => setAssignees((prev) => {
        const base = prev ?? props.parentAssignees;
        return base.includes(id) ? base.filter((x) => x !== id) : [...base, id];
    });
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        if (!title().trim() || busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            const body: any = {
                houseId: props.parent.houseId,
                ownerMemberId: "",
                title: title().trim(),
                tag: tag().trim() || undefined,
                dueAt: due() || undefined,
                estimateMinutes: estimate() ? Number(estimate()) : undefined,
                parentTaskId: props.parent.taskId,
            };
            if (assignees() !== null)
                body.assignees = assignees();
            const created = await taskClient.createTask(normalizeTaskCreate(body));
            if (props.afterCreate)
                await props.afterCreate(created.taskId);
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
        "style": "margin:6px 0 10px 38px;padding:12px 14px;background:color-mix(in oklab, var(--paper) 92%, var(--grass-1) 8%);border:1px solid var(--line);border-radius:var(--r-md);display:flex;flex-direction:column;gap:8px",
        get children() {
            return [
                el("div", {
                    "style": "display:flex;flex-wrap:wrap;gap:8px;align-items:center",
                    get children() {
                        return [
                            el("input", {
                                "type": "text",
                                "placeholder": "Subtask title\u2026",
                                get "value"() {
                                    return title();
                                },
                                "onInput": (e) => setTitle(e.currentTarget.value),
                                "required": true,
                                "autofocus": true,
                                "style": "flex:2 1 260px;padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
                            }),
                            el("input", {
                                "type": "text",
                                "placeholder": "tag",
                                get "value"() {
                                    return tag();
                                },
                                "onInput": (e) => setTag(e.currentTarget.value),
                                "style": "flex:0 1 120px;padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
                            }),
                            DateTimePicker({
                                get "value"() {
                                    return due();
                                },
                                get "onChange"() {
                                    return setDue;
                                },
                                "title": "Due (optional)"
                            }),
                            el("input", {
                                "type": "number",
                                "min": "0",
                                "placeholder": "est min",
                                get "value"() {
                                    return estimate();
                                },
                                "onInput": (e) => setEstimate(e.currentTarget.value),
                                "style": "width:88px;padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
                            })
                        ];
                    }
                }),
                el("div", {
                    "style": "display:flex;flex-wrap:wrap;gap:6px;align-items:center",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
                                get children() {
                                    return "Assignees";
                                }
                            }),
                            Repeat({
                                get "each"() {
                                    return props.members;
                                },
                                children: (m) => {
                                    const on = () => effective().includes(m.memberId);
                                    return (el("button", {
                                        "type": "button",
                                        "onClick": () => toggleA(m.memberId),
                                        get "style"() {
                                            return `display:inline-flex;align-items:center;gap:4px;padding:2px 8px 2px 2px;border-radius:9999px;font-size:11px;cursor:pointer;border:1px solid ${on() ? "var(--grass-4)" : "var(--line)"};background:${on() ? "color-mix(in oklab, var(--grass-1) 30%, var(--paper))" : "var(--paper)"};color:var(--ink)`;
                                        },
                                        get children() {
                                            return [
                                                el("span", {
                                                    get "class"() {
                                                        return `a sm ${memberSwatch(m.memberId)}`;
                                                    },
                                                    "style": "width:16px;height:16px;font-size:9px",
                                                    get children() {
                                                        return live(() => initial(m));
                                                    }
                                                }),
                                                live(() => displayName(m))
                                            ];
                                        }
                                    }));
                                }
                            }),
                            When({
                                get "when"() {
                                    return assignees() === null;
                                },
                                get children() {
                                    return el("span", {
                                        "style": "font-size:11px;color:var(--ink-mute);font-style:italic",
                                        get children() {
                                            return "(inherits from parent)";
                                        }
                                    });
                                }
                            })
                        ];
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
                }),
                el("div", {
                    "style": "display:flex;gap:8px;justify-content:flex-end",
                    get children() {
                        return [
                            el("button", {
                                "type": "button",
                                "class": "btn btn-ghost",
                                get "onClick"() {
                                    return props.onCancel;
                                },
                                get "disabled"() {
                                    return busy();
                                },
                                "style": "padding:4px 12px",
                                get children() {
                                    return "Cancel";
                                }
                            }),
                            el("button", {
                                "type": "submit",
                                "class": "btn btn-primary",
                                get "disabled"() {
                                    return busy() || !title().trim();
                                },
                                "style": "padding:4px 12px",
                                get children() {
                                    return live(() => busy() ? "…" : "Add subtask");
                                }
                            })
                        ];
                    }
                })
            ];
        }
    }));
};
const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);
const ownerMember = (t: Task, byId: Map<string, Member>): Member | undefined => {
    if (!t.ownerMemberId)
        return undefined;
    return byId.get(t.ownerMemberId);
};
