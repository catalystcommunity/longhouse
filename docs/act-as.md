# LinkKeys act-as access

An application can act for a Longhouse user with a LinkKeys grant. The user
approves the application and its scopes on the LinkKeys consent page. The
application does not need a Longhouse API token that represents the user.

Longhouse supports application grantees and approved LinkKeys local-RP
grantees. Longhouse is the audience. It must have an enrolled LinkKeys
application identity.

## Access rules

The application gets the intersection of the approved scopes and the user's
current Longhouse permissions. A grant does not create a house membership.
Longhouse reads current memberships and roles on each delegated request.
Resource permissions remain in force.

Use `longhouse.full` to permit all supported Longhouse actions. This includes
house administration, sharing, and deletion when the user has permission.
An application cannot use a delegated bearer to create an independent browser
or CLI session. It must continue to use the LinkKeys grant.

For less access, select scopes from this table. Read and write scopes are
separate. Select both when the application needs both.

| Scope prefix | Permitted service data and actions |
| --- | --- |
| `tasks` | Tasks, personal tasks, task access grants, bug reports |
| `projects` | Projects, milestones, project tasks, members, owners, access grants |
| `dependencies` | Task and project dependencies |
| `calendar` | Events, invitations, calendar views, connections, project deadlines, event trash |
| `comments` | Comments on resources |
| `notifications` | Notifications and read status |
| `houses` | House records |
| `members` | Member profiles, avatars, activation status |
| `groups` | Groups and group members |
| `skills` | Skills and skill assignments |
| `roles` | Roles and role assignments |
| `trust` | Trusted identity domains |
| `sharing` | External resource sharing |
| `settings` | House settings |
| `trash` | Deleted resources, restore, permanent deletion |
| `sessions` | Session list, session revocation, CLI request denial |

Each prefix supports `.read` and `.write`. Two additional scopes,
`identity.read` and `audit.read`, permit identity and audit reads. Every usable
grant can call `AuthService.me` to identify the user and available houses.
Scope descriptions are included in the signed scope set for user consent.

Scopes apply to service operations. For example, `projects.read` includes
project task lists and project members. It does not permit `TaskService` or
`MemberService` calls. Existing resource checks still apply to each result.
The explicit operation list is in `api/internal/auth/delegation.go`. New
operations require a policy entry before delegated callers can use them.

## Application flow

Use the generated Longhouse client and a LinkKeys SDK. Send Longhouse RPCs
to `POST /api/csil/v1/rpc` with `Content-Type: application/cbor`.

1. Call the public `AuthService.get-act-as-scope-set` operation. Set
   `grantee_domain`, `grantee_user_id`, and `grantee_application_id` for an
   application grantee. For a local RP, set only `local_rp_fingerprint`.
   Set `scopes` to the requested nonempty list. To manage tasks and projects,
   request `tasks.read`, `tasks.write`, `projects.read`, and `projects.write`.
   Request `longhouse.full` only when full access is needed.
2. Decode `signed_scope_set` as LinkKeys `SignedActAsScopeSet`. Verify its
   audience and signatures with the LinkKeys SDK. Use the audience identity
   in this set for the following steps.
3. Sign an `ActAsGrantRequest` with the grantee's key. Include the scope set,
   a callback URL, a random nonce, and the requested grant lifetime. Send the
   user to the home domain's LinkKeys act-as consent page. The Go SDK provides
   `SignGrantRequest` and `GrantRequestURL`.
4. At the callback, check the nonce and reject duplicate query parameters.
   The callback gives `act_as_grant_id`, not the grant bytes. Use the SDK's
   signed refresh request to obtain the grant. A regular RP application uses
   its own RP for this call. A local RP uses its SDK's home-domain transport.
5. Create a fresh signed `ActAsPresentation`. Its request digest must be the
   SHA-256 hash of these exact UTF-8 bytes, with no newline:

   ```text
   longhouse/AuthService/exchange-act-as-credential
   ```

6. Encode the grant and presentation as LinkKeys `ActAsCredential`. Call
   the public `AuthService.exchange-act-as-credential` operation with those
   CBOR bytes in `credential`.
7. Use the returned `token` in `Authorization: Bearer <token>`, or in the
   CSIL-RPC envelope's `auth` field. The response also gives the approved
   scopes, grant ID, grantee identity, user identity, and expiry time.
8. Before the bearer expires, exchange a new presentation with a new random
   nonce. Renew the grant through LinkKeys when necessary. Do not call
   `AuthService.refresh` or a session refresh operation with this bearer.

The bearer lifetime is at most five minutes, the configured Longhouse bearer
lifetime, and the remaining grant lifetime. Longhouse stores presentation
nonces in PostgreSQL. A presentation cannot be used again after an API restart
or through another API replica.

