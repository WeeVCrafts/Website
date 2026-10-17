# Brand search discovery — 17 October 2026

The preferred public name is **WeevCrafts**, on `https://weevcrafts.com/`.
The homepage sends this identity in its title, description, Open Graph metadata,
one WebSite node and one linked Organization node. Recognized alternate spellings
and the domain are supplied once each; social profiles and claims are not invented.

## Findings and changes

The live homepage allows indexing and HTTP redirects to HTTPS. Its canonical URL,
robots.txt and XML sitemap identify the same root domain. The brand metadata was
already present in deployment e429390 from 16 October, while the retrieved search
result still used the older "Handmade Today" title. This supports a stale-result
hypothesis; it does not prove Google's selected canonical or current ranking.

A verified code defect silently omitted maker pages: sitemap generation queried
`sellers.slug`, which does not exist. It now uses display names and the exact shared
public maker URL convention. Product, category and brand entries require approved
products from active sellers; category membership uses the existing relation.
A failed query returns retryable HTTP 503 instead of publishing a partial sitemap.
Regression checks exercise actual PostgreSQL queries, maker inclusion, structured
identity, canonical redirects and failure recovery.

`www.weevcrafts.com` does not currently resolve and is not attached to Railway.
The root domain works. This does not prevent indexing the canonical root domain.
Adding the optional www alias requires a Railway domain, TLS certificate and a DNS
record at the authoritative DNS provider; no unverified DNS change is made here.

## Verified Search Console property: operator follow-up

The owner confirmed the property is verified. This session has no connected Search
Console tool, and local browser access is disabled, so no Search Console inspection
or indexing request is claimed.

After this release:

1. Open the existing property in https://search.google.com/search-console .
2. Inspect `https://weevcrafts.com/`, compare Google's selected canonical with the
   declared root URL, and run **Test live URL**, then **Request indexing**.
3. Submit or resubmit `https://weevcrafts.com/sitemap.xml` in **Sitemaps**.
4. In Performance, filter queries containing `weevcrafts`, compare country/device
   segments and record impressions, position and clicks after the next crawl.
5. Inspect any exclusion or manual-action report before changing canonicals or
   blocking additional paths. Avoid repeated indexing requests for the same URL.

Google controls indexing and ranking; capitalization changes alone do not create
separate brand rankings. A code deployment cannot guarantee immediate visibility.
Google documents that recrawling can take days to weeks and does not guarantee
inclusion. No paid links, keyword stuffing, or unrelated indexing API is used.

References:
- https://developers.google.com/search/docs/appearance/site-names
- https://developers.google.com/search/docs/crawling-indexing/ask-google-to-recrawl
