import React from 'react';
import ReactDOM from 'react-dom/client';
import AppBridgeProvider from './providers/AppBridgeProvider';
import ErrorBoundary from './components/ErrorBoundary';
import App from './App';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <AppBridgeProvider>
        <App />
      </AppBridgeProvider>
    </ErrorBoundary>
  </React.StrictMode>,
);
