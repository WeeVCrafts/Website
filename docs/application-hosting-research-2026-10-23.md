# WeevCrafts: complete application hosting comparison

Research checked: 23 October 2026. Scope: hosting the existing Go application, website, API, background processing, PostgreSQL, Redis, and media storage. Recommendations are assessments for this repository; prices are published rates or explicitly labeled estimates, not provider quotations.

## Recommendation

Keep Railway on the shortlist because deployment configuration already exists. Evaluate Render for managed data services and Northflank for an integrated container platform. DigitalOcean is a credible fixed-price alternative, with an important new database-tier restriction. Fly.io is attractive when geography or its managed HA Postgres justifies the additional operational work. Sliplane deserves a trial for predictable server-based pricing. For the lowest infrastructure bill, consider a VPS with Coolify or Dokploy, accepting responsibility for the database and recovery.

There is no single cheapest provider independently of resource consumption, bandwidth, recovery requirements, and operator time. A reasonable launch planning envelope is approximately **$30–80/month for hosted platform setups** or **$15–45/month for a small self-managed server**, before paid email/search, tax, and engineering labor. These envelopes are estimates, not demonstrated capacity or availability guarantees.

This is a broad comparison of the relevant deployment families and representative providers, rather than a claim to enumerate every hosting company worldwide.

## What this application actually needs

Repository evidence: [README](../README.md), [architecture](architecture.md), [deployment requirements](deployment-security.md), [Firebase integration](firebase-integration.md), [Dockerfile](../Dockerfile), and [composition root](../cmd/website/main.go).

| Requirement | Deployment consequence |
|---|---|
| One Go service renders templ HTML and serves `/api/v1` | One application deployment covers the website and API. A separate frontend host is unnecessary. |
| Node/Tailwind/templ execute during the image build | The production service runs the compiled Go binary. Preserve the existing multi-stage Dockerfile. |
| PostgreSQL holds business data | Use PostgreSQL with the existing SQL schema and migrations; another database product is not automatically compatible. |
| Redis backs sessions and rate limits | A live Redis-compatible connection is required. An HTTP-only KV API is not an equivalent configuration change. |
| S3-compatible private media | Configure an S3 endpoint and credentials; retain access checks for private media. |
| Periodic jobs run every 10–60 seconds | Keep a process running with usable CPU, or deliberately move jobs to a reliable worker/scheduler. |
| PostgreSQL migrations run at startup | Multiple replicas and rolling deployments need coordinated migration behavior. |
| Production validates TLS and secure cookies | Preserve `sslmode=verify-full`, trusted CA handling, HTTPS, and secure object storage. |
| Firebase identity/push and Resend already integrate | Changing hosting does not require replacing those providers. |
| Meilisearch is optional, with PostgreSQL fallback | A separate search service can be deferred at launch. |

The local Codebase Memory CLI worked. Its recorded graph generation was 16 October and reported changed metadata for `cmd/website/main.go`; the relevant current source was therefore read directly. No runtime usage measurements, cloud invoices, load tests, or deployment trials were performed for this research.

## How to read the costs

The launch comparison assumes one always-on application instance, approximately 512 MB–1 GB application memory, a small PostgreSQL instance, a small Redis service, 10 GB of database data, 10 GB of media, and 100 GB/month of outbound application traffic. Provider plan sizes differ; these are comparable deployment examples, not identical CPU performance benchmarks.

The budget ranges allow some room for storage, backups, and usage variation. They do not promise HA. Domain registration, payment processing, GST/VAT, foreign-exchange/card fees, paid support, engineering time, and a continuously running staging copy are excluded. Existing Firebase features and Resend are assumed to remain within their applicable no-cost quotas. Paid email/search are itemized later.

For hourly rates, calculations use 720 hours unless stated otherwise. Some vendors publish 730/744-hour equivalents or monthly caps. Always use the selected region and checkout configuration for the final quote. Temporary trial/startup credits are not recurring savings.

