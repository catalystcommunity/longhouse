import { computed, query, cell } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { Link } from "~/ui/router";
import { AuthGate } from "~/components/AuthGate";
import { memberClient, roleClient, skillClient } from "~/data/clients";
import { displayName, initial, lastSeenLabel, memberHandle, memberStatus, memberSwatch } from "~/lib/derive";
import { currentMemberId, hasRole, useCurrentHouseId } from "~/stores/auth";
import { loadHouses } from "~/lib/session";
import type { Member, Role, Skill } from "@longhouse/client";
export const MembersPage = () => {
    const houseId = useCurrentHouseId();
    const isAdmin = () => hasRole("admin");
    const [members, { refetch }] = query(() => houseId(), async (h) => memberClient.listMembers({ houseId: h }));
    const [roles, { refetch: refetchRoles }] = query(() => houseId(), async (h) => roleClient.listRoles({ houseId: h }));
    const [skills, { refetch: refetchSkills }] = query(() => houseId(), async (h) => skillClient.listSkills({ houseId: h }));
    const [inviteOpen, setInviteOpen] = cell(false);
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
                                        "Members ",
                                        el("em", {
                                            get children() {
                                                return "the household";
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("p", {
                                "class": "lead",
                                get children() {
                                    return "Everyone with access to this Longhouse instance.";
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
                                                "onClick": () => setInviteOpen((v) => !v),
                                                get children() {
                                                    return live(() => inviteOpen() ? "Cancel" : "Invite member");
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
                        return inviteOpen();
                    },
                    get children() {
                        return InviteForm({
                            get "houseId"() {
                                return houseId()!;
                            },
                            "onCancel": () => setInviteOpen(false),
                            "onInvited": async () => { setInviteOpen(false); await refetch(); }
                        });
                    }
                }),
                el("section", {
                    "class": "card folks reveal d1",
                    "style": "margin-top:0;padding: 8px 22px 18px",
                    get children() {
                        return When({
                            get "when"() {
                                return !members.loading;
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
                                        return (members() ?? []).length > 0;
                                    },
                                    get "fallback"() {
                                        return el("p", {
                                            "style": "padding:24px;color:var(--ink-mute)",
                                            get children() {
                                                return "No members in this house yet.";
                                            }
                                        });
                                    },
                                    get children() {
                                        return Repeat({
                                            get "each"() {
                                                return members()!;
                                            },
                                            children: (m) => (MemberRow({
                                                get "member"() {
                                                    return m;
                                                },
                                                get "houseId"() {
                                                    return houseId()!;
                                                },
                                                get "houseRoles"() {
                                                    return roles() ?? [];
                                                },
                                                get "houseSkills"() {
                                                    return skills() ?? [];
                                                },
                                                "onChanged": async () => {
                                                    await refetch();
                                                    await refetchRoles();
                                                    await refetchSkills();
                                                }
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
// ─── Invite ───────────────────────────────────────────────────────────
const InviteForm = (props: {
    houseId: string;
    onCancel: () => void;
    onInvited: () => Promise<void> | void;
}) => {
    const [domain, setDomain] = cell("");
    const [userId, setUserId] = cell("");
    const [displayN, setDisplayN] = cell("");
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        if (!domain().trim() || !userId().trim() || busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            await memberClient.createMember({
                memberId: "",
                houseId: props.houseId,
                linkkeysDomain: domain().trim(),
                linkkeysUserId: userId().trim(),
                displayName: displayN().trim() || undefined,
                createdAt: "",
                updatedAt: "",
            } as any);
            await props.onInvited();
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
        "style": "margin-bottom:16px;padding:16px 20px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);display:grid;grid-template-columns:1fr 1fr 1fr auto auto;gap:10px;align-items:end",
        get children() {
            return [
                el("label", {
                    "style": "display:flex;flex-direction:column;gap:4px",
                    get children() {
                        return [
                            el("span", {
                                "style": "font-size:12px;color:var(--ink-mute)",
                                get children() {
                                    return "Linkkeys domain";
                                }
                            }),
                            el("input", {
                                "type": "text",
                                get "value"() {
                                    return domain();
                                },
                                "onInput": (e) => setDomain(e.currentTarget.value),
                                "placeholder": "todandlorna.com",
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
                                    return "Linkkeys user_id (UUID)";
                                }
                            }),
                            el("input", {
                                "type": "text",
                                get "value"() {
                                    return userId();
                                },
                                "onInput": (e) => setUserId(e.currentTarget.value),
                                "placeholder": "019d6ac9-\u2026",
                                "required": true,
                                "style": "padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;font-family:var(--mono, monospace);font-size:12px"
                            }),
                            el("span", {
                                "style": "font-size:11px;color:var(--ink-mute)",
                                get children() {
                                    return "The person's linkkeys account id \u2014 not their email. Ask them for it, or copy it from their linkkeys profile.";
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
                                "style": "font-size:12px;color:var(--ink-mute)",
                                get children() {
                                    return "Display name (optional)";
                                }
                            }),
                            el("input", {
                                "type": "text",
                                get "value"() {
                                    return displayN();
                                },
                                "onInput": (e) => setDisplayN(e.currentTarget.value),
                                "placeholder": "how the UI shows them",
                                "style": "padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
                            })
                        ];
                    }
                }),
                el("button", {
                    "class": "btn btn-primary",
                    get "disabled"() {
                        return busy() || !domain().trim() || !userId().trim();
                    },
                    "type": "submit",
                    get children() {
                        return live(() => busy() ? "Saving…" : "Invite");
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
                        return !domain().trim() || !userId().trim();
                    },
                    get children() {
                        return el("p", {
                            "style": "color:var(--ink-mute);font-size:12px;grid-column:1/-1;margin:0",
                            get children() {
                                return "Fill in both the linkkeys domain and user_id to enable Invite.";
                            }
                        });
                    }
                }),
                el("p", {
                    "style": "color:var(--ink-mute);font-size:12px;grid-column:1/-1;margin:0",
                    get children() {
                        return [
                            "Adding everyone from a domain? Skip per-person invites \u2014 add a",
                            live(() => " "),
                            Link({
                                "href": "/settings",
                                "style": "color:var(--ocean-2)",
                                get children() {
                                    return "trusted domain";
                                }
                            }),
                            live(() => " "),
                            "and anyone signing in from it auto-joins on their next sign-in."
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
                })
            ];
        }
    }));
};
// ─── Row ──────────────────────────────────────────────────────────────
const MemberRow = (props: {
    member: Member;
    houseId: string;
    houseRoles: Role[];
    houseSkills: Skill[];
    onChanged: () => Promise<unknown>;
}) => {
    const isSelf = () => currentMemberId() === props.member.memberId;
    const canEdit = () => isSelf() || hasRole("admin");
    const canAdmin = () => hasRole("admin");
    const [editing, setEditing] = cell(false);
    const status = () => memberStatus(props.member);
    const [memberRoles, { refetch: refetchRoles }] = query(() => ({ houseId: props.houseId, memberId: props.member.memberId }), async (req) => roleClient.listMemberRoles(req));
    const [memberSkills, { refetch: refetchSkills }] = query(() => ({ houseId: props.houseId, memberId: props.member.memberId }), async (req) => skillClient.listMemberSkills(req));
    const isDeactivated = () => !!props.member.deactivatedAt;
    const deactivateMember = async () => {
        if (!confirm(`Deactivate ${displayName(props.member)}? They'll be denied future login, but their record and content stay and you can reactivate them later.`))
            return;
        try {
            await memberClient.deactivateMember(props.member.memberId);
            await props.onChanged();
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
    };
    const reactivateMember = async () => {
        try {
            await memberClient.reactivateMember(props.member.memberId);
            await props.onChanged();
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
    };
    return (el("div", {
        "style": "padding:14px 0;border-bottom:1px solid var(--line)",
        get children() {
            return [
                el("div", {
                    get "class"() {
                        return `folk ${status() === "away" ? "away" : ""}`;
                    },
                    "style": "padding:0;border:0",
                    get children() {
                        return [
                            el("span", {
                                get "class"() {
                                    return `a lg ${memberSwatch(props.member.memberId)}`;
                                },
                                get children() {
                                    return live(() => initial(props.member));
                                }
                            }),
                            el("div", {
                                "style": "flex:1;min-width:0",
                                get children() {
                                    return When({
                                        get "when"() {
                                            return !editing();
                                        },
                                        get "fallback"() {
                                            return DisplayNameEditor({
                                                get "member"() {
                                                    return props.member;
                                                },
                                                "onCancel": () => setEditing(false),
                                                "onSaved": async () => {
                                                    setEditing(false);
                                                    await props.onChanged();
                                                    if (isSelf()) {
                                                        try {
                                                            await loadHouses();
                                                        }
                                                        catch { /* tolerated */ }
                                                    }
                                                }
                                            });
                                        },
                                        get children() {
                                            return [
                                                el("div", {
                                                    "class": "who-name",
                                                    get children() {
                                                        return [
                                                            live(() => displayName(props.member)),
                                                            When({
                                                                get "when"() {
                                                                    return isSelf();
                                                                },
                                                                get children() {
                                                                    return el("span", {
                                                                        "style": "font-size:11px;font-weight:400;color:var(--ink-mute);margin-left:8px",
                                                                        get children() {
                                                                            return "you";
                                                                        }
                                                                    });
                                                                }
                                                            }),
                                                            When({
                                                                get "when"() {
                                                                    return isDeactivated();
                                                                },
                                                                get children() {
                                                                    return el("span", {
                                                                        "style": "font-size:11px;font-weight:400;color:var(--rust);margin-left:8px",
                                                                        get children() {
                                                                            return "deactivated";
                                                                        }
                                                                    });
                                                                }
                                                            })
                                                        ];
                                                    }
                                                }),
                                                el("div", {
                                                    "class": "doing",
                                                    get children() {
                                                        return live(() => memberHandle(props.member) ??
                                                            `${props.member.linkkeysUserId}@${props.member.linkkeysDomain}`);
                                                    }
                                                })
                                            ];
                                        }
                                    });
                                }
                            }),
                            When({
                                get "when"() {
                                    return !editing() && canEdit();
                                },
                                get children() {
                                    return el("button", {
                                        "class": "btn-quiet",
                                        "onClick": () => setEditing(true),
                                        "style": "font-size:12px;padding:4px 10px",
                                        get children() {
                                            return live(() => isSelf() ? "Edit profile" : "Edit");
                                        }
                                    });
                                }
                            }),
                            When({
                                get "when"() {
                                    return !editing() && canAdmin() && !isSelf();
                                },
                                get children() {
                                    return When({
                                        get "when"() {
                                            return isDeactivated();
                                        },
                                        get "fallback"() {
                                            return el("button", {
                                                "class": "btn-quiet",
                                                get "onClick"() {
                                                    return deactivateMember;
                                                },
                                                "style": "font-size:12px;padding:4px 10px;color:var(--rust)",
                                                get children() {
                                                    return "Deactivate";
                                                }
                                            });
                                        },
                                        get children() {
                                            return el("button", {
                                                "class": "btn-quiet",
                                                get "onClick"() {
                                                    return reactivateMember;
                                                },
                                                "style": "font-size:12px;padding:4px 10px;color:var(--grass-4)",
                                                get children() {
                                                    return "Reactivate";
                                                }
                                            });
                                        }
                                    });
                                }
                            }),
                            When({
                                get "when"() {
                                    return !editing();
                                },
                                get children() {
                                    return el("span", {
                                        "class": "ago",
                                        get children() {
                                            return live(() => lastSeenLabel(props.member) ?? "");
                                        }
                                    });
                                }
                            })
                        ];
                    }
                }),
                el("div", {
                    "style": "margin-top:8px;display:grid;grid-template-columns:80px 1fr;gap:6px 12px;align-items:start;font-size:13px",
                    get children() {
                        return [
                            el("span", {
                                "style": "color:var(--ink-mute)",
                                get children() {
                                    return "Roles";
                                }
                            }),
                            TagChips({
                                get "assigned"() {
                                    return memberRoles() ?? [];
                                },
                                get "all"() {
                                    return props.houseRoles;
                                },
                                "getId": (r) => r.roleId,
                                "getName": (r) => r.name,
                                get "canEdit"() {
                                    return canAdmin();
                                },
                                "color": "var(--ocean-2)",
                                "onAdd": async (roleId) => {
                                    await roleClient.grantRole({ memberId: props.member.memberId, roleId });
                                    await refetchRoles();
                                },
                                "onRemove": async (roleId) => {
                                    await roleClient.revokeRole({ memberId: props.member.memberId, roleId });
                                    await refetchRoles();
                                }
                            }),
                            el("span", {
                                "style": "color:var(--ink-mute)",
                                get children() {
                                    return "Skills";
                                }
                            }),
                            TagChips({
                                get "assigned"() {
                                    return memberSkills() ?? [];
                                },
                                get "all"() {
                                    return props.houseSkills;
                                },
                                "getId": (s) => s.skillId,
                                "getName": (s) => s.name,
                                get "canEdit"() {
                                    return canEdit();
                                },
                                "color": "var(--grass-4)",
                                "onAdd": async (skillId) => {
                                    await skillClient.addMemberSkill({ memberId: props.member.memberId, skillId });
                                    await refetchSkills();
                                },
                                "onRemove": async (skillId) => {
                                    await skillClient.removeMemberSkill({ memberId: props.member.memberId, skillId });
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
// Edits the profile fields a member owns: display name and avatar URL. Email is
// shown read-only — it's seeded from a verified linkkeys claim and isn't
// user-settable here (verification territory). The avatar URL is user-
// overridable; the api fetches + caches the image it points at.
const DisplayNameEditor = (props: {
    member: Member;
    onCancel: () => void;
    onSaved: () => Promise<void> | void;
}) => {
    const [name, setName] = cell(props.member.displayName ?? "");
    const [avatar, setAvatar] = cell(props.member.avatarUrl ?? "");
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        if (busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            await memberClient.updateMember({
                ...props.member,
                displayName: name().trim() || undefined,
                avatarUrl: avatar().trim() || undefined,
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
    const field = "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px";
    return (el("form", {
        get "onSubmit"() {
            return submit;
        },
        "style": "display:flex;gap:8px;align-items:center;flex-wrap:wrap",
        get children() {
            return [
                el("input", {
                    "type": "text",
                    get "value"() {
                        return name();
                    },
                    "onInput": (e) => setName(e.currentTarget.value),
                    "placeholder": "display name",
                    "autofocus": true,
                    get "style"() {
                        return `flex:1 1 200px;${field}`;
                    }
                }),
                el("input", {
                    "type": "url",
                    get "value"() {
                        return avatar();
                    },
                    "onInput": (e) => setAvatar(e.currentTarget.value),
                    "placeholder": "avatar image URL",
                    get "style"() {
                        return `flex:1 1 200px;${field}`;
                    }
                }),
                el("button", {
                    "class": "btn btn-primary",
                    get "disabled"() {
                        return busy();
                    },
                    "type": "submit",
                    "style": "padding:6px 14px",
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
                    "style": "padding:6px 12px",
                    get children() {
                        return "Cancel";
                    }
                }),
                When({
                    get "when"() {
                        return props.member.email;
                    },
                    children: (em) => el("span", {
                        "style": "color:var(--ink-mute);font-size:12px;flex:1 1 100%",
                        get children() {
                            return [
                                live(() => em()),
                                " \u00B7 from linkkeys"
                            ];
                        }
                    })
                }),
                When({
                    get "when"() {
                        return err();
                    },
                    children: (m) => el("span", {
                        "style": "color:var(--rust);font-size:12px;flex:1 1 100%",
                        get children() {
                            return live(() => m());
                        }
                    })
                })
            ];
        }
    }));
};
// ─── Generic chip-set editor ──────────────────────────────────────────
function TagChips<T>(props: {
    assigned: T[];
    all: T[];
    getId: (x: T) => string;
    getName: (x: T) => string;
    canEdit: boolean;
    color: string;
    onAdd: (id: string) => Promise<unknown>;
    onRemove: (id: string) => Promise<unknown>;
}) {
    const [selected, setSelected] = cell("");
    const [busy, setBusy] = cell(false);
    const available = computed(() => {
        const taken = new Set(props.assigned.map(props.getId));
        return props.all.filter((x) => !taken.has(props.getId(x)));
    });
    const add = async () => {
        const id = selected();
        if (!id || busy())
            return;
        setBusy(true);
        try {
            await props.onAdd(id);
            setSelected("");
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
        finally {
            setBusy(false);
        }
    };
    const remove = async (id: string) => {
        if (busy())
            return;
        setBusy(true);
        try {
            await props.onRemove(id);
        }
        catch (e) {
            alert(e instanceof Error ? e.message : String(e));
        }
        finally {
            setBusy(false);
        }
    };
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
                            "style": "color:var(--ink-mute);font-size:12px;font-style:italic",
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
                            children: (item) => (el("span", {
                                get "style"() {
                                    return `display:inline-flex;align-items:center;gap:4px;padding:2px 8px;font-size:12px;border-radius:9999px;border:1px solid ${props.color};background:color-mix(in oklab, var(--paper) 88%, ${props.color} 12%);color:var(--ink)`;
                                },
                                get children() {
                                    return [
                                        live(() => props.getName(item)),
                                        When({
                                            get "when"() {
                                                return props.canEdit;
                                            },
                                            get children() {
                                                return el("button", {
                                                    "type": "button",
                                                    "onClick": () => remove(props.getId(item)),
                                                    "style": "background:transparent;border:0;color:var(--ink-mute);cursor:pointer;font-size:13px;line-height:1;padding:0",
                                                    "title": "Remove",
                                                    get "disabled"() {
                                                        return busy();
                                                    },
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
                                                return "+ add";
                                            }
                                        }),
                                        Repeat({
                                            get "each"() {
                                                return available();
                                            },
                                            children: (x) => el("option", {
                                                get "value"() {
                                                    return props.getId(x);
                                                },
                                                get children() {
                                                    return live(() => props.getName(x));
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
                                        get "onClick"() {
                                            return add;
                                        },
                                        get "disabled"() {
                                            return busy();
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
}
