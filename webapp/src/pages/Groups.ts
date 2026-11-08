import { computed, query, cell } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { AuthGate } from "~/components/AuthGate";
import { groupClient, memberClient, skillClient } from "~/data/clients";
import { displayName, initial, memberSwatch } from "~/lib/derive";
import { hasRole, useCurrentHouseId } from "~/stores/auth";
import type { Group, Member, Skill } from "@longhouse/client";
export const GroupsPage = () => {
    const houseId = useCurrentHouseId();
    const isAdmin = () => hasRole("admin");
    const [groups, { refetch }] = query(() => houseId(), async (h) => groupClient.listGroups({ houseId: h }));
    const [members] = query(() => houseId(), async (h) => memberClient.listMembers({ houseId: h }));
    const [skills] = query(() => houseId(), async (h) => skillClient.listSkills({ houseId: h }));
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
                                        "Groups ",
                                        el("em", {
                                            get children() {
                                                return "buckets of members";
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("p", {
                                "class": "lead",
                                get children() {
                                    return "Admin-curated subsets \u2014 \"House team\", \"Garden crew\", whatever serves the work.";
                                }
                            }),
                            When({
                                get "when"() {
                                    return isAdmin();
                                },
                                get children() {
                                    return el("div", {
                                        "style": "margin-top:12px",
                                        get children() {
                                            return el("button", {
                                                "class": "btn btn-primary",
                                                "onClick": () => setComposerOpen((v) => !v),
                                                get children() {
                                                    return live(() => composerOpen() ? "Cancel" : "New group");
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
                        return composerOpen();
                    },
                    get children() {
                        return GroupForm({
                            get "houseId"() {
                                return houseId()!;
                            },
                            "onCancel": () => setComposerOpen(false),
                            "onSaved": async () => { setComposerOpen(false); await refetch(); }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return !groups.loading;
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
                                return (groups() ?? []).length > 0;
                            },
                            get "fallback"() {
                                return el("section", {
                                    "style": "margin-top:24px;padding:32px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);text-align:center;color:var(--ink-mute)",
                                    get children() {
                                        return [
                                            el("p", {
                                                "style": "margin:0 0 4px",
                                                get children() {
                                                    return el("b", {
                                                        get children() {
                                                            return "No groups yet.";
                                                        }
                                                    });
                                                }
                                            }),
                                            el("p", {
                                                "style": "margin:0",
                                                get children() {
                                                    return "Create a group to bundle members for events, tasks, or skills.";
                                                }
                                            })
                                        ];
                                    }
                                });
                            },
                            get children() {
                                return el("div", {
                                    "style": "display:grid;grid-template-columns:1fr;gap:16px;margin-top:8px",
                                    get children() {
                                        return Repeat({
                                            get "each"() {
                                                return groups()!;
                                            },
                                            children: (g) => (GroupCard({
                                                get "group"() {
                                                    return g;
                                                },
                                                get "houseMembers"() {
                                                    return members() ?? [];
                                                },
                                                get "houseSkills"() {
                                                    return skills() ?? [];
                                                },
                                                get "canAdmin"() {
                                                    return isAdmin();
                                                },
                                                "onChanged": async () => { await refetch(); }
                                            }))
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
// ─── Group card ────────────────────────────────────────────────────────
const GroupCard = (props: {
    group: Group;
    houseMembers: Member[];
    houseSkills: Skill[];
    canAdmin: boolean;
    onChanged: () => Promise<unknown>;
}) => {
    const [members, { refetch: refetchMembers }] = query(() => props.group.groupId, async (gid) => groupClient.listGroupMembers({ houseId: "", memberId: gid }));
    const [skills, { refetch: refetchSkills }] = query(() => props.group.groupId, async (gid) => skillClient.listGroupSkills(gid));
    const [editing, setEditing] = cell(false);
    const remove = async () => {
        if (!confirm(`Delete group "${props.group.name}"?`))
            return;
        try {
            await groupClient.deleteGroup(props.group.groupId);
            await props.onChanged();
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
    };
    return (el("article", {
        "style": "padding:18px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)",
        get children() {
            return [
                When({
                    get "when"() {
                        return !editing();
                    },
                    get "fallback"() {
                        return GroupForm({
                            get "houseId"() {
                                return props.group.houseId;
                            },
                            get "existing"() {
                                return props.group;
                            },
                            "onCancel": () => setEditing(false),
                            "onSaved": async () => { setEditing(false); await props.onChanged(); }
                        });
                    },
                    get children() {
                        return el("div", {
                            "style": "display:flex;justify-content:space-between;align-items:start;gap:12px",
                            get children() {
                                return [
                                    el("div", {
                                        get children() {
                                            return [
                                                el("h3", {
                                                    "style": "margin:0 0 4px;font-family:var(--display);font-size:22px;color:var(--grass-4)",
                                                    get children() {
                                                        return live(() => props.group.name);
                                                    }
                                                }),
                                                When({
                                                    get "when"() {
                                                        return props.group.description;
                                                    },
                                                    children: (d) => el("p", {
                                                        "style": "margin:0;color:var(--ink-mute);font-size:14px",
                                                        get children() {
                                                            return live(() => d());
                                                        }
                                                    })
                                                })
                                            ];
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return props.canAdmin;
                                        },
                                        get children() {
                                            return el("div", {
                                                "style": "display:flex;gap:8px",
                                                get children() {
                                                    return [
                                                        el("button", {
                                                            "class": "btn-quiet",
                                                            "onClick": () => setEditing(true),
                                                            "style": "font-size:12px;padding:4px 10px",
                                                            get children() {
                                                                return "Edit";
                                                            }
                                                        }),
                                                        el("button", {
                                                            "class": "btn-quiet",
                                                            get "onClick"() {
                                                                return remove;
                                                            },
                                                            "style": "font-size:12px;padding:4px 10px;color:var(--rust)",
                                                            get children() {
                                                                return "Delete";
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
                }),
                el("div", {
                    "style": "margin-top:14px;display:grid;grid-template-columns:80px 1fr;gap:6px 12px;align-items:start;font-size:13px",
                    get children() {
                        return [
                            el("span", {
                                "style": "color:var(--ink-mute)",
                                get children() {
                                    return "Members";
                                }
                            }),
                            MemberPickerLite({
                                get "assigned"() {
                                    return members() ?? [];
                                },
                                get "all"() {
                                    return props.houseMembers;
                                },
                                get "canEdit"() {
                                    return props.canAdmin;
                                },
                                "onAdd": async (memberId) => {
                                    await groupClient.addGroupMember({ groupId: props.group.groupId, memberId });
                                    await refetchMembers();
                                },
                                "onRemove": async (memberId) => {
                                    await groupClient.removeGroupMember({ groupId: props.group.groupId, memberId });
                                    await refetchMembers();
                                }
                            }),
                            el("span", {
                                "style": "color:var(--ink-mute)",
                                get children() {
                                    return "Skills";
                                }
                            }),
                            SkillPickerLite({
                                get "assigned"() {
                                    return skills() ?? [];
                                },
                                get "all"() {
                                    return props.houseSkills;
                                },
                                get "canEdit"() {
                                    return props.canAdmin;
                                },
                                "onAdd": async (skillId) => {
                                    await skillClient.addGroupSkill({ groupId: props.group.groupId, skillId });
                                    await refetchSkills();
                                },
                                "onRemove": async (skillId) => {
                                    await skillClient.removeGroupSkill({ groupId: props.group.groupId, skillId });
                                    await refetchSkills();
                                }
                            })
                        ];
                    }
                })
            ];
        }
    }));
};
// ─── Composer ──────────────────────────────────────────────────────────
const GroupForm = (props: {
    houseId: string;
    existing?: Group;
    onCancel: () => void;
    onSaved: () => Promise<void> | void;
}) => {
    const [name, setName] = cell(props.existing?.name ?? "");
    const [description, setDesc] = cell(props.existing?.description ?? "");
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        if (!name().trim() || busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            if (props.existing) {
                await groupClient.updateGroup({
                    ...props.existing,
                    name: name().trim(),
                    description: description().trim() || undefined,
                } as any);
            }
            else {
                await groupClient.createGroup({
                    groupId: "",
                    houseId: props.houseId,
                    name: name().trim(),
                    description: description().trim() || undefined,
                    createdAt: "",
                    updatedAt: "",
                } as any);
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
        "class": "lh-form-grid",
        "style": "margin:8px 0 12px;padding:14px 16px;background:color-mix(in oklab, var(--paper) 92%, var(--ocean-1) 8%);border:1px solid var(--line);border-radius:var(--r-md);display:grid;grid-template-columns:1fr 2fr auto auto;gap:10px;align-items:end",
        get children() {
            return [
                el("label", {
                    "style": "display:flex;flex-direction:column;gap:4px",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
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
                                    return "Description";
                                }
                            }),
                            el("input", {
                                "type": "text",
                                get "value"() {
                                    return description();
                                },
                                "onInput": (e) => setDesc(e.currentTarget.value),
                                "style": "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
                            })
                        ];
                    }
                }),
                el("button", {
                    "class": "btn btn-primary",
                    get "disabled"() {
                        return busy() || !name().trim();
                    },
                    "type": "submit",
                    get children() {
                        return live(() => busy() ? "…" : props.existing ? "Save" : "Create");
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
                When({
                    get "when"() {
                        return err();
                    },
                    children: (m) => el("span", {
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
// ─── Pickers ───────────────────────────────────────────────────────────
const MemberPickerLite = (props: {
    assigned: Member[];
    all: Member[];
    canEdit: boolean;
    onAdd: (memberId: string) => Promise<unknown>;
    onRemove: (memberId: string) => Promise<unknown>;
}) => {
    const [selected, setSelected] = cell("");
    const [busy, setBusy] = cell(false);
    const available = computed(() => {
        const taken = new Set(props.assigned.map((m) => m.memberId));
        return props.all.filter((m) => !taken.has(m.memberId));
    });
    return (el("div", {
        "style": "display:flex;flex-wrap:wrap;gap:6px;align-items:center",
        get children() {
            return [
                When({
                    get "when"() {
                        return props.assigned.length > 0;
                    },
                    get "fallback"() {
                        return el("span", {
                            "style": "font-size:12px;color:var(--ink-mute);font-style:italic",
                            get children() {
                                return "none yet";
                            }
                        });
                    },
                    get children() {
                        return Repeat({
                            get "each"() {
                                return props.assigned;
                            },
                            children: (m) => (el("span", {
                                "style": "display:inline-flex;align-items:center;gap:4px;padding:3px 8px 3px 3px;background:color-mix(in oklab, var(--paper) 90%, var(--ocean-1) 10%);border:1px solid var(--line);border-radius:9999px",
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
                                        el("span", {
                                            "style": "font-size:12px",
                                            get children() {
                                                return live(() => displayName(m));
                                            }
                                        }),
                                        When({
                                            get "when"() {
                                                return props.canEdit;
                                            },
                                            get children() {
                                                return el("button", {
                                                    "type": "button",
                                                    "onClick": () => { setBusy(true); props.onRemove(m.memberId).finally(() => setBusy(false)); },
                                                    "style": "background:transparent;border:0;cursor:pointer;color:var(--ink-mute);font-size:13px;line-height:1;padding:0",
                                                    get "disabled"() {
                                                        return busy();
                                                    },
                                                    "title": "Remove",
                                                    get children() {
                                                        return "\u00D7";
                                                    }
                                                });
                                            }
                                        })
                                    ];
                                }
                            }))
                        });
                    }
                }),
                When({
                    get "when"() {
                        return props.canEdit && available().length > 0;
                    },
                    get children() {
                        return [
                            el("select", {
                                get "value"() {
                                    return selected();
                                },
                                "onChange": (e) => setSelected(e.currentTarget.value),
                                "style": "padding:3px 8px;border:1px dashed var(--line);border-radius:9999px;background:var(--paper);font-size:12px",
                                get "disabled"() {
                                    return busy();
                                },
                                get children() {
                                    return [
                                        el("option", {
                                            "value": "",
                                            get children() {
                                                return "+ add member";
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
                            When({
                                get "when"() {
                                    return selected();
                                },
                                get children() {
                                    return el("button", {
                                        "type": "button",
                                        "class": "btn-quiet",
                                        get "disabled"() {
                                            return busy();
                                        },
                                        "onClick": async () => {
                                            setBusy(true);
                                            try {
                                                await props.onAdd(selected());
                                                setSelected("");
                                            }
                                            catch (e) {
                                                alert(e instanceof Error ? e.message : String(e));
                                            }
                                            finally {
                                                setBusy(false);
                                            }
                                        },
                                        "style": "font-size:11px;padding:2px 8px",
                                        get children() {
                                            return "add";
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
const SkillPickerLite = (props: {
    assigned: Skill[];
    all: Skill[];
    canEdit: boolean;
    onAdd: (skillId: string) => Promise<unknown>;
    onRemove: (skillId: string) => Promise<unknown>;
}) => {
    const [selected, setSelected] = cell("");
    const [busy, setBusy] = cell(false);
    const available = computed(() => {
        const taken = new Set(props.assigned.map((s) => s.skillId));
        return props.all.filter((s) => !taken.has(s.skillId));
    });
    return (el("div", {
        "style": "display:flex;flex-wrap:wrap;gap:6px;align-items:center",
        get children() {
            return [
                When({
                    get "when"() {
                        return props.assigned.length > 0;
                    },
                    get "fallback"() {
                        return el("span", {
                            "style": "font-size:12px;color:var(--ink-mute);font-style:italic",
                            get children() {
                                return "none yet";
                            }
                        });
                    },
                    get children() {
                        return Repeat({
                            get "each"() {
                                return props.assigned;
                            },
                            children: (s) => (el("span", {
                                "style": "display:inline-flex;align-items:center;gap:4px;padding:2px 8px;font-size:12px;border:1px solid var(--grass-4);background:color-mix(in oklab, var(--paper) 88%, var(--grass-4) 12%);border-radius:9999px;color:var(--ink)",
                                get children() {
                                    return [
                                        live(() => s.name),
                                        When({
                                            get "when"() {
                                                return props.canEdit;
                                            },
                                            get children() {
                                                return el("button", {
                                                    "type": "button",
                                                    "onClick": () => { setBusy(true); props.onRemove(s.skillId).finally(() => setBusy(false)); },
                                                    "style": "background:transparent;border:0;cursor:pointer;color:var(--ink-mute);font-size:13px;line-height:1;padding:0",
                                                    get "disabled"() {
                                                        return busy();
                                                    },
                                                    "title": "Remove",
                                                    get children() {
                                                        return "\u00D7";
                                                    }
                                                });
                                            }
                                        })
                                    ];
                                }
                            }))
                        });
                    }
                }),
                When({
                    get "when"() {
                        return props.canEdit && available().length > 0;
                    },
                    get children() {
                        return [
                            el("select", {
                                get "value"() {
                                    return selected();
                                },
                                "onChange": (e) => setSelected(e.currentTarget.value),
                                "style": "padding:3px 8px;border:1px dashed var(--line);border-radius:9999px;background:var(--paper);font-size:12px",
                                get "disabled"() {
                                    return busy();
                                },
                                get children() {
                                    return [
                                        el("option", {
                                            "value": "",
                                            get children() {
                                                return "+ add skill";
                                            }
                                        }),
                                        Repeat({
                                            get "each"() {
                                                return available();
                                            },
                                            children: (s) => el("option", {
                                                get "value"() {
                                                    return s.skillId;
                                                },
                                                get children() {
                                                    return live(() => s.name);
                                                }
                                            })
                                        })
                                    ];
                                }
                            }),
                            When({
                                get "when"() {
                                    return selected();
                                },
                                get children() {
                                    return el("button", {
                                        "type": "button",
                                        "class": "btn-quiet",
                                        get "disabled"() {
                                            return busy();
                                        },
                                        "onClick": async () => {
                                            setBusy(true);
                                            try {
                                                await props.onAdd(selected());
                                                setSelected("");
                                            }
                                            catch (e) {
                                                alert(e instanceof Error ? e.message : String(e));
                                            }
                                            finally {
                                                setBusy(false);
                                            }
                                        },
                                        "style": "font-size:11px;padding:2px 8px",
                                        get children() {
                                            return "add";
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
