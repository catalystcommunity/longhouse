import { observe, computed, query, cell } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { Link } from "~/ui/router";
import { AuthGate } from "~/components/AuthGate";
import { projectClient } from "~/data/clients";
import { PROJECT_LIST_LIMIT, effectiveProjectsFilter, loadMyProjectIds, readProjectsFilter, writeProjectsFilter, type ProjectsFilter, } from "~/lib/projects";
import { normalizeProjectCreate } from "~/lib/taskCreate";
import { currentMemberId, useCurrentHouseId } from "~/stores/auth";
import type { Project } from "@longhouse/client";
const SWATCHES = ["r1", "r2", "r3", "r4"] as const;
const swatchFor = (id: string) => {
    let h = 0;
    for (let i = 0; i < id.length; i++)
        h = (h * 31 + id.charCodeAt(i)) | 0;
    return SWATCHES[Math.abs(h) % SWATCHES.length];
};
export const ProjectsPage = () => {
    const houseId = useCurrentHouseId();
    const [projects, { refetch }] = query(() => houseId(), async (h) => (await projectClient.listProjects({ houseId: h, limit: PROJECT_LIST_LIMIT })).projects);
    // All / Mine filter. Mine = projects I created (created_by fallback owner)
    // or hold an explicit owner/member grant on — same rule as the dashboard
    // (lib/projects.ts). The choice is remembered per member in localStorage;
    // with nothing saved it defaults to Mine when I have any projects.
    const [myIds] = query(() => {
        const me = currentMemberId();
        const ps = projects.state === "ready" || projects.state === "refreshing" ? projects.latest : undefined;
        if (!me || !ps)
            return null;
        return { me, ps };
    }, async ({ me, ps }) => loadMyProjectIds(projectClient, ps, me));
    const [saved, setSaved] = cell<ProjectsFilter | undefined>();
    observe(() => {
        const me = currentMemberId();
        setSaved(me ? readProjectsFilter(me) : undefined);
    });
    const myIdsLoaded = () => (myIds.state === "ready" || myIds.state === "refreshing" ? myIds.latest : undefined);
    const filter = () => effectiveProjectsFilter(saved(), myIdsLoaded()?.size ?? 0);
    const choose = (v: ProjectsFilter) => {
        setSaved(v);
        const me = currentMemberId();
        if (me)
            writeProjectsFilter(me, v);
    };
    const shown = computed(() => {
        const ps = projects() ?? [];
        if (filter() === "all")
            return ps;
        const mine = myIdsLoaded() ?? new Set<string>();
        return ps.filter((p) => mine.has(p.projectId));
    });
    // Hold the list until "mine" is known, so the default doesn't flip from
    // All to Mine (or show an empty Mine list) mid-load. A failed grant
    // lookup degrades to the created_by rule inside loadMyProjectIds.
    const filterReady = () => !!myIdsLoaded() || myIds.state === "errored" || !currentMemberId();
    const [composerOpen, setComposerOpen] = cell(false);
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
                                        "Projects ",
                                        el("em", {
                                            get children() {
                                                return "in flight";
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("p", {
                                "class": "lead",
                                get children() {
                                    return "Longer pieces of work with their own members, tasks, and discussion.";
                                }
                            }),
                            el("div", {
                                "style": "margin-top:12px;display:flex;flex-wrap:wrap;gap:12px;align-items:center",
                                get children() {
                                    return [
                                        el("button", {
                                            "class": "btn btn-primary",
                                            "onClick": () => setComposerOpen((v) => !v),
                                            get children() {
                                                return live(() => composerOpen() ? "Cancel" : "New project");
                                            }
                                        }),
                                        el("div", {
                                            "class": "seg",
                                            "role": "group",
                                            "aria-label": "Show projects",
                                            get children() {
                                                return [
                                                    el("button", {
                                                        "type": "button",
                                                        get "class"() {
                                                            return filter() === "all" ? "on" : "";
                                                        },
                                                        get "aria-pressed"() {
                                                            return filter() === "all";
                                                        },
                                                        "onClick": () => choose("all"),
                                                        get children() {
                                                            return "All";
                                                        }
                                                    }),
                                                    el("button", {
                                                        "type": "button",
                                                        get "class"() {
                                                            return filter() === "mine" ? "on" : "";
                                                        },
                                                        get "aria-pressed"() {
                                                            return filter() === "mine";
                                                        },
                                                        "onClick": () => choose("mine"),
                                                        get children() {
                                                            return "Mine";
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
                        return composerOpen();
                    },
                    get children() {
                        return ProjectComposer({
                            get "houseId"() {
                                return houseId()!;
                            },
                            "onCancel": () => setComposerOpen(false),
                            "onCreated": async () => { setComposerOpen(false); await refetch(); }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return !projects.loading && filterReady();
                    },
                    get "fallback"() {
                        return el("p", {
                            "style": "padding:24px;color:var(--ink-mute)",
                            get children() {
                                return "Loading\u2026";
                            }
                        });
                    },
                    get children() {
                        return When({
                            get "when"() {
                                return (projects() ?? []).length > 0;
                            },
                            get "fallback"() {
                                return EmptyProjects();
                            },
                            get children() {
                                return When({
                                    get "when"() {
                                        return shown().length > 0;
                                    },
                                    get "fallback"() {
                                        return EmptyMine({
                                            "onShowAll": () => choose("all")
                                        });
                                    },
                                    get children() {
                                        return el("div", {
                                            "class": "rooms",
                                            get children() {
                                                return Repeat({
                                                    get "each"() {
                                                        return shown();
                                                    },
                                                    children: (p, i) => ProjectCard({
                                                        get "p"() {
                                                            return p;
                                                        },
                                                        get "delay"() {
                                                            return 2 + (i() % 4);
                                                        }
                                                    })
                                                });
                                            }
                                        });
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
const EmptyProjects = () => (el("section", {
    "style": "margin-top:24px;padding:32px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);text-align:center;color:var(--ink-mute)",
    get children() {
        return [
            el("p", {
                "style": "margin:0 0 4px",
                get children() {
                    return el("b", {
                        get children() {
                            return "No projects yet.";
                        }
                    });
                }
            }),
            el("p", {
                "style": "margin:0",
                get children() {
                    return "Create a project to gather related tasks, members and milestones in one place.";
                }
            })
        ];
    }
}));
const EmptyMine = (props: {
    onShowAll: () => void;
}) => (el("section", {
    "style": "margin-top:24px;padding:32px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);text-align:center;color:var(--ink-mute)",
    get children() {
        return [
            el("p", {
                "style": "margin:0 0 4px",
                get children() {
                    return el("b", {
                        get children() {
                            return "You are not in a project yet.";
                        }
                    });
                }
            }),
            el("p", {
                "style": "margin:0",
                get children() {
                    return el("button", {
                        "type": "button",
                        "class": "btn-quiet",
                        get "onClick"() {
                            return props.onShowAll;
                        },
                        get children() {
                            return "Show all projects";
                        }
                    });
                }
            })
        ];
    }
}));
export const ProjectCard = (props: {
    p: Project;
    delay?: number;
}) => (Link({
    get "href"() {
        return `/projects/${props.p.projectId}`;
    },
    get "class"() {
        return `room ${swatchFor(props.p.projectId)} reveal ${props.delay ? `d${props.delay}` : ""}`;
    },
    get children() {
        return [
            When({
                get "when"() {
                    return props.p.personal;
                },
                get "fallback"() {
                    return When({
                        get "when"() {
                            return props.p.category;
                        },
                        children: (c) => el("span", {
                            "class": "corner",
                            get children() {
                                return live(() => c());
                            }
                        })
                    });
                },
                get children() {
                    return el("span", {
                        "class": "corner",
                        get children() {
                            return "Private";
                        }
                    });
                }
            }),
            el("div", {
                "class": "swatch",
                get children() {
                    return live(() => props.p.name.charAt(0));
                }
            }),
            el("h3", {
                get children() {
                    return live(() => props.p.name);
                }
            }),
            When({
                get "when"() {
                    return props.p.description;
                },
                children: (d) => el("p", {
                    get children() {
                        return live(() => d());
                    }
                })
            })
        ];
    }
}));
const ProjectComposer = (props: {
    houseId: string;
    onCancel: () => void;
    onCreated: () => Promise<void> | void;
}) => {
    const [name, setName] = cell("");
    const [category, setCategory] = cell("");
    const [description, setDesc] = cell("");
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        if (!name().trim() || busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            await projectClient.createProject(normalizeProjectCreate({
                houseId: props.houseId,
                name: name().trim(),
                description: description().trim() || undefined,
                category: category().trim() || undefined,
            }));
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
        "style": "margin:16px 0;padding:18px 20px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low);display:grid;grid-template-columns:2fr 1fr;gap:12px",
        get children() {
            return [
                el("label", {
                    "style": "display:flex;flex-direction:column;gap:4px;grid-column:1/-1",
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
                                "required": true,
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
                                    return "Category (optional)";
                                }
                            }),
                            el("input", {
                                "type": "text",
                                get "value"() {
                                    return category();
                                },
                                "onInput": (e) => setCategory(e.currentTarget.value),
                                "placeholder": "Operations, Events, \u2026",
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
                When({
                    get "when"() {
                        return err();
                    },
                    children: (m) => el("p", {
                        "style": "color:var(--rust);font-size:13px;grid-column:1/-1;margin:0",
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
                                    return busy() || !name().trim();
                                },
                                "type": "submit",
                                get children() {
                                    return live(() => busy() ? "Saving…" : "Create project");
                                }
                            })
                        ];
                    }
                })
            ];
        }
    }));
};
