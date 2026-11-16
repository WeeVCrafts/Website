ALTER TABLE product_variants ADD COLUMN return_class_request text NOT NULL DEFAULT 'unclassified' CHECK(return_class_request IN ('unclassified','textile_craft','fragile_art'));