Longhouse checks grant revocation through its RP on every delegated request,
including avatar requests. A failed revocation lookup refuses the request.
The RP controls the freshness of its remote data. Mutations record the user,
grant ID, grantee identity, and approved scopes in the audit log. Revoke the
grant on the user's LinkKeys account page to stop the application's access.

## Server setup

The todandlorna and catalystsquad release jobs use their existing LinkKeys
accounts. Application signing keys were enrolled under those accounts on
2026-11-09. Each deployment has three distinct Ed25519 keys.

| Deployment | Existing account | Reactorcide secret reference |
| --- | --- | --- |
| todandlorna | Longhouse App (`longhouse`) | `tnl/longhouse:application_keys` |
| catalystsquad | Longhouse webapp (`longhouse-web`) | `catalystsquad/longhouse:application_keys` |

Both the local store and `https://reactorcide.catalystsquad.com` hold these
secrets. The existing per-job grants already permit each deployment to read
its own secret path. No broader grant is required.

On the next release, each job passes its key file as
`LONGHOUSE_APPLICATION_KEYS`. The deployment script validates the file and
sends it through stdin to Kubernetes. It creates or updates the
`longhouse-application-keys` Secret before Helm runs. Each values file enables
act-as and mounts `keys.json` from that Secret. The key-file checksum changes
the pod template when keys change. No private key is stored in Git or Helm
values.

The server uses its existing RP API key, TCP configuration, and DNS discovery.
The act-as adapter does not require separate RP addresses or fingerprints.
It requires pinned TLS even if browser login permits an insecure development
connection. It renews application attestations at startup and every six hours.
Renewal uses application signatures and does not need the account API key.

The existing `Longhouse Release Tag` workflow creates a release after a pull
request is merged into `main`. The tag workflow builds the images, then runs
both deployment jobs. Migration 27 is applied by normal API startup.

### Repeat provisioning or repair a missing secret copy

Build the current Longhouse binary, then run:

```sh
go -C api build -o /tmp/longhouse-key-setup .
python3 scripts/provision-application-keys.py --binary /tmp/longhouse-key-setup
```

Use `--env todandlorna` or `--env catalystsquad` to select one deployment.
The helper reads the existing account API key from the local Reactorcide
store. It confirms the account identity, reuses stored keys, and copies a
missing secret to the other store. If neither store has keys, it generates
them and saves both copies before enrollment. A retry reuses those copies.
The helper refuses different local and remote key sets. It also refuses to
reset an enrolled instance whose public keys differ.

The helper reads `~/.reactorcide-pass` and obtains the coordinator credential
from `reactorcide/k8s:api_token`. It sends secret values through stdin or
protected temporary files. It does not print them. It accepts optional
`--todandlorna-tcp-addr` and `--catalystsquad-tcp-addr` arguments for a local
port-forward. TLS fingerprints still come from domain DNS.

### Other deployments

Use `longhouse application-keys` to generate and enroll keys for an existing
LinkKeys account. This command is an alias of `calendar-keys`.

```sh
longhouse application-keys --action=generate --file=/secure/longhouse-keys.json \
  --domain=example.com --user-id=OWNER_UUID \
  --application-id=longhouse --instance-id=longhouse-production

longhouse application-keys --action=ensure --file=/secure/longhouse-keys.json \
  --account-api-key-file=/secure/owner-api-key
```

`ensure` enrolls only a missing instance. For an existing instance, it checks
that the public keys match, then renews their attestations. It does not reset
the instance. Optional `--tcp-addr` and `--fingerprints` arguments override
home-domain DNS discovery.

Store the enrolled file in a Kubernetes Secret under `keys.json`. Set:

```yaml
actAs:
  enabled: true
  existingSecret: longhouse-application-keys
  keysKey: keys.json
```

For a deployment without Helm, set `LONGHOUSE_ACT_AS_KEYS_FILE` to the enrolled
key file. Use the existing `LONGHOUSE_LINKKEYS_IDP_DOMAIN` and
`LONGHOUSE_LINKKEYS_PKI_API_KEY` settings. The normal TCP address and
fingerprint overrides remain available. An absent key-file setting disables
delegation. Invalid configured keys or transport settings stop startup.

Keep the application identity stable across deployments so existing grants
continue to name the same audience. Private-key rotation is separate from
attestation renewal.

## Validation

The tests cover LinkKeys signature vectors, scope enforcement, session
escalation rejection, grant expiry, replay rejection, revocation failures,
current memberships, and application audit attribution. The PostgreSQL test
checks concurrent nonce use and persistence across store instances.

Live account checks, enrollment, public-key comparison, secret read-back, and
renewal passed for both domains. A complete live act-as check still needs the
new API deployment and a consenting LinkKeys user. Request a narrow grant, create a task and a project,
then revoke the grant and confirm that the same bearer fails.
