package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	domaincatalog "github.com/wecratfs/commerce/internal/domain/catalog"
	"github.com/wecratfs/commerce/internal/ports"
 "github.com/wecratfs/commerce/internal/domain/storefront"
)

type SearchService struct {
	repository ports.ProductSearchRepository
	cache      ports.Cache
	engine     ports.SearchEngine
	observer   ports.SearchObserver
	flight     searchFlight
}

const (
	SearchSortRelevance = ""
	SearchSortPriceAsc  = "price_asc"
	SearchSortPriceDesc = "price_desc"
	SearchSortNewest    = "newest"
)

func NewSearchService(repository ports.ProductSearchRepository, cache ports.Cache) *SearchService {
	return NewSearchServiceWithEngine(repository, cache, nil)
}

func NewSearchServiceWithEngine(repository ports.ProductSearchRepository, cache ports.Cache, engine ports.SearchEngine) *SearchService {
	return &SearchService{repository: repository, cache: cache, engine: engine}
}

func NewSearchServiceWithEngineAndObserver(repository ports.ProductSearchRepository, cache ports.Cache, engine ports.SearchEngine, observer ports.SearchObserver) *SearchService {
	return &SearchService{repository: repository, cache: cache, engine: engine, observer: observer}
}

func (s *SearchService) Search(ctx context.Context, query, category string, limit, offset int) ([]domaincatalog.Product, int, error) {
	return s.SearchWithOptions(ctx, query, category, SearchSortRelevance, limit, offset)
}

func (s *SearchService) SearchWithOptions(ctx context.Context, query, category, sort string, limit, offset int) ([]domaincatalog.Product, int, error) {
	products, total, _, err := s.SearchWithOptionsAndFacets(ctx, query, category, sort, limit, offset)
	return products, total, err
}

func (s *SearchService) SearchWithOptionsAndFacets(ctx context.Context, query, category, sort string, limit, offset int) ([]domaincatalog.Product, int, []ports.SearchFacet, error) {
	return s.SearchWithFilters(ctx, ports.ProductSearchOptions{Query: query, CategorySlug: category, Sort: sort, Limit: limit, Offset: offset})
}

func (s *SearchService) SearchWithFilters(ctx context.Context, options ports.ProductSearchOptions) ([]domaincatalog.Product, int, []ports.SearchFacet, error) {
	started := time.Now()
	if options.Limit <= 0 || options.Limit > 48 {
		options.Limit = 24
	}
	if options.Offset < 0 || options.Offset > 10000 {
		options.Offset = 0
	}
	options.Query = strings.TrimSpace(options.Query)
	options.Sort = normalizeSearchSort(options.Sort)
	options.PriceRange = normalizePriceRange(options.PriceRange)
	if options.CategorySlug != "" {
		options.CategorySlug = normalizeSlug(options.CategorySlug)
	}
	options.CategorySlugs = normalizeSlugs(options.CategorySlugs)
	options.BrandSlugs = normalizeSlugs(options.BrandSlugs)
	cacheKeyBytes, _ := json.Marshal(options)
	var revision int64
	if source, ok := s.repository.(interface {
		CatalogRevision(context.Context) (int64, error)
	}); ok {
		var err error
		revision, err = source.CatalogRevision(ctx)
		if err != nil {
			return nil, 0, nil, err
		}
	}
	rev,_:=storefront.Current(ctx)
 cacheKey := fmt.Sprintf("catalog:search:%d:%d:%x", rev.ID,revision, sha256.Sum256(cacheKeyBytes))
	if s.cache != nil && !options.DatabaseOnly && !options.InStock {
		if value, err := s.cache.Get(ctx, cacheKey); err == nil {
			var cached searchResult
			if json.Unmarshal(value, &cached) == nil {
				s.observe(ctx, ports.SearchObservation{Backend: "cache", CacheHit: true, Duration: time.Since(started), ResultCount: len(cached.Products)})
				return cached.Products, cached.Total, cached.Facets, nil
			}
		}
	}
	call, leader := s.flight.begin(cacheKey)
	if !leader {
		select {
		case <-call.done:
			return call.result.Products, call.result.Total, call.result.Facets, call.err
		case <-ctx.Done():
			return nil, 0, nil, ctx.Err()
		}
	}
	result, err := s.searchBackend(ctx, options, cacheKey)
	s.flight.finish(cacheKey, call, result, err)
	s.observe(ctx, ports.SearchObservation{Backend: result.Backend, Error: err != nil, Duration: time.Since(started), ResultCount: len(result.Products)})
	return result.Products, result.Total, result.Facets, err
}

