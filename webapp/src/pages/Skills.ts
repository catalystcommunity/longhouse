import { query, cell } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { AuthGate } from "~/components/AuthGate";
import { skillClient } from "~/data/clients";
import { hasRole, useCurrentHouseId } from "~/stores/auth";
import type { Skill } from "@longhouse/client";
export const SkillsPage = () => {
    const houseId = useCurrentHouseId();
    const isAdmin = () => hasRole("admin");
    const [skills, { refetch }] = query(() => houseId(), async (h) => skillClient.listSkills({ houseId: h }));
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
                                        "Skills ",
                                        el("em", {
                                            get children() {
                                                return "what people are good at";
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("p", {
                                "class": "lead",
                                get children() {
                                    return "A house-wide catalog. Members add their own; admins can add for anyone. Attach to groups on the Groups page; to people on the Members page.";
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
                                                    return live(() => composerOpen() ? "Cancel" : "New skill");
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
                        return SkillForm({
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
                        return !skills.loading;
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
                                return (skills() ?? []).length > 0;
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
                                                            return "No skills yet.";
                                                        }
                                                    });
                                                }
                                            }),
                                            el("p", {
                                                "style": "margin:0",
                                                get children() {
                                                    return "Capabilities like \"Plumbing\", \"Bookkeeping\", or \"First aid\". Add one to begin attaching it to people or groups.";
                                                }
                                            })
                                        ];
                                    }
                                });
                            },
                            get children() {
                                return el("ul", {
                                    "style": "margin:8px 0 0;padding:0;list-style:none;display:grid;grid-template-columns:1fr;gap:10px",
                                    get children() {
                                        return Repeat({
                                            get "each"() {
                                                return skills()!;
                                            },
                                            children: (s) => (SkillRow({
                                                get "skill"() {
                                                    return s;
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
const SkillRow = (props: {
    skill: Skill;
    canAdmin: boolean;
    onChanged: () => Promise<unknown>;
}) => {
    const [editing, setEditing] = cell(false);
    const remove = async () => {
        if (!confirm(`Delete skill "${props.skill.name}"?`))
            return;
        try {
            await skillClient.deleteSkill(props.skill.skillId);
            await props.onChanged();
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
    };
    return (el("li", {
        "style": "padding:14px 18px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);display:flex;justify-content:space-between;align-items:center;gap:12px",
        get children() {
            return When({
                get "when"() {
                    return !editing();
                },
                get "fallback"() {
                    return SkillForm({
                        get "houseId"() {
                            return props.skill.houseId;
                        },
                        get "existing"() {
                            return props.skill;
                        },
                        "onCancel": () => setEditing(false),
                        "onSaved": async () => { setEditing(false); await props.onChanged(); }
                    });
                },
                get children() {
                    return [
                        el("div", {
                            get children() {
                                return [
                                    el("div", {
                                        "style": "font-weight:600",
                                        get children() {
                                            return live(() => props.skill.name);
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return props.skill.description;
                                        },
                                        children: (d) => el("div", {
                                            "style": "color:var(--ink-mute);font-size:13px",
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
                                                "style": "font-size:12px;padding:4px 10px",
                                                "onClick": () => setEditing(true),
                                                get children() {
                                                    return "Edit";
                                                }
                                            }),
                                            el("button", {
                                                "class": "btn-quiet",
                                                "style": "font-size:12px;padding:4px 10px;color:var(--rust)",
                                                get "onClick"() {
                                                    return remove;
                                                },
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
    }));
};
const SkillForm = (props: {
    houseId: string;
    existing?: Skill;
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
                await skillClient.updateSkill({
                    ...props.existing,
                    name: name().trim(),
                    description: description().trim() || undefined,
                } as any);
            }
            else {
                await skillClient.createSkill({
                    skillId: "",
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
        "style": "display:grid;grid-template-columns:1fr 2fr auto auto;gap:10px;align-items:end;width:100%",
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
