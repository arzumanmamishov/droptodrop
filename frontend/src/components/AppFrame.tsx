import { ReactNode, useCallback, useState } from 'react';
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

interface AppFrameProps {
  shop: Shop;
  children: ReactNode;
}

export default function AppFrame({ shop, children }: AppFrameProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [mobileNavActive, setMobileNavActive] = useState(false);

  const toggleMobileNav = useCallback(() => setMobileNavActive((v) => !v), []);

  const supplierNavItems = [
    { url: '/', label: 'Dashboard', icon: HomeIcon, selected: location.pathname === '/' },
    { url: '/supplier/setup', label: 'Supplier Setup', icon: StoreIcon, selected: location.pathname === '/supplier/setup' },
    { url: '/supplier/listings', label: 'Listings', icon: ProductIcon, selected: location.pathname === '/supplier/listings' },
    { url: '/supplier/resellers', label: 'My Resellers', icon: PersonIcon, selected: location.pathname === '/supplier/resellers' },
    { url: '/orders', label: 'Orders', icon: OrderIcon, selected: location.pathname.startsWith('/orders') },
    { url: '/analytics', label: 'Analytics', icon: ChartVerticalFilledIcon, selected: location.pathname === '/analytics' },
    { url: '/disputes', label: 'Disputes', icon: AlertCircleIcon, selected: location.pathname === '/disputes' },
    { url: '/messages', label: 'Messages', icon: ChatIcon, selected: location.pathname === '/messages' },
    { url: '/announcements', label: 'Announcements', icon: MegaphoneIcon, selected: location.pathname === '/announcements' },
    { url: '/reviews', label: 'Reviews', icon: StarIcon, selected: location.pathname === '/reviews' },
    { url: '/deals', label: 'Deals', icon: DiscountIcon, selected: location.pathname === '/deals' },
    { url: '/shipping-rules', label: 'Shipping', icon: DeliveryIcon, selected: location.pathname === '/shipping-rules' },
    { url: '/samples', label: 'Samples', icon: InventoryIcon, selected: location.pathname === '/samples' },
    { url: '/notifications', label: 'Notifications', icon: NotificationIcon, selected: location.pathname === '/notifications' },
    { url: '/payouts', label: 'Payouts', icon: CashDollarIcon, selected: location.pathname === '/payouts' },
    { url: '/billing', label: 'Billing', icon: CashDollarIcon, selected: location.pathname === '/billing' },
    { url: '/audit', label: 'Audit Log', icon: ListBulletedIcon, selected: location.pathname === '/audit' },
  ];

  const resellerNavItems = [
    { url: '/', label: 'Dashboard', icon: HomeIcon, selected: location.pathname === '/' },
    { url: '/marketplace', label: 'Marketplace', icon: StoreIcon, selected: location.pathname === '/marketplace' },
    { url: '/imports', label: 'Imports', icon: ImportIcon, selected: location.pathname === '/imports' },
    { url: '/orders', label: 'Orders', icon: OrderIcon, selected: location.pathname.startsWith('/orders') },
    { url: '/analytics', label: 'Analytics', icon: ChartVerticalFilledIcon, selected: location.pathname === '/analytics' },
    { url: '/disputes', label: 'Disputes', icon: AlertCircleIcon, selected: location.pathname === '/disputes' },
    { url: '/messages', label: 'Messages', icon: ChatIcon, selected: location.pathname === '/messages' },
    { url: '/announcements', label: 'Announcements', icon: MegaphoneIcon, selected: location.pathname === '/announcements' },
    { url: '/reviews', label: 'Reviews', icon: StarIcon, selected: location.pathname === '/reviews' },
    { url: '/deals', label: 'Deals', icon: DiscountIcon, selected: location.pathname === '/deals' },
    { url: '/samples', label: 'Samples', icon: InventoryIcon, selected: location.pathname === '/samples' },
    { url: '/notifications', label: 'Notifications', icon: NotificationIcon, selected: location.pathname === '/notifications' },
    { url: '/payouts', label: 'Payouts', icon: CashDollarIcon, selected: location.pathname === '/payouts' },
    { url: '/billing', label: 'Billing', icon: CashDollarIcon, selected: location.pathname === '/billing' },
    { url: '/audit', label: 'Audit Log', icon: ListBulletedIcon, selected: location.pathname === '/audit' },
    { url: '/reseller/settings', label: 'Settings', icon: SettingsIcon, selected: location.pathname === '/reseller/settings' },
  ];

  const navItems = shop.role === 'supplier' ? supplierNavItems : resellerNavItems;

  const navigation = (
    <Navigation location={location.pathname}>
      <Navigation.Section
        title="DropToDrop"
        items={navItems.map((item) => ({
          ...item,
          onClick: () => navigate(item.url),
        }))}
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
