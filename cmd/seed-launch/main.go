// seed-launch provisions the requested QA logins and a small, photo-matched catalog.
// Passwords are supplied at runtime and are never stored in the repository.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/application/auth"
)

type account struct{ key, name, role string }

var accounts = []account{
	{"customer", "QA Customer", "customer"},
	{"seller", "QA Seller", "seller_owner"},
	{"sellerstaff", "QA Seller Staff", "seller_staff"},
	{"marketplaceadmin", "QA Marketplace Admin", "marketplace_admin"},
	{"superadmin", "QA Super Admin", "super_admin"},
	{"support", "QA Support", "support_agent"},
	{"operations", "QA Operations", "operations_returns"},
	{"finance", "QA Finance", "finance_operator"},
}

type product struct {
	slug, name, description, category, image, sku string
	price                                         int64
}

// Each catalog image depicts the named item. Listing copy makes no claim about
// maker, provenance, materials, or available inventory that has not been verified.
var products = []product{
	{"weevcrafts-madhubani-tree", "Tree of Life Painting", "Decorative tree of life artwork.", "arts", "madhubani-tree.webp", "WC-TREE-001", 249900},
	{"weevcrafts-brass-elephant", "Elephant Figurine", "Decorative elephant figurine.", "crafts", "brass-elephant.webp", "WC-ELE-002", 189900},
	{"weevcrafts-maroon-saree", "Maroon Saree", "Maroon draped saree.", "textiles", "saree-maroon.webp", "WC-SAR-003", 549900},
	{"weevcrafts-blue-saree", "Blue Saree", "Blue draped saree.", "textiles", "saree-blue.webp", "WC-SAR-004", 549900},
	{"weevcrafts-wooden-box", "Decorative Wooden Box", "Decorative box for small items.", "home", "wooden-box.webp", "WC-BOX-005", 129900},
	{"weevcrafts-ceramic-mugs", "Decorative Mugs", "Pair of decorative mugs.", "home", "ceramic-mugs.webp", "WC-MUG-006", 129800},
	{"weevcrafts-blue-tote", "Blue Tote Bag", "Blue everyday tote bag.", "textiles", "blue-tote.webp", "WC-TOT-007", 89900},
	{"weevcrafts-terracotta-lamp", "Decorative Table Lamp", "Decorative table lamp.", "home", "terracotta-lamp.webp", "WC-LMP-008", 159900},
	{"weevcrafts-terracotta-planter", "Decorative Planter", "Decorative plant pot.", "home", "terracotta-planter.webp", "WC-PLN-009", 109900},
	{"weevcrafts-black-cushion", "Black Cushion", "Black decorative cushion.", "home", "black-cushion.webp", "WC-CSH-010", 79900},
}

