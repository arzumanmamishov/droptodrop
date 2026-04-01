import { useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AppProvider } from '@shopify/polaris';
import enTranslations from '@shopify/polaris/locales/en.json';
import '@shopify/polaris/build/esm/styles.css';

import { api, setSessionToken } from './utils/api';
import { Shop } from './types';

import AppFrame from './components/AppFrame';
import Dashboard from './pages/Dashboard';
import SupplierSetup from './pages/SupplierSetup';
import SupplierListings from './pages/SupplierListings';
import Marketplace from './pages/Marketplace';
import Imports from './pages/Imports';
import Orders from './pages/Orders';
import OrderDetail from './pages/OrderDetail';
import Settings from './pages/Settings';
import AuditLog from './pages/AuditLog';
import ListingEdit from './pages/ListingEdit';
import SupplierInfo from './pages/SupplierInfo';
import Analytics from './pages/Analytics';
import Disputes from './pages/Disputes';
import Notifications from './pages/Notifications';
import Resellers from './pages/Resellers';
import Payouts from './pages/Payouts';
import Billing from './pages/Billing';
import BulkImport from './pages/BulkImport';
import Messages from './pages/Messages';
import Announcements from './pages/Announcements';
import Reviews from './pages/Reviews';
import Deals from './pages/Deals';
import ShippingRules from './pages/ShippingRules';
import Samples from './pages/Samples';
import SetupWizard from './pages/SetupWizard';
import Onboarding from './pages/Onboarding';
import ResellerSettings from './pages/ResellerSettings';

export default function App() {
  const [shop, setShop] = useState<Shop | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Extract session token from URL if present
    const params = new URLSearchParams(window.location.search);
    const sessionParam = params.get('session');
    if (sessionParam) {
      setSessionToken(sessionParam);
    }

    // Fetch current shop
    api
      .get<Shop>('/shop')
      .then((data) => {
        setShop(data);
      })
      .catch(() => {
        // Not authenticated
        setShop(null);
      })
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <AppProvider i18n={enTranslations}>
        <div style={{ display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', height: '100vh', gap: '16px' }}>
          <div style={{ width: '40px', height: '40px', border: '4px solid #e9ecef', borderTop: '4px solid #1e40af', borderRadius: '50%', animation: 'spin 1s linear infinite' }} />
          <p style={{ color: '#6d7175', fontSize: '14px' }}>Loading DropToDrop...</p>
          <style>{`@keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }`}</style>
        </div>
      </AppProvider>
    );
  }

  const needsOnboarding = shop && shop.role === 'unset';

  return (
    <AppProvider i18n={enTranslations}>
      <BrowserRouter>
        {needsOnboarding ? (
          <Routes>
            <Route path="*" element={<Onboarding onComplete={(role) => setShop({ ...shop, role: role as Shop['role'] })} />} />
          </Routes>
        ) : shop ? (
          <AppFrame shop={shop}>
            <Routes>
              <Route path="/" element={<Dashboard />} />
              {shop.role === 'supplier' && (
                <>
                  <Route path="/supplier/setup" element={<SupplierSetup />} />
                  <Route path="/supplier/listings" element={<SupplierListings />} />
                  <Route path="/supplier/resellers" element={<Resellers />} />
                  <Route path="/supplier/listings/:id" element={<ListingEdit />} />
                </>
              )}
              {shop.role === 'reseller' && (
                <>
                  <Route path="/marketplace" element={<Marketplace />} />
                  <Route path="/bulk-import" element={<BulkImport />} />
                  <Route path="/supplier/:id" element={<SupplierInfo />} />
                  <Route path="/imports" element={<Imports />} />
                  <Route path="/reseller/settings" element={<ResellerSettings />} />
                </>
              )}
              <Route path="/analytics" element={<Analytics />} />
              <Route path="/disputes" element={<Disputes />} />
              <Route path="/notifications" element={<Notifications />} />
              <Route path="/payouts" element={<Payouts role={shop.role} />} />
              <Route path="/billing" element={<Billing />} />
              <Route path="/messages" element={<Messages />} />
              <Route path="/announcements" element={<Announcements role={shop.role} />} />
              <Route path="/reviews" element={<Reviews role={shop.role} shopId={shop.id} />} />
              <Route path="/deals" element={<Deals role={shop.role} />} />
              <Route path="/shipping-rules" element={<ShippingRules />} />
              <Route path="/samples" element={<Samples role={shop.role} />} />
              <Route path="/setup-wizard" element={<SetupWizard role={shop.role} onComplete={() => window.location.href = '/'} />} />
              <Route path="/orders" element={<Orders role={shop.role} />} />
              <Route path="/orders/:id" element={<OrderDetail role={shop.role} />} />
              <Route path="/settings" element={<Settings />} />
              <Route path="/audit" element={<AuditLog />} />
              <Route path="*" element={<Navigate to="/" />} />
            </Routes>
          </AppFrame>
        ) : (
          <div style={{ padding: '3rem', textAlign: 'center', maxWidth: '500px', margin: '0 auto' }}>
            <div style={{ fontSize: '48px', marginBottom: '16px' }}>🔒</div>
            <h2 style={{ fontSize: '20px', fontWeight: 600, marginBottom: '8px', color: '#202223' }}>App Not Connected</h2>
            <p style={{ color: '#6d7175', marginBottom: '20px', lineHeight: 1.6 }}>
              To use DropToDrop, please open this app from your Shopify admin panel.
              Go to your Shopify store → Apps → DropToDrop.
            </p>
            <a href="https://admin.shopify.com" style={{
              display: 'inline-block', padding: '10px 24px', background: '#1e40af',
              color: 'white', borderRadius: '8px', textDecoration: 'none', fontWeight: 500,
            }}>Go to Shopify Admin</a>
          </div>
        )}
      </BrowserRouter>
    </AppProvider>
  );
}
