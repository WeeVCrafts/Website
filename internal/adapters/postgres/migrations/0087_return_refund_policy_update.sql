ALTER TABLE marketplace_policy_versions ALTER COLUMN published_by DROP NOT NULL;

DO $$
DECLARE
    v_admin_id bigint;
    v_content jsonb;
BEGIN
    SELECT id INTO v_admin_id FROM users WHERE status='active' ORDER BY id LIMIT 1;

    v_content := '[
        {
            "Title": "1. Handmade & Handloom Products",
            "Bullets": [
                "Handmade crafts and handloom products are generally not eligible for change-of-mind returns.",
                "Minor natural variations in colour, texture, weave and finish are part of traditional craftsmanship.",
                "This restriction does not apply to defective, damaged, incorrect or materially misdescribed products."
            ]
        },
        {
            "Title": "2. Damaged or Incorrect Products",
            "Text": "If you receive a damaged, defective, incorrect or incomplete product:",
            "Bullets": [
                "Report the issue to WeevCrafts as soon as possible, preferably within 48 hours of delivery.",
                "Upload clear photographs or an unboxing video showing the parcel, shipping label and product.",
                "Keep the original packaging and product until the complaint is resolved.",
                "We will review your complaint and arrange an eligible replacement or refund."
            ]
        },
        {
            "Title": "3. Video Verification – Our Transparency Promise",
            "Bullets": [
                "Every seller must record a pre-dispatch packing video for each order.",
                "The video will be uploaded to WeevCrafts and shared with the customer through a secure link, including WhatsApp notifications where applicable.",
                "Customers are encouraged to record an unboxing video when opening their parcel.",
                "If an unboxing video is unavailable, other relevant evidence may be considered."
            ]
        },
        {
            "Title": "4. Refunds & Replacements",
            "Text": "Approved refunds will be processed to the original payment method wherever possible. Return shipping for verified seller-side or transit-related issues will be arranged without additional cost to the customer, subject to applicable law."
        },
        {
            "Title": "5. Important & Customer Support",
            "Text": "Our video verification process is designed to ensure fairness, transparency and protection for both customers and sellers. Nothing in this policy limits consumer rights available under applicable Indian law.",
            "Bullets": [
                "For assistance, please contact WeevCrafts Customer Support at support@weevcrafts.com"
            ]
        }
    ]'::jsonb;

    INSERT INTO marketplace_policy_versions(version, content, published_at, published_by)
    VALUES('2026-11-23-v2', v_content, '2026-11-23 00:00:00+05:30', v_admin_id)
    ON CONFLICT(version) DO UPDATE SET content=EXCLUDED.content, published_at=EXCLUDED.published_at;

    UPDATE marketplace_policy_settings
    SET version = '2026-11-23-v2',
        published_at = '2026-11-23 00:00:00+05:30',
        published_by = v_admin_id,
        updated_at = now()
    WHERE id;
END $$;
