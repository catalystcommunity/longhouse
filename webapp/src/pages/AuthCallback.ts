import { cell, afterMount } from "~/ui/state";
import { When, el, live } from "~/ui/dom";
import { navigation, searchState } from "~/ui/router";
/**
 * /auth/callback — the route the linkkeys IDP redirects the browser to after
 * authentication. It carries the sealed `encrypted_token` in the query. We
 * pass it to AuthService.Complete (the api decrypts + verifies, returns our
 * session token), stash the session, then redirect into the app. The session
 * token never travels in a URL — only the sealed IDP token does, and that's
 * safe (it's encrypted IDP↔RP).
 */
import { authClient } from "~/data/clients";
import { finishLogin, takeAuthReturnTo } from "~/lib/session";
export const AuthCallback = () => {
    const [params] = searchState();
    const navigate = navigation();
    const [error, setError] = cell<string | null>(null);
    afterMount(async () => {
        const token = params.encrypted_token;
        if (typeof token !== "string" || token === "") {
            setError("The identity provider didn't return a token.");
            return;
        }
        try {
            const resp = await authClient.complete({ encryptedToken: token });
            await finishLogin(resp);
            navigate(takeAuthReturnTo(), { replace: true });
        }
        catch (e) {
            setError(e instanceof Error ? e.message : String(e));
        }
    });
    return (el("section", {
        "class": "auth-callback reveal d1",
        get children() {
            return When({
                get "when"() {
                    return error();
                },
                get "fallback"() {
                    return el("div", {
                        "class": "auth-callback-card",
                        get children() {
                            return [
                                el("div", {
                                    "class": "spinner",
                                    "aria-hidden": "true"
                                }),
                                el("p", {
                                    get children() {
                                        return "Signing you in\u2026";
                                    }
                                })
                            ];
                        }
                    });
                },
                children: (message) => (el("div", {
                    "class": "auth-callback-card",
                    get children() {
                        return [
                            el("h1", {
                                get children() {
                                    return "Sign-in failed";
                                }
                            }),
                            el("p", {
                                "class": "err",
                                get children() {
                                    return live(() => message());
                                }
                            }),
                            el("a", {
                                "class": "btn btn-primary",
                                "href": "/api/v1/auth/start",
                                "rel": "external",
                                get children() {
                                    return "Try again";
                                }
                            })
                        ];
                    }
                }))
            });
        }
    }));
};
