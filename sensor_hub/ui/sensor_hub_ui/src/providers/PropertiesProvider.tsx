import { type ReactNode, useCallback, useState } from 'react';
import { WEBSOCKET_BASE } from '../environment/Environment';
import { useReconnectingWebSocket } from '../hooks/useReconnectingWebSocket';
import { logger } from '../tools/logger';
import { useAuth } from './AuthContext';
import { PropertiesContext } from './PropertiesContext';

interface PropertiesProviderProps {
  children: ReactNode;
}

export default function PropertiesProvider({ children }: PropertiesProviderProps) {
  const [properties, setProperties] = useState<Record<string, string>>({});
  const { user } = useAuth();

  const handleMessage = useCallback((event: MessageEvent) => {
    if (!event.data || event.data === 'null') return;
    try {
      const updatedProperties = JSON.parse(event.data) as Record<string, string>;
      setProperties(updatedProperties);
      logger.debug('Received properties update via WebSocket:', updatedProperties);
    } catch (err) {
      logger.error('Failed to handle properties WebSocket message:', err);
    }
  }, []);

  useReconnectingWebSocket({
    url: `${WEBSOCKET_BASE}/properties/ws`,
    onMessage: handleMessage,
    enabled: user != null,
  });

  return (
    <PropertiesContext.Provider value={properties}>
      {children}
    </PropertiesContext.Provider>
  );
}