## 1. Platforms that can run the current Docker application

| Platform | Published pricing anchors | Estimated launch budget | Fit and main tradeoff |
|---|---|---|---|
| **Railway** | Hobby $5 minimum usage; Pro $20 minimum usage with included credits. RAM about $10/GB-month, consumed CPU about $20/vCPU-month, volumes about $0.15/GB-month, service egress $0.05/GB; buckets $0.015/GB-month with free bucket egress. | **$30–60/month** | Smallest migration effort. CPU/memory consumption determines the bill. Pro's larger storage limit matters for this example. [Pricing](https://railway.com/pricing), [plans](https://docs.railway.com/pricing/plans). |
| **Render** | App $7/512 MB or $25/2 GB; PostgreSQL $6/256 MB or $19/1 GB; paid Key Value $10/256 MB. Hobby workspace $0; Pro workspace $25. | **$45–80/month** with a 1 GB database; more with a larger app/team plan | Strong managed-service alternative. Count workspace fees, database disk, and outbound traffic separately. [Compute prices](https://render.com/pricing), [workspace changes](https://render.com/changelog/updated-plans-for-render-workspaces). |
| **DigitalOcean App Platform** | App $5/512 MiB, $10/fixed 1 GiB, $12/scalable 1 GiB; PostgreSQL table starts $15.15; Valkey $15; Spaces $5. | **$45–65/month** for a small single-node setup | Predictable product prices and one cloud account. Small app tiers have scaling limits; new database HA deployments require Advanced Edition. [App pricing](https://www.digitalocean.com/pricing/app-platform), [data services](https://www.digitalocean.com/pricing/managed-databases), [Spaces](https://www.digitalocean.com/pricing/spaces-object-storage). |
| **Northflank** | Published container size: 0.5 vCPU/1 GB $12/month; 1 vCPU/2 GB $24. Disk $0.11657/GiB-month; egress $0.06/GB. | **$45–70/month** for app + PostgreSQL + Redis allocations and external storage | Good integrated container/services workflow. Database replicas and resource allocations are billable. Free sandbox allows only one database, so it does not cover both required data services by itself. [Pricing](https://northflank.com/pricing). |
| **Fly.io** | 1 GB app: $6.70 in Ashburn or $8.50 Singapore per 30 days. Managed Postgres Basic $38 + $0.28/GB-month. Redis and object storage are additional. | **$55–80/month** using managed Postgres and small external Redis/storage | Useful geographic flexibility. Managed Postgres includes HA/backups; this is a different reliability tier from a single-node entry database. [Resource pricing](https://docs.fly.io/about/pricing), [managed Postgres](https://docs.fly.io/postgres). |
| **Sliplane** | Displayed Germany runtime: 2 GB €17.80 or 4 GB €28.80; managed PostgreSQL €19 with 10 GB; object storage €5 per additional 250 GB block after first free GB. | **€40–65/month** using managed PostgreSQL and Redis on runtime capacity | Server-based pricing lets several services share a capacity allocation. Displayed region/billing selection matters; Singapore prices require checking. [Pricing](https://sliplane.io/pricing). |
| **Koyeb** | New accounts require Pro or higher: Pro $29/month includes $10 compute. Eco 1 GB $5.36/month; small Postgres displayed $29.76/month + $0.50/GB-month. | **$60–90/month**, depending on runtime/Redis configuration | Technically compatible, but new-account economics differ from older free-tier articles. Product is transitioning toward Mistral Compute. [Pricing](https://www.koyeb.com/pricing), [instances](https://www.koyeb.com/docs/reference/instances), [account changes](https://www.koyeb.com/blog/koyeb-is-joining-mistral-ai-to-build-the-future-of-ai-infrastructure). |
| **Heroku** | Always-on Basic dyno $7; Postgres Essential-1 $9/10 GB; Key-Value Mini $3/25 MB, Premium-0 $15/50 MB. Object storage additional. | **$20–50/month** at very small limits; stronger data/compute tiers cost more | Compatible, but small connection/cache limits are material. Sustaining-engineering strategy makes it a secondary greenfield choice. [Pricing](https://www.heroku.com/pricing/), [March 2026 strategy update](https://www.heroku.com/blog/march-2026-update/). |

Railway's platform handles official template provisioning and image security patches. Backups/PITR, HA, pooling, and major upgrades need user action; customized database internals can move outside its template support scope. A plan's availability target is not proof that the application or its database is configured for HA. [Database responsibilities](https://docs.railway.com/databases).

DigitalOcean changed new Standard Edition PostgreSQL/MySQL availability on **15 October 2026** for accounts that had never created either engine: new clusters cannot add standby/read-only nodes or exceed 4 GiB. Advanced Edition is required for those features. Existing qualifying clusters are treated differently. Do not use older $60 HA examples as a quote for a new account. [Official change notice](https://docs.digitalocean.com/release-notes/upcoming/dbaas-plan-changes/).

## 2. Worked complete-deployment examples

### Railway: usage-based example

Assume average combined memory of 1.75 GB across app, PostgreSQL, and Redis; combined consumed CPU of 0.15 vCPU; 20 GB persistent volumes; 10 GB bucket storage; and 100 GB application egress:

| Item | Approximate monthly cost |
|---|---:|
| Memory: 1.75 × $10 | $17.50 |
| Consumed CPU: 0.15 × $20 | $3.00 |
| Volumes: 20 × $0.15 | $3.00 |
| Bucket: 10 × $0.015 | $0.15 |
| App egress: 100 × $0.05 | $5.00 |
| **Resource subtotal** | **$28.65** |

The Pro minimum is covered by this usage; it is not another $20 added to the subtotal. Allow **$35–60** for backup storage and usage variation. Memory/CPU assumptions are illustrative and must be replaced with observed metrics. [Billing plans](https://docs.railway.com/pricing/plans).

### Render: fixed service prices, variable bandwidth

| Item | Monthly example |
|---|---:|
| App 512 MB | $7.00 |
| PostgreSQL 1 GB RAM | $19.00 |
| Database disk expansion: 9 GB beyond included 1 GB | $2.70 |
| Key Value 256 MB | $10.00 |
| R2 media: 10 GB and operations within free allowances | $0.00 |
| Hobby workspace | $0.00 |
| 100 GB app egress: (100 − 5) × $0.15 | $14.25 |
| **Subtotal** | **$52.95** |

On a Pro workspace, replacing the workspace and bandwidth rows gives **$74.95**. A 2 GB app adds $18. This example retains background jobs inside the Go service, so it does not add a second worker service. Object backup/version storage can add cost. [Render pricing](https://render.com/pricing), [bandwidth allowances](https://render.com/docs/new-workspace-plans), [R2 pricing](https://developers.cloudflare.com/r2/pricing/).

### DigitalOcean: one cloud account

The published entry combination is $5 app + approximately $15–18 PostgreSQL + $15 Valkey + $5 Spaces, or **approximately $40–43 before adjustments**. A 512 MiB app's included 50 GiB transfer means 100 GiB delivery adds about $1 at $0.02/GiB. Budget **$45–65**, allowing a larger app and disk/backup differences. The database pricing overview and documentation differ slightly in their starting figure; use the selected region's live quote to confirm the included disk and final amount. [App rates](https://www.digitalocean.com/pricing/app-platform), [PostgreSQL pricing documentation](https://docs.digitalocean.com/products/databases/postgresql/details/pricing/).

### Northflank: explicit capacity allocations

Three published $12 allocations for app, PostgreSQL, and Redis total $36. Add 20 GiB disk at approximately $2.33 and 100 GB egress at $6: **approximately $44.33**, plus object storage and backup/log storage as applicable. Budget **$45–70**. Allocating a full 1 GB to Redis is conservative for a small store; actual selectable addon plans may allow a different allocation. [Resource rates](https://northflank.com/pricing).

### Fly.io: database HA included in the example

Singapore 1 GB app $8.50 + Basic managed Postgres $38 + 10 GB database usage $2.80 + 100 GB regional internet egress at $0.04/GB = **$53.30 before Redis, media, and other usage**. Adding small external Redis and storage makes **$55–80** a planning range. Confirm that managed Postgres and Redis are available sufficiently close to the chosen app region. [Machine/network prices](https://docs.fly.io/about/pricing), [Postgres rates and regional reference](https://docs.fly.io/postgres).

### Sliplane: managed database versus shared-server deployment

Using the displayed Germany prices: €17.80 runtime for app/Redis + €19 managed PostgreSQL + €5 storage = **€41.80**, subject to runtime fit and selected billing. A 4 GB runtime makes it **€52.80**. Running PostgreSQL on that runtime instead can reduce the invoice, but it changes who manages the database. Neither arrangement automatically provides database HA. [Live pricing](https://sliplane.io/pricing).

## 3. VPS and virtual-machine alternatives

A VPS can run the Go app, PostgreSQL, Redis, and optionally Meilisearch using containers, with object storage external or on the server. The existing development Compose file needs production configuration; it is not a production deployment manifest. Start by evaluating **4 GB RAM**, with 2 GB only after measuring the workload and building images elsewhere. This is a sizing recommendation, not a measured requirement.

| Provider | Published price examples | What to budget/consider |
|---|---|---|
| **DigitalOcean Droplet** | 2 GB $12; 4 GB $24/month | Approximately **$18–40/month** with small external storage and backup provision. Database operations remain yours. [Pricing](https://www.digitalocean.com/pricing/droplets). |
| **AWS Lightsail** | Public IPv4 Linux: 2 GB $12; 4 GB $24; snapshots $0.05/GB-month | Approximately **$18–40/month**, depending on snapshot/media storage. Its cheapest managed database tier lists no data encryption, which conflicts with this project's requirements. [Pricing](https://aws.amazon.com/lightsail/pricing/). |
| **Hetzner** | June price notice: Europe CX23 $6.49, CX33 $9.99; Singapore CPX12 $17.99, CPX22 $30.99, excluding IPv4 and tax | Europe and Singapore have different economics. Budget approximately **$15–30 Europe / $25–50 Singapore** including small backup/media allowances, conditional on availability and sufficient capacity. [Current price adjustments](https://docs.hetzner.com/general/infrastructure-and-availability/price-adjustment/). |
| **Akamai/Linode** | 1 GB Nanode $5; 8 GB $48/month | Compatible VM route. Use its current calculator for an intermediate 2/4 GB size; this research did not verify those exact tariffs. [Pricing](https://www.akamai.com/cloud/pricing). |
| **Hostinger VPS** | KVM 1, 4 GB: displayed $6.49 promotional effective rate, $11.99 renewal; KVM 2, 8 GB: $8.79 promotional, $14.99 renewal | Attractive cash cost; compare commitment, renewal, region, and independent database backups. Prices are effective advertised rates, not a promise of monthly cancellable billing. [Offer and renewal](https://www.hostinger.com/vps-hosting). |
| **IONOS VPS** | 4 GB VPS M+: $5/month for first 3 months with one-year term; displayed regular $14. 8 GB: $8 promotional / $25 regular | Compare full-term spend, not first-three-month pricing. Operator responsibility resembles other VPSs. [Pricing](https://www.ionos.com/servers/vps). |
| **E2E Networks** | C3 8 GB with 100 GB disk: $35.64/month; S3 storage first 250 GB $10 flat; transfer $0.045/GB | Indian-provider alternative to quote when local support/location matter. Add backup and transfer; verify the selected actual region. [Pricing](https://www.e2enetworks.com/pricing). |
| **Oracle OCI** | Always Free offers eligible compute, subject to quotas/capacity; paid compute and storage are metered | Suitable for experimentation or a carefully operated VM deployment. Idle free compute may be reclaimed; do not base a paid marketplace's recovery plan on guaranteed free capacity. [Free resource conditions](https://docs.oracle.com/en-us/iaas/Content/FreeTier/freetier_topic-Always_Free_Resources.htm), [paid price list](https://www.oracle.com/cloud/price-list/). |
| **Vultr** | Public pricing page returned HTTP 403 in this research | Technically relevant VM family; current price was **not verified**, so no numerical total is ranked. [Official pricing](https://www.vultr.com/pricing/). |
| **OVHcloud** | Region/shape-specific VM, managed data, and object-storage rates | A valid cloud/VPS family, but no normalized small full-stack quote was verified here. [Official price list](https://www.ovhcloud.com/en/public-cloud/prices/). |

Single-server deployments have a shared failure domain. Backups help recovery but do not provide automatic failover. A VM image snapshot alone does not establish a tested, application-consistent PostgreSQL recovery process. Include database-aware backups and a separate copy of media.

## 4. Deployment panels and bring-your-own-cloud options

These manage infrastructure you supply. Their subscription is additional to servers, storage, network, and managed databases.

| Tool/platform | Published management price | Suitability |
|---|---|---|
| **Coolify** | Self-hosted free; Cloud $5/month for two connected servers, then $3/additional server | Strong Docker-oriented VPS workflow. Cloud hosts the panel; it does not include your app servers or automatically transfer database-operation responsibility. [Pricing](https://coolify.io/pricing). |
| **Dokploy** | Cloud page displays Hobby $4.50/server/month, with a yearly/monthly billing selector | Compose-oriented candidate. Confirm selected billing period and server charges before purchase. [Pricing](https://dokploy.com/pricing). |
| **CapRover** | Free self-hosted software | Docker/Swarm/nginx route with HTTPS automation. You operate the hosts and data services. [Official site](https://caprover.com/). |
| **Dokku** | Open-source self-hosted core; optional Pro | Smaller Git-to-deploy platform with Docker-based isolation. Check plugin/backup operations for the actual deployment. [Official site](https://dokku.com/). |
| **Cloud 66** | Free tier up to one server; Developer displayed $23/month up to two, with billing selector | Managed deployment layer on your own infrastructure. Verify feature and billing selection. [Pricing](https://www.cloud66.com/pricing/). |
| **Zeabur** | Free own-server management tier; Dev $5/month; Pro $19/month; Team $79/month | Current page distinguishes management plan from runtime/server purchases. Do not treat its subscription as an entire application hosting bill. [Pricing](https://zeabur.com/pricing). |
| **Qovery** | Starter $299/month billed yearly, plus cloud resources | Relevant for larger teams managing their own cloud; expensive for one small marketplace. [Pricing](https://www.qovery.com/pricing). |
| **Northflank BYOC** | $0.02083/node-vCPU-hour management rate on pricing page, plus your cloud bill; sandbox conditions differ | Consider when cloud-account ownership and platform controls become a requirement. [Pricing](https://northflank.com/pricing). |

## 5. Hyperscalers and newer serverless/container choices

| Platform | How a complete deployment would work | Pricing and fit |
|---|---|---|
| **Google Cloud Run** | Go container + Cloud SQL + Memorystore + object storage; or external PostgreSQL/Redis | Tier-1 instance-based rate $0.000018/vCPU-second and $0.000002/GiB-second. A continuously allocated 1 vCPU/1 GiB service is about **$51.84/month before free allowances**, then add data/network services. All-Google managed deployments can be much higher than the app bill. [Pricing](https://cloud.google.com/run/pricing). |
| **AWS ECS Express Mode / Fargate** | Container + RDS PostgreSQL + ElastiCache/Valkey + S3; Express Mode provisions load balancing/networking | Express Mode has no extra fee, but its underlying resources are charged. Illustrative US-East 0.25 vCPU/0.5 GB Fargate compute is about **$8.89/720 hours**, before load balancer, data services, IPv4, logs, and network. A complete managed bill needs a regional calculator quote. [Express Mode](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/express-service-overview.html), [Fargate pricing](https://aws.amazon.com/fargate/pricing/). |
| **AWS Elastic Beanstalk / EC2** | Docker on EC2, with external managed database/cache/storage or deliberately operated local services | Compatible alternative to direct ECS. Total depends on instances, disks, load balancer, RDS, cache, and network. No verified fixed full-stack quote is claimed. [Docker support](https://docs.aws.amazon.com/elasticbeanstalk/latest/dg/create_deploy_docker.html). |
| **Azure Container Apps** | Container + Azure PostgreSQL + Azure Managed Redis; S3 storage externally or a storage adapter change | Consumption includes monthly grants of 180k vCPU-seconds, 360k GiB-seconds, and 2m requests. A minimum replica is needed for the current periodic workers. Regional rates and data services require a quote; the free grant is not an entire free stack. [Billing](https://learn.microsoft.com/en-us/azure/container-apps/billing), [runtime](https://azure.microsoft.com/en-us/pricing/details/container-apps/), [Postgres](https://azure.microsoft.com/en-us/pricing/details/postgresql/flexible-server/), [Managed Redis](https://azure.microsoft.com/en-us/pricing/details/managed-redis/). |
| **Scaleway Serverless Containers** | Go container plus separately provisioned PostgreSQL, Redis, and object storage | Paris displayed €0.00001/vCPU-second + €0.000002/GB-second, after allowances. **€31.10** for continuous 1 vCPU/1 GB before allowances and data services. Evaluate geography and worker execution semantics. [Pricing](https://www.scaleway.com/en/pricing/serverless/). |
| **Cloudflare Containers** | Existing Go image with Worker/Durable Object routing, external PostgreSQL/Redis, R2 media | Starts with $5 Workers Paid, then container memory/CPU/disk and routing usage. CPU $0.000020/vCPU-second; memory $0.0000025/GiB-second beyond included usage. This is a real Go-container possibility, but needs a deployment integration and lifecycle test. [Overview](https://developers.cloudflare.com/containers/), [pricing](https://developers.cloudflare.com/containers/platform/pricing/). |
| **Vercel Go / Services / Daemons** | Go server support and Services are available in beta; continuous jobs belong in Daemons rather than assumed request execution | Pro advertised $20/month plus usage; PostgreSQL/Redis/storage are separate services/providers. The beta routes make this possible to evaluate, with worker/build changes and a fresh quote. [Go runtime](https://vercel.com/docs/functions/runtimes/go), [Services](https://vercel.com/docs/services), [Daemons](https://vercel.com/docs/daemons), [pricing](https://vercel.com/pricing). |
| **Netlify** | Functions/hosting plus data services; adapting continuous Go jobs would require additional work | Personal $9/month; Pro starts $20/month with credit-based usage. Not the first migration candidate for the current long-lived container. [Functions](https://docs.netlify.com/build/functions/overview/), [pricing](https://www.netlify.com/pricing/). |
| **Kubernetes** | Any suitable managed cluster or self-managed nodes, with PostgreSQL/Redis operators or managed external data | Compatible but usually adds unnecessary platform work for the present single-service application. Nodes, load balancers, disks, backups, and sometimes cluster management are separate costs. No fixed total is implied. |

Cloud Run request-based billing does not supply the same CPU execution semantics as an always-on server. For current background goroutines, use a suitable instance-based/minimum-instance arrangement or extract work into jobs/queues. Setting a warm instance alone does not establish background CPU allocation. [Google's billing/background guidance](https://docs.cloud.google.com/run/docs/configuring/billing-settings).

**AWS App Runner is not a new-customer option as of this research date.** It stopped accepting new customers on 30 April 2026. Existing customers can continue operating; AWS recommends ECS Express Mode. [Official availability notice](https://docs.aws.amazon.com/apprunner/latest/dg/apprunner-availability-change.html).

## 6. Mixing providers can reduce cost, with tradeoffs

You can host the Go application on Railway, Render, Fly, or a VPS while sourcing the database/cache/storage elsewhere. This is a full deployment assembled from services, rather than one all-inclusive platform.

| Component | Useful options and published pricing | Integration considerations |
|---|---|---|
| PostgreSQL | Neon Launch $0.106/CU-hour + $0.35/GB-month. Supabase Pro $25/month with a credit covering one Micro; 8 GB DB allowance. Aiven Developer $5 / Hobbyist from $12. Crunchy Bridge headline from $10/month. | Current periodic SQL work may prevent a serverless database sleeping. Keep app/data close; use real PostgreSQL connections, certificate verification, suitable pooling, and migration credentials. [Neon](https://neon.com/pricing), [Supabase](https://supabase.com/pricing), [Aiven](https://aiven.io/pricing/postgresql), [Crunchy](https://www.crunchydata.com/pricing). |
| Redis | Upstash Free 500k commands/month; pay-as-you-go $0.20/100k commands, fixed plans displayed from $10. DigitalOcean Valkey starts $15. | Use Redis protocol over TLS with the current Go client; verify commands, Lua behavior, eviction, timeouts, and connection limits. 1m billed commands is $2, not 1m website requests. Production SLA/HA may require an add-on. [Upstash](https://upstash.com/pricing/redis), [Valkey](https://docs.digitalocean.com/products/databases/valkey/details/pricing/). |
| Object storage | R2: $0.015/GB-month, 10 GB free Standard storage, operation allowances, free bucket egress. Spaces $5 including 250 GiB and 1 TiB transfer. B2 from $6.95/TB-month with egress allowance up to 3× stored volume. | R2 is a plausible S3 adapter fit, but the application's proxy still incurs app-host egress. Test actual S3 operations and backup/versioning. [R2](https://developers.cloudflare.com/r2/pricing/), [Spaces](https://www.digitalocean.com/pricing/spaces-object-storage), [B2](https://www.backblaze.com/cloud-storage/pricing). |

Aiven's $5 Developer plan does not let you choose the region; that is a material compromise for an India-facing storefront. Supabase's included daily backup is different from PITR, which starts at an additional $100/month. Entry prices therefore do not establish equivalent recovery capabilities. [Aiven plan conditions](https://aiven.io/pricing/postgresql), [Supabase add-ons](https://supabase.com/pricing).

For Neon, an illustrative always-awake 0.25 CU costs 0.25 × 720 × $0.106 = **$19.08 compute/month**, plus 10 GB at $0.35 = **$3.50**, before history/other usage. This is why an idle-user storefront with periodic database workers may not receive the savings implied by a serverless label. Actual sizing and billing depend on the plan and measured activity. [Neon rates](https://neon.com/pricing).

Supabase, Firebase, Appwrite, Convex, and similar backend products should not be counted as complete hosting for this Go application by their base subscription alone. Their databases/auth/storage can be useful components. Replacing the marketplace's business services with their APIs is a separate architecture decision. This report's deployment recommendations preserve the current application.

## 7. Costs common to all choices

| Item | Cost treatment |
|---|---|
| Transactional email | Existing Resend Free includes 3,000/month but only 100/day. Pro is $20/month for 50,000. Add $20 if operational peaks exceed the daily cap even below 3,000 monthly. [Pricing](https://resend.com/pricing?volume=50000). |
| Search | Existing PostgreSQL fallback avoids another hosting service. Meilisearch Cloud advertises from $20/month; the displayed XS resource example is $23 including disk. [Pricing](https://www.meilisearch.com/pricing). |
| Firebase | Keep existing identity/push integration; applicable no-cost limits differ by product and Identity Platform status. SMS is separate. Do not upgrade or migrate identity solely to move hosting. [Pricing](https://firebase.google.com/pricing). |
| Backups | Include encrypted database backups and independent media copies; verify retention, storage charges, and a real restore. Cheap entry plans are not equivalent to HA/PITR plans. |
| Logs/builds/monitoring | Check retention, build quotas, registry storage, health probes, and log export. A free observability allowance may become billable with frequent jobs or verbose logs. |
| Staging | A continuous second application/database environment can add a substantial fraction of the production bill. Pause disposable staging when appropriate. |
| Payments/domain/tax | Payment gateway fees and domain renewal remain separate from infrastructure. Use actual billing currency, tax, and card conversion at purchase. |

## 8. Bandwidth changes the ranking

The present app proxies uploaded images. At 100 GB outbound, Railway service transfer is $5; Render Hobby bandwidth overage is $14.25, or $11.25 on Pro before the $25 workspace fee. At 1 TB treated as 1,000 GB, those figures become $50 and $149.25 respectively. These are calculations from published transfer rates, not complete invoices. [Railway transfer rates](https://railway.com/pricing), [Render workspace rates](https://render.com/docs/new-workspace-plans).

Moving public product images to appropriate CDN delivery could change those economics. Private profile images still require access control. This is an optional application change, not an assumption built into the totals above.

## 9. Location and migration effort

For an India-facing audience, benchmark a complete Singapore deployment against an India-region alternative. Put the app, PostgreSQL, and Redis close together; do not choose each component's cheapest region independently. Railway and Render list Singapore; Supabase lists Mumbai and Singapore, while Neon's current region list includes Singapore. Geography alone does not establish measured latency. [Railway regions](https://docs.railway.com/deployments/regions), [Render regions](https://render.com/docs/regions), [Supabase regions](https://supabase.com/docs/guides/platform/regions), [Neon regions](https://neon.com/docs/introduction/regions).

| Route | Relative migration work | Work to validate |
|---|---|---|
| Stay on Railway | Lowest | Measure usage, configure recovery/HA as required, check region and storage limits. |
| Render / Northflank / DigitalOcean / Fly / Sliplane | Low–medium | Provision data services, transfer PostgreSQL/media, configure secrets/TLS, run Docker health checks, verify jobs and callbacks. |
| VPS + deployment panel | Medium–high operational work | Production networking, PostgreSQL TLS/backups/upgrades, Redis isolation, disk monitoring, image builds and recovery. |
| Cloud Run / ECS / Azure | Medium–high setup work | Service identity, network paths, data-service sizing, logs, scaling and background execution. |
| Vercel / Cloudflare container integration / function-based hosts | Higher validation uncertainty | Entrypoint/build routing, lifecycle semantics, durable jobs, connections and current beta/product constraints. |

Before a move, validate upload/read/delete, checkout webhook idempotency, login/session revocation, emails, periodic cleanup, connection recovery, and backup restoration in an isolated environment. Those checks are part of migrating a marketplace, not evidence that any platform has already been tested here.

## Decision for WeevCrafts

1. **Least disruption:** keep Railway and measure its real bill before migrating for speculative savings.
2. **Strong managed-platform alternative:** trial Render with app/database/cache in Singapore, budgeting its actual delivery traffic and workspace tier.
3. **Integrated container alternative:** trial Northflank with an explicit resource and backup quote.
4. **Fixed-price cloud account:** consider DigitalOcean, accounting for the October 2026 database HA restrictions.
5. **Predictable server-priced platform:** test Sliplane, checking regional price and runtime capacity.
6. **Lowest server cash cost:** compare a 4 GB VPS plus Coolify/Dokploy, off-server backups, and external media. Count operator work as part of the decision.
7. **Geographic/HA database preference:** Fly.io is worth evaluating, particularly when its included managed Postgres HA is valuable.

The current source already provides portability through Docker and standard infrastructure interfaces. A hosting move does not require changing Go, templ, Tailwind, PostgreSQL, or the accepted storefront design.
