# Manual settlement controls

These are authenticated HTML form endpoints, not automated bank transfer APIs.
The same controls apply under `/admin/finance/settlements` and
`/super-admin/finance/settlements`. Each endpoint requires the corresponding
current account role. Operators use individual accounts assigned to two
different people. The database and application enforce different preparer and
approver account IDs; software cannot establish whether two accounts belong to
different people.

## Forms and state

All POST forms require `csrf_token`. All actions except `create` also require
positive `settlement_id` and the current `revision`. A stale or invalid decision
returns 409 without saving a partial state. Successful requests redirect with
303 to the settlement workspace. No button sends money.

| Action | Required inputs | Result |
| --- | --- | --- |
| `POST /create` | CSRF token | Batches eligible open ledger entries; records preparer. |
| `POST /prepare` | Account holder `payee_name`, `bank_name`, four-digit `account_last4`, eleven-character `routing_code` (IFSC), `verification_note`, multipart `payee_evidence` | Creates a recipient version, clears approval and returns batch to pending. Only the original preparer, or an operator claiming an unprepared historical batch, can do this before transfer proof exists. |
| `POST /status`, `status=approved` | `verified_payee=yes` | A different operator verifies the private seller/bank document and payable amount. Saves approver, time, recipient version and ledger membership/amount hash. |
| `POST /transfer` | `transfer_amount_cents`, `transfer_date` (`YYYY-MM-DD`), `bank_reference`, multipart `transfer_evidence` | The original preparer records an external completed transfer after approval. Amount must equal approved paise total; date must fall between approval date and today in India. Bank reference must be unique. |
| `POST /status`, `status=paid` | `verified_transfer=yes` | The saved independent approver confirms transfer proof. Revalidates recipient, ledger and holds; marks the batch paid and linked open entries settled in one transaction. |
| `POST /status`, `status=failed` | Current revision | Cancels a batch and releases linked open entries only before any transfer proof is recorded. |

Only PDF, PNG and JPEG bytes up to 5 MiB are accepted. Private recipient evidence
must allow the approver to match the seller and full bank details. Only the
recipient name, bank, IFSC and account suffix are stored as searchable form
fields. Full documents remain in the private object store. Do not enter full
account credentials, passwords or unrelated personal data into notes.

Recipient versions cannot be edited in place. Preparing a replacement version
requires new approval. A later recipient version for the same seller prevents
an older approved batch from proceeding. Changes to ledger membership, saved
amounts, payment holds, refund state or unsettled refunds block transfer/payment
confirmation. Free-text ledger notes do not change financial approval.

Once transfer proof is submitted, the application does not replace the proof
or release that batch into another payable batch. Investigate any mismatch
against the bank statement before taking another financial action. This avoids
paying the same liability twice following an uncertain transfer.

## Private evidence

`GET` and `HEAD /evidence?settlement_id=N&kind=payee|transfer` require authorized
Admin or Super Admin access on that role's route. Responses use
`Cache-Control: private, no-store`, attachment disposition and `nosniff`.
Stored content type, length and SHA-256 must match before bytes are returned.
Private bucket addresses and object keys are never returned. Sellers receive
only their own settlement amount, bank reference, transfer date and evidence
status; they do not receive company proof URLs or bank documents.

## Migration and operational boundaries

Migration 55 is additive. Existing paid rows, amounts and ledger status remain
unchanged and are shown as requiring historical evidence reconciliation.
Existing pending or approved rows must be prepared under the new controls.
Do not fabricate bank references or evidence for historical records.

A database trigger rejects new status-only paid writes from an older
application. Deploy the migration and matching application together. Retain the
new schema during rollback and disable finance mutations until a compatible
application is restored. Do not restore older unsafe settlement writers.

Audit entries commit with each financial transition and include record,
revision and control identities, without bank document bytes or full recipient
details. Back up private finance evidence together with database references.
The business owner must approve document retention, recipient-verification
procedure, payout eligibility and operator access before production use.
