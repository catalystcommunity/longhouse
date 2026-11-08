import { cell } from "~/ui/state";
import { When, Overlay, el, live } from "~/ui/dom";
import { bugClient } from "~/data/clients";
import { useCurrentHouseId } from "~/stores/auth";
import { bugReportsEnabled } from "~/stores/settings";
import { Bug } from "./Icons";
/**
 * Icon-button + modal for the in-app bug reporter. Only renders when the
 * per-house `bug_reports_enabled` setting is on; the modal collects a
 * title and (optional) description and posts to BugService.ReportBug.
 * The server attributes the reporter in the description itself — the SPA
 * doesn't add anything visible to the reporter on top.
 */
export const BugReportButton = () => {
    const houseId = useCurrentHouseId();
    const [open, setOpen] = cell(false);
    const [title, setTitle] = cell("");
    const [description, setDescription] = cell("");
    const [busy, setBusy] = cell(false);
    const [err, setErr] = cell<string | null>(null);
    const [done, setDone] = cell(false);
    const reset = () => {
        setTitle("");
        setDescription("");
        setErr(null);
        setDone(false);
    };
    const close = () => {
        setOpen(false);
        reset();
    };
    const submit = async (e: SubmitEvent) => {
        e.preventDefault();
        const hid = houseId();
        if (!hid || !title().trim() || busy())
            return;
        setBusy(true);
        setErr(null);
        try {
            await bugClient.reportBug({
                houseId: hid,
                title: title().trim(),
                description: description().trim() || undefined,
            });
            setDone(true);
        }
        catch (e2) {
            setErr(e2 instanceof Error ? e2.message : String(e2));
        }
        finally {
            setBusy(false);
        }
    };
    return (When({
        get "when"() {
            return bugReportsEnabled();
        },
        get children() {
            return [
                el("button", {
                    "class": "icon-btn",
                    "onClick": () => setOpen(true),
                    "aria-label": "Report a bug",
                    "title": "Report a bug",
                    get children() {
                        return Bug();
                    }
                }),
                When({
                    get "when"() {
                        return open();
                    },
                    get children() {
                        return Overlay({
                            get children() {
                                return el("div", {
                                    "role": "dialog",
                                    "aria-modal": "true",
                                    "aria-label": "Report a bug",
                                    "style": "position:fixed;inset:0;background:rgba(0,0,0,0.45);display:flex;align-items:center;justify-content:center;z-index:1000",
                                    "onClick": (e) => {
                                        if (e.target === e.currentTarget)
                                            close();
                                    },
                                    get children() {
                                        return el("div", {
                                            "style": "background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-high);padding:22px 24px;width:min(520px,calc(100% - 32px));max-height:calc(100% - 32px);overflow:auto",
                                            get children() {
                                                return [
                                                    el("h2", {
                                                        "style": "margin:0 0 4px;font-family:var(--display);font-size:22px;color:var(--grass-4)",
                                                        get children() {
                                                            return "Report a bug";
                                                        }
                                                    }),
                                                    el("p", {
                                                        "style": "margin:0 0 16px;color:var(--ink-mute);font-size:13px",
                                                        get children() {
                                                            return "Tell us what went wrong. A house admin will see this as a task in the bug-fixes project.";
                                                        }
                                                    }),
                                                    When({
                                                        get "when"() {
                                                            return !done();
                                                        },
                                                        get "fallback"() {
                                                            return el("div", {
                                                                get children() {
                                                                    return [
                                                                        el("p", {
                                                                            "style": "margin:0 0 16px;color:var(--grass-4)",
                                                                            get children() {
                                                                                return "Thanks \u2014 your report was submitted.";
                                                                            }
                                                                        }),
                                                                        el("div", {
                                                                            "style": "display:flex;justify-content:flex-end;gap:8px",
                                                                            get children() {
                                                                                return el("button", {
                                                                                    "class": "btn btn-ghost",
                                                                                    "type": "button",
                                                                                    get "onClick"() {
                                                                                        return close;
                                                                                    },
                                                                                    get children() {
                                                                                        return "Close";
                                                                                    }
                                                                                });
                                                                            }
                                                                        })
                                                                    ];
                                                                }
                                                            });
                                                        },
                                                        get children() {
                                                            return el("form", {
                                                                get "onSubmit"() {
                                                                    return submit;
                                                                },
                                                                "style": "display:flex;flex-direction:column;gap:12px",
                                                                get children() {
                                                                    return [
                                                                        el("label", {
                                                                            "style": "display:flex;flex-direction:column;gap:4px",
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
                                                                                        "maxLength": 512,
                                                                                        "placeholder": "Short summary",
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
                                                                                            return "Description (optional)";
                                                                                        }
                                                                                    }),
                                                                                    el("textarea", {
                                                                                        get "value"() {
                                                                                            return description();
                                                                                        },
                                                                                        "onInput": (e) => setDescription(e.currentTarget.value),
                                                                                        "rows": 5,
                                                                                        "placeholder": "What did you expect? What actually happened?",
                                                                                        "style": "padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;font-family:inherit;resize:vertical"
                                                                                    })
                                                                                ];
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
                                                                            "style": "display:flex;justify-content:flex-end;gap:8px;margin-top:4px",
                                                                            get children() {
                                                                                return [
                                                                                    el("button", {
                                                                                        "class": "btn btn-ghost",
                                                                                        "type": "button",
                                                                                        get "onClick"() {
                                                                                            return close;
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
                                                                                        "type": "submit",
                                                                                        get "disabled"() {
                                                                                            return busy() || !title().trim();
                                                                                        },
                                                                                        get children() {
                                                                                            return live(() => busy() ? "Submitting…" : "Submit report");
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
