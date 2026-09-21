package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/config"
)

const demoPassword = "WeeVCrafts-Demo-2026!"

type account struct {
	Key, Email, Name, Role string
}

type product struct {
	Key, Slug, BrandSlug, BrandName, CategorySlug, CategoryName, Name, Description, SKU, Image string
	Price, Compare, Stock                                                                      int64
	SellerKey                                                                                  string
}

type order struct {
	Key, Number, CustomerKey, ProductKey, Status, PaymentStatus, FulfillmentStatus string
	Quantity                                                                       int
}

var accounts = []account{
	{"customer-01", "demo.customer01@weevcrafts.local", "Priya Sharma", "customer"},
	{"customer-02", "demo.customer02@weevcrafts.local", "Sneha Iyer", "customer"},
	{"customer-03", "demo.customer03@weevcrafts.local", "Arjun Nair", "customer"},
	{"customer-04", "demo.customer04@weevcrafts.local", "Kavita Rao", "customer"},
	{"customer-05", "demo.customer05@weevcrafts.local", "Sophia Lee", "customer"},
	{"customer-06", "demo.customer06@weevcrafts.local", "Rahul Menon", "customer"},
	{"customer-07", "demo.customer07@weevcrafts.local", "Meera Joshi", "customer"},
	{"customer-08", "demo.customer08@weevcrafts.local", "Vikram Das", "customer"},
	{"owner-01", "demo.owner01@weevcrafts.local", "Aditi Rao", "seller_owner"},
	{"owner-02", "demo.owner02@weevcrafts.local", "Mithila Arts Owner", "seller_owner"},
	{"owner-03", "demo.owner03@weevcrafts.local", "CraftVilla Owner", "seller_owner"},
	{"staff-01", "demo.staff01@weevcrafts.local", "Arjun Nair Staff", "seller_staff"},
	{"staff-02", "demo.staff02@weevcrafts.local", "Nisha Kapoor Staff", "seller_staff"},
	{"staff-03", "demo.staff03@weevcrafts.local", "Dev Shah Staff", "seller_staff"},
	{"marketplace-01", "demo.marketplace01@weevcrafts.local", "Marketplace Reviewer", "marketplace_admin"},
	{"marketplace-02", "demo.marketplace02@weevcrafts.local", "Marketplace Operations", "marketplace_admin"},
	{"marketplace-03", "demo.marketplace03@weevcrafts.local", "Marketplace Content", "marketplace_admin"},
	{"super-01", "demo.super01@weevcrafts.local", "Super Admin One", "super_admin"},
	{"super-02", "demo.super02@weevcrafts.local", "Super Admin Two", "super_admin"},
	{"support-01", "demo.support01@weevcrafts.local", "Aditi Support", "support_agent"},
	{"support-02", "demo.support02@weevcrafts.local", "Rohan Support", "support_agent"},
	{"finance-01", "demo.finance01@weevcrafts.local", "Finance Operator", "finance_operator"},
	{"finance-02", "demo.finance02@weevcrafts.local", "Settlement Operator", "finance_operator"},
	{"operations-01", "demo.operations01@weevcrafts.local", "Returns Operator", "operations_returns"},
}

