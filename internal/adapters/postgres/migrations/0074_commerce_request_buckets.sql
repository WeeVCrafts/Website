ALTER VIEW commerce_request_summary RENAME TO commerce_request_rows;
CREATE VIEW commerce_request_summary AS
SELECT r.*,
 CASE WHEN status IN ('completed','processed','refunded') THEN 'completed'
 WHEN status='rejected' THEN 'rejected'
 WHEN status IN ('requested','submitted','review_requested','received','inspected')
   OR (kind='legacy_return' AND status='pending') THEN 'needs_review'
 ELSE 'in_progress' END AS bucket
FROM commerce_request_rows r;
