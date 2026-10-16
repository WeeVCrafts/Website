# Homepage and category management

Admin and Super Admin prepare category names, pictures, numeric display order
and discovery visibility in **Storefront → Categories**. Admin submits drafts;
Super Admin reviews and publishes them. Customers see the published category
presentation across navigation, discovery, listings and filters. Lower order
numbers appear first; slugs break ties and remain fixed. Hiding a category
removes discovery links without unpublishing products.

Use **Storefront → Media** for JPEG, PNG and WebP uploads up to 8 MiB /
16 megapixels, then copy the permanent image URL into the category draft.
The persistent object store retains normalized JPEGs and responsive variants
for previews and release history. Upload failures report actionable errors and
preserve the current published image. See [the editor guide](storefront-editor.md).

Products have one primary category and any number of additional categories.
The primary continues to control existing reporting and commission calculations.
Company administrators edit assignments in the product editor or use
**Products → Categories → Manage products** (`/admin/category-management` or
`/super-admin/category-management`) to add/remove up to 25 selected products.
Publish new categories before assigning products. Removing a primary
membership requires choosing its replacement in the editor first. Categories
referenced by any product or commission rule cannot be deleted. Sellers choose
existing categories and cannot create taxonomy or submit cross-listing fields.
These actions retain server role checks, CSRF protection and audit records.

The homepage shares one responsive hero across all widths. It advances every
four seconds and has arrows, slide selectors and touch swipe, with no Pause/Play
buttons as requested. Hover and keyboard focus pause rotation; leaving the hero
resumes a fresh four-second interval. Selecting/swiping a slide starts a fresh
interval unless another pause condition applies. Hidden tabs and offscreen
heroes pause; reduced-motion users use manual navigation. Failed images do not
replace the current slide with a blank frame.

Four bounded, independent catalog queries feed Featured (6, featured order),
New arrivals (8, newest first), Shop by category (5, selected category), and
Gifts under ₹1,500 (6, Gifts membership, price strictly below ₹1,500). They show
approved, available products from active sellers. Each section has its own
empty state and filtered catalog link. Category/filter results include secondary
memberships without duplicate product rows. A database catalog revision prevents
old cached category/product projections from surviving a committed change.
Database-only and in-stock searches read fresh stock rather than cached results.

## Import actual inventory

Copy `docs/catalog-import-template.csv` and fill its rows from verified inventory.
The columns, in order, are:

| Column | Meaning |
| --- | --- |
| `sku`, `slug` | Distinct, stable SKU and lowercase URL slug |
| `name`, `description` | Merchant-supplied product details |
| `primary_category` | Existing category slug |
| `additional_categories` | Optional category slugs separated by semicolons |
| `price_inr` | Positive rupee amount with up to two decimal places |
| `stock` | Confirmed whole quantity, including zero |
| `image_file` | Matching photograph relative to the image directory |
| `delivery_note` | Optional merchant delivery note |

The seven launch category slugs are `arts`, `home`, `textiles`, `gifts`,
`pottery-ceramics`, `handloom-sarees` and `brass-metalwork`. The target is at least
ten distinct primary products in each: 70 total. Additional memberships do not
inflate these primary counts. No example inventory is included or published.

Set the intended local `DATABASE_URL` and existing storage variables as described
in README. Existing designated usernames default to `seller` and `admin`; use
`--seller-username` / `--admin-username` to select other existing accounts. The
seller must have the Seller role, and the actor must be Admin or Super Admin.
The tool never creates accounts, changes passwords, migrates schema or approves
listings. Start the updated app once to apply migrations first.

```powershell
# Read-only validation of the complete manifest, photographs and account mapping.
go run ./cmd/import-catalog --csv "C:/Inventory/products.csv" --images "C:/Inventory/photos"

# Enforce the full launch target during validation.
go run ./cmd/import-catalog --csv "C:/Inventory/products.csv" --images "C:/Inventory/photos" --require-launch-counts

# Explicitly create private drafts and persist matching photographs.
go run ./cmd/import-catalog --csv "C:/Inventory/products.csv" --images "C:/Inventory/photos" --apply
```

Photos must be genuine JPEG, PNG or WebP files, no larger than 25 MiB / 40
megapixels, and stay inside the image directory even after resolving symlinks.
CSV manifests accept up to 8 MiB and 10,000 rows. The complete CSV is checked
before applying it. Existing unrelated SKUs/slugs
are rejected. A retry resumes only records created by this importer; completed
products and later price, stock or content edits are retained. An interrupted
photo upload leaves a draft that the same import can resume. Per-product commits
allow recovery after interruption; completed earlier rows remain drafts.

Review imported drafts through the existing submit/approve workflow. Publication
requires the real inventory and photographs; neither the application nor this
tool fabricates sellable stock, item specifications, reviews or discounts.