var products = []product{
	{"product-01", "madhubani-tree", "mithila-arts", "Mithila Arts", "arts", "Arts", "Madhubani Painting - Tree of Life", "A hand-painted celebration of life, colour, and the landscapes of Bihar.", "MA-TREE-001", "/assets/images/customer/madhubani-tree.webp", 249900, 350000, 18, "owner-01"},
	{"product-02", "brass-elephant", "kalakriti", "Kalakriti", "crafts", "Crafts", "Brass Elephant Figurine", "A warm Moradabad brass keepsake finished by hand.", "KA-ELE-002", "/assets/images/customer/brass-elephant.webp", 189900, 240000, 24, "owner-02"},
	{"product-03", "chanderi-royal", "heritage-looms", "Heritage Looms", "textiles", "Textiles", "Chanderi Royal Saree", "Lightweight silk cotton woven in small batches.", "HL-SAR-003", "/assets/images/customer/saree-maroon.webp", 549900, 680000, 12, "owner-03"},
	{"product-04", "wooden-box", "craftvilla", "CraftVilla", "home", "Home", "Hand-carved Wooden Keepsake Box", "A useful mango-wood box for letters, jewellery, and small rituals.", "CV-BOX-004", "/assets/images/customer/wooden-box.webp", 129900, 179900, 30, "owner-03"},
	{"product-05", "ceramic-mugs", "clay-stories", "Clay Stories", "home", "Home", "Handpainted Ceramic Mugs - Set of 2", "A pair of bright studio mugs with individually painted details.", "CS-MUG-005", "/assets/images/customer/ceramic-mugs.webp", 129800, 159800, 40, "owner-02"},
	{"product-06", "blue-tote", "weavers-touch", "Weaver's Touch", "textiles", "Textiles", "Indigo Everyday Tote", "A sturdy naturally dyed cotton tote for everyday carrying.", "WT-TOT-006", "/assets/images/customer/blue-tote.webp", 89900, 119900, 35, "owner-01"},
	{"product-07", "terracotta-lamp", "earth-and-fire", "Earth & Fire", "home", "Home", "Terracotta Evening Lamp", "Soft light shaped and fired by a family pottery studio.", "EF-LMP-007", "/assets/images/customer/terracotta-lamp.webp", 159900, 219900, 16, "owner-02"},
	{"product-08", "block-print-scarf", "rangrez", "Rangrez Studio", "textiles", "Textiles", "Block Print Cotton Scarf", "A breathable cotton scarf printed with traditional wooden blocks.", "RS-SCF-008", "/assets/images/customer/blue-tote.webp", 79900, 99900, 22, "owner-03"},
	{"product-09", "wicker-basket", "bamboo-root", "Bamboo Root", "home", "Home", "Handwoven Wicker Basket", "A flexible storage basket woven from locally harvested fibres.", "BR-BSK-009", "/assets/images/customer/wooden-box.webp", 109900, 139900, 20, "owner-01"},
	{"product-10", "blue-pottery", "jaipur-blue", "Jaipur Blue", "crafts", "Crafts", "Blue Pottery Serving Plate", "A glazed serving piece inspired by Jaipur's blue pottery tradition.", "JB-PLT-010", "/assets/images/customer/ceramic-mugs.webp", 179900, 229900, 14, "owner-02"},
	{"product-11", "copper-bottle", "tamra", "Tamra", "wellness", "Wellness", "Hand-hammered Copper Bottle", "A reusable copper bottle with a tactile hand-hammered finish.", "TM-BTL-011", "/assets/images/customer/brass-elephant.webp", 99900, 129900, 28, "owner-03"},
	{"product-12", "natural-soap", "forest-rituals", "Forest Rituals", "wellness", "Wellness", "Neem and Sandalwood Soap", "A gentle cold-process soap made with simple, traceable ingredients.", "FR-SOP-012", "/assets/images/customer/terracotta-lamp.webp", 49900, 59900, 50, "owner-01"},
}

var orders = []order{
	{"order-01", "WCDEMO1001", "customer-01", "product-01", "pending_payment", "created", "pending", 1},
	{"order-02", "WCDEMO1002", "customer-02", "product-02", "paid", "captured", "processing", 1},
	{"order-03", "WCDEMO1003", "customer-03", "product-03", "shipped", "captured", "shipped", 1},
	{"order-04", "WCDEMO1004", "customer-04", "product-04", "delivered", "captured", "delivered", 2},
	{"order-05", "WCDEMO1005", "customer-05", "product-05", "cancelled", "failed", "pending", 1},
	{"order-06", "WCDEMO1006", "customer-06", "product-06", "payment_failed", "failed", "pending", 1},
	{"order-07", "WCDEMO1007", "customer-07", "product-07", "refunded", "refunded", "delivered", 1},
	{"order-08", "WCDEMO1008", "customer-08", "product-08", "processing", "captured", "processing", 1},
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cfg := config.FromEnv()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	if err := postgres.ApplyMigrations(ctx, pool); err != nil {
		log.Fatal(err)
	}
	if err := seed(ctx, pool); err != nil {
		log.Fatal(err)
	}
	fmt.Println("WeCratfs demo data is ready. All demo accounts use password:", demoPassword)
}

