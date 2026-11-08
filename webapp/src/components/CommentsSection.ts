import { computed, query, cell } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { commentClient } from "~/data/clients";
import { currentMemberId, hasRole } from "~/stores/auth";
import { displayName, initial, memberSwatch } from "~/lib/derive";
import { normalizeCommentCreate } from "~/lib/taskCreate";
import type { Comment, Member } from "@longhouse/client";
/**
 * Discussion thread for a single target (a task or a project). Lists existing
 * comments oldest-first with author + local-time stamp, then a text entry bar
 * with an "Add" button that posts a new comment.
 *
 * Permission gating mirrors the API (which is authoritative):
 *   - delete icon shows for the comment's author OR any house admin.
 *   - edit shows only for the author — admins can remove but not rewrite
 *     someone else's words.
 *
 * All DB timestamps are UTC; we render them in the viewer's local timezone via
 * Date#toLocaleString.
 */
interface Props {
    targetType: "task" | "project";
    targetId: string;
    /** The target's house; used to stamp the new comment. */
    houseId: string;
    /** House members, for resolving author names + avatars. */
    members: Member[];
}
const localStamp = (iso: string): string => {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime()))
        return iso;
    return d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
};
export const CommentsSection = (props: Props) => {
    const [comments, { refetch, mutate }] = query(() => ({ t: props.targetType, id: props.targetId }), async ({ t, id }) => id ? await commentClient.listComments({ targetType: t as any, targetId: id }) : []);
    const memberById = computed(() => {
        const m = new Map<string, Member>();
        for (const x of props.members)
            m.set(x.memberId, x);
        return m;
    });
    const [draft, setDraft] = cell("");
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const isAdmin = () => hasRole("admin");
    const canDelete = (c: Comment) => c.memberId === currentMemberId() || isAdmin();
    const canEdit = (c: Comment) => c.memberId === currentMemberId();
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        const text = draft().trim();
        if (!text || busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            await commentClient.createComment(normalizeCommentCreate({
                houseId: props.houseId,
                targetType: props.targetType as any,
                targetId: props.targetId,
                body: text,
            }));
            setDraft("");
            await refetch();
        }
        catch (e2) {
            setErr(e2 instanceof Error ? e2.message : String(e2));
        }
        finally {
            setBusy(false);
        }
    };
    const remove = async (c: Comment) => {
        if (!confirm("Delete this comment?"))
            return;
        mutate((prev) => (prev ?? []).filter((x) => x.commentId !== c.commentId));
        try {
            await commentClient.deleteComment(c.commentId);
            await refetch();
        }
        catch (e2) {
            setErr(e2 instanceof Error ? e2.message : String(e2));
            await refetch();
        }
    };
    return (el("div", {
        "style": "margin-top:12px;border-top:1px solid var(--line);padding-top:10px;display:flex;flex-direction:column;gap:10px",
        get children() {
            return [
                el("span", {
                    "style": "font-size:11px;color:var(--ink-mute);text-transform:uppercase;letter-spacing:.04em",
                    get children() {
                        return "Discussion";
                    }
                }),
                When({
                    get "when"() {
                        return !comments.loading;
                    },
                    get "fallback"() {
                        return el("span", {
                            "style": "font-size:12px;color:var(--ink-faint)",
                            get children() {
                                return "Loading\u2026";
                            }
                        });
                    },
                    get children() {
                        return When({
                            get "when"() {
                                return (comments() ?? []).length > 0;
                            },
                            get "fallback"() {
                                return el("span", {
                                    "style": "font-size:12.5px;color:var(--ink-faint)",
                                    get children() {
                                        return "No comments yet.";
                                    }
                                });
                            },
                            get children() {
                                return el("div", {
                                    "style": "display:flex;flex-direction:column;gap:10px",
                                    get children() {
                                        return Repeat({
                                            get "each"() {
                                                return comments();
                                            },
                                            children: (c) => (CommentItem({
                                                get "comment"() {
                                                    return c;
                                                },
                                                get "author"() {
                                                    return memberById().get(c.memberId);
                                                },
                                                get "canEdit"() {
                                                    return canEdit(c);
                                                },
                                                get "canDelete"() {
                                                    return canDelete(c);
                                                },
                                                "onChanged": async () => { await refetch(); },
                                                "onDelete": () => remove(c),
                                                get "onError"() {
                                                    return setErr;
                                                }
                                            }))
                                        });
                                    }
                                });
                            }
                        });
                    }
                }),
                el("form", {
                    get "onSubmit"() {
                        return submit;
                    },
                    "style": "display:flex;gap:8px;align-items:flex-start",
                    get children() {
                        return [
                            el("textarea", {
                                get "value"() {
                                    return draft();
                                },
                                "onInput": (e) => setDraft(e.currentTarget.value),
                                "placeholder": "Add a comment\u2026",
                                "rows": 1,
                                "onKeyDown": (e) => {
                                    if ((e.metaKey || e.ctrlKey) && e.key === "Enter")
                                        submit(e as any);
                                },
                                "style": "flex:1;padding:7px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px;font-family:inherit;resize:vertical;min-height:34px"
                            }),
                            el("button", {
                                "type": "submit",
                                "class": "btn btn-primary",
                                get "disabled"() {
                                    return busy() || !draft().trim();
                                },
                                "style": "padding:7px 14px;white-space:nowrap",
                                get children() {
                                    return live(() => busy() ? "…" : "Add");
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
                })
            ];
        }
    }));
};
// ─── A single comment row, with inline edit ──────────────────────────────
const CommentItem = (props: {
    comment: Comment;
    author?: Member;
    canEdit: boolean;
    canDelete: boolean;
    onChanged: () => Promise<unknown>;
    onDelete: () => void;
    onError: (m: string) => void;
}) => {
    const [editing, setEditing] = cell(false);
    const [body, setBody] = cell(props.comment.body);
    const [busy, setBusy] = cell(false);
    const author = () => props.author ? displayName(props.author) : "Member";
    const swatch = () => memberSwatch(props.comment.memberId);
    const edited = () => props.comment.updatedAt && props.comment.updatedAt !== props.comment.createdAt;
    const save = async () => {
        const text = body().trim();
        if (!text || busy())
            return;
        setBusy(true);
        try {
            await commentClient.updateComment({ ...props.comment, body: text } as any);
            setEditing(false);
            await props.onChanged();
        }
        catch (e) {
            props.onError(e instanceof Error ? e.message : String(e));
        }
        finally {
            setBusy(false);
        }
    };
    return (el("div", {
        "style": "display:flex;gap:8px;align-items:flex-start",
        get children() {
            return [
                el("span", {
                    get "class"() {
                        return `a sm ${swatch()}`;
                    },
                    "style": "width:22px;height:22px;font-size:11px;flex:none;margin-top:1px",
                    get children() {
                        return live(() => props.author ? initial(props.author) : "?");
                    }
                }),
                el("div", {
                    "style": "flex:1;min-width:0",
                    get children() {
                        return [
                            el("div", {
                                "style": "display:flex;align-items:baseline;gap:8px",
                                get children() {
                                    return [
                                        el("span", {
                                            "style": "font-size:12.5px;font-weight:600;color:var(--ink)",
                                            get children() {
                                                return live(() => author());
                                            }
                                        }),
                                        el("span", {
                                            "style": "font-size:11px;color:var(--ink-faint)",
                                            get children() {
                                                return [
                                                    live(() => localStamp(props.comment.createdAt)),
                                                    When({
                                                        get "when"() {
                                                            return edited();
                                                        },
                                                        get children() {
                                                            return " \u00B7 edited";
                                                        }
                                                    })
                                                ];
                                            }
                                        }),
                                        el("span", {
                                            "style": "margin-left:auto;display:flex;gap:4px",
                                            get children() {
                                                return [
                                                    When({
                                                        get "when"() {
                                                            return props.canEdit && !editing();
                                                        },
                                                        get children() {
                                                            return el("button", {
                                                                "type": "button",
                                                                "aria-label": "Edit comment",
                                                                "onClick": () => setEditing(true),
                                                                "style": "background:transparent;border:0;color:var(--ink-mute);font-size:13px;cursor:pointer;padding:0 4px",
                                                                get children() {
                                                                    return "\u270E";
                                                                }
                                                            });
                                                        }
                                                    }),
                                                    When({
                                                        get "when"() {
                                                            return props.canDelete;
                                                        },
                                                        get children() {
                                                            return el("button", {
                                                                "type": "button",
                                                                "aria-label": "Delete comment",
                                                                "onClick": () => props.onDelete(),
                                                                "style": "background:transparent;border:0;color:var(--ink-mute);font-size:16px;line-height:1;cursor:pointer;padding:0 4px",
                                                                get children() {
                                                                    return "\u00D7";
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
                            When({
                                get "when"() {
                                    return editing();
                                },
                                get "fallback"() {
                                    return el("p", {
                                        "style": "margin:2px 0 0;font-size:13px;color:var(--ink);white-space:pre-wrap;word-break:break-word",
                                        get children() {
                                            return live(() => props.comment.body);
                                        }
                                    });
                                },
                                get children() {
                                    return el("div", {
                                        "style": "display:flex;flex-direction:column;gap:6px;margin-top:4px",
                                        get children() {
                                            return [
                                                el("textarea", {
                                                    get "value"() {
                                                        return body();
                                                    },
                                                    "onInput": (e) => setBody(e.currentTarget.value),
                                                    "rows": 2,
                                                    "style": "padding:6px 9px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px;font-family:inherit;resize:vertical"
                                                }),
                                                el("div", {
                                                    "style": "display:flex;gap:6px;justify-content:flex-end",
                                                    get children() {
                                                        return [
                                                            el("button", {
                                                                "type": "button",
                                                                "class": "btn btn-ghost",
                                                                "onClick": () => { setBody(props.comment.body); setEditing(false); },
                                                                get "disabled"() {
                                                                    return busy();
                                                                },
                                                                "style": "padding:4px 10px",
                                                                get children() {
                                                                    return "Cancel";
                                                                }
                                                            }),
                                                            el("button", {
                                                                "type": "button",
                                                                "class": "btn btn-primary",
                                                                get "onClick"() {
                                                                    return save;
                                                                },
                                                                get "disabled"() {
                                                                    return busy() || !body().trim();
                                                                },
                                                                "style": "padding:4px 12px",
                                                                get children() {
                                                                    return live(() => busy() ? "Saving…" : "Save");
                                                                }
                                                            })
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
                })
            ];
        }
    }));
};
