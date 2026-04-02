import { ReactNode, useCallback, useState, useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Frame, Navigation, TopBar } from '@shopify/polaris';
import {
  HomeIcon,
  ProductIcon,
  OrderIcon,
  SettingsIcon,
  ListBulletedIcon,
  ImportIcon,
  StoreIcon,
  ChartVerticalFilledIcon,
  AlertCircleIcon,
  NotificationIcon,
  CashDollarIcon,
  ChatIcon,
  MegaphoneIcon,
  StarIcon,
  DiscountIcon,
  PersonIcon,
  DeliveryIcon,
  InventoryIcon,
} from '@shopify/polaris-icons';
import { Shop } from '../types';
import { api } from '../utils/api';

interface AppFrameProps {
  shop: Shop;
  children: ReactNode;
}

interface NavCounts {
  orders: number;
  messages: number;
  notifications: number;
  payouts: number;
  disputes: number;
}

export default function AppFrame({ shop, children }: AppFrameProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [mobileNavActive, setMobileNavActive] = useState(false);
  const [counts, setCounts] = useState<NavCounts>({ orders: 0, messages: 0, notifications: 0, payouts: 0, disputes: 0 });

  const toggleMobileNav = useCallback(() => setMobileNavActive((v) => !v), []);

  useEffect(() => {
    const fetchCounts = () => {
      api.get<NavCounts>('/nav-counts').then(setCounts).catch(() => {});
    };
    fetchCounts();
    const interval = setInterval(fetchCounts, 30000);
    return () => clearInterval(interval);
  }, []);

  const badge = (count: number) => count > 0 ? String(count) : undefined;

  const mapItems = (items: typeof supplierMain) =>
    items.map((item) => ({ ...item, onClick: () => navigate(item.url) }));

  // === SUPPLIER NAV ===
  const supplierMain = [
    { url: '/', label: 'Dashboard', icon: HomeIcon, selected: location.pathname === '/' },
    { url: '/orders', label: 'Orders', icon: OrderIcon, selected: location.pathname.startsWith('/orders'), badge: badge(counts.orders) },
    { url: '/supplier/listings', label: 'Listings', icon: ProductIcon, selected: location.pathname === '/supplier/listings' },
    { url: '/payouts', label: 'Payouts', icon: CashDollarIcon, selected: location.pathname === '/payouts', badge: badge(counts.payouts) },
    { url: '/messages', label: 'Messages', icon: ChatIcon, selected: location.pathname === '/messages', badge: badge(counts.messages) },
    { url: '/notifications', label: 'Notifications', icon: NotificationIcon, selected: location.pathname === '/notifications', badge: badge(counts.notifications) },
    { url: '/disputes', label: 'Disputes', icon: AlertCircleIcon, selected: location.pathname === '/disputes', badge: badge(counts.disputes) },
  ];
  const supplierMore = [
    { url: '/supplier/resellers', label: 'My Resellers', icon: PersonIcon, selected: location.pathname === '/supplier/resellers' },
    { url: '/analytics', label: 'Analytics', icon: ChartVerticalFilledIcon, selected: location.pathname === '/analytics' },
    { url: '/shipping-rules', label: 'Shipping', icon: DeliveryIcon, selected: location.pathname === '/shipping-rules' },
    { url: '/announcements', label: 'Announcements', icon: MegaphoneIcon, selected: location.pathname === '/announcements' },
    { url: '/reviews', label: 'Reviews', icon: StarIcon, selected: location.pathname === '/reviews' },
    { url: '/deals', label: 'Deals', icon: DiscountIcon, selected: location.pathname === '/deals' },
    { url: '/samples', label: 'Samples', icon: InventoryIcon, selected: location.pathname === '/samples' },
    { url: '/billing', label: 'Billing', icon: CashDollarIcon, selected: location.pathname === '/billing' },
    { url: '/audit', label: 'Audit Log', icon: ListBulletedIcon, selected: location.pathname === '/audit' },
    { url: '/supplier/setup', label: 'Settings', icon: SettingsIcon, selected: location.pathname === '/supplier/setup' },
  ];

  // === RESELLER NAV ===
  const resellerMain = [
    { url: '/', label: 'Dashboard', icon: HomeIcon, selected: location.pathname === '/' },
    { url: '/marketplace', label: 'Marketplace', icon: StoreIcon, selected: location.pathname === '/marketplace' },
    { url: '/orders', label: 'Orders', icon: OrderIcon, selected: location.pathname.startsWith('/orders'), badge: badge(counts.orders) },
    { url: '/imports', label: 'Imports', icon: ImportIcon, selected: location.pathname === '/imports' },
    { url: '/payouts', label: 'Payouts', icon: CashDollarIcon, selected: location.pathname === '/payouts', badge: badge(counts.payouts) },
    { url: '/messages', label: 'Messages', icon: ChatIcon, selected: location.pathname === '/messages', badge: badge(counts.messages) },
    { url: '/notifications', label: 'Notifications', icon: NotificationIcon, selected: location.pathname === '/notifications', badge: badge(counts.notifications) },
    { url: '/disputes', label: 'Disputes', icon: AlertCircleIcon, selected: location.pathname === '/disputes', badge: badge(counts.disputes) },
  ];
  const resellerMore = [
    { url: '/analytics', label: 'Analytics', icon: ChartVerticalFilledIcon, selected: location.pathname === '/analytics' },
    { url: '/announcements', label: 'Announcements', icon: MegaphoneIcon, selected: location.pathname === '/announcements' },
    { url: '/reviews', label: 'Reviews', icon: StarIcon, selected: location.pathname === '/reviews' },
    { url: '/deals', label: 'Deals', icon: DiscountIcon, selected: location.pathname === '/deals' },
    { url: '/samples', label: 'Samples', icon: InventoryIcon, selected: location.pathname === '/samples' },
    { url: '/billing', label: 'Billing', icon: CashDollarIcon, selected: location.pathname === '/billing' },
    { url: '/audit', label: 'Audit Log', icon: ListBulletedIcon, selected: location.pathname === '/audit' },
    { url: '/reseller/settings', label: 'Settings', icon: SettingsIcon, selected: location.pathname === '/reseller/settings' },
  ];

  const mainItems = shop.role === 'supplier' ? supplierMain : resellerMain;
  const moreItems = shop.role === 'supplier' ? supplierMore : resellerMore;

  const navigation = (
    <Navigation location={location.pathname}>
      <Navigation.Section
        title="DropToDrop"
        items={mapItems(mainItems)}
      />
      <Navigation.Section
        title="More"
        separator
        items={mapItems(moreItems)}
      />
    </Navigation>
  );

  const topBar = (
    <TopBar
      showNavigationToggle
      onNavigationToggle={toggleMobileNav}
    />
  );

  return (
    <Frame
      topBar={topBar}
      navigation={navigation}
      showMobileNavigation={mobileNavActive}
      onNavigationDismiss={toggleMobileNav}
    >
      {children}
    </Frame>
  );
}