func seed(ctx context.Context, pool *pgxpool.Pool) error {
	hasher := auth.NewArgon2idHasher()
	passwordHash, err := hasher.Hash(demoPassword)
	if err != nil {
		return err
	}
	return postgres.WithinTransaction(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		users := map[string]int64{}
		for _, item := range accounts {
			id, err := ensureAccount(ctx, tx, item, passwordHash)
			if err != nil {
				return fmt.Errorf("account %s: %w", item.Key, err)
			}
			users[item.Key] = id
		}
		sellers := map[string]int64{}
		for _, item := range []struct{ key, name string }{{"owner-01", "Weaver's Touch"}, {"owner-02", "Mithila Arts"}, {"owner-03", "CraftVilla"}} {
			id, err := ensureSeller(ctx, tx, users[item.key], item.name)
			if err != nil {
				return fmt.Errorf("seller %s: %w", item.key, err)
			}
			sellers[item.key] = id
		}
		if err := seedStaff(ctx, tx, users, sellers); err != nil {
			return fmt.Errorf("seller staff: %w", err)
		}
		productIDs := map[string]int64{}
		variantIDs := map[string]int64{}
		for _, item := range products {
			id, variantID, err := ensureProduct(ctx, tx, item, sellers[item.SellerKey])
			if err != nil {
				return fmt.Errorf("product %s: %w", item.Key, err)
			}
			productIDs[item.Key], variantIDs[item.Key] = id, variantID
		}
		if err := seedCustomers(ctx, tx, users, productIDs); err != nil {
			return fmt.Errorf("customers: %w", err)
		}
		orderIDs := map[string]int64{}
		for _, item := range orders {
			id, err := ensureOrder(ctx, tx, item, users, productIDs, variantIDs, sellers)
			if err != nil {
				return fmt.Errorf("order %s: %w", item.Key, err)
			}
			orderIDs[item.Key] = id
		}
		if err := seedReviewsNotifications(ctx, tx, users, productIDs, orderIDs); err != nil {
			return fmt.Errorf("reviews and notifications: %w", err)
		}
		if err := seedPlatformData(ctx, tx, users, productIDs, sellers, orderIDs); err != nil {
			return fmt.Errorf("platform data: %w", err)
		}
		return seedLocalWorkflowData(ctx, tx, users, productIDs, variantIDs, sellers, orderIDs)
	})
}

func ensureAccount(ctx context.Context, tx pgx.Tx, item account, passwordHash string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `INSERT INTO users (email, display_name, status) VALUES ($1::text, $2::text, 'active') ON CONFLICT (email) DO NOTHING RETURNING id`, item.Email, item.Name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1::text`, item.Email).Scan(&id); err != nil {
			return 0, fmt.Errorf("lookup existing user: %w", err)
		}
		return id, nil
	}
	if err != nil {
		return 0, fmt.Errorf("ensure account %s: insert user: %w", item.Key, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO credentials (user_id, password_hash) VALUES ($1::bigint, $2::text)`, id, passwordHash); err != nil {
		return 0, fmt.Errorf("ensure account %s: credentials: %w", item.Key, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_slug) VALUES ($1::bigint, $2::text) ON CONFLICT DO NOTHING`, id, item.Role); err != nil {
		return 0, fmt.Errorf("ensure account %s: role: %w", item.Key, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET email_verified_at = NOW() WHERE id = $1::bigint`, id); err != nil {
		return 0, fmt.Errorf("ensure account %s: verification: %w", item.Key, err)
	}
	return id, nil
}

func ensureSeller(ctx context.Context, tx pgx.Tx, ownerID int64, name string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `INSERT INTO sellers (owner_user_id, display_name, status) VALUES ($1, $2, 'active') ON CONFLICT (owner_user_id) DO NOTHING RETURNING id`, ownerID, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id FROM sellers WHERE owner_user_id = $1`, ownerID).Scan(&id)
	}
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_slug) VALUES ($1, 'seller_owner') ON CONFLICT DO NOTHING`, ownerID)
	return id, err
}

