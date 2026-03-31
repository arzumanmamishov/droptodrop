package main

const privacyPolicyHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Privacy Policy — DropToDrop</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 800px; margin: 0 auto; padding: 40px 20px; color: #333; line-height: 1.7; }
h1 { color: #2d6a4f; border-bottom: 2px solid #d8f3dc; padding-bottom: 12px; }
h2 { color: #2d6a4f; margin-top: 32px; }
a { color: #2d6a4f; }
.updated { color: #6c757d; font-size: 14px; }
</style>
</head>
<body>
<h1>Privacy Policy</h1>
<p class="updated">Last updated: March 2026</p>

<h2>1. Introduction</h2>
<p>DropToDrop ("we", "our", "us") is a Shopify application that connects product suppliers with resellers for dropshipping. This Privacy Policy explains how we collect, use, and protect your information when you use our app.</p>

<h2>2. Information We Collect</h2>
<p>When you install DropToDrop, we collect:</p>
<ul>
<li><strong>Shop Information:</strong> Your Shopify store domain, shop name, and email address (provided by Shopify during OAuth installation).</li>
<li><strong>Product Data:</strong> Product titles, descriptions, images, prices, and variants that you choose to list on our marketplace.</li>
<li><strong>Order Data:</strong> Customer shipping name, address, email, and phone number — only for orders that are routed through our platform. This data is necessary to fulfill orders.</li>
<li><strong>Access Token:</strong> Your Shopify API access token, which is encrypted using AES-256-GCM and stored securely.</li>
</ul>

<h2>3. How We Use Your Information</h2>
<p>We use collected information to:</p>
<ul>
<li>Operate the dropshipping marketplace (listing products, routing orders, syncing fulfillment)</li>
<li>Display your products to potential resellers</li>
<li>Route customer orders from resellers to suppliers</li>
<li>Sync fulfillment tracking information</li>
<li>Send in-app notifications about orders, messages, and disputes</li>
<li>Generate analytics and usage reports for your dashboard</li>
</ul>

<h2>4. Data Sharing</h2>
<p>We share data only between connected suppliers and resellers within the platform:</p>
<ul>
<li>Suppliers see reseller order details (shipping address, items) to fulfill orders</li>
<li>Resellers see supplier product information (titles, prices, images) to import products</li>
</ul>
<p>We do not sell your data to third parties. We do not share data with any external services except Shopify's own APIs.</p>

<h2>5. Data Retention</h2>
<p>We retain your data for as long as your app is installed. You can configure data retention days in the app Settings. When you uninstall the app:</p>
<ul>
<li>Your shop is deactivated and sessions are invalidated immediately</li>
<li>Customer PII is redacted within 48 hours per Shopify's compliance requirements</li>
<li>You can request complete data deletion by contacting us</li>
</ul>

<h2>6. Data Security</h2>
<ul>
<li>Access tokens are encrypted with AES-256-GCM</li>
<li>All API communication uses HTTPS</li>
<li>Webhook payloads are verified with HMAC-SHA256</li>
<li>Database access is restricted and connection-pooled</li>
<li>Rate limiting protects against abuse</li>
</ul>

<h2>7. GDPR Compliance</h2>
<p>We comply with GDPR and Shopify's mandatory privacy requirements:</p>
<ul>
<li><strong>Data Access Request:</strong> We process customer data access requests received via Shopify webhooks</li>
<li><strong>Data Deletion:</strong> We redact customer PII (name, address, email, phone) upon receiving deletion requests</li>
<li><strong>Shop Data Deletion:</strong> We remove all shop data 48 hours after app uninstall</li>
</ul>

<h2>8. Your Rights</h2>
<p>You have the right to:</p>
<ul>
<li>Access your data through the app dashboard and audit logs</li>
<li>Delete your data by uninstalling the app or contacting us</li>
<li>Export your data via the CSV export feature</li>
<li>Restrict processing by pausing or archiving your listings</li>
</ul>

<h2>9. Contact Us</h2>
<p>For privacy-related inquiries, contact us at the support email configured in your app Settings page.</p>
</body>
</html>`

const termsOfServiceHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Terms of Service — DropToDrop</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 800px; margin: 0 auto; padding: 40px 20px; color: #333; line-height: 1.7; }
h1 { color: #2d6a4f; border-bottom: 2px solid #d8f3dc; padding-bottom: 12px; }
h2 { color: #2d6a4f; margin-top: 32px; }
a { color: #2d6a4f; }
.updated { color: #6c757d; font-size: 14px; }
</style>
</head>
<body>
<h1>Terms of Service</h1>
<p class="updated">Last updated: March 2026</p>

<h2>1. Acceptance of Terms</h2>
<p>By installing and using DropToDrop ("the App"), you agree to these Terms of Service. If you do not agree, please uninstall the app.</p>

<h2>2. Description of Service</h2>
<p>DropToDrop is a Shopify application that provides a dropshipping marketplace connecting product suppliers with resellers. The App facilitates product listing, importing, order routing, and fulfillment tracking between Shopify stores.</p>

<h2>3. Accounts and Roles</h2>
<ul>
<li><strong>Suppliers</strong> list their products for resellers to import and sell. Suppliers are responsible for fulfilling orders and providing accurate product information.</li>
<li><strong>Resellers</strong> import supplier products to their own stores. Resellers are responsible for customer service and setting retail prices.</li>
</ul>

<h2>4. Billing and Pricing</h2>
<ul>
<li><strong>Free Plan:</strong> €0/month — 5 products, 10 orders/month, no platform fees</li>
<li><strong>Standard Plan:</strong> €29/month — unlimited products and orders, 2% per-order platform fee, 14-day free trial</li>
<li><strong>Premium Plan:</strong> €79/month — unlimited products and orders, 0% platform fees, priority support, 14-day free trial</li>
</ul>
<p>Prices are in EUR. Billing is managed through the Shopify billing system. You can cancel anytime from the Billing page in the app.</p>

<h2>5. Supplier Responsibilities</h2>
<ul>
<li>Provide accurate product information (titles, descriptions, images, prices)</li>
<li>Maintain adequate inventory levels</li>
<li>Fulfill accepted orders within the stated processing time</li>
<li>Provide valid tracking information</li>
<li>Respond to disputes in a timely manner</li>
</ul>

<h2>6. Reseller Responsibilities</h2>
<ul>
<li>Set fair retail prices for imported products</li>
<li>Handle customer service for end customers</li>
<li>Not misrepresent supplier products</li>
<li>Pay any applicable platform fees</li>
</ul>

<h2>7. Disputes</h2>
<p>Either party may file a dispute for order issues (quality, wrong item, damage, non-delivery). We provide a dispute resolution system within the app but do not mediate financial disputes between merchants. Both parties should work in good faith to resolve issues.</p>

<h2>8. Trust and Reliability</h2>
<p>Suppliers receive a reliability score based on fulfillment rate, shipping speed, and dispute ratio. Suppliers with fulfillment rates below 80% may have their listings automatically paused to protect resellers.</p>

<h2>9. Intellectual Property</h2>
<p>Product listings, images, and descriptions remain the property of their respective owners. By listing products on DropToDrop, suppliers grant resellers a non-exclusive license to display and sell those products.</p>

<h2>10. Limitation of Liability</h2>
<p>DropToDrop facilitates connections between suppliers and resellers but is not a party to any transaction. We are not liable for product quality, shipping delays, or disputes between merchants. Our liability is limited to the amount of fees paid to us in the 12 months preceding any claim.</p>

<h2>11. Termination</h2>
<p>You can terminate your use of the App at any time by uninstalling it from your Shopify store. We may terminate or suspend accounts that violate these terms, have consistently low trust scores, or engage in fraudulent activity.</p>

<h2>12. Changes to Terms</h2>
<p>We may update these Terms from time to time. Continued use of the App after changes constitutes acceptance of the new terms.</p>

<h2>13. Governing Law</h2>
<p>These Terms are governed by the laws of the European Union. Any disputes shall be resolved through the courts of the jurisdiction where the App operator is registered.</p>

<h2>14. Contact</h2>
<p>For questions about these Terms, contact us at the support email configured in your app Settings page.</p>
</body>
</html>`