func (s *SearchService) searchBackend(ctx context.Context, options ports.ProductSearchOptions, cacheKey string) (searchResult, error) {
	if s.engine != nil && !requiresDatabaseSearch(options) {
		indexed, engineErr := s.engine.Search(ctx, ports.SearchRequest{
			Query: options.Query, CategorySlug: options.CategorySlug, CategorySlugs: options.CategorySlugs,
			BrandSlugs: options.BrandSlugs, Sort: options.Sort, PriceRange: options.PriceRange,
			InStock: options.InStock, OffersOnly: options.OffersOnly, Limit: options.Limit, Offset: options.Offset,
		})
		if engineErr == nil {
			indexed.ProductSlugs = normalizeSearchSlugs(indexed.ProductSlugs)
			products, hydrateErr := s.repository.SearchBySlugs(ctx, indexed.ProductSlugs)
			if hydrateErr == nil {
				// Memberships, labels and discovery visibility are managed in
				// PostgreSQL; index facets still describe primary categories.
				facets := s.searchFacets(ctx, options)
				result := searchResult{Products: products, Total: indexed.Total, Facets: facets, Backend: "index"}
				if s.cache != nil && !options.DatabaseOnly && !options.InStock {
					if value, marshalErr := json.Marshal(result); marshalErr == nil {
						_ = s.cache.Set(ctx, cacheKey, value, 30)
					}
				}
				return result, nil
			}
		}
	}
	var products []domaincatalog.Product
	var total int
	var err error
	if advanced, ok := s.repository.(ports.ProductSearchOptionsRepository); ok {
		products, total, err = advanced.SearchWithOptions(ctx, options)
	} else {
		products, total, err = s.repository.Search(ctx, options.Query, options.CategorySlug, options.Limit, options.Offset)
	}
	if err != nil {
		return searchResult{}, err
	}
	result := searchResult{Products: products, Total: total, Facets: s.searchFacets(ctx, options), Backend: "postgresql"}
	if s.cache != nil && !options.DatabaseOnly && !options.InStock {
		if value, marshalErr := json.Marshal(result); marshalErr == nil {
			_ = s.cache.Set(ctx, cacheKey, value, 30)
		}
	}
	return result, nil
}

func requiresDatabaseSearch(options ports.ProductSearchOptions) bool {
	return options.CategorySlug != "" || len(options.CategorySlugs) > 0 || len(options.BrandSlugs) > 0 ||
		options.SelectedOnly || options.MinPriceCents>0 || options.MaxPriceCents>0 || options.PriceRange != "" || options.InStock || options.OffersOnly || options.DatabaseOnly || options.Sort != "" || options.Offset > 0
}

func (s *SearchService) searchFacets(ctx context.Context, options ports.ProductSearchOptions) []ports.SearchFacet {
	provider, ok := s.repository.(ports.ProductSearchFacetsRepository)
	if !ok {
		return nil
	}
	facets, err := provider.SearchFacets(ctx, options)
	if err != nil {
		return nil
	}
	return facets
}

func normalizePriceRange(value string) string {
	switch value {
	case "under-1500", "under-1000", "1000-2500", "2500-5000", "over-5000":
		return value
	default:
		return ""
	}
}

func normalizeSlugs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || len(value) > 120 {
			continue
		}
		valid := true
		for _, char := range value {
			if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-') {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeSlug(value string) string {
	values := normalizeSlugs([]string{value})
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (s *SearchService) observe(ctx context.Context, observation ports.SearchObservation) {
	if s.observer != nil {
		s.observer.ObserveSearch(ctx, observation)
	}
}

func normalizeSearchSort(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case SearchSortPriceAsc:
		return SearchSortPriceAsc
	case SearchSortPriceDesc:
		return SearchSortPriceDesc
	case SearchSortNewest:
		return SearchSortNewest
	default:
		return SearchSortRelevance
	}
}

type searchCall struct {
	done   chan struct{}
	result searchResult
	err    error
}

type searchFlight struct {
	mu    sync.Mutex
	calls map[string]*searchCall
}

func (f *searchFlight) begin(key string) (*searchCall, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = make(map[string]*searchCall)
	}
	if call, exists := f.calls[key]; exists {
		return call, false
	}
	call := &searchCall{done: make(chan struct{})}
	f.calls[key] = call
	return call, true
}

func (f *searchFlight) finish(key string, call *searchCall, result searchResult, err error) {
	f.mu.Lock()
	call.result = result
	call.err = err
	delete(f.calls, key)
	close(call.done)
	f.mu.Unlock()
}

func normalizeSearchSlugs(slugs []string) []string {
	result := make([]string, 0, len(slugs))
	seen := make(map[string]struct{}, len(slugs))
	for _, slug := range slugs {
		slug = strings.TrimSpace(slug)
		if slug == "" {
			continue
		}
		if _, exists := seen[slug]; exists {
			continue
		}
		seen[slug] = struct{}{}
		result = append(result, slug)
	}
	return result
}

type searchResult struct {
	Products []domaincatalog.Product `json:"products"`
	Total    int                     `json:"total"`
	Facets   []ports.SearchFacet     `json:"facets,omitempty"`
	Backend  string                  `json:"-"`
}
