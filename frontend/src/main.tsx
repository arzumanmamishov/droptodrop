import React from 'react';
import ReactDOM from 'react-dom/client';
import AppBridgeProvider from './providers/AppBridgeProvider';
import App from './App';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AppBridgeProvider>
      <App />
    </AppBridgeProvider>
  </React.StrictMode>,
);
