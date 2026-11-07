---
name: cloudflare-origin-reveal
description: "Correlate DNS, certificate, infrastructure, and response evidence to test direct-origin exposure"
---

# Cloudflare origin exposure assessment

Adapted from the KSIR-ARTEX origin-reveal playbook (`240ae7c`). The source considered DNS history, CT logs, MX/SPF clues, favicon and response fingerprints, Shodan/Censys, subdomain patterns, and active origin checks. These are leads, not ownership proof. The goal is to identify an exposed origin for the operator's service and measure the exact control difference; a candidate IP is never automatically in task scope.

## Gather candidates without contacting them

1. Save the protected hostname, one normal edge response, TLS certificate name, Cloudflare Ray or cache headers, and collection time.
2. Use `dig` for current A/AAAA/CNAME records and the operator's DNS history or passive DNS export for previous records. Certificate Transparency and passive search provider results may be attached as evidence; record their source and date.
3. Review adjacent application subdomains, mail/SPF infrastructure, and operator-provided cloud inventory only as clues. SPF senders, MX servers, a shared CDN address, or a neighboring IP are not by themselves web origins.
4. Compare favicon hashes, page title, certificate SANs, and response headers only when those observations were actually collected. Generic content and shared hosting produce false matches.
5. Rank candidates by independent evidence: an operator inventory mapping plus historical DNS is stronger than one passive search result. Record which clues contradict a candidate.

## Verify one exact candidate

1. Have the exact candidate hostname or IP recorded in the task's operator-entered scope and pass the interception gate. Keep the protected hostname in the TLS SNI and Host header; verify its certificate normally.
2. In an isolated test or approved direct-origin check, compare one read-only route through Cloudflare and the same route to the candidate using `curl --resolve protected.example.test:443:CANDIDATE_IP https://protected.example.test/ROUTE` through the configured capture path. Do not use `-k` or suppress certificate errors. If the capture path cannot route `--resolve` safely, mark the direct check blocked rather than switching silently to direct egress.
3. Compare body marker, response digest, certificate, server headers, and application request IDs. A generic default page is not an origin match.
4. Test one benign control difference only if the task permits it. A candidate that serves the same page directly is an exposure; an absent WAF rule, authentication boundary, or rate limit requires its own observed test.
5. Report confidence, exact candidate, scope, route, edge/origin evidence, and recommended origin firewall or authenticated origin pull. Keep any later application exploit as a separate scoped test.

Do not scan adjacent address ranges, trigger account registration or email, blindly trust a search-engine result, or infer an origin from a Cloudflare block page. Preserve DNS and response provenance so another operator can reproduce the conclusion.
