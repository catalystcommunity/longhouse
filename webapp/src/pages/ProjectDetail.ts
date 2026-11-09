import { transaction, observe, computed, query, cell } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { Link, navigation, routeParams } from "~/ui/router";
import { ProjectDeadlines } from "~/components/ProjectDeadlines";
import { AuthGate } from "~/components/AuthGate";
import { AvatarStack } from "~/components/Avatar";
import { Check } from "~/components/Icons";
import { DateTimePicker } from "~/components/DateTimePicker";
import { RecurrenceFields, recurrenceLabel, toRecurrence, type RecurrenceFreq } from "~/components/RecurrenceFields";
import { TaskDetailEditor } from "~/components/TaskDetailEditor";
import { CommentsSection } from "~/components/CommentsSection";
import { DependenciesSection } from "~/components/DependenciesSection";
import { memberClient, projectClient, taskClient } from "~/data/clients";
import { displayName, dueLabel, initial, isTaskClosed, memberSwatch, toAvatar } from "~/lib/derive";
import { normalizeTaskCreate } from "~/lib/taskCreate";
import { hasRole } from "~/stores/auth";
import { createTaskFeed, listAllTasks, type TaskFeed } from "~/lib/taskFeed";
import { TaskFeedList } from "~/components/TaskFeedList";
import type { Member, Milestone, Project, Task } from "@longhouse/client";
const MILESTONE_STATES = ["future", "current", "done"] as const;
interface DetailBundle {
    project: Project;
    houseMembers: Member[];
    members: Member[];
    owners: Member[];
    milestones: Milestone[];
}
export const ProjectDetail = () => {
    const params = routeParams<{
        slug: string;
    }>();
    const navigate = navigation();
    const projectId = () => params.slug;
    const [bundle, { refetch }] = query(projectId, async (id): Promise<DetailBundle | null> => {
        try {
            const project = await projectClient.getProject(id);
            const [houseMembers, members, owners, milestones] = await Promise.all([
                memberClient.listMembers({ houseId: project.houseId }),
                projectClient.listProjectMembers(id),
                projectClient.listProjectOwners(id),
                projectClient.listMilestones(id),
            ]);
            return { project, houseMembers, members, owners, milestones };
        }
        catch {
            return null;
        }
    });
    // The project's tasks, paged in project order. Counts come from the
    // server, so the progress bar is right before every page loads. Done
    // means closed: done or cancelled.
    const taskFeed = createTaskFeed();
    // Same ids, same key: a bundle reload (members, milestones) must not
    // reset the task list.
    const projectKey = computed(() => {
        const p = bundle()?.project;
        return p ? { houseId: p.houseId, projectId: p.projectId } : null;
    }, undefined, { equals: (a, b) => a?.projectId === b?.projectId && a?.houseId === b?.houseId });
    observe(() => {
        const k = projectKey();
        taskFeed.reset(k ? (offset, limit) => projectClient.listProjectTasks({ ...k, offset, limit }) : null);
    });
    const [closedCount, { refetch: refetchClosed }] = query(projectKey, async (k) => {
        const res = await projectClient.listProjectTasks({ ...k, state: "closed", limit: 1 });
        return Number(res.total ?? 0);
    });
    const doneCount = () => (closedCount.state === "ready" ? closedCount() : 0);
    const progress = computed(() => {
        const total = taskFeed.total();
        return total === 0 ? 0 : Math.round((doneCount() / total) * 100);
    });
    const [deadlineRevision, setDeadlineRevision] = cell(0);
    const refreshTasks = async () => {
        setDeadlineRevision(v => v + 1);
        await Promise.all([taskFeed.refresh(), refetchClosed()]);
    };
    const [editing, setEditing] = cell(false);
    const isAdmin = () => hasRole("admin");
    const onDeleteProject = async () => {
        const b = bundle();
        if (!b)
            return;
        if (!confirm(`Delete project "${b.project.name}"? This is permanent.`))
            return;
        try {
            await projectClient.deleteProject(b.project.projectId);
            navigate("/projects", { replace: true });
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
    };
    return (AuthGate({
        get children() {
            return When({
                get "when"() {
                    return !bundle.loading;
                },
                get "fallback"() {
                    return el("p", {
                        "style": "padding:48px;text-align:center;color:var(--ink-mute)",
                        get children() {
                            return "Loading\u2026";
                        }
                    });
                },
                get children() {
                    return When({
                        get "when"() {
                            return bundle();
                        },
                        get "fallback"() {
                            return el("div", {
                                "style": "padding:80px 0;text-align:center;color:var(--ink-mute)",
                                get children() {
                                    return [
                                        el("p", {
                                            get children() {
                                                return "Project not found.";
                                            }
                                        }),
                                        el("p", {
                                            get children() {
                                                return Link({
                                                    "href": "/projects",
                                                    "style": "color:var(--ocean-2)",
                                                    get children() {
                                                        return "Back to projects";
                                                    }
                                                });
                                            }
                                        })
                                    ];
                                }
                            });
                        },
                        children: (b) => {
                            const p = () => b().project;
                            return ([
                                el("div", {
                                    "class": "divider",
                                    get children() {
                                        return [
                                            el("span", {
                                                "class": "rule"
                                            }),
                                            el("span", {
                                                "class": "label",
                                                get children() {
                                                    return "project detail";
                                                }
                                            }),
                                            el("span", {
                                                "class": "rule"
                                            })
                                        ];
                                    }
                                }),
                                el("article", {
                                    "class": "room-detail reveal",
                                    get children() {
                                        return [
                                            el("div", {
                                                "class": "room-banner",
                                                get children() {
                                                    return el("span", {
                                                        "class": "banner-tag",
                                                        get children() {
                                                            return [
                                                                live(() => p().category ?? "Project"),
                                                                " \u00B7 ",
                                                                live(() => progress()),
                                                                "% complete"
                                                            ];
                                                        }
                                                    });
                                                }
                                            }),
                                            el("div", {
                                                "class": "room-meta",
                                                get children() {
                                                    return [
                                                        el("div", {
                                                            "class": "room-title",
                                                            get children() {
                                                                return [
                                                                    el("div", {
                                                                        "class": "crumbs",
                                                                        get children() {
                                                                            return [
                                                                                Link({
                                                                                    "href": "/projects",
                                                                                    get children() {
                                                                                        return "Projects";
                                                                                    }
                                                                                }),
                                                                                el("i", {}),
                                                                                When({
                                                                                    get "when"() {
                                                                                        return p().category;
                                                                                    },
                                                                                    children: (c) => [
                                                                                        live(() => c()),
                                                                                        el("i", {})
                                                                                    ]
                                                                                }),
                                                                                el("span", {
                                                                                    get children() {
                                                                                        return live(() => p().name);
                                                                                    }
                                                                                })
                                                                            ];
                                                                        }
                                                                    }),
                                                                    When({
                                                                        get "when"() {
                                                                            return !editing();
                                                                        },
                                                                        get "fallback"() {
                                                                            return ProjectEdit({
                                                                                get "project"() {
                                                                                    return p();
                                                                                },
                                                                                "onCancel": () => setEditing(false),
                                                                                "onSaved": async () => { setEditing(false); await refetch(); }
                                                                            });
                                                                        },
                                                                        get children() {
                                                                            return [
                                                                                el("h2", {
                                                                                    get children() {
                                                                                        return live(() => p().name);
                                                                                    }
                                                                                }),
                                                                                When({
                                                                                    get "when"() {
                                                                                        return p().description;
                                                                                    },
                                                                                    children: (d) => el("p", {
                                                                                        "class": "lede",
                                                                                        get children() {
                                                                                            return live(() => d());
                                                                                        }
                                                                                    })
                                                                                }),
                                                                                When({
                                                                                    get "when"() {
                                                                                        return isAdmin();
                                                                                    },
                                                                                    get children() {
                                                                                        return el("div", {
                                                                                            "style": "margin-top:12px;display:flex;gap:10px",
                                                                                            get children() {
                                                                                                return [
                                                                                                    el("button", {
                                                                                                        "class": "btn btn-ghost",
                                                                                                        "onClick": () => setEditing(true),
                                                                                                        get children() {
                                                                                                            return "Edit";
                                                                                                        }
                                                                                                    }),
                                                                                                    el("button", {
                                                                                                        "class": "btn btn-ghost",
                                                                                                        get "onClick"() {
                                                                                                            return onDeleteProject;
                                                                                                        },
                                                                                                        "style": "color:var(--rust)",
                                                                                                        get children() {
                                                                                                            return "Delete project";
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
                                                        }),
                                                        el("aside", {
                                                            "class": "facts",
                                                            get children() {
                                                                return el("dl", {
                                                                    get children() {
                                                                        return [
                                                                            el("dt", {
                                                                                get children() {
                                                                                    return "Owners";
                                                                                }
                                                                            }),
                                                                            el("dd", {
                                                                                get children() {
                                                                                    return PeopleEditor({
                                                                                        "kind": "owner",
                                                                                        get "project"() {
                                                                                            return p();
                                                                                        },
                                                                                        get "assigned"() {
                                                                                            return b().owners;
                                                                                        },
                                                                                        get "houseMembers"() {
                                                                                            return b().houseMembers;
                                                                                        },
                                                                                        get "canEdit"() {
                                                                                            return isAdmin();
                                                                                        },
                                                                                        "onChanged": async () => { await refetch(); }
                                                                                    });
                                                                                }
                                                                            }),
                                                                            el("dt", {
                                                                                get children() {
                                                                                    return "Members";
                                                                                }
                                                                            }),
                                                                            el("dd", {
                                                                                get children() {
                                                                                    return PeopleEditor({
                                                                                        "kind": "member",
                                                                                        get "project"() {
                                                                                            return p();
                                                                                        },
                                                                                        get "assigned"() {
                                                                                            return b().members;
                                                                                        },
                                                                                        get "houseMembers"() {
                                                                                            return b().houseMembers;
                                                                                        },
                                                                                        get "canEdit"() {
                                                                                            return isAdmin();
                                                                                        },
                                                                                        "onChanged": async () => { await refetch(); }
                                                                                    });
                                                                                }
                                                                            }),
                                                                            el("dt", {
                                                                                get children() {
                                                                                    return "Started";
                                                                                }
                                                                            }),
                                                                            el("dd", {
                                                                                get children() {
                                                                                    return live(() => new Date(p().createdAt).toLocaleDateString(undefined, { month: "long", day: "numeric", year: "numeric" }));
                                                                                }
                                                                            }),
                                                                            el("dt", {
                                                                                get children() {
                                                                                    return "Tasks";
                                                                                }
                                                                            }),
                                                                            el("dd", {
                                                                                get children() {
                                                                                    return [
                                                                                        live(() => doneCount()),
                                                                                        " of ",
                                                                                        live(() => taskFeed.total()),
                                                                                        " done"
                                                                                    ];
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
                                            ProjectTasksSection({
                                                get "project"() {
                                                    return p();
                                                },
                                                get "feed"() {
                                                    return taskFeed;
                                                },
                                                get "houseMembers"() {
                                                    return b().houseMembers;
                                                },
                                                get "canEdit"() {
                                                    return hasRole("admin", "member") || isAdmin();
                                                },
                                                get "onChanged"() {
                                                    return refreshTasks;
                                                }
                                            }),
                                            ProjectDeadlines({
                                                get projectId() { return p().projectId; },
                                                get revision() { return deadlineRevision(); },
                                                get category() { return p().category; },
                                            }),
                                            MilestoneList({
                                                get "project"() {
                                                    return p();
                                                },
                                                get "milestones"() {
                                                    return b().milestones;
                                                },
                                                get "canEdit"() {
                                                    return isAdmin();
                                                },
                                                "onChanged": async () => { await refetch(); }
                                            }),
                                            el("section", {
                                                "style": "margin-top:28px",
                                                get children() {
                                                    return [
                                                        el("h3", {
                                                            "style": "margin:0 0 8px;font-family:var(--display);font-size:20px;color:var(--grass-4)",
                                                            get children() {
                                                                return "Dependencies";
                                                            }
                                                        }),
                                                        DependenciesSection({
                                                            "nodeType": "project",
                                                            get "nodeId"() {
                                                                return p().projectId;
                                                            },
                                                            get "houseId"() {
                                                                return p().houseId;
                                                            }
                                                        })
                                                    ];
                                                }
                                            }),
                                            el("section", {
                                                "style": "margin-top:28px",
                                                get children() {
                                                    return [
                                                        el("h3", {
                                                            "style": "margin:0 0 8px;font-family:var(--display);font-size:20px;color:var(--grass-4)",
                                                            get children() {
                                                                return "Discussion";
                                                            }
                                                        }),
                                                        CommentsSection({
                                                            "targetType": "project",
                                                            get "targetId"() {
                                                                return p().projectId;
                                                            },
                                                            get "houseId"() {
                                                                return p().houseId;
                                                            },
                                                            get "members"() {
                                                                return b().houseMembers;
                                                            }
                                                        })
                                                    ];
                                                }
                                            })
                                        ];
                                    }
                                })
                            ]);
                        }
                    });
                }
            });
        }
    }));
};
// ─── Project header edit ──────────────────────────────────────────────
const ProjectEdit = (props: {
    project: Project;
    onCancel: () => void;
    onSaved: () => Promise<void> | void;
}) => {
    const [name, setName] = cell(props.project.name);
    const [category, setCategory] = cell(props.project.category ?? "");
    const [description, setDesc] = cell(props.project.description ?? "");
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        if (busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            await projectClient.updateProject({
                ...props.project,
                name: name().trim() || props.project.name,
                category: category().trim() || undefined,
                description: description().trim() || undefined,
            } as any);
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
        "class": "lh-form-grid",
        "style": "display:grid;grid-template-columns:2fr 1fr;gap:10px;margin-top:8px",
        get children() {
            return [
                el("label", {
                    "style": "grid-column:1/-1;display:flex;flex-direction:column;gap:4px",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:12px;color:var(--ink-mute)",
                                get children() {
                                    return "Name";
                                }
                            }),
                            el("input", {
                                "type": "text",
                                get "value"() {
                                    return name();
                                },
                                "onInput": (e) => setName(e.currentTarget.value),
                                "autofocus": true,
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
                                    return "Category";
                                }
                            }),
                            el("input", {
                                "type": "text",
                                get "value"() {
                                    return category();
                                },
                                "onInput": (e) => setCategory(e.currentTarget.value),
                                "style": "padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
                            })
                        ];
                    }
                }),
                el("label", {
                    "style": "grid-column:1/-1;display:flex;flex-direction:column;gap:4px",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:12px;color:var(--ink-mute)",
                                get children() {
                                    return "Description";
                                }
                            }),
                            el("textarea", {
                                get "value"() {
                                    return description();
                                },
                                "onInput": (e) => setDesc(e.currentTarget.value),
                                "rows": "3",
                                "style": "padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;resize:vertical"
                            })
                        ];
                    }
                }),
                When({
                    get "when"() {
                        return err();
                    },
                    children: (m) => el("p", {
                        "style": "color:var(--rust);grid-column:1/-1;margin:0",
                        get children() {
                            return live(() => m());
                        }
                    })
                }),
                el("div", {
                    "style": "grid-column:1/-1;display:flex;justify-content:flex-end;gap:10px",
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
                                    return busy();
                                },
                                "type": "submit",
                                get children() {
                                    return live(() => busy() ? "Saving…" : "Save");
                                }
                            })
                        ];
                    }
                })
            ];
        }
    }));
};
// ─── Members / Owners picker ──────────────────────────────────────────
const PeopleEditor = (props: {
    kind: "member" | "owner";
    project: Project;
    assigned: Member[];
    houseMembers: Member[];
    canEdit: boolean;
    onChanged: () => Promise<unknown>;
}) => {
    const [selected, setSelected] = cell("");
    const [busy, setBusy] = cell(false);
    const available = computed(() => {
        const taken = new Set(props.assigned.map((m) => m.memberId));
        return props.houseMembers.filter((m) => !taken.has(m.memberId));
    });
    const add = async () => {
        const mid = selected();
        if (!mid || busy())
            return;
        setBusy(true);
        try {
            if (props.kind === "owner") {
                await projectClient.addProjectOwner({ projectId: props.project.projectId, memberId: mid });
            }
            else {
                await projectClient.addProjectMember({ projectId: props.project.projectId, memberId: mid });
            }
            setSelected("");
            await props.onChanged();
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
        finally {
            setBusy(false);
        }
    };
    const remove = async (m: Member) => {
        if (busy())
            return;
        setBusy(true);
        try {
            if (props.kind === "owner") {
                await projectClient.removeProjectOwner({ projectId: props.project.projectId, memberId: m.memberId });
            }
            else {
                await projectClient.removeProjectMember({ projectId: props.project.projectId, memberId: m.memberId });
            }
            await props.onChanged();
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
        finally {
            setBusy(false);
        }
    };
    return (el("div", {
        "style": "display:flex;flex-direction:column;gap:8px",
        get children() {
            return [
                When({
                    get "when"() {
                        return props.assigned.length > 0;
                    },
                    get "fallback"() {
                        return el("span", {
                            "style": "color:var(--ink-mute);font-size:13px",
                            get children() {
                                return "None yet.";
                            }
                        });
                    },
                    get children() {
                        return [
                            When({
                                get "when"() {
                                    return props.canEdit;
                                },
                                get "fallback"() {
                                    return When({
                                        get "when"() {
                                            return props.kind === "owner";
                                        },
                                        get children() {
                                            return el("span", {
                                                get children() {
                                                    return live(() => props.assigned.map(displayName).join(", "));
                                                }
                                            });
                                        }
                                    });
                                },
                                get children() {
                                    return el("div", {
                                        "style": "display:flex;flex-wrap:wrap;gap:8px",
                                        get children() {
                                            return Repeat({
                                                get "each"() {
                                                    return props.assigned;
                                                },
                                                children: (m) => (el("span", {
                                                    "style": "display:inline-flex;align-items:center;gap:6px;padding:4px 8px 4px 4px;background:color-mix(in oklab, var(--paper) 90%, var(--ocean-1) 10%);border:1px solid var(--line);border-radius:9999px",
                                                    get children() {
                                                        return [
                                                            el("span", {
                                                                get "class"() {
                                                                    return `a sm ${memberSwatch(m.memberId)}`;
                                                                },
                                                                get children() {
                                                                    return live(() => initial(m));
                                                                }
                                                            }),
                                                            el("span", {
                                                                "style": "font-size:13px",
                                                                get children() {
                                                                    return live(() => displayName(m));
                                                                }
                                                            }),
                                                            el("button", {
                                                                "type": "button",
                                                                "onClick": () => remove(m),
                                                                get "disabled"() {
                                                                    return busy();
                                                                },
                                                                "title": "Remove",
                                                                "style": "background:transparent;border:0;cursor:pointer;color:var(--ink-mute);font-size:14px;line-height:1;padding:0 2px",
                                                                get children() {
                                                                    return "\u00D7";
                                                                }
                                                            })
                                                        ];
                                                    }
                                                }))
                                            });
                                        }
                                    });
                                }
                            }),
                            When({
                                get "when"() {
                                    return !props.canEdit && props.kind === "member";
                                },
                                get children() {
                                    return AvatarStack({
                                        get "bits"() {
                                            return props.assigned.map(toAvatar);
                                        }
                                    });
                                }
                            })
                        ];
                    }
                }),
                When({
                    get "when"() {
                        return props.canEdit && available().length > 0;
                    },
                    get children() {
                        return el("div", {
                            "style": "display:flex;gap:6px;align-items:center",
                            get children() {
                                return [
                                    el("select", {
                                        get "value"() {
                                            return selected();
                                        },
                                        "onChange": (e) => setSelected(e.currentTarget.value),
                                        "style": "flex:1;padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px",
                                        get children() {
                                            return [
                                                el("option", {
                                                    "value": "",
                                                    get children() {
                                                        return [
                                                            "Add a ",
                                                            live(() => props.kind),
                                                            "\u2026"
                                                        ];
                                                    }
                                                }),
                                                Repeat({
                                                    get "each"() {
                                                        return available();
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
                                    }),
                                    el("button", {
                                        "type": "button",
                                        "class": "btn-quiet",
                                        get "onClick"() {
                                            return add;
                                        },
                                        get "disabled"() {
                                            return busy() || !selected();
                                        },
                                        get children() {
                                            return "Add";
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
// ─── Project tasks ────────────────────────────────────────────────────
const ProjectTasksSection = (props: {
    project: Project;
    feed: TaskFeed;
    houseMembers: Member[];
    canEdit: boolean;
    onChanged: () => Promise<unknown>;
}) => {
    const [composerOpen, setComposerOpen] = cell(false);
    const [pickerOpen, setPickerOpen] = cell(false);
    const [attachId, setAttachId] = cell("");
    const [busy, setBusy] = cell(false);
    const [attachErr, setAttachErr] = cell<string | null>(null);
    // New tasks go after every row the project holds, readable or not.
    const nextPosition = () => props.feed.total();
    // Open house tasks not yet in this project — what the "Attach existing"
    // picker offers. It needs both full lists, so it loads them only when
    // someone opens it.
    const [attachable] = query(() => (pickerOpen() ? props.project : null), async (p) => {
        const [houseOpen, inProject] = await Promise.all([
            listAllTasks((offset, limit) => taskClient.listTasks({ houseId: p.houseId, state: "open", offset, limit })),
            listAllTasks((offset, limit) => projectClient.listProjectTasks({ houseId: p.houseId, projectId: p.projectId, offset, limit })),
        ]);
        const attached = new Set(inProject.map((t) => t.taskId));
        return houseOpen.filter((t) => !attached.has(t.taskId) && !t.deletedAt);
    });
    const memberById = computed(() => {
        const map = new Map<string, Member>();
        for (const m of props.houseMembers)
            map.set(m.memberId, m);
        return map;
    });
    const attach = async () => {
        const id = attachId();
        if (!id || busy())
            return;
        setBusy(true);
        setAttachErr(null);
        try {
            await projectClient.addProjectTask({
                projectId: props.project.projectId,
                taskId: id,
                position: nextPosition(),
            });
            setAttachId("");
            setPickerOpen(false);
            await props.onChanged();
        }
        catch (e) {
            setAttachErr(e instanceof Error ? e.message : String(e));
        }
        finally {
            setBusy(false);
        }
    };
    return (el("div", {
        "class": "section-padded",
        "style": "margin-top:24px;margin-bottom:18px",
        get children() {
            return [
                el("div", {
                    "style": "display:flex;justify-content:space-between;align-items:center;gap:8px;margin-bottom:10px",
                    get children() {
                        return [
                            el("h3", {
                                "style": "margin:0;font-family:var(--display);font-size:20px;color:var(--grass-4)",
                                get children() {
                                    return "Tasks";
                                }
                            }),
                            When({
                                get "when"() {
                                    return props.canEdit;
                                },
                                get children() {
                                    return el("span", {
                                        "style": "display:flex;gap:4px",
                                        get children() {
                                            return [
                                                el("button", {
                                                    "class": "btn-quiet",
                                                    "onClick": () => setPickerOpen((v) => !v),
                                                    get children() {
                                                        return live(() => pickerOpen() ? "Cancel" : "Attach existing…");
                                                    }
                                                }),
                                                el("button", {
                                                    "class": "btn-quiet",
                                                    "onClick": () => setComposerOpen((v) => !v),
                                                    get children() {
                                                        return live(() => composerOpen() ? "Cancel" : "+ New task");
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
                When({
                    get "when"() {
                        return composerOpen();
                    },
                    get children() {
                        return ProjectTaskComposer({
                            get "project"() {
                                return props.project;
                            },
                            get "nextPosition"() {
                                return nextPosition();
                            },
                            "onCancel": () => setComposerOpen(false),
                            "onCreated": async () => { setComposerOpen(false); await props.onChanged(); }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return pickerOpen();
                    },
                    get children() {
                        return el("div", {
                            "style": "display:flex;flex-wrap:wrap;gap:8px;align-items:center;margin-bottom:12px",
                            get children() {
                                return [
                                    When({
                                        get "when"() {
                                            return attachable.state === "ready";
                                        },
                                        get "fallback"() {
                                            return el("span", {
                                                "style": "font-size:12px;color:var(--ink-mute)",
                                                get children() {
                                                    return live(() => attachable.error ? "Could not load tasks." : "Loading tasks…");
                                                }
                                            });
                                        },
                                        get children() {
                                            return When({
                                                get "when"() {
                                                    return (attachable() ?? []).length > 0;
                                                },
                                                get "fallback"() {
                                                    return el("span", {
                                                        "style": "font-size:12px;color:var(--ink-mute)",
                                                        get children() {
                                                            return "Every open task is already in this project.";
                                                        }
                                                    });
                                                },
                                                get children() {
                                                    return [
                                                        el("select", {
                                                            get "value"() {
                                                                return attachId();
                                                            },
                                                            "onChange": (e) => setAttachId(e.currentTarget.value),
                                                            get "disabled"() {
                                                                return busy();
                                                            },
                                                            "aria-label": "Task to attach",
                                                            "style": "flex:1;min-width:0;padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);color:var(--ink);font-size:13px",
                                                            get children() {
                                                                return [
                                                                    el("option", {
                                                                        "value": "",
                                                                        get children() {
                                                                            return "\u2014 pick a task \u2014";
                                                                        }
                                                                    }),
                                                                    Repeat({
                                                                        get "each"() {
                                                                            return attachable();
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
                                                        }),
                                                        el("button", {
                                                            "type": "button",
                                                            "class": "btn-quiet",
                                                            get "onClick"() {
                                                                return attach;
                                                            },
                                                            get "disabled"() {
                                                                return busy() || !attachId();
                                                            },
                                                            get children() {
                                                                return "Attach";
                                                            }
                                                        })
                                                    ];
                                                }
                                            });
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return attachErr();
                                        },
                                        children: (m) => el("span", {
                                            "style": "color:var(--rust);font-size:12px;flex-basis:100%",
                                            get children() {
                                                return live(() => m());
                                            }
                                        })
                                    })
                                ];
                            }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return !props.feed.ready() || props.feed.total() > 0;
                    },
                    get "fallback"() {
                        return el("p", {
                            "style": "color:var(--ink-mute);font-size:13px;margin:0",
                            get children() {
                                return When({
                                    get "when"() {
                                        return props.canEdit;
                                    },
                                    "fallback": "No tasks attached to this project yet.",
                                    get children() {
                                        return "No tasks attached yet \u2014 create one, or attach an existing house task.";
                                    }
                                });
                            }
                        });
                    },
                    get children() {
                        return el("div", {
                            "class": "tasks",
                            "style": "border:1px solid var(--line);border-radius:var(--r-md)",
                            get children() {
                                return TaskFeedList({
                                    get "feed"() {
                                        return props.feed;
                                    },
                                    "renderTask": (t) => (ProjectTaskRow({
                                        get "task"() {
                                            return t;
                                        },
                                        get "project"() {
                                            return props.project;
                                        },
                                        get "byId"() {
                                            return memberById();
                                        },
                                        get "members"() {
                                            return props.houseMembers;
                                        },
                                        get "canEdit"() {
                                            return props.canEdit;
                                        },
                                        get "nextPosition"() {
                                            return nextPosition();
                                        },
                                        get "onChanged"() {
                                            return props.onChanged;
                                        }
                                    }))
                                });
                            }
                        });
                    }
                })
            ];
        }
    }));
};
const ProjectTaskComposer = (props: {
    project: Project;
    nextPosition: number;
    onCancel: () => void;
    onCreated: () => Promise<void> | void;
}) => {
    const [title, setTitle] = cell("");
    const [tag, setTag] = cell("");
    const [due, setDue] = cell("");
    const [estimate, setEstimate] = cell("");
    const [recFreq, setRecFreq] = cell<RecurrenceFreq>("");
    const [recInterval, setRecInterval] = cell(1);
    const [recNextAt, setRecNextAt] = cell("");
    const [recByWeekday, setRecByWeekday] = cell<number[]>([]);
    const [recBySetpos, setRecBySetpos] = cell(1);
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        if (!title().trim() || busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            // Create the task. `assignees: []` is explicit-empty so the server
            // skips its default-to-caller rule (the project should hand the
            // work out later, not auto-pin it to whoever happened to click +).
            const dueIso = due() || undefined;
            const created = await taskClient.createTask(normalizeTaskCreate({
                houseId: props.project.houseId,
                ownerMemberId: "",
                title: title().trim(),
                tag: tag().trim() || undefined,
                dueAt: dueIso,
                estimateMinutes: estimate() ? Number(estimate()) : undefined,
                assignees: [],
                ...toRecurrence(recFreq(), recInterval(), recNextAt(), recByWeekday(), recBySetpos(), dueIso),
            } as any));
            // Then attach it to the project at the end of the list.
            await projectClient.addProjectTask({
                projectId: props.project.projectId,
                taskId: created.taskId,
                position: props.nextPosition,
            });
            transaction(() => {
                setTitle("");
                setTag("");
                setDue("");
                setEstimate("");
                setRecFreq("");
                setRecInterval(1);
                setRecNextAt("");
                setRecByWeekday([]);
                setRecBySetpos(1);
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
        "class": "lh-form-grid",
        "style": "margin:0 0 12px;padding:14px 16px;background:color-mix(in oklab, var(--paper) 92%, var(--ocean-1) 8%);border:1px solid var(--line);border-radius:var(--r-md);display:grid;grid-template-columns:2fr 1fr auto 90px auto auto;gap:10px;align-items:end",
        get children() {
            return [
                el("label", {
                    "style": "display:flex;flex-direction:column;gap:4px;grid-column:1/-1",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
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
                                "style": "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
                            })
                        ];
                    }
                }),
                el("label", {
                    "style": "display:flex;flex-direction:column;gap:4px",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
                                get children() {
                                    return "Tag";
                                }
                            }),
                            el("input", {
                                "type": "text",
                                get "value"() {
                                    return tag();
                                },
                                "onInput": (e) => setTag(e.currentTarget.value),
                                "placeholder": "house, barn, \u2026",
                                "style": "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
                            })
                        ];
                    }
                }),
                el("label", {
                    "style": "display:flex;flex-direction:column;gap:4px",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
                                get children() {
                                    return "Due";
                                }
                            }),
                            DateTimePicker({
                                get "value"() {
                                    return due();
                                },
                                get "onChange"() {
                                    return setDue;
                                },
                                "title": "Due (optional)"
                            })
                        ];
                    }
                }),
                el("label", {
                    "style": "display:flex;flex-direction:column;gap:4px",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
                                get children() {
                                    return "Est (min)";
                                }
                            }),
                            el("input", {
                                "type": "number",
                                "min": "0",
                                get "value"() {
                                    return estimate();
                                },
                                "onInput": (e) => setEstimate(e.currentTarget.value),
                                "style": "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
                            })
                        ];
                    }
                }),
                el("button", {
                    "class": "btn btn-primary",
                    get "disabled"() {
                        return busy() || !title().trim();
                    },
                    "type": "submit",
                    get children() {
                        return live(() => busy() ? "…" : "Add");
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
                    get children() {
                        return "Cancel";
                    }
                }),
                el("div", {
                    "style": "grid-column:1/-1",
                    get children() {
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
                            },
                            "compact": true
                        });
                    }
                }),
                When({
                    get "when"() {
                        return err();
                    },
                    children: (m) => el("p", {
                        "style": "color:var(--rust);grid-column:1/-1;margin:0;font-size:12px",
                        get children() {
                            return live(() => m());
                        }
                    })
                })
            ];
        }
    }));
};
const ProjectTaskRow = (props: {
    task: Task;
    project: Project;
    byId: Map<string, Member>;
    members: Member[];
    canEdit: boolean;
    nextPosition: number;
    onChanged: () => Promise<unknown>;
}) => {
    const [busy, setBusy] = cell(false);
    const [showAssignees, setShowAssignees] = cell(false);
    const [showSubtask, setShowSubtask] = cell(false);
    const [showDetail, setShowDetail] = cell(false);
    const closed = () => isTaskClosed(props.task);
    const toggle = async () => {
        if (busy())
            return;
        setBusy(true);
        try {
            await taskClient.updateTask({
                ...props.task,
                status: closed() ? "open" : "done",
            } as any);
            await props.onChanged();
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
        finally {
            setBusy(false);
        }
    };
    const detach = async () => {
        if (busy())
            return;
        if (!confirm(`Remove "${props.task.title}" from this project? The task itself isn't deleted.`))
            return;
        setBusy(true);
        try {
            await projectClient.removeProjectTask({
                projectId: props.project.projectId,
                taskId: props.task.taskId,
            });
            await props.onChanged();
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
        finally {
            setBusy(false);
        }
    };
    const saveAssignees = async (next: string[]) => {
        if (busy())
            return;
        setBusy(true);
        try {
            await taskClient.updateTask({
                ...props.task,
                assignees: next,
            } as any);
            await props.onChanged();
            setShowAssignees(false);
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
        finally {
            setBusy(false);
        }
    };
    const assignees = () => (props.task.assignees ?? [])
        .map((mid) => props.byId.get(mid))
        .filter((m): m is Member => Boolean(m));
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
                            When({
                                get "when"() {
                                    return props.canEdit;
                                },
                                get "fallback"() {
                                    return el("div", {
                                        "class": "check",
                                        get children() {
                                            return Check();
                                        }
                                    });
                                },
                                get children() {
                                    return el("button", {
                                        "class": "check",
                                        get "title"() {
                                            return closed() ? "Mark as open" : "Mark as done";
                                        },
                                        "onClick": (ev) => { ev.stopPropagation(); toggle(); },
                                        "style": "cursor:pointer",
                                        get children() {
                                            return Check();
                                        }
                                    });
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
                                                                return live(() => capWord(tag()!));
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
                                    return When({
                                        get "when"() {
                                            return props.canEdit;
                                        },
                                        get "fallback"() {
                                            return When({
                                                get "when"() {
                                                    return assignees().length > 0;
                                                },
                                                get children() {
                                                    return AvatarStack({
                                                        get "bits"() {
                                                            return assignees().map(toAvatar);
                                                        }
                                                    });
                                                }
                                            });
                                        },
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
                                                                return el("span", {
                                                                    "style": "font-size:11px;color:var(--ink-mute);padding:0 6px",
                                                                    get children() {
                                                                        return "+ assign";
                                                                    }
                                                                });
                                                            },
                                                            get children() {
                                                                return AvatarStack({
                                                                    get "bits"() {
                                                                        return assignees().map(toAvatar);
                                                                    }
                                                                });
                                                            }
                                                        });
                                                    }
                                                }),
                                                el("button", {
                                                    "type": "button",
                                                    "onClick": () => setShowSubtask((v) => !v),
                                                    "title": "Add a subtask (also attached to this project)",
                                                    "style": "background:transparent;border:1px dashed var(--line);color:var(--ink-mute);font-size:11px;padding:2px 8px;border-radius:9999px;cursor:pointer",
                                                    get children() {
                                                        return "+ subtask";
                                                    }
                                                }),
                                                el("button", {
                                                    "type": "button",
                                                    get "onClick"() {
                                                        return detach;
                                                    },
                                                    "title": "Remove from this project (task itself is kept)",
                                                    "style": "background:transparent;border:0;color:var(--ink-mute);font-size:18px;line-height:1;padding:4px 8px;cursor:pointer",
                                                    get children() {
                                                        return "\u00D7";
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
                When({
                    get "when"() {
                        return showAssignees();
                    },
                    get children() {
                        return ProjectAssigneeEditor({
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
                        return ProjectSubtaskComposer({
                            get "parent"() {
                                return props.task;
                            },
                            get "project"() {
                                return props.project;
                            },
                            get "nextPosition"() {
                                return props.nextPosition;
                            },
                            get "parentAssignees"() {
                                return (props.task.assignees ?? []).slice();
                            },
                            get "members"() {
                                return props.members;
                            },
                            "onCancel": () => setShowSubtask(false),
                            "onCreated": async () => { setShowSubtask(false); await props.onChanged(); }
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
                            "onSaved": async () => { await props.onChanged(); },
                            "onDelete": async () => {
                                // From the project view, "delete" detaches + soft-deletes —
                                // the row would otherwise stay linked to a soft-deleted task.
                                await projectClient.removeProjectTask({
                                    projectId: props.project.projectId,
                                    taskId: props.task.taskId,
                                });
                                await taskClient.deleteTask(props.task.taskId);
                                await props.onChanged();
                            }
                        });
                    }
                })
            ];
        }
    }));
};
const capWord = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);
// ─── Project task inline editors ──────────────────────────────────────
const ProjectAssigneeEditor = (props: {
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
const ProjectSubtaskComposer = (props: {
    parent: Task;
    project: Project;
    nextPosition: number;
    parentAssignees: string[];
    members: Member[];
    onCancel: () => void;
    onCreated: () => Promise<void> | void;
}) => {
    const [title, setTitle] = cell("");
    const [tag, setTag] = cell("");
    const [due, setDue] = cell("");
    const [estimate, setEstimate] = cell("");
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
            // Also attach the new subtask to the same project so it shows up
            // here. The api keeps subtasks and project-tasks as independent
            // relations; from the user's POV "+ subtask on a project task"
            // means "in this project," so we wire it explicitly.
            await projectClient.addProjectTask({
                projectId: props.project.projectId,
                taskId: created.taskId,
                position: props.nextPosition,
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
// ─── Milestones ───────────────────────────────────────────────────────
const MilestoneList = (props: {
    project: Project;
    milestones: Milestone[];
    canEdit: boolean;
    onChanged: () => Promise<unknown>;
}) => {
    const [adding, setAdding] = cell(false);
    // The wrapper inherits .room-detail's `overflow:hidden + border-radius`,
    // so children with no horizontal padding visually meet the rounded
    // corners. Match .room-meta's 32px gutters (22px under 820px) so the
    // section's heading, add button, and empty state all sit inside the
    // rounded shape instead of getting clipped on either end.
    return (el("div", {
        "class": "section-padded",
        "style": "margin-top:4px;padding-bottom:24px",
        get children() {
            return [
                el("div", {
                    "style": "display:flex;justify-content:space-between;align-items:center;margin-bottom:10px",
                    get children() {
                        return [
                            el("h3", {
                                "style": "margin:0;font-family:var(--display);font-size:20px;color:var(--grass-4)",
                                get children() {
                                    return "Milestones";
                                }
                            }),
                            When({
                                get "when"() {
                                    return props.canEdit;
                                },
                                get children() {
                                    return el("button", {
                                        "class": "btn-quiet",
                                        "onClick": () => setAdding((v) => !v),
                                        get children() {
                                            return live(() => adding() ? "Cancel" : "+ Add milestone");
                                        }
                                    });
                                }
                            })
                        ];
                    }
                }),
                When({
                    get "when"() {
                        return adding();
                    },
                    get children() {
                        return MilestoneForm({
                            get "project"() {
                                return props.project;
                            },
                            "onCancel": () => setAdding(false),
                            "onSaved": async () => { setAdding(false); await props.onChanged(); }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return props.milestones.length > 0;
                    },
                    get "fallback"() {
                        return el("p", {
                            "style": "color:var(--ink-mute);font-size:13px;margin:0",
                            get children() {
                                return When({
                                    get "when"() {
                                        return props.canEdit;
                                    },
                                    "fallback": "No milestones yet.",
                                    get children() {
                                        return "No milestones yet \u2014 add the first to start the timeline.";
                                    }
                                });
                            }
                        });
                    },
                    get children() {
                        return el("div", {
                            "class": "ribbon",
                            get children() {
                                return Repeat({
                                    get "each"() {
                                        return props.milestones;
                                    },
                                    children: (m) => (MilestoneItem({
                                        get "milestone"() {
                                            return m;
                                        },
                                        get "canEdit"() {
                                            return props.canEdit;
                                        },
                                        get "onChanged"() {
                                            return props.onChanged;
                                        }
                                    }))
                                });
                            }
                        });
                    }
                })
            ];
        }
    }));
};
const MilestoneItem = (props: {
    milestone: Milestone;
    canEdit: boolean;
    onChanged: () => Promise<unknown>;
}) => {
    const [editing, setEditing] = cell(false);
    const remove = async () => {
        if (!confirm(`Delete milestone "${props.milestone.label}"?`))
            return;
        try {
            await projectClient.deleteMilestone(props.milestone.milestoneId);
            await props.onChanged();
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
    };
    return (When({
        get "when"() {
            return !editing();
        },
        get "fallback"() {
            return MilestoneForm({
                get "project"() {
                    return { projectId: props.milestone.projectId } as Project;
                },
                get "existing"() {
                    return props.milestone;
                },
                "onCancel": () => setEditing(false),
                "onSaved": async () => { setEditing(false); await props.onChanged(); }
            });
        },
        get children() {
            return el("div", {
                get "class"() {
                    return `rib ${props.milestone.state}`;
                },
                "style": "position:relative",
                get children() {
                    return [
                        el("span", {
                            "class": "when",
                            get children() {
                                return live(() => props.milestone.whenLabel);
                            }
                        }),
                        el("span", {
                            "class": "what",
                            get children() {
                                return live(() => props.milestone.label);
                            }
                        }),
                        When({
                            get "when"() {
                                return props.canEdit;
                            },
                            get children() {
                                return el("div", {
                                    "style": "display:flex;gap:6px;margin-top:6px",
                                    get children() {
                                        return [
                                            el("button", {
                                                "class": "btn-quiet",
                                                "style": "font-size:11px;padding:2px 8px",
                                                "onClick": () => setEditing(true),
                                                get children() {
                                                    return "edit";
                                                }
                                            }),
                                            el("button", {
                                                "class": "btn-quiet",
                                                "style": "font-size:11px;padding:2px 8px;color:var(--rust)",
                                                get "onClick"() {
                                                    return remove;
                                                },
                                                get children() {
                                                    return "delete";
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
    }));
};
const MilestoneForm = (props: {
    project: Project;
    existing?: Milestone;
    onCancel: () => void;
    onSaved: () => Promise<void> | void;
}) => {
    const [label, setLabel] = cell(props.existing?.label ?? "");
    const [whenLabel, setWhenLabel] = cell(props.existing?.whenLabel ?? "");
    const [state, setState] = cell((props.existing?.state as string) ?? "future");
    const [position, setPosition] = cell(props.existing?.position ?? 0);
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        if (!label().trim() || busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            if (props.existing) {
                await projectClient.updateMilestone({
                    ...props.existing,
                    label: label().trim(),
                    whenLabel: whenLabel().trim(),
                    state: state() as any,
                    position: position(),
                });
            }
            else {
                await projectClient.createMilestone({
                    milestoneId: "",
                    projectId: props.project.projectId,
                    label: label().trim(),
                    whenLabel: whenLabel().trim(),
                    state: state() as any,
                    position: position(),
                    createdAt: "",
                    updatedAt: "",
                });
            }
            transaction(() => { setLabel(""); setWhenLabel(""); });
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
        "class": "lh-form-grid",
        "style": "margin:0 0 12px;padding:14px 16px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);display:grid;grid-template-columns:2fr 1fr 1fr 80px auto;gap:10px;align-items:end",
        get children() {
            return [
                el("label", {
                    "style": "display:flex;flex-direction:column;gap:4px",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
                                get children() {
                                    return "Label";
                                }
                            }),
                            el("input", {
                                "type": "text",
                                get "value"() {
                                    return label();
                                },
                                "onInput": (e) => setLabel(e.currentTarget.value),
                                "required": true,
                                "autofocus": true,
                                "style": "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
                            })
                        ];
                    }
                }),
                el("label", {
                    "style": "display:flex;flex-direction:column;gap:4px",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
                                get children() {
                                    return "When";
                                }
                            }),
                            el("input", {
                                "type": "text",
                                get "value"() {
                                    return whenLabel();
                                },
                                "onInput": (e) => setWhenLabel(e.currentTarget.value),
                                "placeholder": "May \u00B7 current",
                                "style": "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
                            })
                        ];
                    }
                }),
                el("label", {
                    "style": "display:flex;flex-direction:column;gap:4px",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
                                get children() {
                                    return "State";
                                }
                            }),
                            el("select", {
                                get "value"() {
                                    return state();
                                },
                                "onChange": (e) => setState(e.currentTarget.value),
                                "style": "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px",
                                get children() {
                                    return Repeat({
                                        get "each"() {
                                            return MILESTONE_STATES;
                                        },
                                        children: (s) => el("option", {
                                            get "value"() {
                                                return s;
                                            },
                                            get children() {
                                                return live(() => s);
                                            }
                                        })
                                    });
                                }
                            })
                        ];
                    }
                }),
                el("label", {
                    "style": "display:flex;flex-direction:column;gap:4px",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
                                get children() {
                                    return "Order";
                                }
                            }),
                            el("input", {
                                "type": "number",
                                get "value"() {
                                    return position();
                                },
                                "min": "0",
                                "onInput": (e) => setPosition(Number(e.currentTarget.value)),
                                "style": "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px;width:100%"
                            })
                        ];
                    }
                }),
                el("div", {
                    "style": "display:flex;gap:6px",
                    get children() {
                        return [
                            el("button", {
                                "class": "btn btn-primary",
                                get "disabled"() {
                                    return busy() || !label().trim();
                                },
                                "type": "submit",
                                get children() {
                                    return live(() => busy() ? "…" : props.existing ? "Save" : "Add");
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
                                get children() {
                                    return "Cancel";
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
                        "style": "color:var(--rust);grid-column:1/-1;margin:0",
                        get children() {
                            return live(() => m());
                        }
                    })
                })
            ];
        }
    }));
};