func seedStaff(ctx context.Context, tx pgx.Tx, users, sellers map[string]int64) error {
	permissions := []string{"PRODUCT_READ", "PRODUCT_WRITE", "INVENTORY_READ", "INVENTORY_WRITE", "ORDER_READ", "ORDER_FULFILL", "RETURN_READ", "REPORT_READ"}
	for _, item := range []struct{ user, owner string }{{"staff-01", "owner-01"}, {"staff-02", "owner-02"}, {"staff-03", "owner-03"}} {
		if _, err := tx.Exec(ctx, `INSERT INTO seller_users (seller_id, user_id, status, permissions, added_by) VALUES ($1, $2, 'active', $3, $4) ON CONFLICT (seller_id, user_id) DO NOTHING`, sellers[item.owner], users[item.user], permissions, users[item.owner]); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_slug) VALUES ($1, 'seller_staff') ON CONFLICT DO NOTHING`, users[item.user]); err != nil {
			return err
		}
	}
	return nil
}

func ensureProduct(ctx context.Context, tx pgx.Tx, item product, sellerID int64) (int64, int64, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO brands (slug, name) VALUES ($1, $2) ON CONFLICT (slug) DO NOTHING`, item.BrandSlug, item.BrandName); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO categories (slug, name) VALUES ($1, $2) ON CONFLICT (slug) DO NOTHING`, item.CategorySlug, item.CategoryName); err != nil {
		return 0, 0, err
	}
	var id int64
	err := tx.QueryRow(ctx, `INSERT INTO products (slug, brand_slug, category_slug, name, description, price_cents, compare_at_cents, delivery_label, image_url, seller_id, status, featured_rank) VALUES ($1,$2,$3,$4,$5,$6,$7,'Delivery in 4 - 6 days',$8,$9,'approved',$10) ON CONFLICT (slug) DO UPDATE SET image_url = EXCLUDED.image_url RETURNING id`, item.Slug, item.BrandSlug, item.CategorySlug, item.Name, item.Description, item.Price, item.Compare, item.Image, sellerID, len(item.Slug)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `SELECT id FROM products WHERE slug = $1`, item.Slug).Scan(&id); err != nil {
			return 0, 0, err
		}
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, err
	}
	var variantID int64
	err = tx.QueryRow(ctx, `INSERT INTO product_variants (product_id, sku, name, price_cents, compare_at_cents) VALUES ($1,$2,'Default variant',$3,$4) ON CONFLICT (sku) DO NOTHING RETURNING id`, id, item.SKU, item.Price, item.Compare).Scan(&variantID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id FROM product_variants WHERE sku = $1`, item.SKU).Scan(&variantID)
	}
	if err != nil {
		return 0, 0, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO inventory_stock (variant_id, available_quantity) VALUES ($1,$2) ON CONFLICT (variant_id) DO NOTHING`, variantID, item.Stock)
	return id, variantID, err
}

func seedCustomers(ctx context.Context, tx pgx.Tx, users, products map[string]int64) error {
	for i := 1; i <= 8; i++ {
		userID := users[fmt.Sprintf("customer-%02d", i)]
		if _, err := tx.Exec(ctx, `INSERT INTO addresses (user_id, recipient_name, line1, city, state, postal_code, country_code) SELECT $1,$2,$3,$4,$5,$6,'IN' WHERE NOT EXISTS (SELECT 1 FROM addresses WHERE user_id=$1 AND line1=$3 AND postal_code=$6)`, userID, fmt.Sprintf("Demo Customer %02d", i), fmt.Sprintf("%d Artisan Lane", i), "Bengaluru", "Karnataka", fmt.Sprintf("560%03d", i)); err != nil {
			return err
		}
		if i <= 6 {
			if _, err := tx.Exec(ctx, `INSERT INTO wishlist_items (user_id, product_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, products[fmt.Sprintf("product-%02d", i)]); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureOrder(ctx context.Context, tx pgx.Tx, item order, users map[string]int64, products, variants, sellers map[string]int64) (int64, error) {
	customerID := users[item.CustomerKey]
	productID := products[item.ProductKey]
	variantID := variants[item.ProductKey]
	var price int64
	var sellerID int64
	if err := tx.QueryRow(ctx, `SELECT price_cents, seller_id FROM products WHERE id = $1`, productID).Scan(&price, &sellerID); err != nil {
		return 0, err
	}
	subtotal := price * int64(item.Quantity)
	var orderID int64
	err := tx.QueryRow(ctx, `INSERT INTO orders (order_number,user_id,status,currency,subtotal_cents,shipping_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) VALUES ($1,$2,$3,'INR',$4,0,$4,$5,'demo',$1) ON CONFLICT (order_number) DO NOTHING RETURNING id`, item.Number, customerID, item.Status, subtotal, fmt.Sprintf(`{"recipient_name":"%s","line1":"Demo Artisan Lane","city":"Bengaluru","state":"Karnataka","postal_code":"560001","country_code":"IN"}`, item.CustomerKey)).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `SELECT id FROM orders WHERE order_number = $1`, item.Number).Scan(&orderID); err != nil {
			return 0, err
		}
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO order_items (order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents) SELECT $1,id,name,sku,$2,$3,$4 FROM product_variants WHERE id=$5 ON CONFLICT DO NOTHING`, orderID, price, item.Quantity, subtotal, variantID); err != nil {
		return 0, err
	}
	providerOrderID := "local_order_" + item.Number
	providerPaymentID := "local_payment_" + item.Number
	if _, err := tx.Exec(ctx, `INSERT INTO payment_attempts (order_id,provider,provider_order_id,provider_payment_id,status,amount_cents,currency) VALUES ($1,'local',$2,$3,$4,$5,'INR') ON CONFLICT DO NOTHING`, orderID, providerOrderID, providerPaymentID, item.PaymentStatus, subtotal); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO order_status_history (order_id,from_status,to_status,actor_id,reason) SELECT $1,NULL,$2,$3,'seeded demo scenario' WHERE NOT EXISTS (SELECT 1 FROM order_status_history WHERE order_id=$1 AND reason='seeded demo scenario')`, orderID, item.Status, customerID); err != nil {
		return 0, err
	}
	if item.FulfillmentStatus != "pending" {
		if _, err := tx.Exec(ctx, `INSERT INTO seller_fulfillments (seller_id,order_id,status,carrier,tracking_number,last_note,updated_by) VALUES ($1,$2,$3,'India Post',$4,'Demo fulfilment update',$5) ON CONFLICT (seller_id,order_id) DO NOTHING`, sellerID, orderID, item.FulfillmentStatus, "DEMO-"+item.Number, users["owner-01"]); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO shipments (order_id,seller_id,carrier,tracking_number,status,estimated_delivery) VALUES ($1,$2,'India Post',$3,CASE WHEN $4='processing' OR $4='shipped' THEN 'in_transit' ELSE $4 END,CURRENT_DATE + 4) ON CONFLICT (order_id,seller_id) DO NOTHING`, orderID, sellerID, "DEMO-"+item.Number, item.FulfillmentStatus); err != nil {
			return 0, err
		}
	}
	if item.Status == "pending_payment" {
		if _, err := tx.Exec(ctx, `INSERT INTO inventory_reservations (order_id,status,expires_at) VALUES ($1,'active',NOW()+INTERVAL '30 minutes') ON CONFLICT (order_id) DO NOTHING`, orderID); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `WITH inserted AS (INSERT INTO inventory_reservation_items (reservation_id,variant_id,quantity) SELECT id,$2,$3 FROM inventory_reservations WHERE order_id=$1 ON CONFLICT DO NOTHING RETURNING variant_id,quantity) UPDATE inventory_stock i SET available_quantity=i.available_quantity-u.quantity,reserved_quantity=i.reserved_quantity+u.quantity,updated_at=NOW() FROM inserted u WHERE i.variant_id=u.variant_id`, orderID, variantID, item.Quantity); err != nil {
			return 0, err
		}
	}
	if item.Status == "refunded" {
		if _, err := tx.Exec(ctx, `INSERT INTO payment_refunds (order_id,request_key,amount_cents,currency,reason,provider_refund_id,status) VALUES ($1,$2,$3,'INR','Demo refund','local_refund_'||$2,'processed') ON CONFLICT (order_id) DO NOTHING`, orderID, "demo-refund-"+item.Number, subtotal); err != nil {
			return 0, err
		}
	}
	return orderID, nil
}

func seedReviewsNotifications(ctx context.Context, tx pgx.Tx, users, products, orders map[string]int64) error {
	if _, err := tx.Exec(ctx, `INSERT INTO product_reviews (product_id,user_id,order_id,rating,title,body,status,verified_purchase) VALUES ($1,$2,$3,5,'Beautiful and thoughtful','The craft and finish are lovely.','published',TRUE) ON CONFLICT DO NOTHING`, products["product-01"], users["customer-04"], orders["order-04"]); err != nil {
		return err
	}
	for i := 1; i <= 8; i++ {
		if _, err := tx.Exec(ctx, `INSERT INTO notifications (user_id,kind,title,body) SELECT $1,'order','Your WeCratfs demo order is moving','A demo order is available to explore in your account.' WHERE NOT EXISTS (SELECT 1 FROM notifications WHERE user_id=$1 AND kind='order' AND title='Your WeCratfs demo order is moving')`, users[fmt.Sprintf("customer-%02d", i)]); err != nil {
			return err
		}
	}
	return nil
}

func seedPlatformData(ctx context.Context, tx pgx.Tx, users, products, sellers, orders map[string]int64) error {
	admin := users["marketplace-01"]
	if _, err := tx.Exec(ctx, `INSERT INTO promotions (slug,name,description,discount_type,discount_value,status,starts_at,ends_at,usage_limit,created_by) VALUES ('handmade-today','Handmade Today','A local demo promotion for first orders','percentage',10,'active',NOW()-INTERVAL '1 day',NOW()+INTERVAL '30 days',100,$1::bigint) ON CONFLICT (slug) DO NOTHING`, admin); err != nil {
		return err
	}
	var promotionID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM promotions WHERE slug='handmade-today'`).Scan(&promotionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO coupons (promotion_id,code,usage_limit) VALUES ($1::bigint,'WELCOME10',100) ON CONFLICT (code) DO NOTHING`, promotionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO promotion_products (promotion_id,product_id) VALUES ($1::bigint,$2::bigint) ON CONFLICT DO NOTHING`, promotionID, products["product-01"]); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO content_entries (kind,slug,title,body,status,published_at,created_by) VALUES ('story','from-indian-hands','From Indian Hands to Indian Homes','Stories, materials, and useful rituals from Indian makers for homes across India.','published',NOW(),$1::bigint) ON CONFLICT (slug) DO UPDATE SET title=EXCLUDED.title, body=EXCLUDED.body, status=EXCLUDED.status`, admin); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO commission_rules (name,category_slug,percentage_basis_points,active,created_by) VALUES ('Default marketplace commission',NULL,1200,TRUE,$1::bigint) ON CONFLICT (name) DO NOTHING`, admin); err != nil {
		return err
	}
	for _, item := range []struct{ customer, number, subject, status string }{
		{"customer-01", "WCDEMO1001", "Where is my payment link?", "open"},
		{"customer-03", "WCDEMO1003", "Tracking update requested", "in_progress"},
		{"customer-04", "WCDEMO1004", "Return received", "resolved"},
	} {
		orderKey := fmt.Sprintf("order-%02d", int(item.number[len(item.number)-1]-'0'))
		if _, err := tx.Exec(ctx, `INSERT INTO support_tickets (ticket_number,user_id,order_id,subject,message,status,priority,agent_note,updated_by) VALUES ($1::text,$2::bigint,$3::bigint,$4::text,'This is a seeded support conversation.',$5::text,'normal','Demo support note',$6::bigint) ON CONFLICT (ticket_number) DO NOTHING`, "WCDSP"+item.number, users[item.customer], orders[orderKey], item.subject, item.status, users["support-01"]); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO return_requests (order_id,user_id,reason,status) VALUES ($1::bigint,$2::bigint,'Item arrived damaged','pending') ON CONFLICT (order_id) DO NOTHING`, orders["order-04"], users["customer-04"]); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_id,action,resource_type,resource_id,request_id,metadata) SELECT $1::bigint,'demo.seeded','platform','local-demo','seed-demo','{"source":"idempotent-demo-seed"}'::jsonb WHERE NOT EXISTS (SELECT 1 FROM audit_logs WHERE action='demo.seeded' AND resource_id='local-demo')`, users["super-01"]); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO system_settings (setting_key,setting_value,updated_by) VALUES ('demo.local_mode','true',$1::bigint) ON CONFLICT (setting_key) DO NOTHING`, users["super-01"]); err != nil {
		return err
	}
	return nil
}

func seedLocalWorkflowData(ctx context.Context, tx pgx.Tx, users, products, variants, sellers, orders map[string]int64) error {
	for _, item := range []struct {
		user, product string
		quantity      int
	}{{"customer-01", "product-01", 1}, {"customer-02", "product-02", 2}} {
		var cartID int64
		err := tx.QueryRow(ctx, `INSERT INTO carts (user_id,status,expires_at) SELECT $1,'active',NOW()+INTERVAL '30 days' WHERE NOT EXISTS (SELECT 1 FROM carts WHERE user_id=$1 AND status='active') RETURNING id`, users[item.user]).Scan(&cartID)
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.QueryRow(ctx, `SELECT id FROM carts WHERE user_id=$1 AND status='active'`, users[item.user]).Scan(&cartID); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO cart_items (cart_id,variant_id,quantity) VALUES ($1,$2,$3) ON CONFLICT (cart_id,variant_id) DO NOTHING`, cartID, variants[item.product], item.quantity); err != nil {
			return err
		}
	}

	for index, sellerKey := range []string{"owner-01", "owner-02", "owner-03"} {
		objectKey := fmt.Sprintf("demo/sellers/%s/business-registration.pdf", sellerKey)
		if _, err := tx.Exec(ctx, `INSERT INTO seller_documents (seller_id,document_type,object_key,status,reviewed_by,review_note) VALUES ($1,'business_registration',$2,'verified',$3,'Verified local demo document') ON CONFLICT (object_key) DO NOTHING`, sellers[sellerKey], objectKey, users[fmt.Sprintf("marketplace-%02d", index+1)]); err != nil {
			return err
		}
	}

	for _, orderKey := range []string{"order-02", "order-03", "order-04"} {
		var shipmentID int64
		if err := tx.QueryRow(ctx, `SELECT id FROM shipments WHERE order_id=$1`, orders[orderKey]).Scan(&shipmentID); err != nil {
			return err
		}
		status := map[string]string{"order-02": "in_transit", "order-03": "in_transit", "order-04": "delivered"}[orderKey]
		if _, err := tx.Exec(ctx, `INSERT INTO shipment_events (shipment_id,status,location,note) SELECT $1,$2,$3,$4 WHERE NOT EXISTS (SELECT 1 FROM shipment_events WHERE shipment_id=$1 AND status=$2 AND note=$4)`, shipmentID, status, "Bengaluru", "Seeded tracking event"); err != nil {
			return err
		}
	}

	for _, userKey := range []string{"super-01", "super-02"} {
		if _, err := tx.Exec(ctx, `INSERT INTO mfa_methods (user_id,method,secret_ciphertext,enabled) VALUES ($1,'totp',$2,TRUE) ON CONFLICT (user_id,method) DO NOTHING`, users[userKey], "demo-only-totp-secret-"+userKey); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO security_events (user_id,actor_id,event_type,metadata) SELECT $1,$1,'mfa.enabled','{"source":"seed-demo"}'::jsonb WHERE NOT EXISTS (SELECT 1 FROM security_events WHERE user_id=$1 AND event_type='mfa.enabled')`, users["super-01"]); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO data_export_requests (user_id,status,object_key,completed_at) SELECT $1,'ready','demo/exports/customer-01.json',NOW() WHERE NOT EXISTS (SELECT 1 FROM data_export_requests WHERE user_id=$1 AND object_key='demo/exports/customer-01.json')`, users["customer-01"]); err != nil {
		return err
	}
	return nil
}
