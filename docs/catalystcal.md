# Optional catalystcal integration

Longhouse uses catalystcal Go client v0.3.0. It supports the two required
consent scopes, durable create and split receipts, and moved occurrence
lookup. The integration is optional. Squizzlezig is the validation environment.
Other deployments remain disabled until their operators configure them.

## Local assignments

An admin can propose an event for an active member of the same house. The
proposal stays in Longhouse. Only the assigned member can accept or decline
it. A stored calendar grant is not acceptance.

Open an item in **Assignments and local events** to review its saved details.
Save changes before accepting. Acceptance names the revision that the owner
reviewed. An edit makes an old acceptance request invalid.

With catalystcal off, acceptance adds the event to the local calendar. With
catalystcal on, acceptance creates the event with the owner's act-as bearer.
The application bearer does not create live events for members.

Longhouse stores acceptance, the creation key, and the exact request before
it sends a remote create. During that call, edits and cancellation are
blocked. An uncertain result stays locked. The owner can select **Retry
publication** to recover the saved request. Unsaved form changes do not
change that request.

Recovery reads the durable receipt. If no receipt is available, it calls
`create-event-once` with the same key and content. It never calls the ordinary
create operation. A lost response, a restart, or a failed local save does
not cause a second event. An authorization failure keeps the revision locked.
Reconnect if necessary, then retry.

A receipt for an event in trash, a deleted event, or an inaccessible event
keeps the original local link and shows a notice. Recovery does not restore
or create that event. A move to another calendar does not expose its content
to the original house. Restore from calendar trash when applicable.

An uncertain publication from a version without the request journal needs
operator review. Longhouse refuses to retry it automatically.

## Application keys

Generate a new key file outside the repository. The command creates three
Ed25519 keys. It uses mode 0600 and refuses to replace an existing file.

```sh
longhouse calendar-keys --action=generate --file=/secure/longhouse-keys.json \
  --domain=example.org --user-id=ACCOUNT_UUID --instance-id=production
```

Enroll the instance at its home domain. Put the account owner's API key in a
separate protected file. Do not put that key in a command argument.

```sh
longhouse calendar-keys --action=enroll --file=/secure/longhouse-keys.json \
  --tcp-addr=HOME_HOST:PORT --fingerprints=HOME_TLS_FINGERPRINT \
  --account-api-key-file=/secure/home-account-api-key
```

The regular-RP SDK makes the possession proofs and signed requests. Private
keys do not leave Longhouse. Enrollment uses the account API key. Attestation
renewal uses the application's own signatures. The server renews every six
hours. A manual renewal uses:

```sh
longhouse calendar-keys --action=renew --file=/secure/longhouse-keys.json \
  --tcp-addr=HOME_HOST:PORT --fingerprints=HOME_TLS_FINGERPRINT
```

The signing keys have a requested life of one year. The home domain can
reduce that life. Attestation renewal does not extend key life. Rotate keys
before their expiry through LinkKeys application-key management.

## Deployment settings

Keep `catalystcal.enabled: false` until the application keys and access are
configured. Store the key file in an existing Kubernetes Secret. Set:

| Helm value | Purpose |
| --- | --- |
| `catalystcal.enabled` | Mount keys and configure the service |
| `catalystcal.url` | catalystcal HTTPS base URL |
| `catalystcal.callbackUrl` | Longhouse URL ending in `/calendar` |
| `catalystcal.existingSecret` | Secret with the private key file |
| `catalystcal.keysKey` | Secret data key, default `keys.json` |
| `catalystcal.homeTcpAddr` | Application home-domain TCP endpoint |
| `catalystcal.homeFingerprints` | Pinned home-domain TLS fingerprints |

The client also requires the existing LinkKeys RP TCP address, pinned
fingerprints, RP API key, IDP domain, and IDP browser URL. The RP and the
application home domain can be different servers. Consent currently supports
the configured IDP domain.

