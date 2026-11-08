import { query, cell, observe, cleanup } from "~/ui/state";
import { Repeat, When, live, el } from "~/ui/dom";
import type { Event as ApiEvent } from "@longhouse/client";
import { calendarClient, eventClient } from "~/data/clients";
import { hasRole } from "~/stores/auth";
export const CalendarConnection = (props: {
    houseId: string;
    refresh: number;
    onEdit: (event: ApiEvent) => void;
    onEnabled: (enabled: boolean) => void;
}) => {
    const [status, { refetch }] = query(() => [props.houseId, props.refresh] as const, async ([id]) => {
        const result = await calendarClient.getStatus(id);
        props.onEnabled(result.enabled);
        return result;
    });
    observe(() => {
        const current = status();
        if (current?.enabled && (!current.ready || current.syncPendingSince)) {
            const timer = window.setInterval(() => { void refetch(); }, 5000);
            cleanup(() => window.clearInterval(timer));
        }
    });
    const [local] = query(() => [props.houseId, props.refresh] as const, ([id]) => eventClient.listLocalEvents(id));
    const [error, setError] = cell("");
    const [busy, setBusy] = cell(false);
    const [preview, setPreview] = cell<Awaited<ReturnType<typeof calendarClient.previewMigration>>>();
    const [trash, setTrash] = cell<Awaited<ReturnType<typeof calendarClient.listTrash>>>();
    const run = async (action: () => Promise<unknown>) => {
        setBusy(true);
        setError("");
        try {
            await action();
            await refetch();
        }
        catch (e) {
            setError(e instanceof Error ? e.message : String(e));
        }
        finally {
            setBusy(false);
        }
    };
    const connect = () => run(async () => {
        const result = await calendarClient.beginConnect(props.houseId);
        sessionStorage.setItem("calendar-connect-house", props.houseId);
        window.location.assign(result.url);
    });
    return el("section", {
        "aria-label": "Calendar connection and assignments",
        "style": "margin-bottom:16px",
        get children() {
            return [
                When({
                    get "when"() {
                        return status.error || local.error || error();
                    },
                    get children() {
                        return el("p", {
                            "role": "alert",
                            get children() {
                                return live(() => error() || String(status.error || local.error));
                            }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return status()?.available;
                    },
                    get children() {
                        return When({
                            get "when"() {
                                return status()?.enabled;
                            },
                            get "fallback"() {
                                return When({
                                    get "when"() {
                                        return hasRole("admin");
                                    },
                                    get children() {
                                        return [
                                            el("button", {
                                                "class": "btn btn-ghost",
                                                get "disabled"() {
                                                    return busy();
                                                },
                                                "onClick": () => run(async () => setPreview(await calendarClient.previewMigration(props.houseId))),
                                                get children() {
                                                    return "Review catalystcal setup";
                                                }
                                            }),
                                            When({
                                                get "when"() {
                                                    return preview();
                                                },
                                                children: rows => el("div", {
                                                    get children() {
                                                        return [
                                                            el("p", {
                                                                get children() {
                                                                    return "Existing events stay in Longhouse until their owners review and accept them. Stored recurring occurrences need separate review.";
                                                                }
                                                            }),
                                                            el("p", {
                                                                get children() {
                                                                    return [
                                                                        live(() => rows().length),
                                                                        " existing events need review."
                                                                    ];
                                                                }
                                                            }),
                                                            el("details", {
                                                                get children() {
                                                                    return [
                                                                        el("summary", {
                                                                            get children() {
                                                                                return "Migration report";
                                                                            }
                                                                        }),
                                                                        el("ul", {
                                                                            get children() {
                                                                                return Repeat({
                                                                                    get "each"() {
                                                                                        return rows();
                                                                                    },
                                                                                    children: row => el("li", {
                                                                                        get children() {
                                                                                            return [
                                                                                                live(() => row.title),
                                                                                                ": ",
                                                                                                live(() => row.reason)
                                                                                            ];
                                                                                        }
                                                                                    })
                                                                                });
                                                                            }
                                                                        })
                                                                    ];
                                                                }
                                                            }),
                                                            el("button", {
                                                                "class": "btn btn-primary",
                                                                get "disabled"() {
                                                                    return busy();
                                                                },
                                                                "onClick": () => run(async () => {
                                                                    await calendarClient.configureCalendar({ houseId: props.houseId, enabled: true });
                                                                    setPreview(undefined);
                                                                }),
                                                                get children() {
                                                                    return "Enable catalystcal for this house";
                                                                }
                                                            })
                                                        ];
                                                    }
                                                })
                                            })
                                        ];
                                    }
                                });
                            },
                            get children() {
                                return [
                                    el("p", {
                                        get children() {
                                            return live(() => status()?.ready ? "This house uses catalystcal." : "Calendar setup is pending.");
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return status()?.lastError;
                                        },
                                        get children() {
                                            return el("p", {
                                                "role": "alert",
                                                get children() {
                                                    return live(() => status()?.lastError);
                                                }
                                            });
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return status()?.syncPendingSince;
                                        },
                                        get children() {
                                            return el("p", {
                                                get children() {
                                                    return "Calendar access changes are waiting to sync.";
                                                }
                                            });
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return status()?.ready;
                                        },
                                        get children() {
                                            return When({
                                                get "when"() {
                                                    return status()?.connected;
                                                },
                                                get "fallback"() {
                                                    return el("button", {
                                                        "class": "btn btn-primary",
                                                        get "disabled"() {
                                                            return busy();
                                                        },
                                                        get "onClick"() {
                                                            return connect;
                                                        },
                                                        get children() {
                                                            return "Connect your calendar";
                                                        }
                                                    });
                                                },
                                                get children() {
                                                    return [
                                                        el("button", {
                                                            "class": "btn btn-ghost",
                                                            get "disabled"() {
                                                                return busy();
                                                            },
                                                            "onClick": () => run(() => calendarClient.disconnect(props.houseId)),
                                                            get children() {
                                                                return "Disconnect calendar";
                                                            }
                                                        }),
                                                        el("button", {
                                                            "class": "btn btn-ghost",
                                                            get "disabled"() {
                                                                return busy();
                                                            },
                                                            "onClick": () => run(async () => setTrash(await calendarClient.listTrash(props.houseId))),
                                                            get children() {
                                                                return "Calendar trash";
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
                When({
                    get "when"() {
                        return trash();
                    },
                    get children() {
                        return el("ul", {
                            get children() {
                                return Repeat({
                                    get "each"() {
                                        return trash();
                                    },
                                    children: entry => el("li", {
                                        get children() {
                                            return [
                                                live(() => entry.event.title),
                                                " ",
                                                el("button", {
                                                    get "disabled"() {
                                                        return busy();
                                                    },
                                                    "onClick": () => run(async () => {
                                                        const restored = await calendarClient.restoreEvent({ houseId: props.houseId, trashId: entry.trashId });
                                                        setTrash(await calendarClient.listTrash(props.houseId));
                                                        props.onEdit(restored);
                                                    }),
                                                    get children() {
                                                        return "Restore";
                                                    }
                                                })
                                            ];
                                        }
                                    })
                                });
                            }
                        });
                    }
                }),
                When({
                    get "when"() {
                        return (local()?.length ?? 0) > 0;
                    },
                    get children() {
                        return [
                            el("h3", {
                                get children() {
                                    return "Assignments and local events";
                                }
                            }),
                            el("p", {
                                get children() {
                                    return "These items stay in Longhouse. Review an assignment before you accept it.";
                                }
                            }),
                            el("ul", {
                                get children() {
                                    return Repeat({
                                        get "each"() {
                                            return local();
                                        },
                                        children: event => el("li", {
                                            get children() {
                                                return [
                                                    el("button", {
                                                        "class": "btn btn-ghost",
                                                        "onClick": () => props.onEdit(event),
                                                        get children() {
                                                            return live(() => event.title);
                                                        }
                                                    }),
                                                    el("span", {
                                                        get children() {
                                                            return live(() => event.editPendingMemberId ? "Calendar edit needs recovery" : event.calendarUnbound ? "Occurrence no longer exists" : event.calendarState === "publishing" ? "Publication needs recovery" : event.calendarState === "remote" ? "Published event unavailable" : event.calendarState === "declined" ? "Declined" : event.startsAt ? "Needs review" : "Unscheduled");
                                                        }
                                                    })
                                                ];
                                            }
                                        })
                                    });
                                }
                            })
                        ];
                    }
                })
            ];
        }
    });
};
