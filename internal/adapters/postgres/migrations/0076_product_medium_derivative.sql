ALTER TABLE product_image_derivatives
    DROP CONSTRAINT product_image_derivatives_width_check,
    ADD CONSTRAINT product_image_derivatives_width_check CHECK (width IN (320, 480, 640, 960));