Direct API deployments use `LONGHOUSE_CATALYSTCAL_URL`,
`LONGHOUSE_CATALYSTCAL_CALLBACK_URL`, `LONGHOUSE_CATALYSTCAL_KEYS_FILE`,
`LONGHOUSE_CALENDAR_HOME_TCP_ADDR`, and
`LONGHOUSE_CALENDAR_HOME_FINGERPRINTS`.

The catalystcal operator must enable act-as and allow the full Longhouse
application identity: `ACCOUNT_UUID@DOMAIN/APPLICATION_ID`. The application
name alone does not identify an account.

## House setup and access

An admin reviews the migration report before enabling a house. This creates
a durable setup job. The job creates an application-owned calendar, a member
group with `contribute`, and an admin group with `write`.

Membership and role changes update a durable sync counter in the same
database transaction. Workers use a database lock for each house and send the
current member lists with the remote group revision. Failed updates retry.
An hourly repair reads the full current membership. Delays over five minutes
produce an error log and appear in calendar status.

Longhouse checks current membership before calendar access. Direct access to
catalystcal changes after group sync succeeds. Deleting a house leaves a
cleanup job for its remote calendar and groups.

Members can read without consent. Writes require **Connect your calendar**.
The callback nonce is bound to the current member, expires after ten minutes,
and can be used once. The fetched grant must name that member, Longhouse as
grantee, and catalystcal as audience.

Each write checks the grant through LinkKeys. Longhouse reuses a short
catalystcal bearer to reduce credential exchanges. A revoked or expired
grant blocks writes. A temporary rate limit blocks that attempt but does
not mark the saved connection as broken.

## Existing events and recovery

`calendar preview-migration` is a read-only report. It includes pending and
declined assignments, uncertain publications, unscheduled items, and deleted
source rows. Deleted source rows stay in the backup and are not published. House admins
can restore local source rows through Longhouse trash. Those rows return to
the local review list and still need owner acceptance.
Enabling a house copies
every source event to `calendar_legacy_backup`. Existing rows stay local for
owner review. The old owner field does not prove who created an event.

An accepted legacy row becomes one remote event. Longhouse does not infer
missing overrides, deleted instances, or a recurrence rule. Review the future
schedule separately. Unscheduled items stay local until their owner supplies
a start and end time.

Published events retain their local UUID in `calendar_refs`. Comments and
links use that UUID. If a read finds an event before publication recovery,
both local references stay valid. The remote service stores the event body.
After receipt recovery succeeds, Longhouse removes the saved request body.

Occurrence links use the series ID and the original recurrence ID.
`get-occurrence` resolves the current times, including a moved instance.
All-day dates are converted to midnight in the event time zone for display.
An excluded, deleted, or inaccessible occurrence returns 404.

Calendar view preferences remain local. Remote trash and restore use the
member's grant.

To split a series, open an occurrence and select **This and future
occurrences**. Longhouse stores the exact edit request before it sends it.
It reads all reference mappings from the durable receipt before it changes
any local reference. Earlier links stay on the original series. Future
links move to the successor series. An occurrence with no match keeps its
local link and comments and shows a notice. It cannot be edited or deleted
as another occurrence.

If the result is uncertain, the series stays locked. The member who saved
the edit can select **Retry saved edit**. The retry uses the saved request,
not changes in the form. Recovery also works after a restart or failed local
save. It does not split again or restore a deleted series.

Keep the source backup for at least 90 days after migration approval. There is
no automatic source purge. Export ICS from catalystcal and back up the local
database before any later migration. A house with catalystcal enabled cannot
be switched back by a flag; that would hide published events.

## CLI

The generated API exposes `event accept`, `event decline`, `event local`,
`event recover-edit`,
`calendar status`, `calendar configure`, `calendar connect`,
`calendar complete-connect`, `calendar disconnect`,
`calendar preview-migration`, `calendar trash`, and `calendar restore`.
Use `--help` on an operation for its fields. Acceptance and decline require
the event ID and current revision. Remote event lists require both date
range bounds. Reads split at the remote range limit and split again when a
response is truncated.

CLI event-name lookup includes local proposals. For a remote calendar it
searches from two years before today to two years after today. Use the stable
event ID for an event outside that range.

## Squizzlezig validation

