# MSG91 transactional SMS

The Go marketplace sends approved MSG91 templates from the durable worker.
SMS, email, in-app notifications, payment and shipment states remain separate.
No promotional messages, OTP login or international-routing workaround is added.

## Account readiness

The owner deferred MSG91 on 16 November 2026. Keep it disabled; no additional
account verification, key provisioning or SMS activation is currently requested.

The account inspected on 15 November 2026 showed incomplete KYC, a ₹50 balance,
and no SMS sender IDs or templates. On 16 November the owner asked to prepare
integration while approvals remain outstanding. The accompanying
[template drafts](msg91-sms-templates.draft.json) are local drafts; no provider or
DLT approval is implied.

MSG91 requires KYC for actual content from a live account and approved sender,
entity and content mappings for Indian SMS. See [account activation](https://msg91.com/help/text-sms/my-account-status-is-in-demo-what-can-i-do-to-make-my-account-live),
[template setup](https://msg91.com/help/template/how-to-create-flow-id-to-send-sms-via-api)
and [DLT requirements](https://msg91.com/help/dlt-registration-in-india/dlt-faqs).
An API success in demo mode does not prove delivery of a WeevCrafts transaction.

Production Coolify settings are staged with SMS, KYC approval and DLT approval
flags false. No live SMS, account verification, paid recharge or DLT registration
was performed. The new application/migration must be deployed before activation.

## Events and contacts

| Template key | Recipient and events |
| --- | --- |
| `customer_order` | Current customer: order receipt, captured-payment confirmation, collection, out for delivery, delivery, NDR/RTO, cancellation, processed refund, exceptional-return changes |
| `seller_order` | Current seller owner: new paid order, acceptance reminder, cancellation, processed refund, exceptional return changes, NDR/RTO |
| `customer_support` | Current customer: company support replies |
| `seller_support` | Current seller owner: company support replies and private seller coordination |

Only the account's own `users.phone` is used. A delivery-address contact may be
a gift recipient, so it is not a fallback. Missing or invalid Indian mobiles are
skipped. Accounts must be active with exactly one current customer/seller role.
Order notices honor `notification_preferences.order_updates`; support notices
honor `maker_messages`. Eligibility is rechecked at dispatch.

SMS includes only a reference and short status label. Private complaint text,
return reasons, names, addresses, bank details, invoice links and credentials
are excluded. The drafts use no dynamic URL or link shortening.

## Configuration

Privately configure these only with actual account approvals and IDs:

```text
WECRATFS_SMS_ENABLED=false
MSG91_AUTHKEY=<dedicated server key>
MSG91_AUTHKEY_FILE=<optional private server credential file instead>
MSG91_KYC_VERIFIED=true
MSG91_DLT_VERIFIED=true
MSG91_SENDER_ID=<approved six-letter uppercase header>
MSG91_ENTITY_ID=<approved 19-digit principal-entity ID>
MSG91_SMS_TEMPLATES=<JSON mapping described below>
```

`MSG91_SMS_TEMPLATES` maps all four keys above to objects with `template_id`
(24-character MSG91 ID), `dlt_template_id` (19-digit approved DLT ID), and
`variables`. Map the exact provider variable names to `reference` and `status`
once each, such as `"variables":{"VAR1":"reference","VAR2":"status"}`.
The selected active provider template must already have the matching approved
sender, principal entity and content. The DLT ID is readiness metadata;
the [send API](https://docs.msg91.com/sms/send-sms) selects its approved mapping
using the MSG91 `template_id`. Do not interchange these IDs.

Flags record an operator's verification; they do not request or grant approval.
Check the default template version, DLT verification, sender/PE–TM binding and
any applicable CTA approval in the actual account. `website check-msg91` validates
local configuration without sending SMS or asserting provider readiness.

Review queued records before enabling `WECRATFS_SMS_ENABLED=true` on the
production candidate. Development/staging refuse live SMS. After approval,
perform a controlled genuine send to an owner-approved account contact and
inspect MSG91 logs for actual content and final delivery. No test recipient,
fake entity or demo template is included in production settings.

The in-app account inspection confirmed that the template editor has no sender
to select. Choosing India in Add Sender ID exposes the DLT Entity/PE ID and
PE–TM registration confirmation. These cannot be filled truthfully before
registration. Opening AuthKey settings separately requests account mobile
verification; the owner must complete that step in the browser. No OTP is
requested through chat and no MSG91 auth key has been provisioned yet.

## Persistence and recovery

Migration 61 commits the notification, `transactional_sms` receipt, durable job
and outbox event together. Notification uniqueness prevents duplicate queueing.
Historical notices are not backfilled; unsent messages expire after 24 hours.
PostgreSQL claims each message before the Redis worker performs HTTP.

The adapter posts one recipient to `https://control.msg91.com/api/v5/flow` with
the private `authkey` header, approved template, mapped variables and
`short_url=0`. Redirects are rejected. Provider bodies, keys and phones are not
logged or returned to application users.

States: `queued`, `submitting`, `accepted`, `uncertain`, `rejected`, `skipped`.
Accepted records a provider request ID and means API acceptance, not handset
delivery. This release does not consume unverified DLR callbacks; final delivery
is checked in MSG91 logs. SMS cannot change payment or parcel delivery state.

Explicit provider errors are rejected. Timeout, malformed response, redirect,
server failure, interrupted worker or lost result persistence become uncertain.
Stale send intents become uncertain after five minutes. Neither is blindly
resubmitted: the provider API establishes no idempotent-send contract. Inspect
provider logs and reconcile before approving a manual retry. A missing saved
response does not prove no SMS was sent.

Operator queries omit recipient numbers and message bodies:

```sql
SELECT status,count(*) FROM transactional_sms GROUP BY status ORDER BY status;
SELECT id,template_key,status,provider_request_id,submitted_at,updated_at
FROM transactional_sms WHERE status IN ('uncertain','rejected') ORDER BY id;
```

If delivery must pause, disable the switch and restart the compatible candidate.
Retain receipts for review. An SMS outage does not undo a captured payment,
refund, label or order action.

Tests cover phone normalization, approval gates, template/recipient isolation,
redacted errors, ambiguous responses, disabled delivery, replay prevention,
business rollback, preferences/roles, returns and item refunds using disposable
PostgreSQL. Automated checks do not establish live account or handset delivery.
