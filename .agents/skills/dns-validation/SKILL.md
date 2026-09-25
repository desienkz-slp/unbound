---
name: dns-validation
description: Comprehensive end-to-end DNS quality validation workflow combining automated browser subagents for live website rendering (Shopee, streaming, media CDNs) with DNS CLI benchmarks and authoritative tracing.
---

# Skill: DNS Validation via Browser Subagent & CDN Diagnostics

Use this skill when:
- Validating a new Unbound deployment or configuration reload.
- Investigating customer reports of broken images, slow videos, or failed streaming on media-heavy sites (e.g. Shopee, TikTok, YouTube).
- Auditing resolver quality against real-world commercial CDNs.

---

## 1. Prerequisites & Environment Check

1. **Verify Client Active Resolver:**
   Confirm that the testing machine's active network adapter uses the Netora DNS server:
   ```powershell
   Get-DnsClientServerAddress -AddressFamily IPv4 | Format-Table InterfaceAlias, ServerAddresses
   ```
2. **Verify Resolver Service Status:**
   ```bash
   ssh <dns-host> "systemctl status unbound && unbound-control status"
   ```

---

## 2. Browser Subagent Visual Testing Workflow

Never rely solely on CLI `dig` or `nslookup` for production validation, as they cannot test dynamic multi-CDN asset chains and lazy-loaded resources.

### 2.1. Guardrails & Invocation Guidelines
- **Strict Step Limit:** Set `MaxSteps: 6-10`. Never allow open-ended or indefinite navigation.
- **Measurable Stop Condition:** Stop immediately once hero banners, category icons, product thumbnails, or live player elements are rendered and captured.
- **Bot Detection Precaution:** Avoid rapid-fire navigation in headless sessions that trigger anti-bot WAF challenges (e.g. Shopee Captcha). Introduce small settling pauses (1-2 seconds) between scrolls to allow lazy-loading intersection observers to fire.

### 2.2. Typical Task Structure for Browser Subagent
```text
TaskName: Validating Media and CDN Delivery on <TargetSite>
RecordingName: <sitename>_dns_test
Constraints:
- Maximum 8 steps.
- Stop Condition: Stop once homepage banners and media-heavy product/video feed have rendered.
Steps:
1. Navigate directly to <TargetURL>.
2. Dismiss any initial promo pop-up/dialog.
3. Observe hero banners and navigation icons.
4. Scroll down moderately to observe product feed thumbnails or video players.
5. Capture screenshot and record WebP video artifact.
6. Report whether all media assets rendered cleanly or if placeholder icons appeared.
```

---

## 3. Investigating Asset Failures & CDN Non-Compliance

If images fail with placeholders or requests time out:

1. **Identify the CDN Domain:**
   Check the image source domain (e.g. `down-id.img.susercontent.com`, `cf.shopee.co.id`).
2. **Benchmark Resolution via Resolver:**
   ```powershell
   Resolve-DnsName -Name <domain> -Server <resolver-ip>
   ```
   If it returns `DNS_ERROR_RCODE_NAME_ERROR` (NXDOMAIN), test against a public resolver (`8.8.8.8`):
   ```powershell
   Resolve-DnsName -Name <domain> -Server 8.8.8.8
   ```
3. **Trace Intermediate Labels for RFC 7816 Violations:**
   Query the authoritative nameserver directly for intermediate empty non-terminals:
   ```bash
   dig @<auth-ns> <intermediate-label> A
   ```
   If the authoritative nameserver returns `NXDOMAIN` for intermediate labels while the leaf domain exists, the CDN violates RFC 7816.
4. **Remediation:**
   - Ensure `qname-minimisation: no` is set in `/etc/unbound/unbound.conf`.
   - Flush affected cached negative zones:
     ```bash
     unbound-control flush_zone <zone>
     ```
   - Re-test resolution and verify visual rendering with the browser subagent.

---

## 4. Deliverables Checklist
- [ ] Active client DNS verification proof
- [ ] Pre- and post-optimization latency benchmark table
- [ ] Browser Subagent screenshots (`.png`) and session recording (`.webp`) in artifact directory
- [ ] Root workspace `walkthrough.md` updated with the findings and hard evidence