Validated on 2026-11-08 UTC with catalystcal client v0.3.0. Longhouse runs
`dev-20261108-002700-0cc8985-dirty` in namespace `longhouse-squizzlezig`.
The application identity is
`01a09813-25af-7010-b69b-03bc4e2782d2@squizzlezig.com/longhouse`, instance
`longhouse-squizzlezig-1`. Secret `longhouse-calendar` stores `keys.json`.
The catalystcal squizzlezig allow-list contains this identity.

The live checks passed for two-scope consent, owner acceptance, legacy
publication, moved links, series splits, unmatched links with kept comments,
trash and restore, grant revocation, current membership checks, group sync,
and house cleanup. The browser showed 67 events in one month. An all-day
Denver event across the spring clock change kept its dates and 23-hour span.
A deleted legacy event restored locally for review after the house switch.

Temporary test houses, their remote calendars and groups, and test grants
were removed. The enrolled application and deployment configuration remain.

## Next Actions

Review the implementation and the migration report before the PR. Run CI
after the PR is created. Configure application keys and access separately
for each deployment before enabling its houses.

## Project category deadlines

Each enabled house has one application-owned deadline calendar for each
project category. The calendar name starts with the category label and
includes a stable house identifier. Categories are case-sensitive. Leading
and trailing spaces are removed. A blank category shows as `Uncategorized`.
An explicit category named `Uncategorized` remains a separate category.

Longhouse owns task content, deadlines, project categories, and access rules.
Catalystcal stores the calendar entries. These entries use the Longhouse
application identity. They need no member act-as grant. Personal event writes
continue to use the member's grant and the existing acceptance process.

A task with a deadline gets one entry in each of its project categories.
Several projects in the same category share that entry. Subtasks and spawned
recurring tasks inherit their project categories. A recurring task produces
entries for its stored occurrences; it does not create a second recurrence
rule in Catalystcal. Completed and cancelled tasks keep their deadline entries
with a status prefix in the remote title.

Each entry starts at the deadline and lasts one minute. Catalystcal stores
whole seconds, so subsecond precision is removed. Each entry is
transparent to free/busy checks. It has no attendees, reminders, or invitations.
Edit the task in Longhouse to change its deadline. An entry does not copy the
task description. Tasks without deadlines have no entry.

Open a project to see its **Deadline calendar**. Use the month buttons to
change the date range. Times use the browser's local time zone. This view
reads Catalystcal and includes only the current project's tasks that the
caller can read. The view reports pending updates and failed requests. It
does not show an old deadline while a date change waits for synchronization.

Category calendars have no member or group shares. This prevents a shared
category from exposing a private project. Longhouse checks current membership,
roles, project access, and task access before it returns deadline data. Do not
add direct shares to these calendars. Use Longhouse for filtered access.

Database migration 26 adds durable update jobs, category mappings, and saved
create requests. Task, project, and project-task changes queue work in the
same database transaction. The worker checks every 15 seconds. It retries a
failed job after one minute and checks each enabled house at least hourly.
A task save does not wait for Catalystcal.

The worker saves the exact create request before it sends it. After a lost
response or restart, it reads the durable create receipt. If no receipt
exists, it uses the same request and key. A newer task edit is applied only
after that recovery. Updates and deletes can be retried. A concurrent source
change keeps the job pending for another pass.

Removing a deadline, deleting a task or project, or removing its last category
link removes the corresponding entry. Restore or a new deadline creates a
new entry. Empty category calendars remain for reuse. House deletion removes
all category calendars, including a calendar whose create response was lost.
Cleanup retries until it succeeds.

A moved, deleted, or inaccessible event found during create recovery requires
operator review. Recovery does not restore it or create another event. Keep
application credentials stable during recovery. The existing application key
configuration also supplies these calendars; no new secret is required.

Deploy the API and web images together. Existing enabled houses receive the
new jobs when migration 26 runs. Other houses must first enable the existing
Catalystcal integration. The generated API operation is
`CalendarService.list-project-deadlines`. Its date range must be greater than
zero and no longer than 93 days.