func main() {
	if os.Getenv("WECRATFS_SEED_LAUNCH") != "true" {
		log.Fatal("set WECRATFS_SEED_LAUNCH=true to provision launch data")
	}
	passwords := make(map[string]string, len(accounts))
	for _, item := range accounts {
		key := "WECRATFS_" + strings.ToUpper(item.key) + "_PASSWORD"
		passwords[item.key] = os.Getenv(key)
		if passwords[item.key] == "" {
			log.Fatalf("missing %s", key)
		}
	}
	stock := 0
	if value := os.Getenv("WECRATFS_SEED_VERIFIED_STOCK"); value != "" {
		var err error
		stock, err = strconv.Atoi(value)
		if err != nil || stock < 0 {
			log.Fatal("WECRATFS_SEED_VERIFIED_STOCK must be a nonnegative integer")
		}
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
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
	hasher := auth.NewArgon2idHasher()
	hashes := make(map[string]string, len(accounts))
	for _, item := range accounts {
		hashes[item.key], err = hasher.Hash(passwords[item.key])
		if err != nil {
			log.Fatal(err)
		}
	}
	if err := postgres.WithinTransaction(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		var sellerID int64
		accountIDs := make(map[string]int64, len(accounts))
		for _, item := range accounts {
			email := item.key + "@weevcrafts.test"
			var id int64
			if err := tx.QueryRow(ctx, `INSERT INTO users (email, display_name, status, email_verified_at) VALUES ($1,$2,'active',NOW()) ON CONFLICT (email) DO UPDATE SET display_name=EXCLUDED.display_name, status='active', email_verified_at=NOW() RETURNING id`, email, item.name).Scan(&id); err != nil {
				return fmt.Errorf("user %s: %w", email, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO credentials (user_id,password_hash) VALUES ($1,$2) ON CONFLICT (user_id) DO UPDATE SET password_hash=EXCLUDED.password_hash, password_changed_at=NOW(), failed_attempts=0, locked_until=NULL`, id, hashes[item.key]); err != nil {
				return fmt.Errorf("credentials %s: %w", email, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id,role_slug) VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, item.role); err != nil {
				return fmt.Errorf("role %s: %w", email, err)
			}
			accountIDs[item.key] = id
			if item.key == "seller" {
				if err := tx.QueryRow(ctx, `INSERT INTO sellers (owner_user_id,display_name,status) VALUES ($1,'Weevcrafts Seller','active') ON CONFLICT (owner_user_id) DO UPDATE SET status='active' RETURNING id`, id).Scan(&sellerID); err != nil {
					return fmt.Errorf("seller: %w", err)
				}
			}
		}
		if sellerID == 0 {
			return fmt.Errorf("seller account missing")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO seller_users (seller_id,user_id,status,permissions,added_by) VALUES ($1,$2,'active',$3,$4) ON CONFLICT (seller_id,user_id) DO UPDATE SET status='active',permissions=EXCLUDED.permissions`, sellerID, accountIDs["sellerstaff"], []string{"PRODUCT_READ", "PRODUCT_WRITE", "INVENTORY_READ", "INVENTORY_WRITE", "ORDER_READ", "ORDER_FULFILL", "RETURN_READ", "REPORT_READ"}, accountIDs["seller"]); err != nil {
			return fmt.Errorf("seller staff: %w", err)
		}
		for _, item := range products {
			if _, err := tx.Exec(ctx, `INSERT INTO categories (slug,name) VALUES ($1,initcap($1)) ON CONFLICT DO NOTHING`, item.category); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO brands (slug,name) VALUES ('weevcrafts','Weevcrafts') ON CONFLICT DO NOTHING`); err != nil {
				return err
			}
			var productID int64
			if err := tx.QueryRow(ctx, `INSERT INTO products (slug,brand_slug,category_slug,name,description,price_cents,image_url,seller_id,status) VALUES ($1,'weevcrafts',$2,$3,$4,$5,$6,$7,'approved') ON CONFLICT (slug) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, price_cents=EXCLUDED.price_cents, image_url=EXCLUDED.image_url, status='approved' RETURNING id`, item.slug, item.category, item.name, item.description, item.price, "/assets/images/customer/"+item.image, sellerID).Scan(&productID); err != nil {
				return fmt.Errorf("product %s: %w", item.slug, err)
			}
			var variantID int64
			if err := tx.QueryRow(ctx, `INSERT INTO product_variants (product_id,sku,price_cents) VALUES ($1,$2,$3) ON CONFLICT (sku) DO UPDATE SET price_cents=EXCLUDED.price_cents RETURNING id`, productID, item.sku, item.price).Scan(&variantID); err != nil {
				return fmt.Errorf("variant %s: %w", item.sku, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO inventory_stock (variant_id,available_quantity) VALUES ($1,$2) ON CONFLICT (variant_id) DO NOTHING`, variantID, stock); err != nil {
				return fmt.Errorf("stock %s: %w", item.sku, err)
			}
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Provisioned %d role accounts and %d catalog listings (new listing stock: %d).\n", len(accounts), len(products), stock)
}
