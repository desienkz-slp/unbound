# CDN & GSLB Compatibility Rules

As an ISP recursive DNS platform, Netora DNS must ensure customer traffic to global and regional CDNs resolves without interruptions.

## 1. QNAME Minimisation & Empty Non-Terminal (ENT) Invariant
- **Standard vs. Reality:** RFC 7816 / RFC 9156 define QNAME Minimisation for privacy. However, many enterprise CDN nameservers and Global Server Load Balancers (GSLBs) — notably Baishan Cloud (`*.baishan-cloud.net`), Shopee GSLB (`*.sgw.shopeemobile.com`), Akamai GSLB, and AWS Route53 nested zones — violate this standard.
- **The Bug:** When an intermediate empty non-terminal (ENT) is queried, these authoritative servers erroneously respond with `NXDOMAIN` instead of `NODATA/NOERROR`.
- **Resolver Behavior:** If `qname-minimisation: yes` is enforced strictly on Unbound, Unbound treats the entire subtree as non-existent and returns `NXDOMAIN` to subscribers, causing broken images, failed streaming, and broken website components.
- **Rule:** For production ISP recursive resolvers, `qname-minimisation` MUST default to `no` unless upstream fallbacks are proven reliable in the target network environment.

## 2. Multi-CDN Resolution Diagnostics
When customers or tests report missing assets or slow rendering on high-traffic sites (e.g. Shopee, Tokopedia, TikTok, YouTube):
1. **Never Assume Resolver Failure:** Test the full domain CNAME chain.
2. **Authoritative Server Check:** Query the authoritative nameserver directly using `dig @<authoritative-ns> <domain> A` to compare responses with Unbound's cached result.
3. **Empty Non-Terminal Test:** Query intermediate labels directly to determine if the authoritative nameserver emits erroneous `NXDOMAIN`.
4. **Cache Invalidation:** After configuration changes, immediately invalidate affected CDN zones with `unbound-control flush_zone <zone>`.
