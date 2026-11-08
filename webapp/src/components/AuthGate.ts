import { When, type Content, live, el } from "~/ui/dom";
import { useCurrentHouseId, useHousesLoaded, useRestoring, useSession } from "~/stores/auth";
/**
 * Wraps a page's body. When the caller has no session, renders a
 * "you're not signed in" CTA card (link to /dev-login in dev, to the
 * api's /auth/start in prod — same rel="external" trick so the router
 * lets the browser hit the api). When signed in but no house is yet
 * selected, shows a short status while loadHouses fills in — or, once
 * loadHouses has confirmed the caller is in zero houses, a "talk to an
 * admin about trusting your domain" explainer instead of a forever-spinner.
 *
 * Pages stay simple: they wrap their content in <AuthGate> and the
 * fallback handling lives once, here.
 */
export const AuthGate = (props: {
    children: Content;
}) => {
    const session = useSession();
    const houseId = useCurrentHouseId();
    const housesLoaded = useHousesLoaded();
    const restoring = useRestoring();
    return (When({
        get "when"() {
            return session();
        },
        get "fallback"() {
            return restoring() ? StatusCard({
                "text": "Signing you in\u2026"
            }) : SignedOutCTA();
        },
        get children() {
            return When({
                get "when"() {
                    return houseId();
                },
                get "fallback"() {
                    return housesLoaded() ? NoHouseCTA() : StatusCard({
                        "text": "Loading your houses\u2026"
                    });
                },
                get children() {
                    return live(() => props.children);
                }
            });
        }
    }));
};
const signInHref = import.meta.env.DEV ? "/dev-login" : "/api/v1/auth/start";
const signInRel = import.meta.env.DEV ? undefined : "external";
const SignedOutCTA = () => (el("section", {
    "class": "reveal d1",
    "style": "margin-top:48px;padding:48px 28px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low);text-align:center;max-width:560px;margin-left:auto;margin-right:auto",
    get children() {
        return [
            el("h2", {
                "style": "font-family:var(--display);font-variation-settings:'opsz' 80, 'SOFT' 100, 'wght' 420;font-size:32px;color:var(--grass-4);margin:0 0 12px",
                get children() {
                    return "You're not signed in";
                }
            }),
            el("p", {
                "style": "font-size:15px;color:var(--ink-mute);max-width:46ch;margin:0 auto 24px",
                get children() {
                    return "Sign in to see your house's tasks, calendar, projects, and members.";
                }
            }),
            el("a", {
                get "href"() {
                    return signInHref;
                },
                get "rel"() {
                    return signInRel;
                },
                "class": "btn btn-primary",
                "style": "padding:10px 22px",
                get children() {
                    return "Sign in";
                }
            })
        ];
    }
}));
const StatusCard = (props: {
    text: string;
}) => (el("section", {
    "class": "reveal d1",
    "style": "margin-top:48px;padding:32px 24px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);text-align:center;max-width:520px;margin-left:auto;margin-right:auto",
    get children() {
        return el("p", {
            "style": "margin:0;color:var(--ink-mute)",
            get children() {
                return live(() => props.text);
            }
        });
    }
}));
const NoHouseCTA = () => {
    const session = useSession();
    const domain = () => session()?.domain;
    return (el("section", {
        "class": "reveal d1",
        "style": "margin-top:48px;padding:48px 28px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low);text-align:center;max-width:560px;margin-left:auto;margin-right:auto",
        get children() {
            return [
                el("h2", {
                    "style": "font-family:var(--display);font-variation-settings:'opsz' 80, 'SOFT' 100, 'wght' 420;font-size:32px;color:var(--grass-4);margin:0 0 12px",
                    get children() {
                        return "You're not in a house yet";
                    }
                }),
                el("p", {
                    "style": "font-size:15px;color:var(--ink-mute);max-width:48ch;margin:0 auto 16px",
                    get children() {
                        return "You're signed in, but your account isn't a member of any house, so there's nothing to show yet.";
                    }
                }),
                el("p", {
                    "style": "font-size:15px;color:var(--ink-mute);max-width:48ch;margin:0 auto",
                    get children() {
                        return [
                            "Ask an administrator to add you \u2014 either by trusting your domain",
                            When({
                                get "when"() {
                                    return domain();
                                },
                                get children() {
                                    return [
                                        live(() => " "),
                                        "(",
                                        el("strong", {
                                            "style": "color:var(--ink)",
                                            get children() {
                                                return live(() => domain());
                                            }
                                        }),
                                        ")"
                                    ];
                                }
                            }),
                            live(() => " "),
                            "so anyone from it joins automatically, or by inviting your account directly."
                        ];
                    }
                })
            ];
        }
    }));
};
