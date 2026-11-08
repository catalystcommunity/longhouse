import { cell } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { DateTimePicker } from "./DateTimePicker";
import { MarkdownEditor } from "./MarkdownEditor";
import { RecurrenceFields, type RecurrenceFreq } from "./RecurrenceFields";
import { CommentsSection } from "./CommentsSection";
import { DependenciesSection } from "./DependenciesSection";
import { AddToProject } from "./AddToProject";
import { taskClient } from "~/data/clients";
import { displayName, initial, memberSwatch } from "~/lib/derive";
import type { Member, Task } from "@longhouse/client";
/**
 * Inline detail-and-edit panel for a single task. Lives directly under
 * the task row in any list view (Tasks page, project detail). Surfaces
 * every editable field — title, description, tag, due, estimate, status,
 * assignees, recurrence — plus a delete action and an "Add to project"
 * control (see AddToProject.ts).
 *
 * Behavior: every field is an editable form control from the start; there
 * is no "view then edit" two-step. Save pushes a single UpdateTask call;
 * close just collapses without saving. The parent owns the open/closed
 * state via its own signal and re-fetches the task list on save so the
 * row above reflects the new values.
 */
interface Props {
    task: Task;
    members: Member[];
    onClose: () => void;
    onSaved: () => Promise<unknown>;
    /** Optional: confirm-and-delete this task. When omitted, no delete button. */
    onDelete?: () => Promise<unknown>;
}
const TASK_STATUSES = ["open", "in_progress", "done", "cancelled"] as const;
export const TaskDetailEditor = (props: Props) => {
    // We seed each control from the task and own the edit state locally;
    // saving sends the full updated Task back through UpdateTask.
    const [title, setTitle] = cell(props.task.title);
    const [description, setDesc] = cell(props.task.description ?? "");
    const [tag, setTag] = cell(props.task.tag ?? "");
    const [due, setDue] = cell(props.task.dueAt ?? "");
    const [estimate, setEstimate] = cell(props.task.estimateMinutes !== undefined ? String(props.task.estimateMinutes) : "");
    const [status, setStatus] = cell<string>(typeof props.task.status === "string" ? props.task.status : "open");
    const [assignees, setAssignees] = cell<string[]>((props.task.assignees ?? []).slice());
    const initRecFreq = (): RecurrenceFreq => (typeof props.task.recurrenceFreq === "string" ? props.task.recurrenceFreq : "") as RecurrenceFreq;
    const [recFreq, setRecFreq] = cell<RecurrenceFreq>(initRecFreq());
    const [recInterval, setRecInterval] = cell(props.task.recurrenceInterval ?? 1);
    const [recNextAt, setRecNextAt] = cell(props.task.nextRecurrenceAt ?? "");
    const [recByWeekday, setRecByWeekday] = cell<number[]>(Array.isArray(props.task.recurrenceByWeekday)
        ? props.task.recurrenceByWeekday.map((n) => Number(n))
        : []);
    const [recBySetpos, setRecBySetpos] = cell(props.task.recurrenceBySetpos ?? 1);
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const toggleAssignee = (id: string) => setAssignees((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
    const save = async (e: SubmitEvent) => {
        e.preventDefault();
        if (busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            const body: any = {
                ...props.task,
                title: title().trim() || props.task.title,
                description: description(),
                tag: tag().trim() || undefined,
                dueAt: due() || undefined,
                estimateMinutes: estimate() ? Number(estimate()) : undefined,
                status: status(),
                assignees: assignees(),
                recurrenceFreq: recFreq() || undefined,
                clearRecurrence: !recFreq(),
                recurrenceInterval: recFreq() ? recInterval() : undefined,
                nextRecurrenceAt: recNextAt() || undefined,
                recurrenceByWeekday: recFreq() ? recByWeekday() : [],
                recurrenceBySetpos: recFreq() === "monthly" || recFreq() === "quarterly" || recFreq() === "yearly"
                    ? recBySetpos()
                    : 0,
            };
            await taskClient.updateTask(body);
            await props.onSaved();
            props.onClose();
        }
        catch (e2) {
            setErr(e2 instanceof Error ? e2.message : String(e2));
        }
        finally {
            setBusy(false);
        }
    };
    const remove = async () => {
        if (!props.onDelete)
            return;
        if (!confirm(`Delete "${props.task.title}"?`))
            return;
        setBusy(true);
        setErr(null);
        try {
            await props.onDelete();
            props.onClose();
        }
        catch (e2) {
            setErr(e2 instanceof Error ? e2.message : String(e2));
        }
        finally {
            setBusy(false);
        }
    };
    return (el("div", {
        "style": "margin:6px 0 12px 38px;padding:14px 16px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-md);box-shadow:var(--shadow-low);display:flex;flex-direction:column;gap:10px",
        get children() {
            return [
                el("form", {
                    get "onSubmit"() {
                        return save;
                    },
                    "style": "display:flex;flex-direction:column;gap:10px",
                    get children() {
                        return [
                            el("div", {
                                "class": "lh-form-grid",
                                "style": "display:grid;grid-template-columns:1fr 1fr;gap:10px",
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
                                                        "style": "padding:7px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
                                                    })
                                                ];
                                            }
                                        }),
                                        el("label", {
                                            "style": "display:flex;flex-direction:column;gap:4px;grid-column:1/-1",
                                            get children() {
                                                return [
                                                    el("span", {
                                                        "style": "font-size:11px;color:var(--ink-mute)",
                                                        get children() {
                                                            return "Description (markdown \u2014 click to edit)";
                                                        }
                                                    }),
                                                    MarkdownEditor({
                                                        get "value"() {
                                                            return description();
                                                        },
                                                        get "onChange"() {
                                                            return setDesc;
                                                        },
                                                        "placeholder": "Click to add a description (markdown supported)\u2026",
                                                        "rows": 3
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
                                                            return "Status";
                                                        }
                                                    }),
                                                    el("select", {
                                                        get "value"() {
                                                            return status();
                                                        },
                                                        "onChange": (e) => setStatus(e.currentTarget.value),
                                                        "style": "padding:6px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px",
                                                        get children() {
                                                            return Repeat({
                                                                get "each"() {
                                                                    return TASK_STATUSES;
                                                                },
                                                                children: (s) => el("option", {
                                                                    get "value"() {
                                                                        return s;
                                                                    },
                                                                    get children() {
                                                                        return live(() => s.replace("_", " "));
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
                                                        "style": "padding:7px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
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
                                                            return "Estimate (min)";
                                                        }
                                                    }),
                                                    el("input", {
                                                        "type": "number",
                                                        "min": "0",
                                                        get "value"() {
                                                            return estimate();
                                                        },
                                                        "onInput": (e) => setEstimate(e.currentTarget.value),
                                                        "style": "padding:7px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px"
                                                    })
                                                ];
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("div", {
                                "style": "display:flex;flex-wrap:wrap;gap:6px;align-items:center",
                                get children() {
                                    return [
                                        el("span", {
                                            "style": "font-size:11px;color:var(--ink-mute);margin-right:4px",
                                            get children() {
                                                return "Assignees";
                                            }
                                        }),
                                        Repeat({
                                            get "each"() {
                                                return props.members;
                                            },
                                            children: (m) => {
                                                const on = () => assignees().includes(m.memberId);
                                                return (el("button", {
                                                    "type": "button",
                                                    "onClick": () => toggleAssignee(m.memberId),
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
                                        When({
                                            get "when"() {
                                                return props.onDelete;
                                            },
                                            get children() {
                                                return el("button", {
                                                    "type": "button",
                                                    "class": "btn btn-ghost",
                                                    get "onClick"() {
                                                        return remove;
                                                    },
                                                    get "disabled"() {
                                                        return busy();
                                                    },
                                                    "style": "color:var(--rust);margin-right:auto;padding:6px 12px",
                                                    get children() {
                                                        return "Delete";
                                                    }
                                                });
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
                                            "style": "padding:6px 12px",
                                            get children() {
                                                return "Close";
                                            }
                                        }),
                                        el("button", {
                                            "type": "submit",
                                            "class": "btn btn-primary",
                                            get "disabled"() {
                                                return busy() || !title().trim();
                                            },
                                            "style": "padding:6px 14px",
                                            get children() {
                                                return live(() => busy() ? "Saving…" : "Save");
                                            }
                                        })
                                    ];
                                }
                            })
                        ];
                    }
                }),
                AddToProject({
                    get "task"() {
                        return props.task;
                    },
                    get "onAdded"() {
                        return props.onSaved;
                    }
                }),
                DependenciesSection({
                    "nodeType": "task",
                    get "nodeId"() {
                        return props.task.taskId;
                    },
                    get "houseId"() {
                        return props.task.houseId;
                    }
                }),
                CommentsSection({
                    "targetType": "task",
                    get "targetId"() {
                        return props.task.taskId;
                    },
                    get "houseId"() {
                        return props.task.houseId;
                    },
                    get "members"() {
                        return props.members;
                    }
                })
            ];
        }
    }));
};
